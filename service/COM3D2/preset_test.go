package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPresetService(t *testing.T) {
	s := &PresetService{}
	inputPath := "../../testdata/test.preset"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.preset not found, skipping test")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.preset")
	jsonPath := filepath.Join(tempDir, "test.preset.json")
	backPath := filepath.Join(tempDir, "test_back.preset")

	// 1. Test ReadPresetFile
	preset, err := s.ReadPresetFile(inputPath)
	if err != nil {
		t.Fatalf("ReadPresetFile failed: %v", err)
	}
	if preset == nil {
		t.Fatal("ReadPresetFile returned nil")
	}

	// 2. Test WritePresetFile
	err = s.WritePresetFile(outputPath, preset)
	if err != nil {
		t.Fatalf("WritePresetFile failed: %v", err)
	}

	// 3. Test ConvertPresetToJson
	err = s.ConvertPresetToJson(inputPath, jsonPath)
	if err != nil {
		t.Fatalf("ConvertPresetToJson failed: %v", err)
	}

	// 4. Test ConvertJsonToPreset
	err = s.ConvertJsonToPreset(jsonPath, backPath)
	if err != nil {
		t.Fatalf("ConvertJsonToPreset failed: %v", err)
	}

	// Optional: Re-read and verify
	presetBack, err := s.ReadPresetFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted preset failed: %v", err)
	}
	if presetBack.Signature != preset.Signature {
		t.Errorf("Signature mismatch: %s != %s", presetBack.Signature, preset.Signature)
	}
}
