package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// KCES ExportCM 旁车 JSON 线格式的调度实现，实际扩展名能力由对应扩展名文件声明
// Dispatcher for KCES ExportCM sidecar JSON wires, with each extension file declaring whether that wire variant is available

// isExportCMPayloadExtension 判断扩展名是否声明了 ExportCM JSON 线格式
// isExportCMPayloadExtension reports whether an extension declares an ExportCM JSON wire format
func isExportCMPayloadExtension(extension string) bool {
	descriptor, ok := kcesPayloadDescriptorByExtension[NormalizeKCESPayloadExtension(extension)]
	return ok && descriptor.ExportCMKind != ""
}

// decodeExportCMPayload 按扩展名声明的存储变体解码 ExportCM JSON 旁车
// decodeExportCMPayload decodes an ExportCM JSON sidecar using the storage variant declared for its extension
func decodeExportCMPayload(data []byte, extension string) (*KCESPayloadEnvelope, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	descriptor, ok := kcesPayloadDescriptorByExtension[ext]
	if !ok || descriptor.ExportCMKind == "" {
		return nil, fmt.Errorf("extension %q has no ExportCM JSON wire", ext)
	}
	storageVariant := descriptor.ExportCMStorageVariant
	jsonData := data
	if storageVariant == PayloadStorageExportCMDotNetStringJSON {
		var err error
		jsonData, err = decodeExportCMDotNetString(data)
		if err != nil {
			return nil, fmt.Errorf("decode ExportCM BinaryWriter string: %w", err)
		}
	}
	if !utf8.Valid(jsonData) {
		return nil, fmt.Errorf("ExportCM JSON is not valid UTF-8")
	}

	compact, err := compactExportCMJSON(jsonData)
	if err != nil {
		return nil, err
	}
	return &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESExportCM,
		Extension:      ext,
		LengthPrefixed: false,
		StorageVariant: storageVariant,
		Kind:           descriptor.ExportCMKind,
		Text:           string(jsonData),
		JSON:           compact,
	}, nil
}

// encodeExportCMPayload 校验扩展名、类型和存储变体后编码 ExportCM JSON 旁车
// encodeExportCMPayload validates the extension, kind, and storage variant before encoding an ExportCM JSON sidecar
func encodeExportCMPayload(env *KCESPayloadEnvelope, storageVariant string) ([]byte, error) {
	ext := NormalizeKCESPayloadExtension(env.Extension)
	if env.LengthPrefixed {
		return nil, fmt.Errorf("ExportCM storageVariant %q does not use the int32 lengthPrefixed wire", storageVariant)
	}

	descriptor, ok := kcesPayloadDescriptorByExtension[ext]
	if !ok || descriptor.ExportCMKind == "" {
		return nil, fmt.Errorf("extension %q has no ExportCM JSON wire", ext)
	}
	expectedKind := descriptor.ExportCMKind
	expectedStorage := descriptor.ExportCMStorageVariant
	if storageVariant != expectedStorage {
		return nil, fmt.Errorf("extension %q requires ExportCM storageVariant %q, got %q", ext, expectedStorage, storageVariant)
	}
	if env.Kind != expectedKind {
		return nil, fmt.Errorf("extension %q with storageVariant %q requires kind %q, got %q", ext, storageVariant, expectedKind, env.Kind)
	}

	jsonData, err := editableExportCMJSON(env)
	if err != nil {
		return nil, err
	}

	if storageVariant == PayloadStorageExportCMDotNetStringJSON {
		return encodeExportCMDotNetString(jsonData)
	}
	return append([]byte(nil), jsonData...), nil
}

// editableExportCMJSON 选择应写出的 JSON 字节，并在编辑视图未变化时复用原始文本
// editableExportCMJSON selects the JSON bytes to write and reuses the original text when the editable view is unchanged
func editableExportCMJSON(env *KCESPayloadEnvelope) ([]byte, error) {
	if len(env.JSON) != 0 {
		if !utf8.Valid(env.JSON) {
			return nil, fmt.Errorf("ExportCM envelope json is not valid UTF-8")
		}
		compactJSON, err := compactExportCMJSON(env.JSON)
		if err != nil {
			return nil, err
		}

		// Text 是解码器捕获的准确旁车字符串，JSON 是可编辑解析视图，语义未变化时逐字节保留原始空白和可选 UTF-8 BOM，仅在实际编辑后改用紧凑 JSON
		// Text is the exact sidecar string captured by the decoder and JSON is its editable parsed view, preserving original whitespace and an optional UTF-8 BOM byte-for-byte when semantically unchanged and using compact JSON only after an actual edit
		if env.Text != "" && utf8.ValidString(env.Text) {
			compactText, textErr := compactExportCMJSON([]byte(env.Text))
			if textErr == nil && bytes.Equal(compactText, compactJSON) {
				return []byte(env.Text), nil
			}
		}
		return append([]byte(nil), compactJSON...), nil
	}
	if !utf8.ValidString(env.Text) {
		return nil, fmt.Errorf("ExportCM envelope text is not valid UTF-8")
	}
	if env.Text == "" {
		return nil, fmt.Errorf("ExportCM envelope json or text is required")
	}
	text := []byte(env.Text)
	if _, err := compactExportCMJSON(text); err != nil {
		return nil, err
	}
	return text, nil
}

// compactExportCMJSON 去除可选 UTF-8 BOM 并返回校验后的紧凑 JSON
// compactExportCMJSON removes an optional UTF-8 BOM and returns validated compact JSON
func compactExportCMJSON(data []byte) (json.RawMessage, error) {
	data = trimExportCMUTF8BOM(data)
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return nil, fmt.Errorf("ExportCM JSON is invalid: %w", err)
	}
	return append(json.RawMessage(nil), compact.Bytes()...), nil
}

// decodeExportCMDotNetString 解码占满输入的 BinaryWriter UTF-8 字符串
// decodeExportCMDotNetString decodes a BinaryWriter UTF-8 string that must consume the entire input
func decodeExportCMDotNetString(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)
	value, err := binaryio.ReadString(r)
	if err != nil {
		return nil, err
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("BinaryWriter string has %d trailing bytes", r.Len())
	}
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("BinaryWriter string is not valid UTF-8")
	}
	return []byte(value), nil
}

// encodeExportCMDotNetString 将 UTF-8 JSON 编码为 BinaryWriter 字符串
// encodeExportCMDotNetString encodes UTF-8 JSON as a BinaryWriter string
func encodeExportCMDotNetString(value []byte) ([]byte, error) {
	if !utf8.Valid(value) {
		return nil, fmt.Errorf("ExportCM BinaryWriter string is not valid UTF-8")
	}
	var out bytes.Buffer
	if err := binaryio.WriteString(&out, string(value)); err != nil {
		return nil, fmt.Errorf("write ExportCM BinaryWriter string: %w", err)
	}
	return out.Bytes(), nil
}

// trimExportCMUTF8BOM 去除 ExportCM JSON 开头的可选 UTF-8 BOM
// trimExportCMUTF8BOM removes an optional UTF-8 BOM from the beginning of ExportCM JSON
func trimExportCMUTF8BOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return data[3:]
	}
	return data
}
