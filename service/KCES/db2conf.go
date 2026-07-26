package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// DB2ConfService 专门处理 .db2conf 文件 / DB2ConfService handles .db2conf files
type DB2ConfService struct{}

// ReadDB2ConfFile 读取并解码 .db2conf 文件
// ReadDB2ConfFile reads and decodes a .db2conf file
func (s *DB2ConfService) ReadDB2ConfFile(path string) (*serializationKCES.KCESPayloadEnvelope, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDB2ConfExtension {
		return nil, fmt.Errorf("not a .db2conf file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .db2conf file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeDB2Conf(data)
	if err != nil {
		return nil, fmt.Errorf("decode .db2conf file %q: %w", path, err)
	}
	return value, nil
}

// WriteDB2ConfFile 编码并写入 .db2conf 文件
// WriteDB2ConfFile encodes and writes a .db2conf file
func (s *DB2ConfService) WriteDB2ConfFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDB2ConfExtension {
		return fmt.Errorf("not a .db2conf output path: %s", path)
	}
	if value == nil || serializationKCES.NormalizeKCESPayloadExtension(value.Extension) != serializationKCES.KCESDB2ConfExtension {
		return fmt.Errorf(".db2conf output requires a .db2conf KCES payload envelope")
	}
	encoded, err := serializationKCES.EncodeDB2Conf(value)
	if err != nil {
		return fmt.Errorf("encode .db2conf file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .db2conf file %q: %w", path, err)
	}
	return nil
}

// ConvertDB2ConfToJson 将 .db2conf 文件转换为编辑 JSON
// ConvertDB2ConfToJson converts a .db2conf file to editing JSON
func (s *DB2ConfService) ConvertDB2ConfToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadDB2ConfFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .db2conf JSON: %w", err)
	}
	return writeDB2ConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToDB2Conf 将编辑 JSON 转换为 .db2conf 文件
// ConvertJsonToDB2Conf converts editing JSON to a .db2conf file
func (s *DB2ConfService) ConvertJsonToDB2Conf(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .db2conf JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .db2conf JSON"); err != nil {
		return fmt.Errorf("parse .db2conf JSON: %w", err)
	}
	if value.Format != serializationKCES.PayloadFormatKCESMessagePack && value.Format != serializationKCES.PayloadFormatKCESExportCM {
		return fmt.Errorf("unsupported .db2conf JSON format %q", value.Format)
	}
	actual := serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	if actual == "" {
		value.Extension = serializationKCES.KCESDB2ConfExtension
	} else if actual != serializationKCES.KCESDB2ConfExtension {
		return fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, serializationKCES.KCESDB2ConfExtension)
	}
	encoded, err := serializationKCES.EncodeDB2Conf(&value)
	if err != nil {
		return fmt.Errorf("encode .db2conf file: %w", err)
	}
	return writeDB2ConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeDB2ConfConversionOutput 在上下文有效且大小不超限时写入 .db2conf 转换结果
// writeDB2ConfConversionOutput writes .db2conf conversion output while the context is active and the size remains within the limit
func writeDB2ConfConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .db2conf conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .db2conf conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .db2conf conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
