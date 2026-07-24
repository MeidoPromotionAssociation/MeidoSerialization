package KCES

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestMoveablePanelSaveDataTypedRoundTrip(t *testing.T) {
	pose := "PosePanel"
	hair := "HairLengthPanel"
	value := &MoveablePanelSaveData{
		MoveablePanelPosition: []MoveablePanelPositionEntry{
			{PanelName: &pose, Position: Vector3{X: 1.25, Y: -2.5, Z: 3}},
			{PanelName: nil, Position: Vector3{X: math.Float32frombits(0x7fc12345), Y: math.Float32frombits(0x80000000), Z: 99.5}},
		},
		MoveablePanelActiveState: []MoveablePanelActiveStateEntry{
			{PanelName: &hair, Active: true},
			{PanelName: nil, Active: false},
		},
	}
	wire, err := EncodeMoveablePanelSaveData(value)
	if err != nil {
		t.Fatalf("EncodeMoveablePanelSaveData() error = %v", err)
	}
	decoded, err := DecodeMoveablePanelSaveData(wire)
	if err != nil {
		t.Fatalf("DecodeMoveablePanelSaveData() error = %v", err)
	}
	roundTripWire, err := EncodeMoveablePanelSaveData(decoded)
	if err != nil {
		t.Fatalf("re-encode decoded MoveablePanelSaveData: %v", err)
	}
	if !bytes.Equal(roundTripWire, wire) {
		t.Fatalf("round-trip wire changed:\n got  %x\n want %x", roundTripWire, wire)
	}
	if math.Float32bits(decoded.MoveablePanelPosition[1].Position.X) != 0x7fc12345 || math.Float32bits(decoded.MoveablePanelPosition[1].Position.Y) != 0x80000000 {
		t.Fatal("Vector3 float32 bit patterns changed")
	}
}

func TestMoveablePanelSaveDataNullableRootListsAndNames(t *testing.T) {
	wire, err := EncodeMoveablePanelSaveData(nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeMoveablePanelSaveData(wire)
	if err != nil || decoded != nil {
		t.Fatalf("nil root = %#v, %v", decoded, err)
	}
	value := &MoveablePanelSaveData{
		MoveablePanelPosition: []MoveablePanelPositionEntry{{PanelName: nil}},
	}
	wire, err = EncodeMoveablePanelSaveData(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = DecodeMoveablePanelSaveData(wire)
	if err != nil || decoded.MoveablePanelPosition[0].PanelName != nil || decoded.MoveablePanelActiveState != nil {
		t.Fatalf("typed nullability changed: %#v, %v", decoded, err)
	}
}

func TestMoveablePanelSaveDataRejectsUnsupportedLayouts(t *testing.T) {
	validPosition := []interface{}{nil, []interface{}{float32(1), float32(2), float32(3)}}
	validActive := []interface{}{nil, true}
	tests := []struct {
		name string
		root interface{}
		want string
	}{
		{name: "short root", root: []interface{}{[]interface{}{}}, want: "indexed-array width 1, expected 2"},
		{name: "high root slot", root: []interface{}{[]interface{}{}, []interface{}{}, nil}, want: "indexed-array width 3, expected 2"},
		{name: "short position pair", root: []interface{}{[]interface{}{[]interface{}{nil}}, []interface{}{}}, want: "array(2)"},
		{name: "long position pair", root: []interface{}{[]interface{}{append(validPosition, nil)}, []interface{}{}}, want: "array(2)"},
		{name: "short vector", root: []interface{}{[]interface{}{[]interface{}{nil, []interface{}{float32(1), float32(2)}}}, []interface{}{}}, want: "Vector3 indexed-array width 2, expected 3"},
		{name: "long vector", root: []interface{}{[]interface{}{[]interface{}{nil, []interface{}{float32(1), float32(2), float32(3), float32(4)}}}, []interface{}{}}, want: "Vector3 indexed-array width 4, expected 3"},
		{name: "nil vector coordinate", root: []interface{}{[]interface{}{[]interface{}{nil, []interface{}{nil, float32(2), float32(3)}}}, []interface{}{}}, want: "number"},
		{name: "long active pair", root: []interface{}{[]interface{}{}, []interface{}{append(validActive, nil)}}, want: "array(2)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire, err := ct.EncodeMsgpack(test.root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeMoveablePanelSaveData(wire); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeMoveablePanelSaveData() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestMoveablePanelSaveDataRejectsTrailingData(t *testing.T) {
	wire, err := EncodeMoveablePanelSaveData(&MoveablePanelSaveData{})
	if err != nil {
		t.Fatal(err)
	}
	wire = append(wire, 0xc0)
	if _, err := DecodeMoveablePanelSaveData(wire); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("DecodeMoveablePanelSaveData() error = %v, want trailing data", err)
	}
}
