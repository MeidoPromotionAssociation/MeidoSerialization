package KCES

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestPresetPanelNameSaveDataRoundTripAndDeterministicWire(t *testing.T) {
	original := &PresetPanelNameSaveData{BoxNameList: []*string{editDataString("BOX1"), nil, editDataString("日本語"), editDataString("")}}
	originalNames := append([]*string(nil), original.BoxNameList...)

	encoded, err := EncodePresetPanelNameSaveData(original)
	if err != nil {
		t.Fatalf("EncodePresetPanelNameSaveData() error = %v", err)
	}
	expected := editDataPresetNamesWire(original.BoxNameList)
	if !bytes.Equal(encoded, expected) {
		t.Fatalf("encoded wire = %x, want canonical wire %x", encoded, expected)
	}
	if !reflect.DeepEqual(original.BoxNameList, originalNames) {
		t.Fatalf("encoder modified caller: got %#v, want %#v", original.BoxNameList, originalNames)
	}

	encodedAgain, err := EncodePresetPanelNameSaveData(original)
	if err != nil {
		t.Fatalf("second EncodePresetPanelNameSaveData() error = %v", err)
	}
	if !bytes.Equal(encodedAgain, encoded) {
		t.Fatalf("encoding is not deterministic: first=%x second=%x", encoded, encodedAgain)
	}

	decoded, err := DecodePresetPanelNameSaveData(encoded)
	if err != nil {
		t.Fatalf("DecodePresetPanelNameSaveData() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round trip = %#v, want %#v", decoded, original)
	}
}

func TestPresetPanelNameSaveDataAllowsSchemaValidNonDefaultLengths(t *testing.T) {
	// PresetPanel currently creates ten names because PageCount is ten, but the
	// source constructor accepts an arbitrary IReadOnlyList<string>. The list
	// length is therefore a business default, not a MessagePack schema constant.
	for _, names := range [][]*string{{}, {editDataString("only-one")}, make([]*string, 11)} {
		encoded, err := EncodePresetPanelNameSaveData(&PresetPanelNameSaveData{BoxNameList: names})
		if err != nil {
			t.Fatalf("EncodePresetPanelNameSaveData(len=%d) error = %v", len(names), err)
		}
		decoded, err := DecodePresetPanelNameSaveData(encoded)
		if err != nil {
			t.Fatalf("DecodePresetPanelNameSaveData(len=%d) error = %v", len(names), err)
		}
		if decoded.BoxNameList == nil || !reflect.DeepEqual(decoded.BoxNameList, names) {
			t.Fatalf("decoded len=%d names = %#v, want non-nil %#v", len(names), decoded.BoxNameList, names)
		}
	}
}

func TestPresetPanelNameSaveDataCollectionAndStringHeaderBoundaries(t *testing.T) {
	stringLengths := []int{0, 31, 32, 255, 256, 65535, 65536}
	names := make([]*string, 0, len(stringLengths))
	for _, length := range stringLengths {
		names = append(names, editDataString(strings.Repeat("x", length)))
	}
	encoded, err := EncodePresetPanelNameSaveData(&PresetPanelNameSaveData{BoxNameList: names})
	if err != nil {
		t.Fatalf("EncodePresetPanelNameSaveData(string boundaries) error = %v", err)
	}
	if expected := editDataPresetNamesWire(names); !bytes.Equal(encoded, expected) {
		t.Fatalf("string-boundary wire differs from canonical MessagePack-CSharp layout")
	}
	decoded, err := DecodePresetPanelNameSaveData(encoded)
	if err != nil || !reflect.DeepEqual(decoded.BoxNameList, names) {
		t.Fatalf("string-boundary round trip = (%#v, %v)", decoded, err)
	}

	for _, count := range []int{15, 16, 65535, 65536} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			source := &PresetPanelNameSaveData{BoxNameList: make([]*string, count)}
			encoded, err := EncodePresetPanelNameSaveData(source)
			if err != nil {
				t.Fatalf("EncodePresetPanelNameSaveData(count=%d) error = %v", count, err)
			}
			decoded, err := DecodePresetPanelNameSaveData(encoded)
			if err != nil {
				t.Fatalf("DecodePresetPanelNameSaveData(count=%d) error = %v", count, err)
			}
			if decoded.BoxNameList == nil || len(decoded.BoxNameList) != count {
				t.Fatalf("decoded count = %d, want %d", len(decoded.BoxNameList), count)
			}
		})
	}
}

