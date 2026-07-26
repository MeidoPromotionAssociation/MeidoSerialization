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
	// APIVersion 是服务器公开的稳定 gRPC API 版本 / APIVersion is the stable gRPC API version exposed by the server
	APIVersion = "meido.serialization.v1"
	// DefaultMaxInlineBytes 是单个请求或响应默认允许内联的最大字节数 / DefaultMaxInlineBytes is the default maximum number of bytes inlined in one request or response
	DefaultMaxInlineBytes = 3 << 20
	// MaxInlineBytes 是 gRPC 消息安全允许的最大内联字节数 / MaxInlineBytes is the maximum inline byte count permitted by the gRPC message safety bound
	MaxInlineBytes = 3 << 20
	// DefaultChunkBytes 是 blob 下载使用的默认分块字节数 / DefaultChunkBytes is the default chunk size in bytes used for blob downloads
	DefaultChunkBytes = 256 << 10
	// MaxChunkBytes 是单个 blob 上传或下载分块允许的最大字节数 / MaxChunkBytes is the maximum number of bytes permitted in one blob upload or download chunk
	MaxChunkBytes = 1 << 20
	// DefaultArchivePageSize 是归档列表未指定页大小时返回的默认条目数 / DefaultArchivePageSize is the default number of archive entries returned when no page size is specified
	DefaultArchivePageSize = 128
	// MaxArchivePageSize 是单个归档列表页面允许请求的最大条目数 / MaxArchivePageSize is the maximum number of archive entries requested for one page
	MaxArchivePageSize = 1000
	// MaxArchivePageBytes 是单个序列化归档列表响应允许的最大字节数 / MaxArchivePageBytes is the maximum serialized byte size of one archive-list response
	MaxArchivePageBytes = 2 << 20
)

// Config 配置 gRPC 序列化服务器的依赖和传输资源限制 / Config configures dependencies and transport resource limits for a gRPC serialization server
type Config struct {
	// Engine 执行格式检测、转换、校验和归档操作 / Engine performs format detection, conversion, validation, and archive operations
	Engine *application.Engine
	// Roots 限制 RPC 文件引用能够访问的目录 / Roots confines directories accessible through RPC file references
	Roots *application.RootSet
	// Blobs 保存上传内容和无法安全内联的结果 / Blobs stores uploads and results that cannot be safely inlined
	Blobs *blobstore.Store
	// MaxInlineBytes 限制一个制品集合在消息中内联的合计字节数 / MaxInlineBytes limits aggregate bytes inlined for one artifact bundle
	MaxInlineBytes int64
	// ChunkBytes 指定 blob 下载消息的分块字节数 / ChunkBytes specifies the chunk size in bytes for blob download messages
	ChunkBytes int
}

// Server 将应用引擎和 blob 存储公开为版本化 gRPC 服务 / Server exposes the application engine and blob store as a versioned gRPC service
type Server struct {
	// UnimplementedSerializationServiceServer 保持新增 RPC 时的向前兼容性 / UnimplementedSerializationServiceServer preserves forward compatibility when RPCs are added
	serializationv1.UnimplementedSerializationServiceServer
	// engine 执行与文件格式有关的应用操作 / engine performs application operations related to file formats
	engine *application.Engine
	// roots 解析受限文件输入 / roots resolves confined file inputs
	roots *application.RootSet
	// blobs 管理上传和大型结果的临时对象 / blobs manages temporary objects for uploads and large results
	blobs *blobstore.Store
	// maxInlineBytes 限制主要制品与伴随文件的合计内联字节数 / maxInlineBytes limits aggregate inline bytes for a primary artifact and companions
	maxInlineBytes int64
	// chunkBytes 是 blob 下载使用的分块大小 / chunkBytes is the chunk size used for blob downloads
	chunkBytes int
	// archivePager 签名并验证服务器本地归档分页游标 / archivePager signs and verifies server-local archive page cursors
	archivePager *application.ArchivePager
}

// New 校验配置并创建版本化 gRPC 序列化服务器
// New validates configuration and creates a versioned gRPC serialization server
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

