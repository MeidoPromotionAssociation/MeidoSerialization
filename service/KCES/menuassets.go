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

const menuAssetsExtension = ".menuassets"

// MenuAssetsService 专门处理 .menuassets 文件 / MenuAssetsService handles .menuassets files
type MenuAssetsService struct{}

// IsKCESMenuAssetsFile 判断路径是否为 .menuassets 文件
// IsKCESMenuAssetsFile reports whether a path names a .menuassets file
func IsKCESMenuAssetsFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), menuAssetsExtension)
}

// IsKCESMenuAssetsJSONFile 判断路径是否为 .menuassets 编辑 JSON
// IsKCESMenuAssetsJSONFile reports whether a path names .menuassets editing JSON
func IsKCESMenuAssetsJSONFile(path string) bool {
	return partsExtFromJSONPath(path) == menuAssetsExtension
}

// ReadMenuAssetsFile 读取并解码 .menuassets 文件
// ReadMenuAssetsFile reads and decodes a .menuassets file
func (s *MenuAssetsService) ReadMenuAssetsFile(path string) (*serializationKCES.MenuAssets, error) {
	return readMenuAssetsFile(path)
}

// WriteMenuAssetsFile 编码并写入 .menuassets 文件
// WriteMenuAssetsFile encodes and writes a .menuassets file
func (s *MenuAssetsService) WriteMenuAssetsFile(path string, value *serializationKCES.MenuAssets) error {
	return writeMenuAssetsFile(path, value)
}

// ConvertMenuAssetsToJson 将 .menuassets 文件转换为编辑 JSON
// ConvertMenuAssetsToJson converts a .menuassets file to editing JSON
func (s *MenuAssetsService) ConvertMenuAssetsToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadMenuAssetsFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal .menuassets JSON: %w", err)
	}
	return writeMenuAssetsConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToMenuAssets 将编辑 JSON 转换为 .menuassets 文件
// ConvertJsonToMenuAssets converts editing JSON to a .menuassets file
func (s *MenuAssetsService) ConvertJsonToMenuAssets(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .menuassets JSON %q: %w", inputPath, err)
	}
	encoded, err := encodeMenuAssetsJSONWithOptions(data, &serializationKCES.LookupHashOptions{RecalculateHash: true})
	if err != nil {
		return err
	}
	return writeMenuAssetsConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WriteMenuAssetsFile 为聚合 service 保留 .menuassets 直接写入 API
// WriteMenuAssetsFile preserves the direct .menuassets writer on the aggregate service
func (s *PartsService) WriteMenuAssetsFile(path string, value *serializationKCES.MenuAssets) error {
	return writeMenuAssetsFile(path, value)
}

// readMenuAssetsFile 读取并解码原生 .menuassets 数据
// readMenuAssetsFile reads and decodes native .menuassets data
func readMenuAssetsFile(path string) (*serializationKCES.MenuAssets, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return serializationKCES.DecodeMenuAssets(data)
}

// encodeMenuAssetsJSON 严格解码编辑 JSON 并编码原生 .menuassets 数据
// encodeMenuAssetsJSON strictly decodes editing JSON and encodes native .menuassets data
func encodeMenuAssetsJSON(data []byte) ([]byte, error) {
	return encodeMenuAssetsJSONWithOptions(data, nil)
}

// encodeMenuAssetsJSONWithOptions 严格解码编辑 JSON 并按指定查找字段选项编码原生 .menuassets 数据
// encodeMenuAssetsJSONWithOptions strictly decodes editing JSON and encodes native .menuassets data with the selected lookup-field options
func encodeMenuAssetsJSONWithOptions(data []byte, options *serializationKCES.LookupHashOptions) ([]byte, error) {
	var value *serializationKCES.MenuAssets
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES menuassets JSON"); err != nil {
		return nil, fmt.Errorf("parse menuassets json: %w", err)
	}
	return serializationKCES.EncodeMenuAssetsWithOptions(value, options)
}

// writeMenuAssetsFile 编码并直接写入原生 .menuassets 数据
// writeMenuAssetsFile encodes and directly writes native .menuassets data
func writeMenuAssetsFile(path string, value *serializationKCES.MenuAssets) error {
	encoded, err := serializationKCES.EncodeMenuAssetsWithOptions(value, &serializationKCES.LookupHashOptions{RecalculateHash: true})
	if err != nil {
		return fmt.Errorf("encode KCES menuassets: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .menuassets file %q: %w", path, err)
	}
	return nil
}

// writeMenuAssetsConversionOutput 在上下文有效且大小不超限时写入 .menuassets 转换结果
// writeMenuAssetsConversionOutput writes .menuassets conversion output while the context is active and the size remains within the limit
func writeMenuAssetsConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .menuassets conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .menuassets conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .menuassets conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
