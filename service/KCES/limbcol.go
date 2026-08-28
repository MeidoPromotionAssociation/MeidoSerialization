package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

// LimbColService 专门处理 .limbcol 文件 / LimbColService handles .limbcol files
type LimbColService struct{}

// ReadLimbColFile 读取并解码 .limbcol 文件
// ReadLimbColFile reads and decodes a .limbcol file
func (s *LimbColService) ReadLimbColFile(path string) (*serializationKCES.LimbColliderPackage, error) {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESLimbColExtension {
		return nil, fmt.Errorf("not a .limbcol file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .limbcol file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeLimbCol(data)
	if err != nil {
		return nil, fmt.Errorf("decode .limbcol file %q: %w", path, err)
	}
	return value, nil
}

// WriteLimbColFile 编码并写入 .limbcol 文件
// WriteLimbColFile encodes and writes a .limbcol file
func (s *LimbColService) WriteLimbColFile(path string, value *serializationKCES.LimbColliderPackage) error {
	if serializationKCES.NormalizeKCESPayloadExtension(path) != serializationKCES.KCESLimbColExtension {
		return fmt.Errorf("not a .limbcol output path: %s", path)
	}
	encoded, err := serializationKCES.EncodeLimbCol(value)
	if err != nil {
		return fmt.Errorf("encode .limbcol file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .limbcol file %q: %w", path, err)
	}
	return nil
}

// ConvertLimbColToJson 将 .limbcol 文件转换为编辑 JSON
// ConvertLimbColToJson converts a .limbcol file to editing JSON
func (s *LimbColService) ConvertLimbColToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadLimbColFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .limbcol JSON: %w", err)
	}
	return writeLimbColConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJsonToLimbCol 将编辑 JSON 转换为 .limbcol 文件
// ConvertJsonToLimbCol converts editing JSON to a .limbcol file
func (s *LimbColService) ConvertJsonToLimbCol(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .limbcol JSON %q: %w", inputPath, err)
	}
	var value *serializationKCES.LimbColliderPackage
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES .limbcol JSON"); err != nil {
		return fmt.Errorf("parse .limbcol JSON: %w", err)
	}
	encoded, err := serializationKCES.EncodeLimbCol(value)
	if err != nil {
		return fmt.Errorf("encode .limbcol file: %w", err)
	}
	return writeLimbColConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeLimbColConversionOutput 在上下文有效且大小不超限时写入 .limbcol 转换结果
// writeLimbColConversionOutput writes .limbcol conversion output while the context is active and the size remains within the limit
func writeLimbColConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .limbcol conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .limbcol conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .limbcol conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
