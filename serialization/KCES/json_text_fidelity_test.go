package KCES

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestKCESJSONTextKeepsOnlyJSONSemantics(t *testing.T) {
	original := append([]byte{0xef, 0xbb, 0xbf}, []byte("{\r\n  \"version\" : -7,\r\n  \"future\" : [ 1, 2 ]\r\n}\r\n")...)
	for _, extension := range []string{".undressdat", ".undresspdat", ".nson"} {
		value, err := DecodeKCESJSONText(original, extension)
		if err != nil {
			t.Fatalf("DecodeKCESJSONText(%s) error = %v", extension, err)
		}
		if !bytes.Equal(value.JSON, []byte(`{"version":-7,"future":[1,2]}`)) {
			t.Fatalf("decoded %s JSON = %s", extension, value.JSON)
		}
		encoded, err := EncodeKCESJSONText(value)
		if err != nil {
			t.Fatalf("EncodeKCESJSONText(%s) error = %v", extension, err)
		}
		if bytes.Equal(encoded, original) || bytes.HasPrefix(encoded, []byte{0xef, 0xbb, 0xbf}) || !json.Valid(encoded) {
			t.Fatalf("%s was not normalized: %x", extension, encoded)
		}

		value.JSON = json.RawMessage(` { "edited" : true } `)
		edited, err := EncodeKCESJSONText(value)
		if err != nil || !json.Valid(edited) {
			t.Fatalf("edited %s JSON = %x, %v", extension, edited, err)
		}
	}
}