func TestPresetPanelNameSaveDataMatchesIndexedObjectCompatibility(t *testing.T) {
	// DynamicObjectTypeBuilder returns nil for a nil class value. PresetPanel's
	// caller treats a nil result as missing data and regenerates BOX1..BOX10.
	decodedNil, err := DecodePresetPanelNameSaveData([]byte{0xc0})
	if err != nil || decodedNil != nil {
		t.Fatalf("DecodePresetPanelNameSaveData(nil) = (%#v, %v), want (nil, nil)", decodedNil, err)
	}
	encodedNil, err := EncodePresetPanelNameSaveData(nil)
	if err != nil || !bytes.Equal(encodedNil, []byte{0xc0}) {
		t.Fatalf("EncodePresetPanelNameSaveData(nil) = (%x, %v), want c0", encodedNil, err)
	}

	// Future indexed-object slots are skipped by DynamicObjectTypeBuilder.
	wire := editDataAppendArrayHeader(nil, 3)
	wire = append(wire, editDataPresetNamesWire([]*string{nil, editDataString("BOX2")})[1:]...)
	wire = append(wire, 0x92, 0x01, 0x02)      // unknown Key(1)
	wire = append(wire, 0x81, 0xa1, 'x', 0xc3) // unknown Key(2)
	decoded, err := DecodePresetPanelNameSaveData(wire)
	if err != nil {
		t.Fatalf("DecodePresetPanelNameSaveData(future keys) error = %v", err)
	}
	fieldCount := int32(3)
	want := &PresetPanelNameSaveData{
		BoxNameList: []*string{nil, editDataString("BOX2")},
		FieldCount:  &fieldCount,
		FutureSlots: [][]byte{{0x92, 0x01, 0x02}, {0x81, 0xa1, 'x', 0xc3}},
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("future-key decode = %#v, want %#v", decoded, want)
	}
	reencoded, err := EncodePresetPanelNameSaveData(decoded)
	if err != nil || !bytes.Equal(reencoded, wire) {
		t.Fatalf("future-key wire changed: equal=%v err=%v", bytes.Equal(reencoded, wire), err)
	}
}

func TestPresetPanelNameSaveDataRejectsMalformedWire(t *testing.T) {
	valid := editDataPresetNamesWire([]*string{editDataString("BOX1")})
	tests := map[string][]byte{
		"empty":                    nil,
		"top-level wrong type":     {0x80},
		"name list wrong type":     {0x91, 0x80},
		"name element wrong type":  {0x91, 0x91, 0x01},
		"truncated string":         {0x91, 0x91, 0xa2, 'x'},
		"declared collection bomb": {0x91, 0xdd, 0xff, 0xff, 0xff, 0xff},
		"truncated future key":     append(append([]byte{0x92}, valid[1:]...), 0xdc, 0x00),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodePresetPanelNameSaveData(data); err == nil {
				t.Fatalf("DecodePresetPanelNameSaveData(%x) unexpectedly succeeded", data)
			}
		})
	}
}

func TestPresetPanelNameSaveDataMatchesGameInvalidUTF8Replacement(t *testing.T) {
	decoded, err := DecodePresetPanelNameSaveData([]byte{0x91, 0x91, 0xa1, 0xff})
	if err != nil {
		t.Fatalf("DecodePresetPanelNameSaveData(invalid UTF-8) error = %v", err)
	}
	if decoded == nil || len(decoded.BoxNameList) != 1 || decoded.BoxNameList[0] == nil || *decoded.BoxNameList[0] != "\uFFFD" {
		t.Fatalf("invalid UTF-8 replacement = %#v, want U+FFFD", decoded)
	}
	encoded, err := EncodePresetPanelNameSaveData(decoded)
	if err != nil {
		t.Fatalf("EncodePresetPanelNameSaveData(replacement) error = %v", err)
	}
	if expected := editDataPresetNamesWire([]*string{editDataString("\uFFFD")}); !bytes.Equal(encoded, expected) {
		t.Fatalf("normalized wire = %x, want %x", encoded, expected)
	}
}

