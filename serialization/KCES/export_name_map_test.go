package KCES

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// There is no export_map.enm sample in testdata. These fixtures are built
// directly from ExportFileNameMap.OnBeforeSerialize and
// ScourtExtensionsDictionary.SerializeDictionary in the KCES 1.34.4 source.
func TestKCESExportNameMapSourceCompatibleNativeRoundTrip(t *testing.T) {
	native := []byte(`{"version":1000,"serializeData":"{\"keys\":[\"GP03_EXPORT_HAIR.MENU\",\"gp03_export_body.mate\"],\"values\":[\"0.MENU\",\"1.MATE\"]}"}`)

	decoded, err := DecodeKCESExportNameMap(append([]byte{0xef, 0xbb, 0xbf}, native...))
	if err != nil {
		t.Fatalf("DecodeKCESExportNameMap source fixture: %v", err)
	}
	want := []KCESExportNameMapEntry{
		{InternalName: "GP03_EXPORT_HAIR.MENU", FileName: "0.MENU"},
		{InternalName: "gp03_export_body.mate", FileName: "1.MATE"},
	}
	if decoded.Format != KCESExportNameMapFormat || decoded.Version != KCESExportNameMapVersion || !reflect.DeepEqual(decoded.Entries, want) {
		t.Fatalf("decoded = %+v, want entries %+v", decoded, want)
	}

	encoded, err := EncodeKCESExportNameMap(decoded)
	if err != nil {
		t.Fatalf("EncodeKCESExportNameMap: %v", err)
	}
	decodedAgain, err := DecodeKCESExportNameMap(encoded)
	if err != nil {
		t.Fatalf("re-decode native export map: %v", err)
	}
	if !reflect.DeepEqual(decodedAgain, decoded) {
		t.Fatalf("round trip changed export map: got %+v want %+v", decodedAgain, decoded)
	}

	// Native output is stable even though a C# Dictionary's enumeration order
	// is not a useful editing contract.
	encodedAgain, err := EncodeKCESExportNameMap(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodedAgain, encoded) {
		t.Fatalf("native encoding is not deterministic:\n%s\n%s", encoded, encodedAgain)
	}
}

func TestKCESExportNameMapEncodePreservesWithoutMutatingCaller(t *testing.T) {
	value := &KCESExportNameMap{
		Format:  KCESExportNameMapFormat,
		Version: KCESExportNameMapVersion,
		Entries: []KCESExportNameMapEntry{
			{InternalName: "ZETA.MENU", FileName: "9.MENU"},
			{InternalName: "Alpha.MATE", FileName: "0.MATE"},
		},
	}
	before := &KCESExportNameMap{
		Format:  value.Format,
		Version: value.Version,
		Entries: append([]KCESExportNameMapEntry(nil), value.Entries...),
	}

	native, err := EncodeKCESExportNameMap(value)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("EncodeKCESExportNameMap mutated caller: got %+v want %+v", value, before)
	}
	got, err := DecodeKCESExportNameMap(native)
	if err != nil {
		t.Fatal(err)
	}
	want := []KCESExportNameMapEntry{
		{InternalName: "ZETA.MENU", FileName: "9.MENU"},
		{InternalName: "Alpha.MATE", FileName: "0.MATE"},
	}
	if !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("canonical entries = %+v, want %+v", got.Entries, want)
	}
}

func TestKCESExportNameMapRejectsConsumerSchemaAnomalies(t *testing.T) {
	validNested := `{\"keys\":[\"internal.menu\"],\"values\":[\"0.menu\"]}`
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "invalid UTF-8", data: append([]byte(`{"version":1000,"serializeData":"`), 0xff), want: "UTF-8"},
		{name: "trailing value", data: []byte(`{"version":1000,"serializeData":"` + validNested + `"} {}`), want: "trailing"},
		{name: "missing version", data: []byte(`{"serializeData":"` + validNested + `"}`), want: "version"},
		{name: "null version", data: []byte(`{"version":null,"serializeData":"` + validNested + `"}`), want: "version"},
		{name: "version overflow", data: []byte(`{"version":2147483648,"serializeData":"` + validNested + `"}`), want: "version"},
		{name: "missing serializeData", data: []byte(`{"version":1000}`), want: "serializeData"},
		{name: "null serializeData", data: []byte(`{"version":1000,"serializeData":null}`), want: "serializeData"},
		{name: "empty serializeData", data: []byte(`{"version":1000,"serializeData":""}`), want: "dictionary"},
		{name: "nested null", data: []byte(`{"version":1000,"serializeData":"null"}`), want: "dictionary"},
		{name: "keys missing", data: []byte(`{"version":1000,"serializeData":"{\"values\":[]}"}`), want: "keys"},
		{name: "keys null", data: []byte(`{"version":1000,"serializeData":"{\"keys\":null,\"values\":[]}"}`), want: "keys"},
		{name: "values missing", data: []byte(`{"version":1000,"serializeData":"{\"keys\":[]}"}`), want: "values"},
		{name: "values null", data: []byte(`{"version":1000,"serializeData":"{\"keys\":[],\"values\":null}"}`), want: "values"},
		{name: "unequal arrays", data: []byte(`{"version":1000,"serializeData":"{\"keys\":[\"a\"],\"values\":[]}"}`), want: "different lengths"},
		{name: "null key", data: []byte(`{"version":1000,"serializeData":"{\"keys\":[null],\"values\":[\"x\"]}"}`), want: "null"},
		{name: "null value", data: []byte(`{"version":1000,"serializeData":"{\"keys\":[\"a\"],\"values\":[null]}"}`), want: "null"},
		{name: "duplicate key", data: []byte(`{"version":1000,"serializeData":"{\"keys\":[\"a\",\"a\"],\"values\":[\"0\",\"1\"]}"}`), want: "duplicated"},
		{name: "nested trailing", data: []byte(`{"version":1000,"serializeData":"{\"keys\":[],\"values\":[]} []"}`), want: "dictionary"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeKCESExportNameMap(test.data); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestKCESExportNameMapRejectsUnknownJSONFields(t *testing.T) {
	native := []byte("\xef\xbb\xbf {\r\n  \"version\" : -7,\r\n  \"serializeData\" : \"{\\\"keys\\\":[\\\"a\\\"],\\\"values\\\":[\\\"b\\\"],\\\"futureNested\\\":true}\",\r\n  \"futureOuter\" : [1,2]\r\n}\r\n")
	if _, err := DecodeKCESExportNameMap(native); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("DecodeKCESExportNameMap() error = %v, want unknown-field rejection", err)
	}
}

