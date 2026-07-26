package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// DSLConfService 专门处理 .dslconf 文件 / DSLConfService handles .dslconf files
type DSLConfService struct{}

// ReadDSLConfFile 读取并解码 .dslconf 文件
// ReadDSLConfFile reads and decodes a .dslconf file
func (s *DSLConfService) ReadDSLConfFile(path string) (*serializationKCES.KCESPayloadEnvelope, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSLConfExtension {
		return nil, fmt.Errorf("not a .dslconf file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .dslconf file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESPayload(data, serializationKCES.KCESDSLConfExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .dslconf file %q: %w", path, err)
	}
	return value, nil
}

// WriteDSLConfFile 编码并写入 .dslconf 文件
// WriteDSLConfFile encodes and writes a .dslconf file
func (s *DSLConfService) WriteDSLConfFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSLConfExtension {
		return fmt.Errorf("not a .dslconf output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESPayloadExtension(value.Extension) != serializationKCES.KCESDSLConfExtension {
		return fmt.Errorf(".dslconf output requires a .dslconf KCES payload envelope")
	}
	encoded, err := serializationKCES.EncodeKCESPayload(value)
	if err != nil {
		return fmt.Errorf("encode .dslconf file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .dslconf file %q: %w", path, err)
	}
	return nil
}

// ConvertDSLConfToJson 将 .dslconf 文件转换为编辑 JSON
// ConvertDSLConfToJson converts a .dslconf file to editing JSON
func (s *DSLConfService) ConvertDSLConfToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadDSLConfFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .dslconf JSON: %w", err)
	}
	return writeDSLConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToDSLConf 将编辑 JSON 转换为 .dslconf 文件
// ConvertJsonToDSLConf converts editing JSON to a .dslconf file
func (s *DSLConfService) ConvertJsonToDSLConf(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .dslconf JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .dslconf JSON"); err != nil {
		return fmt.Errorf("parse .dslconf JSON: %w", err)
	}
	if value.Format != serializationKCES.PayloadFormatKCESMessagePack && value.Format != serializationKCES.PayloadFormatKCESExportCM {
		return fmt.Errorf("unsupported .dslconf JSON format %q", value.Format)
	}
	actual := serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESDSLConfExtension
	} else if actual != serializationKCES.KCESDSLConfExtension {
		return fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, serializationKCES.KCESDSLConfExtension)
	}
	encoded, err := serializationKCES.EncodeKCESPayload(&value)
	if err != nil {
		return fmt.Errorf("encode .dslconf file: %w", err)
	}
	return writeDSLConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeDSLConfConversionOutput 在上下文有效且大小不超限时写入 .dslconf 转换结果
// writeDSLConfConversionOutput writes .dslconf conversion output while the context is active and the size remains within the limit
func writeDSLConfConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .dslconf conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .dslconf conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .dslconf conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
