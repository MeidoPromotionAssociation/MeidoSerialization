package KCES

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// GP03BridgeEditing 表示 .brd 的完整 typed editing JSON，空长度块以 null 表示 / GP03BridgeEditing represents the fully typed editing JSON for .brd, with zero-length blocks represented as null
type GP03BridgeEditing struct {
	Format        string                                `json:"format"`        // JSON 表示格式标识 / JSON representation format identifier
	Signature     string                                `json:"signature"`     // 文件签名 GP03_BRIDGE / File signature GP03_BRIDGE
	Version       int32                                 `json:"version"`       // 外层桥接版本 2000 或 2001 / Outer bridge version 2000 or 2001
	GUID          string                                `json:"guid"`          // 传输角色的 GUID / GUID of the transferred character
	LegacyPreset  *serializationCOM3D2.Preset           `json:"legacyPreset"`  // v2001 的 COM3D2 preset，v2000 必须为 null / COM3D2 preset for v2001, required to be null for v2000
	CurrentPreset *serializationKCES.ExpandedKCESPreset `json:"currentPreset"` // 可空 KCES preset / Nullable KCES preset
}

// GP03BridgeService 在 .brd wire framing 与完整 typed editing JSON 之间转换 / GP03BridgeService converts between .brd wire framing and fully typed editing JSON
type GP03BridgeService struct{}

// IsKCESGP03BridgeFile 判断路径是否为原生 .brd 文件
// IsKCESGP03BridgeFile reports whether a path names a native .brd file
func IsKCESGP03BridgeFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), ".brd")
}

// IsKCESGP03BridgeJSONFile 判断路径是否为 .brd.json editing JSON
// IsKCESGP03BridgeJSONFile reports whether a path names .brd.json editing JSON
func IsKCESGP03BridgeJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.EqualFold(filepath.Ext(base), ".brd")
}

// ReadBridgeFile 读取 .brd 并完整解析两个已知预设块
// ReadBridgeFile reads .brd and fully decodes both known preset blocks
func (s *GP03BridgeService) ReadBridgeFile(path string) (*GP03BridgeEditing, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read GP03 bridge file %q: %w", path, err)
	}
	value, err := decodeGP03Bridge(data)
	if err != nil {
		return nil, fmt.Errorf("decode GP03 bridge file %q: %w", path, err)
	}
	return value, nil
}

// ConvertBridgeToJSON 将 .brd 转换为只含 typed preset 的 editing JSON
// ConvertBridgeToJSON converts .brd into editing JSON containing only typed presets
func (s *GP03BridgeService) ConvertBridgeToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadBridgeFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal GP03 bridge JSON: %w", err)
	}
	data = append(data, '\n')
	return writeBridgeConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJSONToBridge 从 typed editing JSON 重建两个预设块和 .brd framing
// ConvertJSONToBridge rebuilds both preset blocks and .brd framing from typed editing JSON
func (s *GP03BridgeService) ConvertJSONToBridge(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read GP03 bridge JSON %q: %w", inputPath, err)
	}
	value, err := decodeGP03BridgeEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse GP03 bridge JSON %q: %w", inputPath, err)
	}
	encoded, err := encodeGP03BridgeEditing(value)
	if err != nil {
		return fmt.Errorf("encode GP03 bridge JSON %q: %w", inputPath, err)
	}
	return writeBridgeConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// decodeGP03Bridge 解码 framing 后立即解析每个非空已知预设块
// decodeGP03Bridge decodes the framing and immediately parses every nonempty known preset block
func decodeGP03Bridge(data []byte) (*GP03BridgeEditing, error) {
	wire, err := serializationKCES.DecodeGP03Bridge(data)
	if err != nil {
		return nil, err
	}
	value := &GP03BridgeEditing{
		Format:    serializationKCES.KCESGP03BridgeFormat,
		Signature: wire.Signature,
		Version:   wire.Version,
		GUID:      wire.GUID,
	}
	if len(wire.LegacyPreset) != 0 {
		value.LegacyPreset, err = serializationCOM3D2.ReadPreset(bytes.NewReader(wire.LegacyPreset))
		if err != nil {
			return nil, fmt.Errorf("decode GP03 bridge legacyPreset as COM3D2 preset: %w", err)
		}
	}
	if len(wire.CurrentPreset) != 0 {
		value.CurrentPreset, err = serializationKCES.DecodeExpandedKCESPreset(wire.CurrentPreset)
		if err != nil {
			return nil, fmt.Errorf("decode GP03 bridge currentPreset as KCES preset: %w", err)
		}
	}
	return value, nil
}

