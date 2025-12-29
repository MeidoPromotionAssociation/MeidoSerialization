package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMate(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.mate")
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
			mate, err := ReadMate(br)
			if err != nil {
				t.Fatalf("failed to read mate: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = mate.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump mate: %v", err)
			}

			// Re-read from dumped buffer
			br2 := bufio.NewReader(&buf)
			mate2, err := ReadMate(br2)
			if err != nil {
				t.Fatalf("failed to re-read dumped mate: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(mate, mate2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
