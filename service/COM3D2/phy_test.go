package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhyService(t *testing.T) {
	s := &PhyService{}
	inputPath := "../../testdata/test.phy"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.phy not found, skipping test")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.phy")
	jsonPath := filepath.Join(tempDir, "test.phy.json")
	backPath := filepath.Join(tempDir, "test_back.phy")

	// 1. Test ReadPhyFile
	phy, err := s.ReadPhyFile(inputPath)
	if err != nil {
		t.Fatalf("ReadPhyFile failed: %v", err)
	}
	if phy == nil {
		t.Fatal("ReadPhyFile returned nil")
	}

	// 2. Test WritePhyFile
	err = s.WritePhyFile(outputPath, phy)
	if err != nil {
		t.Fatalf("WritePhyFile failed: %v", err)
	}

	// 3. Test ConvertPhyToJson
	err = s.ConvertPhyToJson(inputPath, jsonPath)
	if err != nil {
		t.Fatalf("ConvertPhyToJson failed: %v", err)
	}

	// 4. Test ConvertJsonToPhy
	err = s.ConvertJsonToPhy(jsonPath, backPath)
	if err != nil {
		t.Fatalf("ConvertJsonToPhy failed: %v", err)
	}

	// Optional: Re-read and verify
	phyBack, err := s.ReadPhyFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted phy failed: %v", err)
	}
	if phyBack.Signature != phy.Signature {
		t.Errorf("Signature mismatch: %s != %s", phyBack.Signature, phy.Signature)
	}
}
