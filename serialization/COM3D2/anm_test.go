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

func TestAnmReadsSingleOptionalBustFlagAndWritesCompletePair(t *testing.T) {
	original := Anm{Signature: AnmSignature, Version: 1001, BustKeyLeft: true}
	var complete bytes.Buffer
	if err := original.Dump(&complete); err != nil {
		t.Fatal(err)
	}
	wire := complete.Bytes()
	short := wire[:len(wire)-1]
	decoded, err := ReadAnm(bytes.NewReader(short))
	if err != nil {
		t.Fatalf("ReadAnm: %v", err)
	}
	if !decoded.BustKeyLeft || decoded.BustKeyRight {
		t.Fatalf("single bust flag decoded as left=%v right=%v", decoded.BustKeyLeft, decoded.BustKeyRight)
	}
	var normalized bytes.Buffer
	if err := decoded.Dump(&normalized); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if !bytes.Equal(normalized.Bytes(), wire) {
		t.Fatalf("complete bust pair mismatch: got %x want %x", normalized.Bytes(), wire)
	}
}

func TestAnmDumpRejectsUnrepresentablePropertyIndexBeforeWriting(t *testing.T) {
	value := Anm{BoneCurves: []BoneCurveData{{PropertyCurves: []PropertyCurve{{PropertyIndex: 156}}}}}
	var output bytes.Buffer
	if err := value.Dump(&output); err == nil {
		t.Fatal("Dump accepted a PropertyIndex that wraps the chunk byte")
	}
	if output.Len() != 0 {
		t.Fatalf("rejected ANM wrote %d bytes", output.Len())
	}
}

func TestAnmDumpRejectsBustFlagsUnavailableInStoredVersion(t *testing.T) {
	value := Anm{Signature: AnmSignature, Version: 1000, BustKeyLeft: true}
	var output bytes.Buffer
	if err := value.Dump(&output); err == nil {
		t.Fatal("Dump silently discarded a bust-key flag from version 1000")
	}
	if output.Len() != 0 {
		t.Fatalf("rejected ANM wrote %d bytes", output.Len())
	}
}
