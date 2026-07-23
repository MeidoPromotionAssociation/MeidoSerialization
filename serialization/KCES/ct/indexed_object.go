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

// indexedObjectField 描述一个导出的 MessagePack 槽位
// 编解码库负责实际值转换，此描述符只用于保留数组宽度和新于 Go 模型的原始槽位
// indexedObjectField describes one exported MessagePack slot
// The codec library handles actual value conversion while this descriptor only retains array width and raw slots newer than the Go model
type indexedObjectField struct {
	index []int  // reflect.FieldByIndex 使用的嵌套字段路径 / Nested field path used by reflect.FieldByIndex
	name  string // 用于错误信息的 JSON 或 Go 字段名称 / JSON or Go field name used in errors
}

// indexedObjectLayout 缓存一个 Go 结构体映射到 MessagePack-CSharp int-key 数组的反射布局
// indexedObjectLayout caches the reflection layout mapping one Go struct to a MessagePack-CSharp int-key array
type indexedObjectLayout struct {
	typ           reflect.Type         // 模型结构体类型 / Model struct type
	metadataIndex int                  // 外层元数据字段的直接结构体索引 / Direct struct index of the outer metadata field
	metadataPtr   bool                 // 元数据字段是否为指针 / Whether the metadata field is a pointer
	metadataTyped bool                 // 元数据是否支持嵌套 null 标注 / Whether the metadata supports nested null annotations
	fields        []indexedObjectField // 按线格式 Key 顺序展平的已知字段 / Known fields flattened in wire-key order
	label         string               // 用于校验错误的类型标签 / Type label used in validation errors
}

var (
	indexedObjectMetadataType         = reflect.TypeOf(IndexedObjectMetadata{})
	indexedObjectMetadataPtrType      = reflect.PointerTo(indexedObjectMetadataType)
	typedIndexedObjectMetadataType    = reflect.TypeOf(TypedIndexedObjectMetadata{})
	typedIndexedObjectMetadataPtrType = reflect.PointerTo(typedIndexedObjectMetadataType)
	codecRawType                      = reflect.TypeOf(codec.Raw{})
	emptyInterfaceType                = reflect.TypeOf((*interface{})(nil)).Elem()
	// indexedObjectLayouts 按 reflect.Type 缓存已验证布局
	// indexedObjectLayouts caches validated layouts by reflect.Type
	indexedObjectLayouts sync.Map
)

// EncodeIndexedObjectSelf 是 MessagePack-CSharp int-key 契约共用的 codec.Selfer 编码实现
// 已知字段仍由 ugorji/codec 编码，此适配器只恢复解码出的短宽度并以 codec.Raw 追加已校验未来槽位
// 使用者必须嵌入带 codec:"-" 的 IndexedObjectMetadata 并公开 codec:",toarray" 标记
// 外层编码器还必须启用 MsgpackHandle.Raw，EncodeIndexedMsgpack 会在本函数校验所有原始未来槽位后安全完成该设置
// EncodeIndexedObjectSelf is the shared codec.Selfer encoder for MessagePack-CSharp int-key contracts
// Known fields remain encoded by ugorji/codec while this adapter only restores a decoded short width and appends validated future slots as codec.Raw
// Users must embed IndexedObjectMetadata with codec:"-" and expose the codec:",toarray" marker
// The surrounding encoder must also enable MsgpackHandle.Raw, which EncodeIndexedMsgpack safely does after this function validates every raw future slot
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

