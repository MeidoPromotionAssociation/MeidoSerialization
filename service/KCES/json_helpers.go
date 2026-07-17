package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

func trimJSONUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
}

// decodeStrictJSON decodes one complete UTF-8 JSON value using the editing
// schema exposed by this package. Editing JSON is not a forward-compatible
// game wire format: silently accepting an unknown field would drop the user's
// data when it is converted back to the proprietary representation.
func decodeStrictJSON(data []byte, out interface{}, name string) error {
	data = trimJSONUTF8BOM(data)
	if !utf8.Valid(data) {
		return fmt.Errorf("%s is not valid UTF-8", name)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("%s must be a JSON object, not null", name)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("trailing content: %w", err)
	}
	return nil
}
