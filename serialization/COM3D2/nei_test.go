package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNei(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.nei")
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

			nei, err := ReadNei(f, nil)
			if err != nil {
				t.Fatalf("failed to read nei: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = nei.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump nei: %v", err)
			}

			// Re-read from dumped buffer
			nei2, err := ReadNei(&buf, nil)
			if err != nil {
				t.Fatalf("failed to re-read dumped nei: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(nei, nei2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
