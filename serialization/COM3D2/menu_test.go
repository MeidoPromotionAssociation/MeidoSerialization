package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMenu(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.menu")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
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

			// Compare complete structure
			if !reflect.DeepEqual(menu, menu2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
