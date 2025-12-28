package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestColService(t *testing.T) {
	s := &ColService{}
	inputPath := "../../testdata/test.col"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.col not found, skipping test")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.col")
	jsonPath := filepath.Join(tempDir, "test.col.json")
	backPath := filepath.Join(tempDir, "test_back.col")

	// 1. Test ReadColFile
	col, err := s.ReadColFile(inputPath)
	if err != nil {
		t.Fatalf("ReadColFile failed: %v", err)
	}
	if col == nil {
		t.Fatal("ReadColFile returned nil")
	}

	// 2. Test WriteColFile
	err = s.WriteColFile(outputPath, col)
	if err != nil {
		t.Fatalf("WriteColFile failed: %v", err)
	}

	// 3. Test ConvertColToJson
	err = s.ConvertColToJson(inputPath, jsonPath)
	if err != nil {
		t.Fatalf("ConvertColToJson failed: %v", err)
	}

	// 4. Test ConvertJsonToCol
	err = s.ConvertJsonToCol(jsonPath, backPath)
	if err != nil {
		t.Fatalf("ConvertJsonToCol failed: %v", err)
	}

	// Optional: Re-read and verify
	colBack, err := s.ReadColFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted col failed: %v", err)
	}
	if colBack.Signature != col.Signature {
		t.Errorf("Signature mismatch: %s != %s", colBack.Signature, col.Signature)
	}
}
