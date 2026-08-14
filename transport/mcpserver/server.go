package mcpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
	knowledgev1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/knowledge/v1"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// DefaultMaxResultBytes 是检查工具默认允许内联返回的最大字节数 / DefaultMaxResultBytes is the default maximum number of bytes returned inline by the inspect tool
	DefaultMaxResultBytes int64 = 2 << 20
	// DefaultMaxWriteBytes 是转换和提取工具默认允许写入的最大字节数 / DefaultMaxWriteBytes is the default maximum number of bytes written by conversion and extraction tools
	DefaultMaxWriteBytes = application.DefaultMaxOutputBytes

	restrictedEditingWritePolicy   = "Use `output_root_id` and `output_relative_path`. Write only beneath a configured writable root declared by `meido://capabilities`."
	unrestrictedEditingWritePolicy = "Use `output_path` only for a path explicitly authorized by the user. This mode has no configured-root confinement and uses the server process account."
)

// Config 配置 MCP 序列化服务器的依赖、文件系统模式和资源限制 / Config configures dependencies, filesystem mode, and resource limits for an MCP serialization server
type Config struct {
	// Engine 执行格式检测、转换、校验和归档操作 / Engine performs format detection, conversion, validation, and archive operations
	Engine *application.Engine
	// Roots 保存受限模式公开的可读写根目录 / Roots stores readable and writable roots exposed in restricted mode
	Roots *application.RootSet
	// FilesystemMode 选择受限根目录路径或直接文件系统路径 / FilesystemMode selects confined root paths or direct filesystem paths
	FilesystemMode FilesystemMode
	// Logger 接收 MCP SDK 和服务器诊断信息 / Logger receives MCP SDK and server diagnostics
	Logger *slog.Logger
	// Version 是 MCP 实现公开的应用版本 / Version is the application version advertised by the MCP implementation
	Version string
	// MaxResultBytes 限制检查和归档列表工具的内联结果大小 / MaxResultBytes limits inline results from inspect and archive-list tools
	MaxResultBytes int64
	// MaxWriteBytes 限制转换或提取制品的合计写入大小 / MaxWriteBytes limits aggregate bytes written for converted or extracted artifacts
	MaxWriteBytes int64
}

// Server 将应用引擎公开为带资源、提示和文件工具的 MCP 服务器 / Server exposes the application engine as an MCP server with resources, prompts, and file tools
type Server struct {
	// engine 执行与游戏文件格式有关的应用操作 / engine performs application operations related to game file formats
	engine *application.Engine
	// roots 解析受限输入和输出路径 / roots resolves confined input and output paths
	roots *application.RootSet
	// filesystemMode 控制工具使用受限根目录参数还是直接路径参数 / filesystemMode controls whether tools use confined root arguments or direct path arguments
	filesystemMode FilesystemMode
	// server 是处理 MCP 协议的 SDK 服务器 / server is the SDK server that handles the MCP protocol
	server *mcp.Server
	// maxResultBytes 限制结构化或文本工具结果的内联字节数 / maxResultBytes limits inline bytes in structured or textual tool results
	maxResultBytes int64
	// maxWriteBytes 限制安装主要制品及伴随文件的合计字节数 / maxWriteBytes limits aggregate bytes installed for a primary artifact and companions
	maxWriteBytes int64
	// archivePager 签名并验证服务器本地归档分页游标 / archivePager signs and verifies server-local archive page cursors
	archivePager *application.ArchivePager
	// directWriteMu 串行化非受限模式下的直接文件提交 / directWriteMu serializes direct filesystem commits in unrestricted mode
	directWriteMu sync.Mutex
}

// New 校验配置并创建已注册工具、资源和提示的 MCP 服务器
// New validates configuration and creates an MCP server with tools, resources, and prompts registered
func New(config Config) (*Server, error) {
	if config.Engine == nil {
		return nil, fmt.Errorf("application engine is required")
	}
	if config.Roots == nil {
		config.Roots = application.NewRootSet()
	}
	filesystemMode, err := resolveFilesystemMode(config.FilesystemMode, config.Roots)
	if err != nil {
		return nil, err
	}
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	if config.Version == "" {
		config.Version = "dev"
	}
	if config.MaxResultBytes <= 0 {
		config.MaxResultBytes = DefaultMaxResultBytes
	}
	if config.MaxWriteBytes <= 0 {
		config.MaxWriteBytes = DefaultMaxWriteBytes
	}
	archivePager, err := application.NewArchivePager()
	if err != nil {
		return nil, err
	}
	instructions := "Before editing, read the format schema and guide or use the meido.edit_format prompt. " +
		"Editing JSON preserves fields that cannot be represented safely by generic JSON object transports."
	if filesystemMode == FilesystemModeRestricted {
		instructions = "Inspect, validate, convert, and extract COM3D2 and KCES game files through configured root IDs. " + instructions
	} else {
		instructions = "Inspect, validate, convert, and extract COM3D2 and KCES game files through direct filesystem paths. " +
			"Filesystem access is unrestricted and uses the server process account. " + instructions
	}
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "meido-serialization",
		Title:   "MeidoSerialization",
		Version: config.Version,
	}, &mcp.ServerOptions{
		Logger:       config.Logger,
		Instructions: instructions,
	})
	s := &Server{
		engine: config.Engine, roots: config.Roots, filesystemMode: filesystemMode,
		server: mcpServer, maxResultBytes: config.MaxResultBytes, maxWriteBytes: config.MaxWriteBytes,
		archivePager: archivePager,
	}
	if err := s.registerTools(); err != nil {
		return nil, err
	}
	s.registerResources()
	s.registerPrompts()
	return s, nil
}

// Run 通过标准输入输出传输运行 MCP 服务器直到上下文结束
// Run serves MCP over standard input and output until the context ends
func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// MCPServer 返回底层 SDK 服务器以供嵌入和测试
// MCPServer returns the underlying SDK server for embedding and tests
func (s *Server) MCPServer() *mcp.Server { return s.server }

// rootedFileInput 描述受限根目录中的单个输入文件 / rootedFileInput describes one input file beneath a confined root
type rootedFileInput struct {
	// RootID 是能力资源公开的配置根标识符 / RootID is a configured root identifier advertised by the capabilities resource
	RootID string `json:"root_id" jsonschema:"configured root ID"`
	// RelativePath 是相对于配置根目录的可移植文件路径 / RelativePath is a portable file path relative to the configured root
	RelativePath string `json:"relative_path" jsonschema:"portable path relative to the configured root"`
}

// directFileInput 描述非受限模式下的直接输入路径 / directFileInput describes a direct input path in unrestricted mode
type directFileInput struct {
	// Path 是绝对路径或相对于服务器工作目录的路径 / Path is an absolute path or a path relative to the server working directory
	Path string `json:"path" jsonschema:"absolute path or path relative to the MCP server working directory"`
}

// detectOutput 描述 MCP 工具返回的文件格式检测结果 / detectOutput describes a file format detection result returned by an MCP tool
type detectOutput struct {
	// FormatID 是应用注册表中的稳定格式标识符 / FormatID is the stable format identifier in the application registry
	FormatID string `json:"format_id"`
	// Game 是检测到的游戏或工具名称 / Game is the detected game or tool name
	Game string `json:"game"`
	// FileType 是检测到的规范文件类型 / FileType is the detected canonical file type
	FileType string `json:"file_type"`
	// Representation 表示文件是原生格式还是编辑 JSON / Representation indicates whether the file is native data or editing JSON
	Representation application.Representation `json:"representation"`
	// StorageFormat 描述文件使用的底层存储编码 / StorageFormat describes the underlying storage encoding used by the file
	StorageFormat string `json:"storage_format"`
	// Signature 是检测到的可选文件签名 / Signature is the optional detected file signature
	Signature string `json:"signature,omitempty"`
	// Version 是检测到的精确格式版本 / Version is the exact detected format version
	Version int32 `json:"version,omitempty"`
	// Name 是不包含目录的输入文件名 / Name is the input filename without directory components
	Name string `json:"name"`
	// Size 是输入文件的精确字节数 / Size is the exact input file size in bytes
	Size int64 `json:"size"`
}

// inspectInput 描述受限模式下将原生文件检查为编辑 JSON 的请求 / inspectInput describes a request to inspect a native file as editing JSON in restricted mode
type inspectInput struct {
	// RootID 是输入文件所在的配置根标识符 / RootID is the configured root identifier containing the input file
	RootID string `json:"root_id" jsonschema:"configured root ID"`
	// RelativePath 是相对于配置根目录的可移植输入路径 / RelativePath is the portable input path relative to the configured root
	RelativePath string `json:"relative_path" jsonschema:"portable path relative to the configured root"`
	// FormatID 是可选显式格式标识符，空值启用检测 / FormatID is an optional explicit format identifier with an empty value enabling detection
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
}

