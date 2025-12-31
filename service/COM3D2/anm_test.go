package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestAnmService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.anm")
	if err != nil {
		t.Fatal(err)
	}

	s := &AnmService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
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

			// Re-read and verify consistency
			anmBack, err := s.ReadAnmFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted anm failed: %v", err)
			}
			if !reflect.DeepEqual(anm, anmBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			anmRepack, err := s.ReadAnmFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written anm failed: %v", err)
			}
			if !reflect.DeepEqual(anm, anmRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
