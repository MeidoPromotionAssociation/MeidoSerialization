package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// NSONService 专门处理 .nson 文件 / NSONService handles .nson files
type NSONService struct{}

// IsKCESNSONFile 判断路径是否为 .nson 文件
// IsKCESNSONFile reports whether a path names a .nson file
func IsKCESNSONFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && serializationKCES.NormalizeKCESJSONTextExtension(path) == serializationKCES.KCESNSONExtension
}

// IsKCESNSONJSONFile 判断路径是否为 .nson 编辑 JSON
// IsKCESNSONJSONFile reports whether a path names .nson editing JSON
func IsKCESNSONJSONFile(path string) bool {
	return miscExtFromJSONPath(path) == serializationKCES.KCESNSONExtension
}

// ReadNSONFile 读取并解码 .nson 文件
// ReadNSONFile reads and decodes a .nson file
func (s *NSONService) ReadNSONFile(path string) (*serializationKCES.KCESJSONText, error) {
	if serializationKCES.NormalizeKCESJSONTextExtension(path) != serializationKCES.KCESNSONExtension {
		return nil, fmt.Errorf("not a .nson file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .nson file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESJSONText(data, serializationKCES.KCESNSONExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .nson file %q: %w", path, err)
	}
	return value, nil
}

// WriteNSONFile 编码并写入 .nson 文件
// WriteNSONFile encodes and writes a .nson file
func (s *NSONService) WriteNSONFile(path string, value *serializationKCES.KCESJSONText) error {
	if serializationKCES.NormalizeKCESJSONTextExtension(path) != serializationKCES.KCESNSONExtension {
		return fmt.Errorf("not a .nson output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESJSONTextExtension(value.Extension) != serializationKCES.KCESNSONExtension {
		return fmt.Errorf(".nson output requires a .nson KCES JSON-text value")
	}
	encoded, err := serializationKCES.EncodeKCESJSONText(value)
	if err != nil {
		return fmt.Errorf("encode .nson file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .nson file %q: %w", path, err)
	}
	return nil
}

// ConvertNSONToJson 将 .nson 文件转换为编辑 JSON
// ConvertNSONToJson converts a .nson file to editing JSON
func (s *NSONService) ConvertNSONToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadNSONFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .nson JSON: %w", err)
	}
	return writeNSONConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToNSON 将编辑 JSON 转换为 .nson 文件
// ConvertJsonToNSON converts editing JSON to a .nson file
func (s *NSONService) ConvertJsonToNSON(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .nson JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESJSONText
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .nson JSON"); err != nil {
		return fmt.Errorf("parse .nson JSON: %w", err)
	}
	actual := serializationKCES.NormalizeKCESJSONTextExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESNSONExtension
	} else if actual != serializationKCES.KCESNSONExtension {
		return fmt.Errorf("KCES JSON-text envelope extension %q does not match file extension %q", actual, serializationKCES.KCESNSONExtension)
	}
	encoded, err := serializationKCES.EncodeKCESJSONText(&value)
	if err != nil {
		return fmt.Errorf("encode .nson file: %w", err)
	}
	return writeNSONConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeNSONConversionOutput 在上下文有效且大小不超限时写入 .nson 转换结果
// writeNSONConversionOutput writes .nson conversion output while the context is active and the size remains within the limit
func writeNSONConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .nson conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .nson conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .nson conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
