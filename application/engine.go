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
	// DefaultMaxInputBytes 是单次操作允许读取的默认最大输入字节数 / DefaultMaxInputBytes is the default maximum number of input bytes read by one operation
	DefaultMaxInputBytes int64 = 10 << 30 // 10 Gib
	// DefaultMaxOutputBytes 是单次操作允许生成的默认最大输出字节数 / DefaultMaxOutputBytes is the default maximum number of output bytes produced by one operation
	DefaultMaxOutputBytes int64 = 10 << 30 // 10 Gib
	// DefaultMaxArchiveListingBytes 是列出归档前允许物化的默认最大输入字节数 / DefaultMaxArchiveListingBytes is the default maximum number of input bytes materialized before listing an archive
	DefaultMaxArchiveListingBytes int64 = 10 << 30 // 10 Gib
	// DefaultMaxArchiveEntries 是单个归档列表允许返回的默认最大条目数 / DefaultMaxArchiveEntries is the default maximum number of entries returned by one archive listing
	DefaultMaxArchiveEntries = 100_000
)

// EngineOptions 配置应用引擎使用的注册表和资源限制 / EngineOptions configures the registry and resource limits used by an application engine
type EngineOptions struct {
	// Registry 是引擎用于查找格式的注册表，空值使用默认注册表 / Registry is the format registry used by the engine with nil selecting the default registry
	Registry *Registry
	// MaxInputBytes 是单次操作允许读取的最大输入字节数 / MaxInputBytes is the maximum number of input bytes read by one operation
	MaxInputBytes int64
	// MaxOutputBytes 是单次操作允许生成的最大输出字节数 / MaxOutputBytes is the maximum number of output bytes produced by one operation
	MaxOutputBytes int64
	// MaxArchiveListingBytes 是列出归档前允许物化的最大输入字节数 / MaxArchiveListingBytes is the maximum number of input bytes materialized before listing an archive
	MaxArchiveListingBytes int64
	// MaxArchiveEntries 是单个归档列表允许返回的最大条目数 / MaxArchiveEntries is the maximum number of entries returned by one archive listing
	MaxArchiveEntries int
}

// Engine 协调格式检测、转换、校验和归档操作 / Engine coordinates format detection, conversion, validation, and archive operations
type Engine struct {
	// registry 保存引擎可用的格式定义 / registry stores the format definitions available to the engine
	registry *Registry
	// maxInputBytes 限制单次操作读取的输入字节数 / maxInputBytes limits input bytes read by one operation
	maxInputBytes int64
	// maxOutputBytes 限制单次操作生成的输出字节数 / maxOutputBytes limits output bytes produced by one operation
	maxOutputBytes int64
	// maxArchiveListingBytes 独立限制归档列表操作物化的输入字节数 / maxArchiveListingBytes independently limits input bytes materialized for archive listings
	maxArchiveListingBytes int64
	// maxArchiveEntries 限制单个归档列表返回的条目数 / maxArchiveEntries limits entries returned by one archive listing
	maxArchiveEntries int
}

// NewEngine 使用提供的选项创建应用引擎并为无效限制填充默认值
// NewEngine creates an application engine and fills invalid limits with defaults
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

// Formats 返回引擎注册表中的公开格式元数据
// Formats returns public format metadata from the engine registry
func (e *Engine) Formats() []Format { return e.registry.Formats() }

// ArchiveListingLimits 返回归档列表独立使用的输入字节数和条目数限制
// ArchiveListingLimits returns the independent input-byte and entry-count limits used for archive listings
func (e *Engine) ArchiveListingLimits() (int64, int) {
	return e.maxArchiveListingBytes, e.maxArchiveEntries
}

