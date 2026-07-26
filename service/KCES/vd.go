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

// BridgeSessionService 在 CRCEdit 的 bridge_session.vd 与严格编辑 JSON 封套之间转换 / BridgeSessionService converts between CRCEdit bridge_session.vd files and strict editing JSON envelopes
type BridgeSessionService struct{}

// IsKCESBridgeSessionFile 不区分大小写识别准确的游戏文件名，避免把其他 .vd VirtualDirectory 载荷误判为 bridge session
// IsKCESBridgeSessionFile recognizes the exact game file name case-insensitively so unrelated .vd VirtualDirectory payloads are not treated as bridge sessions
func IsKCESBridgeSessionFile(path string) bool {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return false
	}
	return strings.EqualFold(kcesBridgeSessionPathBase(path), "bridge_session.vd")
}

// IsKCESBridgeSessionJSONFile 识别 bridge_session.vd 对应的编辑 JSON 双扩展名
// IsKCESBridgeSessionJSONFile recognizes the editing JSON double extension corresponding to bridge_session.vd
func IsKCESBridgeSessionJSONFile(path string) bool {
	ext := filepath.Ext(path)
	if !strings.EqualFold(ext, ".json") {
		return false
	}
	base := strings.TrimSuffix(path, ext)
	return strings.EqualFold(kcesBridgeSessionPathBase(base), "bridge_session.vd")
}

// kcesBridgeSessionPathBase 使用两种路径分隔符提取 bridge session 基础文件名
// kcesBridgeSessionPathBase extracts the bridge session base name while accepting both path separators
func kcesBridgeSessionPathBase(path string) string {
	// filepath 遵循宿主系统规则，因此先统一跨平台来源可能包含的分隔符 / filepath follows the host system rules so separators from cross-platform sources are normalized first
	return filepath.Base(strings.ReplaceAll(path, "\\", "/"))
}

// ReadBridgeSessionFile 读取并解码 bridge_session.vd 文件
// ReadBridgeSessionFile reads and decodes a bridge_session.vd file
func (s *BridgeSessionService) ReadBridgeSessionFile(path string) (*serializationKCES.KCESBridgeSession, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read KCES bridge session %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESBridgeSession(data)
	if err != nil {
		return nil, fmt.Errorf("decode KCES bridge session %q: %w", path, err)
	}
	return value, nil
}

// ConvertBridgeSessionToJSON 将 bridge_session.vd 转换为严格编辑 JSON
// ConvertBridgeSessionToJSON converts bridge_session.vd to strict editing JSON
func (s *BridgeSessionService) ConvertBridgeSessionToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	value, err := s.ReadBridgeSessionFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES bridge session JSON: %w", err)
	}
	data = append(data, '\n')
	return writeBridgeSessionConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJSONToBridgeSession 将严格编辑 JSON 转换为 bridge_session.vd
// ConvertJSONToBridgeSession converts strict editing JSON to bridge_session.vd
func (s *BridgeSessionService) ConvertJSONToBridgeSession(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES bridge session JSON %q: %w", inputPath, err)
	}
	value, err := decodeKCESBridgeSessionEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse KCES bridge session JSON %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeKCESBridgeSession(value)
	if err != nil {
		return fmt.Errorf("encode KCES bridge session JSON %q: %w", inputPath, err)
	}
	return writeBridgeSessionConversionOutput(ctx, outputPath, encoded, maxOutputBytes)
}

// decodeKCESBridgeSessionEditingJSON 严格解码 bridge session 编辑 JSON 并校验必需的嵌套字段
// decodeKCESBridgeSessionEditingJSON strictly decodes bridge session editing JSON and validates required nested fields
func decodeKCESBridgeSessionEditingJSON(data []byte) (*serializationKCES.KCESBridgeSession, error) {
	var value serializationKCES.KCESBridgeSession
	if err := decodeStrictJSON(data, &value, "KCES bridge session JSON"); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimJSONUTF8BOM(data), &fields); err != nil {
		return nil, err
	}
	for _, name := range []string{"format", "containerVersion", "sessionData"} {
		if _, found := fields[name]; !found {
			return nil, fmt.Errorf("%s is required", name)
		}
	}
	if bytes.Equal(bytes.TrimSpace(fields["sessionData"]), []byte("null")) {
		return nil, fmt.Errorf("sessionData must not be null")
	}
	var sessionFields map[string]json.RawMessage
	if err := json.Unmarshal(fields["sessionData"], &sessionFields); err != nil {
		return nil, err
	}
	for _, name := range []string{"version", "sessionId", "hideMenuFileIds"} {
		if _, found := sessionFields[name]; !found {
			return nil, fmt.Errorf("sessionData.%s is required", name)
		}
	}
	if value.Format == "" {
		return nil, fmt.Errorf("format is missing or null")
	}
	if value.Format != serializationKCES.KCESBridgeSessionFormat {
		return nil, fmt.Errorf("unsupported KCES bridge session JSON format %q", value.Format)
	}
	if _, err := serializationKCES.EncodeKCESBridgeSession(&value); err != nil {
		return nil, err
	}
	return &value, nil
}

// WriteBridgeSessionFile 将 KCES bridge session 结构直接编码并写入 bridge_session.vd 文件
// WriteBridgeSessionFile directly encodes a KCES bridge session value and writes it to a bridge_session.vd file
func (s *BridgeSessionService) WriteBridgeSessionFile(path string, value *serializationKCES.KCESBridgeSession) error {
	encoded, err := serializationKCES.EncodeKCESBridgeSession(value)
	if err != nil {
		return fmt.Errorf("encode KCES bridge session: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write bridge_session.vd file %q: %w", path, err)
	}
	return nil
}

// writeBridgeSessionConversionOutput 在上下文有效且大小不超限时写入 bridge_session.vd 转换结果
// writeBridgeSessionConversionOutput writes bridge_session.vd conversion output while the context is active and the size remains within the limit
func writeBridgeSessionConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive bridge_session.vd conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: bridge_session.vd conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write bridge_session.vd conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
