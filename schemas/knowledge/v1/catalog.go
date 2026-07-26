// Package knowledgev1 提供 MCP 和 gRPC 传输使用的源码审核游戏语义指南
// Package knowledgev1 provides source-reviewed game-semantics guides used by the MCP and gRPC transports
package knowledgev1

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// Version 是当前格式指南目录版本 / Version is the current format-guide catalog version
	Version = "1.0.0"
	// MediaType 是格式指南 JSON 文档的媒体类型 / MediaType is the media type of format-guide JSON documents
	MediaType = "application/vnd.meido.format-guide+json"
	// IDPrefix 是格式指南规范标识符的固定前缀 / IDPrefix is the fixed prefix for canonical format-guide identifiers
	IDPrefix = "urn:meido-serialization:format-guide:v1:"

	// CoverageRuntimeVerified 表示字段语义已由游戏运行时代码验证 / CoverageRuntimeVerified indicates field semantics verified against game runtime code
	CoverageRuntimeVerified = "runtime_verified"
	// CoverageSerializationVerified 表示字段布局已由序列化代码验证 / CoverageSerializationVerified indicates field layout verified against serialization code
	CoverageSerializationVerified = "serialization_verified"
	// CoverageSchemaOnly 表示字段只有模式结构而没有源码语义审核 / CoverageSchemaOnly indicates fields with schema structure but no source-reviewed semantics
	CoverageSchemaOnly = "schema_only"

	// ConfidenceVerified 表示字段语义已由游戏源码验证 / ConfidenceVerified indicates field semantics verified against game source
	ConfidenceVerified = "verified"
	// ConfidenceSerializationOnly 表示字段信息仅由序列化源码验证 / ConfidenceSerializationOnly indicates field information verified only against serialization source
	ConfidenceSerializationOnly = "serialization_only"
	// ConfidenceSchemaOnly 表示字段信息仅来自编辑模式 / ConfidenceSchemaOnly indicates field information derived only from the editing schema
	ConfidenceSchemaOnly = "schema_only"

	// HumanReviewPrefix 是仅供人工明确批准状态使用的保留前缀 / HumanReviewPrefix is reserved for states explicitly approved by a human
	HumanReviewPrefix = "human_"

	// CoverageHumanRuntimeVerified 表示经过人工批准的运行时语义覆盖 / CoverageHumanRuntimeVerified indicates human-approved runtime-semantic coverage
	CoverageHumanRuntimeVerified = HumanReviewPrefix + CoverageRuntimeVerified
	// CoverageHumanSerializationVerified 表示经过人工批准的序列化布局覆盖 / CoverageHumanSerializationVerified indicates human-approved serialization-layout coverage
	CoverageHumanSerializationVerified = HumanReviewPrefix + CoverageSerializationVerified
	// ConfidenceHumanVerified 表示经过人工批准的游戏语义证据 / ConfidenceHumanVerified indicates human-approved game-semantic evidence
	ConfidenceHumanVerified = HumanReviewPrefix + ConfidenceVerified
	// ConfidenceHumanSerializationOnly 表示经过人工批准但仅覆盖序列化布局的证据 / ConfidenceHumanSerializationOnly indicates human-approved evidence limited to serialization layout
	ConfidenceHumanSerializationOnly = HumanReviewPrefix + ConfidenceSerializationOnly

	// SourceKindGame 标识来自游戏源码的证据 / SourceKindGame identifies evidence from game source
	SourceKindGame = "game_source"
	// SourceKindImplementation 标识来自本库实现源码的证据 / SourceKindImplementation identifies evidence from this library's implementation source
	SourceKindImplementation = "implementation_source"
)

// Document 表示一个已解析且带稳定元数据的有效格式指南文档 / Document represents one resolved effective format-guide document with stable metadata
type Document struct {
	// FormatID 是指南对应的稳定格式标识符 / FormatID is the stable format identifier associated with the guide
	FormatID string
	// Version 是指南目录中声明的文档版本 / Version is the document version declared by the guide catalog
	Version string
	// ID 是指南文档的规范标识符 / ID is the canonical identifier of the guide document
	ID string
	// MediaType 是指南 JSON 文档的媒体类型 / MediaType is the media type of the guide JSON document
	MediaType string
	// SHA256 是指南 JSON 字节的十六进制 SHA-256 摘要 / SHA256 is the hexadecimal SHA-256 digest of the guide JSON bytes
	SHA256 string
	// SchemaID 是指南对应的编辑模式规范标识符 / SchemaID is the canonical editing-schema identifier associated with the guide
	SchemaID string
	// Coverage 是指南的最高语义覆盖等级 / Coverage is the highest semantic coverage level of the guide
	Coverage string
	// JSON 是可直接提供给调用方的指南 JSON / JSON is guide JSON ready to be supplied directly to callers
	JSON []byte
}

