package COM3D2

import (
	"bytes"
	"os"
	"testing"
)

func TestPreset(t *testing.T) {
	filePath := "../../testdata/test.preset"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	preset, err := ReadPreset(f)
	if err != nil {
		t.Fatalf("failed to read preset: %v", err)
	}

	if preset.Signature != "CM3D2_PRESET" {
		t.Errorf("expected signature CM3D2_PRESET, got %s", preset.Signature)
	}

	// Test Dump
	var buf bytes.Buffer
	err = preset.Dump(&buf)
	if err != nil {
		t.Fatalf("failed to dump preset: %v", err)
	}

	// Re-read from dumped buffer
	preset2, err := ReadPreset(&buf)
	if err != nil {
		t.Fatalf("failed to re-read dumped preset: %v", err)
	}

	// Compare basic fields
	if preset.Version != preset2.Version {
		t.Errorf("version mismatch: %d != %d", preset.Version, preset2.Version)
	}
}
