package KCES

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestMoveablePanelSaveDataHandwrittenWireRoundTrip(t *testing.T) {
	wire := moveableTestDataWire(
		[][]byte{
			moveableTestPosition("PosePanel", moveableTestVector3(
				moveableTestFloat32(1.25),
				moveableTestFloat32(-2.5),
				moveableTestFloat32(3),
			)),
			moveableTestPosition("HairLengthPanel", moveableTestVector3(
				moveableTestFloat32(0),
				moveableTestFloat32(float32(math.Copysign(0, -1))),
				moveableTestFloat32(99.5),
			)),
		},
		[][]byte{
			moveableTestActive("HairLengthPanel", true),
			moveableTestActive("PosePanel", false),
		},
	)

	got, err := DecodeMoveablePanelSaveData(wire)
	if err != nil {
		t.Fatalf("DecodeMoveablePanelSaveData: %v", err)
	}
	want := &MoveablePanelSaveData{
		MoveablePanelPosition: []MoveablePanelPositionEntry{
			{PanelName: "PosePanel", Position: Vector3{X: 1.25, Y: -2.5, Z: 3}},
			{PanelName: "HairLengthPanel", Position: Vector3{X: 0, Y: float32(math.Copysign(0, -1)), Z: 99.5}},
		},
		MoveablePanelActiveState: []MoveablePanelActiveStateEntry{
			{PanelName: "HairLengthPanel", Active: true},
			{PanelName: "PosePanel", Active: false},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded value\n got: %#v\nwant: %#v", got, want)
	}
	if !math.Signbit(float64(got.MoveablePanelPosition[1].Position.Y)) {
		t.Fatal("negative zero float32 bits were not preserved")
	}

	snapshot := cloneMoveableTestData(got)
	encoded, err := EncodeMoveablePanelSaveData(got)
	if err != nil {
		t.Fatalf("EncodeMoveablePanelSaveData: %v", err)
	}
	if !bytes.Equal(encoded, wire) {
		t.Fatalf("encoded wire differs\n got: %x\nwant: %x", encoded, wire)
	}
	if !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("encoder modified caller\n got: %#v\nwant: %#v", got, snapshot)
	}
}

func TestMoveablePanelSaveDataCanonicalHeaderBoundaries(t *testing.T) {
	names := []string{
		strings.Repeat("a", 31),
		strings.Repeat("b", 32),
		strings.Repeat("c", 255),
		strings.Repeat("d", 256),
	}
	for index := len(names); index < 16; index++ {
		names = append(names, fmt.Sprintf("Panel%02d", index))
	}

	value := &MoveablePanelSaveData{
		MoveablePanelPosition:    make([]MoveablePanelPositionEntry, 0, len(names)),
		MoveablePanelActiveState: make([]MoveablePanelActiveStateEntry, 0, len(names)),
	}
	positionWire := make([][]byte, 0, len(names))
	activeWire := make([][]byte, 0, len(names))
	for index, name := range names {
		position := Vector3{X: float32(index) + 0.25, Y: -float32(index), Z: float32(index * index)}
		value.MoveablePanelPosition = append(value.MoveablePanelPosition, MoveablePanelPositionEntry{
			PanelName: name,
			Position:  position,
		})
		positionWire = append(positionWire, moveableTestPosition(name, moveableTestVector3(
			moveableTestFloat32(position.X),
			moveableTestFloat32(position.Y),
			moveableTestFloat32(position.Z),
		)))
	}
	// Reverse only the active-state order. Both lists are ordered independently.
	for index := len(names) - 1; index >= 0; index-- {
		entry := MoveablePanelActiveStateEntry{PanelName: names[index], Active: index%2 == 0}
		value.MoveablePanelActiveState = append(value.MoveablePanelActiveState, entry)
		activeWire = append(activeWire, moveableTestActive(entry.PanelName, entry.Active))
	}

	wantWire := moveableTestDataWire(positionWire, activeWire)
	encoded, err := EncodeMoveablePanelSaveData(value)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.Equal(encoded, wantWire) {
		t.Fatalf("boundary wire differs\n got: %x\nwant: %x", encoded, wantWire)
	}
	if len(encoded) < 4 || encoded[0] != 0x92 || encoded[1] != 0xdc || encoded[2] != 0 || encoded[3] != 16 {
		t.Fatalf("outer/list array headers = %x, want 92 dc0010", encoded[:min(len(encoded), 4)])
	}
	decoded, err := DecodeMoveablePanelSaveData(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("boundary round trip\n got: %#v\nwant: %#v", decoded, value)
	}
}

