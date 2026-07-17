package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPhy(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.phy")
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

			phy, err := ReadPhy(f)
			if err != nil {
				t.Fatalf("failed to read phy: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = phy.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump phy: %v", err)
			}

			// Re-read from dumped buffer
			phy2, err := ReadPhy(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped phy: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(phy, phy2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}

func TestPhyDumpRejectsContradictoryCountsBeforeWriting(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Phy)
	}{
		{name: "negative collider count", mutate: func(value *Phy) { value.CollidersCount = -1 }},
		{name: "negative exclusion count", mutate: func(value *Phy) { value.ExclusionsCount = -1 }},
		{name: "partial values in non-partial mode", mutate: func(value *Phy) {
			value.EnablePartialDamping = PartialModeStaticOrCurve
			value.PartialDamping = []BoneValue{{BoneName: "bone", Value: 1}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := &Phy{}
			test.mutate(value)
			var output bytes.Buffer
			if err := value.Dump(&output); err == nil {
				t.Fatal("Dump accepted contradictory PHY data")
			}
			if output.Len() != 0 {
				t.Fatalf("rejected PHY wrote %d bytes", output.Len())
			}
		})
	}
}