// directInspectInput 描述非受限模式下将直接路径检查为编辑 JSON 的请求 / directInspectInput describes a request to inspect a direct path as editing JSON in unrestricted mode
type directInspectInput struct {
	// Path 是绝对输入路径或相对于服务器工作目录的路径 / Path is an absolute input path or a path relative to the server working directory
	Path string `json:"path" jsonschema:"absolute path or path relative to the MCP server working directory"`
	// FormatID 是可选显式格式标识符，空值启用检测 / FormatID is an optional explicit format identifier with an empty value enabling detection
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
}

// inspectOutput 描述检查工具生成的编辑 JSON 制品元数据 / inspectOutput describes editing JSON artifact metadata produced by the inspect tool
type inspectOutput struct {
	// Name 是编辑 JSON 的建议文件名 / Name is the suggested filename for the editing JSON
	Name string `json:"name"`
	// FormatID 是编辑 JSON 对应的稳定格式标识符 / FormatID is the stable format identifier associated with the editing JSON
	FormatID string `json:"format_id"`
	// Size 是编辑 JSON 的精确字节数 / Size is the exact editing JSON size in bytes
	Size int64 `json:"size"`
	// SHA256 是编辑 JSON 内容的十六进制 SHA-256 摘要 / SHA256 is the hexadecimal SHA-256 digest of the editing JSON content
	SHA256 string `json:"sha256"`
}

// validateInput 描述受限模式下校验文件或直接编辑 JSON 的请求 / validateInput describes a request to validate a rooted file or directly supplied editing JSON in restricted mode
type validateInput struct {
	// RootID 是校验受限文件时使用的配置根标识符 / RootID is the configured root identifier used when validating a confined file
	RootID string `json:"root_id,omitempty" jsonschema:"configured root ID when validating a rooted file"`
	// RelativePath 是校验受限文件时相对于根目录的路径 / RelativePath is the path relative to the root when validating a confined file
	RelativePath string `json:"relative_path,omitempty" jsonschema:"relative path when validating a rooted file"`
	// Name 是直接提供编辑 JSON 时包含原生双后缀的文件名 / Name is the filename including the native double suffix when editing JSON is supplied directly
	Name string `json:"name,omitempty" jsonschema:"editing JSON filename including the native double extension; required whenever editing_json is supplied"`
	// EditingJSON 是代替受限文件直接提供的 UTF-8 编辑 JSON / EditingJSON is UTF-8 editing JSON supplied directly instead of a confined file
	EditingJSON string `json:"editing_json,omitempty" jsonschema:"UTF-8 editing JSON supplied directly instead of a rooted file; requires name and ignores root_id and relative_path"`
	// FormatID 是可选显式格式标识符，空值启用检测 / FormatID is an optional explicit format identifier with an empty value enabling detection
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
}

// directValidateInput 描述非受限模式下校验路径或直接编辑 JSON 的请求 / directValidateInput describes a request to validate a path or directly supplied editing JSON in unrestricted mode
type directValidateInput struct {
	// Path 是待校验文件的直接路径 / Path is the direct path of the file to validate
	Path string `json:"path,omitempty" jsonschema:"absolute path or path relative to the MCP server working directory"`
	// Name 是直接提供编辑 JSON 时包含原生双后缀的文件名 / Name is the filename including the native double suffix when editing JSON is supplied directly
	Name string `json:"name,omitempty" jsonschema:"editing JSON filename including the native double extension; required whenever editing_json is supplied"`
	// EditingJSON 是代替文件直接提供的 UTF-8 编辑 JSON / EditingJSON is UTF-8 editing JSON supplied directly instead of a file
	EditingJSON string `json:"editing_json,omitempty" jsonschema:"UTF-8 editing JSON supplied directly instead of a file; requires name and ignores path"`
	// FormatID 是可选显式格式标识符，空值启用检测 / FormatID is an optional explicit format identifier with an empty value enabling detection
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
}

// validateOutput 描述成功完成的格式校验及其检测元数据 / validateOutput describes a successfully completed format validation and its detection metadata
type validateOutput struct {
	// Valid 表示输入已通过完整格式校验 / Valid reports that the input passed full format validation
	Valid bool `json:"valid"`
	// Detection 是已校验输入的格式检测元数据 / Detection is format detection metadata for the validated input
	Detection detectOutput `json:"detection"`
}

// convertInput 描述受限根目录之间的格式转换请求 / convertInput describes a format conversion request between confined roots
type convertInput struct {
	// RootID 是输入文件所在的配置根标识符 / RootID is the configured root identifier containing the input file
	RootID string `json:"root_id" jsonschema:"configured input root ID"`
	// RelativePath 是相对于输入根目录的可移植路径 / RelativePath is the portable path relative to the input root
	RelativePath string `json:"relative_path" jsonschema:"portable input path relative to root_id; the representation this file must already hold is decided by target"`
	// FormatID 是可选显式格式标识符，空值启用检测 / FormatID is an optional explicit format identifier with an empty value enabling detection
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
	// Target 是 native 或 editing_json 目标表示 / Target is the native or editing_json target representation
	Target string `json:"target" jsonschema:"target representation: native or editing_json; the input must hold the other representation, so editing_json reads a native game file and native reads an editing JSON document"`
	// OutputRootID 是接收转换结果的可写根标识符 / OutputRootID is the writable root identifier that receives the conversion result
	OutputRootID string `json:"output_root_id" jsonschema:"configured output root ID"`
	// OutputRelativePath 是相对于输出根目录的可移植目标路径 / OutputRelativePath is the portable destination path relative to the output root
	OutputRelativePath string `json:"output_relative_path" jsonschema:"portable destination path relative to output_root_id"`
}

// directConvertInput 描述非受限模式下直接路径之间的格式转换请求 / directConvertInput describes a format conversion request between direct paths in unrestricted mode
type directConvertInput struct {
	// Path 是绝对输入路径或相对于服务器工作目录的路径 / Path is an absolute input path or a path relative to the server working directory
	Path string `json:"path" jsonschema:"absolute input path or path relative to the MCP server working directory; the representation this file must already hold is decided by target"`
	// FormatID 是可选显式格式标识符，空值启用检测 / FormatID is an optional explicit format identifier with an empty value enabling detection
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
	// Target 是 native 或 editing_json 目标表示 / Target is the native or editing_json target representation
	Target string `json:"target" jsonschema:"target representation: native or editing_json; the input must hold the other representation, so editing_json reads a native game file and native reads an editing JSON document"`
	// OutputPath 是调用方授权的直接目标文件路径 / OutputPath is the direct destination file path authorized by the caller
	OutputPath string `json:"output_path" jsonschema:"absolute destination path or path relative to the MCP server working directory"`
}

// artifactOutput 描述 MCP 转换或提取工具安装的主要制品 / artifactOutput describes a primary artifact installed by an MCP conversion or extraction tool
type artifactOutput struct {
	// Name 是制品的建议文件名 / Name is the suggested filename of the artifact
	Name string `json:"name"`
	// FormatID 是制品对应的稳定格式标识符 / FormatID is the stable format identifier associated with the artifact
	FormatID string `json:"format_id"`
	// Representation 表示制品是原生格式还是编辑 JSON / Representation indicates whether the artifact is native data or editing JSON
	Representation application.Representation `json:"representation"`
	// Size 是主要制品的精确字节数 / Size is the exact primary artifact size in bytes
	Size int64 `json:"size"`
	// SHA256 是主要制品内容的十六进制 SHA-256 摘要 / SHA256 is the hexadecimal SHA-256 digest of the primary artifact content
	SHA256 string `json:"sha256"`
	// RootID 是受限模式下安装制品的根标识符 / RootID is the root identifier where the artifact was installed in restricted mode
	RootID string `json:"root_id,omitempty"`
	// RelativePath 是受限模式下已安装制品的相对路径 / RelativePath is the installed artifact path relative to its root in restricted mode
	RelativePath string `json:"relative_path,omitempty"`
	// Path 是非受限模式下已安装制品的直接路径 / Path is the direct installed artifact path in unrestricted mode
	Path string `json:"path,omitempty"`
	// Attachments 描述随主要制品安装的受管理伴随文件 / Attachments describes managed companion files installed with the primary artifact
	Attachments []artifactAttachmentOutput `json:"attachments,omitempty"`
}

