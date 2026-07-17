package KCES

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

const colorPresetTestGUID = "12345678-1234-4abc-8def-1234567890ab"

func TestColorPresetHandWrittenCurrentWireRoundTrip(t *testing.T) {
	// These are the exact indexed-object shapes from KCES 1.34.4. The raw
	// MessagePack is deliberately assembled here without ct.EncodeMsgpack so a
	// matching mistake in a struct codec cannot make this test pass.
	raw := colorPresetTestAppendArray(nil, 7)
	raw = colorPresetTestAppendInt32(raw, ColorPresetVersion)
	raw = colorPresetTestAppendString(raw, "preset-id")
	raw = colorPresetTestAppendString(raw, "base.menu")
	raw = append(raw, 0xc3, 0xc2) // userCreated, private isAdvancedMode_
	raw = colorPresetTestAppendArray(raw, 1)
	raw = colorPresetTestAppendArray(raw, 10)
	raw = colorPresetTestAppendInt32(raw, ColorPresetPackVersion)
	raw = colorPresetTestAppendArray(raw, 1)
	raw = colorPresetTestAppendInt32(raw, 158) // MPN.hairf in KCES 1.34.4
	raw = colorPresetTestAppendString(raw, "Layer")
	raw = append(raw, 0xc0) // nullable viewName
	raw = colorPresetTestAppendInt32(raw, int(ColorPresetPackColorAndAlpha))
	raw = colorPresetTestAppendArray(raw, 1)
	raw = colorPresetTestAppendLayerColor(raw, 1, 2, -3, 4, 5, 6, -7, 8, 9)
	raw = colorPresetTestAppendArray(raw, 1)
	raw = colorPresetTestAppendGradationColor(raw)
	raw = colorPresetTestAppendFloat32(raw, math.Float32frombits(0x80000000))
	raw = append(raw, 0xc3) // private allowedMpnOverRide
	raw = colorPresetTestAppendArray(raw, 1)
	raw = colorPresetTestAppendString(raw, "hairf") // private mpnNames
	raw = colorPresetTestAppendString(raw, colorPresetTestGUID)

	wire := colorPresetTestCompress(t, raw)
	got, err := DecodeColorPreset(wire)
	if err != nil {
		t.Fatalf("DecodeColorPreset(hand-written wire) error = %v", err)
	}
	if got.Version != ColorPresetVersion || got.ID == nil || *got.ID != "preset-id" || got.BaseMenuFile == nil || *got.BaseMenuFile != "base.menu" {
		t.Fatalf("root fields = %#v", got)
	}
	if !got.UserCreated || got.IsAdvancedMode || got.InstanceGUID != colorPresetTestGUID || len(got.ColorPackList) != 1 {
		t.Fatalf("root callback/private fields = %#v", got)
	}
	pack := got.ColorPackList[0]
	if !reflect.DeepEqual(pack.MPNs, []int{158}) || !reflect.DeepEqual(pack.MPNNames, []string{"hairf"}) || !pack.AllowedMPNOverride {
		t.Fatalf("pack MPN/private fields = %#v", pack)
	}
	if pack.ViewName != nil || len(pack.ColorList) != 1 || len(pack.GradationColorList) != 1 {
		t.Fatalf("pack nullable/color fields = %#v", pack)
	}
	if math.Float32bits(pack.Alpha) != 0x80000000 {
		t.Fatalf("alpha bits = %08x, want negative zero", math.Float32bits(pack.Alpha))
	}
	grad := pack.GradationColorList[0]
	if grad.Position == nil || grad.RangeBefore == nil || grad.RangeAfter == nil || grad.Position.Value != 0.25 || grad.RangeBefore.Value != 0.5 || grad.RangeAfter.Value != 0.75 {
		t.Fatalf("gradation private ControlSlider values = %#v", grad)
	}

	encoded, err := EncodeColorPreset(got)
	if err != nil {
		t.Fatalf("EncodeColorPreset(decoded) error = %v", err)
	}
	encodedRaw, err := ct.DecompressLz4BlockArray(encoded)
	if err != nil {
		t.Fatalf("decompress encoded preset: %v", err)
	}
	if !bytes.Equal(encodedRaw, raw) {
		t.Fatalf("canonical round trip\n got  %x\n want %x", encodedRaw, raw)
	}
}

