package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

var jsonTextExts = map[string]struct{}{
	".undressdat":  {},
	".undresspdat": {},
}

// KCESJSONText 表示 KCES 明文 JSON 资源的封套 / KCESJSONText represents an envelope for KCES plain JSON resources
type KCESJSONText struct {
	Extension string          `json:"extension"` // 原始扩展名，如 .undressdat / Original extension such as .undressdat
	JSON      json.RawMessage `json:"json"`      // 规范化后的 JSON 内容 / Normalized JSON content
}

func DecodeKCESJSONText(data []byte, extension string) (*KCESJSONText, error) {
	ext := NormalizeKCESJSONTextExtension(extension)
	if ext == "" {
		return nil, fmt.Errorf("unsupported KCES JSON text extension %q", extension)
	}

	trimmed := bytes.TrimSpace(data)
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("%s is not valid JSON", ext)
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return nil, fmt.Errorf("compact %s JSON: %w", ext, err)
	}

	return &KCESJSONText{
		Extension: ext,
		JSON:      append(json.RawMessage(nil), compact.Bytes()...),
	}, nil
}

func EncodeKCESJSONText(value *KCESJSONText) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES JSON text")
	}
	ext := NormalizeKCESJSONTextExtension(value.Extension)
	if ext == "" {
		return nil, fmt.Errorf("unsupported KCES JSON text extension %q", value.Extension)
	}
	if len(bytes.TrimSpace(value.JSON)) == 0 {
		return nil, fmt.Errorf("%s JSON payload is empty", ext)
	}
	if !json.Valid(value.JSON) {
		return nil, fmt.Errorf("%s JSON payload is invalid", ext)
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, value.JSON, "", "  "); err != nil {
		return nil, fmt.Errorf("indent %s JSON: %w", ext, err)
	}
	indented.WriteByte('\n')
	return indented.Bytes(), nil
}

func IsKCESJSONTextExtension(extension string) bool {
	return NormalizeKCESJSONTextExtension(extension) != ""
}

func NormalizeKCESJSONTextExtension(pathOrExt string) string {
	lower := strings.ToLower(strings.TrimSpace(filepath.ToSlash(pathOrExt)))
	if lower == "" {
		return ""
	}
	ext := filepath.Ext(lower)
	if _, ok := jsonTextExts[ext]; ok {
		return ext
	}
	return ""
}