// artifactAttachmentOutput 描述 MCP 工具安装的单个制品伴随文件 / artifactAttachmentOutput describes one artifact companion file installed by an MCP tool
type artifactAttachmentOutput struct {
	// Suffix 是追加到主要制品路径后的受管理后缀 / Suffix is the managed suffix appended to the primary artifact path
	Suffix string `json:"suffix"`
	// Name 是伴随文件的建议文件名 / Name is the suggested filename of the companion file
	Name string `json:"name"`
	// Size 是伴随文件的精确字节数 / Size is the exact companion file size in bytes
	Size int64 `json:"size"`
	// SHA256 是伴随文件内容的十六进制 SHA-256 摘要 / SHA256 is the hexadecimal SHA-256 digest of the companion file content
	SHA256 string `json:"sha256"`
	// RootID 是受限模式下安装伴随文件的根标识符 / RootID is the root identifier where the companion was installed in restricted mode
	RootID string `json:"root_id,omitempty"`
	// RelativePath 是受限模式下伴随文件相对于根目录的路径 / RelativePath is the companion path relative to its root in restricted mode
	RelativePath string `json:"relative_path,omitempty"`
	// Path 是非受限模式下伴随文件的直接路径 / Path is the direct companion path in unrestricted mode
	Path string `json:"path,omitempty"`
}

// listArchiveInput 描述受限模式下的归档分页列表请求 / listArchiveInput describes a paginated archive-list request in restricted mode
type listArchiveInput struct {
	// RootID 是归档所在的配置根标识符 / RootID is the configured root identifier containing the archive
	RootID string `json:"root_id" jsonschema:"configured root ID"`
	// RelativePath 是相对于输入根目录的可移植归档路径 / RelativePath is the portable archive path relative to the input root
	RelativePath string `json:"relative_path" jsonschema:"portable archive path relative to root_id"`
	// FormatID 是可选显式归档格式标识符 / FormatID is an optional explicit archive format identifier
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit archive format ID"`
	// PageSize 是可选的页面条目数 / PageSize is the optional number of entries requested for the page
	PageSize int32 `json:"page_size,omitempty" jsonschema:"optional page size; 0 or omitted selects the default 128 and the maximum is 1000; the effective value is returned as page_size"`
	// PageToken 是上一次调用返回的不透明服务器签名游标 / PageToken is the opaque server-signed cursor returned by the previous call
	PageToken string `json:"page_token,omitempty" jsonschema:"opaque server-signed page token returned by the previous call"`
}

// directListArchiveInput 描述非受限模式下的直接路径归档分页列表请求 / directListArchiveInput describes a paginated direct-path archive-list request in unrestricted mode
type directListArchiveInput struct {
	// Path 是绝对归档路径或相对于服务器工作目录的路径 / Path is an absolute archive path or a path relative to the server working directory
	Path string `json:"path" jsonschema:"absolute archive path or path relative to the MCP server working directory"`
	// FormatID 是可选显式归档格式标识符 / FormatID is an optional explicit archive format identifier
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit archive format ID"`
	// PageSize 是可选的页面条目数 / PageSize is the optional number of entries requested for the page
	PageSize int32 `json:"page_size,omitempty" jsonschema:"optional page size; 0 or omitted selects the default 128 and the maximum is 1000; the effective value is returned as page_size"`
	// PageToken 是上一次调用返回的不透明服务器签名游标 / PageToken is the opaque server-signed cursor returned by the previous call
	PageToken string `json:"page_token,omitempty" jsonschema:"opaque server-signed page token returned by the previous call"`
}

// archiveEntryOutput 描述归档列表中的单个条目 / archiveEntryOutput describes one entry in an archive listing
type archiveEntryOutput struct {
	// Name 是归档内部的精确条目名称 / Name is the exact entry name inside the archive
	Name string `json:"name"`
	// Size 是条目解压后的精确字节数 / Size is the exact decompressed entry size in bytes
	Size int64 `json:"size"`
	// Kind 是普通文件、虚拟文件或序列化文件类别 / Kind classifies the entry as a regular, virtual, or serialized file
	Kind string `json:"kind"`
}

// listArchiveOutput 描述一个带可选后续游标的归档列表页面 / listArchiveOutput describes one archive-list page with an optional continuation cursor
type listArchiveOutput struct {
	// FormatID 是实际用于读取归档的稳定格式标识符 / FormatID is the stable format identifier actually used to read the archive
	FormatID string `json:"format_id"`
	// PageSize 是本次调用实际生效的页大小 / PageSize is the page size that actually applied to this call
	PageSize int32 `json:"page_size"`
	// Entries 是当前页面包含的有序归档条目 / Entries contains ordered archive entries in the current page
	Entries []archiveEntryOutput `json:"entries"`
	// NextPageToken 是读取下一页时提交的不透明服务器签名游标 / NextPageToken is the opaque server-signed cursor submitted to read the next page
	NextPageToken string `json:"next_page_token,omitempty"`
}

// extractArchiveInput 描述受限根目录之间的归档条目提取请求 / extractArchiveInput describes an archive-entry extraction request between confined roots
type extractArchiveInput struct {
	// RootID 是输入归档所在的配置根标识符 / RootID is the configured root identifier containing the input archive
	RootID string `json:"root_id" jsonschema:"configured input root ID"`
	// RelativePath 是相对于输入根目录的可移植归档路径 / RelativePath is the portable archive path relative to the input root
	RelativePath string `json:"relative_path" jsonschema:"portable archive path relative to root_id"`
	// FormatID 是可选显式归档格式标识符 / FormatID is an optional explicit archive format identifier
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit archive format ID"`
	// EntryName 是列表工具返回的精确归档条目名称 / EntryName is the exact archive entry name returned by the list tool
	EntryName string `json:"entry_name" jsonschema:"exact entry name returned by meido.list_archive"`
	// OutputRootID 是接收提取结果的可写根标识符 / OutputRootID is the writable root identifier that receives the extracted result
	OutputRootID string `json:"output_root_id" jsonschema:"configured output root ID"`
	// OutputRelativePath 是相对于输出根目录的可移植目标路径 / OutputRelativePath is the portable destination path relative to the output root
	OutputRelativePath string `json:"output_relative_path" jsonschema:"portable destination path relative to output_root_id"`
}

// directExtractArchiveInput 描述非受限模式下的直接路径归档条目提取请求 / directExtractArchiveInput describes a direct-path archive-entry extraction request in unrestricted mode
type directExtractArchiveInput struct {
	// Path 是绝对归档路径或相对于服务器工作目录的路径 / Path is an absolute archive path or a path relative to the server working directory
	Path string `json:"path" jsonschema:"absolute archive path or path relative to the MCP server working directory"`
	// FormatID 是可选显式归档格式标识符 / FormatID is an optional explicit archive format identifier
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit archive format ID"`
	// EntryName 是列表工具返回的精确归档条目名称 / EntryName is the exact archive entry name returned by the list tool
	EntryName string `json:"entry_name" jsonschema:"exact entry name returned by meido.list_archive"`
	// OutputPath 是调用方授权的直接目标文件路径 / OutputPath is the direct destination file path authorized by the caller
	OutputPath string `json:"output_path" jsonschema:"absolute destination path or path relative to the MCP server working directory"`
}

// registerTools 根据文件系统模式注册受限或直接路径版本的文件工具
// registerTools registers confined-root or direct-path file tools according to the filesystem mode
func (s *Server) registerTools() error {
	if s.filesystemMode == FilesystemModeUnrestricted {
		return s.registerUnrestrictedTools()
	}
	validateSchema, err := validateToolInputSchema[validateInput]()
	if err != nil {
		return err
	}
	listArchiveSchema, err := listArchiveToolInputSchema[listArchiveInput]()
	if err != nil {
		return err
	}
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.detect_file",
		Description: "Detect the exact COM3D2 or KCES format of a rooted file.",
	}, s.detectFile)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.inspect_file",
		Description: "Convert a rooted proprietary game file to its lossless editing JSON representation.",
	}, s.inspectFile)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.validate_editing_json",
		Description: "Strictly validate rooted or directly supplied editing JSON by encoding it with the native serializer. Supply either root_id with relative_path, or editing_json with name.",
		InputSchema: validateSchema,
	}, s.validateEditingJSON)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.convert_file",
		Description: "Convert a rooted file to native or editing JSON and write it beneath a configured output root. The input representation is decided by target: target=editing_json reads a native game file, and target=native reads an editing JSON document produced by meido.inspect_file or by an earlier target=editing_json conversion.",
	}, s.convertFile)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.list_archive",
		Description: "List entries in a COM3D2 ARC or KCES CT/ABA container.",
		InputSchema: listArchiveSchema,
	}, s.listArchive)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.extract_archive_entry",
		Description: "Extract one exact archive entry beneath a configured output root.",
	}, s.extractArchiveEntry)
	return nil
}

