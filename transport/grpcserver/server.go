package grpcserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	serializationv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/api/gen/go/meido/serialization/v1"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/blobstore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	APIVersion             = "meido.serialization.v1"
	DefaultMaxInlineBytes  = 3 << 20
	MaxInlineBytes         = 3 << 20
	DefaultChunkBytes      = 256 << 10
	MaxChunkBytes          = 1 << 20
	DefaultArchivePageSize = 128
	MaxArchivePageSize     = 1000
	MaxArchivePageBytes    = 2 << 20
)

type Config struct {
	Engine         *application.Engine
	Roots          *application.RootSet
	Blobs          *blobstore.Store
	MaxInlineBytes int64
	ChunkBytes     int
}

type Server struct {
	serializationv1.UnimplementedSerializationServiceServer
	engine         *application.Engine
	roots          *application.RootSet
	blobs          *blobstore.Store
	maxInlineBytes int64
	chunkBytes     int
	archivePager   *application.ArchivePager
}

func New(config Config) (*Server, error) {
	if config.Engine == nil || config.Blobs == nil {
		return nil, fmt.Errorf("application engine and blob store are required")
	}
	if config.Roots == nil {
		config.Roots = application.NewRootSet()
	}
	if config.MaxInlineBytes <= 0 {
		config.MaxInlineBytes = DefaultMaxInlineBytes
	}
	if config.MaxInlineBytes > MaxInlineBytes {
		return nil, fmt.Errorf("max inline size %d exceeds the safe limit %d", config.MaxInlineBytes, MaxInlineBytes)
	}
	if config.ChunkBytes <= 0 {
		config.ChunkBytes = DefaultChunkBytes
	}
	if config.ChunkBytes > MaxChunkBytes {
		return nil, fmt.Errorf("chunk size %d exceeds limit %d", config.ChunkBytes, MaxChunkBytes)
	}
	archivePager, err := application.NewArchivePager()
	if err != nil {
		return nil, err
	}
	return &Server{
		engine: config.Engine, roots: config.Roots, blobs: config.Blobs,
		maxInlineBytes: config.MaxInlineBytes, chunkBytes: config.ChunkBytes, archivePager: archivePager,
	}, nil
}

func (s *Server) Register(registrar grpc.ServiceRegistrar) {
	serializationv1.RegisterSerializationServiceServer(registrar, s)
}

func (s *Server) GetCapabilities(context.Context, *serializationv1.GetCapabilitiesRequest) (*serializationv1.GetCapabilitiesResponse, error) {
	formats := s.engine.Formats()
	maxArchiveListingBytes, maxArchiveEntries := s.engine.ArchiveListingLimits()
	result := &serializationv1.GetCapabilitiesResponse{
		ApiVersion:             APIVersion,
		RootIds:                s.roots.IDs(),
		MaxInlineBytes:         s.maxInlineBytes,
		MaxArchiveListingBytes: maxArchiveListingBytes,
		MaxArchiveEntries:      int64(maxArchiveEntries),
		Formats:                make([]*serializationv1.FormatCapability, 0, len(formats)),
	}
	result.MaxBlobBytes, result.MaxTotalBlobBytes = s.blobs.Limits()
	maxBlobs, maxNameBytes := s.blobs.ObjectLimits()
	result.MaxBlobCount, result.MaxBlobNameBytes = int64(maxBlobs), int64(maxNameBytes)
	result.WritableRootIds = s.roots.WritableIDs()
	for _, format := range formats {
		result.Formats = append(result.Formats, &serializationv1.FormatCapability{
			Id: format.ID, Game: format.Game, FileType: format.FileType,
			NativeSuffixes: format.NativeSuffixes, CanDetect: format.Capability.Detect,
			CanConvert: format.Capability.Convert, CanValidate: format.Capability.Validate,
			IsArchive: format.Capability.Archive, HasEditingSchema: format.SchemaVersion != "",
			EditingSchemaVersion: format.SchemaVersion, EditingSchemaId: format.SchemaID,
			EditingSchemaSha256: format.SchemaSHA256,
			HasFormatGuide:      format.GuideVersion != "", FormatGuideVersion: format.GuideVersion,
			FormatGuideId: format.GuideID, FormatGuideSha256: format.GuideSHA256,
			FormatGuideCoverage: format.GuideCoverage,
		})
	}
	return result, nil
}

