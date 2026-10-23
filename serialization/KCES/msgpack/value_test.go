package msgpack

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// TestValueFormsRoundTrip 验证各宽度整数、浮点、字符串、二进制与扩展类型能往返
// TestValueFormsRoundTrip verifies that integers of every width, floats, strings, binary, and extensions round-trip
func TestValueFormsRoundTrip(t *testing.T) {
	values := []Value{
		{Kind: KindUint, Uint: 0},
		{Kind: KindUint, Uint: 127},
		{Kind: KindUint, Uint: 128},
		{Kind: KindUint, Uint: 65535},
		{Kind: KindUint, Uint: 65536},
		{Kind: KindUint, Uint: 4294967295},
		{Kind: KindUint, Uint: 4294967296},
		{Kind: KindUint, Uint: 936000000000},
		{Kind: KindInt, Int: -1},
		{Kind: KindInt, Int: -32},
		{Kind: KindInt, Int: -33},
		{Kind: KindInt, Int: -129},
		{Kind: KindInt, Int: -32769},
		{Kind: KindInt, Int: -2147483649},
		{Kind: KindFloat32, Float32: 0.1},
		{Kind: KindFloat32, Float32: 1},
		{Kind: KindFloat32, Float32: 1.5},
		{Kind: KindFloat64, Float64: 0.1},
		{Kind: KindFloat64, Float64: 1.0 / 3.0},
		{Kind: KindString, Str: ""},
		{Kind: KindString, Str: "主人公"},
		{Kind: KindString, Str: string(make([]byte, 300))},
		{Kind: KindBool, Bool: true},
		{Kind: KindNil},
		{Kind: KindBin, Bin: []byte{1, 2, 3}},
		{Kind: KindExt, Ext: Ext{Type: 5, Data: []byte{4, 5, 6, 7}}},
	}
	for _, value := range values {
		encoded, err := EncodeValue(value)
		if err != nil {
			t.Fatalf("EncodeValue %s: %v", value.Kind, err)
		}
		decoded, err := DecodeValue(encoded)
		if err != nil {
			t.Fatalf("DecodeValue %s: %v", value.Kind, err)
		}
		if !reflect.DeepEqual(decoded, value) {
			t.Fatalf("%s binary round trip = %+v, want %+v", value.Kind, decoded, value)
		}

		jsonData, err := EncodeValueJSON(value)
		if err != nil {
			t.Fatalf("EncodeValueJSON %s: %v", value.Kind, err)
		}
		fromJSON, err := DecodeValueJSON(jsonData)
		if err != nil {
			t.Fatalf("DecodeValueJSON %s (%s): %v", value.Kind, jsonData, err)
		}
		if !reflect.DeepEqual(fromJSON, value) {
			t.Fatalf("%s JSON round trip = %+v, want %+v", value.Kind, fromJSON, value)
		}
	}
}

// TestValueShortestIntegerEncodings 验证非负整数一律写成无符号最短形式
// TestValueShortestIntegerEncodings verifies that non-negative integers are always written in the shortest unsigned form
func TestValueShortestIntegerEncodings(t *testing.T) {
	tests := []struct {
		value Value
		want  []byte
	}{
		{Value{Kind: KindUint, Uint: 5}, []byte{0x05}},
		{Value{Kind: KindUint, Uint: 200}, []byte{0xcc, 0xc8}},
		{Value{Kind: KindUint, Uint: 1000}, []byte{0xcd, 0x03, 0xe8}},
		{Value{Kind: KindUint, Uint: 100000}, []byte{0xce, 0x00, 0x01, 0x86, 0xa0}},
		{Value{Kind: KindInt, Int: -1}, []byte{0xff}},
	}
	for _, test := range tests {
		got, err := EncodeValue(test.value)
		if err != nil {
			t.Fatalf("EncodeValue %+v: %v", test.value, err)
		}
		if !bytes.Equal(got, test.want) {
			t.Fatalf("EncodeValue %+v = %x, want %x", test.value, got, test.want)
		}
	}
}

// TestValueFloatWidthInference 验证浮点宽度按能否由 float32 精确表示推断
// TestValueFloatWidthInference verifies that float width is inferred from whether float32 represents the value exactly
func TestValueFloatWidthInference(t *testing.T) {
	tests := []struct {
		name   string
		value  Value
		marker byte
	}{
		{name: "double precision value", value: Value{Kind: KindFloat64, Float64: 0.1}, marker: 0xcb},
		{name: "double that float32 holds", value: Value{Kind: KindFloat64, Float64: 1}, marker: 0xca},
	}
	for _, test := range tests {
		encoded, err := EncodeValue(test.value)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if encoded[0] != test.marker {
			t.Fatalf("%s wrote 0x%02x, want 0x%02x", test.name, encoded[0], test.marker)
		}
	}

	// 单精度 0.1 被 ugorji 精确加宽后必须还原成 float32，否则每个浮点字段都会翻倍
	// The single-precision 0.1 widened exactly by ugorji must come back as float32, or every float field doubles in size
	single := Value{Kind: KindFloat32, Float32: 0.1}
	encoded, err := EncodeValue(single)
	if err != nil {
		t.Fatalf("EncodeValue float32: %v", err)
	}
	decoded, err := DecodeValue(encoded)
	if err != nil {
		t.Fatalf("DecodeValue float32: %v", err)
	}
	if !reflect.DeepEqual(decoded, single) {
		t.Fatalf("float32 round trip = %+v, want %+v", decoded, single)
	}
}