// registerUnrestrictedTools 注册使用直接文件系统路径的非受限工具处理器
// registerUnrestrictedTools registers unrestricted tool handlers that accept direct filesystem paths
func (s *Server) registerUnrestrictedTools() error {
	validateSchema, err := validateToolInputSchema[directValidateInput]()
	if err != nil {
		return err
	}
	listArchiveSchema, err := listArchiveToolInputSchema[directListArchiveInput]()
	if err != nil {
		return err
	}
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.detect_file",
		Description: "Detect the exact COM3D2 or KCES format of a file path.",
	}, s.detectDirectFile)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.inspect_file",
		Description: "Convert a proprietary game file path to its lossless editing JSON representation.",
	}, s.inspectDirectFile)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.validate_editing_json",
		Description: "Strictly validate file-based or directly supplied editing JSON by encoding it with the native serializer. Supply either path, or editing_json with name.",
		InputSchema: validateSchema,
	}, s.validateDirectEditingJSON)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.convert_file",
		Description: "Convert a file to native or editing JSON and write it to an unrestricted filesystem path. The input representation is decided by target: target=editing_json reads a native game file, and target=native reads an editing JSON document produced by meido.inspect_file or by an earlier target=editing_json conversion.",
	}, s.convertDirectFile)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.list_archive",
		Description: "List entries in a COM3D2 ARC or KCES CT/ABA container at a filesystem path.",
		InputSchema: listArchiveSchema,
	}, s.listDirectArchive)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.extract_archive_entry",
		Description: "Extract one exact archive entry to an unrestricted filesystem path.",
	}, s.extractDirectArchiveEntry)
	return nil
}

// toolInputSchema 从工具输入类型推断输入模式，以便在推断结果上追加 Go 结构体标签无法表达的声明式约束
// toolInputSchema infers an input schema from a tool input type so that declarative constraints beyond Go struct tags can be appended to the inferred result
func toolInputSchema[Input any]() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[Input](&jsonschema.ForOptions{})
	if err != nil {
		return nil, err
	}
	if schema == nil || schema.Type != "object" {
		return nil, fmt.Errorf("inferred tool input schema is not an object")
	}
	return schema, nil
}

// validateToolInputSchema 声明内联提供编辑 JSON 时 name 同时必填，使该条件要求出现在工具输入模式中而不是只在运行时报错
// validateToolInputSchema declares that name is required alongside inline editing JSON so that the conditional requirement appears in the tool input schema instead of only failing at run time
func validateToolInputSchema[Input any]() (*jsonschema.Schema, error) {
	schema, err := toolInputSchema[Input]()
	if err != nil {
		return nil, err
	}
	nonEmptyEditingJSON := 1
	schema.If = &jsonschema.Schema{
		Required:   []string{"editing_json"},
		Properties: map[string]*jsonschema.Schema{"editing_json": {MinLength: &nonEmptyEditingJSON}},
	}
	schema.Then = &jsonschema.Schema{Required: []string{"name"}}
	return schema, nil
}

// listArchiveToolInputSchema 在工具输入模式中公开归档页大小的精确边界，使越界请求在协议层就被拒绝
// listArchiveToolInputSchema publishes the exact archive page-size bounds in the tool input schema so that out-of-range requests are rejected at the protocol layer
func listArchiveToolInputSchema[Input any]() (*jsonschema.Schema, error) {
	schema, err := toolInputSchema[Input]()
	if err != nil {
		return nil, err
	}
	pageSize, found := schema.Properties["page_size"]
	if !found {
		return nil, fmt.Errorf("inferred archive listing schema has no page_size property")
	}
	minimumPageSize, maximumPageSize := float64(0), float64(maxArchivePageSize)
	pageSize.Minimum = &minimumPageSize
	pageSize.Maximum = &maximumPageSize
	return schema, nil
}

// registerResources 注册能力、格式模式、字段指南和编辑技能资源
// registerResources registers capabilities, format schemas, field guides, and editing-skill resources
func (s *Server) registerResources() {
	s.server.AddResource(&mcp.Resource{
		Name:        "meido-serialization-capabilities",
		Title:       "MeidoSerialization capabilities",
		Description: "Registered format IDs, conversion capabilities, limits, filesystem mode, and configured root IDs.",
		MIMEType:    "application/json",
		URI:         "meido://capabilities",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, err := json.MarshalIndent(s.capabilities(), "", "  ")
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "meido://capabilities", MIMEType: "application/json", Text: string(data)}}}, nil
	})
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "meido-format-schema",
		Title:       "MeidoSerialization editing JSON schema",
		Description: "Versioned JSON Schema used to build a strongly typed editing interface before conversion.",
		URITemplate: "meido://schemas/{format_id}",
		MIMEType:    "application/schema+json",
	}, func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		formatID, err := resourceFormatID(request, "meido://schemas/")
		if err != nil {
			return nil, err
		}
		document, err := s.engine.GetFormatSchema(formatID)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: request.Params.URI, MIMEType: document.MediaType, Text: string(document.JSON),
		}}}, nil
	})
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "meido-format-guide",
		Title:       "MeidoSerialization game field guide",
		Description: "Complete field inventory plus source-reviewed game usage, edit roles, risks, invariants, and source evidence.",
		URITemplate: "meido://guides/{format_id}",
		MIMEType:    knowledgev1.MediaType,
	}, func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		formatID, err := resourceFormatID(request, "meido://guides/")
		if err != nil {
			return nil, err
		}
		document, err := s.engine.GetFormatGuide(formatID)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: request.Params.URI, MIMEType: document.MediaType, Text: string(document.JSON),
		}}}, nil
	})
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "meido-format-editing-skill",
		Title:       "MeidoSerialization format editing skill",
		Description: "Portable LLM workflow for editing one format without losing opaque or source-unreviewed fields.",
		URITemplate: "meido://skills/editing/{format_id}",
		MIMEType:    knowledgev1.SkillMediaType,
	}, func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		formatID, err := resourceFormatID(request, "meido://skills/editing/")
		if err != nil {
			return nil, err
		}
		document, err := s.engine.GetFormatGuide(formatID)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: request.Params.URI, MIMEType: knowledgev1.SkillMediaType,
			Text: s.editingSkill(formatID, document.FormatVerification),
		}}}, nil
	})
}

// resourceFormatID 从资源 URI 校验并提取规范化格式标识符
// resourceFormatID validates and extracts a normalized format identifier from a resource URI
func resourceFormatID(request *mcp.ReadResourceRequest, prefix string) (string, error) {
	if request == nil || request.Params == nil || !strings.HasPrefix(request.Params.URI, prefix) {
		return "", fmt.Errorf("invalid format resource URI")
	}
	formatID := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(request.Params.URI, prefix)))
	if formatID == "" || strings.ContainsAny(formatID, `/\\?#`) {
		return "", fmt.Errorf("invalid format resource URI")
	}
	return formatID, nil
}

// registerPrompts 注册根据文件系统模式调整路径参数的格式编辑提示
// registerPrompts registers the format-editing prompt with path arguments adapted to the filesystem mode
func (s *Server) registerPrompts() {
	arguments := []*mcp.PromptArgument{
		{Name: "format_id", Description: "format ID advertised by meido://capabilities", Required: true},
		{Name: "objective", Description: "specific user-requested edit", Required: true},
	}
	if s.filesystemMode == FilesystemModeRestricted {
		arguments = append(arguments,
			&mcp.PromptArgument{Name: "input_root_id", Description: "configured root containing the input file"},
			&mcp.PromptArgument{Name: "input_relative_path", Description: "input path relative to input_root_id"},
			&mcp.PromptArgument{Name: "output_root_id", Description: "configured writable output root"},
			&mcp.PromptArgument{Name: "output_relative_path", Description: "output path relative to output_root_id"},
		)
	} else {
		arguments = append(arguments,
			&mcp.PromptArgument{Name: "input_path", Description: "absolute input path or path relative to the MCP server working directory"},
			&mcp.PromptArgument{Name: "output_path", Description: "absolute output path or path relative to the MCP server working directory"},
		)
	}
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "meido.edit_format",
		Title:       "Edit a COM3D2 or KCES format",
		Description: "Load the exact editing Schema, game field guide, and lossless editing workflow for one format. The required arguments are format_id and objective; the remaining path arguments are optional. MCP prompts declare their parameters in this prompt's arguments list rather than in a tool input schema.",
		Arguments:   arguments,
	}, s.editFormatPrompt)
}

