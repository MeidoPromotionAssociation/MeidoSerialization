package COM3D2

import (
	"bytes"
	"os"
	"testing"
)

func TestPsk(t *testing.T) {
	filePath := "../../testdata/test.psk"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	psk, err := ReadPsk(f)
	if err != nil {
		t.Fatalf("failed to read psk: %v", err)
	}

	if psk.Signature != "CM3D21_PSK" {
		t.Errorf("expected signature CM3D21_PSK, got %s", psk.Signature)
	}

	// Test Dump
	var buf bytes.Buffer
	err = psk.Dump(&buf)
	if err != nil {
		t.Fatalf("failed to dump psk: %v", err)
	}

	// Re-read from dumped buffer
	psk2, err := ReadPsk(&buf)
	if err != nil {
		t.Fatalf("failed to re-read dumped psk: %v", err)
	}

	// Compare basic fields
	if psk.Version != psk2.Version {
		t.Errorf("version mismatch: %d != %d", psk.Version, psk2.Version)
	}
}