// Detection 描述文件格式检测得到的规范元数据 / Detection describes canonical metadata produced by file format detection
type Detection struct {
	// FormatID 是注册表使用的稳定格式标识符 / FormatID is the stable format identifier used by the registry
	FormatID string
	// Game 是检测到的游戏或工具名称 / Game is the detected game or tool name
	Game string
	// FileType 是检测到的规范文件类型 / FileType is the detected canonical file type
	FileType string
	// Representation 表示文件是原生格式还是编辑 JSON / Representation indicates whether the file is native data or editing JSON
	Representation Representation
	// StorageFormat 描述底层存储编码方式 / StorageFormat describes the underlying storage encoding
	StorageFormat string
	// Signature 是格式携带的文件签名 / Signature is the file signature carried by the format
	Signature string
	// Version 是检测到的精确格式版本 / Version is the exact detected format version
	Version int32
	// Name 是不包含目录的源文件名 / Name is the source filename without directory components
	Name string
	// Size 是源文件的字节数 / Size is the source file size in bytes
	Size int64
}

// Artifact 描述转换或提取操作产生的主要制品 / Artifact describes the primary artifact produced by conversion or extraction
type Artifact struct {
	// Name 是建议用于保存主要制品的文件名 / Name is the suggested filename for the primary artifact
	Name string
	// FormatID 是制品对应的稳定格式标识符 / FormatID is the stable format identifier associated with the artifact
	FormatID string
	// Representation 表示制品是原生格式还是编辑 JSON / Representation indicates whether the artifact is native data or editing JSON
	Representation Representation
	// Size 是主要制品的字节数 / Size is the primary artifact size in bytes
	Size int64
	// SHA256 是主要制品内容的 SHA-256 摘要 / SHA256 is the SHA-256 digest of the primary artifact content
	SHA256 string
	// Attachments 保存与主要制品共同生成的可选伴随文件 / Attachments contains optional companion files emitted with the primary artifact
	Attachments *ArtifactAttachmentSet
}

// ArtifactAttachment 描述与主要制品共同生成的单个伴随文件 / ArtifactAttachment describes one companion file emitted with the primary artifact
type ArtifactAttachment struct {
	// Suffix 是追加到主要制品名称后的受管理后缀 / Suffix is the managed suffix appended to the primary artifact name
	Suffix string
	// Name 是伴随文件的完整建议文件名 / Name is the complete suggested filename for the companion file
	Name string
	// Size 是伴随文件的字节数 / Size is the companion file size in bytes
	Size int64
	// SHA256 是伴随文件内容的 SHA-256 摘要 / SHA256 is the SHA-256 digest of the companion file content
	SHA256 string
	// Data 保存在短生命周期工作区中生成的伴随文件内容 / Data retains companion content produced inside the short-lived workspace
	Data []byte
}

// ArtifactAttachmentSet 保存主要制品的全部伴随文件 / ArtifactAttachmentSet contains all companion files for a primary artifact
type ArtifactAttachmentSet struct {
	// Files 是按受管理后缀顺序排列的伴随文件 / Files contains companion files in managed-suffix order
	Files []ArtifactAttachment
}

// AttachmentFiles 返回制品伴随文件的深拷贝
// AttachmentFiles returns a deep copy of the artifact companion files
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

// TotalSize 返回主要文件与全部伴随文件的合计字节数
// TotalSize returns the combined size in bytes of the primary file and all companion files
func (a Artifact) TotalSize() int64 {
	total := a.Size
	for _, attachment := range a.AttachmentFiles() {
		if attachment.Size > 0 && total <= math.MaxInt64-attachment.Size {
			total += attachment.Size
		}
	}
	return total
}

// ConvertRequest 描述一次原生格式与编辑 JSON 之间的转换请求 / ConvertRequest describes one conversion request between native data and editing JSON
type ConvertRequest struct {
	// Source 是需要转换的输入制品 / Source is the input artifact to convert
	Source Source
	// FormatID 是可选的显式格式标识符，空值触发自动检测 / FormatID is an optional explicit format identifier with an empty value requesting detection
	FormatID string
	// To 是转换目标表示形式 / To is the target representation of the conversion
	To Representation
}

// Detect 物化抽象输入源并识别其游戏文件格式
// Detect materializes an abstract source and identifies its game file format
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

// detectPath 按 COM3D2、KCES、舞蹈数据和 ARC 的优先顺序识别本地文件
// detectPath identifies a local file in COM3D2, KCES, dance-data, and ARC priority order
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

