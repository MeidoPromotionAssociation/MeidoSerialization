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

const kcModelExtension = serializationKCES.KCModelExtension

// KCModelService 专门处理 KCES2 导出的单个 .kcmodel 文件 / KCModelService handles individual .kcmodel files exported by KCES2
type KCModelService struct{}

// IsKCESKCModelFile 判断路径是否为 KCES2 单个 .kcmodel 文件
// IsKCESKCModelFile reports whether a path names an individual KCES2 .kcmodel file
func IsKCESKCModelFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), kcModelExtension)
}

// IsKCESKCModelJSONFile 判断路径是否为 KCES2 单个 .kcmodel 的编辑 JSON
// IsKCESKCModelJSONFile reports whether a path names editing JSON for an individual KCES2 .kcmodel file
func IsKCESKCModelJSONFile(path string) bool {
	return partsExtFromJSONPath(path) == kcModelExtension
}

// ReadKCModelFile 读取并解码 KCES2 单个 .kcmodel 文件
// ReadKCModelFile reads and decodes an individual KCES2 .kcmodel file
func (s *KCModelService) ReadKCModelFile(path string) (*serializationKCES.Model, error) {
	return readKCModelFile(path)
}

// WriteKCModelFile 编码并写入 KCES2 单个 .kcmodel 文件，同时按目标文件名重算可确定的查找字段
// WriteKCModelFile encodes and writes an individual KCES2 .kcmodel file while recalculating determinable lookup fields from the destination filename
func (s *KCModelService) WriteKCModelFile(path string, value *serializationKCES.Model) error {
	return writeKCModelFile(path, value)
}

// ConvertKCModelToJson 将 KCES2 单个 .kcmodel 文件转换为编辑 JSON
// ConvertKCModelToJson converts an individual KCES2 .kcmodel file to editing JSON
func (s *KCModelService) ConvertKCModelToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadKCModelFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal KCES2 .kcmodel JSON: %w", err)
	}
	return writeKCModelConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToKCModel 将编辑 JSON 转换为 KCES2 单个 .kcmodel 文件，并按目标文件名重算可确定的查找字段
// ConvertJsonToKCModel converts editing JSON to an individual KCES2 .kcmodel file and recalculates determinable lookup fields from the destination filename
func (s *KCModelService) ConvertJsonToKCModel(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES2 .kcmodel JSON %q: %w", inputPath, err)
	}
	encoded, err := encodeKCModelJSONWithOptions(data, kcModelLookupHashOptions(outputPath))
	if err != nil {
		return err
	}
	return writeKCModelConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// readKCModelFile 读取并解码原生 KCES2 .kcmodel 数据
// readKCModelFile reads and decodes native KCES2 .kcmodel data
func readKCModelFile(path string) (*serializationKCES.Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return serializationKCES.DecodeKCModel(data)
}

// encodeKCModelJSON 严格解码编辑 JSON 并编码原生 KCES2 .kcmodel 数据
// encodeKCModelJSON strictly decodes editing JSON and encodes native KCES2 .kcmodel data
func encodeKCModelJSON(data []byte) ([]byte, error) {
	return encodeKCModelJSONWithOptions(data, nil)
}

// encodeKCModelJSONWithOptions 严格解码编辑 JSON 并按指定查找字段选项编码原生 KCES2 .kcmodel 数据
// encodeKCModelJSONWithOptions strictly decodes editing JSON and encodes native KCES2 .kcmodel data with the selected lookup-field options
func encodeKCModelJSONWithOptions(data []byte, options *serializationKCES.LookupHashOptions) ([]byte, error) {
	var value *serializationKCES.Model
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES2 kcmodel JSON"); err != nil {
		return nil, fmt.Errorf("parse KCES2 kcmodel JSON: %w", err)
	}
	return serializationKCES.EncodeKCModelWithOptions(value, options)
}

// writeKCModelFile 编码并直接写入原生 KCES2 .kcmodel 数据
// writeKCModelFile encodes and directly writes native KCES2 .kcmodel data
func writeKCModelFile(path string, value *serializationKCES.Model) error {
	encoded, err := serializationKCES.EncodeKCModelWithOptions(value, kcModelLookupHashOptions(path))
	if err != nil {
		return fmt.Errorf("encode KCES2 kcmodel: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write KCES2 .kcmodel file %q: %w", path, err)
	}
	return nil
}

// kcModelLookupHashOptions 根据目标 .kcmodel 文件路径生成写出时使用的查找字段选项
// kcModelLookupHashOptions builds lookup-field options for writing from the destination .kcmodel path
func kcModelLookupHashOptions(path string) *serializationKCES.LookupHashOptions {
	return &serializationKCES.LookupHashOptions{RecalculateHash: true, FileName: kcModelWireFileName(path)}
}

// kcModelWireFileName 从目标 .kcmodel 路径推导游戏写入 Model.fileName 的小写逻辑名称
// kcModelWireFileName derives the lower-case logical name written to Model.fileName from a destination .kcmodel path
func kcModelWireFileName(path string) string {
	fileName := filepath.Base(path)
	extension := filepath.Ext(fileName)
	if strings.EqualFold(extension, kcModelExtension) {
		fileName = fileName[:len(fileName)-len(extension)]
	}
	return strings.ToLower(fileName)
}

// writeKCModelConversionOutput 在上下文有效且大小不超限时写入 KCES2 .kcmodel 转换结果
// writeKCModelConversionOutput writes KCES2 .kcmodel conversion output while the context is active and the size remains within the limit
func writeKCModelConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive KCES2 .kcmodel conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: KCES2 .kcmodel conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write KCES2 .kcmodel conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
