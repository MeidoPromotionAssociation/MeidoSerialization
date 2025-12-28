package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMenuService(t *testing.T) {
	s := &MenuService{}
	inputPath := "../../testdata/test.menu"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.menu not found, skipping test")
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.menu")
	jsonPath := filepath.Join(tempDir, "test.menu.json")
	backPath := filepath.Join(tempDir, "test_back.menu")

	// 1. Test ReadMenuFile
	menu, err := s.ReadMenuFile(inputPath)
	if err != nil {
		t.Fatalf("ReadMenuFile failed: %v", err)
	}
	if menu == nil {
		t.Fatal("ReadMenuFile returned nil")
	}

	// 2. Test WriteMenuFile
	err = s.WriteMenuFile(outputPath, menu)
	if err != nil {
		t.Fatalf("WriteMenuFile failed: %v", err)
	}

	// 3. Test ConvertMenuToJson
	err = s.ConvertMenuToJson(inputPath, jsonPath)
	if err != nil {
		t.Fatalf("ConvertMenuToJson failed: %v", err)
	}

	// 4. Test ConvertJsonToMenu
	err = s.ConvertJsonToMenu(jsonPath, backPath)
	if err != nil {
		t.Fatalf("ConvertJsonToMenu failed: %v", err)
	}

	// Optional: Re-read and verify
	menuBack, err := s.ReadMenuFile(backPath)
	if err != nil {
		t.Fatalf("Read re-converted menu failed: %v", err)
	}
	if menuBack.Signature != menu.Signature {
		t.Errorf("Signature mismatch: %s != %s", menuBack.Signature, menu.Signature)
	}
}
