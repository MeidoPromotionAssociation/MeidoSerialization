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

// SavedAttachService 在 KCES GP03 SAVED_ATTACH_DATA .sad 文件与带格式标记的编辑 JSON 之间转换 / SavedAttachService converts between KCES GP03 SAVED_ATTACH_DATA .sad files and marker-based editing JSON
type SavedAttachService struct{}

// savedAttachEditingJSON 使用指针字段区分 .sad 编辑 JSON 中的缺失值和显式零值 / savedAttachEditingJSON uses pointer fields to distinguish missing values from explicit zero values in .sad editing JSON
type savedAttachEditingJSON struct {
	Signature *string                              `json:"signature"`
	Version   *int32                               `json:"version"`
	Items     *[]serializationKCES.SavedAttachData `json:"items"`
}

// IsKCESSavedAttachFile 判断路径是否为原生 .sad 文件
// IsKCESSavedAttachFile reports whether a path names a native .sad file
func IsKCESSavedAttachFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), ".sad")
}

// IsKCESSavedAttachJSONFile 判断路径是否为 .sad 编辑 JSON
// IsKCESSavedAttachJSONFile reports whether a path names .sad editing JSON
func IsKCESSavedAttachJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.EqualFold(filepath.Ext(base), ".sad")
}

// ReadSavedAttachFile 读取并解码原生 .sad 文件
// ReadSavedAttachFile reads and decodes a native .sad file
func (s *SavedAttachService) ReadSavedAttachFile(path string) (*serializationKCES.SavedAttachFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read saved-attach file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeSavedAttach(data)
	if err != nil {
		return nil, fmt.Errorf("decode saved-attach file %q: %w", path, err)
	}
	return value, nil
}

// ConvertSavedAttachToJSON 将原生 .sad 文件转换为编辑 JSON
// ConvertSavedAttachToJSON converts a native .sad file to editing JSON
func (s *SavedAttachService) ConvertSavedAttachToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadSavedAttachFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal saved-attach JSON: %w", err)
	}
	data = append(data, '\n')
	return writeSavedAttachConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJSONToSavedAttach 将编辑 JSON 转换为原生 .sad 文件
// ConvertJSONToSavedAttach converts editing JSON to a native .sad file
func (s *SavedAttachService) ConvertJSONToSavedAttach(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read saved-attach JSON %q: %w", inputPath, err)
	}
	value, err := decodeSavedAttachEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse saved-attach JSON %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeSavedAttach(value)
	if err != nil {
		return fmt.Errorf("encode saved-attach JSON %q: %w", inputPath, err)
	}
	return writeSavedAttachConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// decodeSavedAttachEditingJSON 严格解码 .sad 编辑 JSON 并校验必需字段和 wire 约束
// decodeSavedAttachEditingJSON strictly decodes .sad editing JSON and validates required fields and wire constraints
func decodeSavedAttachEditingJSON(data []byte) (*serializationKCES.SavedAttachFile, error) {
	var editing savedAttachEditingJSON
	if err := decodeStrictJSON(data, &editing, "saved-attach JSON"); err != nil {
		return nil, err
	}
	if editing.Signature == nil {
		return nil, fmt.Errorf("signature is missing or null")
	}
	if editing.Version == nil {
		return nil, fmt.Errorf("version is missing or null")
	}
	if *editing.Signature != serializationKCES.SavedAttachSignature {
		return nil, fmt.Errorf("invalid saved-attach signature %q", *editing.Signature)
	}
	var items []serializationKCES.SavedAttachData
	if editing.Items != nil {
		items = *editing.Items
	}
	value := serializationKCES.SavedAttachFile{
		Signature: *editing.Signature,
		Version:   *editing.Version,
		Items:     items,
	}
	if _, err := serializationKCES.EncodeSavedAttach(&value); err != nil {
		return nil, err
	}
	return &value, nil
}

// WriteSavedAttachFile 将保存的附着物结构直接编码并写入 .sad 文件
// WriteSavedAttachFile directly encodes a saved-attach value and writes it to a .sad file
func (s *SavedAttachService) WriteSavedAttachFile(path string, value *serializationKCES.SavedAttachFile) error {
	encoded, err := serializationKCES.EncodeSavedAttach(value)
	if err != nil {
		return fmt.Errorf("encode KCES saved-attach data: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .sad file %q: %w", path, err)
	}
	return nil
}

// writeSavedAttachConversionOutput 在上下文有效且大小不超限时写入 .sad 转换结果
// writeSavedAttachConversionOutput writes .sad conversion output while the context is active and the size remains within the limit
func writeSavedAttachConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .sad conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .sad conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .sad conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
