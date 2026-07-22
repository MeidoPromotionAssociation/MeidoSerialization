package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/conversionio"
	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
)

const (
	DefaultMaxInputBytes          int64 = 10 << 30 // 10 Gib
	DefaultMaxOutputBytes         int64 = 10 << 30 // 10 Gib
	DefaultMaxArchiveListingBytes int64 = 10 << 30 // 10 Gib
	DefaultMaxArchiveEntries            = 100_000
)

type EngineOptions struct {
	Registry               *Registry
	MaxInputBytes          int64
	MaxOutputBytes         int64
	MaxArchiveListingBytes int64
	MaxArchiveEntries      int
}

type Engine struct {
	registry               *Registry
	maxInputBytes          int64
	maxOutputBytes         int64
	maxArchiveListingBytes int64
	maxArchiveEntries      int
}

func NewEngine(options EngineOptions) *Engine {
	if options.Registry == nil {
		options.Registry = DefaultRegistry()
	}
	if options.MaxInputBytes <= 0 {
		options.MaxInputBytes = DefaultMaxInputBytes
	}
	if options.MaxOutputBytes <= 0 {
		options.MaxOutputBytes = DefaultMaxOutputBytes
	}
	if options.MaxArchiveListingBytes <= 0 {
		options.MaxArchiveListingBytes = DefaultMaxArchiveListingBytes
	}
	if options.MaxArchiveEntries <= 0 {
		options.MaxArchiveEntries = DefaultMaxArchiveEntries
	}
	return &Engine{
		registry: options.Registry, maxInputBytes: options.MaxInputBytes, maxOutputBytes: options.MaxOutputBytes,
		maxArchiveListingBytes: options.MaxArchiveListingBytes, maxArchiveEntries: options.MaxArchiveEntries,
	}
}

func (e *Engine) Formats() []Format { return e.registry.Formats() }

// ArchiveListingLimits returns the independent input-byte and entry-count
// budgets applied before an archive listing is returned.
func (e *Engine) ArchiveListingLimits() (int64, int) {
	return e.maxArchiveListingBytes, e.maxArchiveEntries
}

type Detection struct {
	FormatID       string
	Game           string
	FileType       string
	Representation Representation
	StorageFormat  string
	Signature      string
	Version        int32
	Name           string
	Size           int64
}

type Artifact struct {
	Name           string
	FormatID       string
	Representation Representation
	Size           int64
	SHA256         string
	Attachments    *ArtifactAttachmentSet
}

// ArtifactAttachment is a sidecar emitted together with the primary artifact.
// Data is retained because path-based converters create sidecars inside the
// engine's short-lived workspace.
type ArtifactAttachment struct {
	Suffix string
	Name   string
	Size   int64
	SHA256 string
	Data   []byte
}

type ArtifactAttachmentSet struct {
	Files []ArtifactAttachment
}

// AttachmentFiles returns a deep copy of the artifact's sidecar files.
func (a Artifact) AttachmentFiles() []ArtifactAttachment {
	if a.Attachments == nil {
		return nil
	}
	result := make([]ArtifactAttachment, len(a.Attachments.Files))
	for i, attachment := range a.Attachments.Files {
		result[i] = attachment
		result[i].Data = append([]byte(nil), attachment.Data...)
	}
	return result
}

// TotalSize returns the combined size of the primary file and all sidecars.
func (a Artifact) TotalSize() int64 {
	total := a.Size
	for _, attachment := range a.AttachmentFiles() {
		if attachment.Size > 0 && total <= math.MaxInt64-attachment.Size {
			total += attachment.Size
		}
	}
	return total
}

type ConvertRequest struct {
	Source   Source
	FormatID string
	To       Representation
}

