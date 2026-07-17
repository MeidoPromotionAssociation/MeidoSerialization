package KCES

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestMenuAssetsPreservesMalformedGradientBytes(t *testing.T) {
	assets := &MenuAssets{Assets: []Menu{{
		ColvariInfo: &Colvari{ColvariDatas: []ColvariData{{
			ColData: PartsColor{GradaBytes: GradaBytes{Value: []byte{1, 0, 0, 0}}},
		}}},
	}}}

	wire, err := EncodeMenuAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMenuAssets: %v", err)
	}
	got, err := DecodeMenuAssets(wire)
	if err != nil {
		t.Fatalf("DecodeMenuAssets: %v", err)
	}
	if value := got.Assets[0].ColvariInfo.ColvariDatas[0].ColData.GradaBytes.Value; !reflect.DeepEqual(value, []byte{1, 0, 0, 0}) {
		t.Fatalf("malformed gradient bytes changed: %#v", value)
	}
	jsonData, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonData), `"m_gradaDecodeError"`) {
		t.Fatalf("malformed readable view was not reported: %s", jsonData)
	}
}

func TestMenuAssetsAcceptsCurrentAndNullableGradientShapes(t *testing.T) {
	for _, value := range []interface{}{
		[]byte{0, 0, 0, 0}, // current game: zero gradient colors
		nil,                // MessagePack-CSharp ByteArrayFormatter accepts nil
	} {
		assets := &MenuAssets{Assets: []Menu{{
			ColvariInfo: &Colvari{IconColor: PartsColor{GradaBytes: GradaBytes{Value: value}}},
		}}}
		encoded, err := EncodeMenuAssets(assets)
		if err != nil {
			t.Fatalf("EncodeMenuAssets(%T): %v", value, err)
		}
		if _, err := DecodeMenuAssets(encoded); err != nil {
			t.Fatalf("DecodeMenuAssets(%T): %v", value, err)
		}
	}
}

func TestPartsColorGradaReadableJSONAndExplicitEditing(t *testing.T) {
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
	trailing := []byte{0xaa, 0xbb, 0xcc}
	raw := append(append([]byte(nil), encoded...), trailing...)

	decoded, gotTrailing, err := DecodePartsColorGrada(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, values) || !bytes.Equal(gotTrailing, trailing) {
		t.Fatalf("DeserializeGrada layout mismatch: values=%+v trailing=%x", decoded, gotTrailing)
	}

	original := PartsColor{MainHue: 99, GradaBytes: GradaBytes{Value: raw}}
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &object); err != nil {
		t.Fatal(err)
	}
	if _, ok := object["m_grada"]; !ok {
		t.Fatalf("readable m_grada view missing: %s", jsonData)
	}
	if _, ok := object["m_gradaTrailingBytes"]; !ok {
		t.Fatalf("trailing-byte preservation field missing: %s", jsonData)
	}

	var unchanged PartsColor
	if err := json.Unmarshal(jsonData, &unchanged); err != nil {
		t.Fatal(err)
	}
	if got, ok := unchanged.GradaBytes.Value.([]byte); !ok || !bytes.Equal(got, raw) {
		t.Fatalf("unchanged readable JSON did not retain exact raw bytes: %T %x", unchanged.GradaBytes.Value, got)
	}

	var editable []PartsColorGrada
	if err := json.Unmarshal(object["m_grada"], &editable); err != nil {
		t.Fatal(err)
	}
	editable[0].MainHue = 123456
	object["m_grada"], err = json.Marshal(editable)
	if err != nil {
		t.Fatal(err)
	}
	editedJSON, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	var edited PartsColor
	if err := json.Unmarshal(editedJSON, &edited); err != nil {
		t.Fatal(err)
	}
	editedValues, editedTrailing, err := edited.GradaBytes.DecodeGrada()
	if err != nil {
		t.Fatal(err)
	}
	if editedValues[0].MainHue != 123456 || !bytes.Equal(editedTrailing, trailing) {
		t.Fatalf("m_grada edit was not encoded faithfully: values=%+v trailing=%x", editedValues, editedTrailing)
	}
}

