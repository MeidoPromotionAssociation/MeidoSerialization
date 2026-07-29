package application

import (
	"fmt"
	"strings"

	knowledgev1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/knowledge/v1"
)

// GuideDocument 表示某个已注册格式的版本化编辑指南文档 / GuideDocument represents a versioned editing guide document for one registered format
type GuideDocument struct {
	// FormatID 是指南对应的稳定格式标识符 / FormatID is the stable format identifier associated with the guide
	FormatID string
	// Version 是指南文档版本 / Version is the guide document version
	Version string
	// ID 是指南文档的规范标识符 / ID is the canonical identifier of the guide document
	ID string
	// MediaType 是指南文档的媒体类型 / MediaType is the media type of the guide document
	MediaType string
	// SHA256 是指南 JSON 的 SHA-256 摘要 / SHA256 is the SHA-256 digest of the guide JSON
	SHA256 string
	// SchemaID 是指南引用的编辑模式标识符 / SchemaID is the editing schema identifier referenced by the guide
	SchemaID string
	// FormatVerification 描述整个文件格式的认证等级 / FormatVerification describes the verification level of the whole file format
	FormatVerification string
	// JSON 保存可直接交给调用方的指南 JSON / JSON contains the guide JSON ready for callers to consume
	JSON []byte
}

// FormatGuide 解析格式标识符并返回与其已发布编辑模式匹配的指南
// FormatGuide resolves a format identifier and returns the guide matching its published editing schema
func (r *Registry) FormatGuide(formatID string) (GuideDocument, error) {
	formatID = strings.ToLower(strings.TrimSpace(formatID))
	if formatID == "" {
		return GuideDocument{}, opError("get format guide", CodeInvalidArgument, fmt.Errorf("format ID is required"))
	}
	format, ok := r.Lookup(formatID)
	if !ok {
		return GuideDocument{}, opError("get format guide", CodeNotFound, fmt.Errorf("format %q is not registered", formatID))
	}
	if !format.Capability.Convert {
		return GuideDocument{}, opError("get format guide", CodeUnsupported, fmt.Errorf("format %q has no editing JSON representation", format.ID))
	}
	schema, err := r.FormatSchema(format.ID)
	if err != nil {
		return GuideDocument{}, err
	}
	document, err := knowledgev1.Resolve(format.ID, schema.ID, schema.JSON)
	if err != nil {
		return GuideDocument{}, opError("get format guide", CodeInternal, err)
	}
	return GuideDocument{
		FormatID: document.FormatID, Version: document.Version, ID: document.ID,
		MediaType: document.MediaType, SHA256: document.SHA256, SchemaID: document.SchemaID,
		FormatVerification: document.FormatVerification, JSON: append([]byte(nil), document.JSON...),
	}, nil
}

// GetFormatGuide 返回指定格式的指南并作为 FormatGuide 的兼容别名
// GetFormatGuide returns the guide for a format as a compatibility alias of FormatGuide
func (r *Registry) GetFormatGuide(formatID string) (GuideDocument, error) {
	return r.FormatGuide(formatID)
}

// FormatGuide 通过引擎注册表返回指定格式的指南
// FormatGuide returns the guide for a format through the engine registry
func (e *Engine) FormatGuide(formatID string) (GuideDocument, error) {
	if e == nil || e.registry == nil {
		return GuideDocument{}, opError("get format guide", CodeInternal, fmt.Errorf("engine is not initialized"))
	}
	return e.registry.FormatGuide(formatID)
}

// GetFormatGuide 返回指定格式的指南并作为 FormatGuide 的兼容别名
// GetFormatGuide returns the guide for a format as a compatibility alias of FormatGuide
func (e *Engine) GetFormatGuide(formatID string) (GuideDocument, error) {
	return e.FormatGuide(formatID)
}
