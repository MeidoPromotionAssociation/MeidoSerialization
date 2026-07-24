package KCES

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestPartsColorGradaTypedRoundTrip(t *testing.T) {
	values := []PartsColorGrada{
		{
			MainHue:          math.MinInt32,
			MainChroma:       -2,
			MainBrightness:   -1,
			MainContrast:     0,
			ShadowRate:       1,
			ShadowHue:        2,
			ShadowChroma:     3,
			ShadowBrightness: 4,
			ShadowContrast:   math.MaxInt32,
		},
		{MainHue: 10, MainChroma: 11, MainBrightness: 12, MainContrast: 13, ShadowRate: 14, ShadowHue: 15, ShadowChroma: 16, ShadowBrightness: 17, ShadowContrast: 18},
	}
	encoded, err := EncodePartsColorGrada(values)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePartsColorGrada(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, values) {
		t.Fatalf("gradient changed:\n got  %#v\n want %#v", decoded, values)
	}
}

func TestPartsColorJSONExposesOnlyTypedGradient(t *testing.T) {
	value := PartsColor{
		MainHue: 99,
		Grada:   []PartsColorGrada{{MainHue: 7, ShadowContrast: 9}},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"m_grada"`) {
		t.Fatalf("typed m_grada is missing: %s", text)
	}
	for _, forbidden := range []string{"gradaBytes", "m_gradaBytes", "m_gradaTrailingBytes", "m_gradaDecodeError"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("JSON exposes %q: %s", forbidden, text)
		}
	}
	var decoded PartsColor
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("JSON round trip changed: got %#v want %#v", decoded, value)
	}
}

func TestPartsColorJSONRejectsRemovedGradientFallbackFields(t *testing.T) {
	for _, field := range []string{"m_gradaBytes", "m_gradaTrailingBytes", "m_gradaDecodeError"} {
		var value PartsColor
		data := []byte(`{"` + field + `":null}`)
		if err := json.Unmarshal(data, &value); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("json.Unmarshal(%s) error = %v, want unknown field", data, err)
		}
	}
}

func TestCustomKCESJSONDecodersRejectUnknownFieldsAndTrailingValues(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		target any
	}{
		{name: "dynamic bone unknown", data: `{"unknown":1}`, target: &DynamicBoneStatus{}},
		{name: "cloth unknown", data: `{"unknown":1}`, target: &ClothParams{}},
		{name: "pre-mul unknown", data: `{"unknown":1}`, target: &PreMulTexDatas{}},
		{name: "trans texture unknown", data: `{"unknown":1}`, target: &TransTexData{}},
		{name: "infinity color parameter unknown", data: `{"unknown":1}`, target: &InfColorParam{}},
		{name: "part color definition unknown", data: `{"unknown":1}`, target: &PartColDef{}},
		{name: "infinity color data unknown", data: `{"unknown":1}`, target: &InfColData{}},
		{name: "nested parts color unknown", data: `{"pc":{"unknown":1}}`, target: &InfColorParam{}},
		{name: "dynamic bone trailing value", data: `{} {}`, target: &DynamicBoneStatus{}},
		{name: "parts color trailing value", data: `{} null`, target: &PartsColor{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(test.data), test.target); err == nil {
				t.Fatalf("json.Unmarshal(%s) unexpectedly succeeded", test.data)
			}
		})
	}
}

func TestPartsColorMessagePackRebuildsGradientBytes(t *testing.T) {
	value := PartsColor{
		MainHue: 1,
		Grada:   []PartsColorGrada{{MainHue: 2, MainContrast: 3, ShadowContrast: 4}},
	}
	wire, err := ct.EncodeIndexedMsgpack(&value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded PartsColor
	if err := ct.DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("MessagePack round trip changed: got %#v want %#v", decoded, value)
	}
}

func TestPartsColorRejectsMalformedGradientBytes(t *testing.T) {
	malformed := [][]byte{
		{1, 0, 0, 0},
		{0xff, 0xff, 0xff, 0xff},
		{0, 0, 0, 0, 0xaa},
	}
	for _, gradient := range malformed {
		root := []interface{}{int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), gradient}
		wire, err := ct.EncodeMsgpack(root)
		if err != nil {
			t.Fatal(err)
		}
		var decoded PartsColor
		if err := ct.DecodeMsgpack(wire, &decoded); err == nil {
			t.Fatalf("malformed gradient %x unexpectedly decoded", gradient)
		}
	}
}

func TestPartsColorGradaRejectsTrailingBytes(t *testing.T) {
	wire, err := EncodePartsColorGrada([]PartsColorGrada{{MainHue: 1}})
	if err != nil {
		t.Fatal(err)
	}
	wire = append(wire, 0xaa)
	if _, err := DecodePartsColorGrada(wire); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("DecodePartsColorGrada() error = %v, want trailing data", err)
	}
}
