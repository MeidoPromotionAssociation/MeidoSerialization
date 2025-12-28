package COM3D2

import (
	"bytes"
	"os"
	"testing"
)

func TestAnm(t *testing.T) {
	testFiles := []string{
		"../../testdata/test.anm",
		"../../testdata/test2.anm",
	}

	for _, filePath := range testFiles {
		t.Run(filePath, func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()

			anm, err := ReadAnm(f)
			if err != nil {
				t.Fatalf("failed to read anm: %v", err)
			}

			if anm.Signature != "CM3D2_ANIM" {
				t.Errorf("expected signature CM3D2_ANIM, got %s", anm.Signature)
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

			// Compare basic fields
			if anm.Version != anm2.Version {
				t.Errorf("version mismatch: %d != %d", anm.Version, anm2.Version)
			}
			if len(anm.BoneCurves) != len(anm2.BoneCurves) {
				t.Errorf("bone curves count mismatch: %d != %d", len(anm.BoneCurves), len(anm2.BoneCurves))
			}
		})
	}
}
