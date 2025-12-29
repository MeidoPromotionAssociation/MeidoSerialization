package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTex(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.tex")
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

			tex, err := ReadTex(f)
			if err != nil {
				t.Fatalf("failed to read tex: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = tex.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump tex: %v", err)
			}

			// Re-read from dumped buffer
			tex2, err := ReadTex(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped tex: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(tex, tex2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