func TestPresetPanelNameSaveDataEncodeValidation(t *testing.T) {
	wire, err := EncodePresetPanelNameSaveData(&PresetPanelNameSaveData{})
	if err != nil {
		t.Fatalf("nil BoxNameList encode: %v", err)
	}
	got, err := DecodePresetPanelNameSaveData(wire)
	if err != nil || got.BoxNameList != nil {
		t.Fatalf("nil BoxNameList round trip = %#v, %v", got, err)
	}
	invalid := "\xff"
	if _, err := EncodePresetPanelNameSaveData(&PresetPanelNameSaveData{BoxNameList: []*string{&invalid}}); err == nil {
		t.Fatal("invalid UTF-8 box name unexpectedly encoded")
	}
}

func TestPaletteColorSaveDataRoundTripAndDeterministicWire(t *testing.T) {
	colors := map[int32]int32{
		8: 808, 7: -707, 6: 606, 5: -505, 4: 404,
		3: -303, 2: 202, 1: -101, 0: 0,
		42: 4242, -7: -7000, // forward-compatible Dictionary entries
	}
	original := &PaletteColorSaveData{Color: colors, Index: 7, IsSave: 1}
	originalColors := editDataCloneIntMap(colors)

	encoded, err := EncodePaletteColorSaveData(original)
	if err != nil {
		t.Fatalf("EncodePaletteColorSaveData() error = %v", err)
	}
	expected := editDataPaletteWire(originalColors, original.Index, original.IsSave, false)
	if !bytes.Equal(encoded, expected) {
		t.Fatalf("encoded wire = %x, want canonical sorted-key wire %x", encoded, expected)
	}
	if !reflect.DeepEqual(original.Color, originalColors) {
		t.Fatalf("encoder modified caller map: got %#v, want %#v", original.Color, originalColors)
	}

	reordered := &PaletteColorSaveData{Color: make(map[int32]int32), Index: 7, IsSave: 1}
	keys := make([]int32, 0, len(originalColors))
	for key := range originalColors {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	slices.Reverse(keys)
	for _, key := range keys {
		reordered.Color[key] = originalColors[key]
	}
	encodedAgain, err := EncodePaletteColorSaveData(reordered)
	if err != nil {
		t.Fatalf("EncodePaletteColorSaveData(reordered) error = %v", err)
	}
	if !bytes.Equal(encodedAgain, encoded) {
		t.Fatalf("map insertion order changed wire: first=%x second=%x", encoded, encodedAgain)
	}

	// A Dictionary has no wire ordering guarantee. Decode a deliberately
	// reversed source order and ensure values, not encounter order, win.
	reversedWire := editDataPaletteWire(originalColors, original.Index, original.IsSave, true)
	decoded, err := DecodePaletteColorSaveData(reversedWire)
	if err != nil {
		t.Fatalf("DecodePaletteColorSaveData() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round trip = %#v, want %#v", decoded, original)
	}
}

func TestPaletteColorSaveDataMatchesIndexedObjectCompatibility(t *testing.T) {
	colors := editDataNineColors()
	colors[99] = 12345 // Dictionary<int,int> permits and preserves unknown keys.

	// Key(1) and Key(2) are absent in an old/short indexed object. The game's
	// parameterless constructor leaves both int fields at zero.
	shortWire := editDataAppendArrayHeader(nil, 1)
	shortWire = append(shortWire, editDataPaletteMap(colors, true)...)
	decoded, err := DecodePaletteColorSaveData(shortWire)
	if err != nil {
		t.Fatalf("DecodePaletteColorSaveData(short object) error = %v", err)
	}
	if decoded.FieldCount == nil || *decoded.FieldCount != 1 || !reflect.DeepEqual(decoded.Color, colors) || decoded.Index != 0 || decoded.IsSave != 0 {
		t.Fatalf("short-object decode changed shape/value: %#v", decoded)
	}
	reencoded, err := EncodePaletteColorSaveData(decoded)
	if err != nil || len(reencoded) == 0 || reencoded[0] != 0x91 {
		t.Fatalf("short-object width changed: wire=%x err=%v", reencoded, err)
	}

	// A future Key(3) is skipped. It is distinct from bytes trailing after the
	// root value, which this library deliberately rejects.
	longWire := editDataAppendArrayHeader(nil, 4)
	longWire = append(longWire, editDataPaletteMap(colors, true)...)
	longWire = editDataAppendInt(longWire, 3)
	longWire = editDataAppendInt(longWire, 1)
	longWire = append(longWire, 0x82, 0xa1, 'a', 0x91, 0xc0, 0xa1, 'b', 0xc7, 0x01, 0x2a, 0xff)
	decoded, err = DecodePaletteColorSaveData(longWire)
	if err != nil {
		t.Fatalf("DecodePaletteColorSaveData(future key) error = %v", err)
	}
	if decoded.FieldCount == nil || *decoded.FieldCount != 4 || len(decoded.FutureSlots) != 1 || !reflect.DeepEqual(decoded.Color, colors) || decoded.Index != 3 || decoded.IsSave != 1 {
		t.Fatalf("future-key decode changed shape/value: %#v", decoded)
	}
	reencoded, err = EncodePaletteColorSaveData(decoded)
	if err != nil || len(reencoded) == 0 || reencoded[0] != 0x94 || !bytes.HasSuffix(reencoded, decoded.FutureSlots[0]) {
		t.Fatalf("future-key shape/raw value changed: wire=%x err=%v", reencoded, err)
	}
}

func TestPaletteColorSaveDataAcceptsAllInt32WireBoundaries(t *testing.T) {
	colors := editDataNineColors()
	colors[0] = math.MinInt32
	colors[8] = math.MaxInt32
	wire := editDataPaletteWire(colors, math.MinInt32, math.MaxInt32, true)
	decoded, err := DecodePaletteColorSaveData(wire)
	if err != nil {
		t.Fatalf("DecodePaletteColorSaveData(boundaries) error = %v", err)
	}
	if decoded.Color[0] != math.MinInt32 || decoded.Color[8] != math.MaxInt32 || decoded.Index != math.MinInt32 || decoded.IsSave != math.MaxInt32 {
		t.Fatalf("decoded Int32 boundaries incorrectly: %#v", decoded)
	}
	if _, err := EncodePaletteColorSaveData(decoded); err != nil {
		t.Fatalf("EncodePaletteColorSaveData(boundaries) error = %v", err)
	}
}

func TestPaletteColorSaveDataAcceptsEveryInt32CompatibleMarker(t *testing.T) {
	variants := []struct {
		name string
		raw  []byte
		want int32
	}{
		{name: "positive fixint", raw: []byte{0x7f}, want: 127},
		{name: "uint8", raw: []byte{0xcc, 0xff}, want: 255},
		{name: "uint16", raw: []byte{0xcd, 0xff, 0xff}, want: 65535},
		{name: "uint32", raw: []byte{0xce, 0x7f, 0xff, 0xff, 0xff}, want: math.MaxInt32},
		{name: "uint64", raw: []byte{0xcf, 0, 0, 0, 0, 0x7f, 0xff, 0xff, 0xff}, want: math.MaxInt32},
		{name: "negative fixint", raw: []byte{0xe0}, want: -32},
		{name: "int8", raw: []byte{0xd0, 0x80}, want: math.MinInt8},
		{name: "int16", raw: []byte{0xd1, 0x80, 0x00}, want: math.MinInt16},
		{name: "int32", raw: []byte{0xd2, 0x80, 0x00, 0x00, 0x00}, want: math.MinInt32},
		{name: "int64", raw: []byte{0xd3, 0xff, 0xff, 0xff, 0xff, 0x80, 0x00, 0x00, 0x00}, want: math.MinInt32},
	}
	entries := editDataPaletteEntries(0, 8)
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			wire := editDataPaletteWireWithEntries(entries, variant.raw, editDataAppendInt(nil, 1))
			decoded, err := DecodePaletteColorSaveData(wire)
			if err != nil {
				t.Fatalf("DecodePaletteColorSaveData(%s) error = %v", variant.name, err)
			}
			if decoded.Index != variant.want {
				t.Fatalf("decoded index = %d, want %d", decoded.Index, variant.want)
			}
		})
	}
}

