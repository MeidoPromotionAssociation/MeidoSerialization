package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnmService(t *testing.T) {
	s := &AnmService{}
	inputPath := "../../testdata/test.anm"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.anm not found, skipping test")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.anm")
	jsonPath := filepath.Join(tempDir, "test.anm.json")
	backPath := filepath.Join(tempDir, "test_back.anm")

	// 1. Test ReadAnmFile
	anm, err := s.ReadAnmFile(inputPath)
	if err != nil {
		t.Fatalf("ReadAnmFile failed: %v", err)
	}
	if anm == nil {
		t.Fatal("ReadAnmFile returned nil")
	}

	// 2. Test WriteAnmFile
	err = s.WriteAnmFile(outputPath, anm)
	if err != nil {
		t.Fatalf("WriteAnmFile failed: %v", err)
	}

	// 3. Test ConvertAnmToJson
	err = s.ConvertAnmToJson(inputPath, jsonPath)
	if err != nil {
		t.Fatalf("ConvertAnmToJson failed: %v", err)
	}

	// 4. Test ConvertJsonToAnm
	err = s.ConvertJsonToAnm(jsonPath, backPath)
	if err != nil {
		t.Fatalf("ConvertJsonToAnm failed: %v", err)
	}

	// Optional: Re-read and verify backPath
	anmBack, err := s.ReadAnmFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted anm failed: %v", err)
	}
	if anmBack.Signature != anm.Signature {
		t.Errorf("Signature mismatch: %s != %s", anmBack.Signature, anm.Signature)
	}
}
