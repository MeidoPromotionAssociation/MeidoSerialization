package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// IKColBytesService 专门处理 .ikcol.bytes 文件 / IKColBytesService handles .ikcol.bytes files
type IKColBytesService struct{}

// ReadIKColBytesFile 读取并解码 .ikcol.bytes 文件
// ReadIKColBytesFile reads and decodes a .ikcol.bytes file
func (s *IKColBytesService) ReadIKColBytesFile(path string) (*serializationKCES.KCESPayloadEnvelope, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESIKColBytesExtension {
		return nil, fmt.Errorf("not a .ikcol.bytes file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .ikcol.bytes file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESPayload(data, serializationKCES.KCESIKColBytesExtension)
	if err != nil {
		return nil, fmt.Errorf("decode .ikcol.bytes file %q: %w", path, err)
	}
	return value, nil
}

// WriteIKColBytesFile 编码并写入 .ikcol.bytes 文件
// WriteIKColBytesFile encodes and writes a .ikcol.bytes file
func (s *IKColBytesService) WriteIKColBytesFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESIKColBytesExtension {
		return fmt.Errorf("not a .ikcol.bytes output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESPayloadExtension(value.Extension) != serializationKCES.KCESIKColBytesExtension {
		return fmt.Errorf(".ikcol.bytes output requires a .ikcol.bytes KCES payload envelope")
	}
	encoded, err := serializationKCES.EncodeKCESPayload(value)
	if err != nil {
		return fmt.Errorf("encode .ikcol.bytes file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .ikcol.bytes file %q: %w", path, err)
	}
	return nil
}

// ConvertIKColBytesToJson 将 .ikcol.bytes 文件转换为编辑 JSON
// ConvertIKColBytesToJson converts a .ikcol.bytes file to editing JSON
func (s *IKColBytesService) ConvertIKColBytesToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadIKColBytesFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .ikcol.bytes JSON: %w", err)
	}
	return writeIKColBytesConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToIKColBytes 将编辑 JSON 转换为 .ikcol.bytes 文件
// ConvertJsonToIKColBytes converts editing JSON to a .ikcol.bytes file
func (s *IKColBytesService) ConvertJsonToIKColBytes(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .ikcol.bytes JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .ikcol.bytes JSON"); err != nil {
		return fmt.Errorf("parse .ikcol.bytes JSON: %w", err)
	}
	if value.Format != serializationKCES.PayloadFormatKCESMessagePack && value.Format != serializationKCES.PayloadFormatKCESExportCM {
		return fmt.Errorf("unsupported .ikcol.bytes JSON format %q", value.Format)
	}
	actual := serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESIKColBytesExtension
	} else if actual != serializationKCES.KCESIKColBytesExtension {
		return fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, serializationKCES.KCESIKColBytesExtension)
	}
	encoded, err := serializationKCES.EncodeKCESPayload(&value)
	if err != nil {
		return fmt.Errorf("encode .ikcol.bytes file: %w", err)
	}
	return writeIKColBytesConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeIKColBytesConversionOutput 在上下文有效且大小不超限时写入 .ikcol.bytes 转换结果
// writeIKColBytesConversionOutput writes .ikcol.bytes conversion output while the context is active and the size remains within the limit
func writeIKColBytesConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .ikcol.bytes conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .ikcol.bytes conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .ikcol.bytes conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