func TestPaletteColorSaveDataRejectsMalformedWire(t *testing.T) {
	valid := editDataPaletteWire(editDataNineColors(), 2, 1, false)
	duplicateEntries := editDataPaletteEntries(0, 7)
	duplicateEntries = append(duplicateEntries, editDataIntPair(0, 999))
	duplicate := editDataPaletteWireWithEntries(duplicateEntries, editDataAppendInt(nil, 2), editDataAppendInt(nil, 1))
	wrongKeyEntries := editDataPaletteEntries(1, 8)
	wrongKeyEntries = append([][]byte{append(editDataAppendString(nil, "zero"), editDataAppendInt(nil, 0)...)}, wrongKeyEntries...)
	wrongValueEntries := editDataPaletteEntries(1, 8)
	wrongValueEntries = append([][]byte{append(editDataAppendInt(nil, 0), editDataAppendString(nil, "zero")...)}, wrongValueEntries...)

	tests := map[string][]byte{
		"empty":                   nil,
		"top-level wrong type":    {0x80},
		"color map wrong type":    {0x93, 0x90, 0x00, 0x00},
		"duplicate color key":     duplicate,
		"non-integer color key":   editDataPaletteWireWithEntries(wrongKeyEntries, editDataAppendInt(nil, 2), editDataAppendInt(nil, 1)),
		"non-integer color value": editDataPaletteWireWithEntries(wrongValueEntries, editDataAppendInt(nil, 2), editDataAppendInt(nil, 1)),
		"non-integer index":       editDataPaletteWireWithEntries(editDataPaletteEntries(0, 8), []byte{0xa1, '2'}, editDataAppendInt(nil, 1)),
		"non-integer isSave":      editDataPaletteWireWithEntries(editDataPaletteEntries(0, 8), editDataAppendInt(nil, 2), []byte{0xc3}),
		"declared map bomb":       {0x93, 0xdf, 0xff, 0xff, 0xff, 0xff},
		"truncated future key":    append(append([]byte{0x94}, valid[1:]...), 0xc7, 0x02, 0x2a, 0x00),
		"truncated":               valid[:len(valid)-1],
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodePaletteColorSaveData(data); err == nil {
				t.Fatalf("DecodePaletteColorSaveData(%x) unexpectedly succeeded", data)
			}
		})
	}
}

