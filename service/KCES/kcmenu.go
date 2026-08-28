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

// KCMenuService 专门处理 KCES2 导出的单个 .kcmenu 文件 / KCMenuService handles individual .kcmenu files exported by KCES2
type KCMenuService struct{}

// IsKCESKCMenuFile 判断路径是否为 KCES2 单个 .kcmenu 文件
// IsKCESKCMenuFile reports whether a path names an individual KCES2 .kcmenu file
func IsKCESKCMenuFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), serializationKCES.KCMenuExtension)
}

// IsKCESKCMenuJSONFile 判断路径是否为 KCES2 单个 .kcmenu 的编辑 JSON
// IsKCESKCMenuJSONFile reports whether a path names editing JSON for an individual KCES2 .kcmenu file
func IsKCESKCMenuJSONFile(path string) bool {
	return partsExtFromJSONPath(path) == serializationKCES.KCMenuExtension
}

// ReadKCMenuFile 读取并解码 KCES2 单个 .kcmenu 文件
// ReadKCMenuFile reads and decodes an individual KCES2 .kcmenu file
func (s *KCMenuService) ReadKCMenuFile(path string) (*serializationKCES.Menu, error) {
	return readKCMenuFile(path)
}

// WriteKCMenuFile 编码并写入 KCES2 单个 .kcmenu 文件，同时按目标文件名重算可确定的查找字段
// WriteKCMenuFile encodes and writes an individual KCES2 .kcmenu file while recalculating determinable lookup fields from the destination filename
func (s *KCMenuService) WriteKCMenuFile(path string, value *serializationKCES.Menu) error {
	return writeKCMenuFile(path, value)
}

// ConvertKCMenuToJson 将 KCES2 单个 .kcmenu 文件转换为编辑 JSON
// ConvertKCMenuToJson converts an individual KCES2 .kcmenu file to editing JSON
func (s *KCMenuService) ConvertKCMenuToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadKCMenuFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal KCES2 .kcmenu JSON: %w", err)
	}
	return writeKCMenuConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToKCMenu 将编辑 JSON 转换为 KCES2 单个 .kcmenu 文件，并按目标文件名重算可确定的查找字段
// ConvertJsonToKCMenu converts editing JSON to an individual KCES2 .kcmenu file and recalculates determinable lookup fields from the destination filename
func (s *KCMenuService) ConvertJsonToKCMenu(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES2 .kcmenu JSON %q: %w", inputPath, err)
	}
	encoded, err := encodeKCMenuJSONWithOptions(data, kcMenuLookupHashOptions(outputPath))
	if err != nil {
		return err
	}
	return writeKCMenuConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// readKCMenuFile 读取并解码原生 KCES2 .kcmenu 数据
// readKCMenuFile reads and decodes native KCES2 .kcmenu data
func readKCMenuFile(path string) (*serializationKCES.Menu, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return serializationKCES.DecodeKCMenu(data)
}

// encodeKCMenuJSON 严格解码编辑 JSON 并编码原生 KCES2 .kcmenu 数据
// encodeKCMenuJSON strictly decodes editing JSON and encodes native KCES2 .kcmenu data
func encodeKCMenuJSON(data []byte) ([]byte, error) {
	return encodeKCMenuJSONWithOptions(data, nil)
}

// encodeKCMenuJSONWithOptions 严格解码编辑 JSON 并按指定查找字段选项编码原生 KCES2 .kcmenu 数据
// encodeKCMenuJSONWithOptions strictly decodes editing JSON and encodes native KCES2 .kcmenu data with the selected lookup-field options
func encodeKCMenuJSONWithOptions(data []byte, options *serializationKCES.LookupHashOptions) ([]byte, error) {
	var value *serializationKCES.Menu
	if err := decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES2 kcmenu JSON"); err != nil {
		return nil, fmt.Errorf("parse KCES2 kcmenu JSON: %w", err)
	}
	return serializationKCES.EncodeKCMenuWithOptions(value, options)
}

// writeKCMenuFile 编码并直接写入原生 KCES2 .kcmenu 数据
// writeKCMenuFile encodes and directly writes native KCES2 .kcmenu data
func writeKCMenuFile(path string, value *serializationKCES.Menu) error {
	encoded, err := serializationKCES.EncodeKCMenuWithOptions(value, kcMenuLookupHashOptions(path))
	if err != nil {
		return fmt.Errorf("encode KCES2 kcmenu: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write KCES2 .kcmenu file %q: %w", path, err)
	}
	return nil
}

// kcMenuLookupHashOptions 根据目标 .kcmenu 文件路径生成写出时使用的查找字段选项
// kcMenuLookupHashOptions builds lookup-field options for writing from the destination .kcmenu path
func kcMenuLookupHashOptions(path string) *serializationKCES.LookupHashOptions {
	return &serializationKCES.LookupHashOptions{RecalculateHash: true, FileName: filepath.Base(path)}
}

// writeKCMenuConversionOutput 在上下文有效且大小不超限时写入 KCES2 .kcmenu 转换结果
// writeKCMenuConversionOutput writes KCES2 .kcmenu conversion output while the context is active and the size remains within the limit
func writeKCMenuConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive KCES2 .kcmenu conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: KCES2 .kcmenu conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write KCES2 .kcmenu conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