func TestColorPresetPrivateLz4BlockArrayThreshold(t *testing.T) {
	tests := []struct {
		idLength       int
		wantRawLength  int
		wantCompressed bool
	}{
		{idLength: 16, wantRawLength: 63, wantCompressed: false},
		{idLength: 17, wantRawLength: 64, wantCompressed: true},
	}
	for _, test := range tests {
		t.Run(strings.Repeat("x", test.idLength), func(t *testing.T) {
			id := strings.Repeat("x", test.idLength)
			value := &ColorPreset{
				Version:       1003,
				ID:            &id,
				ColorPackList: []*ColorPresetColorPack{},
				InstanceGUID:  colorPresetTestGUID,
			}
			before := cloneColorPresetTestValue(value)
			wire, err := EncodeColorPreset(value)
			if err != nil {
				t.Fatalf("EncodeColorPreset() error = %v", err)
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatalf("encoder modified caller: got %#v, want %#v", value, before)
			}
			raw, err := ct.DecompressLz4BlockArray(wire)
			if err != nil {
				t.Fatalf("DecompressLz4BlockArray() error = %v", err)
			}
			if len(raw) != test.wantRawLength {
				t.Fatalf("raw length = %d, want %d; raw=%x", len(raw), test.wantRawLength, raw)
			}
			if test.wantCompressed {
				if bytes.Equal(wire, raw) || len(wire) < 3 || wire[0] != 0x92 || wire[1] != 0xd4 || wire[2] != byte(ct.Lz4ArrayType) {
					t.Fatalf("64-byte wire is not direct Lz4BlockArray: %x", wire)
				}
			} else if !bytes.Equal(wire, raw) {
				t.Fatalf("63-byte wire was compressed: %x", wire)
			}
		})
	}
}

func TestColorPresetVersionsAndMPNFieldsArePreserved(t *testing.T) {
	for _, version := range []int{-1, 0, 1002, 1003, ColorPresetVersion, 1005} {
		t.Run("root_version_"+fmt.Sprint(version), func(t *testing.T) {
			got, err := DecodeColorPreset(colorPresetTestMinimalRoot(version, colorPresetTestGUID, nil))
			if err != nil {
				t.Fatalf("DecodeColorPreset(version=%d): %v", version, err)
			}
			if got.Version != version {
				t.Fatalf("decoded version = %d, want %d", got.Version, version)
			}
			wire, err := EncodeColorPreset(got)
			if err != nil {
				t.Fatalf("EncodeColorPreset(version=%d): %v", version, err)
			}
			roundTrip, err := DecodeColorPreset(wire)
			if err != nil || roundTrip.Version != version {
				t.Fatalf("version round trip = %#v, %v; want %d", roundTrip, err, version)
			}
		})
	}

	t.Run("numeric and name representations remain independent", func(t *testing.T) {
		layer := "Layer"
		value := &ColorPreset{
			Version: 1003,
			ColorPackList: []*ColorPresetColorPack{{
				Version:            -1,
				MPNs:               []int{158, -1, 164},
				LayerName:          &layer,
				Type:               ColorPresetPackOnlyAlpha,
				ColorList:          []*ColorPresetLayerFreeColor{},
				GradationColorList: []*ColorPresetGradationColor{},
				Alpha:              0.5,
				AllowedMPNOverride: true,
				MPNNames:           []string{"Hairf", "158", "body,hairf", "not_an_mpn", "stale"},
			}},
			InstanceGUID: colorPresetTestGUID,
		}
		before := cloneColorPresetTestValue(value)
		wire, err := EncodeColorPreset(value)
		if err != nil {
			t.Fatalf("EncodeColorPreset: %v", err)
		}
		if !reflect.DeepEqual(value, before) {
			t.Fatalf("encoder modified caller\n got  %#v\n want %#v", value, before)
		}
		got, err := DecodeColorPreset(wire)
		if err != nil {
			t.Fatalf("DecodeColorPreset: %v", err)
		}
		if got.Version != value.Version || len(got.ColorPackList) != 1 {
			t.Fatalf("root/pack result = %#v", got)
		}
		pack := got.ColorPackList[0]
		if pack.Version != -1 || !reflect.DeepEqual(pack.MPNs, value.ColorPackList[0].MPNs) ||
			!reflect.DeepEqual(pack.MPNNames, value.ColorPackList[0].MPNNames) || !pack.AllowedMPNOverride {
			t.Fatalf("wire MPN/version fields changed: %#v", pack)
		}
	})

	t.Run("hand-written mismatched fields are not migrated", func(t *testing.T) {
		pack := colorPresetTestMinimalPack([]int{158}, []string{"skin"})
		got, err := DecodeColorPreset(colorPresetTestMinimalRoot(1002, colorPresetTestGUID, pack))
		if err != nil {
			t.Fatalf("DecodeColorPreset: %v", err)
		}
		if !reflect.DeepEqual(got.ColorPackList[0].MPNs, []int{158}) ||
			!reflect.DeepEqual(got.ColorPackList[0].MPNNames, []string{"skin"}) {
			t.Fatalf("decoder applied MPN callback: %#v", got.ColorPackList[0])
		}
	})
}

