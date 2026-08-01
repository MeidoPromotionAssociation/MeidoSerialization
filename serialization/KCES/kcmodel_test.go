package KCES

import "testing"

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
