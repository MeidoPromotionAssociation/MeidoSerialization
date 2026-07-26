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

const priorityMaterialAssetsExtension = ".pmatassets"

// PriorityMaterialAssetsService 专门处理 .pmatassets 文件 / PriorityMaterialAssetsService handles .pmatassets files
type PriorityMaterialAssetsService struct{}

// IsKCESPriorityMaterialAssetsFile 判断路径是否为 .pmatassets 文件
// IsKCESPriorityMaterialAssetsFile reports whether a path names a .pmatassets file
func IsKCESPriorityMaterialAssetsFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), priorityMaterialAssetsExtension)
}

// IsKCESPriorityMaterialAssetsJSONFile 判断路径是否为 .pmatassets 编辑 JSON
// IsKCESPriorityMaterialAssetsJSONFile reports whether a path names .pmatassets editing JSON
func IsKCESPriorityMaterialAssetsJSONFile(path string) bool {
	return partsExtFromJSONPath(path) == priorityMaterialAssetsExtension
}

// ReadPriorityMaterialAssetsFile 读取并解码 .pmatassets 文件
// ReadPriorityMaterialAssetsFile reads and decodes a .pmatassets file
func (s *PriorityMaterialAssetsService) ReadPriorityMaterialAssetsFile(path string) (*serializationKCES.PriorityMaterialAssets, error) {
	return readPriorityMaterialAssetsFile(path)
}

// WritePriorityMaterialAssetsFile 编码并写入 .pmatassets 文件
// WritePriorityMaterialAssetsFile encodes and writes a .pmatassets file
func (s *PriorityMaterialAssetsService) WritePriorityMaterialAssetsFile(path string, value *serializationKCES.PriorityMaterialAssets) error {
	return writePriorityMaterialAssetsFile(path, value)
}

// ConvertPriorityMaterialAssetsToJson 将 .pmatassets 文件转换为编辑 JSON
// ConvertPriorityMaterialAssetsToJson converts a .pmatassets file to editing JSON
func (s *PriorityMaterialAssetsService) ConvertPriorityMaterialAssetsToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadPriorityMaterialAssetsFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal .pmatassets JSON: %w", err)
	}
	return writePriorityMaterialAssetsConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToPriorityMaterialAssets 将编辑 JSON 转换为 .pmatassets 文件
// ConvertJsonToPriorityMaterialAssets converts editing JSON to a .pmatassets file
func (s *PriorityMaterialAssetsService) ConvertJsonToPriorityMaterialAssets(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read .pmatassets JSON %q: %w", inputPath, err)
	}
	encoded, err := encodePriorityMaterialAssetsJSON(data)
	if err != nil {
		return err
	}
	return writePriorityMaterialAssetsConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WritePriorityMaterialAssetsFile 为聚合 service 保留 .pmatassets 直接写入 API
// WritePriorityMaterialAssetsFile preserves the direct .pmatassets writer on the aggregate service
func (s *PartsService) WritePriorityMaterialAssetsFile(path string, value *serializationKCES.PriorityMaterialAssets) error {
	return writePriorityMaterialAssetsFile(path, value)
}

// readPriorityMaterialAssetsFile 读取并解码原生 .pmatassets 数据
// readPriorityMaterialAssetsFile reads and decodes native .pmatassets data
func readPriorityMaterialAssetsFile(path string) (*serializationKCES.PriorityMaterialAssets, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return serializationKCES.DecodePriorityMaterialAssets(data)
}

// encodePriorityMaterialAssetsJSON 严格解码编辑 JSON 并编码原生 .pmatassets 数据
// encodePriorityMaterialAssetsJSON strictly decodes editing JSON and encodes native .pmatassets data
func encodePriorityMaterialAssetsJSON(data []byte) ([]byte, error) {
	var value *serializationKCES.PriorityMaterialAssets
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES pmatassets JSON"); err != nil {
		return nil, fmt.Errorf("parse pmatassets json: %w", err)
	}
	return serializationKCES.EncodePriorityMaterialAssets(value)
}

// writePriorityMaterialAssetsFile 编码并直接写入原生 .pmatassets 数据
// writePriorityMaterialAssetsFile encodes and directly writes native .pmatassets data
func writePriorityMaterialAssetsFile(path string, value *serializationKCES.PriorityMaterialAssets) error {
	encoded, err := serializationKCES.EncodePriorityMaterialAssets(value)
	if err != nil {
		return fmt.Errorf("encode KCES pmatassets: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .pmatassets file %q: %w", path, err)
	}
	return nil
}

// writePriorityMaterialAssetsConversionOutput 在上下文有效且大小不超限时写入 .pmatassets 转换结果
// writePriorityMaterialAssetsConversionOutput writes .pmatassets conversion output while the context is active and the size remains within the limit
func writePriorityMaterialAssetsConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .pmatassets conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .pmatassets conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .pmatassets conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