func (e *Engine) Detect(ctx context.Context, source Source) (Detection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if source == nil {
		return Detection{}, opError("detect", CodeInvalidArgument, fmt.Errorf("source is required"))
	}
	workspace, path, err := e.materialize(ctx, source, source.Name())
	if err != nil {
		return Detection{}, err
	}
	defer os.RemoveAll(workspace)
	return e.detectPath(ctx, path)
}

func (e *Engine) detectPath(ctx context.Context, path string) (Detection, error) {
	if err := ctx.Err(); err != nil {
		return Detection{}, opError("detect", CodeCanceled, err)
	}
	legacy, matched, err := (&COM3D2Service.CommonService{}).TryFileTypeDetermine(path)
	if err != nil {
		return Detection{}, opError("detect COM3D2", CodeInvalidArgument, err)
	}
	if matched {
		return detectionFromFileInfo(legacy), nil
	}
	info, matched, err := (&KCESService.FileTypeService{}).TryFileTypeDetermine(path)
	if err != nil {
		return Detection{}, opError("detect KCES", CodeInvalidArgument, err)
	}
	if matched {
		return detectionFromFileInfo(info), nil
	}
	if dance, matched := detectDanceFile(path); matched {
		return dance, nil
	}
	if strings.EqualFold(filepath.Ext(path), ".arc") {
		arcFile, closer, arcErr := (&COM3D2Service.ArcService{}).ReadArcLazy(path)
		if arcErr != nil {
			return Detection{}, opError("detect COM3D2 ARC", CodeInvalidArgument, arcErr)
		}
		_ = arcFile
		_ = closer.Close()
		stat, _ := os.Stat(path)
		return Detection{FormatID: "com3d2.arc", Game: "COM3D2", FileType: "arc", Representation: RepresentationNative, StorageFormat: "binary", Name: filepath.Base(path), Size: stat.Size()}, nil
	}
	return Detection{}, opError("detect", CodeUnsupported, fmt.Errorf("file format is not recognized"))
}

func detectDanceFile(path string) (Detection, bool) {
	name := filepath.Base(path)
	lowerName := strings.ToLower(name)
	representation := RepresentationNative
	danceType := COM3D2Service.DanceBytesUnknown
	if strings.HasSuffix(lowerName, ".bytes.json") {
		representation = RepresentationEditingJSON
		data, err := os.ReadFile(path)
		if err != nil || !json.Valid(data) {
			return Detection{}, false
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
			return Detection{}, false
		}
		_, hasTotalFrame := fields["TotalFrame"]
		_, hasFrameRate := fields["FrameRate"]
		_, hasTracks := fields["Tracks"]
		_, hasEntries := fields["Entries"]
		hasTimeline := hasTotalFrame && hasFrameRate && hasTracks
		switch {
		case hasTimeline && (!hasEntries || strings.Contains(lowerName, "timeline")):
			var value serializationCOM3D2.TimelineData
			if err := json.Unmarshal(data, &value); err != nil {
				return Detection{}, false
			}
			danceType = COM3D2Service.DanceBytesTimeline
		case hasEntries:
			var value serializationCOM3D2.DanceObjectData
			if err := json.Unmarshal(data, &value); err != nil {
				return Detection{}, false
			}
			danceType = COM3D2Service.DanceBytesObjectData
		default:
			return Detection{}, false
		}
	} else if strings.HasSuffix(lowerName, ".bytes") {
		var err error
		danceType, err = (&COM3D2Service.DanceService{}).SniffDanceBytesType(path)
		if err != nil {
			return Detection{}, false
		}
	} else {
		return Detection{}, false
	}

	fileType := string(danceType)
	signature := ""
	if danceType == COM3D2Service.DanceBytesTimeline {
		signature = serializationCOM3D2.TimelineSignature
	}
	stat, err := os.Stat(path)
	if err != nil {
		return Detection{}, false
	}
	return Detection{
		FormatID: "com3d2." + fileType, Game: "COM3D2", FileType: fileType,
		Representation: representation, StorageFormat: map[bool]string{true: "json", false: "binary"}[representation == RepresentationEditingJSON],
		Signature: signature, Name: name, Size: stat.Size(),
	}, true
}

