package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// PresetService 专门处理 KCES 引入的 .preset VirtualDirectory 格式，旧 CM3D2_PRESET 继续由 service/COM3D2 处理 / PresetService handles the KCES .preset VirtualDirectory format while legacy CM3D2_PRESET files remain in service/COM3D2
type PresetService struct{}

// IsKCESPresetFile 通过 wire 签名识别当前 KCES 预设，而不依赖与旧格式共用的 .preset 扩展名
// IsKCESPresetFile distinguishes current KCES presets by their wire signature instead of relying on the shared .preset extension
func IsKCESPresetFile(path string) bool {
	lower := strings.ToLower(path)
	if (!strings.HasSuffix(lower, serializationKCES.KCESPresetExtension) && !strings.HasSuffix(lower, serializationKCES.KCESPersetExtension)) || strings.HasSuffix(lower, ".json") {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	header := make([]byte, 7)
	if _, err := io.ReadFull(f, header); err != nil {
		return false
	}
	return serializationKCES.IsKCESPresetData(header)
}

// IsKCESPresetJSONFile 通过明确的格式标记区分 KCES 预设 JSON 与旧版预设 JSON
// IsKCESPresetJSONFile distinguishes KCES preset JSON from legacy preset JSON through the explicit format marker
func IsKCESPresetJSONFile(path string) bool {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, serializationKCES.KCESPresetExtension+".json") && !strings.HasSuffix(lower, serializationKCES.KCESPersetExtension+".json") {
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
	return header.Format == serializationKCES.KCESPresetFormat
}

// ReadPresetFile 读取并完整展开 KCES 预设中的三个已知内部块
// ReadPresetFile reads a KCES preset and fully expands its three known inner blocks
func (s *PresetService) ReadPresetFile(path string) (*serializationKCES.ExpandedKCESPreset, error) {
	if strings.HasSuffix(strings.ToLower(path), serializationKCES.KCESPersetExtension) {
		return (&PersetService{}).ReadPersetFile(path)
	}
	if !strings.HasSuffix(strings.ToLower(path), serializationKCES.KCESPresetExtension) {
		return nil, fmt.Errorf("not a .preset file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read KCES preset %q: %w", path, err)
	}
	preset, err := serializationKCES.DecodeExpandedKCESPreset(data)
	if err != nil {
		return nil, fmt.Errorf("parse KCES preset %q: %w", path, err)
	}
	return preset, nil
}

// ConvertPresetToJson 将 KCES 预设文件转换为完整展开的编辑 JSON
// ConvertPresetToJson converts a KCES preset file to fully expanded editing JSON
func (s *PresetService) ConvertPresetToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if strings.HasSuffix(strings.ToLower(inputPath), serializationKCES.KCESPersetExtension) {
		return (&PersetService{}).ConvertPersetToJson(ctx, inputPath, outputPath, maxOutputBytes)
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	preset, err := s.ReadPresetFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES preset JSON: %w", err)
	}
	data = append(data, '\n')
	return writePresetConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToPreset 将完整展开的编辑 JSON 转换为 KCES 预设文件
// ConvertJsonToPreset converts fully expanded editing JSON to a KCES preset file
func (s *PresetService) ConvertJsonToPreset(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if strings.HasSuffix(strings.ToLower(outputPath), serializationKCES.KCESPersetExtension) {
		return (&PersetService{}).ConvertJsonToPerset(ctx, inputPath, outputPath, maxOutputBytes)
	}
	if !strings.HasSuffix(strings.ToLower(outputPath), serializationKCES.KCESPresetExtension) {
		return fmt.Errorf("not a .preset output path: %s", outputPath)
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	var preset serializationKCES.ExpandedKCESPreset
	if err := decodeStrictJSON(data, &preset, "KCES preset JSON"); err != nil {
		return fmt.Errorf("parse KCES preset JSON: %w", err)
	}
	if preset.Format != serializationKCES.KCESPresetFormat {
		return fmt.Errorf("unsupported KCES preset JSON format %q", preset.Format)
	}
	encoded, err := serializationKCES.EncodeExpandedKCESPreset(&preset)
	if err != nil {
		return err
	}
	return writePresetConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WritePresetFile 将完整展开的 KCES 预设直接编码并写入 .preset 文件，并为旧调用方分派 .perset 输出
// WritePresetFile directly encodes a fully expanded KCES preset to .preset while dispatching .perset output for legacy callers
func (s *PresetService) WritePresetFile(path string, value *serializationKCES.ExpandedKCESPreset) error {
	if strings.HasSuffix(strings.ToLower(path), serializationKCES.KCESPersetExtension) {
		return (&PersetService{}).WritePersetFile(path, value)
	}
	if !strings.HasSuffix(strings.ToLower(path), serializationKCES.KCESPresetExtension) {
		return fmt.Errorf("not a .preset output path: %s", path)
	}
	encoded, err := serializationKCES.EncodeExpandedKCESPreset(value)
	if err != nil {
		return fmt.Errorf("encode KCES preset: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write KCES preset file %q: %w", path, err)
	}
	return nil
}

// writePresetConversionOutput 在上下文有效且大小不超限时写入 KCES 预设转换结果
// writePresetConversionOutput writes KCES preset conversion output while the context is active and the size remains within the limit
func writePresetConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive KCES preset conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: KCES preset conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write KCES preset conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