func TestPaletteColorSaveDataRejectsInt32WireOverflow(t *testing.T) {
	validEntries := editDataPaletteEntries(0, 8)
	uint32Overflow := []byte{0xce, 0x80, 0x00, 0x00, 0x00}
	int64Underflow := []byte{0xd3, 0xff, 0xff, 0xff, 0xff, 0x7f, 0xff, 0xff, 0xff}

	overflowValueEntries := editDataPaletteEntries(1, 8)
	overflowValueEntries = append([][]byte{append(editDataAppendInt(nil, 0), uint32Overflow...)}, overflowValueEntries...)
	overflowKeyEntries := editDataPaletteEntries(1, 8)
	overflowKeyEntries = append([][]byte{append(append([]byte(nil), uint32Overflow...), editDataAppendInt(nil, 0)...)}, overflowKeyEntries...)

	tests := map[string][]byte{
		"color key above Int32":   editDataPaletteWireWithEntries(overflowKeyEntries, editDataAppendInt(nil, 0), editDataAppendInt(nil, 1)),
		"color value above Int32": editDataPaletteWireWithEntries(overflowValueEntries, editDataAppendInt(nil, 0), editDataAppendInt(nil, 1)),
		"index above Int32":       editDataPaletteWireWithEntries(validEntries, uint32Overflow, editDataAppendInt(nil, 1)),
		"isSave below Int32":      editDataPaletteWireWithEntries(validEntries, editDataAppendInt(nil, 0), int64Underflow),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := DecodePaletteColorSaveData(data)
			if err == nil || !strings.Contains(err.Error(), "Int32") {
				t.Fatalf("DecodePaletteColorSaveData() error = %v, want Int32 overflow rejection", err)
			}
		})
	}
}

func TestPaletteColorSaveDataEncodeValidation(t *testing.T) {
	tests := map[string]*PaletteColorSaveData{
		"nil value": nil,
		"nil map":   {Index: 0, IsSave: 1},
		"missing key": {Color: map[int32]int32{
			0: 0, 1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7,
		}},
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			wire, err := EncodePaletteColorSaveData(value)
			if err != nil {
				t.Fatalf("EncodePaletteColorSaveData(%#v): %v", value, err)
			}
			got, err := DecodePaletteColorSaveData(wire)
			if err != nil || !reflect.DeepEqual(got, value) {
				t.Fatalf("round trip = %#v, %v; want %#v", got, err, value)
			}
		})
	}

}

