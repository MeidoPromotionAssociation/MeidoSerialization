package KCES

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSharePresetPreservesVersionAndPayloadStructure(t *testing.T) {
	gender := "maid"
	png := "png"
	jpg := "jpg"
	menu := "sample.menu"
	value := NewSharePreset()
	value.Version = 77
	value.ExportData.Gender = &gender
	value.ExportData.Height = 158
	value.ExportData.Waist = 61
	value.ExportData.ItemMPNs["wear"] = []*string{&menu, nil}
	value.ExportData.ParamMPNs["sintyou"] = 158
	value.PresetData = []byte("preset")
	value.BaseThumbnail = &SharePresetThumbnail{Extension: &png, Data: []byte("base")}
	value.AdditionalThumbnails = []*SharePresetThumbnail{{Extension: &jpg, Data: []byte("additional")}}
	value.AppendedData = []byte(`{"server":"metadata"}`)

	wire, err := EncodeSharePreset(value)
	if err != nil {
		t.Fatalf("EncodeSharePreset: %v", err)
	}
	if got := binary.LittleEndian.Uint32(wire[7:11]); got != value.Version {
		t.Fatalf("SharePreset version = %d, want %d", got, value.Version)
	}
	metadataOffset := binary.LittleEndian.Uint32(wire[11:15])
	metadataLength := binary.LittleEndian.Uint32(wire[15:19])
	var wireMetadata map[string]json.RawMessage
	if err := json.Unmarshal(wire[metadataOffset:metadataOffset+metadataLength], &wireMetadata); err != nil {
		t.Fatalf("decode SharePreset wire metadata: %v", err)
	}
	if _, ok := wireMetadata["weist"]; !ok {
		t.Fatal("SharePreset wire metadata is missing the game-compatible weist key")
	}
	if _, ok := wireMetadata["waist"]; ok {
		t.Fatal("SharePreset wire metadata unexpectedly uses the editing field name waist")
	}
	editingJSON, err := json.Marshal(value.ExportData)
	if err != nil {
		t.Fatalf("marshal SharePreset editing metadata: %v", err)
	}
	if !bytes.Contains(editingJSON, []byte(`"waist":61`)) || bytes.Contains(editingJSON, []byte(`"weist"`)) {
		t.Fatalf("SharePreset editing JSON uses an incorrect waist spelling: %s", editingJSON)
	}
	decoded, err := DecodeSharePreset(wire)
	if err != nil {
		t.Fatalf("DecodeSharePreset: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("SharePreset round trip mismatch\n got: %#v\nwant: %#v", decoded, value)
	}
}

func TestSharePresetKCES2SamplesRoundTrip(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "KCES2", "*.shpreset"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no KCES2 .shpreset samples")
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeSharePreset(original)
			if err != nil {
				t.Fatalf("DecodeSharePreset: %v", err)
			}
			if decoded.Signature != SharePresetSignature || decoded.Version != SharePresetVersion {
				t.Fatalf("SharePreset header = %q version %d", decoded.Signature, decoded.Version)
			}
			if _, err := DecodeKCESPreset(decoded.PresetData); err != nil {
				t.Fatalf("decode embedded KCES preset: %v", err)
			}
			reencoded, err := EncodeSharePreset(decoded)
			if err != nil {
				t.Fatalf("EncodeSharePreset: %v", err)
			}
			roundTrip, err := DecodeSharePreset(reencoded)
			if err != nil {
				t.Fatalf("decode re-encoded SharePreset: %v", err)
			}
			if !reflect.DeepEqual(roundTrip, decoded) {
				t.Fatal("SharePreset semantic structure changed after re-encoding")
			}
			if !bytes.Equal(roundTrip.PresetData, decoded.PresetData) || !bytes.Equal(roundTrip.AppendedData, decoded.AppendedData) {
				t.Fatal("SharePreset embedded preset or appended data changed")
			}
		})
	}
}

func TestSharePresetRejectsNonContiguousPayloadOffsets(t *testing.T) {
	value := NewSharePreset()
	value.PresetData = []byte("preset")
	wire, err := EncodeSharePreset(value)
	if err != nil {
		t.Fatalf("EncodeSharePreset: %v", err)
	}
	metadataOffset := binary.LittleEndian.Uint32(wire[11:15])
	metadata := wire[metadataOffset:]
	old := []byte(`"Item1": 19`)
	updated := []byte(`"Item1": 20`)
	changed := bytes.Replace(metadata, old, updated, 1)
	if bytes.Equal(changed, metadata) {
		t.Fatal("test metadata offset was not found")
	}
	copy(wire[metadataOffset:], changed)
	if _, err := DecodeSharePreset(wire); err == nil {
		t.Fatal("DecodeSharePreset accepted a non-contiguous preset offset")
	}
}
