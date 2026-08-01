package KCES

import "testing"

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
