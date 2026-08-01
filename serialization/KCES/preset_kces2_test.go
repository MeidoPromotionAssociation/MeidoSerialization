package KCES

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKCESPresetConstructorsSelectMetadataGeneration(t *testing.T) {
	legacy, err := NewKCESPreset()
	if err != nil {
		t.Fatalf("NewKCESPreset: %v", err)
	}
	legacy.Meta.SetPresetName("legacy")
	if legacy.ContainerVersion != 1000 || legacy.MaidData.Version != 1000 || legacy.Meta.Version != 1000 {
		t.Fatalf("legacy versions = container %d, core %d, meta %d", legacy.ContainerVersion, legacy.MaidData.Version, legacy.Meta.Version)
	}
	if _, ok := legacy.Meta.Data["presetName"]; !ok {
		t.Fatalf("legacy metadata keys = %#v, want presetName", legacy.Meta.Data)
	}

	kces2, err := NewKCES2Preset()
	if err != nil {
		t.Fatalf("NewKCES2Preset: %v", err)
	}
	kces2.Meta.SetPresetName("kces2")
	if kces2.ContainerVersion != 1000 || kces2.MaidData.Version != 1000 || kces2.Meta.Version != 1001 {
		t.Fatalf("KCES2 versions = container %d, core %d, meta %d", kces2.ContainerVersion, kces2.MaidData.Version, kces2.Meta.Version)
	}
	if _, ok := kces2.Meta.Data["name"]; !ok {
		t.Fatalf("KCES2 metadata keys = %#v, want name", kces2.Meta.Data)
	}

	wire, err := EncodeKCESPreset(kces2)
	if err != nil {
		t.Fatalf("EncodeKCESPreset: %v", err)
	}
	decoded, err := DecodeKCESPreset(wire)
	if err != nil {
		t.Fatalf("DecodeKCESPreset: %v", err)
	}
	if decoded.ContainerVersion != 1000 || decoded.MaidData.Version != 1000 || decoded.Meta.Version != 1001 {
		t.Fatalf("decoded versions = container %d, core %d, meta %d", decoded.ContainerVersion, decoded.MaidData.Version, decoded.Meta.Version)
	}
	if got := decoded.Meta.PresetName(); got != "kces2" {
		t.Fatalf("decoded preset name = %q, want kces2", got)
	}
}

func TestKCES2PresetSamplesPreserveStoredVersions(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "KCES2", "*.preset"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no KCES2 .preset samples")
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeKCESPreset(original)
			if err != nil {
				t.Fatalf("DecodeKCESPreset: %v", err)
			}
			containerVersion := decoded.ContainerVersion
			coreVersion := decoded.MaidData.Version
			var metaVersion int32
			if decoded.Meta != nil {
				metaVersion = decoded.Meta.Version
			}
			reencoded, err := EncodeKCESPreset(decoded)
			if err != nil {
				t.Fatalf("EncodeKCESPreset: %v", err)
			}
			roundTrip, err := DecodeKCESPreset(reencoded)
			if err != nil {
				t.Fatalf("decode re-encoded KCES preset: %v", err)
			}
			if roundTrip.ContainerVersion != containerVersion || roundTrip.MaidData.Version != coreVersion {
				t.Fatalf("preset versions changed from container %d core %d to container %d core %d", containerVersion, coreVersion, roundTrip.ContainerVersion, roundTrip.MaidData.Version)
			}
			if roundTrip.Meta != nil && roundTrip.Meta.Version != metaVersion {
				t.Fatalf("preset meta version changed from %d to %d", metaVersion, roundTrip.Meta.Version)
			}
		})
	}
}