func TestSimpleEditDataDecoderTruncationsNeverSucceedOrPanic(t *testing.T) {
	preset := editDataPresetNamesWire([]*string{editDataString("BOX1"), nil, editDataString("BOX3")})
	palette := editDataPaletteWire(editDataNineColors(), 8, 1, true)
	for name, test := range map[string]struct {
		wire   []byte
		decode func([]byte) error
	}{
		"preset": {
			wire: preset,
			decode: func(data []byte) error {
				_, err := DecodePresetPanelNameSaveData(data)
				return err
			},
		},
		"palette": {
			wire: palette,
			decode: func(data []byte) error {
				_, err := DecodePaletteColorSaveData(data)
				return err
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			for length := 0; length < len(test.wire); length++ {
				if err := test.decode(test.wire[:length]); err == nil {
					t.Fatalf("truncated prefix %d/%d unexpectedly succeeded", length, len(test.wire))
				}
			}
		})
	}
}

func TestPresetPanelNameSaveDataSkipsEveryFutureMessagePackFamily(t *testing.T) {
	values := map[string][]byte{
		"positive fixint": {0x01},
		"negative fixint": {0xff},
		"fixstr":          {0xa1, 0xff}, // Skip does not decode unknown UTF-8.
		"fixarray":        {0x91, 0xc0},
		"fixmap":          {0x81, 0xc0, 0xc0},
		"nil":             {0xc0},
		"false":           {0xc2},
		"true":            {0xc3},
		"bin8":            {0xc4, 0x01, 0x00},
		"bin16":           {0xc5, 0x00, 0x01, 0x00},
		"bin32":           {0xc6, 0x00, 0x00, 0x00, 0x01, 0x00},
		"ext8":            {0xc7, 0x01, 0x2a, 0x00},
		"ext16":           {0xc8, 0x00, 0x01, 0x2a, 0x00},
		"ext32":           {0xc9, 0x00, 0x00, 0x00, 0x01, 0x2a, 0x00},
		"float32":         {0xca, 0, 0, 0, 0},
		"float64":         {0xcb, 0, 0, 0, 0, 0, 0, 0, 0},
		"uint8":           {0xcc, 0x80},
		"uint16":          {0xcd, 0, 0},
		"uint32":          {0xce, 0, 0, 0, 0},
		"uint64":          {0xcf, 0, 0, 0, 0, 0, 0, 0, 0},
		"int8":            {0xd0, 0},
		"int16":           {0xd1, 0, 0},
		"int32":           {0xd2, 0, 0, 0, 0},
		"int64":           {0xd3, 0, 0, 0, 0, 0, 0, 0, 0},
		"fixext1":         {0xd4, 0x2a, 0},
		"fixext2":         {0xd5, 0x2a, 0, 0},
		"fixext4":         {0xd6, 0x2a, 0, 0, 0, 0},
		"fixext8":         {0xd7, 0x2a, 0, 0, 0, 0, 0, 0, 0, 0},
		"fixext16":        {0xd8, 0x2a, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"str8":            {0xd9, 0x01, 0xff},
		"str16":           {0xda, 0x00, 0x01, 0xff},
		"str32":           {0xdb, 0x00, 0x00, 0x00, 0x01, 0xff},
		"array16":         {0xdc, 0x00, 0x01, 0xc0},
		"array32":         {0xdd, 0x00, 0x00, 0x00, 0x01, 0xc0},
		"map16":           {0xde, 0x00, 0x01, 0xc0, 0xc0},
		"map32":           {0xdf, 0x00, 0x00, 0x00, 0x01, 0xc0, 0xc0},
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			wire := append([]byte{0x92, 0x90}, value...)
			decoded, err := DecodePresetPanelNameSaveData(wire)
			if err != nil {
				t.Fatalf("DecodePresetPanelNameSaveData(future %s) error = %v", name, err)
			}
			if decoded == nil || decoded.BoxNameList == nil || len(decoded.BoxNameList) != 0 {
				t.Fatalf("future %s changed known fields: %#v", name, decoded)
			}
		})
	}
}

