package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNeiService(t *testing.T) {
	s := &NeiService{}
	inputPath := "../../testdata/test.nei"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.nei not found, skipping test")
	}

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

	// Optional: Re-read and verify
	neiBack, err := s.ReadNeiFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted nei failed: %v", err)
	}
	if neiBack.Rows != nei.Rows {
		t.Errorf("Rows mismatch: %d != %d", neiBack.Rows, nei.Rows)
	}
}
