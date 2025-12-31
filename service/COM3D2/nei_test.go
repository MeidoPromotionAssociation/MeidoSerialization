package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestNeiService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.nei")
	if err != nil {
		t.Fatal(err)
	}

	s := &NeiService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, "test.nei")
			csvPath := filepath.Join(tempDir, "test.csv")
			backPath := filepath.Join(tempDir, "test_back.nei")

			// 1. Test ReadNeiFile
			nei, err := s.ReadNeiFile(inputPath)
			if err != nil {
				t.Fatalf("ReadNeiFile failed: %v", err)
			}
			if nei == nil {
				t.Fatal("ReadNeiFile returned nil")
			}

			// 2. Test WriteNeiFile
			err = s.WriteNeiFile(nei, outputPath)
			if err != nil {
				t.Fatalf("WriteNeiFile failed: %v", err)
			}

			// 3. Test NeiFileToCSVFile
			err = s.NeiFileToCSVFile(inputPath, csvPath)
			if err != nil {
				t.Fatalf("NeiFileToCSVFile failed: %v", err)
			}

			// 4. Test CSVFileToNeiFile
			err = s.CSVFileToNeiFile(csvPath, backPath)
			if err != nil {
				t.Fatalf("CSVFileToNeiFile failed: %v", err)
			}

			// Re-read and verify consistency
			neiBack, err := s.ReadNeiFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted nei failed: %v", err)
			}
			if !reflect.DeepEqual(nei, neiBack) {
				t.Errorf("data mismatch after CSV conversion cycle")
			}

			// Also verify direct write consistency
			neiRepack, err := s.ReadNeiFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written nei failed: %v", err)
			}
			if !reflect.DeepEqual(nei, neiRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
