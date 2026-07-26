package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// DSLColService 专门处理 .dslcol 文件 / DSLColService handles .dslcol files
type DSLColService struct{}

// ReadDSLColFile 读取并解码 .dslcol 文件
// ReadDSLColFile reads and decodes a .dslcol file
func (s *DSLColService) ReadDSLColFile(path string) (*serializationKCES.KCESPayloadEnvelope, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSLColExtension {
		return nil, fmt.Errorf("not a .dslcol file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .dslcol file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESPayload(data, serializationKCES.KCESDSLColExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .dslcol file %q: %w", path, err)
	}
	return value, nil
}

// WriteDSLColFile 编码并写入 .dslcol 文件
// WriteDSLColFile encodes and writes a .dslcol file
func (s *DSLColService) WriteDSLColFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSLColExtension {
		return fmt.Errorf("not a .dslcol output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESPayloadExtension(value.Extension) != serializationKCES.KCESDSLColExtension {
		return fmt.Errorf(".dslcol output requires a .dslcol KCES payload envelope")
	}
	encoded, err := serializationKCES.EncodeKCESPayload(value)
	if err != nil {
		return fmt.Errorf("encode .dslcol file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .dslcol file %q: %w", path, err)
	}
	return nil
}

// ConvertDSLColToJson 将 .dslcol 文件转换为编辑 JSON
// ConvertDSLColToJson converts a .dslcol file to editing JSON
func (s *DSLColService) ConvertDSLColToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadDSLColFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .dslcol JSON: %w", err)
	}
	return writeDSLColConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToDSLCol 将编辑 JSON 转换为 .dslcol 文件
// ConvertJsonToDSLCol converts editing JSON to a .dslcol file
func (s *DSLColService) ConvertJsonToDSLCol(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .dslcol JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .dslcol JSON"); err != nil {
		return fmt.Errorf("parse .dslcol JSON: %w", err)
	}
	if value.Format != serializationKCES.PayloadFormatKCESMessagePack && value.Format != serializationKCES.PayloadFormatKCESExportCM {
		return fmt.Errorf("unsupported .dslcol JSON format %q", value.Format)
	}
	actual := serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESDSLColExtension
	} else if actual != serializationKCES.KCESDSLColExtension {
		return fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, serializationKCES.KCESDSLColExtension)
	}
	encoded, err := serializationKCES.EncodeKCESPayload(&value)
	if err != nil {
		return fmt.Errorf("encode .dslcol file: %w", err)
	}
	return writeDSLColConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeDSLColConversionOutput 在上下文有效且大小不超限时写入 .dslcol 转换结果
// writeDSLColConversionOutput writes .dslcol conversion output while the context is active and the size remains within the limit
func writeDSLColConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .dslcol conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .dslcol conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .dslcol conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
