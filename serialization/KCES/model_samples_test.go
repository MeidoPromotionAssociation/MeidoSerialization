package KCES

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
)

func TestModelSamples(t *testing.T) {
	assertPartsSamplesForSuffixRoundTrip(t, ".model", DecodeModel, EncodeModel)
}

func TestModelSamplesReferenceNativeMMeshAssets(t *testing.T) {
	for _, path := range partsSamplePathsBySuffix(t, ".model") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read model sample: %v", err)
			}
			model, err := DecodeModel(data)
			if err != nil {
				t.Fatalf("decode model: %v", err)
			}
			if model.MeshFileName == nil || !strings.HasSuffix(strings.ToLower(*model.MeshFileName), ".mmesh") {
				t.Fatalf("meshFileName = %v, want a native .mmesh Unity Mesh reference", model.MeshFileName)
			}
		})
	}
}

func TestEncodeModelAssetsLookupFieldRecalculatesByDefaultAndCanBeDisabled(t *testing.T) {
	fileName := "MixedCase.model"
	model := NewModel()
	model.FileName = &fileName
	model.ID = 1
	assets := &ModelAssets{Assets: []*Model{model}}

	defaultWire, err := EncodeModelAssets(assets)
	if err != nil {
		t.Fatalf("EncodeModelAssets: %v", err)
	}
	defaultValue, err := DecodeModelAssets(defaultWire)
	if err != nil {
		t.Fatalf("DecodeModelAssets: %v", err)
	}
	if got, want := defaultValue.Assets[0].ID, ct.HashString(fileName); got != want {
		t.Fatalf("default ID = %d, want %d", got, want)
	}

	preservedWire, err := EncodeModelAssetsWithOptions(assets, &LookupHashOptions{RecalculateHash: false})
	if err != nil {
		t.Fatalf("EncodeModelAssetsWithOptions preserve: %v", err)
	}
	preserved, err := DecodeModelAssets(preservedWire)
	if err != nil {
		t.Fatalf("DecodeModelAssets preserve: %v", err)
	}
	if preserved.Assets[0].ID != 1 {
		t.Fatalf("disabled recalculation changed ID: %d", preserved.Assets[0].ID)
	}
	if model.ID != 1 {
		t.Fatalf("encoding mutated input ID: %d", model.ID)
	}
}
