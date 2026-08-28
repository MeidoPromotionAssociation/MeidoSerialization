package application

import (
	"fmt"
	"strings"

	editingv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/schemas/editing/v1"
)

// SchemaDocument 表示某个已注册格式的版本化编辑 JSON 契约 / SchemaDocument represents the versioned editing JSON contract for one registered format
type SchemaDocument struct {
	// FormatID 是模式对应的稳定格式标识符 / FormatID is the stable format identifier associated with the schema
	FormatID string
	// Representation 是该模式描述的应用层表示形式 / Representation is the application representation described by the schema
	Representation Representation
	// Version 是模式文档版本 / Version is the schema document version
	Version string
	// ID 是模式文档的规范标识符 / ID is the canonical identifier of the schema document
	ID string
	// Dialect 是模式使用的 JSON Schema 方言 / Dialect is the JSON Schema dialect used by the document
	Dialect string
	// MediaType 是模式文档的媒体类型 / MediaType is the media type of the schema document
	MediaType string
	// SHA256 是模式 JSON 的 SHA-256 摘要 / SHA256 is the SHA-256 digest of the schema JSON
	SHA256 string
	// NativeSuffixes 是该格式支持的原生文件后缀 / NativeSuffixes contains the native file suffixes supported by the format
	NativeSuffixes []string
	// JSON 保存可直接用于校验或代码生成的模式 JSON / JSON contains schema JSON ready for validation or code generation
	JSON []byte
}

// FormatSchema 解析格式标识符并返回已发布的编辑 JSON 模式
// FormatSchema resolves a format identifier and returns its published editing JSON schema
func (r *Registry) FormatSchema(formatID string) (SchemaDocument, error) {
	formatID = strings.ToLower(strings.TrimSpace(formatID))
	if formatID == "" {
		return SchemaDocument{}, opError("get format schema", CodeInvalidArgument, fmt.Errorf("format ID is required"))
	}
	format, ok := r.Lookup(formatID)
	if !ok {
		return SchemaDocument{}, opError("get format schema", CodeNotFound, fmt.Errorf("format %q is not registered", formatID))
	}
	if !format.Capability.Convert {
		return SchemaDocument{}, opError("get format schema", CodeUnsupported, fmt.Errorf("format %q has no editing JSON representation", format.ID))
	}
	document, found, err := editingv1.Lookup(format.ID)
	if err != nil {
		return SchemaDocument{}, opError("get format schema", CodeInternal, err)
	}
	if !found {
		return SchemaDocument{}, opError("get format schema", CodeUnsupported, fmt.Errorf("format %q has no published editing schema", format.ID))
	}
	return SchemaDocument{
		FormatID:       document.FormatID,
		Representation: RepresentationEditingJSON,
		Version:        document.Version,
		ID:             document.ID,
		Dialect:        document.Dialect,
		MediaType:      document.MediaType,
		SHA256:         document.SHA256,
		NativeSuffixes: append([]string(nil), format.NativeSuffixes...),
		JSON:           append([]byte(nil), document.JSON...),
	}, nil
}

// GetFormatSchema 返回指定格式的模式并作为 FormatSchema 的兼容别名
// GetFormatSchema returns the schema for a format as a compatibility alias of FormatSchema
func (r *Registry) GetFormatSchema(formatID string) (SchemaDocument, error) {
	return r.FormatSchema(formatID)
}

// FormatSchema 通过引擎注册表返回指定格式的编辑 JSON 模式
// FormatSchema returns the editing JSON schema for a format through the engine registry
func (e *Engine) FormatSchema(formatID string) (SchemaDocument, error) {
	if e == nil || e.registry == nil {
		return SchemaDocument{}, opError("get format schema", CodeInternal, fmt.Errorf("engine is not initialized"))
	}
	return e.registry.FormatSchema(formatID)
}

// GetFormatSchema 返回指定格式的模式并作为 FormatSchema 的兼容别名
// GetFormatSchema returns the schema for a format as a compatibility alias of FormatSchema
func (e *Engine) GetFormatSchema(formatID string) (SchemaDocument, error) {
	return e.FormatSchema(formatID)
}
