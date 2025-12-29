package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCol(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.col")
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

			col, err := ReadCol(f)
			if err != nil {
				t.Fatalf("failed to read col: %v", err)
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

			// Compare complete structure
			if !reflect.DeepEqual(col, col2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
