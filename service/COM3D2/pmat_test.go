package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPMatService(t *testing.T) {
	s := &PMatService{}
	inputPath := "../../testdata/test.pmat"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.pmat not found, skipping test")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.pmat")
	jsonPath := filepath.Join(tempDir, "test.pmat.json")
	backPath := filepath.Join(tempDir, "test_back.pmat")

	// 1. Test ReadPMatFile
	pmat, err := s.ReadPMatFile(inputPath)
	if err != nil {
		t.Fatalf("ReadPMatFile failed: %v", err)
	}
	if pmat == nil {
		t.Fatal("ReadPMatFile returned nil")
	}

	// 2. Test WritePMatFile
	err = s.WritePMatFile(outputPath, pmat)
	if err != nil {
		t.Fatalf("WritePMatFile failed: %v", err)
	}

	// 3. Test ConvertPMatToJson
	err = s.ConvertPMatToJson(inputPath, jsonPath)
	if err != nil {
		t.Fatalf("ConvertPMatToJson failed: %v", err)
	}

	// 4. Test ConvertJsonToPMat
	err = s.ConvertJsonToPMat(jsonPath, backPath)
	if err != nil {
		t.Fatalf("ConvertJsonToPMat failed: %v", err)
	}

	// Optional: Re-read and verify
	pmatBack, err := s.ReadPMatFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted pmat failed: %v", err)
	}
	if pmatBack.Signature != pmat.Signature {
		t.Errorf("Signature mismatch: %s != %s", pmatBack.Signature, pmat.Signature)
	}
}
