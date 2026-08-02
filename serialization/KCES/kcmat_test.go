package KCES

import (
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestKCMatPreservesKCES2Width(t *testing.T) {
	material := NewKCES2Material()
	material.RenderQueue = 2450
	wire, err := EncodeKCMat(material)
	if err != nil {
		t.Fatalf("EncodeKCMat: %v", err)
	}
	if got := nestedCompressedArrayWidth(t, wire); got != materialKCES2Width {
		t.Fatalf("KCMat width = %d, want %d", got, materialKCES2Width)
	}
	decoded, err := DecodeKCMat(wire)
	if err != nil {
		t.Fatalf("DecodeKCMat: %v", err)
	}
	if got := decoded.MessagePackIndexedObjectWidth(); got != materialKCES2Width {
		t.Fatalf("decoded KCMat width = %d, want %d", got, materialKCES2Width)
	}
}

func TestEncodeKCMatLookupFieldRecalculatesByDefaultAndCanBeDisabled(t *testing.T) {
	fileName := "stale.kcmat"
	writtenFileName := "MixedCase.kcmat"
	material := NewKCES2Material()
	material.FileName = &fileName
	material.ID = 1

	defaultWire, err := EncodeKCMat(material)
	if err != nil {
		t.Fatalf("EncodeKCMat: %v", err)
	}
	defaultValue, err := DecodeKCMat(defaultWire)
	if err != nil {
		t.Fatalf("DecodeKCMat: %v", err)
	}
	if got, want := defaultValue.ID, ct.HashString(fileName); got != want {
		t.Fatalf("default ID = %d, want %d", got, want)
	}

	preservedWire, err := EncodeKCMatWithOptions(material, &LookupHashOptions{RecalculateHash: false, FileName: writtenFileName})
	if err != nil {
		t.Fatalf("EncodeKCMatWithOptions preserve: %v", err)
	}
	preserved, err := DecodeKCMat(preservedWire)
	if err != nil {
		t.Fatalf("DecodeKCMat preserve: %v", err)
	}
	if preserved.FileName == nil || *preserved.FileName != fileName {
		t.Fatalf("preserved fileName = %v, want %q", preserved.FileName, fileName)
	}
	if preserved.ID != 1 {
		t.Fatalf("disabled recalculation changed ID: %d", preserved.ID)
	}

	wire, err := EncodeKCMatWithOptions(material, &LookupHashOptions{RecalculateHash: true, FileName: writtenFileName})
	if err != nil {
		t.Fatalf("EncodeKCMatWithOptions: %v", err)
	}
	decoded, err := DecodeKCMat(wire)
	if err != nil {
		t.Fatalf("DecodeKCMat: %v", err)
	}
	if decoded.FileName == nil || *decoded.FileName != writtenFileName {
		t.Fatalf("decoded fileName = %v, want %q", decoded.FileName, writtenFileName)
	}
	if got, want := decoded.ID, ct.HashString(writtenFileName); got != want {
		t.Fatalf("decoded ID = %d, want %d", got, want)
	}
	if material.ID != 1 {
		t.Fatalf("EncodeKCMat mutated input ID: %d", material.ID)
	}
}
