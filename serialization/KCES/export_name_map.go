package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

const (
	// KCESExportNameMapFormat identifies the editable JSON representation. The
	// native export_map.enm is also JSON, but uses Unity JsonUtility's
	// version/serializeData wrapper instead of this marker.
	KCESExportNameMapFormat = "kces-export-name-map"

	// KCESExportNameMapSignature is used by content probing for the native
	// JsonUtility document, which has no binary magic string of its own.
	KCESExportNameMapSignature = "KCES_EXPORT_NAME_MAP"

	// KCESExportNameMapVersion is ExportFileNameMap.FixVersion in KCES 1.34.4.
	KCESExportNameMapVersion int32 = 1000
)

// KCESExportNameMap is the stable editing representation of export_map.enm.
// Entry order, spelling, case, and a nil-versus-empty list are preserved.
type KCESExportNameMap struct {
	Format            string                   `json:"format"`
	Version           int32                    `json:"version"`
	Entries           []KCESExportNameMapEntry `json:"entries"`
	NativeText        string                   `json:"nativeText,omitempty"`
	NativeDecodeError string                   `json:"nativeDecodeError,omitempty"`
}

// KCESExportNameMapEntry maps the original internal resource name to the
// short filename actually written next to export_map.enm.
type KCESExportNameMapEntry struct {
	InternalName string `json:"internalName"`
	FileName     string `json:"fileName"`
}

type kcesExportNameMapNativeOutput struct {
	Version       int32  `json:"version"`
	SerializeData string `json:"serializeData"`
}

type kcesExportNameMapDictionaryOutput struct {
	Keys   []string `json:"keys"`
	Values []string `json:"values"`
}

type kcesExportNameMapEditing struct {
	Format            *string                           `json:"format"`
	Version           *int32                            `json:"version"`
	Entries           *[]*kcesExportNameMapEditingEntry `json:"entries"`
	NativeText        *string                           `json:"nativeText"`
	NativeDecodeError *string                           `json:"nativeDecodeError"`
}

type kcesExportNameMapEditingEntry struct {
	InternalName *string `json:"internalName"`
	FileName     *string `json:"fileName"`
}

// DecodeKCESExportNameMap parses the native Unity JsonUtility representation:
//
//	{"version":1000,"serializeData":"{\"keys\":[...],\"values\":[...]}"}
//
// A single leading UTF-8 BOM is accepted because File.ReadAllText consumes it.
// Both the outer document and the nested dictionary must contain exactly one
// JSON value.
func DecodeKCESExportNameMap(data []byte) (*KCESExportNameMap, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("native export name map is not valid UTF-8")
	}
	jsonData := bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if !json.Valid(jsonData) {
		return nil, fmt.Errorf("native export name map JSON is invalid")
	}

	result := &KCESExportNameMap{
		Format:     KCESExportNameMapFormat,
		NativeText: string(data),
	}
	setDecodeError := func(format string, args ...interface{}) {
		if result.NativeDecodeError == "" {
			result.NativeDecodeError = fmt.Sprintf(format, args...)
		}
	}

	var outer map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &outer); err != nil || outer == nil {
		setDecodeError("native export name map root is not an object")
		return result, nil
	}
	versionRaw, hasVersion := outer["version"]
	if !hasVersion || bytes.Equal(bytes.TrimSpace(versionRaw), []byte("null")) {
		setDecodeError("native export name map version is missing or null")
	} else if err := json.Unmarshal(versionRaw, &result.Version); err != nil {
		setDecodeError("native export name map version is not an Int32: %v", err)
	}

	serializeRaw, hasSerializeData := outer["serializeData"]
	if !hasSerializeData || bytes.Equal(bytes.TrimSpace(serializeRaw), []byte("null")) {
		setDecodeError("native export name map serializeData is missing or null")
		return result, nil
	}
	var serializeData string
	if err := json.Unmarshal(serializeRaw, &serializeData); err != nil {
		setDecodeError("native export name map serializeData is not a string: %v", err)
		return result, nil
	}
	if !utf8.ValidString(serializeData) || !json.Valid([]byte(serializeData)) {
		setDecodeError("export name map dictionary JSON is invalid")
		return result, nil
	}

	var dictionary map[string]json.RawMessage
	if err := json.Unmarshal([]byte(serializeData), &dictionary); err != nil || dictionary == nil {
		setDecodeError("export name map dictionary root is not an object")
		return result, nil
	}
	keysRaw, hasKeys := dictionary["keys"]
	if !hasKeys || bytes.Equal(bytes.TrimSpace(keysRaw), []byte("null")) {
		setDecodeError("export name map dictionary keys are missing or null")
		return result, nil
	}
	valuesRaw, hasValues := dictionary["values"]
	if !hasValues || bytes.Equal(bytes.TrimSpace(valuesRaw), []byte("null")) {
		setDecodeError("export name map dictionary values are missing or null")
		return result, nil
	}
	var keys []*string
	if err := json.Unmarshal(keysRaw, &keys); err != nil {
		setDecodeError("export name map dictionary keys are not a string array: %v", err)
		return result, nil
	}
	var values []*string
	if err := json.Unmarshal(valuesRaw, &values); err != nil {
		setDecodeError("export name map dictionary values are not a string array: %v", err)
		return result, nil
	}
	if len(keys) != len(values) {
		setDecodeError("export name map dictionary keys and values have different lengths: %d != %d", len(keys), len(values))
		return result, nil
	}
	entries := make([]KCESExportNameMapEntry, len(keys))
	for index := range keys {
		if keys[index] == nil || values[index] == nil {
			setDecodeError("export name map dictionary contains null at index %d", index)
			return result, nil
		}
		entries[index] = KCESExportNameMapEntry{InternalName: *keys[index], FileName: *values[index]}
	}
	result.Entries = entries
	return result, nil
}

