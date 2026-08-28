package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

// 多个新版物理配置扩展名共用的 MessagePack JSON 字符串模型，字符串内容是 MagicaCloth2 的
// ClothSerializeData Unity JSON 文档
// MessagePack JSON-string model shared by multiple newer physics configuration extensions, whose
// string content is the MagicaCloth2 ClothSerializeData Unity JSON document

// decodeJSONStringMessagePack 解码扩展名声明的原生 MessagePack JSON 字符串载荷
// decodeJSONStringMessagePack decodes the native MessagePack JSON-string payload declared by an extension
func decodeJSONStringMessagePack(data []byte, descriptor kcesPayloadDescriptor) (*MagicaClothSerializeData, error) {
	var text *string
	if err := decodeKCESMessagePackRoot(data, descriptor, &text); err != nil {
		return nil, fmt.Errorf("decode JSON string payload: %w", err)
	}
	if text == nil {
		return nil, nil
	}
	// 根为 JSON null 表示 MessagePack 的 nil 字符串，因此内层字面量 null 无法无损表达，只能拒绝
	// A JSON null root already represents the nil MessagePack string, so an inner literal null cannot round-trip and is rejected
	if bytes.Equal(bytes.TrimSpace([]byte(*text)), []byte("null")) {
		return nil, fmt.Errorf("decode inner MagicaCloth ClothSerializeData JSON: the stored string is the literal null, which is not a ClothSerializeData document")
	}
	var document MagicaClothSerializeData
	if err := decodeKCESJSONStrict([]byte(*text), &document); err != nil {
		return nil, fmt.Errorf("decode inner MagicaCloth ClothSerializeData JSON: %w", err)
	}
	return &document, nil
}

// encodeJSONStringMessagePack 编码扩展名声明的原生 MessagePack JSON 字符串载荷
// encodeJSONStringMessagePack encodes the native MessagePack JSON-string payload declared by an extension
func encodeJSONStringMessagePack(value *MagicaClothSerializeData, descriptor kcesPayloadDescriptor) ([]byte, error) {
	var data []byte
	var err error
	if value == nil {
		data, err = msgpack.EncodeMsgpack(nil)
	} else {
		text, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return nil, fmt.Errorf("encode inner MagicaCloth ClothSerializeData JSON: %w", marshalErr)
		}
		data, err = msgpack.EncodeMsgpack(string(text))
	}
	if err != nil {
		return nil, fmt.Errorf("encode JSON string payload: %w", err)
	}
	return encodeKCESMessagePackRoot(data, descriptor)
}
