package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPhyService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.phy")
	if err != nil {
		t.Fatal(err)
	}

	s := &PhyService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
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

			// Re-read and verify consistency
			phyBack, err := s.ReadPhyFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted phy failed: %v", err)
			}
			if !reflect.DeepEqual(phy, phyBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			phyRepack, err := s.ReadPhyFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written phy failed: %v", err)
			}
			if !reflect.DeepEqual(phy, phyRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
