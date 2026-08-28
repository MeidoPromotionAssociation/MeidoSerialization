package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

// DBConfService 专门处理 .dbconf 文件 / DBConfService handles .dbconf files
type DBConfService struct{}

// ReadDBConfFile 读取并解码 .dbconf 文件
// ReadDBConfFile reads and decodes a .dbconf file
func (s *DBConfService) ReadDBConfFile(path string) (*serializationKCES.DynamicBoneStatus, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDBConfExtension {
		return nil, fmt.Errorf("not a .dbconf file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .dbconf file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeDBConf(data)
	if err != nil {
		return nil, fmt.Errorf("decode .dbconf file %q: %w", path, err)
	}
	return value, nil
}

// WriteDBConfFile 编码并写入 .dbconf 文件
// WriteDBConfFile encodes and writes a .dbconf file
func (s *DBConfService) WriteDBConfFile(path string, value *serializationKCES.DynamicBoneStatus) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDBConfExtension {
		return fmt.Errorf("not a .dbconf output path: %s", path)
	}
	encoded, err := serializationKCES.EncodeDBConf(value)
	if err != nil {
		return fmt.Errorf("encode .dbconf file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .dbconf file %q: %w", path, err)
	}
	return nil
}

// ConvertDBConfToJson 将 .dbconf 文件转换为编辑 JSON
// ConvertDBConfToJson converts a .dbconf file to editing JSON
func (s *DBConfService) ConvertDBConfToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadDBConfFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .dbconf JSON: %w", err)
	}
	return writeDBConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToDBConf 将编辑 JSON 转换为 .dbconf 文件
// ConvertJsonToDBConf converts editing JSON to a .dbconf file
func (s *DBConfService) ConvertJsonToDBConf(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .dbconf JSON %q: %w", inputPath, err)
	}
	var value *serializationKCES.DynamicBoneStatus
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .dbconf JSON"); err != nil {
		return fmt.Errorf("parse .dbconf JSON: %w", err)
	}
	encoded, err := serializationKCES.EncodeDBConf(value)
	if err != nil {
		return fmt.Errorf("encode .dbconf file: %w", err)
	}
	return writeDBConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeDBConfConversionOutput 在上下文有效且大小不超限时写入 .dbconf 转换结果
// writeDBConfConversionOutput writes .dbconf conversion output while the context is active and the size remains within the limit
func writeDBConfConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .dbconf conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .dbconf conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .dbconf conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
