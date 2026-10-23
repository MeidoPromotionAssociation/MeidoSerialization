// Package CRC3D3 实现 CRC3D3 使用的专有文件格式
// Package CRC3D3 implements the proprietary file formats used by CRC3D3
package CRC3D3

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationCRC3D3 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/CRC3D3"
)

// ErrConversionOutputLimitExceeded 表示转换结果超过调用方提供的固定宽度字节上限 / ErrConversionOutputLimitExceeded indicates that conversion output exceeds the caller-provided fixed-width byte limit
var ErrConversionOutputLimitExceeded = errors.New("CRC3D3 conversion output limit exceeded")

// NeimpService 专门处理 .neimp 表格文件 / NeimpService handles .neimp table files
type NeimpService struct{}

// IsCRC3D3NeimpFile 判断路径是否为 .neimp 原生文件
// IsCRC3D3NeimpFile reports whether a path names a native .neimp file
func IsCRC3D3NeimpFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), serializationCRC3D3.NeimpExtension)
}

// IsCRC3D3NeimpJSONFile 判断路径是否为 .neimp JSON
// IsCRC3D3NeimpJSONFile reports whether a path names .neimp JSON
func IsCRC3D3NeimpJSONFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), serializationCRC3D3.NeimpJSONExtension)
}

// ReadNeimpFile 读取并解码 .neimp 文件
// ReadNeimpFile reads and decodes a .neimp file
func (s *NeimpService) ReadNeimpFile(path string) (serializationCRC3D3.Value, error) {
	if !IsCRC3D3NeimpFile(path) {
		return serializationCRC3D3.Value{}, fmt.Errorf("not a .neimp file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return serializationCRC3D3.Value{}, fmt.Errorf("read .neimp file %q: %w", path, err)
	}
	value, err := serializationCRC3D3.DecodeNeimp(data)
	if err != nil {
		return serializationCRC3D3.Value{}, fmt.Errorf("decode .neimp file %q: %w", path, err)
	}
	return value, nil
}

// WriteNeimpFile 编码并写入 .neimp 文件
// WriteNeimpFile encodes and writes a .neimp file
func (s *NeimpService) WriteNeimpFile(path string, value serializationCRC3D3.Value) error {
	if !IsCRC3D3NeimpFile(path) {
		return fmt.Errorf("not a .neimp output path: %s", path)
	}
	encoded, err := serializationCRC3D3.EncodeNeimp(value)
	if err != nil {
		return fmt.Errorf("encode .neimp file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .neimp file %q: %w", path, err)
	}
	return nil
}

// ConvertNeimpToJSON 将 .neimp 文件转换为 JSON
// ConvertNeimpToJSON converts a .neimp file to JSON
func (s *NeimpService) ConvertNeimpToJSON(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	value, err := s.ReadNeimpFile(inputPath)
	if err != nil {
		return err
	}
	data, err := serializationCRC3D3.EncodeNeimpJSON(value)
	if err != nil {
		return fmt.Errorf("encode .neimp JSON: %w", err)
	}
	return writeNeimpConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJSONToNeimp 将 JSON 转换为 .neimp 文件
// ConvertJSONToNeimp converts JSON to a .neimp file
func (s *NeimpService) ConvertJSONToNeimp(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .neimp JSON %q: %w", inputPath, err)
	}
	value, err := serializationCRC3D3.DecodeNeimpJSON(data)
	if err != nil {
		return fmt.Errorf("parse .neimp JSON: %w", err)
	}
	encoded, err := serializationCRC3D3.EncodeNeimp(value)
	if err != nil {
		return fmt.Errorf("encode .neimp file: %w", err)
	}
	return writeNeimpConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writeNeimpConversionOutput 在上下文有效且大小不超限时写入转换结果
// writeNeimpConversionOutput writes conversion output while the context is active and the size stays within the limit
func writeNeimpConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .neimp conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .neimp conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .neimp conversion output %q: %w", path, err)
	}
	return contextError(ctx)
}

// contextError 返回上下文的取消或超时错误，nil 上下文视为有效
// contextError returns the cancellation or deadline error of the context and treats a nil context as valid
func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
