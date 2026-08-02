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

const kcMatExtension = serializationKCES.KCMatExtension

// KCMatService 专门处理 KCES2 导出的单个 .kcmat 文件 / KCMatService handles individual .kcmat files exported by KCES2
type KCMatService struct{}

// IsKCESKCMatFile 判断路径是否为 KCES2 单个 .kcmat 文件
// IsKCESKCMatFile reports whether a path names an individual KCES2 .kcmat file
func IsKCESKCMatFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), kcMatExtension)
}

// IsKCESKCMatJSONFile 判断路径是否为 KCES2 单个 .kcmat 的编辑 JSON
// IsKCESKCMatJSONFile reports whether a path names editing JSON for an individual KCES2 .kcmat file
func IsKCESKCMatJSONFile(path string) bool {
	return partsExtFromJSONPath(path) == kcMatExtension
}

// ReadKCMatFile 读取并解码 KCES2 单个 .kcmat 文件
// ReadKCMatFile reads and decodes an individual KCES2 .kcmat file
func (s *KCMatService) ReadKCMatFile(path string) (*serializationKCES.Material, error) {
	return readKCMatFile(path)
}

// WriteKCMatFile 编码并写入 KCES2 单个 .kcmat 文件，同时按目标文件名重算可确定的查找字段
// WriteKCMatFile encodes and writes an individual KCES2 .kcmat file while recalculating determinable lookup fields from the destination filename
func (s *KCMatService) WriteKCMatFile(path string, value *serializationKCES.Material) error {
	return writeKCMatFile(path, value)
}

// ConvertKCMatToJson 将 KCES2 单个 .kcmat 文件转换为编辑 JSON
// ConvertKCMatToJson converts an individual KCES2 .kcmat file to editing JSON
func (s *KCMatService) ConvertKCMatToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadKCMatFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal KCES2 .kcmat JSON: %w", err)
	}
	return writeKCMatConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToKCMat 将编辑 JSON 转换为 KCES2 单个 .kcmat 文件，并按目标文件名重算可确定的查找字段
// ConvertJsonToKCMat converts editing JSON to an individual KCES2 .kcmat file and recalculates determinable lookup fields from the destination filename
func (s *KCMatService) ConvertJsonToKCMat(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES2 .kcmat JSON %q: %w", inputPath, err)
	}
	encoded, err := encodeKCMatJSONWithOptions(data, kcMatLookupHashOptions(outputPath))
	if err != nil {
		return err
	}
	return writeKCMatConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// readKCMatFile 读取并解码原生 KCES2 .kcmat 数据
// readKCMatFile reads and decodes native KCES2 .kcmat data
func readKCMatFile(path string) (*serializationKCES.Material, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return serializationKCES.DecodeKCMat(data)
}

// encodeKCMatJSON 严格解码编辑 JSON 并编码原生 KCES2 .kcmat 数据
// encodeKCMatJSON strictly decodes editing JSON and encodes native KCES2 .kcmat data
func encodeKCMatJSON(data []byte) ([]byte, error) {
	return encodeKCMatJSONWithOptions(data, nil)
}

// encodeKCMatJSONWithOptions 严格解码编辑 JSON 并按指定查找字段选项编码原生 KCES2 .kcmat 数据
// encodeKCMatJSONWithOptions strictly decodes editing JSON and encodes native KCES2 .kcmat data with the selected lookup-field options
func encodeKCMatJSONWithOptions(data []byte, options *serializationKCES.LookupHashOptions) ([]byte, error) {
	var value *serializationKCES.Material
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES2 kcmat JSON"); err != nil {
		return nil, fmt.Errorf("parse KCES2 kcmat JSON: %w", err)
	}
	return serializationKCES.EncodeKCMatWithOptions(value, options)
}

// writeKCMatFile 编码并直接写入原生 KCES2 .kcmat 数据
// writeKCMatFile encodes and directly writes native KCES2 .kcmat data
func writeKCMatFile(path string, value *serializationKCES.Material) error {
	encoded, err := serializationKCES.EncodeKCMatWithOptions(value, kcMatLookupHashOptions(path))
	if err != nil {
		return fmt.Errorf("encode KCES2 kcmat: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write KCES2 .kcmat file %q: %w", path, err)
	}
	return nil
}

// kcMatLookupHashOptions 根据目标 .kcmat 文件路径生成写出时使用的查找字段选项
// kcMatLookupHashOptions builds lookup-field options for writing from the destination .kcmat path
func kcMatLookupHashOptions(path string) *serializationKCES.LookupHashOptions {
	return &serializationKCES.LookupHashOptions{RecalculateHash: true, FileName: kcMatWireFileName(path)}
}

// kcMatWireFileName 从目标 .kcmat 路径推导游戏写入 Material.fileName 的逻辑名称
// kcMatWireFileName derives the logical name written to Material.fileName from a destination .kcmat path
func kcMatWireFileName(path string) string {
	fileName := filepath.Base(path)
	extension := filepath.Ext(fileName)
	if strings.EqualFold(extension, kcMatExtension) {
		return fileName[:len(fileName)-len(extension)]
	}
	return fileName
}

// writeKCMatConversionOutput 在上下文有效且大小不超限时写入 KCES2 .kcmat 转换结果
// writeKCMatConversionOutput writes KCES2 .kcmat conversion output while the context is active and the size remains within the limit
func writeKCMatConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive KCES2 .kcmat conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: KCES2 .kcmat conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write KCES2 .kcmat conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
