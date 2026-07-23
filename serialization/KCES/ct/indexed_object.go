package ct

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/ugorji/go/codec"
)

// indexedObjectField describes one exported MessagePack slot. The codec
// library already handles the actual value conversion; this small descriptor
// only lets us retain the array width and raw slots that are newer than the Go
// model.
type indexedObjectField struct {
	index []int
	name  string
}

type indexedObjectLayout struct {
	typ           reflect.Type
	metadataIndex int
	metadataPtr   bool
	metadataTyped bool
	fields        []indexedObjectField
	label         string
}

var (
	indexedObjectMetadataType         = reflect.TypeOf(IndexedObjectMetadata{})
	indexedObjectMetadataPtrType      = reflect.PointerTo(indexedObjectMetadataType)
	typedIndexedObjectMetadataType    = reflect.TypeOf(TypedIndexedObjectMetadata{})
	typedIndexedObjectMetadataPtrType = reflect.PointerTo(typedIndexedObjectMetadataType)
	codecRawType                      = reflect.TypeOf(codec.Raw{})
	emptyInterfaceType                = reflect.TypeOf((*interface{})(nil)).Elem()
	indexedObjectLayouts              sync.Map // map[reflect.Type]*indexedObjectLayout
)

// EncodeIndexedObjectSelf is the shared codec.Selfer implementation for
// MessagePack-CSharp int-key contracts. Known fields are still encoded by
// ugorji/codec. This adapter restores decoded short widths and appends
// already-validated future slots as codec.Raw values.
//
// Types using this helper must embed IndexedObjectMetadata with codec:"-" and
// expose the normal codec:",toarray" marker. The surrounding encoder must have
// MsgpackHandle.Raw enabled; EncodeIndexedMsgpack does that safely after this
// helper has validated every raw future slot.
func EncodeIndexedObjectSelf(e *codec.Encoder, value interface{}) {
	rv, layout, err := indexedObjectValue(value)
	if err != nil {
		panic(err)
	}

	metadata := indexedObjectMetadataValue(rv.Field(layout.metadataIndex), layout.metadataPtr)
	count, err := resolveIndexedObjectFieldCount(metadata.FieldCount, int64(len(layout.fields)), metadata.FutureSlots, layout.label)
	if err != nil {
		panic(err)
	}
	if err := validateTypedIndexedObjectMetadata(metadata, count, int64(len(layout.fields)), layout.label); err != nil {
		panic(err)
	}

	for slot := count; slot < int64(len(layout.fields)); slot++ {
		field := rv.FieldByIndex(layout.fields[slot].index)
		if !field.IsZero() {
			panic(fmt.Errorf("%s fieldCount %d would discard %s", layout.label, count, layout.fields[slot].name))
		}
	}

	values := make([]interface{}, 0, count)
	for slot := int64(0); slot < count && slot < int64(len(layout.fields)); slot++ {
		value, err := indexedObjectKnownWireValue(rv.FieldByIndex(layout.fields[slot].index), slot, metadata, layout.label, layout.fields[slot].name)
		if err != nil {
			panic(err)
		}
		values = append(values, value)
	}
	for _, raw := range metadata.FutureSlots {
		if len(raw) == 0 {
			raw = []byte{0xc0}
		}
		values = append(values, codec.Raw(append([]byte(nil), raw...)))
	}
	e.MustEncode(values)
}

