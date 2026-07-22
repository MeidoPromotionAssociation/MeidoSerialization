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
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	DefaultMaxResultBytes int64 = 2 << 20
	DefaultMaxWriteBytes        = application.DefaultMaxOutputBytes

	restrictedEditingWritePolicy   = "Use `output_root_id` and `output_relative_path`. Write only beneath a configured writable root declared by `meido://capabilities`."
	unrestrictedEditingWritePolicy = "Use `output_path` only for a path explicitly authorized by the user. This mode has no configured-root confinement and uses the server process account."
)

type Config struct {
	Engine         *application.Engine
	Roots          *application.RootSet
	FilesystemMode FilesystemMode
	Logger         *slog.Logger
	Version        string
	MaxResultBytes int64
	MaxWriteBytes  int64
}

type Server struct {
	engine         *application.Engine
	roots          *application.RootSet
	filesystemMode FilesystemMode
	server         *mcp.Server
	maxResultBytes int64
	maxWriteBytes  int64
	archivePager   *application.ArchivePager
	directWriteMu  sync.Mutex
}

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
	s.registerTools()
	s.registerResources()
	s.registerPrompts()
	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) MCPServer() *mcp.Server { return s.server }

type rootedFileInput struct {
	RootID       string `json:"root_id" jsonschema:"configured root ID"`
	RelativePath string `json:"relative_path" jsonschema:"portable path relative to the configured root"`
}

type directFileInput struct {
	Path string `json:"path" jsonschema:"absolute path or path relative to the MCP server working directory"`
}

type detectOutput struct {
	FormatID       string                     `json:"format_id"`
	Game           string                     `json:"game"`
	FileType       string                     `json:"file_type"`
	Representation application.Representation `json:"representation"`
	StorageFormat  string                     `json:"storage_format"`
	Signature      string                     `json:"signature,omitempty"`
	Version        int32                      `json:"version,omitempty"`
	Name           string                     `json:"name"`
	Size           int64                      `json:"size"`
}

type inspectInput struct {
	RootID       string `json:"root_id" jsonschema:"configured root ID"`
	RelativePath string `json:"relative_path" jsonschema:"portable path relative to the configured root"`
	FormatID     string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
}

type directInspectInput struct {
	Path     string `json:"path" jsonschema:"absolute path or path relative to the MCP server working directory"`
	FormatID string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
}

type inspectOutput struct {
	Name     string `json:"name"`
	FormatID string `json:"format_id"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

type validateInput struct {
	RootID       string `json:"root_id,omitempty" jsonschema:"configured root ID when validating a rooted file"`
	RelativePath string `json:"relative_path,omitempty" jsonschema:"relative path when validating a rooted file"`
	Name         string `json:"name,omitempty" jsonschema:"editing JSON filename including the native double extension"`
	EditingJSON  string `json:"editing_json,omitempty" jsonschema:"UTF-8 editing JSON supplied directly instead of a rooted file"`
	FormatID     string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
}

type directValidateInput struct {
	Path        string `json:"path,omitempty" jsonschema:"absolute path or path relative to the MCP server working directory"`
	Name        string `json:"name,omitempty" jsonschema:"editing JSON filename including the native double extension"`
	EditingJSON string `json:"editing_json,omitempty" jsonschema:"UTF-8 editing JSON supplied directly instead of a file"`
	FormatID    string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
}

type validateOutput struct {
	Valid     bool         `json:"valid"`
	Detection detectOutput `json:"detection"`
}

type convertInput struct {
	RootID             string `json:"root_id" jsonschema:"configured input root ID"`
	RelativePath       string `json:"relative_path" jsonschema:"portable input path relative to root_id"`
	FormatID           string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
	Target             string `json:"target" jsonschema:"target representation: native or editing_json"`
	OutputRootID       string `json:"output_root_id" jsonschema:"configured output root ID"`
	OutputRelativePath string `json:"output_relative_path" jsonschema:"portable destination path relative to output_root_id"`
}

type directConvertInput struct {
	Path       string `json:"path" jsonschema:"absolute input path or path relative to the MCP server working directory"`
	FormatID   string `json:"format_id,omitempty" jsonschema:"optional explicit format ID; empty enables detection"`
	Target     string `json:"target" jsonschema:"target representation: native or editing_json"`
	OutputPath string `json:"output_path" jsonschema:"absolute destination path or path relative to the MCP server working directory"`
}

type artifactOutput struct {
	Name           string                     `json:"name"`
	FormatID       string                     `json:"format_id"`
	Representation application.Representation `json:"representation"`
	Size           int64                      `json:"size"`
	SHA256         string                     `json:"sha256"`
	RootID         string                     `json:"root_id,omitempty"`
	RelativePath   string                     `json:"relative_path,omitempty"`
	Path           string                     `json:"path,omitempty"`
	Attachments    []artifactAttachmentOutput `json:"attachments,omitempty"`
}

type artifactAttachmentOutput struct {
	Suffix       string `json:"suffix"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	RootID       string `json:"root_id,omitempty"`
	RelativePath string `json:"relative_path,omitempty"`
	Path         string `json:"path,omitempty"`
}

