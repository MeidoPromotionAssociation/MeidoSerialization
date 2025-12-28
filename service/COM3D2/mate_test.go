package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMateService(t *testing.T) {
	s := &MateService{}
	inputPath := "../../testdata/test.mate"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.mate not found, skipping test")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.mate")
	jsonPath := filepath.Join(tempDir, "test.mate.json")
	backPath := filepath.Join(tempDir, "test_back.mate")

	// 1. Test ReadMateFile
	mate, err := s.ReadMateFile(inputPath)
	if err != nil {
		t.Fatalf("ReadMateFile failed: %v", err)
	}
	if mate == nil {
		t.Fatal("ReadMateFile returned nil")
	}

	// 2. Test WriteMateFile
	err = s.WriteMateFile(outputPath, mate)
	if err != nil {
		t.Fatalf("WriteMateFile failed: %v", err)
	}

	// 3. Test ConvertMateToJson
	err = s.ConvertMateToJson(inputPath, jsonPath)
	if err != nil {
		t.Fatalf("ConvertMateToJson failed: %v", err)
	}

	// 4. Test ConvertJsonToMate
	err = s.ConvertJsonToMate(jsonPath, backPath)
	if err != nil {
		t.Fatalf("ConvertJsonToMate failed: %v", err)
	}

	// Optional: Re-read and verify
	mateBack, err := s.ReadMateFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted mate failed: %v", err)
	}
	if mateBack.Signature != mate.Signature {
		t.Errorf("Signature mismatch: %s != %s", mateBack.Signature, mate.Signature)
	}
}