func (s *Server) GetFormatGuide(ctx context.Context, request *serializationv1.GetFormatGuideRequest) (*serializationv1.GetFormatGuideResponse, error) {
	if request == nil || strings.TrimSpace(request.GetFormatId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "format_id is required")
	}
	document, err := s.engine.GetFormatGuide(request.GetFormatId())
	if err != nil {
		return nil, rpcError(err)
	}
	return &serializationv1.GetFormatGuideResponse{
		FormatId: document.FormatID, GuideVersion: document.Version, GuideId: document.ID,
		MediaType: document.MediaType, Sha256: document.SHA256, SchemaId: document.SchemaID,
		Coverage: document.Coverage, GuideJson: append([]byte(nil), document.JSON...),
	}, nil
}

func (s *Server) GetFormatSchema(ctx context.Context, request *serializationv1.GetFormatSchemaRequest) (*serializationv1.GetFormatSchemaResponse, error) {
	if request == nil || strings.TrimSpace(request.GetFormatId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "format_id is required")
	}
	document, err := s.engine.GetFormatSchema(request.GetFormatId())
	if err != nil {
		return nil, rpcError(err)
	}
	return &serializationv1.GetFormatSchemaResponse{
		FormatId: document.FormatID, Representation: representationToProto(document.Representation),
		SchemaVersion: document.Version, SchemaId: document.ID, Dialect: document.Dialect,
		MediaType: document.MediaType, Sha256: document.SHA256,
		SchemaJson: append([]byte(nil), document.JSON...), NativeSuffixes: document.NativeSuffixes,
	}, nil
}

func (s *Server) Detect(ctx context.Context, request *serializationv1.DetectRequest) (*serializationv1.DetectResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	source, err := s.resolveInput(ctx, request.GetInput())
	if err != nil {
		return nil, rpcError(err)
	}
	detection, err := s.engine.Detect(ctx, source)
	if err != nil {
		return nil, rpcError(err)
	}
	return detectionMessage(detection), nil
}

func (s *Server) Convert(ctx context.Context, request *serializationv1.ConvertRequest) (*serializationv1.ConvertResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	target, err := representationFromProto(request.GetTarget())
	if err != nil {
		return nil, rpcError(err)
	}
	source, err := s.resolveInput(ctx, request.GetInput())
	if err != nil {
		return nil, rpcError(err)
	}
	result, err := s.captureResult(ctx, request.GetPreferBlob(), func(writer io.Writer) (application.Artifact, error) {
		return s.engine.Convert(ctx, application.ConvertRequest{Source: source, FormatID: request.GetFormatId(), To: target}, writer)
	})
	if err != nil {
		return nil, err
	}
	return &serializationv1.ConvertResponse{Result: result}, nil
}

func (s *Server) Validate(ctx context.Context, request *serializationv1.ValidateRequest) (*serializationv1.ValidateResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	source, err := s.resolveInput(ctx, request.GetInput())
	if err != nil {
		return nil, rpcError(err)
	}
	detection, err := s.engine.Validate(ctx, source, request.GetFormatId())
	if err != nil {
		return nil, rpcError(err)
	}
	return &serializationv1.ValidateResponse{Valid: true, Detection: detectionMessage(detection)}, nil
}

func (s *Server) Upload(stream grpc.ClientStreamingServer[serializationv1.UploadRequest, serializationv1.UploadResponse]) error {
	first, err := stream.Recv()
	if err != nil {
		if status.Code(err) != codes.Unknown || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return rpcError(err)
		}
		return status.Errorf(codes.InvalidArgument, "receive upload metadata: %v", err)
	}
	metadata := first.GetMetadata()
	if metadata == nil || strings.TrimSpace(metadata.GetName()) == "" {
		return status.Error(codes.InvalidArgument, "the first upload message must contain a non-empty name")
	}
	reader := &uploadReader{stream: stream}
	meta, err := s.blobs.Put(stream.Context(), metadata.GetName(), reader)
	if err != nil {
		return blobError("store upload", err)
	}
	return stream.SendAndClose(&serializationv1.UploadResponse{Blob: blobMetadataMessage(meta)})
}

