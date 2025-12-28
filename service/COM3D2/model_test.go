package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModelService(t *testing.T) {
	s := &ModelService{}
	inputPath := "../../testdata/test.model"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.model not found, skipping test")
	}

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

	// Optional: Re-read and verify
	modelBack, err := s.ReadModelFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted model failed: %v", err)
	}
	if modelBack.Signature != model.Signature {
		t.Errorf("Signature mismatch: %s != %s", modelBack.Signature, model.Signature)
	}
}