// detectDanceFile 通过内容探测识别 COM3D2 舞蹈二进制文件或编辑 JSON
// detectDanceFile identifies a COM3D2 dance binary or editing JSON file through content probing
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

// detectionFromFileInfo 将服务层文件信息转换为应用层检测结果
// detectionFromFileInfo converts service-layer file information into an application detection result
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

// Convert 将输入源转换为目标表示并将主要制品流式写入输出
// Convert transforms a source into the target representation and streams the primary artifact to the output
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

// ConvertBytes 执行转换并将主要制品内容收集到内存中
// ConvertBytes performs a conversion and collects the primary artifact content in memory
func (e *Engine) ConvertBytes(ctx context.Context, request ConvertRequest) (Artifact, []byte, error) {
	var output bytes.Buffer
	artifact, err := e.Convert(ctx, request, &output)
	if err != nil {
		return Artifact{}, nil, err
	}
	return artifact, output.Bytes(), nil
}

// Validate 检测并完整解析输入以确认其符合指定或自动识别的格式
// Validate detects and fully parses input to confirm that it conforms to the specified or automatically identified format
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

// pathConversionErrorCode 将转换器错误映射为稳定的应用错误代码
// pathConversionErrorCode maps a converter error to a stable application error code
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

// materialize 使用引擎的常规输入限制将抽象输入源写入临时工作区
// materialize writes an abstract source into a temporary workspace using the engine's normal input limit
func (e *Engine) materialize(ctx context.Context, source Source, name string) (string, string, error) {
	return e.materializeWithLimit(ctx, source, name, e.maxInputBytes)
}

// materializeWithLimit 将主要输入和伴随文件写入受总字节数限制的临时工作区
// materializeWithLimit writes a primary source and companion files into a temporary workspace under an aggregate byte limit
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

// materializeSourceFile 将单个输入源复制到独占本地文件并校验声明大小
// materializeSourceFile copies one source into an exclusive local file and verifies its declared size
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

// renameMaterializedArtifact 同步重命名已物化的主要文件及其伴随文件
// renameMaterializedArtifact renames a materialized primary file and its companion files together
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

// copyFileArtifact 校验转换输出及伴随文件并将主要文件流式写入调用方
// copyFileArtifact validates conversion output and companion files and streams the primary file to the caller
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

// readArtifactAttachments 读取受管理的转换伴随文件并检查其合计大小
// readArtifactAttachments reads managed conversion companion files and checks their aggregate size
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

// inspectArtifactFiles 检查主要输出和受管理伴随文件均为常规文件且未超过合计限制
// inspectArtifactFiles checks that the primary output and managed companions are regular files within an aggregate limit
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

// contextReader 在每次读取前检查上下文取消状态 / contextReader checks context cancellation before every read
type contextReader struct {
	// ctx 是读取操作遵循的上下文 / ctx is the context observed by read operations
	ctx context.Context
	// reader 是实际提供数据的底层读取器 / reader is the underlying reader that supplies data
	reader io.Reader
}

// Read 在上下文仍有效时从底层读取器读取数据
// Read reads from the underlying reader while the context remains active
func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// formatInputName 为路径转换器选择保留有效后缀的安全输入文件名
// formatInputName selects a safe converter input filename while preserving a valid suffix
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

// formatOutputName 根据目标表示从转换器输入名称派生输出文件名
// formatOutputName derives an output filename from the converter input name and target representation
func formatOutputName(format Format, inputName string, target Representation) string {
	if target == RepresentationEditingJSON {
		return trimJSONSuffix(inputName) + ".json"
	}
	return trimJSONSuffix(inputName)
}

// samePath 判断两个路径清理后是否不区分大小写地指向相同名称
// samePath reports whether two cleaned paths name the same location case-insensitively
func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }

// trimJSONSuffix 不区分大小写地移除文件名末尾的单个 JSON 后缀
// trimJSONSuffix removes one trailing JSON suffix from a filename case-insensitively
func trimJSONSuffix(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".json") {
		return name[:len(name)-len(".json")]
	}
	return name
}
