package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestModelService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.model")
	if err != nil {
		t.Fatal(err)
	}

	s := &ModelService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, "test.model")
			jsonPath := filepath.Join(tempDir, "test.model.json")
			backPath := filepath.Join(tempDir, "test_back.model")

			// 1. Test ReadModelFile
			model, err := s.ReadModelFile(inputPath)
			if err != nil {
				t.Fatalf("ReadModelFile failed: %v", err)
			}
			if model == nil {
				t.Fatal("ReadModelFile returned nil")
			}

			// 2. Test WriteModelFile
			err = s.WriteModelFile(outputPath, model)
			if err != nil {
				t.Fatalf("WriteModelFile failed: %v", err)
			}

			// 3. Test ConvertModelToJson
			err = s.ConvertModelToJson(inputPath, jsonPath)
			if err != nil {
				t.Fatalf("ConvertModelToJson failed: %v", err)
			}

			// 4. Test ConvertJsonToModel
			err = s.ConvertJsonToModel(jsonPath, backPath)
			if err != nil {
				t.Fatalf("ConvertJsonToModel failed: %v", err)
			}

			// Re-read and verify consistency
			modelBack, err := s.ReadModelFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted model failed: %v", err)
			}
			if !reflect.DeepEqual(model, modelBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			modelRepack, err := s.ReadModelFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written model failed: %v", err)
			}
			if !reflect.DeepEqual(model, modelRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
