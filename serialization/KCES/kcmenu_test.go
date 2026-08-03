package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestKCMenuPreservesKCES2Width(t *testing.T) {
	menu := NewKCES2Menu()
	menu.HairMake = NewHairMake()
	wire, err := EncodeKCMenu(menu)
	if err != nil {
		t.Fatalf("EncodeKCMenu: %v", err)
	}
	if got := nestedCompressedArrayWidth(t, wire); got != menuKCES2Width {
		t.Fatalf("KCMenu width = %d, want %d", got, menuKCES2Width)
	}
	decoded, err := DecodeKCMenu(wire)
	if err != nil {
		t.Fatalf("DecodeKCMenu: %v", err)
	}
	if got := decoded.MessagePackIndexedObjectWidth(); got != menuKCES2Width {
		t.Fatalf("decoded KCMenu width = %d, want %d", got, menuKCES2Width)
	}
}

func TestEncodeKCMenuLookupFieldsRecalculateByDefaultAndCanBeDisabled(t *testing.T) {
	fileName := "stale.kcmenu"
	writtenFileName := "MixedCase.KCMENU"
	exportedGUID := "ABCDEF01-2345-6789-ABCD-EF0123456789"
	menu := NewKCES2Menu()
	menu.FileName = &fileName
	menu.ID = 1
	menu.GUID = 2
	menu.HairMake = NewHairMake()
	menu.HairMake.ExportedGUID = &exportedGUID

	defaultWire, err := EncodeKCMenu(menu)
	if err != nil {
		t.Fatalf("EncodeKCMenu: %v", err)
	}
	defaultValue, err := DecodeKCMenu(defaultWire)
	if err != nil {
		t.Fatalf("DecodeKCMenu: %v", err)
	}
	if got, want := defaultValue.ID, ct.HashStringIgnoreCase(fileName); got != want {
		t.Fatalf("default ID = %d, want %d", got, want)
	}
	if got, want := defaultValue.GUID, ct.HashStringIgnoreCase(exportedGUID); got != want {
		t.Fatalf("default GUID = %d, want %d", got, want)
	}

	preservedWire, err := EncodeKCMenuWithOptions(menu, &LookupHashOptions{RecalculateHash: false, FileName: writtenFileName})
	if err != nil {
		t.Fatalf("EncodeKCMenuWithOptions preserve: %v", err)
	}
	preserved, err := DecodeKCMenu(preservedWire)
	if err != nil {
		t.Fatalf("DecodeKCMenu preserve: %v", err)
	}
	if preserved.FileName == nil || *preserved.FileName != fileName {
		t.Fatalf("preserved fileName = %v, want %q", preserved.FileName, fileName)
	}
	if preserved.ID != 1 || preserved.GUID != 2 {
		t.Fatalf("disabled recalculation changed lookup fields: ID=%d GUID=%d", preserved.ID, preserved.GUID)
	}

	wire, err := EncodeKCMenuWithOptions(menu, &LookupHashOptions{RecalculateHash: true, FileName: writtenFileName})
	if err != nil {
		t.Fatalf("EncodeKCMenuWithOptions: %v", err)
	}
	decoded, err := DecodeKCMenu(wire)
	if err != nil {
		t.Fatalf("DecodeKCMenu: %v", err)
	}
	if decoded.FileName == nil || *decoded.FileName != writtenFileName {
		t.Fatalf("decoded fileName = %v, want %q", decoded.FileName, writtenFileName)
	}
	if got, want := decoded.ID, ct.HashStringIgnoreCase(writtenFileName); got != want {
		t.Fatalf("decoded ID = %d, want %d", got, want)
	}
	if got, want := decoded.GUID, ct.HashStringIgnoreCase(exportedGUID); got != want {
		t.Fatalf("decoded GUID = %d, want %d", got, want)
	}
	if menu.ID != 1 || menu.GUID != 2 {
		t.Fatalf("EncodeKCMenu mutated input IDs: ID=%d GUID=%d", menu.ID, menu.GUID)
	}
}

func TestEncodeKCMenuRejectsFileNameWithoutMenuExtension(t *testing.T) {
	fileName := "testmenu"
	menu := NewKCES2Menu()
	menu.FileName = &fileName
	if _, err := EncodeKCMenu(menu); err == nil || !strings.Contains(err.Error(), ".kcmenu") {
		t.Fatalf("EncodeKCMenu error = %v, want missing-extension error", err)
	}
	if _, err := EncodeKCMenuWithOptions(menu, &LookupHashOptions{RecalculateHash: false}); err == nil {
		t.Fatal("EncodeKCMenuWithOptions(preserve) accepted an extensionless filename")
	}
	if _, err := EncodeKCMenuWithOptions(menu, &LookupHashOptions{RecalculateHash: true, FileName: "fixed.kcmenu"}); err != nil {
		t.Fatalf("EncodeKCMenuWithOptions(override): %v", err)
	}
}

func TestEncodeKCMenuRegeneratesGUIDWithoutExportedGUID(t *testing.T) {
	fileName := "no_guid.kcmenu"
	menu := NewKCES2Menu()
	menu.FileName = &fileName
	menu.GUID = 99
	menu.HairMake = NewHairMake()

	first, err := EncodeKCMenuWithOptions(menu, &LookupHashOptions{RecalculateHash: true})
	if err != nil {
		t.Fatalf("EncodeKCMenuWithOptions: %v", err)
	}
	firstDecoded, err := DecodeKCMenu(first)
	if err != nil {
		t.Fatalf("DecodeKCMenu: %v", err)
	}
	if firstDecoded.GUID == 99 || firstDecoded.GUID == 0 {
		t.Fatalf("decoded GUID = %d, want a regenerated non-zero value", firstDecoded.GUID)
	}

	second, err := EncodeKCMenuWithOptions(menu, &LookupHashOptions{RecalculateHash: true})
	if err != nil {
		t.Fatalf("EncodeKCMenuWithOptions second: %v", err)
	}
	secondDecoded, err := DecodeKCMenu(second)
	if err != nil {
		t.Fatalf("DecodeKCMenu second: %v", err)
	}
	if secondDecoded.GUID == firstDecoded.GUID {
		t.Fatalf("consecutive encodes reused GUID %d", firstDecoded.GUID)
	}

	preserved, err := EncodeKCMenuWithOptions(menu, &LookupHashOptions{RecalculateHash: false})
	if err != nil {
		t.Fatalf("EncodeKCMenuWithOptions preserve: %v", err)
	}
	preservedDecoded, err := DecodeKCMenu(preserved)
	if err != nil {
		t.Fatalf("DecodeKCMenu preserve: %v", err)
	}
	if preservedDecoded.GUID != 99 {
		t.Fatalf("disabled recalculation changed GUID to %d, want preserved 99", preservedDecoded.GUID)
	}
	if menu.GUID != 99 {
		t.Fatalf("encoding mutated input GUID: %d", menu.GUID)
	}
}