func TestColorPresetMissingGUIDDoesNotGenerateImplicitIdentity(t *testing.T) {
	t.Run("ordinary decode keeps short root zero values", func(t *testing.T) {
		got, err := DecodeColorPreset([]byte{0x90})
		if err != nil {
			t.Fatalf("DecodeColorPreset(short root): %v", err)
		}
		if got.Version != 0 || got.ColorPackList != nil || got.InstanceGUID != "" {
			t.Fatalf("short root gained constructor defaults: %#v", got)
		}
	})

	t.Run("explicit constructor GUID only supplies missing identity", func(t *testing.T) {
		first, err := DecodeColorPresetWithInstanceGUID([]byte{0x90}, colorPresetTestGUID)
		if err != nil {
			t.Fatalf("DecodeColorPresetWithInstanceGUID() error = %v", err)
		}
		second, err := DecodeColorPresetWithInstanceGUID([]byte{0x90}, colorPresetTestGUID)
		if err != nil || !reflect.DeepEqual(first, second) {
			t.Fatalf("explicit default is not deterministic: first=%#v second=%#v err=%v", first, second, err)
		}
		if first.Version != 0 || first.InstanceGUID != colorPresetTestGUID || first.ColorPackList != nil {
			t.Fatalf("explicit identity changed unrelated fields: %#v", first)
		}
	})

	t.Run("constructor is explicit but encoder accepts empty identity", func(t *testing.T) {
		if _, err := NewColorPreset("not-a-guid"); err == nil {
			t.Fatal("NewColorPreset accepted invalid GUID")
		}
		created, err := NewColorPreset(colorPresetTestGUID)
		if err != nil || created.Version != ColorPresetVersion || created.ColorPackList == nil {
			t.Fatalf("NewColorPreset() = %#v, %v", created, err)
		}
		wire, err := EncodeColorPreset(&ColorPreset{ColorPackList: []*ColorPresetColorPack{}})
		if err != nil {
			t.Fatalf("EncodeColorPreset(empty instance id): %v", err)
		}
		got, err := DecodeColorPreset(wire)
		if err != nil || got.InstanceGUID != "" {
			t.Fatalf("empty instance id round trip = %#v, %v", got, err)
		}
	})

	t.Run("non-empty wire instance identifier is not parsed as a GUID", func(t *testing.T) {
		const instanceID = "legacy-user-instance"
		value, err := DecodeColorPreset(colorPresetTestMinimalRoot(ColorPresetVersion, instanceID, nil))
		if err != nil {
			t.Fatalf("DecodeColorPreset(non-GUID instance id) error = %v", err)
		}
		if value.InstanceGUID != instanceID {
			t.Fatalf("instanceGuid = %q, want %q", value.InstanceGUID, instanceID)
		}
		wire, err := EncodeColorPreset(value)
		if err != nil {
			t.Fatalf("EncodeColorPreset(non-GUID instance id) error = %v", err)
		}
		got, err := DecodeColorPreset(wire)
		if err != nil || got.InstanceGUID != instanceID {
			t.Fatalf("non-GUID instance id round trip = %#v, %v", got, err)
		}
	})

	t.Run("ColorPresetSlot uses the identical base wire", func(t *testing.T) {
		value, err := NewColorPresetSlot(colorPresetTestGUID)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := EncodeColorPresetSlot(value)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeColorPresetSlot(wire)
		if err != nil || !reflect.DeepEqual(got, value) {
			t.Fatalf("ColorPresetSlot round trip = %#v, %v; want %#v", got, err, value)
		}
	})
}

