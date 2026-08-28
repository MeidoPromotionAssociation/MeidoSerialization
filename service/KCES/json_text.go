package KCES

import (
	"encoding/json"
	"fmt"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

// WriteJSONTextFile 为聚合 service 保留 KCES 明文 JSON 资源直接写入 API
// WriteJSONTextFile preserves the direct KCES plain-JSON resource writer on the aggregate service
func (s *MiscService) WriteJSONTextFile(path string, value json.RawMessage) error {
	switch serializationKCES.NormalizeKCESJSONTextExtension(path) {
	case serializationKCES.KCESNSONExtension:
		return (&NSONService{}).WriteNSONFile(path, value)
	default:
		return fmt.Errorf("unsupported KCES JSON-text output path %q", path)
	}
}

// encodeKCESJSONTextJSON 严格解码编辑 JSON 文档并编码指定扩展名的明文 JSON 资源
// encodeKCESJSONTextJSON strictly decodes an editing JSON document and encodes the plain-JSON resource for the requested extension
func encodeKCESJSONTextJSON(data []byte, expectedExtension string) ([]byte, error) {
	value, err := decodeKCESJSONTextEditingJSON(trimJSONUTF8BOM(data), expectedExtension)
	if err != nil {
		return nil, fmt.Errorf("parse %s json: %w", expectedExtension, err)
	}
	return serializationKCES.EncodeKCESJSONText(value, expectedExtension)
}

// decodeKCESJSONTextEditingJSON 严格解析 KCES 明文 JSON 资源的编辑 JSON 文档
// 编辑 JSON 的根就是该资源的 JSON 文档本身，目标扩展名完全由文件名决定
// decodeKCESJSONTextEditingJSON strictly parses the editing JSON document of a KCES plain-JSON resource
// The editing JSON root is the resource's JSON document itself and the destination extension is decided entirely by the file name
func decodeKCESJSONTextEditingJSON(data []byte, expectedExtension string) (json.RawMessage, error) {
	expected := serializationKCES.NormalizeKCESJSONTextExtension(expectedExtension)
	if expected == "" {
		return nil, fmt.Errorf("unsupported KCES JSON-text extension %q", expectedExtension)
	}
	var value json.RawMessage
	if err := decodeStrictJSON(data, &value, "KCES JSON-text document"); err != nil {
		return nil, err
	}
	if _, err := serializationKCES.EncodeKCESJSONText(value, expected); err != nil {
		return nil, err
	}
	return value, nil
}