// DecodeIndexedObjectSelf is the decoding half of EncodeIndexedObjectSelf.
// ugorji/codec splits the array into codec.Raw elements, so extension values,
// duplicate map keys inside a future slot, and non-canonical number markers
// remain byte-for-byte intact. Each known element is then decoded by the same
// library into its declared Go field type.
func DecodeIndexedObjectSelf(d *codec.Decoder, value interface{}) {
	rv, layout, err := indexedObjectValue(value)
	if err != nil {
		panic(err)
	}

	var slots []codec.Raw
	d.MustDecode(&slots)

	// Decoding into a reused value must have the same result as decoding into a
	// freshly allocated C# contract: omitted keys retain their wire-model zero
	// value, not stale Go data and not constructor/migration defaults.
	for _, field := range layout.fields {
		fv := rv.FieldByIndex(field.index)
		fv.Set(reflect.Zero(fv.Type()))
	}
	metadataField := rv.Field(layout.metadataIndex)
	metadataField.Set(reflect.Zero(metadataField.Type()))

	known := len(layout.fields)
	metadata := TypedIndexedObjectMetadata{}
	if len(slots) != known {
		count, err := checkedInt32Count(int64(len(slots)), layout.label+" field count")
		if err != nil {
			panic(err)
		}
		metadata.FieldCount = &count
	}
	if len(slots) > known {
		metadata.FutureSlots = cloneCodecRawSlots(slots[known:])
	}
	for slot := int64(0); slot < int64(len(slots)) && slot < int64(known); slot++ {
		raw := []byte(slots[slot])
		isNil := isRawMessagePackNil(raw)
		if isNil {
			// ugorji represents a nil captured in codec.Raw as an empty slice.
			raw = []byte{0xc0}
		}
		field := rv.FieldByIndex(layout.fields[slot].index)
		if isNil && !typeCanRepresentNil(field.Type()) {
			metadata.NilSlots = append(metadata.NilSlots, int32(slot))
		}
		if err := decodeIndexedObjectKnownField(raw, field); err != nil {
			panic(fmt.Errorf("decode %s slot %d (%s): %w", layout.label, slot, layout.fields[slot].name, err))
		}
		if !isNil {
			if err := captureIndexedObjectNestedNulls(&metadata, slot, raw, field.Type(), layout.label, layout.fields[slot].name); err != nil {
				panic(err)
			}
		}
	}
	setIndexedObjectMetadataField(metadataField, layout, metadata)
}

// decodeIndexedObjectKnownField keeps ugorji/codec responsible for ordinary
// values, while matching MessagePack-CSharp's ReadSingle conversion for Go
// float32 fields. ugorji deliberately rejects a finite float64 that overflows
// float32; the CLR conversion used by the game instead produces +/-Infinity.
// MessagePack-CSharp also accepts integer markers in ReadSingle.
func decodeIndexedObjectKnownField(raw []byte, field reflect.Value) error {
	if field.Kind() == reflect.Int32 {
		value, err := decodeMessagePackInt32(raw)
		if err != nil {
			return err
		}
		field.SetInt(int64(value))
		return nil
	}
	if field.Kind() == reflect.Float32 {
		value, err := decodeMessagePackSingle(raw)
		if err != nil {
			return err
		}
		field.SetFloat(float64(value))
		return nil
	}
	if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Float32 {
		if isRawMessagePackNil(raw) {
			field.Set(reflect.Zero(field.Type()))
			return nil
		}
		var elements []codec.Raw
		if err := DecodeMsgpack(raw, &elements); err != nil {
			return err
		}
		values := reflect.MakeSlice(field.Type(), len(elements), len(elements))
		for index := range elements {
			value, err := decodeMessagePackSingle(elements[index])
			if err != nil {
				return fmt.Errorf("element %d: %w", index, err)
			}
			values.Index(index).SetFloat(float64(value))
		}
		field.Set(values)
		return nil
	}
	return DecodeMsgpack(raw, field.Addr().Interface())
}

// decodeMessagePackInt32 按照 MessagePack-CSharp ReadInt32 的 checked 语义解码一个整数。
// decodeMessagePackInt32 decodes one integer with the checked semantics of MessagePack-CSharp ReadInt32.
func decodeMessagePackInt32(raw []byte) (int32, error) {
	if isRawMessagePackNil(raw) {
		return 0, nil
	}
	var value interface{}
	if err := DecodeMsgpack(raw, &value); err != nil {
		return 0, err
	}
	converted, ok := toInt32(value)
	if !ok {
		return 0, fmt.Errorf("value %v (%T) is not an integer in the C# Int32 range [%d,%d]", value, value, csharpInt32Min, csharpInt32Max)
	}
	return converted, nil
}

func decodeMessagePackSingle(raw []byte) (float32, error) {
	if isRawMessagePackNil(raw) {
		return 0, nil
	}
	var value interface{}
	if err := DecodeMsgpack(raw, &value); err != nil {
		return 0, err
	}
	switch number := value.(type) {
	case float32:
		return number, nil
	case float64:
		return float32(number), nil
	case int8:
		return float32(number), nil
	case int16:
		return float32(number), nil
	case int32:
		return float32(number), nil
	case int64:
		return float32(number), nil
	case uint8:
		return float32(number), nil
	case uint16:
		return float32(number), nil
	case uint32:
		return float32(number), nil
	case uint64:
		return float32(number), nil
	default:
		return 0, fmt.Errorf("expected a MessagePack number accepted by ReadSingle, got %T", value)
	}
}