func TestMoveablePanelSaveDataAllowsIndependentNameSets(t *testing.T) {
	tests := []struct {
		name string
		wire []byte
		want *MoveablePanelSaveData
	}{
		{
			name: "position without active state",
			wire: moveableTestDataWire(
				[][]byte{moveableTestPosition("PositionOnly", moveableTestVector3(
					moveableTestFloat32(1), moveableTestFloat32(2), moveableTestFloat32(3),
				))},
				[][]byte{},
			),
			want: &MoveablePanelSaveData{
				MoveablePanelPosition: []MoveablePanelPositionEntry{{
					PanelName: "PositionOnly",
					Position:  Vector3{X: 1, Y: 2, Z: 3},
				}},
				MoveablePanelActiveState: []MoveablePanelActiveStateEntry{},
			},
		},
		{
			name: "active state without position",
			wire: moveableTestDataWire(
				[][]byte{},
				[][]byte{moveableTestActive("ActiveOnly", true)},
			),
			want: &MoveablePanelSaveData{
				MoveablePanelPosition: []MoveablePanelPositionEntry{},
				MoveablePanelActiveState: []MoveablePanelActiveStateEntry{{
					PanelName: "ActiveOnly",
					Active:    true,
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := DecodeMoveablePanelSaveData(tt.wire)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !reflect.DeepEqual(decoded, tt.want) {
				t.Fatalf("decoded\n got: %#v\nwant: %#v", decoded, tt.want)
			}
			encoded, err := EncodeMoveablePanelSaveData(decoded)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !bytes.Equal(encoded, tt.wire) {
				t.Fatalf("wire changed\n got: %x\nwant: %x", encoded, tt.wire)
			}
		})
	}
}

func TestMoveablePanelSaveDataDecoderMatchesGameVector3Tolerance(t *testing.T) {
	tests := []struct {
		name           string
		vec            []byte
		encodedVec     []byte
		want           Vector3
		wantFieldCount int32
		wantFuture     int
	}{
		{
			name:           "empty vector defaults all components",
			vec:            moveableTestVector3(),
			encodedVec:     moveableTestVector3(),
			want:           Vector3{},
			wantFieldCount: 0,
		},
		{
			name:           "missing components default to zero",
			vec:            moveableTestVector3(moveableTestFloat32(4.5)),
			encodedVec:     moveableTestVector3(moveableTestFloat32(4.5)),
			want:           Vector3{X: 4.5},
			wantFieldCount: 1,
		},
		{
			name: "ReadSingle compatible numeric codes",
			vec: moveableTestVector3(
				moveableTestFloat64(1.5),
				[]byte{0xfe}, // negative fixint -2
				[]byte{0xcc, 3},
			),
			encodedVec: moveableTestVector3(
				moveableTestFloat32(1.5),
				moveableTestFloat32(-2),
				moveableTestFloat32(3),
			),
			want:           Vector3{X: 1.5, Y: -2, Z: 3},
			wantFieldCount: 3,
		},
		{
			name: "extra values are skipped",
			vec: moveableTestVector3(
				moveableTestFloat32(1),
				moveableTestFloat32(2),
				moveableTestFloat32(3),
				[]byte{0x92, 0x01, 0x81, 0xa1, 'x', 0xc3},
			),
			encodedVec: moveableTestVector3(
				moveableTestFloat32(1),
				moveableTestFloat32(2),
				moveableTestFloat32(3),
				[]byte{0x92, 0x01, 0x81, 0xa1, 'x', 0xc3},
			),
			want:           Vector3{X: 1, Y: 2, Z: 3},
			wantFieldCount: 4,
			wantFuture:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := moveableTestDataWire(
				[][]byte{moveableTestPosition("Panel", tt.vec)},
				[][]byte{moveableTestActive("Panel", true)},
			)
			got, err := DecodeMoveablePanelSaveData(wire)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			position := got.MoveablePanelPosition[0].Position
			if position.X != tt.want.X || position.Y != tt.want.Y || position.Z != tt.want.Z {
				t.Fatalf("position = %#v, want components %#v", position, tt.want)
			}
			if tt.wantFieldCount == 3 {
				if position.IndexedObjectMetadata != nil {
					t.Fatalf("canonical Vector3 unexpectedly has metadata: %#v", position.IndexedObjectMetadata)
				}
			} else if position.IndexedObjectMetadata == nil || position.FieldCount == nil || *position.FieldCount != tt.wantFieldCount {
				t.Fatalf("Vector3 field count was not preserved: %#v", position.IndexedObjectMetadata)
			}
			if tt.wantFuture != 0 {
				if position.IndexedObjectMetadata == nil || len(position.FutureSlots) != tt.wantFuture {
					t.Fatalf("Vector3 future slots were not preserved: %#v", position.IndexedObjectMetadata)
				}
			}

			encoded, err := EncodeMoveablePanelSaveData(got)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			// Known numeric values use the normal float32 writer, while the decoded
			// Vector3 width and raw future components remain unchanged.
			wantWire := moveableTestDataWire(
				[][]byte{moveableTestPosition("Panel", tt.encodedVec)},
				[][]byte{moveableTestActive("Panel", true)},
			)
			if !bytes.Equal(encoded, wantWire) {
				t.Fatalf("preserved wire\n got: %x\nwant: %x", encoded, wantWire)
			}
		})
	}
}

func TestMoveablePanelSaveDataDecoderMatchesIndexedObjectTolerance(t *testing.T) {
	tests := []struct {
		name            string
		wire            []byte
		wantPositionNil bool
		wantActiveNil   bool
	}{
		{name: "zero fields", wire: []byte{0x90}, wantPositionNil: true, wantActiveNil: true},
		{name: "one empty position field", wire: []byte{0x91, 0x90}, wantActiveNil: true},
		{name: "unknown field preserved", wire: []byte{0x93, 0x90, 0x90, 0x81, 0xa1, 'x', 0x92, 0x01, 0x02}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeMoveablePanelSaveData(tt.wire)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if (got.MoveablePanelPosition == nil) != tt.wantPositionNil || (got.MoveablePanelActiveState == nil) != tt.wantActiveNil {
				t.Fatalf("missing-field nil state changed: %#v", got)
			}
			if len(got.MoveablePanelPosition) != 0 || len(got.MoveablePanelActiveState) != 0 {
				t.Fatalf("unexpected entries: %#v", got)
			}
			wantFieldCount := int32(tt.wire[0] & 0x0f)
			if got.FieldCount == nil || *got.FieldCount != wantFieldCount || len(got.FutureSlots) != max(0, int(wantFieldCount)-2) {
				t.Fatalf("indexed-object shape was not retained: %#v", got)
			}
			reencoded, err := EncodeMoveablePanelSaveData(got)
			if err != nil || !bytes.Equal(reencoded, tt.wire) {
				t.Fatalf("indexed-object wire changed: equal=%v err=%v", bytes.Equal(reencoded, tt.wire), err)
			}
		})
	}
}

