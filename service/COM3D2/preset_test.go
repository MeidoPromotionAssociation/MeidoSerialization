package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPresetService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.preset")
	if err != nil {
		t.Fatal(err)
	}

	s := &PresetService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
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

			// Re-read and verify consistency
			presetBack, err := s.ReadPresetFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted preset failed: %v", err)
			}
			if !reflect.DeepEqual(preset, presetBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			presetRepack, err := s.ReadPresetFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written preset failed: %v", err)
			}
			if !reflect.DeepEqual(preset, presetRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