func indexedObjectValue(value interface{}) (reflect.Value, *indexedObjectLayout, error) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Ptr || rv.IsNil() {
		return reflect.Value{}, nil, fmt.Errorf("indexed MessagePack object must be a non-nil pointer, got %T", value)
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}, nil, fmt.Errorf("indexed MessagePack object must point to a struct, got %T", value)
	}
	layout, err := indexedObjectLayoutFor(rv.Type())
	if err != nil {
		return reflect.Value{}, nil, err
	}
	return rv, layout, nil
}

func indexedObjectLayoutFor(typ reflect.Type) (*indexedObjectLayout, error) {
	if cached, ok := indexedObjectLayouts.Load(typ); ok {
		return cached.(*indexedObjectLayout), nil
	}

	layout := &indexedObjectLayout{typ: typ, metadataIndex: -1, label: typ.Name()}
	if layout.label == "" {
		layout.label = typ.String()
	}
	hasToArrayMarker := false
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		codecTag := field.Tag.Get("codec")
		if field.Name == "_struct" && strings.Contains(codecTag, "toarray") {
			hasToArrayMarker = true
			continue
		}
		isLegacyMetadata := field.Type == indexedObjectMetadataType || field.Type == indexedObjectMetadataPtrType
		isTypedMetadata := field.Type == typedIndexedObjectMetadataType || field.Type == typedIndexedObjectMetadataPtrType
		if isLegacyMetadata || isTypedMetadata {
			if layout.metadataIndex >= 0 {
				return nil, fmt.Errorf("%s embeds IndexedObjectMetadata more than once", layout.label)
			}
			if strings.Split(codecTag, ",")[0] != "-" {
				return nil, fmt.Errorf("%s IndexedObjectMetadata must use codec:\"-\"", layout.label)
			}
			layout.metadataIndex = index
			layout.metadataPtr = field.Type == indexedObjectMetadataPtrType || field.Type == typedIndexedObjectMetadataPtrType
			layout.metadataTyped = isTypedMetadata
			continue
		}
		if field.PkgPath != "" { // unexported marker or implementation detail
			continue
		}
		if strings.Split(codecTag, ",")[0] == "-" {
			continue
		}
		if codecTagHasOption(codecTag, "inline") {
			if !field.Anonymous || field.Type.Kind() != reflect.Struct {
				return nil, fmt.Errorf("%s field %s uses codec inline but is not an embedded struct", layout.label, field.Name)
			}
			inlineFields, err := indexedObjectInlineFields(field.Type, []int{index}, layout.label+"."+field.Name)
			if err != nil {
				return nil, err
			}
			layout.fields = append(layout.fields, inlineFields...)
			continue
		}
		layout.fields = append(layout.fields, indexedObjectField{index: []int{index}, name: indexedObjectJSONFieldName(field)})
	}
	if !hasToArrayMarker {
		return nil, fmt.Errorf("%s does not declare codec:\",toarray\"", layout.label)
	}
	if layout.metadataIndex < 0 {
		return nil, fmt.Errorf("%s does not embed IndexedObjectMetadata", layout.label)
	}

	actual, _ := indexedObjectLayouts.LoadOrStore(typ, layout)
	return actual.(*indexedObjectLayout), nil
}

// indexedObjectInlineFields flattens explicitly tagged embedded value structs.
// MessagePack-CSharp int-key contracts frequently inherit a base class whose
// keys precede the derived class keys. Go models represent that shape with an
// embedded struct; keeping the flattening here lets those models use the same
// width/null/future-slot machinery as ordinary indexed objects.
func indexedObjectInlineFields(typ reflect.Type, prefix []int, label string) ([]indexedObjectField, error) {
	fields := make([]indexedObjectField, 0, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		codecTag := field.Tag.Get("codec")
		if field.PkgPath != "" || strings.Split(codecTag, ",")[0] == "-" {
			continue
		}
		path := append(append([]int(nil), prefix...), index)
		if codecTagHasOption(codecTag, "inline") {
			if !field.Anonymous || field.Type.Kind() != reflect.Struct {
				return nil, fmt.Errorf("%s field %s uses codec inline but is not an embedded struct", label, field.Name)
			}
			nested, err := indexedObjectInlineFields(field.Type, path, label+"."+field.Name)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
			continue
		}
		if field.Type == indexedObjectMetadataType || field.Type == indexedObjectMetadataPtrType ||
			field.Type == typedIndexedObjectMetadataType || field.Type == typedIndexedObjectMetadataPtrType {
			return nil, fmt.Errorf("%s must keep IndexedObjectMetadata on the outer indexed object", label)
		}
		fields = append(fields, indexedObjectField{index: path, name: indexedObjectJSONFieldName(field)})
	}
	return fields, nil
}

