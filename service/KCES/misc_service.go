package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func (s *MiscService) ConvertMiscToJson(inputPath string, outputPath string) error {
	value, err := s.ReadMiscFile(inputPath)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES misc json: %w", err)
	}
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *MiscService) ConvertJsonToMisc(inputPath string, outputPath string) error {
	data, err := os.ReadFile(inputPath)
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

	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
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
	switch strings.ToLower(ext) {
	case ".hitcheck":
		var hitCheck serializationKCES.HitCheck
		if err := json.Unmarshal(data, &hitCheck); err != nil {
			return nil, fmt.Errorf("parse hitcheck json: %w", err)
		}
		return serializationKCES.EncodeHitCheck(&hitCheck)
	case ".undressdat", ".undresspdat":
		var value serializationKCES.KCESJSONText
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("parse %s json: %w", ext, err)
		}
		if value.Extension == "" {
			value.Extension = ext
		}
		return serializationKCES.EncodeKCESJSONText(&value)
	default:
		return nil, fmt.Errorf("unsupported KCES misc JSON type: %s", ext)
	}
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
