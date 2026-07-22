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

// MiscService 提供 KCES 杂项文件的 JSON 转换服务 / MiscService provides JSON conversion services for KCES miscellaneous files
type MiscService struct{}

func IsKCESMiscFile(path string) bool {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	return isHitCheckFile(path) || serializationKCES.IsKCESJSONTextExtension(path)
}

func IsKCESMiscJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return isHitCheckFile(base) || serializationKCES.IsKCESJSONTextExtension(base)
}

func (s *MiscService) ConvertMiscToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	value, err := s.ReadMiscFile(inputPath)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES misc json: %w", err)
	}
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionFile(ctx, outputPath, jsonData, maxOutputBytes); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *MiscService) ConvertJsonToMisc(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	data, err := readConversionFile(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}

	ext := miscExtFromJSONPath(inputPath)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(outputPath))
	}
	if ext == "" || ext == ".json" {
		return fmt.Errorf("cannot determine KCES misc type from %q", inputPath)
	}

	encoded, err := encodeMiscJSON(ext, data)
	if err != nil {
		return err
	}

	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionFile(ctx, outputPath, encoded, maxOutputBytes); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *MiscService) ReadMiscFile(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case ext == ".hitcheck":
		return serializationKCES.DecodeHitCheck(data)
	case serializationKCES.IsKCESJSONTextExtension(ext):
		return serializationKCES.DecodeKCESJSONText(data, ext)
	default:
		return nil, fmt.Errorf("unsupported KCES misc file type: %s", ext)
	}
}

func encodeMiscJSON(ext string, data []byte) ([]byte, error) {
	data = trimJSONUTF8BOM(data)
	switch strings.ToLower(ext) {
	case ".hitcheck":
		var hitCheck serializationKCES.HitCheck
		if err := decodeStrictJSON(data, &hitCheck, "KCES hitcheck JSON"); err != nil {
			return nil, fmt.Errorf("parse hitcheck json: %w", err)
		}
		if hitCheck.Signature != serializationKCES.HitCheckSignature {
			return nil, fmt.Errorf("parse hitcheck json: invalid signature %q", hitCheck.Signature)
		}
		return serializationKCES.EncodeHitCheck(&hitCheck)
	case ".undressdat", ".undresspdat", ".nson":
		value, err := decodeKCESJSONTextEditingJSON(data, ext)
		if err != nil {
			return nil, fmt.Errorf("parse %s json: %w", ext, err)
		}
		return serializationKCES.EncodeKCESJSONText(value)
	default:
		return nil, fmt.Errorf("unsupported KCES misc JSON type: %s", ext)
	}
}

// decodeKCESJSONTextEditingJSON is the single strict parser used by both the
// conversion service and FileTypeService. The editing envelope intentionally
// has no format marker, so its double extension is the authoritative type.
// Keeping this validation in one place prevents malformed KCES-only files
// from falling through to the legacy JSON/CSV heuristics.
func decodeKCESJSONTextEditingJSON(data []byte, expectedExtension string) (*serializationKCES.KCESJSONText, error) {
	data = trimJSONUTF8BOM(data)
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("KCES JSON-text envelope is not valid UTF-8")
	}

	var editing struct {
		Extension *string         `json:"extension"`
		Text      *string         `json:"text"`
		JSON      json.RawMessage `json:"json"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&editing); err != nil {
		return nil, err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON value")
		}
		return nil, fmt.Errorf("trailing content: %w", err)
	}

	expected := serializationKCES.NormalizeKCESJSONTextExtension(expectedExtension)
	if expected == "" {
		return nil, fmt.Errorf("unsupported KCES JSON-text extension %q", expectedExtension)
	}
	extension := expected
	if editing.Extension != nil && strings.TrimSpace(*editing.Extension) != "" {
		extension = serializationKCES.NormalizeKCESJSONTextExtension(*editing.Extension)
		if extension == "" {
			return nil, fmt.Errorf("unsupported KCES JSON-text envelope extension %q", *editing.Extension)
		}
		if extension != expected {
			return nil, fmt.Errorf("KCES JSON-text envelope extension %q does not match file extension %q", extension, expected)
		}
	}
	if len(editing.JSON) == 0 {
		return nil, fmt.Errorf("json payload is missing")
	}

	value := &serializationKCES.KCESJSONText{
		Extension: extension,
		JSON:      append(json.RawMessage(nil), editing.JSON...),
	}
	if editing.Text != nil {
		value.Text = *editing.Text
	}
	if _, err := serializationKCES.EncodeKCESJSONText(value); err != nil {
		return nil, err
	}
	return value, nil
}

func miscExtFromJSONPath(path string) string {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return ""
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.ToLower(filepath.Ext(base))
}

func isHitCheckFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".hitcheck")
}