func indexedObjectJSONFieldName(field reflect.StructField) string {
	name := field.Name
	if jsonName := strings.Split(field.Tag.Get("json"), ",")[0]; jsonName != "" && jsonName != "-" {
		name = jsonName
	}
	return name
}

func codecTagHasOption(tag, option string) bool {
	parts := strings.Split(tag, ",")
	for _, part := range parts[1:] {
		if part == option {
			return true
		}
	}
	return false
}

func indexedObjectMetadataValue(field reflect.Value, pointer bool) TypedIndexedObjectMetadata {
	if field.Type() == typedIndexedObjectMetadataType {
		return field.Interface().(TypedIndexedObjectMetadata)
	}
	if field.Type() == typedIndexedObjectMetadataPtrType {
		if field.IsNil() {
			return TypedIndexedObjectMetadata{}
		}
		return *field.Interface().(*TypedIndexedObjectMetadata)
	}
	var legacy IndexedObjectMetadata
	if pointer {
		if field.IsNil() {
			return TypedIndexedObjectMetadata{}
		}
		legacy = *field.Interface().(*IndexedObjectMetadata)
	} else {
		legacy = field.Interface().(IndexedObjectMetadata)
	}
	return TypedIndexedObjectMetadata{FieldCount: legacy.FieldCount, FutureSlots: legacy.FutureSlots}
}

func setIndexedObjectMetadataField(field reflect.Value, layout *indexedObjectLayout, metadata TypedIndexedObjectMetadata) {
	hasValue := typedIndexedObjectMetadataHasValue(metadata)
	if layout.metadataTyped {
		if layout.metadataPtr {
			if hasValue {
				field.Set(reflect.ValueOf(&metadata))
			}
			return
		}
		field.Set(reflect.ValueOf(metadata))
		return
	}
	if len(metadata.NilSlots) != 0 || len(metadata.NullElements) != 0 || len(metadata.NullMapValueKeys) != 0 {
		panic(fmt.Errorf("%s must use TypedIndexedObjectMetadata to retain nil markers", layout.label))
	}
	legacy := IndexedObjectMetadata{FieldCount: metadata.FieldCount, FutureSlots: metadata.FutureSlots}
	if layout.metadataPtr {
		if hasValue {
			field.Set(reflect.ValueOf(&legacy))
		}
		return
	}
	field.Set(reflect.ValueOf(legacy))
}

func typedIndexedObjectMetadataHasValue(metadata TypedIndexedObjectMetadata) bool {
	return metadata.FieldCount != nil || len(metadata.FutureSlots) != 0 || len(metadata.NilSlots) != 0 || len(metadata.NullElements) != 0 || len(metadata.NullMapValueKeys) != 0
}

func validateTypedIndexedObjectMetadata(metadata TypedIndexedObjectMetadata, count, known int64, label string) error {
	annotations := make(map[int32]string)
	claim := func(slot int32, kind string) error {
		if slot < 0 || int64(slot) >= int64(known) {
			return fmt.Errorf("%s %s references unknown slot %d (known slots: 0..%d)", label, kind, slot, known-1)
		}
		if int64(slot) >= int64(count) {
			return fmt.Errorf("%s %s references omitted slot %d at fieldCount %d", label, kind, slot, count)
		}
		if previous, exists := annotations[slot]; exists {
			return fmt.Errorf("%s slot %d is annotated by both %s and %s", label, slot, previous, kind)
		}
		annotations[slot] = kind
		return nil
	}
	for index, slot := range metadata.NilSlots {
		if err := claim(slot, fmt.Sprintf("nilSlots[%d]", index)); err != nil {
			return err
		}
	}
	for slot := range metadata.NullElements {
		if err := claim(slot, "nullElements"); err != nil {
			return err
		}
	}
	for slot := range metadata.NullMapValueKeys {
		if err := claim(slot, "nullMapValueKeys"); err != nil {
			return err
		}
	}
	return nil
}