func TestColorPresetNullableShortAndFutureSlots(t *testing.T) {
	t.Run("root and pack future slots are consumed and preserved", func(t *testing.T) {
		pack := colorPresetTestMinimalPack([]int{158}, []string{"hairf"})
		pack[0] = 0x9b // array(11)
		pack = append(pack, 0x82, 0xa1, 'a', 0x91, 0xc0, 0xa1, 'b', 0xd6, 0x2a, 0, 0, 0, 7)
		raw := colorPresetTestMinimalRoot(ColorPresetVersion, colorPresetTestGUID, pack)
		raw[0] = 0x98 // array(8)
		raw = append(raw, 0x81, 0xa1, 'x', 0x92, 0xc0, 0xc3)
		got, err := DecodeColorPreset(colorPresetTestCompress(t, raw))
		if err != nil {
			t.Fatalf("DecodeColorPreset(future slots) error = %v", err)
		}
		if len(got.ColorPackList) != 1 || !reflect.DeepEqual(got.ColorPackList[0].MPNs, []int{158}) {
			t.Fatalf("future-slot decode = %#v", got)
		}
		if got.FieldCount == nil || *got.FieldCount != 8 || len(got.FutureSlots) != 1 || got.ColorPackList[0].FieldCount == nil || *got.ColorPackList[0].FieldCount != 11 || len(got.ColorPackList[0].FutureSlots) != 1 {
			t.Fatalf("future indexed-object shape was lost: %#v", got)
		}
		reencoded, err := EncodeColorPreset(got)
		if err != nil {
			t.Fatal(err)
		}
		reencoded, err = ct.DecompressLz4BlockArray(reencoded)
		if err != nil || !bytes.Equal(reencoded, raw) {
			t.Fatalf("future slots changed during round-trip: equal=%v err=%v", bytes.Equal(reencoded, raw), err)
		}
	})

	t.Run("nested short arrays retain wire zero values", func(t *testing.T) {
		pack := colorPresetTestPackWithNestedShortDefaults()
		got, err := DecodeColorPreset(colorPresetTestMinimalRoot(ColorPresetVersion, colorPresetTestGUID, pack))
		if err != nil {
			t.Fatalf("DecodeColorPreset(short nested objects) error = %v", err)
		}
		decodedPack := got.ColorPackList[0]
		if len(decodedPack.ColorList) != 1 || len(decodedPack.GradationColorList) != 1 {
			t.Fatalf("nested lists = %#v", decodedPack)
		}
		layer := decodedPack.ColorList[0]
		if layer.Version != 0 || layer.BaseColor != nil || layer.ShadowColor != nil || layer.ShadowRate != 0 {
			t.Fatalf("short LayerFreeColor gained constructor defaults: %#v", layer)
		}
		if layer.FieldCount == nil || *layer.FieldCount != 0 {
			t.Fatalf("short LayerFreeColor width was lost: %#v", layer)
		}
		grad := decodedPack.GradationColorList[0]
		if grad.Version != 0 || grad.BaseColor != nil || grad.ShadowColor != nil || grad.Position != nil || grad.RangeBefore != nil || grad.RangeAfter != nil {
			t.Fatalf("short GradationColor gained constructor defaults: %#v", grad)
		}
		if grad.FieldCount == nil || *grad.FieldCount != 0 {
			t.Fatalf("short GradationColor width was lost: %#v", grad)
		}
		reencoded, err := EncodeColorPreset(got)
		if err != nil {
			t.Fatal(err)
		}
		reencoded, err = ct.DecompressLz4BlockArray(reencoded)
		want := colorPresetTestMinimalRoot(ColorPresetVersion, colorPresetTestGUID, pack)
		if err != nil || !bytes.Equal(reencoded, want) {
			t.Fatalf("short nested widths changed: equal=%v err=%v", bytes.Equal(reencoded, want), err)
		}
	})

	t.Run("nullable collections remain nil", func(t *testing.T) {
		pack := colorPresetTestPackWithNilMPNs()
		got, err := DecodeColorPreset(colorPresetTestMinimalRoot(ColorPresetVersion, colorPresetTestGUID, pack))
		if err != nil {
			t.Fatalf("DecodeColorPreset(nil mpns, empty names) error = %v", err)
		}
		if got.ColorPackList[0].MPNs != nil || got.ColorPackList[0].MPNNames == nil {
			t.Fatalf("nil/empty collection distinction changed: %#v", got.ColorPackList[0])
		}

		value, err := NewColorPreset(colorPresetTestGUID)
		if err != nil {
			t.Fatal(err)
		}
		value.ColorPackList = []*ColorPresetColorPack{{
			Version:            ColorPresetPackVersion,
			MPNs:               nil,
			ColorList:          []*ColorPresetLayerFreeColor{},
			GradationColorList: []*ColorPresetGradationColor{},
		}}
		wire, err := EncodeColorPreset(value)
		if err != nil {
			t.Fatalf("EncodeColorPreset(nil mpns) error = %v", err)
		}
		roundTrip, err := DecodeColorPreset(wire)
		if err != nil {
			t.Fatalf("round trip for nil collections failed: %v", err)
		}
		if roundTrip.ColorPackList[0].MPNs != nil || roundTrip.ColorPackList[0].MPNNames != nil {
			t.Fatalf("nil pack collections became non-nil: %#v", roundTrip.ColorPackList[0])
		}
	})

	t.Run("nil pack list and missing final pack field are accepted", func(t *testing.T) {
		got, err := DecodeColorPreset(colorPresetTestRootWithPackListMarker(0xc0))
		if err != nil || got.ColorPackList != nil {
			t.Fatalf("nil pack list = %#v, %v", got, err)
		}
		short, err := DecodeColorPreset(colorPresetTestMinimalRoot(ColorPresetVersion, colorPresetTestGUID, colorPresetTestMinimalPackShort()))
		if err != nil || len(short.ColorPackList) != 1 || short.ColorPackList[0].MPNNames != nil {
			t.Fatalf("short pack = %#v, %v", short, err)
		}
		if short.ColorPackList[0].FieldCount == nil || *short.ColorPackList[0].FieldCount != 9 {
			t.Fatalf("short pack width was lost: %#v", short.ColorPackList[0])
		}
	})

	t.Run("nil instance GUID carrier is distinct from empty string", func(t *testing.T) {
		raw := colorPresetTestAppendArray(nil, 7)
		raw = colorPresetTestAppendInt32(raw, ColorPresetVersion)
		raw = append(raw, 0xc0, 0xc0, 0xc2, 0xc2, 0x90, 0xc0)
		got, err := DecodeColorPreset(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !got.InstanceGUIDIsNil || got.InstanceGUID != "" {
			t.Fatalf("nil GUID carrier changed: %#v", got)
		}
		withConstructor, err := DecodeColorPresetWithInstanceGUID(raw, colorPresetTestGUID)
		if err != nil {
			t.Fatal(err)
		}
		if !withConstructor.InstanceGUIDIsNil || withConstructor.InstanceGUID != colorPresetTestGUID {
			t.Fatalf("explicit constructor view lost nil carrier: %#v", withConstructor)
		}
		for _, value := range []*ColorPreset{got, withConstructor} {
			reencoded, err := EncodeColorPreset(value)
			if err != nil || !bytes.Equal(reencoded, raw) {
				t.Fatalf("nil GUID carrier changed on encode: equal=%v err=%v", bytes.Equal(reencoded, raw), err)
			}
		}
	})
}

func TestColorPresetInt32BoundariesAndFloatBits(t *testing.T) {
	base := &ColorPresetFreeColor{
		Version:    -999,
		Hue:        math.MinInt32,
		Saturation: math.MaxInt32,
		Brightness: math.MinInt32,
		Contrast:   math.MaxInt32,
	}
	shadow := &ColorPresetFreeColor{Version: 9999}
	layer := &ColorPresetLayerFreeColor{
		Version:     -1,
		BaseColor:   base,
		ShadowColor: shadow,
		ShadowRate:  math.MinInt32,
	}
	grad := &ColorPresetGradationColor{
		Version:     -1,
		BaseColor:   base,
		ShadowColor: shadow,
		ShadowRate:  math.MaxInt32,
		Position:    &ColorPresetControlSlider{Value: math.Float32frombits(0x7fc12345)},
		RangeBefore: &ColorPresetControlSlider{Value: float32(math.Inf(1))},
		RangeAfter:  &ColorPresetControlSlider{Value: math.Float32frombits(0x80000000)},
	}
	value, err := NewColorPreset(colorPresetTestGUID)
	if err != nil {
		t.Fatal(err)
	}
	value.ColorPackList = []*ColorPresetColorPack{{
		Version:            -1,
		MPNs:               []int{158},
		Type:               ColorPresetPackColorAndAlpha,
		ColorList:          []*ColorPresetLayerFreeColor{layer},
		GradationColorList: []*ColorPresetGradationColor{grad},
		Alpha:              math.Float32frombits(0xffc54321),
	}}
	wire, err := EncodeColorPreset(value)
	if err != nil {
		t.Fatalf("EncodeColorPreset(Int32/float boundaries) error = %v", err)
	}
	got, err := DecodeColorPreset(wire)
	if err != nil {
		t.Fatalf("DecodeColorPreset(Int32/float boundaries) error = %v", err)
	}
	gotPack := got.ColorPackList[0]
	if gotPack.ColorList[0].BaseColor.Hue != math.MinInt32 || gotPack.ColorList[0].BaseColor.Saturation != math.MaxInt32 || gotPack.ColorList[0].ShadowRate != math.MinInt32 || gotPack.GradationColorList[0].ShadowRate != math.MaxInt32 {
		t.Fatalf("Int32 boundary values changed: %#v", gotPack)
	}
	if math.Float32bits(gotPack.Alpha) != 0xffc54321 || math.Float32bits(gotPack.GradationColorList[0].Position.Value) != 0x7fc12345 || !math.IsInf(float64(gotPack.GradationColorList[0].RangeBefore.Value), 1) || math.Float32bits(gotPack.GradationColorList[0].RangeAfter.Value) != 0x80000000 {
		t.Fatalf("private IEEE-754 bits changed: alpha=%08x grad=%#v", math.Float32bits(gotPack.Alpha), gotPack.GradationColorList[0])
	}

	if strconvIntSizeForColorPresetTest() > 32 {
		overflow := int(int64(math.MaxInt32) + 1)
		value.ColorPackList[0].ColorList[0].BaseColor.Hue = overflow
		if _, err := EncodeColorPreset(value); err == nil || !strings.Contains(err.Error(), "Int32") {
			t.Fatalf("EncodeColorPreset(Int32 overflow) error = %v", err)
		}
	}
}

func TestColorPresetPreservesEveryNestedFutureSlot(t *testing.T) {
	rootFields, packFields, layerFields, freeFields, gradFields, sliderFields := 8, 11, 5, 6, 8, 2
	base := &ColorPresetFreeColor{
		Version:     1000,
		Hue:         1,
		Saturation:  2,
		Brightness:  3,
		Contrast:    4,
		FieldCount:  &freeFields,
		FutureSlots: [][]byte{{0x82, 0xa1, 'x', 0x91, 0xc0, 0xa1, 'y', 0xc3}},
	}
	layer := &ColorPresetLayerFreeColor{
		Version:     1000,
		BaseColor:   base,
		ShadowRate:  5,
		FieldCount:  &layerFields,
		FutureSlots: [][]byte{{0xd4, 0x2a, 0xff}},
	}
	slider := &ColorPresetControlSlider{
		Value:       0.25,
		FieldCount:  &sliderFields,
		FutureSlots: [][]byte{{0x92, 0x01, 0x02}},
	}
	gradation := &ColorPresetGradationColor{
		Version:     1000,
		BaseColor:   base,
		ShadowRate:  6,
		Position:    slider,
		FieldCount:  &gradFields,
		FutureSlots: [][]byte{{0xc7, 0x01, 0x03, 0x7f}},
	}
	pack := &ColorPresetColorPack{
		Version:            1001,
		MPNs:               []int{158},
		ColorList:          []*ColorPresetLayerFreeColor{layer},
		GradationColorList: []*ColorPresetGradationColor{gradation},
		MPNNames:           []string{"hairf"},
		FieldCount:         &packFields,
		FutureSlots:        [][]byte{{0x81, 0xa1, 'p', 0xc2}},
	}
	value := &ColorPreset{
		Version:       1004,
		ColorPackList: []*ColorPresetColorPack{pack},
		InstanceGUID:  colorPresetTestGUID,
		FieldCount:    &rootFields,
		FutureSlots:   [][]byte{{0x91, 0xd6, 0x04, 0, 0, 0, 9}},
	}
	wire, err := EncodeColorPreset(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeColorPreset(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("nested future slots changed:\ngot =%#v\nwant=%#v", decoded, value)
	}
	reencoded, err := EncodeColorPreset(decoded)
	if err != nil || !bytes.Equal(reencoded, wire) {
		t.Fatalf("nested future-slot wire changed: equal=%v err=%v", bytes.Equal(reencoded, wire), err)
	}
}

func TestColorPresetReadSingleAndCanonicalFloat32(t *testing.T) {
	pack := colorPresetTestMinimalPack([]int{158}, []string{"hairf"})
	// Replace the canonical alpha float32 at the known final three fields with
	// an integer accepted by MessagePackReader.ReadSingle.
	const trailingAllowedAndNames = 1 + 1 + 1 + len("hairf")
	alphaOffset := len(pack) - trailingAllowedAndNames - 5
	integerAlpha := append([]byte(nil), pack[:alphaOffset]...)
	integerAlpha = append(integerAlpha, 0x01)
	pack = append(integerAlpha, pack[alphaOffset+5:]...)
	value, err := DecodeColorPreset(colorPresetTestMinimalRoot(ColorPresetVersion, colorPresetTestGUID, pack))
	if err != nil {
		t.Fatalf("DecodeColorPreset(integer Single) error = %v", err)
	}
	if value.ColorPackList[0].Alpha != 1 {
		t.Fatalf("integer Single decoded as %v", value.ColorPackList[0].Alpha)
	}
	wire, err := EncodeColorPreset(value)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ct.DecompressLz4BlockArray(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte{0xca, 0x3f, 0x80, 0x00, 0x00}) {
		t.Fatalf("canonical output does not contain float32 1.0: %x", raw)
	}
}

func TestColorPresetRejectsMalformedAndUnsafeWire(t *testing.T) {
	valid := colorPresetTestMinimalRoot(ColorPresetVersion, colorPresetTestGUID, nil)
	tests := map[string][]byte{
		"map root":       {0x80},
		"truncated root": append([]byte(nil), valid[:len(valid)-1]...),
	}
	for name, wire := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeColorPreset(wire); err == nil {
				t.Fatalf("DecodeColorPreset(%s) unexpectedly succeeded", name)
			}
		})
	}

	t.Run("Int32 overflow", func(t *testing.T) {
		raw := colorPresetTestMinimalRoot(ColorPresetVersion, colorPresetTestGUID, nil)
		raw[1] = 0xcf
		raw = append(raw[:2], append([]byte{0, 0, 0, 0, 0x80, 0, 0, 0}, raw[4:]...)...)
		if _, err := DecodeColorPreset(raw); err == nil || !strings.Contains(err.Error(), "Int32") {
			t.Fatalf("DecodeColorPreset(Int32 overflow) error = %v", err)
		}
	})

	t.Run("declared collection bomb", func(t *testing.T) {
		raw := colorPresetTestAppendArray(nil, 7)
		raw = colorPresetTestAppendInt32(raw, ColorPresetVersion)
		raw = append(raw, 0xc0, 0xc0, 0xc2, 0xc2, 0xdd, 0xff, 0xff, 0xff, 0xff)
		if _, err := DecodeColorPreset(raw); err == nil {
			t.Fatal("DecodeColorPreset accepted array32 collection bomb")
		}
	})
}