func detectionFromFileInfo(info COM3D2Service.FileInfo) Detection {
	representation := RepresentationNative
	if strings.HasSuffix(strings.ToLower(info.Path), ".json") {
		representation = RepresentationEditingJSON
	}
	return Detection{
		FormatID:       strings.ToLower(info.Game) + "." + strings.ToLower(info.FileType),
		Game:           info.Game,
		FileType:       info.FileType,
		Representation: representation,
		StorageFormat:  info.StorageFormat,
		Signature:      info.Signature,
		Version:        info.Version,
		Name:           filepath.Base(info.Path),
		Size:           info.Size,
	}
}

func (e *Engine) Convert(ctx context.Context, request ConvertRequest, output io.Writer) (Artifact, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if request.Source == nil || output == nil {
		return Artifact{}, opError("convert", CodeInvalidArgument, fmt.Errorf("source and output are required"))
	}
	if request.To != RepresentationNative && request.To != RepresentationEditingJSON {
		return Artifact{}, opError("convert", CodeInvalidArgument, fmt.Errorf("invalid target representation %q", request.To))
	}

	workspace, originalPath, err := e.materialize(ctx, request.Source, request.Source.Name())
	if err != nil {
		return Artifact{}, err
	}
	defer os.RemoveAll(workspace)

	formatID := strings.ToLower(strings.TrimSpace(request.FormatID))
	if formatID == "" {
		detection, detectErr := e.detectPath(ctx, originalPath)
		if detectErr != nil {
			return Artifact{}, detectErr
		}
		formatID = detection.FormatID
	}
	format, ok := e.registry.Lookup(formatID)
	if !ok {
		return Artifact{}, opError("convert", CodeUnsupported, fmt.Errorf("format %q is not registered", formatID))
	}
	if !format.Capability.Convert {
		return Artifact{}, opError("convert", CodeUnsupported, fmt.Errorf("format %q does not support native/editing JSON conversion", formatID))
	}
	if request.To == RepresentationNative {
		if err := e.validateEditingJSONPath(ctx, originalPath, format.ID); err != nil {
			return Artifact{}, err
		}
	}

	inputName := formatInputName(format, request.Source.Name(), request.To)
	inputPath := filepath.Join(workspace, inputName)
	if !samePath(inputPath, originalPath) {
		if err := renameMaterializedArtifact(originalPath, inputPath, request.Source); err != nil {
			return Artifact{}, opError("prepare conversion", CodeInternal, err)
		}
	}
	outputName := formatOutputName(format, inputName, request.To)
	outputPath := filepath.Join(workspace, outputName)
	if err := ctx.Err(); err != nil {
		return Artifact{}, opError("convert", CodeCanceled, err)
	}
	if err := format.convert.run(ctx, request.To, inputPath, outputPath, e.maxOutputBytes); err != nil {
		return Artifact{}, opError("convert "+formatID, pathConversionErrorCode(err), err)
	}
	if err := ctx.Err(); err != nil {
		return Artifact{}, opError("convert", CodeCanceled, err)
	}
	return e.copyFileArtifact(ctx, outputPath, outputName, formatID, request.To, output)
}

func (e *Engine) ConvertBytes(ctx context.Context, request ConvertRequest) (Artifact, []byte, error) {
	var output bytes.Buffer
	artifact, err := e.Convert(ctx, request, &output)
	if err != nil {
		return Artifact{}, nil, err
	}
	return artifact, output.Bytes(), nil
}