// Register 将序列化服务实现注册到 gRPC 服务注册器
// Register registers the serialization service implementation with a gRPC service registrar
func (s *Server) Register(registrar grpc.ServiceRegistrar) {
	serializationv1.RegisterSerializationServiceServer(registrar, s)
}

// GetCapabilities 返回服务器格式、根目录、blob 和资源限制能力
// GetCapabilities returns the server format, root, blob, and resource-limit capabilities
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

// GetFormatGuide 返回指定格式已发布的编辑指南文档
// GetFormatGuide returns the published editing guide document for a format
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

// GetFormatSchema 返回指定格式已发布的编辑 JSON 模式
// GetFormatSchema returns the published editing JSON schema for a format
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

// Detect 解析 RPC 制品输入并返回检测到的格式元数据
// Detect resolves an RPC artifact input and returns detected format metadata
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

// Convert 解析输入、执行格式转换并以内联数据或 blob 返回结果
// Convert resolves input, performs format conversion, and returns the result as inline data or a blob
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

// Validate 解析输入并完整校验指定或自动检测的格式
// Validate resolves input and fully validates the specified or automatically detected format
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

// Upload 接收元数据后的客户端流式分块并保存为临时 blob
// Upload receives client-streamed chunks after metadata and stores them as a temporary blob
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

// Download 先发送 blob 元数据再以受限大小分块流式发送内容
// Download sends blob metadata followed by content streamed in bounded chunks
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

// DeleteBlob 删除指定临时 blob 并报告其是否存在
// DeleteBlob removes a temporary blob and reports whether it existed
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

// ListArchive 返回受条目数和序列化消息大小双重限制的归档页面
// ListArchive returns an archive page bounded by both entry count and serialized message size
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

// archivePageSize 规范化客户端请求的归档页大小并应用服务器上限
// archivePageSize normalizes a requested archive page size and applies the server maximum
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

// ExtractArchiveEntry 提取指定归档条目并以内联数据或 blob 返回结果
// ExtractArchiveEntry extracts a named archive entry and returns it as inline data or a blob
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

// resolveInput 解析主要 RPC 输入及全部伴随文件并执行合计内联大小检查
// resolveInput resolves a primary RPC input and all companions while enforcing the aggregate inline-size limit
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

// resolveInputLocation 将内联数据、受限文件引用或 blob 引用转换为应用输入源
// resolveInputLocation converts inline data, a confined file reference, or a blob reference into an application source
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

// resolveInlineInput 校验具名内联内容并创建不可变内存输入源
// resolveInlineInput validates named inline content and creates an immutable in-memory source
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

// resolveFileInput 通过配置的受限根目录解析 RPC 文件引用
// resolveFileInput resolves an RPC file reference through the configured confined roots
func (s *Server) resolveFileInput(file *serializationv1.FileRef) (application.Source, error) {
	if file == nil {
		return nil, &application.OpError{Op: "resolve input", Code: application.CodeInvalidArgument, Err: fmt.Errorf("file reference is required")}
	}
	return s.roots.Resolve(file.GetRootId(), file.GetRelativePath())
}

// resolveBlobInput 校验 blob 引用并创建按需打开其内容的应用输入源
// resolveBlobInput validates a blob reference and creates an application source that opens its content on demand
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

// captureResult 暂存并校验生成制品后依据偏好和内联预算选择响应位置
// captureResult stages and verifies a produced artifact before selecting response locations from preference and inline budget
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

// captureResultAttachments 按剩余内联预算为每个伴随文件选择内联数据或 blob
// captureResultAttachments selects inline data or a blob for each companion file according to the remaining inline budget
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

// verifyArtifactOutput 重新计算暂存主要文件和伴随文件的大小与摘要以验证元数据
// verifyArtifactOutput recomputes staged primary and companion sizes and digests to verify artifact metadata
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

// blobSource 将 blob 存储对象适配为应用输入源 / blobSource adapts a blob-store object into an application source
type blobSource struct {
	// store 是拥有对象的临时 blob 存储 / store is the temporary blob store that owns the object
	store *blobstore.Store
	// id 是 blob 存储中的不透明对象标识符 / id is the opaque object identifier in the blob store
	id string
	// name 是物化 blob 时使用的安全文件名 / name is the safe filename used when materializing the blob
	name string
	// size 是 blob 元数据记录的精确字节数 / size is the exact byte size recorded in blob metadata
	size int64
}

