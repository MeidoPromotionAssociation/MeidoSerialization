package KCES

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// GP03BridgeService converts v2000/v2001 GP03_BRIDGE (.brd) files to and from
// a strict, marker-based JSON representation. Embedded presets remain base64
// byte arrays so the opaque legacy payload is always lossless.
type GP03BridgeService struct{}

func IsKCESGP03BridgeFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), ".brd")
}

func IsKCESGP03BridgeJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.EqualFold(filepath.Ext(base), ".brd")
}

func (s *GP03BridgeService) ReadBridgeFile(path string) (*serializationKCES.GP03BridgeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read GP03 bridge file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeGP03Bridge(data)
	if err != nil {
		return nil, fmt.Errorf("decode GP03 bridge file %q: %w", path, err)
	}
	return value, nil
}

func (s *GP03BridgeService) ConvertBridgeToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	input, err := readConversionFile(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("read GP03 bridge file %q: %w", inputPath, err)
	}
	value, err := serializationKCES.DecodeGP03Bridge(input)
	if err != nil {
		return fmt.Errorf("decode GP03 bridge file %q: %w", inputPath, err)
	}
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal GP03 bridge JSON: %w", err)
	}
	data = append(data, '\n')
	if err := writeConversionFile(ctx, outputPath, data, maxOutputBytes); err != nil {
		return fmt.Errorf("write GP03 bridge JSON %q: %w", outputPath, err)
	}
	return nil
}

func (s *GP03BridgeService) ConvertJSONToBridge(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	data, err := readConversionFile(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("read GP03 bridge JSON %q: %w", inputPath, err)
	}
	value, err := decodeGP03BridgeEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse GP03 bridge JSON %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeGP03Bridge(value)
	if err != nil {
		return fmt.Errorf("encode GP03 bridge JSON %q: %w", inputPath, err)
	}
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionFile(ctx, outputPath, encoded, maxOutputBytes); err != nil {
		return fmt.Errorf("write GP03 bridge file %q: %w", outputPath, err)
	}
	return nil
}

func decodeGP03BridgeEditingJSON(data []byte) (*serializationKCES.GP03BridgeFile, error) {
	data = trimJSONUTF8BOM(data)
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("GP03 bridge JSON is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value serializationKCES.GP03BridgeFile
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON value")
		}
		return nil, fmt.Errorf("trailing content: %w", err)
	}
	if value.Format == "" {
		return nil, fmt.Errorf("format is missing or null")
	}
	if value.Signature == "" {
		return nil, fmt.Errorf("signature is missing or null")
	}
	if _, err := serializationKCES.EncodeGP03Bridge(&value); err != nil {
		return nil, err
	}
	return &value, nil
}
