package KCES

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestKCES2GradPointsDataRoundTripUsesIndependentSixSlotLayout(t *testing.T) {
	value := NewKCES2GradPointsData(true)
	value.GradPointParam = []map[int32]int32{{0: 10, 8: 20}}
	value.ControlPointPosValue = []float32{0.5}
	value.GradationPointPositionRates = []float32{0.25}
	value.PointRangeAfterRates = []float32{0.1}
	value.PointRangeBeforeRates = []float32{0.2}

	wire, err := EncodeKCES2GradPointsData(value)
	if err != nil {
		t.Fatalf("EncodeKCES2GradPointsData: %v", err)
	}
	if got := rawArrayWidth(t, wire); got != 6 {
		t.Fatalf("KCES2GradPointsData width = %d, want 6", got)
	}
	decoded, err := DecodeKCES2GradPointsData(wire)
	if err != nil {
		t.Fatalf("DecodeKCES2GradPointsData: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("KCES2GradPointsData round trip mismatch\n got: %#v\nwant: %#v", decoded, value)
	}
}

func TestKCES2GradPointsDataSystemDataRouting(t *testing.T) {
	value := NewKCES2GradPointsData(true)
	value.GradPointParam = []map[int32]int32{{0: 11, 8: 22}}
	path := "EditData/KCES2GradSv12"
	if got := KCESEditDataKindForPath(path); got != KCESEditDataKCES2GradPoints {
		t.Fatalf("KCESEditDataKindForPath(%q) = %q, want %q", path, got, KCESEditDataKCES2GradPoints)
	}
	if got := KCESEditDataKindForPath("EditData/KCES2GradSv"); got != "" {
		t.Fatalf("KCESEditDataKindForPath without index = %q, want empty", got)
	}

	systemData := NewKCESSystemData()
	systemData.EditData = []KCESEditDataFile{{
		Path:            path,
		Kind:            KCESEditDataKCES2GradPoints,
		KCES2GradPoints: value,
	}}
	wire, err := EncodeKCESSystemData(systemData)
	if err != nil {
		t.Fatalf("EncodeKCESSystemData: %v", err)
	}
	decoded, err := DecodeKCESSystemData(wire)
	if err != nil {
		t.Fatalf("DecodeKCESSystemData: %v", err)
	}
	if len(decoded.EditData) != 1 || decoded.EditData[0].Kind != KCESEditDataKCES2GradPoints || !reflect.DeepEqual(decoded.EditData[0].KCES2GradPoints, value) {
		t.Fatalf("decoded KCES2 gradation route = %#v", decoded.EditData)
	}

	jsonData, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("Marshal system.dat JSON: %v", err)
	}
	var edited KCESSystemData
	if err := json.Unmarshal(jsonData, &edited); err != nil {
		t.Fatalf("Unmarshal system.dat JSON: %v", err)
	}
	reencoded, err := EncodeKCESSystemData(&edited)
	if err != nil {
		t.Fatalf("re-encode KCESSystemData: %v", err)
	}
	roundTrip, err := DecodeKCESSystemData(reencoded)
	if err != nil {
		t.Fatalf("decode re-encoded KCESSystemData: %v", err)
	}
	if !reflect.DeepEqual(roundTrip.EditData, decoded.EditData) {
		t.Fatalf("KCES2 gradation system-data round trip changed\n got: %#v\nwant: %#v", roundTrip.EditData, decoded.EditData)
	}
	if !bytes.Contains(jsonData, []byte(`"gradationPointPositionRates"`)) || bytes.Contains(jsonData, []byte(`"gradaPointPosRates"`)) {
		t.Fatalf("gradation editing JSON uses an incorrect field spelling: %s", jsonData)
	}
}