func colorPresetTestMinimalRoot(version int, guid string, pack []byte) []byte {
	raw := colorPresetTestAppendArray(nil, 7)
	raw = colorPresetTestAppendInt32(raw, version)
	raw = append(raw, 0xc0, 0xc0, 0xc2, 0xc2)
	if pack == nil {
		raw = colorPresetTestAppendArray(raw, 0)
	} else {
		raw = colorPresetTestAppendArray(raw, 1)
		raw = append(raw, pack...)
	}
	return colorPresetTestAppendString(raw, guid)
}

func colorPresetTestRootWithPackListMarker(marker byte) []byte {
	raw := colorPresetTestAppendArray(nil, 7)
	raw = colorPresetTestAppendInt32(raw, ColorPresetVersion)
	raw = append(raw, 0xc0, 0xc0, 0xc2, 0xc2, marker)
	return colorPresetTestAppendString(raw, colorPresetTestGUID)
}

func colorPresetTestMinimalPack(mpns []int, names []string) []byte {
	raw := colorPresetTestAppendArray(nil, 10)
	raw = colorPresetTestAppendInt32(raw, ColorPresetPackVersion)
	raw = colorPresetTestAppendArray(raw, len(mpns))
	for _, mpn := range mpns {
		raw = colorPresetTestAppendInt32(raw, mpn)
	}
	raw = append(raw, 0xc0, 0xc0)
	raw = colorPresetTestAppendInt32(raw, int(ColorPresetPackOnlyAlpha))
	raw = colorPresetTestAppendArray(raw, 0)
	raw = colorPresetTestAppendArray(raw, 0)
	raw = colorPresetTestAppendFloat32(raw, 0)
	raw = append(raw, 0xc2)
	raw = colorPresetTestAppendArray(raw, len(names))
	for _, name := range names {
		raw = colorPresetTestAppendString(raw, name)
	}
	return raw
}