func editDataNineColors() map[int32]int32 {
	colors := make(map[int32]int32, 9)
	for i := int32(0); i <= 8; i++ {
		colors[i] = i * 10
	}
	return colors
}

func editDataCloneIntMap(src map[int32]int32) map[int32]int32 {
	dst := make(map[int32]int32, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func editDataString(value string) *string {
	return &value
}

func editDataPresetNamesWire(names []*string) []byte {
	wire := editDataAppendArrayHeader(nil, 1)
	wire = editDataAppendArrayHeader(wire, uint32(len(names)))
	for _, name := range names {
		if name == nil {
			wire = append(wire, 0xc0)
		} else {
			wire = editDataAppendString(wire, *name)
		}
	}
	return wire
}

func editDataPaletteWire(colors map[int32]int32, index, isSave int32, reverse bool) []byte {
	keys := make([]int32, 0, len(colors))
	for key := range colors {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if reverse {
		slices.Reverse(keys)
	}
	entries := make([][]byte, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, editDataIntPair(key, colors[key]))
	}
	return editDataPaletteWireWithEntries(entries, editDataAppendInt(nil, index), editDataAppendInt(nil, isSave))
}

func editDataPaletteMap(colors map[int32]int32, reverse bool) []byte {
	full := editDataPaletteWire(colors, 0, 0, reverse)
	// The helper above always emits fixarray(3), followed by the complete map.
	// Locate its exact end by subtracting the two one-byte zero fields.
	return append([]byte(nil), full[1:len(full)-2]...)
}

func editDataPaletteEntries(first, last int32) [][]byte {
	entries := make([][]byte, 0, last-first+1)
	for key := first; key <= last; key++ {
		entries = append(entries, editDataIntPair(key, key*10))
	}
	return entries
}

func editDataIntPair(key, value int32) []byte {
	pair := editDataAppendInt(nil, key)
	return editDataAppendInt(pair, value)
}

func editDataPaletteWireWithEntries(entries [][]byte, indexRaw, isSaveRaw []byte) []byte {
	wire := editDataAppendArrayHeader(nil, 3)
	wire = editDataAppendMapHeader(wire, uint32(len(entries)))
	for _, entry := range entries {
		wire = append(wire, entry...)
	}
	wire = append(wire, indexRaw...)
	wire = append(wire, isSaveRaw...)
	return wire
}

func editDataAppendArrayHeader(dst []byte, length uint32) []byte {
	if length <= 15 {
		return append(dst, 0x90|byte(length))
	}
	if length <= math.MaxUint16 {
		return append(dst, 0xdc, byte(length>>8), byte(length))
	}
	dst = append(dst, 0xdd, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(dst[len(dst)-4:], length)
	return dst
}

func editDataAppendMapHeader(dst []byte, length uint32) []byte {
	if length <= 15 {
		return append(dst, 0x80|byte(length))
	}
	if length <= math.MaxUint16 {
		return append(dst, 0xde, byte(length>>8), byte(length))
	}
	dst = append(dst, 0xdf, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(dst[len(dst)-4:], length)
	return dst
}

func editDataAppendString(dst []byte, value string) []byte {
	length := len(value)
	switch {
	case length <= 31:
		dst = append(dst, 0xa0|byte(length))
	case length <= math.MaxUint8:
		dst = append(dst, 0xd9, byte(length))
	case length <= math.MaxUint16:
		dst = append(dst, 0xda, byte(length>>8), byte(length))
	default:
		dst = append(dst, 0xdb, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(length))
	}
	return append(dst, value...)
}

func editDataAppendInt(dst []byte, value int32) []byte {
	switch {
	case value >= 0 && value <= 0x7f:
		return append(dst, byte(value))
	case value >= 0 && value <= math.MaxUint8:
		return append(dst, 0xcc, byte(value))
	case value >= 0 && value <= math.MaxUint16:
		return append(dst, 0xcd, byte(value>>8), byte(value))
	case value >= 0:
		dst = append(dst, 0xce, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(value))
		return dst
	case value >= -32:
		return append(dst, byte(int8(value)))
	case value >= math.MinInt8:
		return append(dst, 0xd0, byte(int8(value)))
	case value >= math.MinInt16:
		return append(dst, 0xd1, byte(int16(value)>>8), byte(int16(value)))
	default:
		dst = append(dst, 0xd2, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(int32(value)))
		return dst
	}
}
