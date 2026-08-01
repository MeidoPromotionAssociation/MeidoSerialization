package KCES

import (
	"bytes"
	"testing"
)

func TestKCTexRoundTripIsByteExact(t *testing.T) {
	value := NewKCTex()
	value.TextureName = "hair_texture"
	value.Data = []byte("\x89PNG\r\n\x1a\nsynthetic")

	wire, err := EncodeKCTex(value)
	if err != nil {
		t.Fatalf("EncodeKCTex: %v", err)
	}
	decoded, err := DecodeKCTex(wire)
	if err != nil {
		t.Fatalf("DecodeKCTex: %v", err)
	}
	reencoded, err := EncodeKCTex(decoded)
	if err != nil {
		t.Fatalf("re-encode KCTex: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatalf("KCTex bytes changed\n got: %x\nwant: %x", reencoded, wire)
	}
}
