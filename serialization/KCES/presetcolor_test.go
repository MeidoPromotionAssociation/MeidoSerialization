package KCES

import (
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

func TestPresetColorRoundTripPreservesKCES2Layout(t *testing.T) {
	value, err := NewKCES2ColorPreset(colorPresetTestGUID)
	if err != nil {
		t.Fatalf("NewKCES2ColorPreset: %v", err)
	}
	label := "shared hair color"
	value.SaveLocationHash = ^uint64(0)
	value.CreationTicks = -123456789
	value.LastUpdateTicks = 987654321
	value.MetaTexts = map[string]*string{"label": &label, "nullable": nil}

	wire, err := EncodePresetColor(value)
	if err != nil {
		t.Fatalf("EncodePresetColor: %v", err)
	}
	raw, err := msgpack.DecompressLz4BlockArray(wire)
	if err != nil {
		t.Fatalf("DecompressLz4BlockArray: %v", err)
	}
	if got := rawArrayWidth(t, raw); got != 11 {
		t.Fatalf("PresetColor width = %d, want 11", got)
	}

	decoded, err := DecodePresetColor(wire)
	if err != nil {
		t.Fatalf("DecodePresetColor: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("PresetColor round trip mismatch\n got: %#v\nwant: %#v", decoded, value)
	}
	reencoded, err := EncodePresetColor(decoded)
	if err != nil {
		t.Fatalf("re-encode PresetColor: %v", err)
	}
	raw, err = msgpack.DecompressLz4BlockArray(reencoded)
	if err != nil {
		t.Fatalf("decompress re-encoded PresetColor: %v", err)
	}
	if got := rawArrayWidth(t, raw); got != 11 {
		t.Fatalf("re-encoded PresetColor width = %d, want 11", got)
	}
}

func TestPresetColorLegacyLayoutRejectsKCES2TailFields(t *testing.T) {
	value, err := NewColorPreset(colorPresetTestGUID)
	if err != nil {
		t.Fatalf("NewColorPreset: %v", err)
	}
	value.SaveLocationHash = 1
	if _, err := EncodeColorPreset(value); err == nil || !strings.Contains(err.Error(), "not representable") {
		t.Fatalf("EncodeColorPreset error = %v, want not-representable error", err)
	}
}

func TestPresetColorRoundTripPreservesLegacyWidths(t *testing.T) {
	legacy := &ColorPreset{
		Version:                   1003,
		ColorPackList:             make([]*ColorPresetColorPack, 0),
		IndexedArrayWidth:         6,
		LegacyInstanceGUIDOmitted: true,
	}
	kces, err := NewColorPreset(colorPresetTestGUID)
	if err != nil {
		t.Fatalf("NewColorPreset: %v", err)
	}
	kces.Version = 777

	tests := []struct {
		name  string
		value *ColorPreset
		width int
	}{
		{name: "six-slot", value: legacy, width: 6},
		{name: "seven-slot", value: kces, width: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire, err := EncodePresetColor(test.value)
			if err != nil {
				t.Fatalf("EncodePresetColor: %v", err)
			}
			raw, err := msgpack.DecompressLz4BlockArray(wire)
			if err != nil {
				t.Fatalf("DecompressLz4BlockArray: %v", err)
			}
			if got := rawArrayWidth(t, raw); got != test.width {
				t.Fatalf("encoded PresetColor width = %d, want %d", got, test.width)
			}

			decoded, err := DecodePresetColor(wire)
			if err != nil {
				t.Fatalf("DecodePresetColor: %v", err)
			}
			if decoded.IndexedArrayWidth != int32(test.width) {
				t.Fatalf("decoded PresetColor width = %d, want %d", decoded.IndexedArrayWidth, test.width)
			}
			if decoded.Version != test.value.Version {
				t.Fatalf("decoded PresetColor version = %d, want %d", decoded.Version, test.value.Version)
			}

			reencoded, err := EncodePresetColor(decoded)
			if err != nil {
				t.Fatalf("re-encode PresetColor: %v", err)
			}
			raw, err = msgpack.DecompressLz4BlockArray(reencoded)
			if err != nil {
				t.Fatalf("decompress re-encoded PresetColor: %v", err)
			}
			if got := rawArrayWidth(t, raw); got != test.width {
				t.Fatalf("re-encoded PresetColor width = %d, want %d", got, test.width)
			}
		})
	}
}
