package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// KCES 明文 JSON 扩展名的统一语义编解码与调度层，支持列表由各扩展名文件声明
// Unified semantic codec and dispatcher for KCES plain-JSON extensions, with each extension file declaring its support descriptor

// kcesJSONTextDescriptor 描述一个受支持的 KCES 明文 JSON 扩展名
// kcesJSONTextDescriptor describes one supported KCES plain-JSON extension
type kcesJSONTextDescriptor struct {
	Extension string // 文件扩展名 / File extension
}

var kcesJSONTextDescriptors = [...]kcesJSONTextDescriptor{
	nsonJSONTextDescriptor,
}

var kcesJSONTextDescriptorByExtension = func() map[string]kcesJSONTextDescriptor {
	result := make(map[string]kcesJSONTextDescriptor, len(kcesJSONTextDescriptors))
	for _, descriptor := range kcesJSONTextDescriptors {
		if descriptor.Extension == "" {
			panic("KCES JSON-text descriptor has an empty extension")
		}
		if _, exists := result[descriptor.Extension]; exists {
			panic("duplicate KCES JSON-text descriptor for " + descriptor.Extension)
		}
		result[descriptor.Extension] = descriptor
	}
	return result
}()

// 这些扩展名的原生文件本身就是 JSON 文本，编辑 JSON 的根就是该文档本身，没有额外封套：
// 目标格式完全由文件名决定，而本库不为这些资源声明任何领域结构
//
// 原生文件同样是 JSON 文本、但本库已经建模了完整领域结构的扩展名不在这里注册，
// 见 unity_json_document.go
//
// The native file of these extensions is already JSON text, so the editing JSON root is that document
// itself with no surrounding envelope: the destination format is determined entirely by the file name,
// and this library declares no domain structure for these resources
//
// Extensions whose native file is also JSON text but whose full domain structure this library does
// model are not registered here; see unity_json_document.go

// DecodeKCESJSONText 校验并解码受支持扩展名的明文 JSON，仅保留 JSON 语义内容
// DecodeKCESJSONText validates and decodes plain JSON for a supported extension while retaining only its semantic JSON content
func DecodeKCESJSONText(data []byte, extension string) (json.RawMessage, error) {
	ext := NormalizeKCESJSONTextExtension(extension)
	if ext == "" {
		return nil, fmt.Errorf("unsupported KCES JSON text extension %q", extension)
	}

	trimmed := bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("%s is not valid JSON", ext)
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return nil, fmt.Errorf("compact %s JSON: %w", ext, err)
	}

	return append(json.RawMessage(nil), compact.Bytes()...), nil
}

// EncodeKCESJSONText 将 JSON 语义内容写为稳定缩进的 UTF-8 JSON
// EncodeKCESJSONText writes semantic JSON content as stably indented UTF-8 JSON
func EncodeKCESJSONText(value json.RawMessage, extension string) ([]byte, error) {
	ext := NormalizeKCESJSONTextExtension(extension)
	if ext == "" {
		return nil, fmt.Errorf("unsupported KCES JSON text extension %q", extension)
	}
	if len(bytes.TrimSpace(value)) == 0 {
		return nil, fmt.Errorf("%s JSON document is empty", ext)
	}
	if !json.Valid(value) {
		return nil, fmt.Errorf("%s JSON document is invalid", ext)
	}
	var compactJSON bytes.Buffer
	if err := json.Compact(&compactJSON, value); err != nil {
		return nil, fmt.Errorf("compact %s JSON document: %w", ext, err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, compactJSON.Bytes(), "", "  "); err != nil {
		return nil, fmt.Errorf("indent %s JSON: %w", ext, err)
	}
	indented.WriteByte('\n')
	return indented.Bytes(), nil
}

// IsKCESJSONTextExtension 判断扩展名或路径是否属于受支持的 KCES 明文 JSON 格式
// IsKCESJSONTextExtension reports whether an extension or path belongs to a supported KCES plain-JSON format
func IsKCESJSONTextExtension(extension string) bool {
	return NormalizeKCESJSONTextExtension(extension) != ""
}

// NormalizeKCESJSONTextExtension 从路径或扩展名提取规范化的受支持扩展名
// NormalizeKCESJSONTextExtension extracts a normalized supported extension from a path or extension
func NormalizeKCESJSONTextExtension(pathOrExt string) string {
	lower := strings.ToLower(strings.TrimSpace(filepath.ToSlash(pathOrExt)))
	if lower == "" {
		return ""
	}
	ext := filepath.Ext(lower)
	if _, ok := kcesJSONTextDescriptorByExtension[ext]; ok {
		return ext
	}
	return ""
}
