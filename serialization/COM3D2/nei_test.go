package COM3D2

import (
	"bytes"
	"os"
	"testing"
)

func TestNei(t *testing.T) {
	filePath := "../../testdata/test.nei"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	nei, err := ReadNei(f, nil)
	if err != nil {
		t.Fatalf("failed to read nei: %v", err)
	}

	if len(nei.Data) == 0 {
		t.Errorf("expected non-empty data")
	}

	// Test Dump
	var buf bytes.Buffer
	err = nei.Dump(&buf)
	if err != nil {
		t.Fatalf("failed to dump nei: %v", err)
	}

	// Re-read from dumped buffer
	nei2, err := ReadNei(&buf, nil)
	if err != nil {
		t.Fatalf("failed to re-read dumped nei: %v", err)
	}

	// Compare basic fields
	if nei.Rows != nei2.Rows {
		t.Errorf("rows mismatch: %d != %d", nei.Rows, nei2.Rows)
	}
	if len(nei.Data) != len(nei2.Data) {
		t.Errorf("data rows mismatch: %d != %d", len(nei.Data), len(nei2.Data))
	}
}
