package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// DSB2ConfService 专门处理 .dsb2conf 文件 / DSB2ConfService handles .dsb2conf files
type DSB2ConfService struct{}

// ReadDSB2ConfFile 读取并解码 .dsb2conf 文件
// ReadDSB2ConfFile reads and decodes a .dsb2conf file
func (s *DSB2ConfService) ReadDSB2ConfFile(path string) (*serializationKCES.KCESPayloadEnvelope, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSB2ConfExtension {
		return nil, fmt.Errorf("not a .dsb2conf file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .dsb2conf file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESPayload(data, serializationKCES.KCESDSB2ConfExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .dsb2conf file %q: %w", path, err)
	}
	return value, nil
}

// WriteDSB2ConfFile 编码并写入 .dsb2conf 文件
// WriteDSB2ConfFile encodes and writes a .dsb2conf file
func (s *DSB2ConfService) WriteDSB2ConfFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSB2ConfExtension {
		return fmt.Errorf("not a .dsb2conf output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESPayloadExtension(value.Extension) != serializationKCES.KCESDSB2ConfExtension {
		return fmt.Errorf(".dsb2conf output requires a .dsb2conf KCES payload envelope")
	}
	encoded, err := serializationKCES.EncodeKCESPayload(value)
	if err != nil {
		return fmt.Errorf("encode .dsb2conf file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .dsb2conf file %q: %w", path, err)
	}
	return nil
}

// ConvertDSB2ConfToJson 将 .dsb2conf 文件转换为编辑 JSON
// ConvertDSB2ConfToJson converts a .dsb2conf file to editing JSON
func (s *DSB2ConfService) ConvertDSB2ConfToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadDSB2ConfFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .dsb2conf JSON: %w", err)
	}
	return writeDSB2ConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToDSB2Conf 将编辑 JSON 转换为 .dsb2conf 文件
// ConvertJsonToDSB2Conf converts editing JSON to a .dsb2conf file
func (s *DSB2ConfService) ConvertJsonToDSB2Conf(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .dsb2conf JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .dsb2conf JSON"); err != nil {
		return fmt.Errorf("parse .dsb2conf JSON: %w", err)
	}
	if value.Format != serializationKCES.PayloadFormatKCESMessagePack && value.Format != serializationKCES.PayloadFormatKCESExportCM {
		return fmt.Errorf("unsupported .dsb2conf JSON format %q", value.Format)
	}
	actual := serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESDSB2ConfExtension
	} else if actual != serializationKCES.KCESDSB2ConfExtension {
		return fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, serializationKCES.KCESDSB2ConfExtension)
	}
	encoded, err := serializationKCES.EncodeKCESPayload(&value)
	if err != nil {
		return fmt.Errorf("encode .dsb2conf file: %w", err)
	}
	return writeDSB2ConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeDSB2ConfConversionOutput 在上下文有效且大小不超限时写入 .dsb2conf 转换结果
// writeDSB2ConfConversionOutput writes .dsb2conf conversion output while the context is active and the size remains within the limit
func writeDSB2ConfConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .dsb2conf conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .dsb2conf conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .dsb2conf conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
