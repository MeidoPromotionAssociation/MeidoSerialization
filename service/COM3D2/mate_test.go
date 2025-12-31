package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestMateService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.mate")
	if err != nil {
		t.Fatal(err)
	}

	s := &MateService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
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

			// Re-read and verify consistency
			mateBack, err := s.ReadMateFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted mate failed: %v", err)
			}
			if !reflect.DeepEqual(mate, mateBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			mateRepack, err := s.ReadMateFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written mate failed: %v", err)
			}
			if !reflect.DeepEqual(mate, mateRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