type listArchiveInput struct {
	RootID       string `json:"root_id" jsonschema:"configured root ID"`
	RelativePath string `json:"relative_path" jsonschema:"portable archive path relative to root_id"`
	FormatID     string `json:"format_id,omitempty" jsonschema:"optional explicit archive format ID"`
	PageSize     int    `json:"page_size,omitempty" jsonschema:"optional page size; defaults to 128"`
	PageToken    string `json:"page_token,omitempty" jsonschema:"opaque server-signed page token returned by the previous call"`
}

type directListArchiveInput struct {
	Path      string `json:"path" jsonschema:"absolute archive path or path relative to the MCP server working directory"`
	FormatID  string `json:"format_id,omitempty" jsonschema:"optional explicit archive format ID"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"optional page size; defaults to 128"`
	PageToken string `json:"page_token,omitempty" jsonschema:"opaque server-signed page token returned by the previous call"`
}

type archiveEntryOutput struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Kind string `json:"kind"`
}

type listArchiveOutput struct {
	FormatID      string               `json:"format_id"`
	Entries       []archiveEntryOutput `json:"entries"`
	NextPageToken string               `json:"next_page_token,omitempty"`
}

type extractArchiveInput struct {
	RootID             string `json:"root_id" jsonschema:"configured input root ID"`
	RelativePath       string `json:"relative_path" jsonschema:"portable archive path relative to root_id"`
	FormatID           string `json:"format_id,omitempty" jsonschema:"optional explicit archive format ID"`
	EntryName          string `json:"entry_name" jsonschema:"exact entry name returned by meido.list_archive"`
	OutputRootID       string `json:"output_root_id" jsonschema:"configured output root ID"`
	OutputRelativePath string `json:"output_relative_path" jsonschema:"portable destination path relative to output_root_id"`
}

type directExtractArchiveInput struct {
	Path       string `json:"path" jsonschema:"absolute archive path or path relative to the MCP server working directory"`
	FormatID   string `json:"format_id,omitempty" jsonschema:"optional explicit archive format ID"`
	EntryName  string `json:"entry_name" jsonschema:"exact entry name returned by meido.list_archive"`
	OutputPath string `json:"output_path" jsonschema:"absolute destination path or path relative to the MCP server working directory"`
}

func (s *Server) registerTools() {
	if s.filesystemMode == FilesystemModeUnrestricted {
		s.registerUnrestrictedTools()
		return
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
		Description: "Strictly validate rooted or directly supplied editing JSON by encoding it with the native serializer.",
	}, s.validateEditingJSON)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.convert_file",
		Description: "Convert a rooted file to native or editing JSON and write it beneath a configured output root.",
	}, s.convertFile)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.list_archive",
		Description: "List entries in a COM3D2 ARC or KCES CT/ABA container.",
	}, s.listArchive)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.extract_archive_entry",
		Description: "Extract one exact archive entry beneath a configured output root.",
	}, s.extractArchiveEntry)
}

func (s *Server) registerUnrestrictedTools() {
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
		Description: "Strictly validate file-based or directly supplied editing JSON by encoding it with the native serializer.",
	}, s.validateDirectEditingJSON)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.convert_file",
		Description: "Convert a file to native or editing JSON and write it to an unrestricted filesystem path.",
	}, s.convertDirectFile)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.list_archive",
		Description: "List entries in a COM3D2 ARC or KCES CT/ABA container at a filesystem path.",
	}, s.listDirectArchive)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "meido.extract_archive_entry",
		Description: "Extract one exact archive entry to an unrestricted filesystem path.",
	}, s.extractDirectArchiveEntry)
}

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
			Text: s.editingSkill(formatID, document.Coverage),
		}}}, nil
	})
}

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
		Description: "Load the exact editing Schema, game field guide, and lossless editing workflow for one format.",
		Arguments:   arguments,
	}, s.editFormatPrompt)
}

func (s *Server) editFormatPrompt(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if request == nil || request.Params == nil {
		return nil, fmt.Errorf("prompt request is required")
	}
	arguments := request.Params.Arguments
	formatID := strings.ToLower(strings.TrimSpace(arguments["format_id"]))
	objective := strings.TrimSpace(arguments["objective"])
	if formatID == "" || strings.ContainsAny(formatID, `/\\?#`) {
		return nil, fmt.Errorf("format_id is required")
	}
	if objective == "" {
		return nil, fmt.Errorf("objective is required")
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
	task.WriteString("\n\nUse only the embedded Schema and guide as format authority. Preserve any schema_only field unless this objective requires a structural edit and the caller supplies its semantics.")
	type promptPathArgument struct {
		label string
		key   string
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
		Description: "Lossless " + formatID + " editing workflow with coverage " + guide.Coverage,
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: s.editingSkill(formatID, guide.Coverage)}},
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

func (s *Server) editingSkill(formatID, coverage string) string {
	policy := unrestrictedEditingWritePolicy
	if s.filesystemMode == FilesystemModeRestricted {
		policy = restrictedEditingWritePolicy
	}
	return knowledgev1.EditingSkill(formatID, coverage, policy)
}

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
			"format_guide_coverage": format.GuideCoverage,
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
		"schema_resource_template": "meido://schemas/{format_id}",
		"guide_resource_template":  "meido://guides/{format_id}",
		"skill_resource_template":  "meido://skills/editing/{format_id}",
		"editing_prompt":           "meido.edit_format",
	}
}

