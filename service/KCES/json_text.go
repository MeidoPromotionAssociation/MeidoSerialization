package KCES

import (
	"encoding/json"
	"fmt"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// WriteJSONTextFile 为聚合 service 保留 KCES 明文 JSON 资源直接写入 API
// WriteJSONTextFile preserves the direct KCES plain-JSON resource writer on the aggregate service
func (s *MiscService) WriteJSONTextFile(path string, value *serializationKCES.KCESJSONText) error {
	switch serializationKCES.NormalizeKCESJSONTextExtension(path) {
	case serializationKCES.KCESUndressDataExtension:
		return (&UndressDataService{}).WriteUndressDataFile(path, value)
	case serializationKCES.KCESUndressPartsDataExtension:
		return (&UndressPartsDataService{}).WriteUndressPartsDataFile(path, value)
	case serializationKCES.KCESNSONExtension:
		return (&NSONService{}).WriteNSONFile(path, value)
	default:
		return fmt.Errorf("unsupported KCES JSON-text output path %q", path)
	}
}

// encodeKCESJSONTextJSON 严格解码编辑封套并编码指定扩展名的明文 JSON 资源
// encodeKCESJSONTextJSON strictly decodes an editing envelope and encodes the plain-JSON resource for the requested extension
func encodeKCESJSONTextJSON(data []byte, expectedExtension string) ([]byte, error) {
	value, err := decodeKCESJSONTextEditingJSON(trimJSONUTF8BOM(data), expectedExtension)
	if err != nil {
		return nil, fmt.Errorf("parse %s json: %w", expectedExtension, err)
	}
	return serializationKCES.EncodeKCESJSONText(value)
}

// decodeKCESJSONTextEditingJSON 严格解析 KCES 明文 JSON 编辑封套并校验双扩展名
// decodeKCESJSONTextEditingJSON strictly parses a KCES plain-JSON editing envelope and validates its double extension
func decodeKCESJSONTextEditingJSON(data []byte, expectedExtension string) (*serializationKCES.KCESJSONText, error) {
	var editing struct {
		Extension *string         `json:"extension"`
		JSON      json.RawMessage `json:"json"`
	}
	if err := decodeStrictJSON(data, &editing, "KCES JSON-text envelope"); err != nil {
		return nil, err
	}
	expected := serializationKCES.NormalizeKCESJSONTextExtension(expectedExtension)
	if expected == "" {
		return nil, fmt.Errorf("unsupported KCES JSON-text extension %q", expectedExtension)
	}
	extension := expected
	if editing.Extension != nil && strings.TrimSpace(*editing.Extension) != "" {
		extension = serializationKCES.NormalizeKCESJSONTextExtension(*editing.Extension)
		if extension == "" {
			return nil, fmt.Errorf("unsupported KCES JSON-text envelope extension %q", *editing.Extension)
		}
		if extension != expected {
			return nil, fmt.Errorf("KCES JSON-text envelope extension %q does not match file extension %q", extension, expected)
		}
	}
	if len(editing.JSON) == 0 {
		return nil, fmt.Errorf("json payload is missing")
	}
	value := &serializationKCES.KCESJSONText{
		Extension: extension,
		JSON:      append(json.RawMessage(nil), editing.JSON...),
	}
	if _, err := serializationKCES.EncodeKCESJSONText(value); err != nil {
		return nil, err
	}
	return value, nil
}