// Guide 描述某个格式的字段语义、证据、规则和编辑工作流 / Guide describes field semantics, evidence, rules, and editing workflow for one format
type Guide struct {
	// ID 是格式指南文档的规范标识符 / ID is the canonical identifier of the format-guide document
	ID string `json:"$id"`
	// FormatID 是指南描述的稳定格式标识符 / FormatID is the stable format identifier described by the guide
	FormatID string `json:"format_id"`
	// Version 是格式指南契约版本 / Version is the format-guide contract version
	Version string `json:"guide_version"`
	// SchemaURI 是通过传输读取对应编辑模式的资源 URI / SchemaURI is the resource URI used by transports to read the corresponding editing schema
	SchemaURI string `json:"schema_uri"`
	// SchemaID 是对应编辑模式的规范标识符 / SchemaID is the canonical identifier of the corresponding editing schema
	SchemaID string `json:"schema_id,omitempty"`
	// Title 是面向编辑者的格式标题 / Title is the editor-facing title of the format
	Title string `json:"title"`
	// Summary 概述格式用途和编辑范围 / Summary summarizes the format purpose and editing scope
	Summary string `json:"summary"`
	// Coverage 描述指南审核等级和覆盖说明 / Coverage describes the guide review level and coverage notes
	Coverage Coverage `json:"coverage"`
	// Sources 汇总指南使用的源码证据 / Sources summarizes source evidence used by the guide
	Sources []Source `json:"sources,omitempty"`
	// Fields 描述具有精确 JSON 路径的字段 / Fields describes fields with exact JSON paths
	Fields []Field `json:"fields,omitempty"`
	// FieldPatterns 描述适用于动态键或数组元素的字段模式 / FieldPatterns describes field patterns for dynamic keys or array elements
	FieldPatterns []FieldPattern `json:"field_patterns,omitempty"`
	// Rules 保存跨字段或保留性编辑规则 / Rules contains cross-field or preservation editing rules
	Rules []Rule `json:"editing_rules,omitempty"`
	// Commands 描述文本命令的语法和游戏语义 / Commands describes text-command syntax and game semantics
	Commands []Command `json:"command_semantics,omitempty"`
	// ValueSets 描述游戏源码中审核过的枚举值集合 / ValueSets describes enum value sets reviewed from game source
	ValueSets []ValueSet `json:"value_sets,omitempty"`
	// Invariants 列出编辑时必须保持的格式不变量 / Invariants lists format invariants that editing must preserve
	Invariants []string `json:"invariants,omitempty"`
	// Workflow 给出建议的无损编辑步骤 / Workflow provides the recommended lossless editing steps
	Workflow []string `json:"editing_workflow,omitempty"`
	// Warnings 列出可能导致数据损坏或语义变化的风险 / Warnings lists risks that may corrupt data or alter semantics
	Warnings []string `json:"warnings,omitempty"`
}

// Coverage 描述格式指南的审核范围和成熟度 / Coverage describes the review scope and maturity of a format guide
type Coverage struct {
	// Level 是指南整体使用的覆盖等级标识符 / Level is the coverage-level identifier used by the guide as a whole
	Level string `json:"level"`
	// ReviewedFields 是具有游戏或序列化源码证据的字段数量 / ReviewedFields is the number of fields carrying game or serialization source evidence
	ReviewedFields int `json:"reviewed_fields,omitempty"`
	// Notes 说明覆盖边界和未审核区域 / Notes explains coverage boundaries and unreviewed areas
	Notes string `json:"notes,omitempty"`
}

// Source 标识支持指南结论的精确源码位置和观察结果 / Source identifies an exact source location and observation supporting a guide conclusion
type Source struct {
	// Kind 区分游戏源码证据和本库实现证据 / Kind distinguishes game-source evidence from library-implementation evidence
	Kind string `json:"kind"`
	// GameVersion 是证据对应的游戏或工具版本 / GameVersion is the game or tool version associated with the evidence
	GameVersion string `json:"game_version"`
	// Path 是证据文件相对于所述源码树的路径 / Path is the evidence file path relative to the referenced source tree
	Path string `json:"path"`
	// Symbol 是证据所在的类型、方法或成员名称 / Symbol is the type, method, or member name containing the evidence
	Symbol string `json:"symbol,omitempty"`
	// LineStart 是证据范围的首行号 / LineStart is the first line number of the evidence range
	LineStart int `json:"line_start,omitempty"`
	// LineEnd 是证据范围的末行号 / LineEnd is the last line number of the evidence range
	LineEnd int `json:"line_end,omitempty"`
	// Observation 记录从该源码位置确认的事实 / Observation records the fact confirmed from the source location
	Observation string `json:"observation"`
}

