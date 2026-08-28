package KCES

import (
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
)

func TestKCModelPreservesHistoricalWidth(t *testing.T) {
	model := NewModel()
	model.Version = 812
	model.SetMessagePackIndexedObjectWidth(10)
	wire, err := EncodeKCModel(model)
	if err != nil {
		t.Fatalf("EncodeKCModel: %v", err)
	}
	if got := nestedCompressedArrayWidth(t, wire); got != 10 {
		t.Fatalf("KCModel width = %d, want 10", got)
	}
	decoded, err := DecodeKCModel(wire)
	if err != nil {
		t.Fatalf("DecodeKCModel: %v", err)
	}
	if got := decoded.MessagePackIndexedObjectWidth(); got != 10 {
		t.Fatalf("decoded KCModel width = %d, want 10", got)
	}
	if decoded.Version != model.Version {
		t.Fatalf("decoded KCModel version = %d, want %d", decoded.Version, model.Version)
	}
}

func TestEncodeKCModelLookupFieldRecalculatesByDefaultAndCanBeDisabled(t *testing.T) {
	fileName := "stale.kcmodel"
	writtenFileName := "MixedCase.kcmodel"
	model := NewModel()
	model.FileName = &fileName
	model.ID = 1

	defaultWire, err := EncodeKCModel(model)
	if err != nil {
		t.Fatalf("EncodeKCModel: %v", err)
	}
	defaultValue, err := DecodeKCModel(defaultWire)
	if err != nil {
		t.Fatalf("DecodeKCModel: %v", err)
	}
	if got, want := defaultValue.ID, ct.HashString(fileName); got != want {
		t.Fatalf("default ID = %d, want %d", got, want)
	}

	preservedWire, err := EncodeKCModelWithOptions(model, &LookupHashOptions{RecalculateHash: false, FileName: writtenFileName})
	if err != nil {
		t.Fatalf("EncodeKCModelWithOptions preserve: %v", err)
	}
	preserved, err := DecodeKCModel(preservedWire)
	if err != nil {
		t.Fatalf("DecodeKCModel preserve: %v", err)
	}
	if preserved.FileName == nil || *preserved.FileName != fileName {
		t.Fatalf("preserved fileName = %v, want %q", preserved.FileName, fileName)
	}
	if preserved.ID != 1 {
		t.Fatalf("disabled recalculation changed ID: %d", preserved.ID)
	}

	wire, err := EncodeKCModelWithOptions(model, &LookupHashOptions{RecalculateHash: true, FileName: writtenFileName})
	if err != nil {
		t.Fatalf("EncodeKCModelWithOptions: %v", err)
	}
	decoded, err := DecodeKCModel(wire)
	if err != nil {
		t.Fatalf("DecodeKCModel: %v", err)
	}
	if decoded.FileName == nil || *decoded.FileName != writtenFileName {
		t.Fatalf("decoded fileName = %v, want %q", decoded.FileName, writtenFileName)
	}
	if got, want := decoded.ID, ct.HashString(writtenFileName); got != want {
		t.Fatalf("decoded ID = %d, want %d", got, want)
	}
	if model.ID != 1 {
		t.Fatalf("EncodeKCModel mutated input ID: %d", model.ID)
	}
}
