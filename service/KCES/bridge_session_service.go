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

// BridgeSessionService converts CRCEdit's bridge_session.vd file to and from
// a strict JSON editing envelope. Routing is intentionally left to the caller;
// this file does not alter the shared FileTypeService or CLI tables.
type BridgeSessionService struct{}

// IsKCESBridgeSessionFile recognizes the exact game file name, case
// insensitively. Other .vd files use unrelated VirtualDirectory payloads.
func IsKCESBridgeSessionFile(path string) bool {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return false
	}
	return strings.EqualFold(kcesBridgeSessionPathBase(path), "bridge_session.vd")
}

// IsKCESBridgeSessionJSONFile recognizes the corresponding editing suffix.
func IsKCESBridgeSessionJSONFile(path string) bool {
	ext := filepath.Ext(path)
	if !strings.EqualFold(ext, ".json") {
		return false
	}
	base := strings.TrimSuffix(path, ext)
	return strings.EqualFold(kcesBridgeSessionPathBase(base), "bridge_session.vd")
}

func kcesBridgeSessionPathBase(path string) string {
	// filepath follows the host OS. Accept both separators because JSON/native
	// paths may be generated on another platform before this predicate runs.
	return filepath.Base(strings.ReplaceAll(path, "\\", "/"))
}

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

func (s *BridgeSessionService) ConvertBridgeSessionToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	data, err := readConversionFile(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("read KCES bridge session %q: %w", inputPath, err)
	}
	value, err := serializationKCES.DecodeKCESBridgeSession(data)
	if err != nil {
		return fmt.Errorf("decode KCES bridge session %q: %w", inputPath, err)
	}
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	data, err = json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES bridge session JSON: %w", err)
	}
	data = append(data, '\n')
	if err := writeConversionFile(ctx, outputPath, data, maxOutputBytes); err != nil {
		return fmt.Errorf("write KCES bridge session JSON %q: %w", outputPath, err)
	}
	return nil
}

func (s *BridgeSessionService) ConvertJSONToBridgeSession(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	data, err := readConversionFile(ctx, inputPath)
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
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionFile(ctx, outputPath, encoded, maxOutputBytes); err != nil {
		return fmt.Errorf("write KCES bridge session %q: %w", outputPath, err)
	}
	return nil
}

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
