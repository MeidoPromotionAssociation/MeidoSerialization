package KCES

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeGradPointsDataHandwrittenWire(t *testing.T) {
	wire := gradTestValidWire()
	got, err := DecodeGradPointsData(wire)
	if err != nil {
		t.Fatalf("DecodeGradPointsData: %v", err)
	}

	want := &GradPointsData{
		GradPointParam: []map[int32]int32{{
			0: 10, 1: 20, 2: 30, 3: 40, 4: 50,
			5: 60, 6: 70, 7: 80, 8: 90,
		}},
		ControlPointPosValue:  []float32{0.25},
		GradaPointPosRates:    []float32{-1.5, 2.5},
		EditMPN:               -2147483648,
		PointRangeAfterRates:  []float32{0.75},
		PointRangeBeforeRates: []float32{1},
		IsSave:                2147483647,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded value mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestEncodeGradPointsDataCanonicalWireAndNoMutation(t *testing.T) {
	values := map[int32]int32{}
	for key := int32(8); key >= 0; key-- {
		values[key] = (key + 1) * 10
	}
	input := &GradPointsData{
		GradPointParam:        []map[int32]int32{values},
		ControlPointPosValue:  []float32{0.25},
		GradaPointPosRates:    []float32{-1.5, 2.5},
		EditMPN:               -2147483648,
		PointRangeAfterRates:  []float32{0.75},
		PointRangeBeforeRates: []float32{1},
		IsSave:                2147483647,
	}
	snapshot := gradTestClone(input)

	first, err := EncodeGradPointsData(input)
	if err != nil {
		t.Fatalf("EncodeGradPointsData: %v", err)
	}
	second, err := EncodeGradPointsData(input)
	if err != nil {
		t.Fatalf("second EncodeGradPointsData: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("encoding the same value twice produced different bytes")
	}
	if !bytes.Equal(first, gradTestCanonicalWire()) {
		t.Fatalf("canonical wire mismatch\n got: % x\nwant: % x", first, gradTestCanonicalWire())
	}
	if !reflect.DeepEqual(input, snapshot) {
		t.Fatalf("encoder mutated caller input\n got: %#v\nwant: %#v", input, snapshot)
	}
}

func TestGradPointsDataEmptyAndLegacySlots(t *testing.T) {
	input := &GradPointsData{
		GradPointParam:        []map[int32]int32{},
		ControlPointPosValue:  []float32{},
		GradaPointPosRates:    []float32{3.25},
		EditMPN:               7,
		PointRangeAfterRates:  []float32{},
		PointRangeBeforeRates: []float32{},
		IsSave:                0,
	}

	wire, err := EncodeGradPointsData(input)
	if err != nil {
		t.Fatalf("EncodeGradPointsData: %v", err)
	}
	got, err := DecodeGradPointsData(wire)
	if err != nil {
		t.Fatalf("DecodeGradPointsData: %v", err)
	}
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("round trip mismatch\n got: %#v\nwant: %#v", got, input)
	}
	if got.GradPointParam == nil || got.ControlPointPosValue == nil || got.GradaPointPosRates == nil || got.PointRangeAfterRates == nil || got.PointRangeBeforeRates == nil {
		t.Fatal("empty non-nil game lists became nil")
	}
}

func TestGradPointsDataPreservesAllFloat32BitPatterns(t *testing.T) {
	// MessagePackReader.ReadSingle accepts every IEEE-754 single value. Do not
	// silently impose a finite-only rule that is absent from the game format.
	input := &GradPointsData{
		GradPointParam:        []map[int32]int32{gradTestColorMap()},
		ControlPointPosValue:  []float32{math.Float32frombits(0x7fc01234)},
		GradaPointPosRates:    []float32{float32(math.Inf(1)), float32(math.Inf(-1))},
		PointRangeAfterRates:  []float32{float32(math.Inf(1))},
		PointRangeBeforeRates: []float32{float32(math.Inf(-1))},
	}
	wire, err := EncodeGradPointsData(input)
	if err != nil {
		t.Fatalf("EncodeGradPointsData: %v", err)
	}
	got, err := DecodeGradPointsData(wire)
	if err != nil {
		t.Fatalf("DecodeGradPointsData: %v", err)
	}
	if math.Float32bits(got.ControlPointPosValue[0]) != 0x7fc01234 {
		t.Fatalf("NaN payload bits got 0x%08x, want 0x7fc01234", math.Float32bits(got.ControlPointPosValue[0]))
	}
	if !math.IsInf(float64(got.GradaPointPosRates[0]), 1) || !math.IsInf(float64(got.GradaPointPosRates[1]), -1) ||
		!math.IsInf(float64(got.PointRangeAfterRates[0]), 1) || !math.IsInf(float64(got.PointRangeBeforeRates[0]), -1) {
		t.Fatalf("non-finite float32 values did not round trip: %#v", got)
	}
}

func TestDecodeGradPointsDataReadSingleCompatibility(t *testing.T) {
	tests := []struct {
		name string
		wire []byte
		want float32
	}{
		{name: "float64", wire: []byte{0x91, 0xcb, 0x3f, 0xd0, 0, 0, 0, 0, 0, 0}, want: 0.25},
		{name: "positive fixint", wire: []byte{0x91, 0x2a}, want: 42},
		{name: "negative fixint", wire: []byte{0x91, 0xff}, want: -1},
		{name: "uint64", wire: []byte{0x91, 0xcf, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, want: float32(uint64(math.MaxUint64))},
		{name: "int64", wire: []byte{0x91, 0xd3, 0x80, 0, 0, 0, 0, 0, 0, 0}, want: float32(int64(math.MinInt64))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DecodeGradPointsData(gradTestWireWithSlot(1, test.wire))
			if err != nil {
				t.Fatalf("DecodeGradPointsData: %v", err)
			}
			if math.Float32bits(got.ControlPointPosValue[0]) != math.Float32bits(test.want) {
				t.Fatalf("value got %v (0x%08x), want %v (0x%08x)", got.ControlPointPosValue[0], math.Float32bits(got.ControlPointPosValue[0]), test.want, math.Float32bits(test.want))
			}
		})
	}
}

func TestEncodeGradPointsDataCollectionHeaders(t *testing.T) {
	t.Run("array16", func(t *testing.T) {
		value := NewGradPointsData()
		for i := 0; i < 16; i++ {
			value.GradPointParam = append(value.GradPointParam, gradTestColorMap())
			value.ControlPointPosValue = append(value.ControlPointPosValue, float32(i))
			value.PointRangeAfterRates = append(value.PointRangeAfterRates, float32(i)+0.25)
			value.PointRangeBeforeRates = append(value.PointRangeBeforeRates, float32(i)+0.5)
		}
		wire, err := EncodeGradPointsData(value)
		if err != nil {
			t.Fatalf("EncodeGradPointsData: %v", err)
		}
		if len(wire) < 4 || !bytes.Equal(wire[:4], []byte{0x97, 0xdc, 0x00, 0x10}) {
			t.Fatalf("outer point list header got % x, want 97 dc 00 10", wire[:min(len(wire), 4)])
		}
		got, err := DecodeGradPointsData(wire)
		if err != nil {
			t.Fatalf("DecodeGradPointsData: %v", err)
		}
		if !reflect.DeepEqual(got, value) {
			t.Fatal("array16 value did not round trip")
		}
	})

	t.Run("map16", func(t *testing.T) {
		color := gradTestColorMap()
		for key := int32(9); key < 16; key++ {
			color[key] = key * 2
		}
		value := &GradPointsData{
			GradPointParam:        []map[int32]int32{color},
			ControlPointPosValue:  []float32{0},
			GradaPointPosRates:    []float32{},
			PointRangeAfterRates:  []float32{0},
			PointRangeBeforeRates: []float32{0},
		}
		wire, err := EncodeGradPointsData(value)
		if err != nil {
			t.Fatalf("EncodeGradPointsData: %v", err)
		}
		if len(wire) < 5 || !bytes.Equal(wire[:5], []byte{0x97, 0x91, 0xde, 0x00, 0x10}) {
			t.Fatalf("color map header got % x, want 97 91 de 00 10", wire[:min(len(wire), 5)])
		}
		got, err := DecodeGradPointsData(wire)
		if err != nil {
			t.Fatalf("DecodeGradPointsData: %v", err)
		}
		if !reflect.DeepEqual(got, value) {
			t.Fatal("map16 value did not round trip")
		}
	})
}

func TestDecodeGradPointsDataIndexedArrayCompatibility(t *testing.T) {
	t.Run("empty short array keeps wire zero values", func(t *testing.T) {
		got, err := DecodeGradPointsData([]byte{0x90})
		if err != nil {
			t.Fatalf("DecodeGradPointsData: %v", err)
		}
		if got.FieldCount == nil || *got.FieldCount != 0 || got.GradPointParam != nil || got.ControlPointPosValue != nil {
			t.Fatalf("short array shape/value changed: %#v", got)
		}
		reencoded, err := EncodeGradPointsData(got)
		if err != nil || !bytes.Equal(reencoded, []byte{0x90}) {
			t.Fatalf("short array re-encoded as %x, %v", reencoded, err)
		}
	})

	t.Run("root nil is an absent saved slot", func(t *testing.T) {
		got, err := DecodeGradPointsData([]byte{0xc0})
		if err != nil {
			t.Fatalf("DecodeGradPointsData(nil): %v", err)
		}
		if got != nil {
			t.Fatalf("decoded root nil as %#v", got)
		}
		wire, err := EncodeGradPointsData(nil)
		if err != nil {
			t.Fatalf("EncodeGradPointsData(nil): %v", err)
		}
		if !bytes.Equal(wire, []byte{0xc0}) {
			t.Fatalf("encoded root nil as % x", wire)
		}
	})

	t.Run("six slots leave isSave at default", func(t *testing.T) {
		valid := gradTestValidWire()
		// The final value is a d2 followed by four payload bytes.
		wire := append([]byte{0x96}, valid[1:len(valid)-5]...)
		got, err := DecodeGradPointsData(wire)
		if err != nil {
			t.Fatalf("DecodeGradPointsData: %v", err)
		}
		if got.IsSave != 0 || got.FieldCount == nil || *got.FieldCount != 6 {
			t.Fatalf("six-slot shape/value changed: %#v", got)
		}
		reencoded, err := EncodeGradPointsData(got)
		if err != nil || len(reencoded) == 0 || reencoded[0] != 0x96 {
			t.Fatalf("six-slot width changed: wire=%x err=%v", reencoded, err)
		}
	})

	t.Run("future fields are consumed and preserved", func(t *testing.T) {
		valid := gradTestValidWire()
		wire := append([]byte{0x98}, valid[1:]...)
		// A nested unknown value exercises recursive Skip rather than merely
		// consuming one scalar byte.
		wire = append(wire, 0x81, 0xa1, 'x', 0x92, 0xc3, 0xc0)
		got, err := DecodeGradPointsData(wire)
		if err != nil {
			t.Fatalf("DecodeGradPointsData: %v", err)
		}
		if got.EditMPN != math.MinInt32 || got.IsSave != math.MaxInt32 || got.FieldCount == nil || *got.FieldCount != 8 || len(got.FutureSlots) != 1 {
			t.Fatalf("known/future fields changed: %#v", got)
		}
		reencoded, err := EncodeGradPointsData(got)
		if err != nil || len(reencoded) == 0 || reencoded[0] != 0x98 || !bytes.HasSuffix(reencoded, got.FutureSlots[0]) {
			t.Fatalf("future-slot shape/raw value changed: wire=%x err=%v", reencoded, err)
		}
	})

	t.Run("malformed future field is still rejected", func(t *testing.T) {
		valid := gradTestValidWire()
		wire := append([]byte{0x98}, valid[1:]...)
		wire = append(wire, 0xc1) // reserved MessagePack marker
		if _, err := DecodeGradPointsData(wire); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("got error %v, want reserved-marker error", err)
		}
	})
}

func TestDecodeGradPointsDataRejectsMalformedWire(t *testing.T) {
	valid := gradTestValidWire()

	tests := []struct {
		name string
		wire []byte
		want string
	}{
		{name: "empty", wire: nil, want: "array"},
		{name: "top-level map", wire: []byte{0x80}, want: "array"},
		{name: "truncated", wire: valid[:len(valid)-1], want: "truncated"},
		{name: "duplicate color key", wire: gradTestWireWithSlot(0, gradTestMapList(9, func(i int) int {
			if i == 8 {
				return 7
			}
			return i
		})), want: "duplicate"},
		{name: "color value wrong type", wire: gradTestWireWithSlot(0, gradTestMapListBadValue()), want: "Int32"},
		{name: "color key overflow", wire: gradTestWireWithSlot(0, gradTestMapListOverflowKey()), want: "Int32"},
		{name: "color value overflow", wire: gradTestWireWithSlot(0, gradTestMapListOverflowValue()), want: "Int32"},
		{name: "edit MPN overflow", wire: gradTestWireWithSlot(3, []byte{0xcf, 0, 0, 0, 0, 0x80, 0, 0, 0}), want: "Int32"},
		{name: "isSave underflow", wire: gradTestWireWithSlot(6, []byte{0xd3, 0xff, 0xff, 0xff, 0xff, 0x7f, 0xff, 0xff, 0xff}), want: "Int32"},
		{name: "bool instead of Single", wire: gradTestWireWithSlot(1, []byte{0x91, 0xc3}), want: "ReadSingle"},
		{name: "string instead of Single", wire: gradTestWireWithSlot(1, []byte{0x91, 0xa1, 'x'}), want: "ReadSingle"},
		{name: "hostile array32", wire: gradTestWireWithSlot(1, []byte{0xdd, 0x7f, 0xff, 0xff, 0xff}), want: "truncated"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeGradPointsData(test.wire)
			if err == nil {
				t.Fatal("malformed wire unexpectedly decoded")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

func TestGradPointsDataPreservesIndependentCollectionLengths(t *testing.T) {
	base := &GradPointsData{
		GradPointParam:        []map[int32]int32{gradTestColorMap()},
		ControlPointPosValue:  []float32{0},
		GradaPointPosRates:    []float32{},
		PointRangeAfterRates:  []float32{0},
		PointRangeBeforeRates: []float32{0},
	}

	tests := []struct {
		name   string
		mutate func(*GradPointsData)
	}{
		{name: "control positions", mutate: func(v *GradPointsData) { v.ControlPointPosValue = []float32{} }},
		{name: "after ranges", mutate: func(v *GradPointsData) { v.PointRangeAfterRates = []float32{} }},
		{name: "before ranges", mutate: func(v *GradPointsData) { v.PointRangeBeforeRates = []float32{} }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := gradTestClone(base)
			test.mutate(value)
			wire, err := EncodeGradPointsData(value)
			if err != nil {
				t.Fatalf("EncodeGradPointsData: %v", err)
			}
			got, err := DecodeGradPointsData(wire)
			if err != nil || !reflect.DeepEqual(got, value) {
				t.Fatalf("independent lengths round trip = %#v, %v; want %#v", got, err, value)
			}
		})
	}
}

func TestEncodeGradPointsDataValidationAndInt32Bounds(t *testing.T) {
	valid := &GradPointsData{
		GradPointParam:        []map[int32]int32{gradTestColorMap()},
		ControlPointPosValue:  []float32{0},
		GradaPointPosRates:    []float32{},
		PointRangeAfterRates:  []float32{0},
		PointRangeBeforeRates: []float32{0},
	}

	if wire, err := EncodeGradPointsData(nil); err != nil || !bytes.Equal(wire, []byte{0xc0}) {
		t.Fatalf("nil GradPointsData encoded as % x with error %v, want c0", wire, err)
	}

	nilLists := []struct {
		name   string
		mutate func(*GradPointsData)
	}{
		{name: "gradPointParam", mutate: func(v *GradPointsData) { v.GradPointParam = nil }},
		{name: "controlPointPosValue", mutate: func(v *GradPointsData) { v.ControlPointPosValue = nil }},
		{name: "gradaPointPosRates", mutate: func(v *GradPointsData) { v.GradaPointPosRates = nil }},
		{name: "pointRangeAfterRates", mutate: func(v *GradPointsData) { v.PointRangeAfterRates = nil }},
		{name: "pointRangeBeforeRates", mutate: func(v *GradPointsData) { v.PointRangeBeforeRates = nil }},
	}
	for _, test := range nilLists {
		t.Run("nil "+test.name, func(t *testing.T) {
			value := gradTestClone(valid)
			test.mutate(value)
			wire, err := EncodeGradPointsData(value)
			if err != nil {
				t.Fatalf("EncodeGradPointsData: %v", err)
			}
			got, err := DecodeGradPointsData(wire)
			if err != nil || !reflect.DeepEqual(got, value) {
				t.Fatalf("nil collection round trip = %#v, %v; want %#v", got, err, value)
			}
		})
	}

	t.Run("nil color map", func(t *testing.T) {
		value := gradTestClone(valid)
		value.GradPointParam[0] = nil
		wire, err := EncodeGradPointsData(value)
		if err != nil {
			t.Fatalf("EncodeGradPointsData: %v", err)
		}
		got, err := DecodeGradPointsData(wire)
		if err != nil || got.GradPointParam[0] != nil {
			t.Fatalf("nil map round trip = %#v, %v", got, err)
		}
	})
	t.Run("missing business key is retained", func(t *testing.T) {
		value := gradTestClone(valid)
		delete(value.GradPointParam[0], 8)
		wire, err := EncodeGradPointsData(value)
		if err != nil {
			t.Fatalf("EncodeGradPointsData: %v", err)
		}
		got, err := DecodeGradPointsData(wire)
		if err != nil {
			t.Fatalf("DecodeGradPointsData: %v", err)
		}
		if _, exists := got.GradPointParam[0][8]; exists {
			t.Fatalf("missing key was synthesized: %#v", got.GradPointParam[0])
		}
	})
	t.Run("extra key is retained", func(t *testing.T) {
		value := gradTestClone(valid)
		value.GradPointParam[0][9] = 1
		wire, err := EncodeGradPointsData(value)
		if err != nil {
			t.Fatalf("EncodeGradPointsData: %v", err)
		}
		got, err := DecodeGradPointsData(wire)
		if err != nil {
			t.Fatalf("DecodeGradPointsData: %v", err)
		}
		if got.GradPointParam[0][9] != 1 {
			t.Fatalf("extra dictionary key was not retained: %#v", got.GradPointParam[0])
		}
	})

	for _, bound := range []int32{math.MinInt32, math.MaxInt32} {
		value := gradTestClone(valid)
		value.EditMPN = bound
		value.IsSave = bound
		value.GradPointParam[0][0] = bound
		wire, err := EncodeGradPointsData(value)
		if err != nil {
			t.Fatalf("encode valid Int32 boundary %d: %v", bound, err)
		}
		got, err := DecodeGradPointsData(wire)
		if err != nil {
			t.Fatalf("decode valid Int32 boundary %d: %v", bound, err)
		}
		if got.EditMPN != bound || got.IsSave != bound || got.GradPointParam[0][0] != bound {
			t.Fatalf("Int32 boundary %d did not round trip: %#v", bound, got)
		}
	}
}

func gradTestValidWire() []byte {
	// This is deliberately assembled without calling the production encoder.
	// The map is in reverse key order to prove the decoder does not depend on
	// the deterministic ascending order emitted by EncodeGradPointsData.
	wire := []byte{0x97, 0x91, 0x89}
	for key := 8; key >= 0; key-- {
		wire = append(wire, byte(key), byte((key+1)*10))
	}
	wire = append(wire, gradTestFloatList(0.25)...)
	wire = append(wire, gradTestFloatList(-1.5, 2.5)...)
	wire = append(wire, 0xd2, 0x80, 0, 0, 0)
	wire = append(wire, gradTestFloatList(0.75)...)
	wire = append(wire, gradTestFloatList(1)...)
	wire = append(wire, 0xd2, 0x7f, 0xff, 0xff, 0xff)
	return wire
}

func gradTestCanonicalWire() []byte {
	wire := []byte{0x97, 0x91, 0x89}
	for key := 0; key <= 8; key++ {
		wire = append(wire, byte(key), byte((key+1)*10))
	}
	wire = append(wire, gradTestFloatList(0.25)...)
	wire = append(wire, gradTestFloatList(-1.5, 2.5)...)
	wire = append(wire, 0xd2, 0x80, 0, 0, 0)
	wire = append(wire, gradTestFloatList(0.75)...)
	wire = append(wire, gradTestFloatList(1)...)
	wire = append(wire, 0xce, 0x7f, 0xff, 0xff, 0xff)
	return wire
}

func gradTestFloatList(values ...float32) []byte {
	if len(values) > 15 {
		panic("gradTestFloatList only supports fixarray")
	}
	wire := []byte{0x90 | byte(len(values))}
	for _, value := range values {
		var bits [4]byte
		binary.BigEndian.PutUint32(bits[:], math.Float32bits(value))
		wire = append(wire, 0xca)
		wire = append(wire, bits[:]...)
	}
	return wire
}

func gradTestWireWithSlot(slot int, replacement []byte) []byte {
	slots := [][]byte{
		gradTestMapList(9, nil),
		gradTestFloatList(0.25),
		gradTestFloatList(-1.5, 2.5),
		{0xd2, 0x80, 0, 0, 0},
		gradTestFloatList(0.75),
		gradTestFloatList(1),
		{0xd2, 0x7f, 0xff, 0xff, 0xff},
	}
	slots[slot] = replacement
	wire := []byte{0x97}
	for _, value := range slots {
		wire = append(wire, value...)
	}
	return wire
}

func gradTestMapList(count int, keyForIndex func(int) int) []byte {
	if count > 15 {
		panic("gradTestMapList only supports fixmap")
	}
	wire := []byte{0x91, 0x80 | byte(count)}
	for i := 0; i < count; i++ {
		key := i
		if keyForIndex != nil {
			key = keyForIndex(i)
		}
		wire = append(wire, byte(key), byte(i+10))
	}
	return wire
}

func gradTestMapListBadValue() []byte {
	wire := []byte{0x91, 0x89}
	for i := 0; i < 9; i++ {
		wire = append(wire, byte(i))
		if i == 8 {
			wire = append(wire, 0xc3)
		} else {
			wire = append(wire, byte(i))
		}
	}
	return wire
}

func gradTestMapListOverflowKey() []byte {
	wire := []byte{0x91, 0x89}
	for i := 0; i < 9; i++ {
		if i == 8 {
			wire = append(wire, 0xce, 0x80, 0, 0, 0)
		} else {
			wire = append(wire, byte(i))
		}
		wire = append(wire, byte(i))
	}
	return wire
}

func gradTestMapListOverflowValue() []byte {
	wire := []byte{0x91, 0x89}
	for i := 0; i < 9; i++ {
		wire = append(wire, byte(i))
		if i == 8 {
			wire = append(wire, 0xce, 0x80, 0, 0, 0)
		} else {
			wire = append(wire, byte(i))
		}
	}
	return wire
}

func gradTestColorMap() map[int32]int32 {
	value := make(map[int32]int32, 9)
	for i := int32(0); i < 9; i++ {
		value[i] = i
	}
	return value
}

func gradTestClone(src *GradPointsData) *GradPointsData {
	if src == nil {
		return nil
	}
	dst := *src
	dst.GradPointParam = make([]map[int32]int32, len(src.GradPointParam))
	for i, values := range src.GradPointParam {
		if values == nil {
			continue
		}
		dst.GradPointParam[i] = make(map[int32]int32, len(values))
		for key, value := range values {
			dst.GradPointParam[i][key] = value
		}
	}
	dst.ControlPointPosValue = append([]float32{}, src.ControlPointPosValue...)
	dst.GradaPointPosRates = append([]float32{}, src.GradaPointPosRates...)
	dst.PointRangeAfterRates = append([]float32{}, src.PointRangeAfterRates...)
	dst.PointRangeBeforeRates = append([]float32{}, src.PointRangeBeforeRates...)
	return &dst
}
