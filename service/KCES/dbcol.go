package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// DBColService 专门处理 .dbcol 文件 / DBColService handles .dbcol files
type DBColService struct{}

// ReadDBColFile 读取并解码 .dbcol 文件
// ReadDBColFile reads and decodes a .dbcol file
func (s *DBColService) ReadDBColFile(path string) (*serializationKCES.KCESPayloadEnvelope, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDBColExtension {
		return nil, fmt.Errorf("not a .dbcol file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .dbcol file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeDBCol(data)
	if err != nil {
		return nil, fmt.Errorf("decode .dbcol file %q: %w", path, err)
	}
	return value, nil
}

// WriteDBColFile 编码并写入 .dbcol 文件
// WriteDBColFile encodes and writes a .dbcol file
func (s *DBColService) WriteDBColFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDBColExtension {
		return fmt.Errorf("not a .dbcol output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESPayloadExtension(value.Extension) != serializationKCES.KCESDBColExtension {
		return fmt.Errorf(".dbcol output requires a .dbcol KCES payload envelope")
	}
	encoded, err := serializationKCES.EncodeDBCol(value)
	if err != nil {
		return fmt.Errorf("encode .dbcol file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .dbcol file %q: %w", path, err)
	}
	return nil
}

// ConvertDBColToJson 将 .dbcol 文件转换为编辑 JSON
// ConvertDBColToJson converts a .dbcol file to editing JSON
func (s *DBColService) ConvertDBColToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadDBColFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .dbcol JSON: %w", err)
	}
	return writeDBColConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToDBCol 将编辑 JSON 转换为 .dbcol 文件
// ConvertJsonToDBCol converts editing JSON to a .dbcol file
func (s *DBColService) ConvertJsonToDBCol(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .dbcol JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .dbcol JSON"); err != nil {
		return fmt.Errorf("parse .dbcol JSON: %w", err)
	}
	if value.Format != serializationKCES.PayloadFormatKCESMessagePack && value.Format != serializationKCES.PayloadFormatKCESExportCM {
		return fmt.Errorf("unsupported .dbcol JSON format %q", value.Format)
	}
	actual := serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESDBColExtension
	} else if actual != serializationKCES.KCESDBColExtension {
		return fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, serializationKCES.KCESDBColExtension)
	}
	encoded, err := serializationKCES.EncodeDBCol(&value)
	if err != nil {
		return fmt.Errorf("encode .dbcol file: %w", err)
	}
	return writeDBColConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeDBColConversionOutput 在上下文有效且大小不超限时写入 .dbcol 转换结果
// writeDBColConversionOutput writes .dbcol conversion output while the context is active and the size remains within the limit
func writeDBColConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .dbcol conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .dbcol conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .dbcol conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