func (e *Engine) Validate(ctx context.Context, source Source, formatID string) (Detection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if source == nil {
		return Detection{}, opError("validate", CodeInvalidArgument, fmt.Errorf("source is required"))
	}
	workspace, path, err := e.materialize(ctx, source, source.Name())
	if err != nil {
		return Detection{}, err
	}
	defer os.RemoveAll(workspace)

	var detection Detection
	if strings.TrimSpace(formatID) == "" {
		detection, err = e.detectPath(ctx, path)
		if err != nil {
			return Detection{}, err
		}
		formatID = detection.FormatID
	} else {
		format, ok := e.registry.Lookup(formatID)
		if !ok {
			return Detection{}, opError("validate", CodeUnsupported, fmt.Errorf("format %q is not registered", formatID))
		}
		detection = Detection{FormatID: format.ID, Game: format.Game, FileType: format.FileType, Name: source.Name(), Size: source.Size()}
		if strings.HasSuffix(strings.ToLower(source.Name()), ".json") {
			detection.Representation = RepresentationEditingJSON
		} else {
			detection.Representation = RepresentationNative
		}
	}

	format, ok := e.registry.Lookup(formatID)
	if !ok {
		return Detection{}, opError("validate", CodeUnsupported, fmt.Errorf("format %q is not registered", formatID))
	}
	if !format.Capability.Validate {
		return Detection{}, opError("validate", CodeUnsupported, fmt.Errorf("format %q does not provide full validation", format.ID))
	}
	if format.Capability.Archive {
		if detection.Representation != RepresentationEditingJSON {
			_, err := e.listArchivePath(ctx, format.ID, path)
			if err != nil {
				return Detection{}, err
			}
			return detection, nil
		}
		if !format.Capability.Convert {
			return Detection{}, opError("validate", CodeUnsupported, fmt.Errorf("format %q has no editing JSON representation", format.ID))
		}
	}
	if !format.Capability.Convert {
		return detection, nil
	}
	if detection.Representation == RepresentationEditingJSON {
		if err := e.validateEditingJSONPath(ctx, path, format.ID); err != nil {
			return Detection{}, err
		}
	}
	target := RepresentationEditingJSON
	if detection.Representation == RepresentationEditingJSON {
		target = RepresentationNative
	}
	inputName := formatInputName(format, source.Name(), target)
	inputPath := filepath.Join(workspace, inputName)
	if !samePath(path, inputPath) {
		if err := renameMaterializedArtifact(path, inputPath, source); err != nil {
			return Detection{}, opError("prepare validation", CodeInternal, err)
		}
	}
	outputPath := filepath.Join(workspace, formatOutputName(format, inputName, target))
	if err := format.convert.run(ctx, target, inputPath, outputPath, e.maxOutputBytes); err != nil {
		return Detection{}, opError("validate "+format.ID, pathConversionErrorCode(err), err)
	}
	return detection, nil
}

func pathConversionErrorCode(err error) ErrorCode {
	if errors.Is(err, conversionio.ErrOutputLimitExceeded) {
		return CodeResourceExhausted
	}
	code := CodeOf(err)
	if code == CodeInternal {
		return CodeInvalidArgument
	}
	return code
}

func (e *Engine) materialize(ctx context.Context, source Source, name string) (string, string, error) {
	return e.materializeWithLimit(ctx, source, name, e.maxInputBytes)
}

func (e *Engine) materializeWithLimit(ctx context.Context, source Source, name string, limit int64) (string, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		return "", "", opError("read source", CodeResourceExhausted, fmt.Errorf("positive input limit is required"))
	}
	attachments := sourceAttachments(source)
	totalSize := source.Size()
	if totalSize < 0 || totalSize > limit {
		return "", "", opError("read source", CodeResourceExhausted, fmt.Errorf("source size %d exceeds limit %d", source.Size(), limit))
	}
	for _, attachment := range attachments {
		size := attachment.Source.Size()
		if size < 0 || size > limit-totalSize {
			return "", "", opError("read source", CodeResourceExhausted, fmt.Errorf("artifact with %s attachment exceeds limit %d", attachment.Suffix, limit))
		}
		totalSize += size
	}
	dir, err := os.MkdirTemp("", "meido-serialization-")
	if err != nil {
		return "", "", opError("create workspace", CodeInternal, err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(dir)
		}
	}()
	path := filepath.Join(dir, cleanSourceName(name))
	written, err := materializeSourceFile(ctx, source, path, limit)
	if err != nil {
		return "", "", err
	}
	totalWritten := written
	for _, attachment := range attachments {
		attachmentWritten, attachmentErr := materializeSourceFile(ctx, attachment.Source, path+attachment.Suffix, limit-totalWritten)
		if attachmentErr != nil {
			return "", "", attachmentErr
		}
		totalWritten += attachmentWritten
	}
	cleanup = false
	return dir, path, nil
}