func colorPresetTestMinimalPackVersion(version int) []byte {
	raw := colorPresetTestMinimalPack([]int{158}, []string{"hairf"})
	// array(10) + uint16 version; all test versions use the same three-byte form.
	binary.BigEndian.PutUint16(raw[2:4], uint16(version))
	return raw
}

func colorPresetTestMinimalPackShort() []byte {
	raw := colorPresetTestMinimalPack([]int{158}, []string{"hairf"})
	// Remove Key(9) and change array(10) to array(9).
	nameWireLength := 1 + 1 + len("hairf")
	raw = raw[:len(raw)-nameWireLength]
	raw[0] = 0x99
	return raw
}

func colorPresetTestPackWithNestedShortDefaults() []byte {
	raw := colorPresetTestAppendArray(nil, 10)
	raw = colorPresetTestAppendInt32(raw, ColorPresetPackVersion)
	raw = colorPresetTestAppendArray(raw, 1)
	raw = colorPresetTestAppendInt32(raw, 158)
	raw = append(raw, 0xc0, 0xc0)
	raw = colorPresetTestAppendInt32(raw, int(ColorPresetPackColorAndAlpha))
	raw = colorPresetTestAppendArray(raw, 1)
	raw = colorPresetTestAppendArray(raw, 0) // LayerFreeColor constructor defaults
	raw = colorPresetTestAppendArray(raw, 1)
	raw = colorPresetTestAppendArray(raw, 0) // GradationColor constructor defaults
	raw = colorPresetTestAppendFloat32(raw, 0)
	raw = append(raw, 0xc2)
	raw = colorPresetTestAppendArray(raw, 1)
	return colorPresetTestAppendString(raw, "hairf")
}

