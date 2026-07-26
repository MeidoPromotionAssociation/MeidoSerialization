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

// SystemDataService 转换基于 VirtualDirectory 的 system.dat，并以 typed JSON 展开已知 EditData 且以 base64 保留未知文件 / SystemDataService converts VirtualDirectory-based system.dat files while exposing known EditData as typed JSON and preserving unknown files as base64
type SystemDataService struct{}

// IsKCESSystemDataFile 判断路径是否为原生 system.dat 文件
// IsKCESSystemDataFile reports whether a path names a native system.dat file
func IsKCESSystemDataFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Base(path), "system.dat")
}

// IsKCESSystemDataJSONFile 判断路径是否为带正确格式标记的 system.dat 编辑 JSON
// IsKCESSystemDataJSONFile reports whether a path names system.dat editing JSON with the correct format marker
func IsKCESSystemDataJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), "system.dat.json") {
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
	return header.Format == serializationKCES.KCESSystemDataFormat
}

// ReadSystemDataFile 读取并解码 KCES system.dat 文件
// ReadSystemDataFile reads and decodes a KCES system.dat file
func (s *SystemDataService) ReadSystemDataFile(path string) (*serializationKCES.KCESSystemData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read KCES system.dat %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESSystemData(data)
	if err != nil {
		return nil, fmt.Errorf("parse KCES system.dat %q: %w", path, err)
	}
	return value, nil
}

// ConvertSystemDataToJSON 将 KCES system.dat 转换为编辑 JSON
// ConvertSystemDataToJSON converts KCES system.dat to editing JSON
func (s *SystemDataService) ConvertSystemDataToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadSystemDataFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES system.dat JSON: %w", err)
	}
	data = append(data, '\n')
	return writeSystemDataConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJSONToSystemData 将编辑 JSON 转换为 KCES system.dat
// ConvertJSONToSystemData converts editing JSON to KCES system.dat
func (s *SystemDataService) ConvertJSONToSystemData(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES system.dat JSON %q: %w", inputPath, err)
	}
	value, err := decodeKCESSystemDataEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse KCES system.dat JSON %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeKCESSystemData(value)
	if err != nil {
		return fmt.Errorf("encode KCES system.dat JSON %q: %w", inputPath, err)
	}
	return writeSystemDataConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// decodeKCESSystemDataEditingJSON 严格解码 system.dat 编辑 JSON 并校验完整 wire 约束
// decodeKCESSystemDataEditingJSON strictly decodes system.dat editing JSON and validates complete wire constraints
func decodeKCESSystemDataEditingJSON(data []byte) (*serializationKCES.KCESSystemData, error) {
	var value serializationKCES.KCESSystemData
	if err := decodeStrictJSON(data, &value, "KCES system.dat JSON"); err != nil {
		return nil, err
	}
	if value.Format != serializationKCES.KCESSystemDataFormat {
		return nil, fmt.Errorf("unsupported KCES system.dat JSON format %q", value.Format)
	}
	if _, err := serializationKCES.EncodeKCESSystemData(&value); err != nil {
		return nil, err
	}
	return &value, nil
}

// WriteSystemDataFile 将 KCES 系统数据结构直接编码并写入 system.dat 文件
// WriteSystemDataFile directly encodes a KCES system data value and writes it to a system.dat file
func (s *SystemDataService) WriteSystemDataFile(path string, value *serializationKCES.KCESSystemData) error {
	encoded, err := serializationKCES.EncodeKCESSystemData(value)
	if err != nil {
		return fmt.Errorf("encode KCES system.dat: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write system.dat file %q: %w", path, err)
	}
	return nil
}

// writeSystemDataConversionOutput 在上下文有效且大小不超限时写入 system.dat 转换结果
// writeSystemDataConversionOutput writes system.dat conversion output while the context is active and the size remains within the limit
func writeSystemDataConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive system.dat conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: system.dat conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write system.dat conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
