package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
)

// 多个新版物理配置扩展名共用的 MessagePack JSON 字符串模型
// MessagePack JSON-string model shared by multiple newer physics configuration extensions

// decodeJSONStringMessagePack 解码扩展名声明的原生 MessagePack JSON 字符串载荷
// decodeJSONStringMessagePack decodes the native MessagePack JSON-string payload declared by an extension
func decodeJSONStringMessagePack(data []byte, descriptor kcesPayloadDescriptor) (*KCESPayloadEnvelope, error) {
	var text *string
	if err := decodeKCESMessagePackRoot(data, descriptor, &text); err != nil {
		return nil, fmt.Errorf("decode JSON string payload: %w", err)
	}
	envelope := newKCESMessagePackEnvelope(descriptor)
	if text == nil {
		return envelope, nil
	}
	if !json.Valid([]byte(*text)) {
		return nil, fmt.Errorf("decode JSON string payload: inner Magica JSON is invalid")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(*text)); err != nil {
		return nil, fmt.Errorf("compact inner Magica JSON: %w", err)
	}
	envelope.JSON = append(json.RawMessage(nil), compact.Bytes()...)
	return envelope, nil
}

// encodeJSONStringMessagePack 编码扩展名声明的原生 MessagePack JSON 字符串载荷
// encodeJSONStringMessagePack encodes the native MessagePack JSON-string payload declared by an extension
func encodeJSONStringMessagePack(env *KCESPayloadEnvelope, descriptor kcesPayloadDescriptor) ([]byte, error) {
	var data []byte
	var err error
	if env.JSON == nil {
		data, err = msgpack.EncodeMsgpack(nil)
	} else {
		text, selectErr := editableMessagePackJSONString(env)
		if selectErr != nil {
			return nil, selectErr
		}
		data, err = msgpack.EncodeMsgpack(text)
	}
	if err != nil {
		return nil, fmt.Errorf("encode JSON string payload: %w", err)
	}
	return encodeKCESMessagePackRoot(data, descriptor)
}

// editableMessagePackJSONString 将 MessagePack 字符串载荷中的 JSON 语义内容规范化为紧凑字符串
// editableMessagePackJSONString normalizes semantic JSON content from a MessagePack string payload into a compact string
func editableMessagePackJSONString(env *KCESPayloadEnvelope) (string, error) {
	if !utf8.Valid(env.JSON) {
		return "", fmt.Errorf("json payload is not valid UTF-8")
	}
	var compactJSON bytes.Buffer
	if err := json.Compact(&compactJSON, env.JSON); err != nil {
		return "", fmt.Errorf("json payload is invalid: %w", err)
	}
	return compactJSON.String(), nil
}