// Field 描述单个精确编辑 JSON 路径的游戏语义和编辑建议 / Field describes game semantics and editing guidance for one exact editing JSON path
type Field struct {
	// JSONPath 是指南中定位字段的 JSONPath 表达式 / JSONPath is the JSONPath expression locating the field in the guide
	JSONPath string `json:"json_path"`
	// SchemaPointer 是对应编辑模式节点的 JSON Pointer / SchemaPointer is the JSON Pointer of the corresponding editing-schema node
	SchemaPointer string `json:"schema_pointer,omitempty"`
	// Title 是面向编辑者的简短字段名称 / Title is the short editor-facing field name
	Title string `json:"title"`
	// Description 描述字段保存的数据 / Description describes the data stored by the field
	Description string `json:"description"`
	// GameUsage 说明游戏运行时如何读取或使用字段 / GameUsage explains how game runtime code reads or uses the field
	GameUsage string `json:"game_usage"`
	// EditRole 说明字段在常见 MOD 编辑中的作用 / EditRole explains the field's role in common MOD editing
	EditRole string `json:"edit_role"`
	// EditGuidance 给出修改或保留字段的具体建议 / EditGuidance gives concrete advice for modifying or preserving the field
	EditGuidance string `json:"edit_guidance"`
	// Risk 描述错误编辑字段可能造成的影响 / Risk describes the impact of editing the field incorrectly
	Risk string `json:"risk,omitempty"`
	// Constraints 列出来自模式或源码的附加约束 / Constraints lists additional constraints derived from schema or source
	Constraints []string `json:"constraints,omitempty"`
	// Recommended 是由源码审核得到的可选建议数值范围 / Recommended is an optional recommended numeric range derived from source review
	Recommended *Range `json:"recommended_range,omitempty"`
	// EnumValues 列出已审核整数值的名称和含义 / EnumValues lists names and meanings for reviewed integer values
	EnumValues []EnumValue `json:"enum_values,omitempty"`
	// Evidence 是支持字段语义说明的源码证据 / Evidence is source evidence supporting the field semantics
	Evidence []Source `json:"evidence,omitempty"`
	// Confidence 表示字段语义结论的证据等级 / Confidence indicates the evidence level of the field-semantic conclusion
	Confidence string `json:"confidence"`
}

// FieldPattern 描述一组动态编辑 JSON 路径共享的游戏语义和编辑建议 / FieldPattern describes shared game semantics and editing guidance for a set of dynamic editing JSON paths
type FieldPattern struct {
	// JSONPathPattern 是匹配动态键或数组元素的路径模式 / JSONPathPattern is the path pattern matching dynamic keys or array elements
	JSONPathPattern string `json:"json_path_pattern"`
	// Title 是面向编辑者的简短字段模式名称 / Title is the short editor-facing field-pattern name
	Title string `json:"title"`
	// Description 描述匹配字段保存的数据 / Description describes the data stored by matching fields
	Description string `json:"description"`
	// GameUsage 说明游戏运行时如何读取或使用匹配字段 / GameUsage explains how game runtime code reads or uses matching fields
	GameUsage string `json:"game_usage"`
	// EditRole 说明匹配字段在常见 MOD 编辑中的作用 / EditRole explains the role of matching fields in common MOD editing
	EditRole string `json:"edit_role"`
	// EditGuidance 给出修改或保留匹配字段的具体建议 / EditGuidance gives concrete advice for modifying or preserving matching fields
	EditGuidance string `json:"edit_guidance"`
	// Constraints 列出来自模式或源码的附加约束 / Constraints lists additional constraints derived from schema or source
	Constraints []string `json:"constraints,omitempty"`
	// Evidence 是支持字段模式语义说明的源码证据 / Evidence is source evidence supporting the field-pattern semantics
	Evidence []Source `json:"evidence,omitempty"`
	// Confidence 表示字段模式语义结论的证据等级 / Confidence indicates the evidence level of the field-pattern semantic conclusion
	Confidence string `json:"confidence"`
}

// Range 描述字段建议使用的闭区间数值范围 / Range describes a recommended closed numeric range for a field
type Range struct {
	// Minimum 是建议范围包含的最小值 / Minimum is the inclusive minimum of the recommended range
	Minimum float64 `json:"minimum"`
	// Maximum 是建议范围包含的最大值 / Maximum is the inclusive maximum of the recommended range
	Maximum float64 `json:"maximum"`
}

