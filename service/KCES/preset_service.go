package KCES

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// PresetService handles the VirtualDirectory-based preset format introduced
// by KCES. Legacy CM3D2_PRESET files continue to use service/COM3D2.
type PresetService struct{}

// IsKCESPresetFile distinguishes current KCES presets by their wire signature
// instead of relying on the shared .preset extension.
func IsKCESPresetFile(path string) bool {
	lower := strings.ToLower(path)
	if (!strings.HasSuffix(lower, serializationKCES.KCESPresetExtension) && !strings.HasSuffix(lower, serializationKCES.KCESPersetExtension)) || strings.HasSuffix(lower, ".json") {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	header := make([]byte, 7)
	if _, err := io.ReadFull(f, header); err != nil {
		return false
	}
	return serializationKCES.IsKCESPresetData(header)
}

// IsKCESPresetJSONFile distinguishes KCES preset JSON from legacy preset JSON
// through the explicit format marker.
func IsKCESPresetJSONFile(path string) bool {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, serializationKCES.KCESPresetExtension+".json") && !strings.HasSuffix(lower, serializationKCES.KCESPersetExtension+".json") {
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
	return header.Format == serializationKCES.KCESPresetFormat
}

func (s *PresetService) ReadPresetFile(path string) (*serializationKCES.KCESPreset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read KCES preset %q: %w", path, err)
	}
	preset, err := serializationKCES.DecodeKCESPreset(data)
	if err != nil {
		return nil, fmt.Errorf("parse KCES preset %q: %w", path, err)
	}
	return preset, nil
}

func (s *PresetService) ConvertPresetToJson(inputPath string, outputPath string) error {
	preset, err := s.ReadPresetFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES preset JSON: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *PresetService) ConvertJsonToPreset(inputPath string, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	var preset serializationKCES.KCESPreset
	if err := decodeStrictJSON(data, &preset, "KCES preset JSON"); err != nil {
		return fmt.Errorf("parse KCES preset JSON: %w", err)
	}
	if preset.Format != serializationKCES.KCESPresetFormat {
		return fmt.Errorf("unsupported KCES preset JSON format %q", preset.Format)
	}
	encoded, err := serializationKCES.EncodeKCESPreset(&preset)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}
