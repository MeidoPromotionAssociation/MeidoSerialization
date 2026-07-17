package aba

import (
	"math"
	"testing"
)

func TestWireLengthChecksUseFormatRanges(t *testing.T) {
	if got, err := int32WireLength("count", math.MaxInt32); err != nil || got != math.MaxInt32 {
		t.Fatalf("int32 max = %d, %v", got, err)
	}
	if _, err := int32WireLength("count", uint64(math.MaxInt32)+1); err == nil {
		t.Fatal("Int32 overflow unexpectedly accepted")
	}
	if got, err := uint32WireLength("size", math.MaxUint32); err != nil || got != math.MaxUint32 {
		t.Fatalf("uint32 max = %d, %v", got, err)
	}
	if _, err := uint32WireLength("size", uint64(math.MaxUint32)+1); err == nil {
		t.Fatal("UInt32 overflow unexpectedly accepted")
	}
}
