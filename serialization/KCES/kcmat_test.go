package KCES

import "testing"

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
