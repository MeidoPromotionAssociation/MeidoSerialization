package COM3D2

import (
	"bytes"
	"os"
	"testing"
)

func TestPmat(t *testing.T) {
	filePath := "../../testdata/test.pmat"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	pmat, err := ReadPMat(f)
	if err != nil {
		t.Fatalf("failed to read pmat: %v", err)
	}

	if pmat.Signature != "CM3D2_PMATERIAL" {
		t.Errorf("expected signature CM3D2_PMATERIAL, got %s", pmat.Signature)
	}

	// Test Dump
	var buf bytes.Buffer
	err = pmat.Dump(&buf, false)
	if err != nil {
		t.Fatalf("failed to dump pmat: %v", err)
	}

	// Re-read from dumped buffer
	pmat2, err := ReadPMat(&buf)
	if err != nil {
		t.Fatalf("failed to re-read dumped pmat: %v", err)
	}

	// Compare basic fields
	if pmat.Version != pmat2.Version {
		t.Errorf("version mismatch: %d != %d", pmat.Version, pmat2.Version)
	}
	if pmat.MaterialName != pmat2.MaterialName {
		t.Errorf("material name mismatch: %s != %s", pmat.MaterialName, pmat2.MaterialName)
	}
}
