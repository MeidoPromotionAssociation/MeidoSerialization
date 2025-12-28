package COM3D2

import (
	"bytes"
	"os"
	"testing"
)

func TestTex(t *testing.T) {
	filePath := "../../testdata/test.tex"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	tex, err := ReadTex(f)
	if err != nil {
		t.Fatalf("failed to read tex: %v", err)
	}

	if tex.Signature != "CM3D2_TEX" {
		t.Errorf("expected signature CM3D2_TEX, got %s", tex.Signature)
	}

	// Test Dump
	var buf bytes.Buffer
	err = tex.Dump(&buf)
	if err != nil {
		t.Fatalf("failed to dump tex: %v", err)
	}

	// Re-read from dumped buffer
	tex2, err := ReadTex(&buf)
	if err != nil {
		t.Fatalf("failed to re-read dumped tex: %v", err)
	}

	// Compare basic fields
	if tex.Version != tex2.Version {
		t.Errorf("version mismatch: %d != %d", tex.Version, tex2.Version)
	}
}
