package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestMenuService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.menu")
	if err != nil {
		t.Fatal(err)
	}

	s := &MenuService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
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

			// Re-read and verify consistency
			menuBack, err := s.ReadMenuFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted menu failed: %v", err)
			}
			if !reflect.DeepEqual(menu, menuBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			menuRepack, err := s.ReadMenuFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written menu failed: %v", err)
			}
			if !reflect.DeepEqual(menu, menuRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
