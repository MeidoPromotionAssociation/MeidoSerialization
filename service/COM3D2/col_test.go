package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestColService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.col")
	if err != nil {
		t.Fatal(err)
	}

	s := &ColService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
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

			// Re-read and verify consistency
			colBack, err := s.ReadColFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted col failed: %v", err)
			}
			if !reflect.DeepEqual(col, colBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			colRepack, err := s.ReadColFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written col failed: %v", err)
			}
			if !reflect.DeepEqual(col, colRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
