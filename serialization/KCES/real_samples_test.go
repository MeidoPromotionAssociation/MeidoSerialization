package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestRealKCESPresetSamplesRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		framing ct.VirtualDirectoryFraming
	}{
		{
			name:    "legacy color layouts",
			path:    filepath.Join("..", "..", "testdata", "aba", "Preset", "box ex", "pre_muku.preset"),
			framing: ct.VirtualDirectoryFramingLegacy,
		},
		{
			name:    "extended container framing",
			path:    filepath.Join("..", "..", "testdata", "aba", "Preset", "box ex", "pre_crc_streetdevil.preset"),
			framing: ct.VirtualDirectoryFramingExtended,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := readOptionalKCESRealSample(t, test.path)
			decoded, err := DecodeExpandedKCESPreset(data)
			if err != nil {
				t.Fatalf("DecodeExpandedKCESPreset(%s): %v", test.path, err)
			}
			if decoded.ContainerFraming != test.framing {
				t.Fatalf("container framing = %d, want %d", decoded.ContainerFraming, test.framing)
			}
			jsonData, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("marshal decoded preset JSON: %v", err)
			}
			var fromJSON ExpandedKCESPreset
			if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
				t.Fatalf("unmarshal decoded preset JSON: %v", err)
			}
			decoded = &fromJSON
			reencoded, err := EncodeExpandedKCESPreset(decoded)
			if err != nil {
				t.Fatalf("EncodeExpandedKCESPreset(%s): %v", test.path, err)
			}
			roundTrip, err := DecodeExpandedKCESPreset(reencoded)
			if err != nil {
				t.Fatalf("decode re-encoded %s: %v", test.path, err)
			}
			if !reflect.DeepEqual(roundTrip, decoded) {
				t.Fatalf("semantic preset round trip changed %s", test.path)
			}
		})
	}
}

func TestRealKCESSystemDataRoundTrip(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "aba", "system.dat")
	data := readOptionalKCESRealSample(t, path)
	decoded, err := DecodeKCESSystemData(data)
	if err != nil {
		t.Fatalf("DecodeKCESSystemData(%s): %v", path, err)
	}
	if decoded.ContainerFraming != ct.VirtualDirectoryFramingExtended {
		t.Fatalf("container framing = %d, want extended", decoded.ContainerFraming)
	}
	jsonData, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal decoded system.dat JSON: %v", err)
	}
	var fromJSON KCESSystemData
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("unmarshal decoded system.dat JSON: %v", err)
	}
	decoded = &fromJSON
	reencoded, err := EncodeKCESSystemData(decoded)
	if err != nil {
		t.Fatalf("EncodeKCESSystemData(%s): %v", path, err)
	}
	if len(reencoded) < ct.HeaderSize || reencoded[7] != ct.SerializeTypeMsgPackExtended {
		t.Fatalf("re-encoded system.dat does not use extended framing")
	}
	roundTrip, err := DecodeKCESSystemData(reencoded)
	if err != nil {
		t.Fatalf("decode re-encoded system.dat: %v", err)
	}
	if !reflect.DeepEqual(roundTrip, decoded) {
		t.Fatal("semantic system.dat round trip changed")
	}
}

func readOptionalKCESRealSample(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skipf("real sample %s is not available", path)
	}
	if err != nil {
		t.Fatalf("read real sample %s: %v", path, err)
	}
	return data
}
