package COM3D2

import (
	"bytes"
	"os"
	"testing"
)

func TestCol(t *testing.T) {
	filePath := "../../testdata/test.col"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	col, err := ReadCol(f)
	if err != nil {
		t.Fatalf("failed to read col: %v", err)
	}

	if col.Signature != "CM3D21_COL" {
		t.Errorf("expected signature CM3D21_COL, got %s", col.Signature)
	}

	// Test Dump
	var buf bytes.Buffer
	err = col.Dump(&buf)
	if err != nil {
		t.Fatalf("failed to dump col: %v", err)
	}

	// Re-read from dumped buffer
	col2, err := ReadCol(&buf)
	if err != nil {
		t.Fatalf("failed to re-read dumped col: %v", err)
	}

	// Compare basic fields
	if col.Version != col2.Version {
		t.Errorf("version mismatch: %d != %d", col.Version, col2.Version)
	}
	if len(col.Colliders) != len(col2.Colliders) {
		t.Errorf("colliders count mismatch: %d != %d", len(col.Colliders), len(col2.Colliders))
	}
}
