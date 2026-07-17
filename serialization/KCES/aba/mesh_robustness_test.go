package aba

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

func TestMeshReadIntUInt32MatchesUncheckedCLRInt32Cast(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:], 0x80000000)
	binary.LittleEndian.PutUint32(data[4:], 0xffffffff)
	if got := meshReadInt(data, 0, 10); got != int(int32(-1<<31)) {
		t.Fatalf("UInt32 0x80000000 converted to %d, want Int32.MinValue", got)
	}
	if got := meshReadInt(data, 4, 10); got != -1 {
		t.Fatalf("UInt32 0xffffffff converted to %d, want -1", got)
	}
}

func TestHalfToFloatMatchesAbaExtractorImplementation(t *testing.T) {
	tests := []struct {
		half uint16
		bits uint32
	}{
		{half: 0x0000, bits: 0x00000000},
		{half: 0x8000, bits: 0x80000000},
		// AbaExtractor copies subnormal mantissas directly rather than using a
		// general-purpose IEEE half normalizer.
		{half: 0x0001, bits: 0x00002000},
		{half: 0x03ff, bits: 0x007fe000},
		{half: 0x0400, bits: 0x38800000},
		{half: 0x3c00, bits: 0x3f800000},
		{half: 0x7c00, bits: 0x7f800000},
		{half: 0xfc00, bits: 0xff800000},
		{half: 0x7e00, bits: 0x7fc00000},
	}
	for _, test := range tests {
		if got := math.Float32bits(halfToFloat(test.half)); got != test.bits {
			t.Fatalf("halfToFloat(%#04x) bits = %#08x, want %#08x", test.half, got, test.bits)
		}
	}
}

func TestValidateMeshChannelBounds(t *testing.T) {
	channels := []meshChannel{{Stream: 0, Offset: 0, Format: 0, Dimension: 3}}
	offsets := []uint64{0}
	strides := []uint64{12}
	if err := validateMeshChannelBounds(0, channels, offsets, strides, 2, 24); err != nil {
		t.Fatalf("valid channel bounds rejected: %v", err)
	}
	if err := validateMeshChannelBounds(0, channels, offsets, strides, 2, 23); err == nil || !strings.Contains(err.Error(), "through byte 24") {
		t.Fatalf("truncated channel error = %v", err)
	}

	channels[0].Stream = 1
	if err := validateMeshChannelBounds(0, channels, offsets, strides, 1, 12); err == nil || !strings.Contains(err.Error(), "missing stream") {
		t.Fatalf("missing stream error = %v", err)
	}

	channels[0].Stream = 0
	if err := validateMeshChannelBounds(0, channels, []uint64{math.MaxUint64 - 1}, []uint64{12}, 1, 12); err == nil || !strings.Contains(err.Error(), "overflows UInt64") {
		t.Fatalf("offset overflow error = %v", err)
	}
	if err := validateMeshChannelBounds(0, channels, []uint64{0}, []uint64{math.MaxUint64}, 3, 12); err == nil || !strings.Contains(err.Error(), "overflows UInt64") {
		t.Fatalf("stride multiplication overflow error = %v", err)
	}
}

func TestPackedMeshMessagePackWidthsMatchAbaExtractorKeys(t *testing.T) {
	meshWire, err := ct.EncodeMsgpack(&PackedMesh{})
	if err != nil {
		t.Fatalf("encode PackedMesh: %v", err)
	}
	var meshSlots []codec.Raw
	if err := ct.DecodeMsgpack(meshWire, &meshSlots); err != nil {
		t.Fatalf("decode PackedMesh slots: %v", err)
	}
	if len(meshSlots) != 8 {
		t.Fatalf("PackedMesh width = %d, want Key(0)..Key(7)", len(meshSlots))
	}

	weightWire, err := ct.EncodeMsgpack(&PackedBoneWeights4{})
	if err != nil {
		t.Fatalf("encode PackedBoneWeights4: %v", err)
	}
	var weightSlots []codec.Raw
	if err := ct.DecodeMsgpack(weightWire, &weightSlots); err != nil {
		t.Fatalf("decode PackedBoneWeights4 slots: %v", err)
	}
	if len(weightSlots) != 8 {
		t.Fatalf("PackedBoneWeights4 width = %d, want Key(0)..Key(7)", len(weightSlots))
	}
}

func TestReadBindPoseMissingFieldMatchesAbaExtractorEmptyList(t *testing.T) {
	bindPose, _ := readBindPose(&TypeTreeValue{})
	if bindPose == nil || len(bindPose) != 0 {
		t.Fatalf("missing bind pose = %#v, want non-nil empty list", bindPose)
	}
}
