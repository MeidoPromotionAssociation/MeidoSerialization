package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestMaterialLayoutRoundTripPreservesKCESGeneration(t *testing.T) {
	tests := []struct {
		name     string
		material *Material
		width    int
		version  int32
	}{
		{name: "KCES", material: NewMaterial(), width: materialLegacyWidth, version: 801},
		{name: "KCES2", material: NewKCES2Material(), width: materialKCES2Width, version: 802},
	}
	for index := range tests {
		tests[index].material.Version = tests[index].version
	}
	tests[1].material.KeywordProps = []*KeywordProp{{Type: 300, Value: true}}
	tests[1].material.RenderQueue = 2450

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := EncodeMaterialAssets(&MaterialAssets{Assets: []*Material{test.material}})
			if err != nil {
				t.Fatalf("EncodeMaterialAssets: %v", err)
			}
			if got := nestedCompressedArrayWidth(t, encoded, 1, 0); got != test.width {
				t.Fatalf("encoded Material width = %d, want %d", got, test.width)
			}

			decoded, err := DecodeMaterialAssets(encoded)
			if err != nil {
				t.Fatalf("DecodeMaterialAssets: %v", err)
			}
			if got := int(decoded.Assets[0].MessagePackIndexedObjectWidth()); got != test.width {
				t.Fatalf("decoded Material width = %d, want %d", got, test.width)
			}
			if decoded.Assets[0].Version != test.version {
				t.Fatalf("decoded Material version = %d, want %d", decoded.Assets[0].Version, test.version)
			}
			if test.width == materialKCES2Width {
				if len(decoded.Assets[0].KeywordProps) != 1 || decoded.Assets[0].RenderQueue != 2450 {
					t.Fatalf("KCES2 material fields were not preserved: %+v", decoded.Assets[0])
				}
			}

			reencoded, err := EncodeMaterialAssets(decoded)
			if err != nil {
				t.Fatalf("re-encode MaterialAssets: %v", err)
			}
			if got := nestedCompressedArrayWidth(t, reencoded, 1, 0); got != test.width {
				t.Fatalf("re-encoded Material width = %d, want %d", got, test.width)
			}
		})
	}
}

func TestHistoricalMaterialLayoutRejectsKCES2TailField(t *testing.T) {
	material := NewMaterial()
	material.RenderQueue = 2450
	if _, err := EncodeMaterialAssets(&MaterialAssets{Assets: []*Material{material}}); err == nil || !strings.Contains(err.Error(), "not representable") {
		t.Fatalf("EncodeMaterialAssets error = %v, want unrepresentable-tail error", err)
	}
}

func TestEncodeMaterialAssetsLookupFieldRecalculatesByDefaultAndCanBeDisabled(t *testing.T) {
	fileName := "MixedCase.mate"
	material := NewKCES2Material()
	material.FileName = &fileName
	material.ID = 1
	assets := &MaterialAssets{Assets: []*Material{material}}

	defaultWire, err := EncodeMaterialAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMaterialAssets: %v", err)
	}
	defaultValue, err := DecodeMaterialAssets(defaultWire)
	if err != nil {
		t.Fatalf("DecodeMaterialAssets: %v", err)
	}
	if got, want := defaultValue.Assets[0].ID, ct.HashString(fileName); got != want {
		t.Fatalf("default ID = %d, want %d", got, want)
	}

	preservedWire, err := EncodeMaterialAssetsWithOptions(assets, &LookupHashOptions{RecalculateHash: false})
	if err != nil {
		t.Fatalf("EncodeMaterialAssetsWithOptions preserve: %v", err)
	}
	preserved, err := DecodeMaterialAssets(preservedWire)
	if err != nil {
		t.Fatalf("DecodeMaterialAssets preserve: %v", err)
	}
	if preserved.Assets[0].ID != 1 {
		t.Fatalf("disabled recalculation changed ID: %d", preserved.Assets[0].ID)
	}
	if material.ID != 1 {
		t.Fatalf("encoding mutated input ID: %d", material.ID)
	}
}
