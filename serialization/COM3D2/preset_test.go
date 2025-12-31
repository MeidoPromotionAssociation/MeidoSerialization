package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPreset(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.preset")
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

			preset, err := ReadPreset(f)
			if err != nil {
				t.Fatalf("failed to read preset: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = preset.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump preset: %v", err)
			}

			// Re-read from dumped buffer
			preset2, err := ReadPreset(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped preset: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(preset, preset2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}