func materializeSourceFile(ctx context.Context, source Source, path string, limit int64) (int64, error) {
	input, err := source.Open(ctx)
	if err != nil {
		return 0, opError("open source", CodeNotFound, err)
	}
	defer input.Close()
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 0, opError("materialize source", CodeInternal, err)
	}
	written, copyErr := io.Copy(output, io.LimitReader(&contextReader{ctx: ctx, reader: input}, limitWithSentinel(limit)))
	closeErr := output.Close()
	if copyErr != nil {
		return 0, opError("materialize source", CodeInternal, copyErr)
	}
	if closeErr != nil {
		return 0, opError("materialize source", CodeInternal, closeErr)
	}
	if written > limit {
		return 0, opError("materialize source", CodeResourceExhausted, fmt.Errorf("artifact exceeds input limit"))
	}
	if source.Size() >= 0 && written != source.Size() {
		return 0, opError("materialize source", CodeInvalidArgument, fmt.Errorf("source size changed: expected %d, read %d", source.Size(), written))
	}
	return written, nil
}

func renameMaterializedArtifact(oldPath, newPath string, source Source) error {
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	for _, attachment := range sourceAttachments(source) {
		if err := os.Rename(oldPath+attachment.Suffix, newPath+attachment.Suffix); err != nil {
			return fmt.Errorf("rename %s attachment: %w", attachment.Suffix, err)
		}
	}
	return nil
}

func (e *Engine) copyFileArtifact(ctx context.Context, path, name, formatID string, representation Representation, output io.Writer) (Artifact, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Artifact{}, opError("read conversion output", CodeInternal, err)
	}
	if !info.Mode().IsRegular() || info.Size() > e.maxOutputBytes {
		return Artifact{}, opError("read conversion output", CodeResourceExhausted, fmt.Errorf("output size %d exceeds limit %d", info.Size(), e.maxOutputBytes))
	}
	attachments, err := readArtifactAttachments(ctx, path, name, e.maxOutputBytes-info.Size())
	if err != nil {
		return Artifact{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return Artifact{}, opError("read conversion output", CodeInternal, err)
	}
	defer f.Close()
	hash := sha256.New()
	writer := &conversionio.LimitWriter{Context: ctx, Writer: io.MultiWriter(output, hash), Remaining: info.Size()}
	written, err := io.Copy(writer, &contextReader{ctx: ctx, reader: f})
	if err != nil {
		if errors.Is(err, conversionio.ErrOutputLimitExceeded) {
			return Artifact{}, opError("stream conversion output", CodeResourceExhausted, fmt.Errorf("output changed while streaming and exceeded its declared size"))
		}
		return Artifact{}, opError("stream conversion output", CodeInternal, err)
	}
	if written != info.Size() {
		return Artifact{}, opError("stream conversion output", CodeInvalidArgument, fmt.Errorf("output size changed: expected %d, read %d", info.Size(), written))
	}
	artifact := Artifact{
		Name: name, FormatID: formatID, Representation: representation,
		Size: written, SHA256: hex.EncodeToString(hash.Sum(nil)),
	}
	if len(attachments) != 0 {
		artifact.Attachments = &ArtifactAttachmentSet{Files: attachments}
	}
	return artifact, nil
}