func colorPresetTestPackWithNilMPNs() []byte {
	raw := colorPresetTestAppendArray(nil, 10)
	raw = colorPresetTestAppendInt32(raw, ColorPresetPackVersion)
	raw = append(raw, 0xc0, 0xc0, 0xc0) // mpns, layerName, viewName
	raw = colorPresetTestAppendInt32(raw, int(ColorPresetPackColorAndAlpha))
	raw = colorPresetTestAppendArray(raw, 0)
	raw = colorPresetTestAppendArray(raw, 0)
	raw = colorPresetTestAppendFloat32(raw, 0)
	raw = append(raw, 0xc2)
	return colorPresetTestAppendArray(raw, 0)
}

func colorPresetTestAppendLayerColor(dst []byte, values ...int) []byte {
	dst = colorPresetTestAppendArray(dst, 4)
	dst = colorPresetTestAppendInt32(dst, ColorPresetColorVersion)
	dst = colorPresetTestAppendFreeColor(dst, values[0:4]...)
	dst = colorPresetTestAppendFreeColor(dst, values[4:8]...)
	return colorPresetTestAppendInt32(dst, values[8])
}

func colorPresetTestAppendFreeColor(dst []byte, values ...int) []byte {
	dst = colorPresetTestAppendArray(dst, 5)
	dst = colorPresetTestAppendInt32(dst, ColorPresetColorVersion)
	for _, value := range values {
		dst = colorPresetTestAppendInt32(dst, value)
	}
	return dst
}

