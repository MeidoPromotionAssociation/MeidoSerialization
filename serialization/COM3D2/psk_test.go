package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
)

func TestPsk(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.psk")
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

func TestPskDumpRejectsGroupsUnsupportedByStoredVersion(t *testing.T) {
	value := Psk{
		Version:                   216,
		PanierRadiusDistribGroups: []PanierRadiusGroup{{BoneName: "bone"}},
	}
	var output bytes.Buffer
	if err := value.Dump(&output); err == nil {
		t.Fatal("Dump accepted radius groups in PSK version 216")
	}
	if output.Len() != 0 {
		t.Fatalf("rejected PSK wrote %d bytes", output.Len())
	}
}

func TestPskDump_KCESLegacyVersionSample(t *testing.T) {
	samplePath := kcesfixtures.TextAssetPath(t, "partsmeta.aba", "default_skirt.psk")
	data, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatalf("read KCES psk sample: %v", err)
	}
	psk, err := ReadPsk(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadPsk: %v", err)
	}
	if psk.Version >= 217 {
		t.Fatalf("expected legacy version sample, got %d", psk.Version)
	}

	var buf bytes.Buffer
	if err := psk.Dump(&buf); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	roundTrip, err := ReadPsk(&buf)
	if err != nil {
		t.Fatalf("ReadPsk dumped sample: %v", err)
	}
	if !reflect.DeepEqual(psk, roundTrip) {
		t.Fatalf("legacy psk changed after dump and re-read")
	}
}
