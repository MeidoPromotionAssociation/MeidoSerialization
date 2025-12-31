package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAnm(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.anm")
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

			anm, err := ReadAnm(f)
			if err != nil {
				t.Fatalf("failed to read anm: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = anm.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump anm: %v", err)
			}

			// Re-read from dumped buffer
			anm2, err := ReadAnm(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped anm: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(anm, anm2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