func (s *Server) detectFile(ctx context.Context, _ *mcp.CallToolRequest, input rootedFileInput) (*mcp.CallToolResult, detectOutput, error) {
	source, err := s.roots.Resolve(input.RootID, input.RelativePath)
	if err != nil {
		return nil, detectOutput{}, err
	}
	return s.detectSource(ctx, source)
}

func (s *Server) detectDirectFile(ctx context.Context, _ *mcp.CallToolRequest, input directFileInput) (*mcp.CallToolResult, detectOutput, error) {
	source, err := directSource(input.Path)
	if err != nil {
		return nil, detectOutput{}, err
	}
	return s.detectSource(ctx, source)
}

func (s *Server) detectSource(ctx context.Context, source application.Source) (*mcp.CallToolResult, detectOutput, error) {
	detection, err := s.engine.Detect(ctx, source)
	if err != nil {
		return nil, detectOutput{}, err
	}
	return nil, detectionOutput(detection), nil
}

func (s *Server) inspectFile(ctx context.Context, _ *mcp.CallToolRequest, input inspectInput) (*mcp.CallToolResult, inspectOutput, error) {
	source, err := s.roots.Resolve(input.RootID, input.RelativePath)
	if err != nil {
		return nil, inspectOutput{}, err
	}
	return s.inspectSource(ctx, source, input.FormatID)
}

func (s *Server) inspectDirectFile(ctx context.Context, _ *mcp.CallToolRequest, input directInspectInput) (*mcp.CallToolResult, inspectOutput, error) {
	source, err := directSource(input.Path)
	if err != nil {
		return nil, inspectOutput{}, err
	}
	return s.inspectSource(ctx, source, input.FormatID)
}

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

func (s *Server) validateSource(ctx context.Context, source application.Source, formatID string) (*mcp.CallToolResult, validateOutput, error) {
	detection, err := s.engine.Validate(ctx, source, formatID)
	if err != nil {
		return nil, validateOutput{}, err
	}
	return nil, validateOutput{Valid: true, Detection: detectionOutput(detection)}, nil
}

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

func (s *Server) listArchive(ctx context.Context, _ *mcp.CallToolRequest, input listArchiveInput) (*mcp.CallToolResult, listArchiveOutput, error) {
	source, err := s.roots.Resolve(input.RootID, input.RelativePath)
	if err != nil {
		return nil, listArchiveOutput{}, err
	}
	return s.listArchiveSource(ctx, source, input.FormatID, input.PageSize, input.PageToken)
}

func (s *Server) listDirectArchive(ctx context.Context, _ *mcp.CallToolRequest, input directListArchiveInput) (*mcp.CallToolResult, listArchiveOutput, error) {
	source, err := directSource(input.Path)
	if err != nil {
		return nil, listArchiveOutput{}, err
	}
	return s.listArchiveSource(ctx, source, input.FormatID, input.PageSize, input.PageToken)
}

func (s *Server) listArchiveSource(ctx context.Context, source application.Source, requestedFormatID string, requestedPageSize int, pageToken string) (*mcp.CallToolResult, listArchiveOutput, error) {
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
	result := listArchiveOutput{FormatID: listing.FormatID, Entries: make([]archiveEntryOutput, 0, pageSize)}
	nextIndex := start
	for nextIndex < len(listing.Entries) && len(result.Entries) < pageSize {
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
	defaultArchivePageSize = 128
	maxArchivePageSize     = 1000
)

func mcpArchivePageSize(value int) (int, error) {
	if value < 0 {
		return 0, fmt.Errorf("page_size must not be negative")
	}
	if value == 0 {
		return defaultArchivePageSize, nil
	}
	if value > maxArchivePageSize {
		return maxArchivePageSize, nil
	}
	return value, nil
}

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

func (s *Server) produceRootedFile(ctx context.Context, rootID, relativePath string, produce func(io.Writer) (application.Artifact, error)) (application.Artifact, error) {
	return s.produceFile(ctx, produce, func(path string, artifact application.Artifact) error {
		return s.installBundle(ctx, s.roots, rootID, relativePath, path, artifact)
	})
}

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

func directSource(path string) (application.Source, error) {
	if strings.TrimSpace(path) == "" || strings.IndexByte(path, 0) >= 0 {
		return nil, fmt.Errorf("path is required and must not contain NUL")
	}
	return application.NewFileSource(path)
}

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

func detectionOutput(value application.Detection) detectOutput {
	return detectOutput{
		FormatID: value.FormatID, Game: value.Game, FileType: value.FileType,
		Representation: value.Representation, StorageFormat: value.StorageFormat,
		Signature: value.Signature, Version: value.Version, Name: value.Name, Size: value.Size,
	}
}

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