func colorPresetTestAppendGradationColor(dst []byte) []byte {
	dst = colorPresetTestAppendArray(dst, 7)
	dst = colorPresetTestAppendInt32(dst, ColorPresetColorVersion)
	dst = colorPresetTestAppendFreeColor(dst, 10, 11, -12, 13)
	dst = colorPresetTestAppendFreeColor(dst, 14, 15, -16, 17)
	dst = colorPresetTestAppendInt32(dst, 18)
	for _, value := range []float32{0.25, 0.5, 0.75} {
		dst = colorPresetTestAppendArray(dst, 1)
		dst = colorPresetTestAppendFloat32(dst, value)
	}
	return dst
}

func colorPresetTestAppendArray(dst []byte, count int) []byte {
	if count <= 15 {
		return append(dst, 0x90|byte(count))
	}
	panic("test helper only supports fixarray")
}

func colorPresetTestAppendInt32(dst []byte, value int) []byte {
	return simpleEditDataAppendInt32(dst, value)
}

func colorPresetTestAppendString(dst []byte, value string) []byte {
	return simpleEditDataAppendString(dst, value)
}

func colorPresetTestAppendFloat32(dst []byte, value float32) []byte {
	bits := math.Float32bits(value)
	return append(dst, 0xca, byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))
}

func colorPresetTestCompress(t *testing.T, raw []byte) []byte {
	t.Helper()
	if len(raw) < 64 {
		return append([]byte(nil), raw...)
	}
	wire, err := ct.CompressLz4BlockArray(raw)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func colorPresetTestItoa(value int) string {
	if value == 1002 {
		return "1002"
	}
	return "1005"
}

func strconvIntSizeForColorPresetTest() int {
	return 32 << (^uint(0) >> 63)
}

func cloneColorPresetTestValue(value *ColorPreset) *ColorPreset {
	if value == nil {
		return nil
	}
	clone := *value
	if value.ID != nil {
		id := *value.ID
		clone.ID = &id
	}
	if value.BaseMenuFile != nil {
		name := *value.BaseMenuFile
		clone.BaseMenuFile = &name
	}
	clone.ColorPackList = append([]*ColorPresetColorPack(nil), value.ColorPackList...)
	if value.ColorPackList != nil {
		clone.ColorPackList = make([]*ColorPresetColorPack, len(value.ColorPackList))
		for i, pack := range value.ColorPackList {
			if pack == nil {
				continue
			}
			packClone := *pack
			if pack.MPNs != nil {
				packClone.MPNs = append(make([]int, 0, len(pack.MPNs)), pack.MPNs...)
			}
			if pack.MPNNames != nil {
				packClone.MPNNames = append(make([]string, 0, len(pack.MPNNames)), pack.MPNNames...)
			}
			if pack.MPNNameNulls != nil {
				packClone.MPNNameNulls = append(make([]bool, 0, len(pack.MPNNameNulls)), pack.MPNNameNulls...)
			}
			if pack.ColorList != nil {
				packClone.ColorList = append(make([]*ColorPresetLayerFreeColor, 0, len(pack.ColorList)), pack.ColorList...)
			}
			if pack.GradationColorList != nil {
				packClone.GradationColorList = append(make([]*ColorPresetGradationColor, 0, len(pack.GradationColorList)), pack.GradationColorList...)
			}
			clone.ColorPackList[i] = &packClone
		}
	}
	return &clone
}
