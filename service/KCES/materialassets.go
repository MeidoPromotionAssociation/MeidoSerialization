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

const materialAssetsExtension = ".materialassets"

// MaterialAssetsService 专门处理 .materialassets 文件 / MaterialAssetsService handles .materialassets files
type MaterialAssetsService struct{}

// IsKCESMaterialAssetsFile 判断路径是否为 .materialassets 文件
// IsKCESMaterialAssetsFile reports whether a path names a .materialassets file
func IsKCESMaterialAssetsFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), materialAssetsExtension)
}

// IsKCESMaterialAssetsJSONFile 判断路径是否为 .materialassets 编辑 JSON
// IsKCESMaterialAssetsJSONFile reports whether a path names .materialassets editing JSON
func IsKCESMaterialAssetsJSONFile(path string) bool {
	return partsExtFromJSONPath(path) == materialAssetsExtension
}

// ReadMaterialAssetsFile 读取并解码 .materialassets 文件
// ReadMaterialAssetsFile reads and decodes a .materialassets file
func (s *MaterialAssetsService) ReadMaterialAssetsFile(path string) (*serializationKCES.MaterialAssets, error) {
	return readMaterialAssetsFile(path)
}

// WriteMaterialAssetsFile 编码并写入 .materialassets 文件
// WriteMaterialAssetsFile encodes and writes a .materialassets file
func (s *MaterialAssetsService) WriteMaterialAssetsFile(path string, value *serializationKCES.MaterialAssets) error {
	return writeMaterialAssetsFile(path, value)
}

// ConvertMaterialAssetsToJson 将 .materialassets 文件转换为编辑 JSON
// ConvertMaterialAssetsToJson converts a .materialassets file to editing JSON
func (s *MaterialAssetsService) ConvertMaterialAssetsToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadMaterialAssetsFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal .materialassets JSON: %w", err)
	}
	return writeMaterialAssetsConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToMaterialAssets 将编辑 JSON 转换为 .materialassets 文件
// ConvertJsonToMaterialAssets converts editing JSON to a .materialassets file
func (s *MaterialAssetsService) ConvertJsonToMaterialAssets(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .materialassets JSON %q: %w", inputPath, err)
	}
	encoded, err := encodeMaterialAssetsJSONWithOptions(data, &serializationKCES.LookupHashOptions{RecalculateHash: true})
	if err != nil {
		return err
	}
	return writeMaterialAssetsConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WriteMaterialAssetsFile 为聚合 service 保留 .materialassets 直接写入 API
// WriteMaterialAssetsFile preserves the direct .materialassets writer on the aggregate service
func (s *PartsService) WriteMaterialAssetsFile(path string, value *serializationKCES.MaterialAssets) error {
	return writeMaterialAssetsFile(path, value)
}

// readMaterialAssetsFile 读取并解码原生 .materialassets 数据
// readMaterialAssetsFile reads and decodes native .materialassets data
func readMaterialAssetsFile(path string) (*serializationKCES.MaterialAssets, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return serializationKCES.DecodeMaterialAssets(data)
}

// encodeMaterialAssetsJSON 严格解码编辑 JSON 并编码原生 .materialassets 数据
// encodeMaterialAssetsJSON strictly decodes editing JSON and encodes native .materialassets data
func encodeMaterialAssetsJSON(data []byte) ([]byte, error) {
	return encodeMaterialAssetsJSONWithOptions(data, nil)
}

// encodeMaterialAssetsJSONWithOptions 严格解码编辑 JSON 并按指定查找字段选项编码原生 .materialassets 数据
// encodeMaterialAssetsJSONWithOptions strictly decodes editing JSON and encodes native .materialassets data with the selected lookup-field options
func encodeMaterialAssetsJSONWithOptions(data []byte, options *serializationKCES.LookupHashOptions) ([]byte, error) {
	var value *serializationKCES.MaterialAssets
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES materialassets JSON"); err != nil {
		return nil, fmt.Errorf("parse materialassets json: %w", err)
	}
	return serializationKCES.EncodeMaterialAssetsWithOptions(value, options)
}

// writeMaterialAssetsFile 编码并直接写入原生 .materialassets 数据
// writeMaterialAssetsFile encodes and directly writes native .materialassets data
func writeMaterialAssetsFile(path string, value *serializationKCES.MaterialAssets) error {
	encoded, err := serializationKCES.EncodeMaterialAssetsWithOptions(value, &serializationKCES.LookupHashOptions{RecalculateHash: true})
	if err != nil {
		return fmt.Errorf("encode KCES materialassets: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .materialassets file %q: %w", path, err)
	}
	return nil
}

// writeMaterialAssetsConversionOutput 在上下文有效且大小不超限时写入 .materialassets 转换结果
// writeMaterialAssetsConversionOutput writes .materialassets conversion output while the context is active and the size remains within the limit
func writeMaterialAssetsConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .materialassets conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .materialassets conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .materialassets conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
