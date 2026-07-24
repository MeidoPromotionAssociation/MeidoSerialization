package strictjson

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeRejectsNullAtValueTypedPositions(t *testing.T) {
	tests := []struct {
		name string
		data string
		out  any
		path string
	}{
		{name: "root", data: `null`, out: new(int32), path: "$"},
		{name: "field", data: `{"value":null}`, out: &struct {
			Value int32 `json:"value"`
		}{}, path: "$.value"},
		{name: "slice element", data: `{"values":[null]}`, out: &struct {
			Values []int32 `json:"values"`
		}{}, path: "$.values[0]"},
		{name: "map value", data: `{"values":{"key":null}}`, out: &struct {
			Values map[string]int32 `json:"values"`
		}{}, path: `$.values["key"]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Decode([]byte(test.data), test.out); err == nil || !strings.Contains(err.Error(), test.path+" must not be null") {
				t.Fatalf("Decode(%s) error = %v, want null rejection at %s", test.data, err, test.path)
			}
		})
	}
}

func TestDecodeAcceptsTypedNull(t *testing.T) {
	var value struct {
		Pointer  *int32            `json:"pointer"`
		Slice    []int32           `json:"slice"`
		Map      map[string]int32  `json:"map"`
		Pointers []*int32          `json:"pointers"`
		Raw      json.RawMessage   `json:"raw"`
		Any      any               `json:"any"`
		Nested   map[string]*int32 `json:"nested"`
	}
	data := []byte(`{"pointer":null,"slice":null,"map":null,"pointers":[null],"raw":null,"any":null,"nested":{"key":null}}`)
	if err := Decode(data, &value); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
}

func TestDecodeRejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	for _, data := range []string{`{"known":1,"unknown":2}`, `{"known":1} {}`} {
		var value struct {
			Known int32 `json:"known"`
		}
		if err := Decode([]byte(data), &value); err == nil {
			t.Fatalf("Decode(%s) unexpectedly succeeded", data)
		}
	}
}
