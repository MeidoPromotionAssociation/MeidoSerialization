package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// DSBConfService 专门处理 .dsbconf 文件 / DSBConfService handles .dsbconf files
type DSBConfService struct{}

// ReadDSBConfFile 读取并解码 .dsbconf 文件
// ReadDSBConfFile reads and decodes a .dsbconf file
func (s *DSBConfService) ReadDSBConfFile(path string) (*serializationKCES.KCESPayloadEnvelope, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSBConfExtension {
		return nil, fmt.Errorf("not a .dsbconf file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .dsbconf file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESPayload(data, serializationKCES.KCESDSBConfExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .dsbconf file %q: %w", path, err)
	}
	return value, nil
}

// WriteDSBConfFile 编码并写入 .dsbconf 文件
// WriteDSBConfFile encodes and writes a .dsbconf file
func (s *DSBConfService) WriteDSBConfFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSBConfExtension {
		return fmt.Errorf("not a .dsbconf output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESPayloadExtension(value.Extension) != serializationKCES.KCESDSBConfExtension {
		return fmt.Errorf(".dsbconf output requires a .dsbconf KCES payload envelope")
	}
	encoded, err := serializationKCES.EncodeKCESPayload(value)
	if err != nil {
		return fmt.Errorf("encode .dsbconf file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .dsbconf file %q: %w", path, err)
	}
	return nil
}

// ConvertDSBConfToJson 将 .dsbconf 文件转换为编辑 JSON
// ConvertDSBConfToJson converts a .dsbconf file to editing JSON
func (s *DSBConfService) ConvertDSBConfToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadDSBConfFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .dsbconf JSON: %w", err)
	}
	return writeDSBConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToDSBConf 将编辑 JSON 转换为 .dsbconf 文件
// ConvertJsonToDSBConf converts editing JSON to a .dsbconf file
func (s *DSBConfService) ConvertJsonToDSBConf(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .dsbconf JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .dsbconf JSON"); err != nil {
		return fmt.Errorf("parse .dsbconf JSON: %w", err)
	}
	if value.Format != serializationKCES.PayloadFormatKCESMessagePack && value.Format != serializationKCES.PayloadFormatKCESExportCM {
		return fmt.Errorf("unsupported .dsbconf JSON format %q", value.Format)
	}
	actual := serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESDSBConfExtension
	} else if actual != serializationKCES.KCESDSBConfExtension {
		return fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, serializationKCES.KCESDSBConfExtension)
	}
	encoded, err := serializationKCES.EncodeKCESPayload(&value)
	if err != nil {
		return fmt.Errorf("encode .dsbconf file: %w", err)
	}
	return writeDSBConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeDSBConfConversionOutput 在上下文有效且大小不超限时写入 .dsbconf 转换结果
// writeDSBConfConversionOutput writes .dsbconf conversion output while the context is active and the size remains within the limit
func writeDSBConfConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .dsbconf conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .dsbconf conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .dsbconf conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
