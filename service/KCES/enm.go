package KCES

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

// IsKCESExportNameMapFile 判断路径是否为原生 export_map.enm 风格的 Unity JsonUtility 文档，并允许内容探测继续校验已改名的 .enm 副本
// IsKCESExportNameMapFile reports whether a path names a native export_map.enm-style Unity JsonUtility document while allowing content probing to validate renamed .enm copies
func IsKCESExportNameMapFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), ".enm")
}

// IsKCESExportNameMapJSONFile 通过 .enm.json 双扩展名识别并完整校验 .enm 编辑文档
// IsKCESExportNameMapJSONFile recognizes an .enm editing document by its .enm.json double extension and fully validates it
func IsKCESExportNameMapJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".enm.json") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	_, err = serializationKCES.DecodeKCESExportNameMapJSON(data)
	return err == nil
}

// ExportNameMapService 在游戏的嵌套 JsonUtility 布局与确定性的条目式编辑 JSON 之间转换 / ExportNameMapService converts between the game's nested JsonUtility layout and the deterministic entry-based editing JSON representation
type ExportNameMapService struct{}

// ReadExportNameMapFile 读取并解码 .enm 文件
// ReadExportNameMapFile reads and decodes an .enm file
func (s *ExportNameMapService) ReadExportNameMapFile(path string) (*serializationKCES.KCESExportNameMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .enm file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESExportNameMap(data)
	if err != nil {
		return nil, fmt.Errorf("decode .enm file %q: %w", path, err)
	}
	return value, nil
}

// ConvertExportNameMapToJSON 将 .enm 文件转换为编辑 JSON
// ConvertExportNameMapToJSON converts an .enm file to editing JSON
func (s *ExportNameMapService) ConvertExportNameMapToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadExportNameMapFile(inputPath)
	if err != nil {
		return err
	}
	encoded, err := serializationKCES.EncodeKCESExportNameMapJSON(value)
	if err != nil {
		return fmt.Errorf("encode KCES export name map editing JSON: %w", err)
	}
	return writeExportNameMapConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// ConvertJSONToExportNameMap 将编辑 JSON 转换为 .enm 文件
// ConvertJSONToExportNameMap converts editing JSON to an .enm file
func (s *ExportNameMapService) ConvertJSONToExportNameMap(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES export name map JSON %q: %w", inputPath, err)
	}
	value, err := serializationKCES.DecodeKCESExportNameMapJSON(data)
	if err != nil {
		return fmt.Errorf("decode KCES export name map JSON %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeKCESExportNameMap(value)
	if err != nil {
		return fmt.Errorf("encode native KCES export name map: %w", err)
	}
	return writeExportNameMapConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WriteExportNameMapFile 将导出名称映射结构直接编码并写入原生 .enm 文件
// WriteExportNameMapFile directly encodes an export name map value and writes it to a native .enm file
func (s *ExportNameMapService) WriteExportNameMapFile(path string, value *serializationKCES.KCESExportNameMap) error {
	encoded, err := serializationKCES.EncodeKCESExportNameMap(value)
	if err != nil {
		return fmt.Errorf("encode KCES export name map: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .enm file %q: %w", path, err)
	}
	return nil
}

// writeExportNameMapConversionOutput 在上下文有效且大小不超限时写入 .enm 转换结果
// writeExportNameMapConversionOutput writes .enm conversion output while the context is active and the size remains within the limit
func writeExportNameMapConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .enm conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .enm conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .enm conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