// TestValueMapKeysKeepOrderAndType 验证映射键按数值升序排列并在 JSON 往返后仍为整数键
// TestValueMapKeysKeepOrderAndType verifies that map keys sort numerically and stay integer keys after the JSON round trip
func TestValueMapKeysKeepOrderAndType(t *testing.T) {
	value := Value{Kind: KindMap, Map: []MapEntry{
		{Key: Value{Kind: KindUint, Uint: 10}, Value: Value{Kind: KindString, Str: "a"}},
		{Key: Value{Kind: KindUint, Uint: 2}, Value: Value{Kind: KindString, Str: "b"}},
		{Key: Value{Kind: KindString, Str: "name"}, Value: Value{Kind: KindString, Str: "c"}},
	}}
	jsonData, err := EncodeValueJSON(value)
	if err != nil {
		t.Fatalf("EncodeValueJSON: %v", err)
	}
	want := "{\n  \"10\": \"a\",\n  \"2\": \"b\",\n  \"name\": \"c\"\n}\n"
	if string(jsonData) != want {
		t.Fatalf("EncodeValueJSON =\n%s\nwant\n%s", jsonData, want)
	}
	decoded, err := DecodeValueJSON(jsonData)
	if err != nil {
		t.Fatalf("DecodeValueJSON: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("map round trip = %+v, want %+v", decoded, value)
	}
}

// TestValueDecodeSortsMapKeys 验证解码时按数值升序整理映射键
// TestValueDecodeSortsMapKeys verifies that decoding orders map keys by ascending numeric value
func TestValueDecodeSortsMapKeys(t *testing.T) {
	payload := []byte{0x83, 0x0a, 0xc2, 0x02, 0xc3, 0xff, 0xc0}
	value, err := DecodeValue(payload)
	if err != nil {
		t.Fatalf("DecodeValue: %v", err)
	}
	want := []MapEntry{
		{Key: Value{Kind: KindInt, Int: -1}, Value: Value{Kind: KindNil}},
		{Key: Value{Kind: KindUint, Uint: 2}, Value: Value{Kind: KindBool, Bool: true}},
		{Key: Value{Kind: KindUint, Uint: 10}, Value: Value{Kind: KindBool, Bool: false}},
	}
	if !reflect.DeepEqual(value.Map, want) {
		t.Fatalf("map entries = %+v, want %+v", value.Map, want)
	}
}

// TestValueJSONRejectsInvalidInput 验证非法 JSON 被拒绝
// TestValueJSONRejectsInvalidInput verifies that invalid JSON is rejected
func TestValueJSONRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "duplicate key", data: `{"1": 1, "1": 2}`},
		{name: "trailing value", data: `1 2`},
		{name: "empty", data: `   `},
		{name: "broken bin tag", data: `{"$bin": "not base64!"}`},
		{name: "broken ext tag", data: `{"$ext": {"type": 1}}`},
		{name: "integer overflow", data: `18446744073709551616`},
		{name: "infinite float", data: `1e400`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := DecodeValueJSON([]byte(test.data))
			if err == nil {
				t.Fatalf("DecodeValueJSON accepted %s as %+v", test.data, value)
			}
		})
	}
}

// TestValueDecodeRejectsMalformedPayload 验证畸形载荷被拒绝
// TestValueDecodeRejectsMalformedPayload verifies that malformed payloads are rejected
func TestValueDecodeRejectsMalformedPayload(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "truncated array", data: []byte{0x93, 0x01}},
		{name: "reserved marker", data: []byte{0xc1}},
		{name: "trailing bytes", data: []byte{0x01, 0x02}},
		{name: "invalid utf8 string", data: []byte{0xa1, 0xff}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := DecodeValue(test.data)
			if err == nil {
				t.Fatalf("DecodeValue accepted %x as %+v", test.data, value)
			}
		})
	}
}

// TestValueDecodeRejectsDeepNesting 验证超过解码深度上限的嵌套被拒绝
// TestValueDecodeRejectsDeepNesting verifies that nesting beyond the decoder depth limit is rejected
func TestValueDecodeRejectsDeepNesting(t *testing.T) {
	data := make([]byte, 0, 300)
	for index := 0; index < 300; index++ {
		data = append(data, 0x91)
	}
	data = append(data, 0xc0)
	if value, err := DecodeValue(data); err == nil {
		t.Fatalf("DecodeValue accepted 300 levels of nesting as %+v", value.Kind)
	}
}

// TestValueJSONMatchesStandardEncoding 验证值树 JSON 是标准 JSON
// TestValueJSONMatchesStandardEncoding verifies that the value tree JSON is standard JSON
func TestValueJSONMatchesStandardEncoding(t *testing.T) {
	value := Value{Kind: KindArray, Array: []Value{
		{Kind: KindString, Str: "a<b&c"},
		{Kind: KindFloat32, Float32: 0.5},
		{Kind: KindNil},
	}}
	jsonData, err := EncodeValueJSON(value)
	if err != nil {
		t.Fatalf("EncodeValueJSON: %v", err)
	}
	if !json.Valid(jsonData) {
		t.Fatalf("EncodeValueJSON produced invalid JSON: %s", jsonData)
	}
}
