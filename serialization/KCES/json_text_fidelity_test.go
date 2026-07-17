package KCES

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestKCESJSONTextPreservesOriginalTextUntilEdited(t *testing.T) {
	original := append([]byte{0xef, 0xbb, 0xbf}, []byte("{\r\n  \"version\" : -7,\r\n  \"future\" : [ 1, 2 ]\r\n}\r\n")...)
	for _, extension := range []string{".undressdat", ".undresspdat", ".nson"} {
		value, err := DecodeKCESJSONText(original, extension)
		if err != nil {
			t.Fatalf("DecodeKCESJSONText(%s) error = %v", extension, err)
		}
		if value.Text != string(original) || !bytes.Equal(value.JSON, []byte(`{"version":-7,"future":[1,2]}`)) {
			t.Fatalf("decoded %s text/json = %x / %s", extension, []byte(value.Text), value.JSON)
		}
		unchanged, err := EncodeKCESJSONText(value)
		if err != nil || !bytes.Equal(unchanged, original) {
			t.Fatalf("unchanged %s text was rebuilt: %x, %v", extension, unchanged, err)
		}

		value.JSON = json.RawMessage(` { "edited" : true } `)
		edited, err := EncodeKCESJSONText(value)
		if err != nil {
			t.Fatalf("EncodeKCESJSONText(edited %s) error = %v", extension, err)
		}
		if bytes.Equal(edited, original) || bytes.HasPrefix(edited, []byte{0xef, 0xbb, 0xbf}) || !json.Valid(edited) {
			t.Fatalf("edited %s JSON was not rebuilt correctly: %x", extension, edited)
		}
	}
}