// editFormatPrompt 组合编辑技能、模式、指南、目标和可选路径参数
// editFormatPrompt combines an editing skill, schema, guide, objective, and optional path arguments
func (s *Server) editFormatPrompt(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if request == nil || request.Params == nil {
		return nil, fmt.Errorf("prompt request is required")
	}
	arguments := request.Params.Arguments
	formatID := strings.ToLower(strings.TrimSpace(arguments["format_id"]))
	objective := strings.TrimSpace(arguments["objective"])
	if formatID == "" || strings.ContainsAny(formatID, `/\\?#`) {
		return nil, fmt.Errorf("prompt argument format_id is required and must be one format ID advertised by meido://capabilities")
	}
	if objective == "" {
		return nil, fmt.Errorf("prompt argument objective is required and must describe the specific requested edit")
	}
	schema, err := s.engine.GetFormatSchema(formatID)
	if err != nil {
		return nil, err
	}
	guide, err := s.engine.GetFormatGuide(formatID)
	if err != nil {
		return nil, err
	}

	var task strings.Builder
	task.WriteString("Editing objective:\n")
	task.WriteString(objective)
	task.WriteString("\n\nUse only the embedded Schema and guide as format authority. A field with an empty verification object is schema-derived only: preserve it unless this objective requires a structural edit and the caller supplies its semantics.")
	// promptPathArgument 将提示参数键映射为任务文本中显示的标签 / promptPathArgument maps a prompt argument key to the label displayed in task text
	type promptPathArgument struct {
		// label 是写入任务文本的路径参数标签 / label is the path-argument label written into task text
		label string
		// key 是从提示请求读取参数值的键 / key is the key used to read the argument value from the prompt request
		key string
	}
	var pathArguments []promptPathArgument
	if s.filesystemMode == FilesystemModeRestricted {
		pathArguments = append(pathArguments,
			promptPathArgument{label: "Input root ID", key: "input_root_id"},
			promptPathArgument{label: "Input relative path", key: "input_relative_path"},
			promptPathArgument{label: "Output root ID", key: "output_root_id"},
			promptPathArgument{label: "Output relative path", key: "output_relative_path"},
		)
	} else {
		pathArguments = append(pathArguments,
			promptPathArgument{label: "Input path", key: "input_path"},
			promptPathArgument{label: "Output path", key: "output_path"},
		)
	}
	for _, value := range pathArguments {
		if argument := strings.TrimSpace(arguments[value.key]); argument != "" {
			task.WriteString("\n")
			task.WriteString(value.label)
			task.WriteString(": ")
			task.WriteString(argument)
		}
	}

	return &mcp.GetPromptResult{
		Description: "Lossless " + formatID + " editing workflow with whole-file verification " + guide.FormatVerification,
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: s.editingSkill(formatID, guide.FormatVerification)}},
			{Role: "user", Content: &mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
				URI: "meido://schemas/" + formatID, MIMEType: schema.MediaType, Text: string(schema.JSON),
			}}},
			{Role: "user", Content: &mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
				URI: "meido://guides/" + formatID, MIMEType: guide.MediaType, Text: string(guide.JSON),
			}}},
			{Role: "user", Content: &mcp.TextContent{Text: task.String()}},
		},
	}, nil
}

// editingSkill 按文件系统模式选择写入策略并呈现格式编辑技能
// editingSkill selects a write policy for the filesystem mode and renders the format-editing skill
func (s *Server) editingSkill(formatID, verification string) string {
	policy := unrestrictedEditingWritePolicy
	if s.filesystemMode == FilesystemModeRestricted {
		policy = restrictedEditingWritePolicy
	}
	return knowledgev1.EditingSkill(formatID, verification, policy)
}

// writePolicyCapabilities 返回当前文件系统模式的结构化写入策略能力
// writePolicyCapabilities returns structured write-policy capabilities for the current filesystem mode
func (s *Server) writePolicyCapabilities() map[string]any {
	if s.filesystemMode == FilesystemModeRestricted {
		return map[string]any{
			"mode": "configured_writable_roots", "output_arguments": []string{"output_root_id", "output_relative_path"},
			"root_confinement": true, "requires_explicit_user_path_authorization": false,
		}
	}
	return map[string]any{
		"mode": "explicit_user_authorized_path", "output_arguments": []string{"output_path"},
		"root_confinement": false, "requires_explicit_user_path_authorization": true,
		"uses_server_process_account": true,
	}
}

// capabilities 汇总格式、根目录、资源限制、写入策略和资源模板信息
// capabilities summarizes formats, roots, resource limits, write policy, and resource templates
func (s *Server) capabilities() map[string]any {
	maxArchiveListingBytes, maxArchiveEntries := s.engine.ArchiveListingLimits()
	formats := s.engine.Formats()
	result := make([]map[string]any, 0, len(formats))
	for _, format := range formats {
		result = append(result, map[string]any{
			"id": format.ID, "game": format.Game, "file_type": format.FileType,
			"native_suffixes": format.NativeSuffixes, "can_convert": format.Capability.Convert,
			"can_detect": format.Capability.Detect, "can_validate": format.Capability.Validate,
			"is_archive": format.Capability.Archive, "has_editing_schema": format.SchemaVersion != "",
			"editing_schema_version": format.SchemaVersion, "editing_schema_id": format.SchemaID,
			"editing_schema_sha256": format.SchemaSHA256,
			"has_format_guide":      format.GuideVersion != "", "format_guide_version": format.GuideVersion,
			"format_guide_id": format.GuideID, "format_guide_sha256": format.GuideSHA256,
			"format_guide_verification": format.GuideVerification,
		})
	}
	return map[string]any{
		"api_version": "meido.serialization.v1", "filesystem_mode": string(s.filesystemMode),
		"root_ids": s.roots.IDs(), "writable_root_ids": s.roots.WritableIDs(),
		"write_policy":             s.writePolicyCapabilities(),
		"max_inspect_result_bytes": s.maxResultBytes, "max_write_bytes": s.maxWriteBytes,
		"default_archive_page_size": defaultArchivePageSize, "max_archive_page_size": maxArchivePageSize,
		"max_archive_listing_bytes": maxArchiveListingBytes, "max_archive_entries": maxArchiveEntries,
		"formats":                  result,
		"format_support_boundary":  formatSupportBoundary,
		"cli_only_operations":      cliOnlyOperations(),
		"schema_resource_template": "meido://schemas/{format_id}",
		"guide_resource_template":  "meido://guides/{format_id}",
		"skill_resource_template":  "meido://skills/editing/{format_id}",
		"editing_prompt":           "meido.edit_format",
	}
}

// formatSupportBoundary 说明 formats 列表就是 MCP 的完整支持集，使调用方不必通过失败的检测去发现边界
// formatSupportBoundary states that the formats list is the complete MCP support set so that callers do not discover the boundary through a failing detection
const formatSupportBoundary = "The formats list is the complete MCP support set. A file type absent from it is not detected, converted, validated, or listed through MCP even when the MeidoSerialization command line supports it, and meido.detect_file reports such a file as not recognized. A listed format with can_convert false has no editing JSON representation, and a listed format with is_archive true is read through meido.list_archive and meido.extract_archive_entry. See cli_only_operations for the conversions that only the command line performs."