func indexedObjectKnownWireValue(field reflect.Value, slot int64, metadata TypedIndexedObjectMetadata, label, fieldName string) (interface{}, error) {
	if int32SliceContains(metadata.NilSlots, int32(slot)) {
		if !field.IsZero() {
			return nil, fmt.Errorf("%s nilSlots contains slot %d (%s), which would discard a populated value", label, slot, fieldName)
		}
		return nil, nil
	}
	if flags, ok := metadata.NullElements[int32(slot)]; ok {
		return indexedObjectNullElementWireValue(field, flags, label, slot, fieldName)
	}
	if rawKeys, ok := metadata.NullMapValueKeys[int32(slot)]; ok {
		return indexedObjectNullMapWireValue(field, rawKeys, label, slot, fieldName)
	}
	return field.Interface(), nil
}

func indexedObjectNullElementWireValue(field reflect.Value, flags []bool, label string, slot int64, fieldName string) (interface{}, error) {
	if field.Kind() != reflect.Slice && field.Kind() != reflect.Array {
		return nil, fmt.Errorf("%s nullElements slot %d (%s) requires a slice or array, got %s", label, slot, fieldName, field.Type())
	}
	if field.Kind() == reflect.Slice && field.IsNil() {
		if len(flags) != 0 {
			return nil, fmt.Errorf("%s nullElements slot %d (%s) cannot annotate a nil collection", label, slot, fieldName)
		}
		return field.Interface(), nil
	}
	if len(flags) > field.Len() {
		return nil, fmt.Errorf("%s nullElements slot %d (%s) has %d flags for %d elements", label, slot, fieldName, len(flags), field.Len())
	}
	if !hasTrueFlag(flags) {
		return field.Interface(), nil
	}
	values := make([]interface{}, field.Len())
	for index := 0; index < field.Len(); index++ {
		element := field.Index(index)
		if index < len(flags) && flags[index] {
			if !element.IsZero() {
				return nil, fmt.Errorf("%s nullElements slot %d (%s)[%d] would discard a populated value", label, slot, fieldName, index)
			}
			values[index] = nil
		} else {
			values[index] = element.Interface()
		}
	}
	return values, nil
}

func indexedObjectNullMapWireValue(field reflect.Value, rawKeys [][]byte, label string, slot int64, fieldName string) (interface{}, error) {
	if field.Kind() != reflect.Map {
		return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s) requires a map, got %s", label, slot, fieldName, field.Type())
	}
	if field.IsNil() {
		if len(rawKeys) != 0 {
			return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s) cannot annotate a nil map", label, slot, fieldName)
		}
		return field.Interface(), nil
	}
	if len(rawKeys) == 0 {
		return field.Interface(), nil
	}

	nullKeySet := reflect.MakeMapWithSize(reflect.MapOf(field.Type().Key(), reflect.TypeOf(false)), len(rawKeys))
	for index, rawKey := range rawKeys {
		if len(rawKey) == 0 {
			return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s)[%d] is empty", label, slot, fieldName, index)
		}
		root, trailing, err := SplitFirstMsgpackValue(rawKey)
		if err != nil {
			return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s)[%d]: %w", label, slot, fieldName, index, err)
		}
		if len(trailing) != 0 {
			return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s)[%d] has %d trailing bytes", label, slot, fieldName, index, len(trailing))
		}
		key := reflect.New(field.Type().Key())
		if err := DecodeMsgpack(root, key.Interface()); err != nil {
			return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s)[%d] key decode: %w", label, slot, fieldName, index, err)
		}
		keyValue := key.Elem()
		if nullKeySet.MapIndex(keyValue).IsValid() {
			return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s) contains a duplicate key", label, slot, fieldName)
		}
		mapValue := field.MapIndex(keyValue)
		if !mapValue.IsValid() {
			return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s)[%d] refers to a missing map key", label, slot, fieldName, index)
		}
		if !mapValue.IsZero() {
			return nil, fmt.Errorf("%s nullMapValueKeys slot %d (%s)[%d] would discard a populated map value", label, slot, fieldName, index)
		}
		nullKeySet.SetMapIndex(keyValue, reflect.ValueOf(true))
	}

	wireMap := reflect.MakeMapWithSize(reflect.MapOf(field.Type().Key(), emptyInterfaceType), field.Len())
	iterator := field.MapRange()
	for iterator.Next() {
		key := iterator.Key()
		var value reflect.Value
		if nullKeySet.MapIndex(key).IsValid() {
			value = reflect.Zero(emptyInterfaceType)
		} else {
			value = reflect.ValueOf(iterator.Value().Interface())
		}
		wireMap.SetMapIndex(key, value)
	}
	return wireMap.Interface(), nil
}