func TestGradaBytesSetGradaIsExplicit(t *testing.T) {
	carrier := GradaBytes{Value: false}
	values := []PartsColorGrada{{MainHue: 7, ShadowContrast: 9}}
	if err := carrier.SetGrada(values, []byte{0xfe}); err != nil {
		t.Fatal(err)
	}
	decoded, trailing, err := carrier.DecodeGrada()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, values) || !bytes.Equal(trailing, []byte{0xfe}) {
		t.Fatalf("explicit SetGrada mismatch: values=%+v trailing=%x", decoded, trailing)
	}
}

func TestGradaBytesMatchesGameByteArrayFormatterCarriers(t *testing.T) {
	accepted := []struct {
		name string
		raw  []byte
		want []byte
		nil  bool
	}{
		{name: "bin8", raw: []byte{0xc4, 0x03, 'a', 'b', 'c'}, want: []byte("abc")},
		{name: "legacy_fixstr", raw: []byte{0xa3, 'a', 'b', 'c'}, want: []byte("abc")},
		{name: "legacy_str16", raw: []byte{0xda, 0x00, 0x01, 'x'}, want: []byte("x")},
		{name: "legacy_str32", raw: []byte{0xdb, 0x00, 0x00, 0x00, 0x01, 'y'}, want: []byte("y")},
		{name: "nil", raw: []byte{0xc0}, nil: true},
		{name: "fixarray", raw: []byte{0x93, 0x00, 0xcc, 0xff, 0x01}, want: []byte{0, 255, 1}},
		{name: "array16", raw: []byte{0xdc, 0x00, 0x02, 0x02, 0x03}, want: []byte{2, 3}},
	}
	for _, test := range accepted {
		t.Run("accept_"+test.name, func(t *testing.T) {
			var got GradaBytes
			if err := ct.DecodeMsgpack(test.raw, &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if test.nil {
				if got.Value != nil {
					t.Fatalf("nil carrier became %#v", got.Value)
				}
			} else if value, ok := got.Value.([]byte); !ok || !bytes.Equal(value, test.want) {
				t.Fatalf("decoded value = %T %#v, want %x", got.Value, got.Value, test.want)
			}

			encoded, err := ct.EncodeMsgpack(got)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if test.nil {
				if !bytes.Equal(encoded, []byte{0xc0}) {
					t.Fatalf("encoded nil = %x", encoded)
				}
			} else if len(encoded) == 0 || encoded[0] < 0xc4 || encoded[0] > 0xc6 {
				t.Fatalf("game byte[] canonical carrier is not bin: %x", encoded)
			}
		})
	}

	rejected := []struct {
		name string
		raw  []byte
	}{
		{name: "str8", raw: []byte{0xd9, 0x01, 'x'}},
		{name: "bool", raw: []byte{0xc2}},
		{name: "negative_array_element", raw: []byte{0x91, 0xff}},
		{name: "overflow_array_element", raw: []byte{0x91, 0xcd, 0x01, 0x00}},
	}
	for _, test := range rejected {
		t.Run("reject_"+test.name, func(t *testing.T) {
			var got GradaBytes
			if err := ct.DecodeMsgpack(test.raw, &got); err == nil {
				t.Fatalf("accepted invalid game byte[] carrier %x as %#v", test.raw, got.Value)
			}
		})
	}
}

func TestGradaBytesRejectsUnsupportedValuesBeforeEncoding(t *testing.T) {
	for _, value := range []interface{}{false, "bytes", []interface{}{1}} {
		if _, err := ct.EncodeMsgpack(GradaBytes{Value: value}); err == nil {
			t.Fatalf("accepted unsupported value %T", value)
		}
		if _, err := json.Marshal(GradaBytes{Value: value}); err == nil {
			t.Fatalf("JSON accepted unsupported value %T", value)
		}
	}
}