// DecodeIndexedObjectSelf 是 EncodeIndexedObjectSelf 对应的解码实现
// ugorji/codec 将数组拆成 codec.Raw 项，因此未来槽位内的扩展值、重复映射键和非标准数值标记可逐字节保留
// 每个已知项随后由同一编解码库解码到声明的 Go 字段类型
// DecodeIndexedObjectSelf is the decoding counterpart of EncodeIndexedObjectSelf
// ugorji/codec splits the array into codec.Raw entries so extension values, duplicate map keys inside future slots, and non-canonical numeric markers remain byte-for-byte intact
// Each known entry is then decoded by the same codec library into its declared Go field type
func DecodeIndexedObjectSelf(d *codec.Decoder, value interface{}) {
	rv, layout, err := indexedObjectValue(value)
	if err != nil {
		panic(err)
	}

	var slots []codec.Raw
	d.MustDecode(&slots)

	// 解码到复用值必须与解码到新分配 C# 契约结果一致
	// 缺失 Key 保持线模型零值，而不是旧 Go 数据或构造及迁移默认值
	// Decoding into a reused value must match decoding into a freshly allocated C# contract
	// Omitted keys retain wire-model zero values rather than stale Go data or constructor and migration defaults
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
			// ugorji 将 codec.Raw 捕获的 nil 表示为空切片
			// ugorji represents nil captured in codec.Raw as an empty slice
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

// decodeIndexedObjectKnownField 让 ugorji/codec 处理普通值，同时为 Go float32 字段匹配 MessagePack-CSharp ReadSingle 转换
// ugorji 会拒绝溢出 float32 的有限 float64，而游戏使用的 CLR 转换会产生正负 Infinity
// MessagePack-CSharp ReadSingle 也接受整数标记
// decodeIndexedObjectKnownField leaves ordinary values to ugorji/codec while matching MessagePack-CSharp ReadSingle conversion for Go float32 fields
// ugorji rejects finite float64 values that overflow float32 while the CLR conversion used by the game produces positive or negative Infinity
// MessagePack-CSharp ReadSingle also accepts integer markers
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

// decodeMessagePackSingle 将 nil、浮点或任一整数 MessagePack 值按 ReadSingle 规则转换为 float32
// decodeMessagePackSingle converts nil, floating-point, or any integer MessagePack value to float32 using ReadSingle rules
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

// indexedObjectValue 验证输入为非 nil 结构体指针并返回值及缓存布局
// indexedObjectValue verifies that input is a non-nil struct pointer and returns its value and cached layout
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

// indexedObjectLayoutFor 构建并缓存结构体的 toarray、元数据及展平字段布局
// indexedObjectLayoutFor builds and caches the toarray marker, metadata, and flattened field layout of a struct
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
		// 未导出字段是标记或实现细节，不占用线格式槽位
		// Unexported fields are markers or implementation details and do not occupy wire slots
		if field.PkgPath != "" {
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

// indexedObjectInlineFields 展平显式标注 inline 的嵌入值结构体
// MessagePack-CSharp int-key 契约常继承 Key 位于派生类之前的基类，Go 模型以嵌入结构体表示该形状
// 在此展平使这些模型可与普通 indexed object 共用宽度、null 和未来槽位机制
// indexedObjectInlineFields flattens embedded value structs explicitly tagged inline
// MessagePack-CSharp int-key contracts often inherit a base class whose keys precede derived keys, represented by embedded structs in Go
// Flattening here lets those models share width, null, and future-slot machinery with ordinary indexed objects
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

// indexedObjectJSONFieldName 返回用于错误信息的 JSON 字段名，缺失时回退到 Go 名称
// indexedObjectJSONFieldName returns the JSON field name used in errors and falls back to the Go name when absent
func indexedObjectJSONFieldName(field reflect.StructField) string {
	name := field.Name
	if jsonName := strings.Split(field.Tag.Get("json"), ",")[0]; jsonName != "" && jsonName != "-" {
		name = jsonName
	}
	return name
}

// codecTagHasOption 判断 codec 标签在主名称之后是否包含指定选项
// codecTagHasOption reports whether a codec tag contains the requested option after its primary name
func codecTagHasOption(tag, option string) bool {
	parts := strings.Split(tag, ",")
	for _, part := range parts[1:] {
		if part == option {
			return true
		}
	}
	return false
}

// indexedObjectMetadataValue 将指针或值形式的新旧元数据统一转换为 TypedIndexedObjectMetadata
// indexedObjectMetadataValue normalizes pointer or value forms of legacy and typed metadata into TypedIndexedObjectMetadata
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

// setIndexedObjectMetadataField 将统一元数据写回模型声明的新旧指针或值字段
// setIndexedObjectMetadataField writes normalized metadata back to the legacy or typed pointer or value field declared by the model
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

// typedIndexedObjectMetadataHasValue 判断元数据是否包含任何需要保留的线格式差异
// typedIndexedObjectMetadataHasValue reports whether metadata contains any wire difference that must be preserved
func typedIndexedObjectMetadataHasValue(metadata TypedIndexedObjectMetadata) bool {
	return metadata.FieldCount != nil || len(metadata.FutureSlots) != 0 || len(metadata.NilSlots) != 0 || len(metadata.NullElements) != 0 || len(metadata.NullMapValueKeys) != 0
}

// validateTypedIndexedObjectMetadata 验证每种 null 标注只引用存在的已知槽位且彼此不冲突
// validateTypedIndexedObjectMetadata verifies that each null annotation references an existing known slot and does not conflict with another annotation
func validateTypedIndexedObjectMetadata(metadata TypedIndexedObjectMetadata, count, known int64, label string) error {
	annotations := make(map[int32]string)
	claim := func(slot int32, kind string) error {
		if slot < 0 || int64(slot) >= known {
			return fmt.Errorf("%s %s references unknown slot %d (known slots: 0..%d)", label, kind, slot, known-1)
		}
		if int64(slot) >= count {
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

// indexedObjectKnownWireValue 根据槽位 null 标注构造已知字段应写出的线格式值
// indexedObjectKnownWireValue constructs the wire value for a known field according to its slot null annotation
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

// indexedObjectNullElementWireValue 将值切片或数组中标注的零值项目恢复为 MessagePack nil
// indexedObjectNullElementWireValue restores annotated zero-valued entries of a value slice or array as MessagePack nil
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

// indexedObjectNullMapWireValue 将值映射中由完整原始 Key 标注的零值恢复为 MessagePack nil
// indexedObjectNullMapWireValue restores zero values in a value map as MessagePack nil using complete raw key annotations
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

// captureIndexedObjectNestedNulls 从已知集合槽位的原始值中捕获 Go 值类型无法表达的嵌套 nil
// captureIndexedObjectNestedNulls captures nested nil values from a known collection slot when its Go value type cannot express them
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

// decodeNullMapValueKeys 找出值映射中值为 nil 的 Key 并以确定顺序返回各 Key 的完整 MessagePack 字节
// decodeNullMapValueKeys finds keys whose values are nil in a value map and returns complete MessagePack bytes for each key in deterministic order
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

// typeCanRepresentNil 判断 Go 类型自身是否能表示 nil
// typeCanRepresentNil reports whether a Go type can represent nil directly
func typeCanRepresentNil(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return true
	default:
		return false
	}
}

// isRawMessagePackNil 判断 codec.Raw 的空切片或单字节 0xc0 是否表示 MessagePack nil
// isRawMessagePackNil reports whether an empty codec.Raw slice or single byte 0xc0 represents MessagePack nil
func isRawMessagePackNil(raw []byte) bool {
	return len(raw) == 0 || (len(raw) == 1 && raw[0] == 0xc0)
}

// int32SliceContains 判断 Int32 切片是否包含目标槽位索引
// int32SliceContains reports whether an Int32 slice contains the target slot index
func int32SliceContains(values []int32, target int32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// hasTrueFlag 判断布尔标志切片中是否至少有一项为 true
// hasTrueFlag reports whether a Boolean flag slice contains at least one true entry
func hasTrueFlag(flags []bool) bool {
	for _, flag := range flags {
		if flag {
			return true
		}
	}
	return false
}

// trimFalseFlags 删除布尔切片尾部无意义的 false，并在全 false 时返回 nil
// trimFalseFlags removes insignificant trailing false values from a Boolean slice and returns nil when all entries are false
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