// cliOnlyOperations 列出 MeidoSerialization 命令行支持而 MCP 表面不公开的操作，并说明每条边界的原因
// cliOnlyOperations lists operations that the MeidoSerialization command line supports while the MCP surface does not expose them, and explains the reason for each boundary
func cliOnlyOperations() []map[string]any {
	return []map[string]any{
		{
			"game": "COM3D2", "file_type": "nei", "native_suffixes": []string{".nei"},
			"cli_commands": []string{"convert2csv", "convert2nei"},
			"detail":       "No MCP format is registered for .nei because it converts to CSV rather than to editing JSON. The command line converts .nei to .csv and .csv back to .nei.",
		},
		{
			"game": "COM3D2", "file_type": "tex", "native_suffixes": []string{".tex"},
			"cli_commands": []string{"convert2image", "convert2tex"},
			"detail":       "The registered com3d2.tex format is detect only because a texture converts to an image rather than to editing JSON. The command line converts .tex to PNG or DDS and an image back to .tex.",
		},
		{
			"game": "KCES", "file_type": "texture2d", "native_suffixes": []string{".tex", ".texture2d"},
			"cli_commands": []string{"convert2image", "convert2texture2d"},
			"detail":       "A native Unity Texture2D primary file is recognized by its class ID rather than by a suffix and has no MCP format. The command line converts it to PNG or DDS and an image back to a native Texture2D. PNG output is upright while DDS passes the Unity block payload through in bottom-up order. convert2texture2d has to keep the original file name and pixel dimensions, because PathIDs are hashed from the canonical file path and atlas textureRect values are absolute pixels, and it always rebuilds inline single-mip RGBA32, so a block-compressed texture grows and loses its mipmaps.",
		},
		{
			"game": "KCES", "file_type": "sprite", "native_suffixes": []string{".sprite"},
			"cli_commands": []string{"convert2image"},
			"detail":       "A native Unity Sprite primary file is recognized by its class ID rather than by a suffix and has no MCP format. The command line exports it to PNG. A Sprite stores no pixels, so no image-to-Sprite conversion exists; change how a Sprite looks by editing the Texture2D it resolves to, which leaves the .sprite and .partsatlas metadata byte identical through a repack. One atlas texture commonly backs hundreds of sprites, so edit only the region belonging to the target sprite.",
		},
		{
			"game": "KCES", "file_type": "mesh", "native_suffixes": []string{".mmesh"},
			"cli_commands": []string{"convert2gltf", "gltf2model"},
			"detail":       "MCP converts kces.model to editing JSON, while the geometry lives in the .mmesh companion that has no MCP format. Exporting a Model with its Mesh to glTF and importing glTF back to .model and .mmesh are command line only.",
		},
		{
			"game": "KCES", "file_type": "animation_clip", "native_suffixes": []string{".anm"},
			"cli_commands": []string{"convert2gltf"},
			"detail":       "A native Unity AnimationClip primary file is recognized by its class ID rather than by a suffix and has no MCP format. The command line exports it to glTF. The registered com3d2.anm format is the unrelated CM3D2_ANIM binary.",
		},
		{
			"game": "KCES", "file_type": "audio_clip", "native_suffixes": []string{".audioclip"},
			"cli_commands": []string{"convert2audio"},
			"detail":       "A native Unity AudioClip primary file is recognized by its class ID rather than by a suffix and has no MCP format. The command line extracts its inline OGG, WAV, or FSB5 payload without transcoding.",
		},
		{
			"game": "COM3D2 and KCES", "file_type": "archive", "native_suffixes": []string{".arc", ".aba", ".ct"},
			"cli_commands": []string{"packArc", "unpackArc", "packAba", "unpackAba", "genCt"},
			"detail":       "MCP lists container entries and extracts one exact entry at a time. Creating a container or unpacking a whole container in one call is command line only.",
		},
	}
}

// detectFile 解析受限根目录文件并返回其格式检测结果
// detectFile resolves a confined-root file and returns its format detection result
func (s *Server) detectFile(ctx context.Context, _ *mcp.CallToolRequest, input rootedFileInput) (*mcp.CallToolResult, detectOutput, error) {
	source, err := s.roots.Resolve(input.RootID, input.RelativePath)
	if err != nil {
		return nil, detectOutput{}, err
	}
	return s.detectSource(ctx, source)
}

// detectDirectFile 打开直接文件系统路径并返回其格式检测结果
// detectDirectFile opens a direct filesystem path and returns its format detection result
func (s *Server) detectDirectFile(ctx context.Context, _ *mcp.CallToolRequest, input directFileInput) (*mcp.CallToolResult, detectOutput, error) {
	source, err := directSource(input.Path)
	if err != nil {
		return nil, detectOutput{}, err
	}
	return s.detectSource(ctx, source)
}

// detectSource 使用应用引擎检测抽象输入源并转换输出模型
// detectSource detects an abstract input source with the application engine and converts the output model
func (s *Server) detectSource(ctx context.Context, source application.Source) (*mcp.CallToolResult, detectOutput, error) {
	detection, err := s.engine.Detect(ctx, source)
	if err != nil {
		return nil, detectOutput{}, err
	}
	return nil, detectionOutput(detection), nil
}

// inspectFile 将受限根目录中的原生文件转换为可内联检查的编辑 JSON
// inspectFile converts a native file beneath a confined root into editing JSON suitable for inline inspection
func (s *Server) inspectFile(ctx context.Context, _ *mcp.CallToolRequest, input inspectInput) (*mcp.CallToolResult, inspectOutput, error) {
	source, err := s.roots.Resolve(input.RootID, input.RelativePath)
	if err != nil {
		return nil, inspectOutput{}, err
	}
	return s.inspectSource(ctx, source, input.FormatID)
}

// inspectDirectFile 将直接路径中的原生文件转换为可内联检查的编辑 JSON
// inspectDirectFile converts a native file at a direct path into editing JSON suitable for inline inspection
func (s *Server) inspectDirectFile(ctx context.Context, _ *mcp.CallToolRequest, input directInspectInput) (*mcp.CallToolResult, inspectOutput, error) {
	source, err := directSource(input.Path)
	if err != nil {
		return nil, inspectOutput{}, err
	}
	return s.inspectSource(ctx, source, input.FormatID)
}

// inspectSource 转换输入源并在结果限制内返回经过 JSON 语法检查的编辑内容
// inspectSource converts a source and returns syntax-checked editing JSON within the result limit
func (s *Server) inspectSource(ctx context.Context, source application.Source, formatID string) (*mcp.CallToolResult, inspectOutput, error) {
	temp, err := os.CreateTemp("", "meido-mcp-inspect-")
	if err != nil {
		return nil, inspectOutput{}, err
	}
	path := temp.Name()
	defer os.Remove(path)
	artifact, err := s.engine.Convert(ctx, application.ConvertRequest{Source: source, FormatID: formatID, To: application.RepresentationEditingJSON}, temp)
	closeErr := temp.Close()
	if err != nil {
		return nil, inspectOutput{}, err
	}
	if closeErr != nil {
		return nil, inspectOutput{}, closeErr
	}
	if artifact.Size > s.maxResultBytes {
		return nil, inspectOutput{}, fmt.Errorf("editing JSON is %d bytes, above the MCP inline limit %d; use meido.convert_file", artifact.Size, s.maxResultBytes)
	}
	output, err := os.ReadFile(path)
	if err != nil {
		return nil, inspectOutput{}, err
	}
	if !json.Valid(output) {
		return nil, inspectOutput{}, fmt.Errorf("converter returned invalid editing JSON")
	}
	result := inspectOutput{Name: artifact.Name, FormatID: artifact.FormatID, Size: artifact.Size, SHA256: artifact.SHA256}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(output)}}}, result, nil
}

// validateEditingJSON 校验受限文件或直接提供的编辑 JSON
// validateEditingJSON validates a confined file or directly supplied editing JSON
func (s *Server) validateEditingJSON(ctx context.Context, _ *mcp.CallToolRequest, input validateInput) (*mcp.CallToolResult, validateOutput, error) {
	var source application.Source
	var err error
	if input.EditingJSON != "" {
		if strings.TrimSpace(input.Name) == "" {
			return nil, validateOutput{}, fmt.Errorf("name is required with directly supplied editing_json")
		}
		source = application.NewBytesSource(input.Name, []byte(input.EditingJSON))
	} else {
		source, err = s.roots.Resolve(input.RootID, input.RelativePath)
		if err != nil {
			return nil, validateOutput{}, err
		}
	}
	return s.validateSource(ctx, source, input.FormatID)
}

// validateDirectEditingJSON 校验直接路径文件或直接提供的编辑 JSON
// validateDirectEditingJSON validates a direct-path file or directly supplied editing JSON
func (s *Server) validateDirectEditingJSON(ctx context.Context, _ *mcp.CallToolRequest, input directValidateInput) (*mcp.CallToolResult, validateOutput, error) {
	var source application.Source
	var err error
	if input.EditingJSON != "" {
		if strings.TrimSpace(input.Name) == "" {
			return nil, validateOutput{}, fmt.Errorf("name is required with directly supplied editing_json")
		}
		source = application.NewBytesSource(input.Name, []byte(input.EditingJSON))
	} else {
		source, err = directSource(input.Path)
		if err != nil {
			return nil, validateOutput{}, err
		}
	}
	return s.validateSource(ctx, source, input.FormatID)
}

// validateSource 使用应用引擎完整校验输入源并转换检测结果
// validateSource fully validates a source with the application engine and converts its detection result
func (s *Server) validateSource(ctx context.Context, source application.Source, formatID string) (*mcp.CallToolResult, validateOutput, error) {
	detection, err := s.engine.Validate(ctx, source, formatID)
	if err != nil {
		return nil, validateOutput{}, err
	}
	return nil, validateOutput{Valid: true, Detection: detectionOutput(detection)}, nil
}