// EnumValue 描述整数枚举值的源码名称和游戏含义 / EnumValue describes the source name and game meaning of an integer enum value
type EnumValue struct {
	// Value 是序列化到编辑 JSON 的整数值 / Value is the integer value serialized into editing JSON
	Value int `json:"value"`
	// Name 是游戏源码中的规范枚举名称 / Name is the canonical enum name in game source
	Name string `json:"name"`
	// Meaning 说明该枚举值在游戏中的行为 / Meaning explains the in-game behavior of the enum value
	Meaning string `json:"meaning"`
}

// Rule 描述编辑时必须遵守的跨字段或保留性规则 / Rule describes a cross-field or preservation rule that editing must follow
type Rule struct {
	// ID 是规则在指南中的稳定标识符 / ID is the stable identifier of the rule within the guide
	ID string `json:"id"`
	// AppliesTo 列出规则适用的 JSON 路径或模式 / AppliesTo lists JSON paths or patterns to which the rule applies
	AppliesTo []string `json:"applies_to,omitempty"`
	// Severity 表示违反规则的风险等级 / Severity indicates the risk level of violating the rule
	Severity string `json:"severity"`
	// Summary 是规则的简短说明 / Summary is the short description of the rule
	Summary string `json:"summary"`
	// Details 给出执行规则所需的完整说明 / Details provides the complete instructions needed to apply the rule
	Details string `json:"details"`
	// Evidence 是支持规则的源码证据 / Evidence is source evidence supporting the rule
	Evidence []Source `json:"evidence,omitempty"`
}

// Command 描述 COM3D2 菜单文本命令的语法集合和运行时效果 / Command describes syntax forms and runtime effects of a COM3D2 menu text command
type Command struct {
	// Name 是命令的规范关键字 / Name is the canonical keyword of the command
	Name string `json:"name"`
	// Aliases 是游戏接受的其他命令关键字 / Aliases contains alternate command keywords accepted by the game
	Aliases []string `json:"aliases,omitempty"`
	// Contexts 列出命令可以出现的菜单执行上下文 / Contexts lists menu execution contexts in which the command may appear
	Contexts []string `json:"contexts"`
	// Forms 列出由源码审核过的参数形式 / Forms contains argument forms reviewed against source
	Forms []CommandForm `json:"forms"`
	// GameEffect 说明命令对游戏状态或资源的影响 / GameEffect explains the command's effect on game state or resources
	GameEffect string `json:"game_effect"`
	// EditGuidance 给出安全创建或修改命令的建议 / EditGuidance gives advice for safely creating or modifying the command
	EditGuidance string `json:"edit_guidance"`
	// Risk 描述错误命令可能造成的影响 / Risk describes the impact of an incorrect command
	Risk string `json:"risk"`
	// Evidence 是支持命令语义和形式的游戏源码证据 / Evidence is game-source evidence supporting command semantics and forms
	Evidence []Source `json:"evidence,omitempty"`
}

// CommandForm 描述命令在特定源码上下文中审核过的单个语法形式 / CommandForm describes one command syntax form reviewed in specific source contexts
type CommandForm struct {
	// Syntax 是面向编辑者的命令形式表达式 / Syntax is the editor-facing expression of the command form
	Syntax string `json:"syntax"`
	// ReviewedIn 列出确认该形式的运行时解析上下文 / ReviewedIn lists runtime parsing contexts that confirm the form
	ReviewedIn []string `json:"reviewed_in"`
	// Arguments 按位置描述命令参数 / Arguments describes command arguments by position
	Arguments []CommandArgument `json:"arguments,omitempty"`
	// Notes 保存该形式特有的附加行为说明 / Notes contains additional behavior notes specific to the form
	Notes string `json:"notes,omitempty"`
}

