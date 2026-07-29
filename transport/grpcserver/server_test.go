package grpcserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	serializationv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/api/gen/go/meido/serialization/v1"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/blobstore"
	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGRPCInlineConversionAndBlobStreaming(t *testing.T) {
	store, err := blobstore.New(blobstore.Config{MaxBlobBytes: 1 << 20, MaxTotalBytes: 2 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api, err := New(Config{Engine: application.NewEngine(application.EngineOptions{}), Blobs: store})
	if err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1 << 20)
	grpcServer := grpc.NewServer()
	api.Register(grpcServer)
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(ctx, "passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := serializationv1.NewSerializationServiceClient(connection)
	capabilities, err := client.GetCapabilities(ctx, &serializationv1.GetCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}
	menuCapability := grpcFormatCapability(capabilities, "com3d2.menu")
	if menuCapability == nil || !menuCapability.GetHasFormatGuide() || menuCapability.GetFormatGuideVerification() != "serialization_verified" {
		t.Fatalf("menu guide capability = %+v", menuCapability)
	}
	schema, err := client.GetFormatSchema(ctx, &serializationv1.GetFormatSchemaRequest{FormatId: "com3d2.menu"})
	if err != nil || !json.Valid(schema.GetSchemaJson()) {
		t.Fatalf("GetFormatSchema before conversion: response=%+v err=%v", schema, err)
	}
	guide, err := client.GetFormatGuide(ctx, &serializationv1.GetFormatGuideRequest{FormatId: "com3d2.menu"})
	if err != nil || !json.Valid(guide.GetGuideJson()) {
		t.Fatalf("GetFormatGuide before conversion: response=%+v err=%v", guide, err)
	}
	if guide.GetSchemaId() != schema.GetSchemaId() || guide.GetSha256() != menuCapability.GetFormatGuideSha256() || guide.GetSha256() != grpcSHA256(guide.GetGuideJson()) {
		t.Fatalf("schema/guide contract mismatch: schema=%+v guide=%+v capability=%+v", schema, guide, menuCapability)
	}
	native := grpcSyntheticMenu(t)
	input := &serializationv1.ArtifactInput{Name: "sample.menu", Location: &serializationv1.ArtifactInput_InlineData{InlineData: native}}

	detection, err := client.Detect(ctx, &serializationv1.DetectRequest{Input: input})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if detection.GetFormatId() != "com3d2.menu" || detection.GetRepresentation() != serializationv1.Representation_REPRESENTATION_NATIVE {
		t.Fatalf("detection = %+v", detection)
	}
	converted, err := client.Convert(ctx, &serializationv1.ConvertRequest{Input: input, Target: serializationv1.Representation_REPRESENTATION_EDITING_JSON})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if converted.GetResult().GetMetadata().GetName() != "sample.menu.json" || !json.Valid(converted.GetResult().GetInlineData()) {
		t.Fatalf("converted = %+v", converted)
	}

	upload, err := client.Upload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := upload.Send(&serializationv1.UploadRequest{Value: &serializationv1.UploadRequest_Metadata{Metadata: &serializationv1.UploadMetadata{Name: "sample.menu"}}}); err != nil {
		t.Fatal(err)
	}
	for start := 0; start < len(native); start += 7 {
		end := start + 7
		if end > len(native) {
			end = len(native)
		}
		if err := upload.Send(&serializationv1.UploadRequest{Value: &serializationv1.UploadRequest_Chunk{Chunk: native[start:end]}}); err != nil {
			t.Fatal(err)
		}
	}
	uploaded, err := upload.CloseAndRecv()
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if uploaded.GetBlob().GetSize() != int64(len(native)) {
		t.Fatalf("uploaded metadata = %+v", uploaded.GetBlob())
	}

	download, err := client.Download(ctx, &serializationv1.DownloadRequest{BlobId: uploaded.GetBlob().GetId()})
	if err != nil {
		t.Fatal(err)
	}
	first, err := download.Recv()
	if err != nil || first.GetMetadata().GetId() != uploaded.GetBlob().GetId() {
		t.Fatalf("download metadata = %+v, err=%v", first, err)
	}
	var downloaded bytes.Buffer
	for {
		message, err := download.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		downloaded.Write(message.GetChunk())
	}
	if !bytes.Equal(downloaded.Bytes(), native) {
		t.Fatal("downloaded blob differs from upload")
	}
	deleted, err := client.DeleteBlob(ctx, &serializationv1.DeleteBlobRequest{BlobId: uploaded.GetBlob().GetId()})
	if err != nil || !deleted.GetDeleted() {
		t.Fatalf("DeleteBlob = %+v, err=%v", deleted, err)
	}
	missingDownload, err := client.Download(ctx, &serializationv1.DownloadRequest{BlobId: uploaded.GetBlob().GetId()})
	if err != nil {
		t.Fatalf("start missing Download: %v", err)
	}
	if _, err := missingDownload.Recv(); status.Code(err) != codes.NotFound {
		t.Fatalf("missing Download status = %v", err)
	}
}

func TestGRPCCapabilitiesExposeBlobAndWritableRootLimits(t *testing.T) {
	readDirectory := t.TempDir()
	writeDirectory := t.TempDir()
	roots := application.NewRootSet()
	defer roots.Close()
	if err := roots.Add("read", readDirectory); err != nil {
		t.Fatal(err)
	}
	if err := roots.AddWritable("write", writeDirectory); err != nil {
		t.Fatal(err)
	}
	store, err := blobstore.New(blobstore.Config{MaxBlobBytes: 64, MaxTotalBytes: 128, MaxBlobs: 7, MaxNameBytes: 19})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api, err := New(Config{Engine: application.NewEngine(application.EngineOptions{
		MaxArchiveListingBytes: 1234, MaxArchiveEntries: 17,
	}), Roots: roots, Blobs: store})
	if err != nil {
		t.Fatal(err)
	}
	capabilities, err := api.GetCapabilities(context.Background(), &serializationv1.GetCapabilitiesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if capabilities.GetMaxInlineBytes() != DefaultMaxInlineBytes || capabilities.GetMaxBlobBytes() != 64 || capabilities.GetMaxTotalBlobBytes() != 128 {
		t.Fatalf("byte limits = %+v", capabilities)
	}
	if capabilities.GetMaxBlobCount() != 7 || capabilities.GetMaxBlobNameBytes() != 19 {
		t.Fatalf("object limits = %+v", capabilities)
	}
	if capabilities.GetMaxArchiveListingBytes() != 1234 || capabilities.GetMaxArchiveEntries() != 17 {
		t.Fatalf("archive listing limits = %+v", capabilities)
	}
	if got := capabilities.GetWritableRootIds(); len(got) != 1 || got[0] != "write" {
		t.Fatalf("writable roots = %v", got)
	}
	if _, err := New(Config{Engine: application.NewEngine(application.EngineOptions{}), Blobs: store, MaxInlineBytes: MaxInlineBytes + 1}); err == nil {
		t.Fatal("unsafe inline limit was accepted")
	}
}

func TestGRPCRawUnityAttachmentsRoundTripInlineAndBlob(t *testing.T) {
	store, err := blobstore.New(blobstore.Config{MaxBlobBytes: 1 << 20, MaxTotalBytes: 4 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api, err := New(Config{Engine: application.NewEngine(application.EngineOptions{}), Blobs: store})
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte{4, 0, 0, 0, 'h', 'a', 'i', 'r', 1, 2, 3}
	input := &serializationv1.ArtifactInput{
		Name:     "hair.mmesh.bytes",
		Location: &serializationv1.ArtifactInput_InlineData{InlineData: raw},
		Attachments: []*serializationv1.ArtifactAttachmentInput{
			{Suffix: ".meta.json", Location: &serializationv1.ArtifactAttachmentInput_InlineData{InlineData: []byte(`{"pathId":42,"loadName":"assets/hair"}`)}},
			{Suffix: ".typetree.json", Location: &serializationv1.ArtifactAttachmentInput_InlineData{InlineData: []byte(`{"format":"kces-unity-typetree","classId":43,"pathId":42,"value":{"typeName":"Mesh"}}`)}},
		},
	}
	editing, err := api.Convert(context.Background(), &serializationv1.ConvertRequest{
		Input: input, FormatId: "kces.bytes", Target: serializationv1.Representation_REPRESENTATION_EDITING_JSON,
	})
	if err != nil {
		t.Fatalf("raw to editing JSON: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(editing.GetResult().GetInlineData(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["pathId"] != float64(42) || envelope["loadName"] != "assets/hair" || envelope["typeTree"] == nil {
		t.Fatalf("editing envelope lost attachments: %#v", envelope)
	}

	jsonInput := &serializationv1.ArtifactInput{
		Name:     editing.GetResult().GetMetadata().GetName(),
		Location: &serializationv1.ArtifactInput_InlineData{InlineData: editing.GetResult().GetInlineData()},
	}
	for _, preferBlob := range []bool{false, true} {
		result, err := api.Convert(context.Background(), &serializationv1.ConvertRequest{
			Input: jsonInput, FormatId: "kces.bytes", Target: serializationv1.Representation_REPRESENTATION_NATIVE, PreferBlob: preferBlob,
		})
		if err != nil {
			t.Fatalf("editing JSON to raw (blob=%v): %v", preferBlob, err)
		}
		artifact := result.GetResult()
		if len(artifact.GetAttachments()) != 2 {
			t.Fatalf("raw result attachments (blob=%v) = %+v", preferBlob, artifact.GetAttachments())
		}
		if preferBlob {
			if artifact.GetBlob().GetId() == "" {
				t.Fatalf("raw primary was not returned as blob: %+v", artifact)
			}
			for _, attachment := range artifact.GetAttachments() {
				if attachment.GetBlob().GetId() == "" {
					t.Fatalf("attachment was not returned as blob: %+v", attachment)
				}
			}
		} else {
			if !bytes.Equal(artifact.GetInlineData(), raw) {
				t.Fatalf("raw inline payload changed: %x", artifact.GetInlineData())
			}
			for _, attachment := range artifact.GetAttachments() {
				if !json.Valid(attachment.GetInlineData()) {
					t.Fatalf("inline attachment is not JSON: %+v", attachment)
				}
			}
		}
	}
}

func TestGRPCInlineBudgetAppliesToWholeArtifactBundle(t *testing.T) {
	store, err := blobstore.New(blobstore.Config{MaxBlobBytes: 64, MaxTotalBytes: 128})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api, err := New(Config{Engine: application.NewEngine(application.EngineOptions{}), Blobs: store, MaxInlineBytes: 8})
	if err != nil {
		t.Fatal(err)
	}

	input := &serializationv1.ArtifactInput{
		Name:     "sample.bytes",
		Location: &serializationv1.ArtifactInput_InlineData{InlineData: []byte("12345")},
		Attachments: []*serializationv1.ArtifactAttachmentInput{{
			Suffix: ".meta.json", Location: &serializationv1.ArtifactAttachmentInput_InlineData{InlineData: []byte("6789")},
		}},
	}
	if _, err := api.resolveInput(context.Background(), input); application.CodeOf(err) != application.CodeResourceExhausted {
		t.Fatalf("aggregate inline input error = %v", err)
	}

	primary := []byte("12345")
	sidecar := []byte("6789")
	artifact := application.Artifact{
		Name: "sample.bytes", FormatID: "kces.bytes", Representation: application.RepresentationNative,
		Size: int64(len(primary)), SHA256: grpcSHA256(primary),
		Attachments: &application.ArtifactAttachmentSet{Files: []application.ArtifactAttachment{{
			Suffix: ".meta.json", Name: "sample.bytes.meta.json", Size: int64(len(sidecar)),
			SHA256: grpcSHA256(sidecar), Data: sidecar,
		}}},
	}
	result, err := api.captureResult(context.Background(), false, func(writer io.Writer) (application.Artifact, error) {
		_, writeErr := writer.Write(primary)
		return artifact, writeErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.GetInlineData(), primary) || len(result.GetAttachments()) != 1 {
		t.Fatalf("bundle result = %+v", result)
	}
	attachment := result.GetAttachments()[0]
	if attachment.GetBlob().GetId() == "" || len(attachment.GetInlineData()) != 0 {
		t.Fatalf("sidecar did not spill to blob: %+v", attachment)
	}
	file, _, err := store.Open(attachment.GetBlob().GetId())
	if err != nil {
		t.Fatal(err)
	}
	stored, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(stored, sidecar) {
		t.Fatalf("stored sidecar = %q, read error=%v, close error=%v", stored, readErr, closeErr)
	}
}

func TestGRPCFormatContractsAreDiscoverableBeforeConversion(t *testing.T) {
	store, err := blobstore.New(blobstore.Config{MaxBlobBytes: 1 << 20, MaxTotalBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api, err := New(Config{Engine: application.NewEngine(application.EngineOptions{}), Blobs: store})
	if err != nil {
		t.Fatal(err)
	}
	capabilities, err := api.GetCapabilities(context.Background(), &serializationv1.GetCapabilitiesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range capabilities.GetFormats() {
		if capability.GetHasEditingSchema() && (!capability.GetHasFormatGuide() || capability.GetFormatGuideVerification() != "serialization_verified") {
			t.Fatalf("editing format has no reviewed guide: %+v", capability)
		}
	}
	menuCapability := grpcFormatCapability(capabilities, "com3d2.menu")
	if menuCapability == nil || !menuCapability.GetHasEditingSchema() || menuCapability.GetEditingSchemaVersion() == "" || menuCapability.GetEditingSchemaSha256() == "" {
		t.Fatalf("menu schema capability = %+v", menuCapability)
	}
	response, err := api.GetFormatSchema(context.Background(), &serializationv1.GetFormatSchemaRequest{FormatId: "COM3D2.MENU"})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetFormatId() != "com3d2.menu" || response.GetRepresentation() != serializationv1.Representation_REPRESENTATION_EDITING_JSON || response.GetMediaType() != "application/schema+json" {
		t.Fatalf("schema metadata = %+v", response)
	}
	if !json.Valid(response.GetSchemaJson()) || response.GetSha256() != menuCapability.GetEditingSchemaSha256() || response.GetSha256() != grpcSHA256(response.GetSchemaJson()) {
		t.Fatalf("schema payload/digest = %q valid=%v", response.GetSha256(), json.Valid(response.GetSchemaJson()))
	}
	for _, test := range []struct {
		formatID     string
		verification string
	}{
		{formatID: "com3d2.menu", verification: "serialization_verified"},
		{formatID: "kces.dbconf", verification: "serialization_verified"},
		{formatID: "kces.dbcol", verification: "serialization_verified"},
		{formatID: "com3d2.mate", verification: "serialization_verified"},
		{formatID: "kces.nson", verification: "serialization_verified"},
	} {
		capability := grpcFormatCapability(capabilities, test.formatID)
		if capability == nil || !capability.GetHasEditingSchema() || !capability.GetHasFormatGuide() || capability.GetFormatGuideVersion() == "" || capability.GetFormatGuideId() == "" || capability.GetFormatGuideSha256() == "" || capability.GetFormatGuideVerification() != test.verification {
			t.Fatalf("format capability %s = %+v", test.formatID, capability)
		}
		schemaResponse, schemaErr := api.GetFormatSchema(context.Background(), &serializationv1.GetFormatSchemaRequest{FormatId: test.formatID})
		if schemaErr != nil {
			t.Fatalf("schema %s: %v", test.formatID, schemaErr)
		}
		guideResponse, guideErr := api.GetFormatGuide(context.Background(), &serializationv1.GetFormatGuideRequest{FormatId: test.formatID})
		if guideErr != nil {
			t.Fatalf("guide %s: %v", test.formatID, guideErr)
		}
		if guideResponse.GetFormatId() != test.formatID || guideResponse.GetMediaType() != "application/vnd.meido.format-guide+json" || guideResponse.GetFormatVerification() != test.verification || guideResponse.GetSchemaId() != schemaResponse.GetSchemaId() {
			t.Fatalf("guide metadata %s = %+v schema=%+v", test.formatID, guideResponse, schemaResponse)
		}
		if guideResponse.GetGuideVersion() != capability.GetFormatGuideVersion() || guideResponse.GetGuideId() != capability.GetFormatGuideId() || guideResponse.GetSha256() != capability.GetFormatGuideSha256() || guideResponse.GetSha256() != grpcSHA256(guideResponse.GetGuideJson()) || !json.Valid(guideResponse.GetGuideJson()) {
			t.Fatalf("guide payload/digest %s = %+v capability=%+v", test.formatID, guideResponse, capability)
		}
		if schemaResponse.GetSha256() != capability.GetEditingSchemaSha256() || schemaResponse.GetSha256() != grpcSHA256(schemaResponse.GetSchemaJson()) {
			t.Fatalf("schema payload/digest %s = %+v capability=%+v", test.formatID, schemaResponse, capability)
		}
		var guideHeader struct {
			SchemaID           string `json:"schema_id"`
			FormatVerification struct {
				Level string `json:"level"`
			} `json:"format_verification"`
		}
		if err := json.Unmarshal(guideResponse.GetGuideJson(), &guideHeader); err != nil || guideHeader.SchemaID != schemaResponse.GetSchemaId() || guideHeader.FormatVerification.Level != test.verification {
			t.Fatalf("guide JSON header %s = %+v err=%v", test.formatID, guideHeader, err)
		}
	}
	if _, err := api.GetFormatSchema(context.Background(), &serializationv1.GetFormatSchemaRequest{FormatId: "com3d2.arc"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("native-only schema status = %v", err)
	}
	if _, err := api.GetFormatGuide(context.Background(), &serializationv1.GetFormatGuideRequest{FormatId: "com3d2.arc"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("native-only guide status = %v", err)
	}
	if _, err := api.GetFormatGuide(context.Background(), &serializationv1.GetFormatGuideRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty guide format status = %v", err)
	}
}

func TestGRPCMalformedBlobIDIsInvalidArgument(t *testing.T) {
	store, err := blobstore.New(blobstore.Config{MaxBlobBytes: 8, MaxTotalBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api, err := New(Config{Engine: application.NewEngine(application.EngineOptions{}), Blobs: store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.DeleteBlob(context.Background(), &serializationv1.DeleteBlobRequest{BlobId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Delete malformed ID = %v", err)
	}
	if got := status.Code(rpcError(fmt.Errorf("%w: test", blobstore.ErrResourceExhausted))); got != codes.ResourceExhausted {
		t.Fatalf("rpcError resource code = %v", got)
	}
	if got := status.Code(rpcError(context.Canceled)); got != codes.Canceled {
		t.Fatalf("rpcError canceled code = %v", got)
	}
}

func TestGRPCListArchivePagination(t *testing.T) {
	store, err := blobstore.New(blobstore.Config{MaxBlobBytes: 1 << 20, MaxTotalBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api, err := New(Config{Engine: application.NewEngine(application.EngineOptions{}), Blobs: store})
	if err != nil {
		t.Fatal(err)
	}
	native := grpcContentTable(t, 5)
	input := &serializationv1.ArtifactInput{Name: "page.ct", Location: &serializationv1.ArtifactInput_InlineData{InlineData: native}}
	ctx := context.Background()
	token := ""
	var names []string
	for page := 0; page < 3; page++ {
		response, listErr := api.ListArchive(ctx, &serializationv1.ListArchiveRequest{Input: input, FormatId: "kces.ct", PageSize: 2, PageToken: token})
		if listErr != nil {
			t.Fatalf("page %d: %v", page, listErr)
		}
		for _, entry := range response.GetEntries() {
			names = append(names, entry.GetName())
		}
		token = response.GetNextPageToken()
	}
	if got, want := len(names), 5; got != want || token != "" {
		t.Fatalf("pagination names=%v next=%q", names, token)
	}
	if _, err := api.ListArchive(ctx, &serializationv1.ListArchiveRequest{Input: input, FormatId: "kces.ct", PageSize: -1}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("negative page size error = %v", err)
	}
	if _, err := api.ListArchive(ctx, &serializationv1.ListArchiveRequest{Input: input, FormatId: "kces.ct", PageToken: "999"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid page token error = %v", err)
	}
}

func grpcContentTable(t *testing.T, count int) []byte {
	t.Helper()
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize), Files: map[string]ct.VirtualFile{}}
	for i := 0; i < count; i++ {
		table.AddFile(fmt.Sprintf("entry-%02d.bin", i), []byte{byte(i)})
	}
	var output bytes.Buffer
	if err := ct.WriteContentTable(&output, table); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func grpcSyntheticMenu(t *testing.T) []byte {
	t.Helper()
	menu := &serializationCOM3D2.Menu{
		Signature: serializationCOM3D2.MenuSignature, Version: 1000,
		SrcFileName: "sample.menu", ItemName: "RPC", Category: "head", InfoText: "test",
		Commands: []serializationCOM3D2.Command{{Command: "name", Args: []string{"rpc"}}},
	}
	var output bytes.Buffer
	if err := menu.Dump(&output); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func grpcFormatCapability(capabilities *serializationv1.GetCapabilitiesResponse, formatID string) *serializationv1.FormatCapability {
	for _, format := range capabilities.GetFormats() {
		if format.GetId() == formatID {
			return format
		}
	}
	return nil
}

func grpcSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("%x", digest[:])
}
