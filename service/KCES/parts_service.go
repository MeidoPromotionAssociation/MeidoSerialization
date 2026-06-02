package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// PartsService 处理部件 MOD 使用的 KCES MessagePack TextAsset 载荷 / PartsService handles KCES MessagePack TextAsset payloads used by parts MODs
type PartsService struct{}

// IsKCESModelFile reports whether path looks like a KCES .model TextAsset payload.
func IsKCESModelFile(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".model") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	_, err = KCES.DecodeModel(data)
	return err == nil
}

// IsKCESPartsFile reports whether path is a supported KCES parts payload.
func IsKCESPartsFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".menuassets", ".materialassets", ".pmatassets":
		return true
	case ".model":
		return IsKCESModelFile(path)
	default:
		return false
	}
}

// IsKCESPartsJSONFile reports whether path is a JSON representation of a
// supported KCES parts payload.
func IsKCESPartsJSONFile(path string) bool {
	ext := partsExtFromJSONPath(path)
	switch ext {
	case ".menuassets", ".materialassets", ".pmatassets":
		return true
	case ".model":
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return false
		}
		_, hasMeshFileName := obj["meshfileName"]
		_, hasTransData := obj["transData"]
		return hasMeshFileName || hasTransData
	default:
		return false
	}
}

func (s *PartsService) ConvertPartsToJson(inputPath string, outputPath string) error {
	value, err := s.ReadPartsFile(inputPath)
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal KCES parts json: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *PartsService) ConvertJsonToParts(inputPath string, outputPath string) error {
	ext := partsExtFromJSONPath(inputPath)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(outputPath))
	}
	if ext == "" || ext == ".json" {
		return fmt.Errorf("cannot determine KCES parts type from %q", inputPath)
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}

	encoded, err := encodePartsJSON(ext, data)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *PartsService) ReadPartsFile(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".menuassets":
		return KCES.DecodeMenuAssets(data)
	case ".materialassets":
		return KCES.DecodeMaterialAssets(data)
	case ".pmatassets":
		return KCES.DecodePriorityMaterialAssets(data)
	case ".model":
		return KCES.DecodeModel(data)
	default:
		return nil, fmt.Errorf("unsupported KCES parts file type: %s", filepath.Ext(path))
	}
}

func encodePartsJSON(ext string, data []byte) ([]byte, error) {
	switch strings.ToLower(ext) {
	case ".menuassets":
		var assets KCES.MenuAssets
		if err := json.Unmarshal(data, &assets); err != nil {
			return nil, fmt.Errorf("parse menuassets json: %w", err)
		}
		return KCES.EncodeMenuAssets(&assets)
	case ".materialassets":
		var assets KCES.MaterialAssets
		if err := json.Unmarshal(data, &assets); err != nil {
			return nil, fmt.Errorf("parse materialassets json: %w", err)
		}
		return KCES.EncodeMaterialAssets(&assets)
	case ".pmatassets":
		var assets KCES.PriorityMaterialAssets
		if err := json.Unmarshal(data, &assets); err != nil {
			return nil, fmt.Errorf("parse pmatassets json: %w", err)
		}
		return KCES.EncodePriorityMaterialAssets(&assets)
	case ".model":
		var model KCES.Model
		if err := json.Unmarshal(data, &model); err != nil {
			return nil, fmt.Errorf("parse model json: %w", err)
		}
		return KCES.EncodeModel(&model)
	default:
		return nil, fmt.Errorf("unsupported KCES parts JSON type: %s", ext)
	}
}

func partsExtFromJSONPath(path string) string {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return ""
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.ToLower(filepath.Ext(base))
}
