package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPmat(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.pmat")
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

			pmat, err := ReadPMat(f)
			if err != nil {
				t.Fatalf("failed to read pmat: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = pmat.Dump(&buf, false)
			if err != nil {
				t.Fatalf("failed to dump pmat: %v", err)
			}

			// Re-read from dumped buffer
			pmat2, err := ReadPMat(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped pmat: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(pmat, pmat2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
