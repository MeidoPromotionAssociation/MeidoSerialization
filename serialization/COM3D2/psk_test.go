package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPsk(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.psk")
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

			psk, err := ReadPsk(f)
			if err != nil {
				t.Fatalf("failed to read psk: %v", err)
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

			// Compare complete structure
			if !reflect.DeepEqual(psk, psk2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