// convertFile 转换受限根目录输入并将完整制品集合安装到可写根目录
// convertFile converts confined-root input and installs the complete artifact bundle beneath a writable root
func (s *Server) convertFile(ctx context.Context, _ *mcp.CallToolRequest, input convertInput) (*mcp.CallToolResult, artifactOutput, error) {
	target, err := parseRepresentation(input.Target)
	if err != nil {
		return nil, artifactOutput{}, err
	}
	if err := s.roots.ValidateWrite(input.OutputRootID, input.OutputRelativePath); err != nil {
		return nil, artifactOutput{}, err
	}
	source, err := s.roots.Resolve(input.RootID, input.RelativePath)
	if err != nil {
		return nil, artifactOutput{}, err
	}
	artifact, err := s.produceRootedFile(ctx, input.OutputRootID, input.OutputRelativePath, func(writer io.Writer) (application.Artifact, error) {
		return s.engine.Convert(ctx, application.ConvertRequest{Source: source, FormatID: input.FormatID, To: target}, writer)
	})
	if err != nil {
		return nil, artifactOutput{}, err
	}
	return nil, artifactResult(artifact, input.OutputRootID, input.OutputRelativePath), nil
}

// convertDirectFile 转换直接路径输入并将完整制品集合安装到授权目标路径
// convertDirectFile converts direct-path input and installs the complete artifact bundle at an authorized destination
func (s *Server) convertDirectFile(ctx context.Context, _ *mcp.CallToolRequest, input directConvertInput) (*mcp.CallToolResult, artifactOutput, error) {
	target, err := parseRepresentation(input.Target)
	if err != nil {
		return nil, artifactOutput{}, err
	}
	outputPath, err := directOutputPath(input.OutputPath)
	if err != nil {
		return nil, artifactOutput{}, err
	}
	source, err := directSource(input.Path)
	if err != nil {
		return nil, artifactOutput{}, err
	}
	artifact, err := s.produceDirectFile(ctx, outputPath, func(writer io.Writer) (application.Artifact, error) {
		return s.engine.Convert(ctx, application.ConvertRequest{Source: source, FormatID: input.FormatID, To: target}, writer)
	})
	if err != nil {
		return nil, artifactOutput{}, err
	}
	return nil, directArtifactResult(artifact, outputPath), nil
}

// listArchive 解析受限根目录归档并返回请求的分页列表
// listArchive resolves a confined-root archive and returns the requested listing page
func (s *Server) listArchive(ctx context.Context, _ *mcp.CallToolRequest, input listArchiveInput) (*mcp.CallToolResult, listArchiveOutput, error) {
	source, err := s.roots.Resolve(input.RootID, input.RelativePath)
	if err != nil {
		return nil, listArchiveOutput{}, err
	}
	return s.listArchiveSource(ctx, source, input.FormatID, input.PageSize, input.PageToken)
}

// listDirectArchive 打开直接路径归档并返回请求的分页列表
// listDirectArchive opens a direct-path archive and returns the requested listing page
func (s *Server) listDirectArchive(ctx context.Context, _ *mcp.CallToolRequest, input directListArchiveInput) (*mcp.CallToolResult, listArchiveOutput, error) {
	source, err := directSource(input.Path)
	if err != nil {
		return nil, listArchiveOutput{}, err
	}
	return s.listArchiveSource(ctx, source, input.FormatID, input.PageSize, input.PageToken)
}

// listArchiveSource 返回同时受条目数和 MCP 结果字节数限制的归档页面
// listArchiveSource returns an archive page bounded by both entry count and MCP result bytes
func (s *Server) listArchiveSource(ctx context.Context, source application.Source, requestedFormatID string, requestedPageSize int32, pageToken string) (*mcp.CallToolResult, listArchiveOutput, error) {
	pageSize, err := mcpArchivePageSize(requestedPageSize)
	if err != nil {
		return nil, listArchiveOutput{}, err
	}
	listing, err := s.engine.ListArchiveListing(ctx, source, requestedFormatID)
	if err != nil {
		return nil, listArchiveOutput{}, err
	}
	start, err := s.archivePager.Decode(listing, pageToken)
	if err != nil {
		return nil, listArchiveOutput{}, err
	}
	result := listArchiveOutput{FormatID: listing.FormatID, PageSize: pageSize, Entries: make([]archiveEntryOutput, 0, pageSize)}
	nextIndex := start
	for nextIndex < len(listing.Entries) && int32(len(result.Entries)) < pageSize {
		entry := listing.Entries[nextIndex]
		result.Entries = append(result.Entries, archiveEntryOutput{Name: entry.Name, Size: entry.Size, Kind: entry.Kind})
		if nextIndex+1 < len(listing.Entries) {
			result.NextPageToken, err = s.archivePager.Encode(listing, nextIndex+1)
			if err != nil {
				return nil, listArchiveOutput{}, err
			}
		} else {
			result.NextPageToken = ""
		}
		encoded, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return nil, listArchiveOutput{}, marshalErr
		}
		if int64(len(encoded)) > s.maxResultBytes {
			result.Entries = result.Entries[:len(result.Entries)-1]
			if len(result.Entries) == 0 {
				return nil, listArchiveOutput{}, fmt.Errorf("archive entry at index %d exceeds MCP result limit %d", nextIndex, s.maxResultBytes)
			}
			result.NextPageToken, err = s.archivePager.Encode(listing, nextIndex)
			if err != nil {
				return nil, listArchiveOutput{}, err
			}
			break
		}
		nextIndex++
	}
	if nextIndex < len(listing.Entries) && result.NextPageToken == "" {
		result.NextPageToken, err = s.archivePager.Encode(listing, nextIndex)
		if err != nil {
			return nil, listArchiveOutput{}, err
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, listArchiveOutput{}, err
	}
	if int64(len(encoded)) > s.maxResultBytes {
		return nil, listArchiveOutput{}, fmt.Errorf("archive page exceeds MCP result limit %d", s.maxResultBytes)
	}
	// StructuredContent is the canonical archive page. Supplying an empty
	// content slice prevents the SDK from duplicating the same JSON as text.
	return &mcp.CallToolResult{Content: []mcp.Content{}}, result, nil
}

const (
	// defaultArchivePageSize 是 MCP 归档列表未指定页大小时返回的默认条目数 / defaultArchivePageSize is the default number of MCP archive entries returned when no page size is specified
	defaultArchivePageSize int32 = 128
	// maxArchivePageSize 是单个 MCP 归档页面允许的最大条目数 / maxArchivePageSize is the maximum number of entries permitted in one MCP archive page
	maxArchivePageSize int32 = 1000
)

// mcpArchivePageSize 把未指定的页大小解析为默认值，并拒绝越界的页大小而不是静默改写请求
// mcpArchivePageSize resolves an unspecified page size to the default and rejects an out-of-range page size instead of silently rewriting the request
func mcpArchivePageSize(value int32) (int32, error) {
	if value < 0 {
		return 0, fmt.Errorf("page_size must not be negative")
	}
	if value == 0 {
		return defaultArchivePageSize, nil
	}
	if value > maxArchivePageSize {
		return 0, fmt.Errorf("page_size %d is above the maximum %d", value, maxArchivePageSize)
	}
	return value, nil
}

// extractArchiveEntry 从受限根目录归档提取条目并安装到可写根目录
// extractArchiveEntry extracts an entry from a confined-root archive and installs it beneath a writable root
func (s *Server) extractArchiveEntry(ctx context.Context, _ *mcp.CallToolRequest, input extractArchiveInput) (*mcp.CallToolResult, artifactOutput, error) {
	if err := s.roots.ValidateWrite(input.OutputRootID, input.OutputRelativePath); err != nil {
		return nil, artifactOutput{}, err
	}
	source, err := s.roots.Resolve(input.RootID, input.RelativePath)
	if err != nil {
		return nil, artifactOutput{}, err
	}
	artifact, err := s.produceRootedFile(ctx, input.OutputRootID, input.OutputRelativePath, func(writer io.Writer) (application.Artifact, error) {
		return s.engine.ExtractArchiveEntry(ctx, source, input.FormatID, input.EntryName, writer)
	})
	if err != nil {
		return nil, artifactOutput{}, err
	}
	return nil, artifactResult(artifact, input.OutputRootID, input.OutputRelativePath), nil
}