func captureIndexedObjectNestedNulls(metadata *TypedIndexedObjectMetadata, slot int64, raw []byte, fieldType reflect.Type, label, fieldName string) error {
	switch fieldType.Kind() {
	case reflect.Slice, reflect.Array:
		if fieldType.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		var elements []codec.Raw
		if err := DecodeMsgpack(raw, &elements); err != nil {
			return fmt.Errorf("decode %s slot %d (%s) raw elements: %w", label, slot, fieldName, err)
		}
		flags := make([]bool, len(elements))
		for index := range elements {
			flags[index] = isRawMessagePackNil(elements[index]) && !typeCanRepresentNil(fieldType.Elem())
		}
		flags = trimFalseFlags(flags)
		if len(flags) != 0 {
			if metadata.NullElements == nil {
				metadata.NullElements = make(map[int32][]bool)
			}
			metadata.NullElements[int32(slot)] = flags
		}
	case reflect.Map:
		if typeCanRepresentNil(fieldType.Elem()) {
			return nil
		}
		rawKeys, err := decodeNullMapValueKeys(raw, fieldType, label, slot, fieldName)
		if err != nil {
			return err
		}
		if len(rawKeys) != 0 {
			if metadata.NullMapValueKeys == nil {
				metadata.NullMapValueKeys = make(map[int32][][]byte)
			}
			metadata.NullMapValueKeys[int32(slot)] = rawKeys
		}
	}
	return nil
}

func decodeNullMapValueKeys(raw []byte, mapType reflect.Type, label string, slot int64, fieldName string) ([][]byte, error) {
	count, err := messagePackMapLength(raw)
	if err != nil {
		return nil, fmt.Errorf("decode %s slot %d (%s) map header: %w", label, slot, fieldName, err)
	}
	rawMapType := reflect.MapOf(mapType.Key(), codecRawType)
	rawMapPointer := reflect.New(rawMapType)
	if err := DecodeMsgpack(raw, rawMapPointer.Interface()); err != nil {
		return nil, fmt.Errorf("decode %s slot %d (%s) raw map: %w", label, slot, fieldName, err)
	}
	rawMap := rawMapPointer.Elem()
	if int64(rawMap.Len()) != count {
		return nil, fmt.Errorf("decode %s slot %d (%s): map contains duplicate keys that the Go model cannot preserve", label, slot, fieldName)
	}
	keys := make([][]byte, 0)
	iterator := rawMap.MapRange()
	for iterator.Next() {
		if !isRawMessagePackNil(iterator.Value().Interface().(codec.Raw)) {
			continue
		}
		encodedKey, err := EncodeMsgpack(iterator.Key().Interface())
		if err != nil {
			return nil, fmt.Errorf("encode %s slot %d (%s) null map key: %w", label, slot, fieldName, err)
		}
		keys = append(keys, encodedKey)
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i], keys[j]) < 0 })
	return keys, nil
}

func typeCanRepresentNil(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return true
	default:
		return false
	}
}

func isRawMessagePackNil(raw []byte) bool {
	return len(raw) == 0 || (len(raw) == 1 && raw[0] == 0xc0)
}

func int32SliceContains(values []int32, target int32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasTrueFlag(flags []bool) bool {
	for _, flag := range flags {
		if flag {
			return true
		}
	}
	return false
}

func trimFalseFlags(flags []bool) []bool {
	last := len(flags)
	for last > 0 && !flags[last-1] {
		last--
	}
	if last == 0 {
		return nil
	}
	return append([]bool(nil), flags[:last]...)
}
