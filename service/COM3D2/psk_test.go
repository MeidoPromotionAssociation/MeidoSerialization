package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPskService(t *testing.T) {
	s := &PskService{}
	inputPath := "../../testdata/test.psk"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.psk not found, skipping test")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.psk")
	jsonPath := filepath.Join(tempDir, "test.psk.json")
	backPath := filepath.Join(tempDir, "test_back.psk")

	// 1. Test ReadPskFile
	psk, err := s.ReadPskFile(inputPath)
	if err != nil {
		t.Fatalf("ReadPskFile failed: %v", err)
	}
	if psk == nil {
		t.Fatal("ReadPskFile returned nil")
	}

	// 2. Test WritePskFile
	err = s.WritePskFile(outputPath, psk)
	if err != nil {
		t.Fatalf("WritePskFile failed: %v", err)
	}

	// 3. Test ConvertPskToJson
	err = s.ConvertPskToJson(inputPath, jsonPath)
	if err != nil {
		t.Fatalf("ConvertPskToJson failed: %v", err)
	}

	// 4. Test ConvertJsonToPsk
	err = s.ConvertJsonToPsk(jsonPath, backPath)
	if err != nil {
		t.Fatalf("ConvertJsonToPsk failed: %v", err)
	}

	// Optional: Re-read and verify
	pskBack, err := s.ReadPskFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted psk failed: %v", err)
	}
	if pskBack.Signature != psk.Signature {
		t.Errorf("Signature mismatch: %s != %s", pskBack.Signature, psk.Signature)
	}
}