// encodeGP03BridgeEditing 从 typed preset 重建长度分隔块并编码 framing
// encodeGP03BridgeEditing rebuilds the length-delimited blocks from typed presets and encodes the framing
func encodeGP03BridgeEditing(value *GP03BridgeEditing) ([]byte, error) {
	wire, err := collapseGP03BridgeEditing(value)
	if err != nil {
		return nil, err
	}
	return serializationKCES.EncodeGP03Bridge(wire)
}

// collapseGP03BridgeEditing 将 typed editing 模型转换为内部 wire framing
// collapseGP03BridgeEditing converts the typed editing model into internal wire framing
func collapseGP03BridgeEditing(value *GP03BridgeEditing) (*serializationKCES.GP03BridgeFile, error) {
	if value == nil {
		return nil, fmt.Errorf("nil GP03 bridge editing value")
	}
	if value.Format != serializationKCES.KCESGP03BridgeFormat {
		return nil, fmt.Errorf("unsupported GP03 bridge JSON format %q", value.Format)
	}
	if value.Signature != serializationKCES.GP03BridgeSignature {
		return nil, fmt.Errorf("invalid GP03 bridge signature %q", value.Signature)
	}
	if value.Version != serializationKCES.GP03BridgeVersion && value.Version != serializationKCES.GP03BridgeCOM3D2Version {
		return nil, fmt.Errorf("unsupported GP03 bridge version %d", value.Version)
	}
	if value.Version == serializationKCES.GP03BridgeCOM3D2Version && value.LegacyPreset != nil {
		return nil, fmt.Errorf("GP03 bridge version %d legacyPreset must be null", value.Version)
	}

	wire := &serializationKCES.GP03BridgeFile{
		Signature: value.Signature,
		Version:   value.Version,
		GUID:      value.GUID,
	}
	if value.LegacyPreset != nil {
		var legacy bytes.Buffer
		if err := value.LegacyPreset.Dump(&legacy); err != nil {
			return nil, fmt.Errorf("encode GP03 bridge legacyPreset as COM3D2 preset: %w", err)
		}
		wire.LegacyPreset = legacy.Bytes()
	}
	if value.CurrentPreset != nil {
		currentPreset, err := serializationKCES.EncodeExpandedKCESPreset(value.CurrentPreset)
		if err != nil {
			return nil, fmt.Errorf("encode GP03 bridge currentPreset as KCES preset: %w", err)
		}
		wire.CurrentPreset = currentPreset
	}
	return wire, nil
}

// decodeGP03BridgeEditingJSON 严格解码唯一 JSON 根值并验证所有 typed 预设可重建
// decodeGP03BridgeEditingJSON strictly decodes one JSON root and validates that every typed preset can be rebuilt
func decodeGP03BridgeEditingJSON(data []byte) (*GP03BridgeEditing, error) {
	var value GP03BridgeEditing
	if err := decodeStrictJSON(data, &value, "GP03 bridge JSON"); err != nil {
		return nil, err
	}
	data = trimJSONUTF8BOM(data)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, name := range []string{"format", "signature", "version", "guid", "legacyPreset", "currentPreset"} {
		if _, found := fields[name]; !found {
			return nil, fmt.Errorf("%s is required", name)
		}
	}
	if _, err := encodeGP03BridgeEditing(&value); err != nil {
		return nil, err
	}
	return &value, nil
}

// WriteBridgeFile 将完整的 GP03 bridge 编辑模型直接编码并写入 .brd 文件
// WriteBridgeFile directly encodes a complete GP03 bridge editing model and writes it to a .brd file
func (s *GP03BridgeService) WriteBridgeFile(path string, value *GP03BridgeEditing) error {
	encoded, err := encodeGP03BridgeEditing(value)
	if err != nil {
		return fmt.Errorf("encode GP03 bridge: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write .brd file %q: %w", path, err)
	}
	return nil
}

// writeBridgeConversionOutput 在上下文有效且大小不超限时写入 .brd 转换结果
// writeBridgeConversionOutput writes .brd conversion output while the context is active and the size remains within the limit
func writeBridgeConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .brd conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .brd conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .brd conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