func (s *Server) Download(request *serializationv1.DownloadRequest, stream grpc.ServerStreamingServer[serializationv1.DownloadResponse]) error {
	if request == nil || strings.TrimSpace(request.GetBlobId()) == "" {
		return status.Error(codes.InvalidArgument, "blob_id is required")
	}
	file, meta, err := s.blobs.Open(request.GetBlobId())
	if err != nil {
		return blobError("open blob", err)
	}
	defer file.Close()
	if err := stream.Send(&serializationv1.DownloadResponse{Value: &serializationv1.DownloadResponse_Metadata{Metadata: blobMetadataMessage(meta)}}); err != nil {
		return err
	}
	buffer := make([]byte, s.chunkBytes)
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			chunk := append([]byte(nil), buffer[:n]...)
			if err := stream.Send(&serializationv1.DownloadResponse{Value: &serializationv1.DownloadResponse_Chunk{Chunk: chunk}}); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return status.Errorf(codes.Internal, "read blob: %v", readErr)
		}
	}
}

func (s *Server) DeleteBlob(ctx context.Context, request *serializationv1.DeleteBlobRequest) (*serializationv1.DeleteBlobResponse, error) {
	if request == nil || strings.TrimSpace(request.GetBlobId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "blob_id is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, rpcError(err)
	}
	deleted, err := s.blobs.Delete(request.GetBlobId())
	if err != nil {
		return nil, blobError("delete blob", err)
	}
	return &serializationv1.DeleteBlobResponse{Deleted: deleted}, nil
}

func (s *Server) ListArchive(ctx context.Context, request *serializationv1.ListArchiveRequest) (*serializationv1.ListArchiveResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	pageSize, err := archivePageSize(request.GetPageSize())
	if err != nil {
		return nil, err
	}
	source, err := s.resolveInput(ctx, request.GetInput())
	if err != nil {
		return nil, rpcError(err)
	}
	listing, err := s.engine.ListArchiveListing(ctx, source, request.GetFormatId())
	if err != nil {
		return nil, rpcError(err)
	}
	start, err := s.archivePager.Decode(listing, request.GetPageToken())
	if err != nil {
		return nil, rpcError(err)
	}
	result := &serializationv1.ListArchiveResponse{FormatId: listing.FormatID, Entries: make([]*serializationv1.ArchiveEntry, 0, pageSize)}
	nextIndex := start
	for nextIndex < len(listing.Entries) && len(result.Entries) < pageSize {
		entry := listing.Entries[nextIndex]
		result.Entries = append(result.Entries, &serializationv1.ArchiveEntry{Name: entry.Name, Size: entry.Size, Kind: entry.Kind})
		if nextIndex+1 < len(listing.Entries) {
			result.NextPageToken, err = s.archivePager.Encode(listing, nextIndex+1)
			if err != nil {
				return nil, rpcError(err)
			}
		} else {
			result.NextPageToken = ""
		}
		if proto.Size(result) > MaxArchivePageBytes {
			result.Entries = result.Entries[:len(result.Entries)-1]
			if len(result.Entries) == 0 {
				return nil, status.Errorf(codes.ResourceExhausted, "archive entry at index %d exceeds page response limit %d", nextIndex, MaxArchivePageBytes)
			}
			result.NextPageToken, err = s.archivePager.Encode(listing, nextIndex)
			if err != nil {
				return nil, rpcError(err)
			}
			break
		}
		nextIndex++
	}
	return result, nil
}

func archivePageSize(value int32) (int, error) {
	if value < 0 {
		return 0, status.Error(codes.InvalidArgument, "page_size must not be negative")
	}
	if value == 0 {
		return DefaultArchivePageSize, nil
	}
	if value > MaxArchivePageSize {
		return MaxArchivePageSize, nil
	}
	return int(value), nil
}

func (s *Server) ExtractArchiveEntry(ctx context.Context, request *serializationv1.ExtractArchiveEntryRequest) (*serializationv1.ExtractArchiveEntryResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	source, err := s.resolveInput(ctx, request.GetInput())
	if err != nil {
		return nil, rpcError(err)
	}
	result, err := s.captureResult(ctx, request.GetPreferBlob(), func(writer io.Writer) (application.Artifact, error) {
		return s.engine.ExtractArchiveEntry(ctx, source, request.GetFormatId(), request.GetEntryName(), writer)
	})
	if err != nil {
		return nil, err
	}
	return &serializationv1.ExtractArchiveEntryResponse{Result: result}, nil
}