func TestMoveablePanelSaveDataRejectsMalformedWire(t *testing.T) {
	validPositions := moveableTestArray(moveableTestPosition("Panel", moveableTestVector3(
		moveableTestFloat32(1), moveableTestFloat32(2), moveableTestFloat32(3),
	)))
	validActive := moveableTestArray(moveableTestActive("Panel", true))

	tests := []struct {
		name string
		wire []byte
		want string
	}{
		{name: "wrong top type", wire: []byte{0x80}, want: "array"},
		{name: "position KVP too short", wire: moveableTestArray(moveableTestArray(moveableTestArray(moveableTestString("Panel"))), validActive), want: "array(2)"},
		{name: "position KVP too long", wire: moveableTestArray(moveableTestArray(moveableTestArray(moveableTestString("Panel"), moveableTestVector3(), []byte{0xc0})), validActive), want: "array(2)"},
		{name: "active KVP too short", wire: moveableTestArray(validPositions, moveableTestArray(moveableTestArray(moveableTestString("Panel")))), want: "array(2)"},
		{name: "binary name", wire: moveableTestArray(moveableTestArray(moveableTestArray([]byte{0xc4, 0x01, 'P'}, moveableTestVector3())), validActive), want: "string"},
		{name: "nil Vector3", wire: moveableTestArray(moveableTestArray(moveableTestArray(moveableTestString("Panel"), []byte{0xc0})), validActive), want: "Vector3"},
		{name: "wrong Vector3 type", wire: moveableTestArray(moveableTestArray(moveableTestArray(moveableTestString("Panel"), []byte{0x80})), validActive), want: "Vector3"},
		{name: "nil coordinate", wire: moveableTestArray(moveableTestArray(moveableTestArray(moveableTestString("Panel"), moveableTestVector3([]byte{0xc0}))), validActive), want: "requires a number"},
		{name: "wrong coordinate type", wire: moveableTestArray(moveableTestArray(moveableTestArray(moveableTestString("Panel"), moveableTestVector3([]byte{0xc3}))), validActive), want: "number"},
		{name: "wrong active type", wire: moveableTestArray(validPositions, moveableTestArray(moveableTestArray(moveableTestString("Panel"), []byte{0x01}))), want: "bool"},
		{name: "truncated string", wire: []byte{0x92, 0x91, 0x92, 0xa5, 'P'}, want: "EOF"},
		{name: "truncated float", wire: []byte{0x92, 0x91, 0x92, 0xa1, 'P', 0x91, 0xca, 0x00}, want: "EOF"},
		{name: "truncated collection", wire: []byte{0x92, 0xdc, 0x00, 0x01}, want: "EOF"},
		{name: "large truncated collection", wire: []byte{0x92, 0xdd, 0x00, 0x10, 0x00, 0x01}, want: "EOF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeMoveablePanelSaveData(tt.wire)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}

	invalidUTF8Wire := moveableTestArray(moveableTestArray(moveableTestArray([]byte{0xa1, 0xff}, moveableTestVector3())), validActive)
	decoded, err := DecodeMoveablePanelSaveData(invalidUTF8Wire)
	if err != nil {
		t.Fatalf("MessagePack-CSharp-readable malformed UTF-8 was rejected: %v", err)
	}
	if decoded.MoveablePanelPosition[0].PanelName != "\uFFFD" {
		t.Fatalf("invalid UTF-8 replacement = %q, want U+FFFD", decoded.MoveablePanelPosition[0].PanelName)
	}
}

func TestMoveablePanelSaveDataPreservesWireValuesWithoutRuntimePolicy(t *testing.T) {
	values := []*MoveablePanelSaveData{
		nil,
		{},
		{MoveablePanelPosition: []MoveablePanelPositionEntry{}, MoveablePanelActiveState: nil},
		{
			MoveablePanelPosition: []MoveablePanelPositionEntry{
				{PanelName: "Panel", Position: Vector3{X: float32(math.Inf(1))}},
				{PanelNameIsNil: true, Position: Vector3{X: math.Float32frombits(0x7fc12345)}},
			},
			MoveablePanelActiveState: []MoveablePanelActiveStateEntry{
				{PanelName: "Panel", Active: true},
				{PanelNameIsNil: true},
			},
		},
	}
	for index, value := range values {
		wire, err := EncodeMoveablePanelSaveData(value)
		if err != nil {
			t.Fatalf("case %d encode: %v", index, err)
		}
		got, err := DecodeMoveablePanelSaveData(wire)
		if err != nil {
			t.Fatalf("case %d decode: %v", index, err)
		}
		if value == nil {
			if got != nil {
				t.Fatalf("root nil became %#v", got)
			}
			continue
		}
		if len(value.MoveablePanelPosition) == 2 {
			if !math.IsInf(float64(got.MoveablePanelPosition[0].Position.X), 1) ||
				math.Float32bits(got.MoveablePanelPosition[1].Position.X) != 0x7fc12345 ||
				!got.MoveablePanelPosition[1].PanelNameIsNil || !got.MoveablePanelActiveState[1].PanelNameIsNil {
				t.Fatalf("special wire values changed: %#v", got)
			}
		} else if !reflect.DeepEqual(got, value) {
			t.Fatalf("case %d round trip = %#v, want %#v", index, got, value)
		}
	}

	invalidUTF8 := &MoveablePanelSaveData{MoveablePanelPosition: []MoveablePanelPositionEntry{{PanelName: "\xff"}}}
	if _, err := EncodeMoveablePanelSaveData(invalidUTF8); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("invalid UTF-8 error = %v", err)
	}
	contradictoryNil := &MoveablePanelSaveData{MoveablePanelPosition: []MoveablePanelPositionEntry{{PanelName: "Panel", PanelNameIsNil: true}}}
	if _, err := EncodeMoveablePanelSaveData(contradictoryNil); err == nil || !strings.Contains(err.Error(), "would discard") {
		t.Fatalf("nil name silently discarded its string value: %v", err)
	}
}

func TestMoveablePanelSaveDataAllowsEmptyButNonNilName(t *testing.T) {
	wire := moveableTestDataWire(
		[][]byte{moveableTestPosition("", moveableTestVector3())},
		[][]byte{moveableTestActive("", false)},
	)
	got, err := DecodeMoveablePanelSaveData(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	encoded, err := EncodeMoveablePanelSaveData(got)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.Equal(encoded, wire) {
		t.Fatalf("empty name/vector changed\n got: %x\nwant: %x", encoded, wire)
	}
}

func cloneMoveableTestData(value *MoveablePanelSaveData) *MoveablePanelSaveData {
	clone := *value
	clone.MoveablePanelPosition = append([]MoveablePanelPositionEntry(nil), value.MoveablePanelPosition...)
	clone.MoveablePanelActiveState = append([]MoveablePanelActiveStateEntry(nil), value.MoveablePanelActiveState...)
	return &clone
}

func moveableTestDataWire(positions, active [][]byte) []byte {
	return moveableTestArray(moveableTestArray(positions...), moveableTestArray(active...))
}

func moveableTestPosition(name string, vector []byte) []byte {
	return moveableTestArray(moveableTestString(name), vector)
}

func moveableTestActive(name string, active bool) []byte {
	b := byte(0xc2)
	if active {
		b = 0xc3
	}
	return moveableTestArray(moveableTestString(name), []byte{b})
}

func moveableTestVector3(values ...[]byte) []byte {
	return moveableTestArray(values...)
}

func moveableTestArray(values ...[]byte) []byte {
	var out []byte
	switch {
	case len(values) < 16:
		out = append(out, 0x90|byte(len(values)))
	case len(values) <= math.MaxUint16:
		out = append(out, 0xdc, byte(len(values)>>8), byte(len(values)))
	default:
		out = append(out, 0xdd, byte(len(values)>>24), byte(len(values)>>16), byte(len(values)>>8), byte(len(values)))
	}
	for _, value := range values {
		out = append(out, value...)
	}
	return out
}

func moveableTestString(value string) []byte {
	b := []byte(value)
	var out []byte
	switch {
	case len(b) < 32:
		out = append(out, 0xa0|byte(len(b)))
	case len(b) <= math.MaxUint8:
		out = append(out, 0xd9, byte(len(b)))
	case len(b) <= math.MaxUint16:
		out = append(out, 0xda, byte(len(b)>>8), byte(len(b)))
	default:
		out = append(out, 0xdb, byte(len(b)>>24), byte(len(b)>>16), byte(len(b)>>8), byte(len(b)))
	}
	return append(out, b...)
}

func moveableTestFloat32(value float32) []byte {
	out := make([]byte, 5)
	out[0] = 0xca
	binary.BigEndian.PutUint32(out[1:], math.Float32bits(value))
	return out
}

func moveableTestFloat64(value float64) []byte {
	out := make([]byte, 9)
	out[0] = 0xcb
	binary.BigEndian.PutUint64(out[1:], math.Float64bits(value))
	return out
}
