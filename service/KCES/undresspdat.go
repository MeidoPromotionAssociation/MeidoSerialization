package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// UndressPartsDataService 专门处理 .undresspdat 文件 / UndressPartsDataService handles .undresspdat files
type UndressPartsDataService struct{}

// IsKCESUndressPartsDataFile 判断路径是否为 .undresspdat 文件
// IsKCESUndressPartsDataFile reports whether a path names a .undresspdat file
func IsKCESUndressPartsDataFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && serializationKCES.NormalizeKCESJSONTextExtension(path) == serializationKCES.KCESUndressPartsDataExtension
}

// IsKCESUndressPartsDataJSONFile 判断路径是否为 .undresspdat 编辑 JSON
// IsKCESUndressPartsDataJSONFile reports whether a path names .undresspdat editing JSON
func IsKCESUndressPartsDataJSONFile(path string) bool {
	return miscExtFromJSONPath(path) == serializationKCES.KCESUndressPartsDataExtension
}

// ReadUndressPartsDataFile 读取并解码 .undresspdat 文件
// ReadUndressPartsDataFile reads and decodes a .undresspdat file
func (s *UndressPartsDataService) ReadUndressPartsDataFile(path string) (json.RawMessage, error) {
	if serializationKCES.NormalizeKCESJSONTextExtension(path) != serializationKCES.KCESUndressPartsDataExtension {
		return nil, fmt.Errorf("not a .undresspdat file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .undresspdat file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESJSONText(data, serializationKCES.KCESUndressPartsDataExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .undresspdat file %q: %w", path, err)
	}
	return value, nil
}

// WriteUndressPartsDataFile 编码并写入 .undresspdat 文件
// WriteUndressPartsDataFile encodes and writes a .undresspdat file
func (s *UndressPartsDataService) WriteUndressPartsDataFile(path string, value json.RawMessage) error {
	if serializationKCES.NormalizeKCESJSONTextExtension(path) != serializationKCES.KCESUndressPartsDataExtension {
		return fmt.Errorf("not a .undresspdat output path: %s", path)
	}
	encoded, err := serializationKCES.EncodeKCESJSONText(value, serializationKCES.KCESUndressPartsDataExtension)
	if err != nil {
		return fmt.Errorf("encode .undresspdat file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .undresspdat file %q: %w", path, err)
	}
	return nil
}

// ConvertUndressPartsDataToJson 将 .undresspdat 文件转换为编辑 JSON
// ConvertUndressPartsDataToJson converts a .undresspdat file to editing JSON
func (s *UndressPartsDataService) ConvertUndressPartsDataToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadUndressPartsDataFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .undresspdat JSON: %w", err)
	}
	return writeUndressPartsDataConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToUndressPartsData 将编辑 JSON 转换为 .undresspdat 文件
// ConvertJsonToUndressPartsData converts editing JSON to a .undresspdat file
func (s *UndressPartsDataService) ConvertJsonToUndressPartsData(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .undresspdat JSON %q: %w", inputPath, err)
	}
	var value json.RawMessage
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .undresspdat JSON"); err != nil {
		return fmt.Errorf("parse .undresspdat JSON: %w", err)
	}
	encoded, err := serializationKCES.EncodeKCESJSONText(value, serializationKCES.KCESUndressPartsDataExtension)
	if err != nil {
		return fmt.Errorf("encode .undresspdat file: %w", err)
	}
	return writeUndressPartsDataConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeUndressPartsDataConversionOutput 在上下文有效且大小不超限时写入 .undresspdat 转换结果
// writeUndressPartsDataConversionOutput writes .undresspdat conversion output while the context is active and the size remains within the limit
func writeUndressPartsDataConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .undresspdat conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .undresspdat conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .undresspdat conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