func (s *Server) resolveInput(ctx context.Context, input *serializationv1.ArtifactInput) (application.Source, error) {
	if input == nil {
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("input is required")}
	}
	inlineBytes := int64(len(input.GetInlineData()))
	for _, attachment := range input.GetAttachments() {
		if attachment == nil {
			continue
		}
		attachmentBytes := int64(len(attachment.GetInlineData()))
		if attachmentBytes > s.maxInlineBytes-inlineBytes {
			return nil, &application.OpError{Op: "resolve input", Code: application.CodeResourceExhausted, Err: fmt.Errorf("inline artifact bundle exceeds %d bytes; upload one or more files as blobs", s.maxInlineBytes)}
		}
		inlineBytes += attachmentBytes
	}
	primary, err := s.resolveInputLocation(input.GetName(), input.GetLocation())
	if err != nil {
		return nil, err
	}
	attachments := make([]application.SourceAttachment, 0, len(input.GetAttachments()))
	for index, attachment := range input.GetAttachments() {
		if attachment == nil || strings.TrimSpace(attachment.GetSuffix()) == "" {
			return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("attachment %d suffix is required", index)}
		}
		attachmentSource, attachmentErr := s.resolveInputLocation(input.GetName()+attachment.GetSuffix(), attachment.GetLocation())
		if attachmentErr != nil {
			return nil, attachmentErr
		}
		attachments = append(attachments, application.SourceAttachment{Suffix: attachment.GetSuffix(), Source: attachmentSource})
	}
	result, err := application.NewBundleSource(primary, attachments)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Server) resolveInputLocation(name string, location interface{}) (application.Source, error) {
	switch location := location.(type) {
	case *serializationv1.ArtifactInput_InlineData:
		return s.resolveInlineInput(name, location.InlineData)
	case *serializationv1.ArtifactAttachmentInput_InlineData:
		return s.resolveInlineInput(name, location.InlineData)
	case *serializationv1.ArtifactInput_File:
		return s.resolveFileInput(location.File)
	case *serializationv1.ArtifactAttachmentInput_File:
		return s.resolveFileInput(location.File)
	case *serializationv1.ArtifactInput_Blob:
		return s.resolveBlobInput(name, location.Blob)
	case *serializationv1.ArtifactAttachmentInput_Blob:
		return s.resolveBlobInput(name, location.Blob)
	default:
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("input location is required")}
	}
}

func (s *Server) resolveInlineInput(name string, data []byte) (application.Source, error) {
	if strings.TrimSpace(name) == "" {
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("inline input name is required")}
	}
	if strings.IndexByte(name, 0) >= 0 {
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("inline input name contains NUL")}
	}
	if int64(len(data)) > s.maxInlineBytes {
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeResourceExhausted, Err: fmt.Errorf("inline input exceeds %d bytes; upload it as a blob", s.maxInlineBytes)}
	}
	return application.NewBytesSource(name, data), nil
}

func (s *Server) resolveFileInput(file *serializationv1.FileRef) (application.Source, error) {
	if file == nil {
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("file reference is required")}
	}
	return s.roots.Resolve(file.GetRootId(), file.GetRelativePath())
}

func (s *Server) resolveBlobInput(name string, blob *serializationv1.BlobRef) (application.Source, error) {
	if blob == nil || strings.TrimSpace(blob.GetId()) == "" {
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("blob ID is required")}
	}
	meta, err := s.blobs.Metadata(blob.GetId())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		name = meta.Name
	}
	if strings.IndexByte(name, 0) >= 0 {
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("blob input name contains NUL")}
	}
	return &blobSource{store: s.blobs, id: meta.ID, name: cleanName(name), size: meta.Size}, nil
}