// EncodeKCESExportNameMap writes the native Unity JsonUtility wrapper. The
// nested keys and values retain their supplied order and spelling while using
// the layout consumed by ScourtExtensionsDictionary.FromJson.
func EncodeKCESExportNameMap(value *KCESExportNameMap) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil export name map")
	}
	if value.Format != "" && value.Format != KCESExportNameMapFormat {
		return nil, fmt.Errorf("unsupported export name map JSON format %q", value.Format)
	}
	if value.NativeText != "" {
		baseline, err := DecodeKCESExportNameMap([]byte(value.NativeText))
		if err != nil {
			return nil, fmt.Errorf("nativeText: %w", err)
		}
		if baseline.Version == value.Version && exportNameMapEntriesEqual(baseline.Entries, value.Entries) {
			return []byte(value.NativeText), nil
		}
	}
	canonical, err := canonicalKCESExportNameMap(value)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(canonical.Entries))
	values := make([]string, len(canonical.Entries))
	for i, entry := range canonical.Entries {
		keys[i] = entry.InternalName
		values[i] = entry.FileName
	}
	nested, err := json.Marshal(kcesExportNameMapDictionaryOutput{Keys: keys, Values: values})
	if err != nil {
		return nil, fmt.Errorf("marshal export name map dictionary: %w", err)
	}
	native, err := json.Marshal(kcesExportNameMapNativeOutput{
		Version:       canonical.Version,
		SerializeData: string(nested),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal native export name map: %w", err)
	}
	return native, nil
}

func exportNameMapEntriesEqual(left, right []KCESExportNameMapEntry) bool {
	if (left == nil) != (right == nil) || len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// DecodeKCESExportNameMapJSON parses the editable, marker-based JSON form.
// Missing and null entry lists both represent a nil list, matching a native
// dictionary whose keys and values are both null.
func DecodeKCESExportNameMapJSON(data []byte) (*KCESExportNameMap, error) {
	var editing kcesExportNameMapEditing
	if err := decodeKCESExportNameMapJSONValue(data, &editing, "export name map editing JSON", true); err != nil {
		return nil, err
	}
	if editing.Format == nil {
		return nil, fmt.Errorf("export name map editing JSON format is missing or null")
	}
	if *editing.Format != KCESExportNameMapFormat {
		return nil, fmt.Errorf("unsupported export name map JSON format %q", *editing.Format)
	}
	if editing.Version == nil {
		return nil, fmt.Errorf("export name map editing JSON version is missing or null")
	}
	var rawEntries []*kcesExportNameMapEditingEntry
	if editing.Entries != nil {
		rawEntries = *editing.Entries
	}
	var entries []KCESExportNameMapEntry
	if rawEntries != nil {
		entries = make([]KCESExportNameMapEntry, len(rawEntries))
	}
	for i, entry := range rawEntries {
		if entry == nil {
			return nil, fmt.Errorf("export name map editing JSON entries[%d] is null", i)
		}
		if entry.InternalName == nil {
			return nil, fmt.Errorf("export name map editing JSON entries[%d].internalName is missing or null", i)
		}
		if entry.FileName == nil {
			return nil, fmt.Errorf("export name map editing JSON entries[%d].fileName is missing or null", i)
		}
		entries[i] = KCESExportNameMapEntry{InternalName: *entry.InternalName, FileName: *entry.FileName}
	}

	return canonicalKCESExportNameMap(&KCESExportNameMap{
		Format:            *editing.Format,
		Version:           *editing.Version,
		Entries:           entries,
		NativeText:        optionalExportNameMapString(editing.NativeText),
		NativeDecodeError: optionalExportNameMapString(editing.NativeDecodeError),
	})
}

func optionalExportNameMapString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// EncodeKCESExportNameMapJSON writes the deterministic editing representation
// used by the service and CLI. It does not mutate value or its Entries slice.
func EncodeKCESExportNameMapJSON(value *KCESExportNameMap) ([]byte, error) {
	canonical, err := canonicalKCESExportNameMap(value)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal export name map editing JSON: %w", err)
	}
	return append(data, '\n'), nil
}

func canonicalKCESExportNameMap(value *KCESExportNameMap) (*KCESExportNameMap, error) {
	if value == nil {
		return nil, fmt.Errorf("nil export name map")
	}
	if value.Format != "" && value.Format != KCESExportNameMapFormat {
		return nil, fmt.Errorf("unsupported export name map JSON format %q", value.Format)
	}

	var entries []KCESExportNameMapEntry
	if value.Entries != nil {
		entries = make([]KCESExportNameMapEntry, len(value.Entries))
	}
	for i, entry := range value.Entries {
		if !utf8.ValidString(entry.InternalName) {
			return nil, fmt.Errorf("export name map entries[%d].internalName is not valid UTF-8", i)
		}
		if !utf8.ValidString(entry.FileName) {
			return nil, fmt.Errorf("export name map entries[%d].fileName is not valid UTF-8", i)
		}
		entries[i] = entry
	}
	return &KCESExportNameMap{
		Format:            KCESExportNameMapFormat,
		Version:           value.Version,
		Entries:           entries,
		NativeText:        value.NativeText,
		NativeDecodeError: value.NativeDecodeError,
	}, nil
}

func decodeKCESExportNameMapJSONValue(data []byte, dst any, label string, allowBOM bool) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("%s is not valid UTF-8", label)
	}
	if allowBOM {
		data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("%s is empty", label)
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("%s dictionary/root is null", label)
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s has trailing JSON value", label)
		}
		return fmt.Errorf("%s has trailing content: %w", label, err)
	}
	return nil
}
