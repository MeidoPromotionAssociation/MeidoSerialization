package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

// PersetService 专门处理 KCES 早期使用且 wire 布局与 .preset 完全相同的 .perset 文件 / PersetService handles the historical KCES .perset files whose wire layout is identical to .preset
type PersetService struct{}

// IsKCESPersetFile 通过扩展名和 VirtualDirectory wire 签名识别 KCES .perset 文件
// IsKCESPersetFile recognizes a KCES .perset file by its extension and VirtualDirectory wire signature
func IsKCESPersetFile(path string) bool {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, serializationKCES.KCESPersetExtension) || strings.HasSuffix(lower, ".json") {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	header := make([]byte, 7)
	if _, err := io.ReadFull(file, header); err != nil {
		return false
	}
	return serializationKCES.IsKCESPresetData(header)
}

// IsKCESPersetJSONFile 通过 .perset.json 双扩展名和格式标记识别编辑 JSON
// IsKCESPersetJSONFile recognizes editing JSON by its .perset.json double extension and format marker
func IsKCESPersetJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), serializationKCES.KCESPersetExtension+".json") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return hasKCESPresetEditingJSONRoot(data)
}

// ReadPersetFile 直接读取并完整展开 KCES .perset 文件中的已知内部块
// ReadPersetFile directly reads a KCES .perset file and fully expands its known inner blocks
func (s *PersetService) ReadPersetFile(path string) (*serializationKCES.ExpandedKCESPreset, error) {
	if !strings.HasSuffix(strings.ToLower(path), serializationKCES.KCESPersetExtension) {
		return nil, fmt.Errorf("not a .perset file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read KCES .perset file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeExpandedKCESPreset(data)
	if err != nil {
		return nil, fmt.Errorf("decode KCES .perset file %q: %w", path, err)
	}
	return value, nil
}

// WritePersetFile 直接编码完整展开的 KCES 预设并写入 .perset 文件
// WritePersetFile directly encodes a fully expanded KCES preset and writes it to a .perset file
func (s *PersetService) WritePersetFile(path string, value *serializationKCES.ExpandedKCESPreset) error {
	if !strings.HasSuffix(strings.ToLower(path), serializationKCES.KCESPersetExtension) {
		return fmt.Errorf("not a .perset output path: %s", path)
	}
	encoded, err := serializationKCES.EncodeExpandedKCESPreset(value)
	if err != nil {
		return fmt.Errorf("encode KCES .perset file: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write KCES .perset file %q: %w", path, err)
	}
	return nil
}

// ConvertPersetToJson 将 KCES .perset 文件转换为完整展开的编辑 JSON
// ConvertPersetToJson converts a KCES .perset file to fully expanded editing JSON
func (s *PersetService) ConvertPersetToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadPersetFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES .perset JSON: %w", err)
	}
	data = append(data, '\n')
	return writePersetConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToPerset 将完整展开的编辑 JSON 转换为 KCES .perset 文件
// ConvertJsonToPerset converts fully expanded editing JSON to a KCES .perset file
func (s *PersetService) ConvertJsonToPerset(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if !strings.HasSuffix(strings.ToLower(outputPath), serializationKCES.KCESPersetExtension) {
		return fmt.Errorf("not a .perset output path: %s", outputPath)
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES .perset JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.ExpandedKCESPreset
	if err := decodeStrictJSON(data, &value, "KCES .perset JSON"); err != nil {
		return fmt.Errorf("parse KCES .perset JSON: %w", err)
	}
	encoded, err := serializationKCES.EncodeExpandedKCESPreset(&value)
	if err != nil {
		return fmt.Errorf("encode KCES .perset file: %w", err)
	}
	return writePersetConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// writePersetConversionOutput 在上下文有效且大小不超限时写入 .perset 转换结果
// writePersetConversionOutput writes .perset conversion output while the context is active and the size remains within the limit
func writePersetConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .perset conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .perset conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .perset conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