func (s *Server) captureResult(ctx context.Context, preferBlob bool, produce func(io.Writer) (application.Artifact, error)) (*serializationv1.ArtifactResult, error) {
	temp, err := os.CreateTemp("", "meido-rpc-result-")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create result buffer: %v", err)
	}
	path := temp.Name()
	defer os.Remove(path)
	artifact, err := produce(temp)
	closeErr := temp.Close()
	if err != nil {
		return nil, rpcError(err)
	}
	if closeErr != nil {
		return nil, status.Errorf(codes.Internal, "close result buffer: %v", closeErr)
	}
	if err := verifyArtifactOutput(path, artifact); err != nil {
		return nil, err
	}
	metadata := artifactMetadataMessage(artifact)
	createdBlobs := make([]string, 0, 1+len(artifact.AttachmentFiles()))
	cleanup := func() {
		for _, id := range createdBlobs {
			_, _ = s.blobs.Delete(id)
		}
	}
	inlineRemaining := s.maxInlineBytes
	if preferBlob || artifact.Size > inlineRemaining {
		file, err := os.Open(path)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "open result buffer: %v", err)
		}
		blob, putErr := s.blobs.Put(ctx, artifact.Name, file)
		_ = file.Close()
		if putErr != nil {
			return nil, blobError("store result blob", putErr)
		}
		createdBlobs = append(createdBlobs, blob.ID)
		result := &serializationv1.ArtifactResult{Metadata: metadata, Location: &serializationv1.ArtifactResult_Blob{Blob: &serializationv1.BlobRef{Id: blob.ID}}}
		if err := s.captureResultAttachments(ctx, preferBlob, artifact.AttachmentFiles(), result, &createdBlobs, &inlineRemaining); err != nil {
			cleanup()
			return nil, err
		}
		return result, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read result buffer: %v", err)
	}
	inlineRemaining -= int64(len(data))
	result := &serializationv1.ArtifactResult{Metadata: metadata, Location: &serializationv1.ArtifactResult_InlineData{InlineData: data}}
	if err := s.captureResultAttachments(ctx, preferBlob, artifact.AttachmentFiles(), result, &createdBlobs, &inlineRemaining); err != nil {
		cleanup()
		return nil, err
	}
	return result, nil
}

func (s *Server) captureResultAttachments(ctx context.Context, preferBlob bool, attachments []application.ArtifactAttachment, result *serializationv1.ArtifactResult, createdBlobs *[]string, inlineRemaining *int64) error {
	for _, attachment := range attachments {
		part := &serializationv1.ArtifactAttachmentResult{Suffix: attachment.Suffix, Name: attachment.Name, Size: attachment.Size, Sha256: attachment.SHA256}
		if preferBlob || attachment.Size > *inlineRemaining {
			blob, err := s.blobs.Put(ctx, attachment.Name, bytes.NewReader(attachment.Data))
			if err != nil {
				return blobError("store result attachment blob", err)
			}
			*createdBlobs = append(*createdBlobs, blob.ID)
			part.Location = &serializationv1.ArtifactAttachmentResult_Blob{Blob: &serializationv1.BlobRef{Id: blob.ID}}
		} else {
			part.Location = &serializationv1.ArtifactAttachmentResult_InlineData{InlineData: append([]byte(nil), attachment.Data...)}
			*inlineRemaining -= attachment.Size
		}
		result.Attachments = append(result.Attachments, part)
	}
	return nil
}

func verifyArtifactOutput(path string, artifact application.Artifact) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != artifact.Size {
		return status.Errorf(codes.Internal, "conversion output metadata mismatch")
	}
	file, err := os.Open(path)
	if err != nil {
		return status.Errorf(codes.Internal, "open conversion output: %v", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		return status.Errorf(codes.Internal, "verify conversion output: %v", errors.Join(copyErr, closeErr))
	}
	if hex.EncodeToString(hash.Sum(nil)) != artifact.SHA256 {
		return status.Error(codes.Internal, "conversion output digest mismatch")
	}
	for _, attachment := range artifact.AttachmentFiles() {
		if int64(len(attachment.Data)) != attachment.Size {
			return status.Error(codes.Internal, "conversion attachment metadata mismatch")
		}
		digest := sha256.Sum256(attachment.Data)
		if hex.EncodeToString(digest[:]) != attachment.SHA256 {
			return status.Error(codes.Internal, "conversion attachment digest mismatch")
		}
	}
	return nil
}

type blobSource struct {
	store *blobstore.Store
	id    string
	name  string
	size  int64
}

func (s *blobSource) Name() string { return s.name }
func (s *blobSource) Size() int64  { return s.size }
func (s *blobSource) Open(ctx context.Context) (application.ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, _, err := s.store.Open(s.id)
	return file, err
}

