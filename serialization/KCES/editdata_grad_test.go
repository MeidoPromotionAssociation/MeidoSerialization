package KCES

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
)

func TestGradPointsDataTypedRoundTrip(t *testing.T) {
	value := &GradPointsData{
		GradPointParam: []map[int32]int32{
			nil,
			{0: 10, 1: 20, 8: 90},
		},
		ControlPointPosValue:  []float32{0.25, math.Float32frombits(0x7fc01234)},
		GradaPointPosRates:    []float32{-1.5, float32(math.Inf(1))},
		EditMPN:               math.MinInt32,
		PointRangeAfterRates:  []float32{0.75},
		PointRangeBeforeRates: []float32{1},
		IsSave:                math.MaxInt32,
	}
	first, err := EncodeGradPointsData(value)
	if err != nil {
		t.Fatalf("EncodeGradPointsData() error = %v", err)
	}
	second, err := EncodeGradPointsData(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("map encoding is not deterministic")
	}
	decoded, err := DecodeGradPointsData(first)
	if err != nil {
		t.Fatalf("DecodeGradPointsData() error = %v", err)
	}
	roundTripWire, err := EncodeGradPointsData(decoded)
	if err != nil {
		t.Fatalf("re-encode decoded GradPointsData: %v", err)
	}
	if !bytes.Equal(roundTripWire, first) {
		t.Fatalf("round-trip wire changed:\n got  %x\n want %x", roundTripWire, first)
	}
	if math.Float32bits(decoded.ControlPointPosValue[1]) != 0x7fc01234 {
		t.Fatal("float32 NaN payload bits changed")
	}
}

func TestGradPointsDataNullableRootAndCollections(t *testing.T) {
	wire, err := EncodeGradPointsData(nil)
	if err != nil || !bytes.Equal(wire, []byte{0xc0}) {
		t.Fatalf("EncodeGradPointsData(nil) = %x, %v", wire, err)
	}
	decoded, err := DecodeGradPointsData(wire)
	if err != nil || decoded != nil {
		t.Fatalf("DecodeGradPointsData(nil) = %#v, %v", decoded, err)
	}
	value := &GradPointsData{}
	wire, err = EncodeGradPointsData(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = DecodeGradPointsData(wire)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.GradPointParam != nil || decoded.ControlPointPosValue != nil || decoded.GradaPointPosRates != nil || decoded.PointRangeAfterRates != nil || decoded.PointRangeBeforeRates != nil {
		t.Fatalf("nil collections changed: %#v", decoded)
	}
}

func TestGradPointsDataRejectsUnsupportedLayouts(t *testing.T) {
	valid := []interface{}{[]interface{}{}, []interface{}{}, []interface{}{}, int64(0), []interface{}{}, []interface{}{}, int64(0)}
	tests := []struct {
		name string
		root []interface{}
		want string
	}{
		{name: "short", root: valid[:6], want: "indexed-array width 6, expected 7"},
		{name: "high slot", root: append(append([]interface{}(nil), valid...), nil), want: "indexed-array width 8, expected 7"},
		{name: "nil int32", root: []interface{}{[]interface{}{}, []interface{}{}, []interface{}{}, nil, []interface{}{}, []interface{}{}, int64(0)}, want: "editMpn"},
		{name: "nil float element", root: []interface{}{[]interface{}{}, []interface{}{nil}, []interface{}{}, int64(0), []interface{}{}, []interface{}{}, int64(0)}, want: "number"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire, err := msgpack.EncodeMsgpack(test.root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeGradPointsData(wire); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeGradPointsData() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestGradPointsDataRejectsTrailingData(t *testing.T) {
	wire, err := EncodeGradPointsData(NewGradPointsData())
	if err != nil {
		t.Fatal(err)
	}
	wire = append(wire, 0xc0)
	if _, err := DecodeGradPointsData(wire); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("DecodeGradPointsData() error = %v, want trailing data", err)
	}
}
