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
		StorageVariant: storageVariant,
		Kind:           descriptor.ExportCMKind,
		JSON:           compact,
	}, nil
}

// encodeExportCMPayload 校验扩展名、类型和存储变体后编码 ExportCM JSON 旁车
// encodeExportCMPayload validates the extension, kind, and storage variant before encoding an ExportCM JSON sidecar
func encodeExportCMPayload(env *KCESPayloadEnvelope, storageVariant string) ([]byte, error) {
	ext := NormalizeKCESPayloadExtension(env.Extension)

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

// editableExportCMJSON 校验并返回规范化的 ExportCM JSON 语义内容
// editableExportCMJSON validates and returns normalized semantic ExportCM JSON content
func editableExportCMJSON(env *KCESPayloadEnvelope) ([]byte, error) {
	if len(env.JSON) == 0 {
		return nil, fmt.Errorf("ExportCM envelope json is required")
	}
	if !utf8.Valid(env.JSON) {
		return nil, fmt.Errorf("ExportCM envelope json is not valid UTF-8")
	}
	compactJSON, err := compactExportCMJSON(env.JSON)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), compactJSON...), nil
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
