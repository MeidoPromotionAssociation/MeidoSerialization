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

// MaidColliderService 专门处理 maid_collider.bytes 与 maid_collider_touch.bytes 文件 / MaidColliderService handles maid_collider.bytes and maid_collider_touch.bytes files
type MaidColliderService struct{}

// IsKCESMaidColliderFile 判断路径是否为原生女仆碰撞体载荷
// IsKCESMaidColliderFile reports whether a path names a native maid-collider payload
func IsKCESMaidColliderFile(path string) bool {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	return isMaidColliderBaseName(filepath.Base(path))
}

// IsKCESMaidColliderJSONFile 判断路径是否为女仆碰撞体编辑 JSON
// IsKCESMaidColliderJSONFile reports whether a path names maid-collider editing JSON
func IsKCESMaidColliderJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	if !isMaidColliderBaseName(filepath.Base(base)) {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var value serializationKCES.MaidColliderFile
	return decodeStrictJSON(trimJSONUTF8BOM(data), &value, "KCES maid collider JSON") == nil
}

// isMaidColliderBaseName 判断文件名是否为游戏使用的女仆碰撞体固定名称
// isMaidColliderBaseName reports whether a filename is one of the fixed maid-collider names used by the game
func isMaidColliderBaseName(name string) bool {
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, ".bytes")
	return name == "maid_collider" || name == "maid_collider_touch"
}

// ReadMaidColliderFile 读取并解码女仆碰撞体载荷
// ReadMaidColliderFile reads and decodes a maid-collider payload
func (s *MaidColliderService) ReadMaidColliderFile(path string) (*serializationKCES.MaidColliderFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read maid collider file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeMaidCollider(data)
	if err != nil {
		return nil, fmt.Errorf("decode maid collider file %q: %w", path, err)
	}
	return value, nil
}

// ConvertMaidColliderToJSON 将女仆碰撞体载荷转换为编辑 JSON
// ConvertMaidColliderToJSON converts a maid-collider payload to editing JSON
func (s *MaidColliderService) ConvertMaidColliderToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadMaidColliderFile(inputPath)
	if err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES maid collider JSON: %w", err)
	}
	jsonData = append(jsonData, '\n')
	return writeMaidColliderConversionOutput(ctx, outputPath, jsonData, maxOutputBytes)
}

// ConvertJSONToMaidCollider 将编辑 JSON 转换为女仆碰撞体载荷
// ConvertJSONToMaidCollider converts editing JSON to a maid-collider payload
func (s *MaidColliderService) ConvertJSONToMaidCollider(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	var value serializationKCES.MaidColliderFile
	if err := decodeStrictJSON(data, &value, "KCES maid collider JSON"); err != nil {
		return fmt.Errorf("parse KCES maid collider JSON: %w", err)
	}
	encoded, err := serializationKCES.EncodeMaidCollider(&value)
	if err != nil {
		return err
	}
	return writeMaidColliderConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// WriteMaidColliderFile 将女仆胶囊碰撞体结构直接编码并写入原生载荷文件
// WriteMaidColliderFile directly encodes a maid capsule collider value and writes it to a native payload file
func (s *MaidColliderService) WriteMaidColliderFile(path string, value *serializationKCES.MaidColliderFile) error {
	encoded, err := serializationKCES.EncodeMaidCollider(value)
	if err != nil {
		return fmt.Errorf("encode KCES maid collider: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write maid collider file %q: %w", path, err)
	}
	return nil
}

// writeMaidColliderConversionOutput 在上下文有效且大小不超限时写入女仆碰撞体转换结果
// writeMaidColliderConversionOutput writes maid-collider conversion output while the context is active and the size remains within the limit
func writeMaidColliderConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive maid collider conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: maid collider conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write maid collider conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
