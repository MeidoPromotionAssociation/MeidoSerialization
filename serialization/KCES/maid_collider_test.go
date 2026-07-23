package KCES

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

func TestMaidColliderRealSystemABASamples(t *testing.T) {
	samples := readMaidColliderSamplesFromSystemABA(t)
	wantSizes := map[string]int{
		// These are TextAsset m_Script sizes. The containing Unity object is
		// slightly larger because it also stores m_Name and alignment padding.
		"maid_collider":       3002,
		"maid_collider_touch": 2869,
	}
	if len(samples) != len(wantSizes) {
		t.Fatalf("found %d maid collider TextAssets, want %d: %v", len(samples), len(wantSizes), samples)
	}
	for name, data := range samples {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			if len(data) != wantSizes[name] {
				t.Fatalf("sample size = %d, want %d", len(data), wantSizes[name])
			}
			decoded, err := DecodeMaidCollider(data)
			if err != nil {
				t.Fatalf("DecodeMaidCollider: %v", err)
			}
			if decoded.Format != MaidColliderFormat || len(decoded.Colliders) == 0 {
				t.Fatalf("incomplete decoded sample: %+v", decoded)
			}
			t.Logf("parsed %d capsule colliders", len(decoded.Colliders))
			encoded, err := EncodeMaidCollider(decoded)
			if err != nil {
				t.Fatalf("EncodeMaidCollider: %v", err)
			}
			if !bytes.Equal(encoded, data) {
				t.Fatalf("real sample changed after byte-exact round-trip: got %d bytes, want %d", len(encoded), len(data))
			}
			redecoded, err := DecodeMaidCollider(encoded)
			if err != nil || !reflect.DeepEqual(redecoded, decoded) {
				t.Fatalf("semantic round-trip mismatch: err=%v got=%+v want=%+v", err, redecoded, decoded)
			}
		})
	}
}

func TestMaidColliderRejectsMalformedButPreservesRuntimeValues(t *testing.T) {
	valid := &MaidColliderFile{Colliders: []MaidCapsuleCollider{{
		BonePath:  "Bip01/Bip01 Pelvis",
		Center:    Vector3{X: 1, Y: 2, Z: 3},
		Direction: 1,
		Height:    2,
		Radius:    0.5,
	}}}
	encoded, err := EncodeMaidCollider(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, cut := range []int{0, 3, len(encoded) - 1} {
		if _, err := DecodeMaidCollider(encoded[:cut]); err == nil {
			t.Fatalf("truncated payload at %d unexpectedly decoded", cut)
		}
	}
	extended := append(append([]byte(nil), encoded...), 0xde, 0xad)
	withTrailing, err := DecodeMaidCollider(extended)
	if err != nil || !bytes.Equal(withTrailing.TrailingData, []byte{0xde, 0xad}) {
		t.Fatalf("trailing bytes were not preserved: value=%+v err=%v", withTrailing, err)
	}
	reencodedTrailing, err := EncodeMaidCollider(withTrailing)
	if err != nil || !bytes.Equal(reencodedTrailing, extended) {
		t.Fatalf("trailing-byte round trip = %x, %v; want %x", reencodedTrailing, err, extended)
	}

	tests := []struct {
		name string
		edit func(*MaidColliderFile)
	}{
		{"missing Bip01", func(v *MaidColliderFile) { v.Colliders[0].BonePath = "root/pelvis" }},
		{"future direction", func(v *MaidColliderFile) { v.Colliders[0].Direction = 3 }},
		{"nan", func(v *MaidColliderFile) { v.Colliders[0].Radius = math.Float32frombits(0x7fc12345) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copyValue := *valid
			copyValue.Colliders = append([]MaidCapsuleCollider(nil), valid.Colliders...)
			test.edit(&copyValue)
			wire, err := EncodeMaidCollider(&copyValue)
			if err != nil {
				t.Fatalf("EncodeMaidCollider: %v", err)
			}
			got, err := DecodeMaidCollider(wire)
			if err != nil {
				t.Fatalf("DecodeMaidCollider: %v", err)
			}
			if test.name == "nan" {
				if math.Float32bits(got.Colliders[0].Radius) != 0x7fc12345 {
					t.Fatalf("NaN bits changed: %08x", math.Float32bits(got.Colliders[0].Radius))
				}
			} else if !reflect.DeepEqual(got.Colliders, copyValue.Colliders) {
				t.Fatalf("runtime value changed: got %+v want %+v", got.Colliders, copyValue.Colliders)
			}
		})
	}
}

func TestMaidColliderRejectsNegativeCount(t *testing.T) {
	wire := []byte{0xff, 0xff, 0xff, 0xff, 1, 2, 3}
	if _, err := DecodeMaidCollider(wire); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("negative count error = %v", err)
	}
}

func TestMaidColliderLargeTruncatedCountUsesPhysicalBounds(t *testing.T) {
	wire := []byte{0x01, 0x00, 0x10, 0x00, 0}
	if _, err := DecodeMaidCollider(wire); err == nil || !strings.Contains(err.Error(), "cannot fit") {
		t.Fatalf("error = %v, want physical truncation rather than an arbitrary count limit", err)
	}
}

func readMaidColliderSamplesFromSystemABA(t *testing.T) map[string][]byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "aba", "system.aba")
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("system.aba is unavailable: %v", err)
	}
	defer f.Close()
	abaFile, err := aba.ReadAba(f)
	if err != nil {
		t.Fatalf("ReadAba(system.aba): %v", err)
	}
	result := map[string][]byte{}
	for directoryIndex, directory := range abaFile.BlockInfo.DirectoryInfos {
		if !directory.IsSerialized() {
			continue
		}
		data, err := abaFile.GetFileData(int64(directoryIndex))
		if err != nil {
			t.Fatalf("GetFileData(%s): %v", directory.Name, err)
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			t.Fatalf("ReadAssetsFile(%s): %v", directory.Name, err)
		}
		for i := range af.Metadata.AssetInfos {
			info := &af.Metadata.AssetInfos[i]
			if info.TypeId != aba.ClassIDTextAsset {
				continue
			}
			name, script, err := af.GetTextAssetData(info)
			if err != nil {
				t.Fatalf("GetTextAssetData(PathID %d): %v", info.PathId, err)
			}
			if name == "maid_collider" || name == "maid_collider_touch" {
				result[name] = append([]byte(nil), script...)
			}
		}
	}
	return result
}
