package KCES

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
)

const colorPresetTestGUID = "12345678-1234-4abc-8def-1234567890ab"

func TestColorPresetTypedRoundTrip(t *testing.T) {
	id := "preset-id"
	baseMenu := "base.menu"
	layerName := "Layer"
	viewName := "View"
	mpnName := "hairf"
	value := &ColorPreset{
		Version:        ColorPresetVersion,
		ID:             &id,
		BaseMenuFile:   &baseMenu,
		UserCreated:    true,
		IsAdvancedMode: true,
		InstanceGUID:   colorPresetStringPointer(colorPresetTestGUID),
		ColorPackList: []*ColorPresetColorPack{
			nil,
			{
				Version:            ColorPresetPackVersion,
				MPNs:               []int32{158},
				LayerName:          &layerName,
				ViewName:           &viewName,
				Type:               ColorPresetPackColorAndAlpha,
				Alpha:              math.Float32frombits(0x80000000),
				AllowedMPNOverride: true,
				MPNNames:           []*string{&mpnName, nil},
				ColorList: []*ColorPresetLayerFreeColor{
					nil,
					{
						Version:     ColorPresetColorVersion,
						BaseColor:   &ColorPresetFreeColor{Version: ColorPresetColorVersion, Hue: math.MinInt32, Saturation: math.MaxInt32},
						ShadowColor: &ColorPresetFreeColor{Version: ColorPresetColorVersion, Brightness: -7, Contrast: 8},
						ShadowRate:  -9,
					},
				},
				GradationColorList: []*ColorPresetGradationColor{
					{
						Version:     ColorPresetColorVersion,
						BaseColor:   &ColorPresetFreeColor{Version: ColorPresetColorVersion},
						ShadowColor: nil,
						ShadowRate:  math.MaxInt32,
						Position:    &ColorPresetControlSlider{Value: math.Float32frombits(0x7fc12345)},
						RangeBefore: &ColorPresetControlSlider{Value: float32(math.Inf(1))},
						RangeAfter:  nil,
					},
				},
			},
		},
	}
	wire, err := EncodeColorPreset(value)
	if err != nil {
		t.Fatalf("EncodeColorPreset() error = %v", err)
	}
	decoded, err := DecodeColorPreset(wire)
	if err != nil {
		t.Fatalf("DecodeColorPreset() error = %v", err)
	}
	roundTripWire, err := EncodeColorPreset(decoded)
	if err != nil {
		t.Fatalf("re-encode decoded ColorPreset: %v", err)
	}
	if !bytes.Equal(roundTripWire, wire) {
		t.Fatalf("round-trip wire changed:\n got  %x\n want %x", roundTripWire, wire)
	}
	if math.Float32bits(decoded.ColorPackList[1].Alpha) != 0x80000000 ||
		math.Float32bits(decoded.ColorPackList[1].GradationColorList[0].Position.Value) != 0x7fc12345 {
		t.Fatal("float32 bit patterns changed")
	}
}

func TestColorPresetUsesTypedNullability(t *testing.T) {
	raw, err := msgpack.EncodeMsgpack([]interface{}{
		int64(ColorPresetVersion),
		nil,
		nil,
		false,
		false,
		[]interface{}{},
		nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeColorPreset(raw)
	if err != nil {
		t.Fatalf("DecodeColorPreset() error = %v", err)
	}
	if decoded.ID != nil || decoded.BaseMenuFile != nil || decoded.InstanceGUID != nil || decoded.ColorPackList == nil {
		t.Fatalf("typed nullability changed: %#v", decoded)
	}
	withGUID, err := DecodeColorPresetWithInstanceGUID(raw, colorPresetTestGUID)
	if err != nil {
		t.Fatalf("DecodeColorPresetWithInstanceGUID() error = %v", err)
	}
	if withGUID.InstanceGUID == nil || *withGUID.InstanceGUID != colorPresetTestGUID {
		t.Fatalf("constructor GUID = %#v", withGUID.InstanceGUID)
	}
}

func TestColorPresetRejectsUnsupportedLayouts(t *testing.T) {
	validRoot := []interface{}{int64(ColorPresetVersion), nil, nil, false, false, []interface{}{}, nil}
	validPack := []interface{}{int64(ColorPresetPackVersion), []interface{}{}, nil, nil, int64(0), []interface{}{}, []interface{}{}, float32(1), false, []interface{}{}}
	tests := []struct {
		name string
		root []interface{}
		want string
	}{
		{name: "short root", root: validRoot[:6], want: "indexed-array width 6, expected 7"},
		{name: "high root slot", root: append(append([]interface{}(nil), validRoot...), nil), want: "indexed-array width 8, expected 7"},
		{name: "nil version", root: []interface{}{nil, nil, nil, false, false, []interface{}{}, nil}, want: "version"},
		{name: "short pack", root: []interface{}{int64(ColorPresetVersion), nil, nil, false, false, []interface{}{validPack[:9]}, nil}, want: "indexed-array width 9, expected 10"},
		{name: "high pack slot", root: []interface{}{int64(ColorPresetVersion), nil, nil, false, false, []interface{}{append(append([]interface{}(nil), validPack...), nil)}, nil}, want: "indexed-array width 11, expected 10"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := msgpack.EncodeMsgpack(test.root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeColorPreset(raw); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeColorPreset() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestColorPresetRejectsTrailingData(t *testing.T) {
	value, err := NewColorPreset(colorPresetTestGUID)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := EncodeColorPreset(value)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := msgpack.DecompressLz4BlockArray(wire)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, 0xc0)
	if _, err := DecodeColorPreset(raw); err == nil || !strings.Contains(strings.ToLower(err.Error()), "trailing") {
		t.Fatalf("DecodeColorPreset() error = %v, want trailing data", err)
	}
}

func colorPresetStringPointer(value string) *string { return &value }
