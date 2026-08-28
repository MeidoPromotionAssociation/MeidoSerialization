package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

// DSL2ConfService 专门处理 .dsl2conf 文件 / DSL2ConfService handles .dsl2conf files
type DSL2ConfService struct{}

// ReadDSL2ConfFile 读取并解码 .dsl2conf 文件
// ReadDSL2ConfFile reads and decodes a .dsl2conf file
func (s *DSL2ConfService) ReadDSL2ConfFile(path string) (*serializationKCES.MagicaClothSerializeData, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSL2ConfExtension {
		return nil, fmt.Errorf("not a .dsl2conf file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .dsl2conf file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeDSL2Conf(data)
	if err != nil {
		return nil, fmt.Errorf("decode .dsl2conf file %q: %w", path, err)
	}
	return value, nil
}

// WriteDSL2ConfFile 编码并写入 .dsl2conf 文件
// WriteDSL2ConfFile encodes and writes a .dsl2conf file
func (s *DSL2ConfService) WriteDSL2ConfFile(path string, value *serializationKCES.MagicaClothSerializeData) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESDSL2ConfExtension {
		return fmt.Errorf("not a .dsl2conf output path: %s", path)
	}
	encoded, err := serializationKCES.EncodeDSL2Conf(value)
	if err != nil {
		return fmt.Errorf("encode .dsl2conf file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .dsl2conf file %q: %w", path, err)
	}
	return nil
}

// ConvertDSL2ConfToJson 将 .dsl2conf 文件转换为编辑 JSON
// ConvertDSL2ConfToJson converts a .dsl2conf file to editing JSON
func (s *DSL2ConfService) ConvertDSL2ConfToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadDSL2ConfFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .dsl2conf JSON: %w", err)
	}
	return writeDSL2ConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToDSL2Conf 将编辑 JSON 转换为 .dsl2conf 文件
// ConvertJsonToDSL2Conf converts editing JSON to a .dsl2conf file
func (s *DSL2ConfService) ConvertJsonToDSL2Conf(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .dsl2conf JSON %q: %w", inputPath, err)
	}
	var value *serializationKCES.MagicaClothSerializeData
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .dsl2conf JSON"); err != nil {
		return fmt.Errorf("parse .dsl2conf JSON: %w", err)
	}
	encoded, err := serializationKCES.EncodeDSL2Conf(value)
	if err != nil {
		return fmt.Errorf("encode .dsl2conf file: %w", err)
	}
	return writeDSL2ConfConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeDSL2ConfConversionOutput 在上下文有效且大小不超限时写入 .dsl2conf 转换结果
// writeDSL2ConfConversionOutput writes .dsl2conf conversion output while the context is active and the size remains within the limit
func writeDSL2ConfConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .dsl2conf conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .dsl2conf conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .dsl2conf conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
