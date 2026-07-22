package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

// .enm (export_map.enm)
// KCES 导出到 COM3D2 时使用的文件名映射，原生文件是 Unity JsonUtility 文档
// 当前版本为 1000，内部字典以嵌套 JSON 字符串保存在 serializeData 中
// .enm (export_map.enm)
// KCES filename map used during COM3D2 export, with the native file stored as a Unity JsonUtility document
// The current version is 1000 and the inner dictionary is stored as a nested JSON string in serializeData

const (
	// KCESExportNameMapFormat 标识可编辑 JSON 表示，原生 export_map.enm 也是 JSON，但使用 Unity JsonUtility 的 version 和 serializeData 封套而不是此标记
	// KCESExportNameMapFormat identifies the editable JSON representation, while native export_map.enm is also JSON but uses Unity JsonUtility's version and serializeData wrapper instead of this marker
	KCESExportNameMapFormat = "kces-export-name-map"

	// KCESExportNameMapSignature 用于探测没有自身二进制魔数的原生 JsonUtility 文档
	// KCESExportNameMapSignature is used to probe native JsonUtility documents that have no binary magic string of their own
	KCESExportNameMapSignature = "KCES_EXPORT_NAME_MAP"

	// KCESExportNameMapVersion 是 KCES 1.34.4 中的 ExportFileNameMap.FixVersion
	// KCESExportNameMapVersion is ExportFileNameMap.FixVersion in KCES 1.34.4
	KCESExportNameMapVersion int32 = 1000
)

// KCESExportNameMap 是 export_map.enm 的稳定编辑表示，保留条目顺序、拼写、大小写以及 nil 与空列表的区别
// KCESExportNameMap is the stable editing representation of export_map.enm, preserving entry order, spelling, case, and the distinction between nil and empty lists
type KCESExportNameMap struct {
	Format            string                   `json:"format"`                      // JSON 表示格式标识 / JSON representation format identifier
	Version           int32                    `json:"version"`                     // ExportFileNameMap 对象版本 / ExportFileNameMap object version
	Entries           []KCESExportNameMapEntry `json:"entries"`                     // 按原生字典序列化顺序保存的映射条目 / Mapping entries in native dictionary serialization order
	NativeText        string                   `json:"nativeText,omitempty"`        // 解码时捕获的原生 JsonUtility 文本 / Native JsonUtility text captured during decoding
	NativeDecodeError string                   `json:"nativeDecodeError,omitempty"` // 有效外层 JSON 中无法解释为游戏结构的原因 / Reason valid outer JSON could not be interpreted as the game structure
}

// KCESExportNameMapEntry 将原始内部资源名映射到实际写在 export_map.enm 旁的短文件名
// KCESExportNameMapEntry maps an original internal resource name to the short filename actually written next to export_map.enm
type KCESExportNameMapEntry struct {
	InternalName string `json:"internalName"` // KCES 内部资源名 / KCES internal resource name
	FileName     string `json:"fileName"`     // 导出到 COM3D2 目录的短文件名 / Short filename exported to the COM3D2 directory
}

// kcesExportNameMapNativeOutput 表示游戏 JsonUtility 写出的外层对象
// kcesExportNameMapNativeOutput represents the outer object written by the game's JsonUtility
type kcesExportNameMapNativeOutput struct {
	Version       int32  `json:"version"`       // 对象版本 / Object version
	SerializeData string `json:"serializeData"` // 作为字符串嵌套的字典 JSON / Dictionary JSON nested as a string
}

// kcesExportNameMapDictionaryOutput 表示 ScourtExtensionsDictionary 的并行键值数组
// kcesExportNameMapDictionaryOutput represents the parallel key and value arrays used by ScourtExtensionsDictionary
type kcesExportNameMapDictionaryOutput struct {
	Keys   []string `json:"keys"`   // 内部资源名数组 / Internal resource-name array
	Values []string `json:"values"` // 导出文件名数组 / Exported filename array
}

// kcesExportNameMapEditing 表示严格解析可编辑 JSON 时使用的可空字段
// kcesExportNameMapEditing represents nullable fields used while strictly parsing editable JSON
type kcesExportNameMapEditing struct {
	Format            *string                           `json:"format"`            // 可空格式标识 / Nullable format identifier
	Version           *int32                            `json:"version"`           // 可空对象版本 / Nullable object version
	Entries           *[]*kcesExportNameMapEditingEntry `json:"entries"`           // 可空条目列表与条目 / Nullable entry list and entries
	NativeText        *string                           `json:"nativeText"`        // 可空原生文本 / Nullable native text
	NativeDecodeError *string                           `json:"nativeDecodeError"` // 可空原生解码错误 / Nullable native decode error
}

// kcesExportNameMapEditingEntry 表示严格解析时字段可空的一个编辑条目
// kcesExportNameMapEditingEntry represents one editing entry with nullable fields for strict parsing
type kcesExportNameMapEditingEntry struct {
	InternalName *string `json:"internalName"` // 可空内部资源名 / Nullable internal resource name
	FileName     *string `json:"fileName"`     // 可空导出文件名 / Nullable exported filename
}

// DecodeKCESExportNameMap 解析原生 Unity JsonUtility 表示
// 因 File.ReadAllText 会处理 BOM，允许一个前导 UTF-8 BOM，外层文档和嵌套字典都必须恰好包含一个 JSON 值
// DecodeKCESExportNameMap parses the native Unity JsonUtility representation shown above
// One leading UTF-8 BOM is accepted because File.ReadAllText consumes it, and both the outer document and nested dictionary must contain exactly one JSON value
//
//	{"version":1000,"serializeData":"{\"keys\":[...],\"values\":[...]}"}
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

// EncodeKCESExportNameMap 写出原生 Unity JsonUtility 封套，嵌套键值保留提供的顺序和拼写，并采用 ScourtExtensionsDictionary.FromJson 读取的布局
// EncodeKCESExportNameMap writes the native Unity JsonUtility wrapper while retaining supplied key and value order and spelling in the layout consumed by ScourtExtensionsDictionary.FromJson
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

// exportNameMapEntriesEqual 比较两个映射条目切片并保留 nil 与空切片的区别
// exportNameMapEntriesEqual compares two mapping-entry slices while preserving the distinction between nil and empty
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

// DecodeKCESExportNameMapJSON 解析带格式标记的可编辑 JSON，缺失和 null 条目列表都表示 nil 列表，与键和值均为 null 的原生字典一致
// DecodeKCESExportNameMapJSON parses the marker-based editable JSON form, where missing and null entry lists both represent nil to match a native dictionary whose keys and values are both null
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

// optionalExportNameMapString 将可空字符串转换为编辑模型使用的空字符串零值
// optionalExportNameMapString converts a nullable string to the empty-string zero value used by the editing model
func optionalExportNameMapString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// EncodeKCESExportNameMapJSON 写出服务和 CLI 使用的确定性编辑表示，不修改 value 或其 Entries 切片
// EncodeKCESExportNameMapJSON writes the deterministic editing representation used by the service and CLI without mutating value or its Entries slice
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

// canonicalKCESExportNameMap 校验并复制映射为规范编辑模型而不改变条目顺序或大小写
// canonicalKCESExportNameMap validates and copies a map into the canonical editing model without changing entry order or case
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

// decodeKCESExportNameMapJSONValue 严格解码一个 UTF-8 JSON 值并拒绝未知字段与尾随内容
// decodeKCESExportNameMapJSONValue strictly decodes one UTF-8 JSON value and rejects unknown fields and trailing content
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