type uploadReader struct {
	stream grpc.ClientStreamingServer[serializationv1.UploadRequest, serializationv1.UploadResponse]
	chunk  []byte
}

func (r *uploadReader) Read(buffer []byte) (int, error) {
	for len(r.chunk) == 0 {
		message, err := r.stream.Recv()
		if err != nil {
			return 0, err
		}
		chunk := message.GetChunk()
		if chunk == nil {
			return 0, status.Error(codes.InvalidArgument, "upload metadata may only appear in the first message")
		}
		if len(chunk) > MaxChunkBytes {
			return 0, status.Errorf(codes.ResourceExhausted, "upload chunk exceeds %d bytes", MaxChunkBytes)
		}
		r.chunk = chunk
	}
	n := copy(buffer, r.chunk)
	r.chunk = r.chunk[n:]
	return n, nil
}

func representationFromProto(value serializationv1.Representation) (application.Representation, error) {
	switch value {
	case serializationv1.Representation_REPRESENTATION_NATIVE:
		return application.RepresentationNative, nil
	case serializationv1.Representation_REPRESENTATION_EDITING_JSON:
		return application.RepresentationEditingJSON, nil
	default:
		return "", &application.OpError{Op: "convert", Code: application.CodeInvalidArgument, Err: fmt.Errorf("target representation is required")}
	}
}

func representationToProto(value application.Representation) serializationv1.Representation {
	if value == application.RepresentationEditingJSON {
		return serializationv1.Representation_REPRESENTATION_EDITING_JSON
	}
	if value == application.RepresentationNative {
		return serializationv1.Representation_REPRESENTATION_NATIVE
	}
	return serializationv1.Representation_REPRESENTATION_UNSPECIFIED
}

func detectionMessage(value application.Detection) *serializationv1.DetectResponse {
	return &serializationv1.DetectResponse{
		FormatId: value.FormatID, Game: value.Game, FileType: value.FileType,
		Representation: representationToProto(value.Representation), StorageFormat: value.StorageFormat,
		Signature: value.Signature, Version: value.Version, Name: value.Name, Size: value.Size,
	}
}

func artifactMetadataMessage(value application.Artifact) *serializationv1.ArtifactMetadata {
	return &serializationv1.ArtifactMetadata{Name: value.Name, FormatId: value.FormatID, Representation: representationToProto(value.Representation), Size: value.Size, Sha256: value.SHA256}
}

func blobMetadataMessage(value blobstore.Metadata) *serializationv1.BlobMetadata {
	return &serializationv1.BlobMetadata{Id: value.ID, Name: value.Name, Size: value.Size, Sha256: value.SHA256, CreatedUnix: value.CreatedAt.Unix(), ExpiresUnix: value.ExpiresAt.Unix()}
}

func cleanName(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	name = filepath.Base(name)
	if name == "" || name == "." {
		return "blob.bin"
	}
	return name
}

func rpcError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.Unknown {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		return status.Error(codes.NotFound, err.Error())
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	if errors.Is(err, blobstore.ErrInvalidArgument) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, blobstore.ErrResourceExhausted) {
		return status.Error(codes.ResourceExhausted, err.Error())
	}
	var code codes.Code
	switch application.CodeOf(err) {
	case application.CodeInvalidArgument:
		code = codes.InvalidArgument
	case application.CodeNotFound:
		code = codes.NotFound
	case application.CodeUnsupported:
		code = codes.Unimplemented
	case application.CodeResourceExhausted:
		code = codes.ResourceExhausted
	case application.CodePermissionDenied:
		code = codes.PermissionDenied
	case application.CodeCanceled:
		code = codes.Canceled
	default:
		code = codes.Internal
	}
	return status.Error(code, err.Error())
}

func blobError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.Unknown {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return rpcError(err)
	}
	switch {
	case errors.Is(err, os.ErrNotExist):
		return status.Errorf(codes.NotFound, "%s: %v", operation, err)
	case errors.Is(err, blobstore.ErrInvalidArgument):
		return status.Errorf(codes.InvalidArgument, "%s: %v", operation, err)
	case errors.Is(err, blobstore.ErrResourceExhausted):
		return status.Errorf(codes.ResourceExhausted, "%s: %v", operation, err)
	default:
		return status.Errorf(codes.Internal, "%s: %v", operation, err)
	}
}
