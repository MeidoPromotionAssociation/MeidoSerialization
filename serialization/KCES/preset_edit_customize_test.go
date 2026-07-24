package KCES

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestKCESPresetEditCustomizeDataTypedRoundTrip(t *testing.T) {
	id := "embedded"
	warpoint := "HairPoint"
	flagA := "first"
	flagZ := "last"
	colorPreset, err := NewColorPreset("12345678-1234-1234-1234-123456789abc")
	if err != nil {
		t.Fatal(err)
	}
	base := &KCESPresetEditBaseData{
		Version: -100,
		ColorPreset: &KCESPresetEditColorPreset{
			ID:               &id,
			SerializedPreset: colorPreset,
		},
		Flags: map[string]*string{"z": &flagZ, "a": &flagA, "nullable": nil},
	}
	wire, err := EncodeKCESPresetEditBaseData(base)
	if err != nil {
		t.Fatalf("EncodeKCESPresetEditBaseData: %v", err)
	}
	decoded, err := DecodeKCESPresetEditBaseData(wire)
	if err != nil {
		t.Fatalf("DecodeKCESPresetEditBaseData: %v", err)
	}
	if !reflect.DeepEqual(decoded, base) {
		t.Fatalf("BaseData changed:\n got  %#v\n want %#v", decoded, base)
	}
	reencoded, err := EncodeKCESPresetEditBaseData(decoded)
	if err != nil {
		t.Fatalf("re-encode BaseData: %v", err)
	}
	redecoded, err := DecodeKCESPresetEditBaseData(reencoded)
	if err != nil || !reflect.DeepEqual(redecoded, decoded) {
		t.Fatalf("BaseData semantic second round trip: got=%#v err=%v want=%#v", redecoded, err, decoded)
	}

	jsonData, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(jsonData, []byte("serializeBinary")) || !bytes.Contains(jsonData, []byte(`"serializedPreset":{`)) {
		t.Fatalf("BaseData JSON did not expand nested ColorPreset: %s", jsonData)
	}

	unit := &KCESPresetEditUnitData{
		Version:      -200,
		PositionX:    1.25,
		PositionY:    -9.5,
		WarpointName: &warpoint,
	}
	unitWire, err := EncodeKCESPresetEditUnitData(unit)
	if err != nil {
		t.Fatalf("EncodeKCESPresetEditUnitData: %v", err)
	}
	unitDecoded, err := DecodeKCESPresetEditUnitData(unitWire)
	if err != nil || !reflect.DeepEqual(unitDecoded, unit) {
		t.Fatalf("UnitData round trip: got=%#v err=%v want=%#v", unitDecoded, err, unit)
	}
}

func TestKCESPresetEditCustomizeDataNullableRootAndSerializedPreset(t *testing.T) {
	nilBase, err := DecodeKCESPresetEditBaseData([]byte{0xc0})
	if err != nil {
		t.Fatal(err)
	}
	if nilBase != nil {
		t.Fatalf("nil BaseData = %+v", nilBase)
	}
	reencoded, err := EncodeKCESPresetEditBaseData(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reencoded, []byte{0xc0}) {
		t.Fatalf("nil BaseData wire = %x", reencoded)
	}
	if _, err := DecodeKCESPresetEditBaseData([]byte{0xc0, 0x01}); err == nil {
		t.Fatal("BaseData accepted trailing bytes after nil root")
	}

	value := &KCESPresetEditBaseData{
		Version:     KCESPresetEditBaseDataVersion,
		ColorPreset: &KCESPresetEditColorPreset{},
	}
	wire, err := EncodeKCESPresetEditBaseData(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeKCESPresetEditBaseData(wire)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ColorPreset == nil || decoded.ColorPreset.SerializedPreset != nil {
		t.Fatalf("nullable serializedPreset = %+v", decoded.ColorPreset)
	}
}
