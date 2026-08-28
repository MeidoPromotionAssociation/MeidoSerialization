package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

const hitCheckExtension = ".hitcheck"

// HitCheckService 专门处理 .hitcheck 文件 / HitCheckService handles .hitcheck files
type HitCheckService struct{}

// IsKCESHitCheckFile 判断路径是否为 .hitcheck 文件
// IsKCESHitCheckFile reports whether a path names a .hitcheck file
func IsKCESHitCheckFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), hitCheckExtension)
}

// IsKCESHitCheckJSONFile 判断路径是否为 .hitcheck 编辑 JSON
// IsKCESHitCheckJSONFile reports whether a path names .hitcheck editing JSON
func IsKCESHitCheckJSONFile(path string) bool {
	return miscExtFromJSONPath(path) == hitCheckExtension
}

// ReadHitCheckFile 读取并解码 .hitcheck 文件
// ReadHitCheckFile reads and decodes a .hitcheck file
func (s *HitCheckService) ReadHitCheckFile(path string) (*serializationKCES.HitCheck, error) {
	return readHitCheckFile(path)
}

// WriteHitCheckFile 编码并写入 .hitcheck 文件
// WriteHitCheckFile encodes and writes a .hitcheck file
func (s *HitCheckService) WriteHitCheckFile(path string, value *serializationKCES.HitCheck) error {
	return writeHitCheckFile(path, value)
}

// ConvertHitCheckToJson 将 .hitcheck 文件转换为编辑 JSON
// ConvertHitCheckToJson converts a .hitcheck file to editing JSON
func (s *HitCheckService) ConvertHitCheckToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadHitCheckFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .hitcheck JSON: %w", err)
	}
	return writeHitCheckConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToHitCheck 将编辑 JSON 转换为 .hitcheck 文件
// ConvertJsonToHitCheck converts editing JSON to a .hitcheck file
func (s *HitCheckService) ConvertJsonToHitCheck(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .hitcheck JSON %q: %w", inputPath, err)
	}
	encoded, err := encodeHitCheckJSON(data)
	if err != nil {
		return err
	}
	return writeHitCheckConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WriteHitCheckFile 为聚合 service 保留 .hitcheck 直接写入 API
// WriteHitCheckFile preserves the direct .hitcheck writer on the aggregate service
func (s *MiscService) WriteHitCheckFile(path string, value *serializationKCES.HitCheck) error {
	return writeHitCheckFile(path, value)
}

// readHitCheckFile 读取并解码原生 .hitcheck 数据
// readHitCheckFile reads and decodes native .hitcheck data
func readHitCheckFile(path string) (*serializationKCES.HitCheck, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return serializationKCES.DecodeHitCheck(data)
}

// encodeHitCheckJSON 严格解码编辑 JSON 并编码原生 .hitcheck 数据
// encodeHitCheckJSON strictly decodes editing JSON and encodes native .hitcheck data
func encodeHitCheckJSON(data []byte) ([]byte, error) {
	var value serializationKCES.HitCheck
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES hitcheck JSON"); err != nil {
		return nil, fmt.Errorf("parse hitcheck json: %w", err)
	}
	if value.Signature != serializationKCES.HitCheckSignature {
		return nil, fmt.Errorf("parse hitcheck json: invalid signature %q", value.Signature)
	}
	return serializationKCES.EncodeHitCheck(&value)
}

// writeHitCheckFile 编码并直接写入原生 .hitcheck 数据
// writeHitCheckFile encodes and directly writes native .hitcheck data
func writeHitCheckFile(path string, value *serializationKCES.HitCheck) error {
	encoded, err := serializationKCES.EncodeHitCheck(value)
	if err != nil {
		return fmt.Errorf("encode KCES hitcheck: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .hitcheck file %q: %w", path, err)
	}
	return nil
}

// writeHitCheckConversionOutput 在上下文有效且大小不超限时写入 .hitcheck 转换结果
// writeHitCheckConversionOutput writes .hitcheck conversion output while the context is active and the size remains within the limit
func writeHitCheckConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .hitcheck conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .hitcheck conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .hitcheck conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