// CommandArgument 描述菜单命令形式中的单个位置参数 / CommandArgument describes one positional argument in a menu command form
type CommandArgument struct {
	// Position 是参数在命令令牌中的一基位置 / Position is the one-based position of the argument in command tokens
	Position int `json:"position"`
	// Name 是面向编辑者的参数名称 / Name is the editor-facing argument name
	Name string `json:"name"`
	// Type 是参数值的语义类型 / Type is the semantic type of the argument value
	Type string `json:"type"`
	// ValueSetRefs 引用适用于参数的已审核值集合 / ValueSetRefs references reviewed value sets applicable to the argument
	ValueSetRefs []string `json:"value_set_refs,omitempty"`
	// Required 表示命令形式是否要求该参数 / Required reports whether the command form requires the argument
	Required bool `json:"required"`
	// Repeatable 表示参数是否可以重复到命令末尾 / Repeatable reports whether the argument may repeat through the end of the command
	Repeatable bool `json:"repeatable,omitempty"`
	// Default 是省略可选参数时游戏使用的文本值 / Default is the textual value used by the game when an optional argument is omitted
	Default string `json:"default,omitempty"`
	// AllowedValues 列出源码明确限制的文本值 / AllowedValues lists textual values explicitly constrained by source
	AllowedValues []string `json:"allowed_values,omitempty"`
	// Description 说明参数如何影响命令行为 / Description explains how the argument affects command behavior
	Description string `json:"description"`
}

// ValueSet 描述从游戏源码审核得到的具名数值集合 / ValueSet describes a named numeric value set reviewed from game source
type ValueSet struct {
	// ID 是指南内引用值集合的稳定标识符 / ID is the stable identifier used to reference the value set within a guide
	ID string `json:"id"`
	// CSharpType 是游戏源码中定义值集合的类型名称 / CSharpType is the type name defining the value set in game source
	CSharpType string `json:"csharp_type"`
	// Description 概述值集合在游戏中的用途 / Description summarizes the value set's purpose in the game
	Description string `json:"description"`
	// EditGuidance 说明编辑者应如何选择和保留值 / EditGuidance explains how editors should select and preserve values
	EditGuidance string `json:"edit_guidance"`
	// ReviewedIn 列出确认值集合使用方式的运行时上下文 / ReviewedIn lists runtime contexts confirming how the value set is used
	ReviewedIn []string `json:"reviewed_in"`
	// Values 按数值列出源码名称 / Values lists source names by numeric value
	Values []ValueSetValue `json:"values"`
	// Evidence 是支持值集合定义和用途的源码证据 / Evidence is source evidence supporting value-set definition and usage
	Evidence []Source `json:"evidence,omitempty"`
	// Confidence 表示值集合结论的证据等级 / Confidence indicates the evidence level of the value-set conclusion
	Confidence string `json:"confidence"`
}

// ValueSetValue 将游戏源码名称映射到精确整数值 / ValueSetValue maps a game-source name to an exact integer value
type ValueSetValue struct {
	// Name 是游戏源码中的规范名称 / Name is the canonical name in game source
	Name string `json:"name"`
	// Number 是名称对应的精确整数值 / Number is the exact integer value associated with the name
	Number int `json:"number"`
}

// Lookup 按格式标识符编码经审核的指南 profile 并返回稳定文档元数据
// Lookup encodes a reviewed guide profile by format identifier and returns stable document metadata
func Lookup(formatID string) (Document, bool, error) {
	id := strings.ToLower(strings.TrimSpace(formatID))
	if id == "" || strings.ContainsAny(id, `/\\`) {
		return Document{}, false, nil
	}
	if profile, found := profileGuide(id); found {
		data, err := json.MarshalIndent(profile, "", "  ")
		if err != nil {
			return Document{}, false, fmt.Errorf("encode format-guide profile %s: %w", id, err)
		}
		data = append(data, '\n')
		digest := sha256.Sum256(data)
		return Document{
			FormatID:  id,
			Version:   profile.Version,
			ID:        profile.ID,
			MediaType: MediaType,
			SHA256:    fmt.Sprintf("%x", digest[:]),
			SchemaID:  profile.SchemaID,
			Coverage:  profile.Coverage.Level,
			JSON:      data,
		}, true, nil
	}
	return Document{}, false, nil
}

// Decode 查找并将格式指南 JSON 解码为强类型指南模型
// Decode looks up and decodes format-guide JSON into the strongly typed guide model
func Decode(formatID string) (Guide, bool, error) {
	document, found, err := Lookup(formatID)
	if err != nil || !found {
		return Guide{}, found, err
	}
	var guide Guide
	if err := json.Unmarshal(document.JSON, &guide); err != nil {
		return Guide{}, false, fmt.Errorf("decode guide %s: %w", formatID, err)
	}
	return guide, true, nil
}

// Formats 返回按字典序排列的全部已注册格式指南标识符
// Formats returns all registered format-guide identifiers in lexical order
func Formats() ([]string, error) {
	return profileFormats(), nil
}

// IsHumanReviewed 判断覆盖或置信状态是否带有明确的人工批准前缀
// IsHumanReviewed reports whether a coverage or confidence state carries the explicit human-approval prefix
func IsHumanReviewed(state string) bool {
	return strings.HasPrefix(state, HumanReviewPrefix)
}
