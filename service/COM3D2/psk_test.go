package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPskService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.psk")
	if err != nil {
		t.Fatal(err)
	}

	s := &PskService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
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

			// Re-read and verify consistency
			pskBack, err := s.ReadPskFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted psk failed: %v", err)
			}
			if !reflect.DeepEqual(psk, pskBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			pskRepack, err := s.ReadPskFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written psk failed: %v", err)
			}
			if !reflect.DeepEqual(psk, pskRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
