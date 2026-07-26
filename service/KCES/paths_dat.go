package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// PathsService 专门处理 paths.dat 文件 / PathsService handles paths.dat files
type PathsService struct{}

// IsKCESPathsFile 判断路径是否为原生 paths.dat 文件
// IsKCESPathsFile reports whether a path names a native paths.dat file
func IsKCESPathsFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Base(path), "paths.dat")
}

// IsKCESPathsJSONFile 判断路径是否为 paths.dat 编辑 JSON
// IsKCESPathsJSONFile reports whether a path names paths.dat editing JSON
func IsKCESPathsJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), "paths.dat.json") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var header struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(trimJSONUTF8BOM(data), &header); err != nil {
		return false
	}
	return header.Format == serializationKCES.KCESPathsFormat
}

// ReadPathsFile 读取并解码 paths.dat 文件
// ReadPathsFile reads and decodes a paths.dat file
func (s *PathsService) ReadPathsFile(path string) (*serializationKCES.KCESPathsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read paths.dat %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESPaths(data)
	if err != nil {
		return nil, fmt.Errorf("decode paths.dat %q: %w", path, err)
	}
	return value, nil
}

// ConvertPathsToJSON 将 paths.dat 文件转换为编辑 JSON
// ConvertPathsToJSON converts a paths.dat file to editing JSON
func (s *PathsService) ConvertPathsToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadPathsFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal paths.dat JSON: %w", err)
	}
	encoded = append(encoded, '\n')
	return writePathsConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJSONToPaths 将编辑 JSON 转换为 paths.dat 文件
// ConvertJSONToPaths converts editing JSON to a paths.dat file
func (s *PathsService) ConvertJSONToPaths(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read paths.dat JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPathsFile
	if err := decodeStrictJSON(data, &value, "KCES paths.dat JSON"); err != nil {
		return fmt.Errorf("parse paths.dat JSON: %w", err)
	}
	if value.Format != serializationKCES.KCESPathsFormat {
		return fmt.Errorf("unsupported paths.dat JSON format %q", value.Format)
	}
	if value.Signature != serializationKCES.KCESPathsSignature {
		return fmt.Errorf("invalid paths.dat JSON signature %q", value.Signature)
	}
	encoded, err := serializationKCES.EncodeKCESPaths(&value)
	if err != nil {
		return err
	}
	return writePathsConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WritePathsFile 将资源搜索路径结构直接编码并写入 paths.dat 文件
// WritePathsFile directly encodes a resource search path value and writes it to a paths.dat file
func (s *PathsService) WritePathsFile(path string, value *serializationKCES.KCESPathsFile) error {
	encoded, err := serializationKCES.EncodeKCESPaths(value)
	if err != nil {
		return fmt.Errorf("encode KCES paths.dat: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write paths.dat file %q: %w", path, err)
	}
	return nil
}

// writePathsConversionOutput 在上下文有效且大小不超限时写入 paths.dat 转换结果
// writePathsConversionOutput writes paths.dat conversion output while the context is active and the size remains within the limit
func writePathsConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive paths.dat conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: paths.dat conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write paths.dat conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
