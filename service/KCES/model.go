package KCES

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

const modelExtension = ".model"

// ModelService 专门处理 KCES MessagePack .model 文件 / ModelService handles KCES MessagePack .model files
type ModelService struct{}

// IsKCESModelFile 通过完整解码区分 KCES 与共用扩展名的其他 .model 格式
// IsKCESModelFile distinguishes KCES from other .model formats sharing the extension by fully decoding the payload
func IsKCESModelFile(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), modelExtension) {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	_, err = serializationKCES.DecodeModel(data)
	return err == nil
}

// IsKCESModelJSONFile 通过 KCES 模型字段识别 .model 编辑 JSON
// IsKCESModelJSONFile recognizes .model editing JSON through KCES model fields
func IsKCESModelJSONFile(path string) bool {
	if partsExtFromJSONPath(path) != modelExtension {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	data = bytes.TrimSpace(trimJSONUTF8BOM(data))
	if bytes.Equal(data, []byte("null")) {
		return true
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return false
	}
	_, hasMeshFileName := object["meshfileName"]
	_, hasTransData := object["transData"]
	return hasMeshFileName || hasTransData
}

// ReadModelFile 读取并解码 KCES .model 文件
// ReadModelFile reads and decodes a KCES .model file
func (s *ModelService) ReadModelFile(path string) (*serializationKCES.Model, error) {
	return readModelFile(path)
}

// WriteModelFile 编码并写入 KCES .model 文件
// WriteModelFile encodes and writes a KCES .model file
func (s *ModelService) WriteModelFile(path string, value *serializationKCES.Model) error {
	return writeModelFile(path, value)
}

// ConvertModelToJson 将 KCES .model 文件转换为编辑 JSON
// ConvertModelToJson converts a KCES .model file to editing JSON
func (s *ModelService) ConvertModelToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadModelFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal KCES .model JSON: %w", err)
	}
	return writeModelConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToModel 将编辑 JSON 转换为 KCES .model 文件
// ConvertJsonToModel converts editing JSON to a KCES .model file
func (s *ModelService) ConvertJsonToModel(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES .model JSON %q: %w", inputPath, err)
	}
	encoded, err := encodeModelJSON(data)
	if err != nil {
		return err
	}
	return writeModelConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WriteModelFile 为聚合 service 保留 KCES .model 直接写入 API
// WriteModelFile preserves the direct KCES .model writer on the aggregate service
func (s *PartsService) WriteModelFile(path string, value *serializationKCES.Model) error {
	return writeModelFile(path, value)
}

// readModelFile 读取并解码原生 KCES .model 数据
// readModelFile reads and decodes native KCES .model data
func readModelFile(path string) (*serializationKCES.Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return serializationKCES.DecodeModel(data)
}

// encodeModelJSON 严格解码编辑 JSON 并编码原生 KCES .model 数据
// encodeModelJSON strictly decodes editing JSON and encodes native KCES .model data
func encodeModelJSON(data []byte) ([]byte, error) {
	var value *serializationKCES.Model
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES model JSON"); err != nil {
		return nil, fmt.Errorf("parse model json: %w", err)
	}
	return serializationKCES.EncodeModel(value)
}

// writeModelFile 编码并直接写入原生 KCES .model 数据
// writeModelFile encodes and directly writes native KCES .model data
func writeModelFile(path string, value *serializationKCES.Model) error {
	encoded, err := serializationKCES.EncodeModel(value)
	if err != nil {
		return fmt.Errorf("encode KCES model: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write KCES .model file %q: %w", path, err)
	}
	return nil
}

// writeModelConversionOutput 在上下文有效且大小不超限时写入 KCES .model 转换结果
// writeModelConversionOutput writes KCES .model conversion output while the context is active and the size remains within the limit
func writeModelConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive KCES .model conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: KCES .model conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write KCES .model conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
