package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// IKColService 专门处理 .ikcol 文件 / IKColService handles .ikcol files
type IKColService struct{}

// ReadIKColFile 读取并解码 .ikcol 文件
// ReadIKColFile reads and decodes a .ikcol file
func (s *IKColService) ReadIKColFile(path string) (*serializationKCES.KCESPayloadEnvelope, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESIKColExtension {
		return nil, fmt.Errorf("not a .ikcol file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .ikcol file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESPayload(data, serializationKCES.KCESIKColExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .ikcol file %q: %w", path, err)
	}
	return value, nil
}

// WriteIKColFile 编码并写入 .ikcol 文件
// WriteIKColFile encodes and writes a .ikcol file
func (s *IKColService) WriteIKColFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESIKColExtension {
		return fmt.Errorf("not a .ikcol output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESPayloadExtension(value.Extension) != serializationKCES.KCESIKColExtension {
		return fmt.Errorf(".ikcol output requires a .ikcol KCES payload envelope")
	}
	encoded, err := serializationKCES.EncodeKCESPayload(value)
	if err != nil {
		return fmt.Errorf("encode .ikcol file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .ikcol file %q: %w", path, err)
	}
	return nil
}

// ConvertIKColToJson 将 .ikcol 文件转换为编辑 JSON
// ConvertIKColToJson converts a .ikcol file to editing JSON
func (s *IKColService) ConvertIKColToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadIKColFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .ikcol JSON: %w", err)
	}
	return writeIKColConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToIKCol 将编辑 JSON 转换为 .ikcol 文件
// ConvertJsonToIKCol converts editing JSON to a .ikcol file
func (s *IKColService) ConvertJsonToIKCol(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .ikcol JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .ikcol JSON"); err != nil {
		return fmt.Errorf("parse .ikcol JSON: %w", err)
	}
	if value.Format != serializationKCES.PayloadFormatKCESMessagePack && value.Format != serializationKCES.PayloadFormatKCESExportCM {
		return fmt.Errorf("unsupported .ikcol JSON format %q", value.Format)
	}
	actual := serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESIKColExtension
	} else if actual != serializationKCES.KCESIKColExtension {
		return fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, serializationKCES.KCESIKColExtension)
	}
	encoded, err := serializationKCES.EncodeKCESPayload(&value)
	if err != nil {
		return fmt.Errorf("encode .ikcol file: %w", err)
	}
	return writeIKColConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeIKColConversionOutput 在上下文有效且大小不超限时写入 .ikcol 转换结果
// writeIKColConversionOutput writes .ikcol conversion output while the context is active and the size remains within the limit
func writeIKColConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .ikcol conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .ikcol conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .ikcol conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
