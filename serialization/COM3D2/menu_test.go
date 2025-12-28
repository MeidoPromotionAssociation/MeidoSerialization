package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"testing"
)

func TestMenu(t *testing.T) {
	filePath := "../../testdata/test.menu"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	menu, err := ReadMenu(br)
	if err != nil {
		t.Fatalf("failed to read menu: %v", err)
	}

	if menu.Signature != "CM3D2_MENU" {
		t.Errorf("expected signature CM3D2_MENU, got %s", menu.Signature)
	}

	// Test Dump
	var buf bytes.Buffer
	err = menu.Dump(&buf)
	if err != nil {
		t.Fatalf("failed to dump menu: %v", err)
	}

	// Re-read from dumped buffer
	br2 := bufio.NewReader(&buf)
	menu2, err := ReadMenu(br2)
	if err != nil {
		t.Fatalf("failed to re-read dumped menu: %v", err)
	}

	// Compare basic fields
	if menu.Version != menu2.Version {
		t.Errorf("version mismatch: %d != %d", menu.Version, menu2.Version)
	}
	if menu.ItemName != menu2.ItemName {
		t.Errorf("item name mismatch: %s != %s", menu.ItemName, menu2.ItemName)
	}
}