func readArtifactAttachments(ctx context.Context, path, name string, remaining int64) ([]ArtifactAttachment, error) {
	var result []ArtifactAttachment
	for _, suffix := range artifactAttachmentSuffixes {
		attachmentPath := path + suffix
		info, err := os.Stat(attachmentPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, opError("read conversion attachment", CodeInternal, err)
		}
		if !info.Mode().IsRegular() || info.Size() > remaining {
			return nil, opError("read conversion attachment", CodeResourceExhausted, fmt.Errorf("artifact output exceeds limit"))
		}
		file, err := os.Open(attachmentPath)
		if err != nil {
			return nil, opError("read conversion attachment", CodeInternal, err)
		}
		var data bytes.Buffer
		hash := sha256.New()
		writer := &conversionio.LimitWriter{
			Context: ctx, Writer: io.MultiWriter(&data, hash), Remaining: info.Size(),
		}
		written, copyErr := io.Copy(writer, &contextReader{ctx: ctx, reader: file})
		closeErr := file.Close()
		if copyErr != nil {
			if errors.Is(copyErr, conversionio.ErrOutputLimitExceeded) {
				return nil, opError("read conversion attachment", CodeResourceExhausted, fmt.Errorf("attachment changed while reading and exceeded its declared size"))
			}
			return nil, opError("read conversion attachment", CodeInternal, copyErr)
		}
		if closeErr != nil {
			return nil, opError("read conversion attachment", CodeInternal, closeErr)
		}
		if written != info.Size() {
			return nil, opError("read conversion attachment", CodeInvalidArgument, fmt.Errorf("attachment size changed: expected %d, read %d", info.Size(), written))
		}
		remaining -= written
		result = append(result, ArtifactAttachment{
			Suffix: suffix, Name: name + suffix, Size: written,
			SHA256: hex.EncodeToString(hash.Sum(nil)), Data: data.Bytes(),
		})
	}
	return result, nil
}

func inspectArtifactFiles(ctx context.Context, path string, limit int64) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, opError("inspect conversion output", CodeCanceled, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, opError("inspect conversion output", CodeInternal, err)
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return 0, opError("inspect conversion output", CodeResourceExhausted, fmt.Errorf("output size %d exceeds limit %d", info.Size(), limit))
	}
	total := info.Size()
	for _, suffix := range artifactAttachmentSuffixes {
		if err := ctx.Err(); err != nil {
			return 0, opError("inspect conversion output", CodeCanceled, err)
		}
		attachment, attachmentErr := os.Stat(path + suffix)
		if os.IsNotExist(attachmentErr) {
			continue
		}
		if attachmentErr != nil {
			return 0, opError("inspect conversion output", CodeInternal, attachmentErr)
		}
		if !attachment.Mode().IsRegular() || attachment.Size() > limit-total {
			return 0, opError("inspect conversion output", CodeResourceExhausted, fmt.Errorf("artifact output exceeds limit %d", limit))
		}
		total += attachment.Size()
	}
	return total, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func formatInputName(format Format, original string, target Representation) string {
	original = cleanSourceName(original)
	if target == RepresentationNative {
		if strings.HasSuffix(strings.ToLower(original), ".json") {
			return original
		}
		return format.DefaultName + ".json"
	}
	lower := strings.ToLower(original)
	for _, suffix := range format.NativeSuffixes {
		if strings.HasSuffix(lower, strings.ToLower(suffix)) {
			return original
		}
	}
	return format.DefaultName
}

func formatOutputName(format Format, inputName string, target Representation) string {
	if target == RepresentationEditingJSON {
		return trimJSONSuffix(inputName) + ".json"
	}
	return trimJSONSuffix(inputName)
}

func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }

func trimJSONSuffix(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".json") {
		return name[:len(name)-len(".json")]
	}
	return name
}
