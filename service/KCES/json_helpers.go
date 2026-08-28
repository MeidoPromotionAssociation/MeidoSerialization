package KCES

import (
	"bytes"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/strictjson"
)

// trimJSONUTF8BOM 删除 editing JSON 开头可选的 UTF-8 BOM
// trimJSONUTF8BOM removes an optional UTF-8 BOM at the start of editing JSON
func trimJSONUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
}

// decodeStrictJSON 使用本包 editing schema 解码唯一完整 UTF-8 JSON 值，editing JSON 不是前向兼容线格式，未知字段或错误 null 不能被静默丢弃
// decodeStrictJSON decodes one complete UTF-8 JSON value using this package's editing schema because editing JSON is not a forward-compatible wire format and unknown fields or invalid null values must not be silently dropped
func decodeStrictJSON(data []byte, out interface{}, name string) error {
	data = trimJSONUTF8BOM(data)
	if err := strictjson.Decode(data, out); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
