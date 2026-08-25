package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// UndressDataService 专门处理 .undressdat 文件 / UndressDataService handles .undressdat files
type UndressDataService struct{}

// IsKCESUndressDataFile 判断路径是否为 .undressdat 文件
// IsKCESUndressDataFile reports whether a path names a .undressdat file
func IsKCESUndressDataFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && serializationKCES.NormalizeKCESJSONTextExtension(path) == serializationKCES.KCESUndressDataExtension
}

// IsKCESUndressDataJSONFile 判断路径是否为 .undressdat 编辑 JSON
// IsKCESUndressDataJSONFile reports whether a path names .undressdat editing JSON
func IsKCESUndressDataJSONFile(path string) bool {
	return miscExtFromJSONPath(path) == serializationKCES.KCESUndressDataExtension
}

// ReadUndressDataFile 读取并解码 .undressdat 文件
// ReadUndressDataFile reads and decodes a .undressdat file
func (s *UndressDataService) ReadUndressDataFile(path string) (json.RawMessage, error) {
	if serializationKCES.NormalizeKCESJSONTextExtension(path) != serializationKCES.KCESUndressDataExtension {
		return nil, fmt.Errorf("not a .undressdat file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .undressdat file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESJSONText(data, serializationKCES.KCESUndressDataExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .undressdat file %q: %w", path, err)
	}
	return value, nil
}

// WriteUndressDataFile 编码并写入 .undressdat 文件
// WriteUndressDataFile encodes and writes a .undressdat file
func (s *UndressDataService) WriteUndressDataFile(path string, value json.RawMessage) error {
	if serializationKCES.NormalizeKCESJSONTextExtension(path) != serializationKCES.KCESUndressDataExtension {
		return fmt.Errorf("not a .undressdat output path: %s", path)
	}
	encoded, err := serializationKCES.EncodeKCESJSONText(value, serializationKCES.KCESUndressDataExtension)
	if err != nil {
		return fmt.Errorf("encode .undressdat file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .undressdat file %q: %w", path, err)
	}
	return nil
}

// ConvertUndressDataToJson 将 .undressdat 文件转换为编辑 JSON
// ConvertUndressDataToJson converts a .undressdat file to editing JSON
func (s *UndressDataService) ConvertUndressDataToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadUndressDataFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .undressdat JSON: %w", err)
	}
	return writeUndressDataConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToUndressData 将编辑 JSON 转换为 .undressdat 文件
// ConvertJsonToUndressData converts editing JSON to a .undressdat file
func (s *UndressDataService) ConvertJsonToUndressData(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .undressdat JSON %q: %w", inputPath, err)
	}
	var value json.RawMessage
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .undressdat JSON"); err != nil {
		return fmt.Errorf("parse .undressdat JSON: %w", err)
	}
	encoded, err := serializationKCES.EncodeKCESJSONText(value, serializationKCES.KCESUndressDataExtension)
	if err != nil {
		return fmt.Errorf("encode .undressdat file: %w", err)
	}
	return writeUndressDataConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeUndressDataConversionOutput 在上下文有效且大小不超限时写入 .undressdat 转换结果
// writeUndressDataConversionOutput writes .undressdat conversion output while the context is active and the size remains within the limit
func writeUndressDataConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .undressdat conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .undressdat conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .undressdat conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