// extractDirectArchiveEntry 从直接路径归档提取条目并安装到授权目标路径
// extractDirectArchiveEntry extracts an entry from a direct-path archive and installs it at an authorized destination
func (s *Server) extractDirectArchiveEntry(ctx context.Context, _ *mcp.CallToolRequest, input directExtractArchiveInput) (*mcp.CallToolResult, artifactOutput, error) {
	outputPath, err := directOutputPath(input.OutputPath)
	if err != nil {
		return nil, artifactOutput{}, err
	}
	source, err := directSource(input.Path)
	if err != nil {
		return nil, artifactOutput{}, err
	}
	artifact, err := s.produceDirectFile(ctx, outputPath, func(writer io.Writer) (application.Artifact, error) {
		return s.engine.ExtractArchiveEntry(ctx, source, input.FormatID, input.EntryName, writer)
	})
	if err != nil {
		return nil, artifactOutput{}, err
	}
	return nil, directArtifactResult(artifact, outputPath), nil
}

// produceRootedFile 暂存生成的制品并将完整集合安装到配置根目录
// produceRootedFile stages a produced artifact and installs the complete bundle beneath a configured root
func (s *Server) produceRootedFile(ctx context.Context, rootID, relativePath string, produce func(io.Writer) (application.Artifact, error)) (application.Artifact, error) {
	return s.produceFile(ctx, produce, func(path string, artifact application.Artifact) error {
		return s.installBundle(ctx, s.roots, rootID, relativePath, path, artifact)
	})
}

// produceDirectFile 暂存生成的制品并将完整集合安装到直接目标路径
// produceDirectFile stages a produced artifact and installs the complete bundle at a direct destination path
func (s *Server) produceDirectFile(ctx context.Context, outputPath string, produce func(io.Writer) (application.Artifact, error)) (application.Artifact, error) {
	return s.produceFile(ctx, produce, func(path string, artifact application.Artifact) error {
		s.directWriteMu.Lock()
		defer s.directWriteMu.Unlock()

		parent := filepath.Dir(outputPath)
		if err := os.MkdirAll(parent, 0755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		roots := application.NewRootSet()
		defer roots.Close()
		const directRootID = "direct-output"
		if err := roots.AddWritable(directRootID, parent); err != nil {
			return err
		}
		return s.installBundle(ctx, roots, directRootID, filepath.Base(outputPath), path, artifact)
	})
}

// produceFile 生成、校验并在写入限制内安装一个完整制品集合
// produceFile produces, verifies, and installs a complete artifact bundle within the write limit
func (s *Server) produceFile(ctx context.Context, produce func(io.Writer) (application.Artifact, error), install func(string, application.Artifact) error) (application.Artifact, error) {
	temp, err := os.CreateTemp("", "meido-mcp-result-")
	if err != nil {
		return application.Artifact{}, err
	}
	path := temp.Name()
	defer os.Remove(path)
	artifact, err := produce(temp)
	closeErr := temp.Close()
	if err != nil {
		return application.Artifact{}, err
	}
	if closeErr != nil {
		return application.Artifact{}, closeErr
	}
	if artifact.Size < 0 || artifact.TotalSize() > s.maxWriteBytes {
		return application.Artifact{}, fmt.Errorf("artifact output is %d bytes, above the write limit %d", artifact.TotalSize(), s.maxWriteBytes)
	}
	if err := verifyArtifactFile(path, artifact); err != nil {
		return application.Artifact{}, err
	}
	if err := install(path, artifact); err != nil {
		return application.Artifact{}, err
	}
	return artifact, nil
}

// installBundle 将已验证的主要文件和内存伴随文件原子写入目标根目录
// installBundle atomically writes a verified primary file and in-memory companions beneath a target root
func (s *Server) installBundle(ctx context.Context, roots *application.RootSet, rootID, relativePath, path string, artifact application.Artifact) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	primarySize := artifact.Size
	files := []application.BundleFile{{Reader: file, ExpectedSize: &primarySize, ExpectedSHA256: artifact.SHA256}}
	for _, attachment := range artifact.AttachmentFiles() {
		expectedSize := attachment.Size
		files = append(files, application.BundleFile{
			Suffix: attachment.Suffix, Reader: bytes.NewReader(attachment.Data),
			ExpectedSize: &expectedSize, ExpectedSHA256: attachment.SHA256,
		})
	}
	_, err = roots.WriteBundle(ctx, rootID, relativePath, files, s.maxWriteBytes)
	return err
}

// directSource 校验直接输入路径并创建包含受管理伴随文件的本地源
// directSource validates a direct input path and creates a local source including managed companions
func directSource(path string) (application.Source, error) {
	if strings.TrimSpace(path) == "" || strings.IndexByte(path, 0) >= 0 {
		return nil, fmt.Errorf("path is required and must not contain NUL")
	}
	return application.NewFileSource(path)
}

// directOutputPath 校验直接输出文件路径并返回清理后的绝对路径
// directOutputPath validates a direct output file path and returns its cleaned absolute path
func directOutputPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" || strings.IndexByte(path, 0) >= 0 {
		return "", fmt.Errorf("output_path is required and must not contain NUL")
	}
	if strings.HasSuffix(path, string(os.PathSeparator)) || (os.PathSeparator == '\\' && strings.HasSuffix(path, "/")) {
		return "", fmt.Errorf("output_path must name a file, not a directory")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve output_path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if info, err := os.Lstat(absolute); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("output_path %q is not a regular file", path)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect output_path: %w", err)
	}
	return absolute, nil
}

// verifyArtifactFile 重新计算暂存主要文件的大小和摘要以验证制品元数据
// verifyArtifactFile recomputes the staged primary file size and digest to verify artifact metadata
func verifyArtifactFile(path string, artifact application.Artifact) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect conversion output: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != artifact.Size {
		return fmt.Errorf("conversion output metadata mismatch: file size=%d artifact size=%d", info.Size(), artifact.Size)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open conversion output for verification: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash conversion output: %w", err)
	}
	if digest := hex.EncodeToString(hash.Sum(nil)); digest != artifact.SHA256 {
		return fmt.Errorf("conversion output digest mismatch: got %s, want %s", digest, artifact.SHA256)
	}
	return nil
}

// parseRepresentation 将 MCP 文本参数解析为应用层目标表示
// parseRepresentation parses an MCP text argument into an application target representation
func parseRepresentation(value string) (application.Representation, error) {
	switch application.Representation(strings.ToLower(strings.TrimSpace(value))) {
	case application.RepresentationNative:
		return application.RepresentationNative, nil
	case application.RepresentationEditingJSON:
		return application.RepresentationEditingJSON, nil
	default:
		return "", fmt.Errorf("target must be native or editing_json")
	}
}

// detectionOutput 将应用检测结果转换为 MCP 结构化输出
// detectionOutput converts an application detection result into MCP structured output
func detectionOutput(value application.Detection) detectOutput {
	return detectOutput{
		FormatID: value.FormatID, Game: value.Game, FileType: value.FileType,
		Representation: value.Representation, StorageFormat: value.StorageFormat,
		Signature: value.Signature, Version: value.Version, Name: value.Name, Size: value.Size,
	}
}

// artifactResult 将受限根目录中的应用制品转换为 MCP 结构化输出
// artifactResult converts an application artifact beneath a confined root into MCP structured output
func artifactResult(value application.Artifact, rootID, relativePath string) artifactOutput {
	result := artifactOutput{
		Name: value.Name, FormatID: value.FormatID, Representation: value.Representation,
		Size: value.Size, SHA256: value.SHA256, RootID: rootID, RelativePath: relativePath,
	}
	for _, attachment := range value.AttachmentFiles() {
		result.Attachments = append(result.Attachments, artifactAttachmentOutput{
			Suffix: attachment.Suffix, Name: attachment.Name, Size: attachment.Size, SHA256: attachment.SHA256,
			RootID: rootID, RelativePath: relativePath + attachment.Suffix,
		})
	}
	return result
}

// directArtifactResult 将直接路径中的应用制品转换为 MCP 结构化输出
// directArtifactResult converts an application artifact at a direct path into MCP structured output
func directArtifactResult(value application.Artifact, path string) artifactOutput {
	result := artifactOutput{
		Name: value.Name, FormatID: value.FormatID, Representation: value.Representation,
		Size: value.Size, SHA256: value.SHA256, Path: path,
	}
	for _, attachment := range value.AttachmentFiles() {
		result.Attachments = append(result.Attachments, artifactAttachmentOutput{
			Suffix: attachment.Suffix, Name: attachment.Name, Size: attachment.Size, SHA256: attachment.SHA256,
			Path: path + attachment.Suffix,
		})
	}
	return result
}