// Name 返回 blob 输入源的安全文件名
// Name returns the safe filename of the blob input source
func (s *blobSource) Name() string { return s.name }

// Size 返回 blob 输入源元数据中的精确字节数
// Size returns the exact byte size from blob input metadata
func (s *blobSource) Size() int64 { return s.size }

// Open 在上下文有效时打开 blob 内容的独立读取器
// Open opens an independent reader for blob content while the context remains active
func (s *blobSource) Open(ctx context.Context) (application.ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, _, err := s.store.Open(s.id)
	return file, err
}

// uploadReader 将 gRPC 上传消息流适配为连续字节读取器 / uploadReader adapts a gRPC upload message stream into a continuous byte reader
type uploadReader struct {
	// stream 提供客户端发送的上传消息 / stream supplies upload messages sent by the client
	stream grpc.ClientStreamingServer[serializationv1.UploadRequest, serializationv1.UploadResponse]
	// chunk 保存尚未复制给调用方的当前消息内容 / chunk retains current message content not yet copied to the caller
	chunk []byte
}

// Read 从上传流接收受限分块并连续填充调用方缓冲区
// Read receives bounded chunks from the upload stream and continuously fills the caller buffer
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

// representationFromProto 将 protobuf 表示枚举转换为应用层表示
// representationFromProto converts a protobuf representation enum into an application representation
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

// representationToProto 将应用层表示转换为 protobuf 枚举
// representationToProto converts an application representation into a protobuf enum
func representationToProto(value application.Representation) serializationv1.Representation {
	if value == application.RepresentationEditingJSON {
		return serializationv1.Representation_REPRESENTATION_EDITING_JSON
	}
	if value == application.RepresentationNative {
		return serializationv1.Representation_REPRESENTATION_NATIVE
	}
	return serializationv1.Representation_REPRESENTATION_UNSPECIFIED
}

// detectionMessage 将应用检测结果转换为 gRPC 检测响应
// detectionMessage converts an application detection result into a gRPC detection response
func detectionMessage(value application.Detection) *serializationv1.DetectResponse {
	return &serializationv1.DetectResponse{
		FormatId: value.FormatID, Game: value.Game, FileType: value.FileType,
		Representation: representationToProto(value.Representation), StorageFormat: value.StorageFormat,
		Signature: value.Signature, Version: value.Version, Name: value.Name, Size: value.Size,
	}
}

// artifactMetadataMessage 将应用制品元数据转换为 protobuf 消息
// artifactMetadataMessage converts application artifact metadata into a protobuf message
func artifactMetadataMessage(value application.Artifact) *serializationv1.ArtifactMetadata {
	return &serializationv1.ArtifactMetadata{Name: value.Name, FormatId: value.FormatID, Representation: representationToProto(value.Representation), Size: value.Size, Sha256: value.SHA256}
}

// blobMetadataMessage 将 blob 存储元数据转换为 protobuf 消息
// blobMetadataMessage converts blob-store metadata into a protobuf message
func blobMetadataMessage(value blobstore.Metadata) *serializationv1.BlobMetadata {
	return &serializationv1.BlobMetadata{Id: value.ID, Name: value.Name, Size: value.Size, Sha256: value.SHA256, CreatedUnix: value.CreatedAt.Unix(), ExpiresUnix: value.ExpiresAt.Unix()}
}

// cleanName 将不可信名称限制为安全的 blob 基本文件名
// cleanName confines an untrusted name to a safe blob base filename
func cleanName(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	name = filepath.Base(name)
	if name == "" || name == "." {
		return "blob.bin"
	}
	return name
}

// rpcError 将应用、上下文和 blob 错误转换为对应 gRPC 状态
// rpcError converts application, context, and blob errors into corresponding gRPC statuses
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

// blobError 为 blob 操作添加上下文并映射为对应 gRPC 状态
// blobError adds blob operation context and maps an error to the corresponding gRPC status
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
