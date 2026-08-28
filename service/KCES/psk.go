package KCES

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/COM3D2"
)

const pskExtension = ".psk"

// PskService 专门处理 KCES 使用的 .psk 共用格式 / PskService handles the shared .psk format used by KCES
type PskService struct{}

// IsKCESPskFile 判断路径是否为 KCES 使用的 .psk 文件
// IsKCESPskFile reports whether a path names a .psk file used by KCES
func IsKCESPskFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), pskExtension)
}

// IsKCESPskJSONFile 判断路径是否为 .psk 编辑 JSON
// IsKCESPskJSONFile reports whether a path names .psk editing JSON
func IsKCESPskJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.EqualFold(filepath.Ext(base), pskExtension)
}

// ReadPskFile 读取 .psk 或 .psk.json 文件
// ReadPskFile reads a .psk or .psk.json file
func (s *PskService) ReadPskFile(path string) (*serializationCOM3D2.Psk, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .psk file %q: %w", path, err)
	}
	defer file.Close()
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		var value serializationCOM3D2.Psk
		if err := json.NewDecoder(file).Decode(&value); err != nil {
			return nil, fmt.Errorf("decode .psk JSON %q: %w", path, err)
		}
		return &value, nil
	}
	value, err := serializationCOM3D2.ReadPsk(file)
	if err != nil {
		return nil, fmt.Errorf("decode .psk file %q: %w", path, err)
	}
	return value, nil
}

// WritePskFile 编码并写入 .psk 文件
// WritePskFile encodes and writes a .psk file
func (s *PskService) WritePskFile(path string, value *serializationCOM3D2.Psk) error {
	return writePskFile(path, value)
}

// ConvertPskToJson 将 .psk 文件转换为编辑 JSON
// ConvertPskToJson converts a .psk file to editing JSON
func (s *PskService) ConvertPskToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadPskFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal .psk JSON: %w", err)
	}
	return writePskConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToPsk 将编辑 JSON 转换为 .psk 文件
// ConvertJsonToPsk converts editing JSON to a .psk file
func (s *PskService) ConvertJsonToPsk(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open .psk JSON %q: %w", inputPath, err)
	}
	var value serializationCOM3D2.Psk
	decodeErr := json.NewDecoder(file).Decode(&value)
	closeErr := file.Close()
	if decodeErr != nil {
		return fmt.Errorf("decode .psk JSON %q: %w", inputPath, decodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close .psk JSON %q: %w", inputPath, closeErr)
	}
	var encoded bytes.Buffer
	if err := value.Dump(&encoded); err != nil {
		return fmt.Errorf("encode .psk file: %w", err)
	}
	return writePskConversionOutput(ctx, outputPath, encoded.Bytes(), maxOutputBytes)
}

// WritePskFile 为聚合 service 保留 .psk 直接写入 API
// WritePskFile preserves the direct .psk writer on the aggregate service
func (s *DataService) WritePskFile(path string, value *serializationCOM3D2.Psk) error {
	return writePskFile(path, value)
}

// writePskFile 编码并直接写入原生 .psk 数据
// writePskFile encodes and directly writes native .psk data
func writePskFile(path string, value *serializationCOM3D2.Psk) error {
	if value == nil {
		return fmt.Errorf("encode KCES shared psk: nil psk")
	}
	var encoded bytes.Buffer
	if err := value.Dump(&encoded); err != nil {
		return fmt.Errorf("encode KCES shared psk: %w", err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0644); err != nil {
		return fmt.Errorf("write .psk file %q: %w", path, err)
	}
	return nil
}

// writePskConversionOutput 在上下文有效且大小不超限时写入 .psk 转换结果
// writePskConversionOutput writes .psk conversion output while the context is active and the size remains within the limit
func writePskConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .psk conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .psk conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .psk conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