func TestKCESExportNameMapEditingJSONIsStrictAndDeterministic(t *testing.T) {
	editing := []byte("\xef\xbb\xbf" + `{
  "format":"kces-export-name-map",
  "version":1000,
  "entries":[
    {"internalName":"Zeta.Menu","fileName":"9.Menu"},
    {"internalName":"alpha.mate","fileName":"0.mate"}
  ]
}`)
	value, err := DecodeKCESExportNameMapJSON(editing)
	if err != nil {
		t.Fatal(err)
	}
	want := []KCESExportNameMapEntry{
		{InternalName: "Zeta.Menu", FileName: "9.Menu"},
		{InternalName: "alpha.mate", FileName: "0.mate"},
	}
	if !reflect.DeepEqual(value.Entries, want) {
		t.Fatalf("entries = %+v, want %+v", value.Entries, want)
	}

	encoded, err := EncodeKCESExportNameMapJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	encodedAgain, err := EncodeKCESExportNameMapJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, encodedAgain) || !bytes.HasSuffix(encoded, []byte("\n")) {
		t.Fatalf("editing JSON is not deterministic/newline terminated:\n%s", encoded)
	}

	invalid := map[string]string{
		"null root":          `null`,
		"missing format":     `{"version":1000,"entries":[]}`,
		"wrong format":       `{"format":"wrong","version":1000,"entries":[]}`,
		"missing version":    `{"format":"kces-export-name-map","entries":[]}`,
		"version null":       `{"format":"kces-export-name-map","version":null,"entries":[]}`,
		"version overflow":   `{"format":"kces-export-name-map","version":2147483648,"entries":[]}`,
		"null entry":         `{"format":"kces-export-name-map","version":1000,"entries":[null]}`,
		"null internal":      `{"format":"kces-export-name-map","version":1000,"entries":[{"internalName":null,"fileName":"a"}]}`,
		"null filename":      `{"format":"kces-export-name-map","version":1000,"entries":[{"internalName":"a","fileName":null}]}`,
		"duplicate internal": `{"format":"kces-export-name-map","version":1000,"entries":[{"internalName":"a","fileName":"0"},{"internalName":"a","fileName":"1"}]}`,
		"trailing":           `{"format":"kces-export-name-map","version":1000,"entries":[]} []`,
	}
	for name, data := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeKCESExportNameMapJSON([]byte(data)); err == nil {
				t.Fatalf("DecodeKCESExportNameMapJSON unexpectedly accepted %s", data)
			}
		})
	}
}

func TestKCESExportNameMapEncodersRejectDuplicateInternalNames(t *testing.T) {
	value := &KCESExportNameMap{
		Format:  KCESExportNameMapFormat,
		Version: KCESExportNameMapVersion,
		Entries: []KCESExportNameMapEntry{
			{InternalName: "a", FileName: "0"},
			{InternalName: "a", FileName: "1"},
		},
	}
	for name, encode := range map[string]func() error{
		"native": func() error {
			_, err := EncodeKCESExportNameMap(value)
			return err
		},
		"editing": func() error {
			_, err := EncodeKCESExportNameMapJSON(value)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := encode(); err == nil || !strings.Contains(err.Error(), "duplicated") {
				t.Fatalf("duplicate internalName error = %v", err)
			}
		})
	}
}

func TestKCESExportNameMapPreservesNilEntryList(t *testing.T) {
	for name, encode := range map[string]func() error{
		"native": func() error {
			_, err := EncodeKCESExportNameMap(&KCESExportNameMap{Version: KCESExportNameMapVersion})
			return err
		},
		"editing": func() error {
			_, err := EncodeKCESExportNameMapJSON(&KCESExportNameMap{Format: KCESExportNameMapFormat, Version: KCESExportNameMapVersion})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := encode(); err != nil {
				t.Fatalf("nil entries should encode, got %v", err)
			}
		})
	}

	empty := &KCESExportNameMap{Version: KCESExportNameMapVersion, Entries: []KCESExportNameMapEntry{}}
	if _, err := EncodeKCESExportNameMap(empty); err != nil {
		t.Fatalf("non-nil empty map should encode: %v", err)
	}
}
