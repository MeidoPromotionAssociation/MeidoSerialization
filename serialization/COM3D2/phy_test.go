package COM3D2

import (
	"bytes"
	"os"
	"testing"
)

func TestPhy(t *testing.T) {
	filePath := "../../testdata/test.phy"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	phy, err := ReadPhy(f)
	if err != nil {
		t.Fatalf("failed to read phy: %v", err)
	}

	if phy.Signature != "CM3D21_PHY" {
		t.Errorf("expected signature CM3D21_PHY, got %s", phy.Signature)
	}

	// Test Dump
	var buf bytes.Buffer
	err = phy.Dump(&buf)
	if err != nil {
		t.Fatalf("failed to dump phy: %v", err)
	}

	// Re-read from dumped buffer
	phy2, err := ReadPhy(&buf)
	if err != nil {
		t.Fatalf("failed to re-read dumped phy: %v", err)
	}

	// Compare basic fields
	if phy.Version != phy2.Version {
		t.Errorf("version mismatch: %d != %d", phy.Version, phy2.Version)
	}
}
