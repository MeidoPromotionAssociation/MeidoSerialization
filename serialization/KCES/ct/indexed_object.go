package ct

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ugorji/go/codec"
)

// indexedObjectField 描述一个导出的 MessagePack 槽位
// 编解码库负责实际值转换，此描述符只映射当前支持的固定槽位
// indexedObjectField describes one exported MessagePack slot
// The codec library handles actual value conversion while this descriptor only maps the currently supported fixed slots
type indexedObjectField struct {
	index  []int  // reflect.FieldByIndex 使用的嵌套字段路径 / Nested field path used by reflect.FieldByIndex
	name   string // 用于错误信息的 JSON 或 Go 字段名称 / JSON or Go field name used in errors
	sparse bool   // 当前 C# 类型未声明且必须为 nil 的稀疏 Key / Sparse key absent from the current C# type and required to be nil
}

// indexedObjectLayout 缓存一个 Go 结构体映射到 MessagePack-CSharp int-key 数组的反射布局
// indexedObjectLayout caches the reflection layout mapping one Go struct to a MessagePack-CSharp int-key array
type indexedObjectLayout struct {
	typ          reflect.Type         // 模型结构体类型 / Model struct type
	fields       []indexedObjectField // 按线格式 Key 顺序展平的已知字段 / Known fields flattened in wire-key order
	decodeWidths []int                // 游戏已知历史版本允许的精确数组宽度 / Exact array widths allowed for known game revisions
	label        string               // 用于校验错误的类型标签 / Type label used in validation errors
}

var (
	codecRawType = reflect.TypeOf(codec.Raw{})
	// indexedObjectLayouts 按 reflect.Type 缓存已验证布局
	// indexedObjectLayouts caches validated layouts by reflect.Type
	indexedObjectLayouts sync.Map
)

// EncodeIndexedObjectSelf 按当前支持的固定 int-key 布局编码 MessagePack 对象，稀疏 Key 始终写为 nil
// EncodeIndexedObjectSelf encodes a MessagePack object using the currently supported fixed int-key layout and always writes sparse keys as nil
func EncodeIndexedObjectSelf(e *codec.Encoder, value interface{}) {
	rv, layout, err := indexedObjectValue(value)
	if err != nil {
		panic(err)
	}

	values := make([]interface{}, len(layout.fields))
	for slot := range layout.fields {
		field := layout.fields[slot]
		if field.sparse {
			values[slot] = nil
			continue
		}
		values[slot] = rv.FieldByIndex(field.index).Interface()
	}
	e.MustEncode(values)
}

// DecodeIndexedObjectSelf 按当前支持的固定 int-key 布局解码对象，并拒绝短数组、高位 Key、非 nil 稀疏槽以及值类型中的 nil
// DecodeIndexedObjectSelf decodes an object using the currently supported fixed int-key layout and rejects short arrays, high keys, non-nil sparse slots, and nil in value-typed positions
func DecodeIndexedObjectSelf(d *codec.Decoder, value interface{}) {
	rv, layout, err := indexedObjectValue(value)
	if err != nil {
		panic(err)
	}

	var slots []codec.Raw
	d.MustDecode(&slots)
	if !layout.supportsDecodeWidth(len(slots)) {
		panic(fmt.Errorf("unsupported %s indexed-array width %d, supported widths are %s", layout.label, len(slots), formatIndexedObjectWidths(layout.decodeWidths)))
	}

	for _, field := range layout.fields {
		if field.sparse {
			continue
		}
		fv := rv.FieldByIndex(field.index)
		fv.Set(reflect.Zero(fv.Type()))
	}
	for slot := int64(0); slot < int64(len(slots)); slot++ {
		raw := []byte(slots[slot])
		isNil := isRawMessagePackNil(raw)
		layoutField := layout.fields[slot]
		if layoutField.sparse {
			if !isNil {
				panic(fmt.Errorf("%s sparse slot %d must be nil", layout.label, slot))
			}
			continue
		}
		if isNil {
			raw = []byte{0xc0}
		}
		field := rv.FieldByIndex(layoutField.index)
		if isNil && !typeCanRepresentNil(field.Type()) {
			panic(fmt.Errorf("decode %s slot %d (%s): nil is not valid for %s", layout.label, slot, layoutField.name, field.Type()))
		}
		if err := decodeIndexedObjectKnownField(raw, field); err != nil {
			panic(fmt.Errorf("decode %s slot %d (%s): %w", layout.label, slot, layoutField.name, err))
		}
		if !isNil {
			if err := rejectIndexedObjectNestedNulls(slot, raw, field.Type(), layout.label, layoutField.name); err != nil {
				panic(err)
			}
		}
	}
}

// supportsDecodeWidth 判断数组宽度是否属于游戏中已知且完整建模的布局
// supportsDecodeWidth reports whether an array width is a known game layout that is fully modeled
func (layout *indexedObjectLayout) supportsDecodeWidth(width int) bool {
	for _, supported := range layout.decodeWidths {
		if width == supported {
			return true
		}
	}
	return false
}

// formatIndexedObjectWidths 格式化受支持宽度列表供严格解码错误使用
// formatIndexedObjectWidths formats supported widths for strict decoding errors
func formatIndexedObjectWidths(widths []int) string {
	values := make([]string, len(widths))
	for index, width := range widths {
		values[index] = strconv.Itoa(width)
	}
	return strings.Join(values, ", ")
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

// indexedObjectLayoutFor 构建并缓存结构体的 toarray 标记和展平字段布局
// indexedObjectLayoutFor builds and caches the toarray marker and flattened field layout of a struct
func indexedObjectLayoutFor(typ reflect.Type) (*indexedObjectLayout, error) {
	if cached, ok := indexedObjectLayouts.Load(typ); ok {
		return cached.(*indexedObjectLayout), nil
	}

	layout := &indexedObjectLayout{typ: typ, label: typ.Name()}
	if layout.label == "" {
		layout.label = typ.String()
	}
	hasToArrayMarker := false
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		codecTag := field.Tag.Get("codec")
		if field.Name == "_struct" && strings.Contains(codecTag, "toarray") {
			hasToArrayMarker = true
			sparseSlots, decodeWidths, err := parseIndexedObjectLayoutTag(field.Tag.Get("kces"), layout.label)
			if err != nil {
				return nil, err
			}
			layout.decodeWidths = decodeWidths
			for _, slot := range sparseSlots {
				layout.fields = append(layout.fields, indexedObjectField{name: fmt.Sprintf("sparse key %d", slot), sparse: true, index: []int{-int(slot) - 1}})
			}
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
	if err := arrangeSparseIndexedObjectFields(layout); err != nil {
		return nil, err
	}
	if err := validateIndexedObjectDecodeWidths(layout); err != nil {
		return nil, err
	}

	actual, _ := indexedObjectLayouts.LoadOrStore(typ, layout)
	return actual.(*indexedObjectLayout), nil
}

// parseIndexedObjectLayoutTag 解析固定 nil Key 和已知历史数组宽度
// parseIndexedObjectLayoutTag parses fixed nil keys and known historical array widths
func parseIndexedObjectLayoutTag(tag, label string) ([]int64, []int, error) {
	if tag == "" {
		return nil, nil, nil
	}
	var sparseSlots []int64
	var decodeWidths []int
	for _, directive := range strings.Split(tag, ";") {
		name, value, found := strings.Cut(directive, "=")
		if !found || value == "" {
			return nil, nil, fmt.Errorf("%s has unsupported kces layout directive %q", label, directive)
		}
		switch name {
		case "nil":
			seen := make(map[int64]struct{})
			for _, part := range strings.Split(value, ",") {
				slot, err := strconv.ParseInt(part, 10, 32)
				if err != nil || slot < 0 {
					return nil, nil, fmt.Errorf("%s has invalid sparse key %q", label, part)
				}
				if _, exists := seen[slot]; exists {
					return nil, nil, fmt.Errorf("%s declares sparse key %d more than once", label, slot)
				}
				seen[slot] = struct{}{}
				sparseSlots = append(sparseSlots, slot)
			}
		case "widths":
			seen := make(map[int]struct{})
			for _, part := range strings.Split(value, ",") {
				width64, err := strconv.ParseInt(part, 10, 32)
				if err != nil || width64 <= 0 {
					return nil, nil, fmt.Errorf("%s has invalid indexed-array width %q", label, part)
				}
				width := int(width64)
				if _, exists := seen[width]; exists {
					return nil, nil, fmt.Errorf("%s declares indexed-array width %d more than once", label, width)
				}
				seen[width] = struct{}{}
				decodeWidths = append(decodeWidths, width)
			}
		default:
			return nil, nil, fmt.Errorf("%s has unsupported kces layout directive %q", label, directive)
		}
	}
	sort.Slice(sparseSlots, func(i, j int) bool { return sparseSlots[i] < sparseSlots[j] })
	sort.Ints(decodeWidths)
	return sparseSlots, decodeWidths, nil
}

// validateIndexedObjectDecodeWidths 校验声明的历史宽度均为当前完整模型的前缀并包含当前宽度
// validateIndexedObjectDecodeWidths validates that declared historical widths are prefixes of the complete model and include its current width
func validateIndexedObjectDecodeWidths(layout *indexedObjectLayout) error {
	currentWidth := len(layout.fields)
	if len(layout.decodeWidths) == 0 {
		layout.decodeWidths = []int{currentWidth}
		return nil
	}
	hasCurrent := false
	for _, width := range layout.decodeWidths {
		if width > currentWidth {
			return fmt.Errorf("%s declares indexed-array width %d beyond current width %d", layout.label, width, currentWidth)
		}
		if width == currentWidth {
			hasCurrent = true
		}
	}
	if !hasCurrent {
		return fmt.Errorf("%s known widths do not include current width %d", layout.label, currentWidth)
	}
	return nil
}

// arrangeSparseIndexedObjectFields 将临时稀疏 Key 声明与普通字段合并为完整固定槽位布局
// arrangeSparseIndexedObjectFields merges temporary sparse-key declarations with ordinary fields into the complete fixed slot layout
func arrangeSparseIndexedObjectFields(layout *indexedObjectLayout) error {
	var sparse []int64
	ordinary := make([]indexedObjectField, 0, len(layout.fields))
	for _, field := range layout.fields {
		if field.sparse {
			sparse = append(sparse, -int64(field.index[0])-1)
			continue
		}
		ordinary = append(ordinary, field)
	}
	if len(sparse) == 0 {
		layout.fields = ordinary
		return nil
	}
	total := int64(len(ordinary) + len(sparse))
	if sparse[len(sparse)-1] >= total {
		return fmt.Errorf("%s sparse key %d is outside fixed width %d", layout.label, sparse[len(sparse)-1], total)
	}
	sparseSet := make(map[int64]struct{}, len(sparse))
	for _, slot := range sparse {
		sparseSet[slot] = struct{}{}
	}
	arranged := make([]indexedObjectField, 0, total)
	ordinaryIndex := int64(0)
	for slot := int64(0); slot < total; slot++ {
		if _, ok := sparseSet[slot]; ok {
			arranged = append(arranged, indexedObjectField{name: fmt.Sprintf("sparse key %d", slot), sparse: true})
			continue
		}
		arranged = append(arranged, ordinary[ordinaryIndex])
		ordinaryIndex++
	}
	layout.fields = arranged
	return nil
}

// indexedObjectInlineFields 展平显式标注 inline 的嵌入值结构体
// MessagePack-CSharp int-key 契约常继承 Key 位于派生类之前的基类，Go 模型以嵌入结构体表示该形状
// 在此展平使这些模型可与普通 indexed object 共用固定宽度与 null 校验机制
// indexedObjectInlineFields flattens embedded value structs explicitly tagged inline
// MessagePack-CSharp int-key contracts often inherit a base class whose keys precede derived keys, represented by embedded structs in Go
// Flattening here lets those models share fixed-width and null validation with ordinary indexed objects
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

// rejectIndexedObjectNestedNulls 拒绝值类型集合元素或映射值中的 MessagePack nil
// rejectIndexedObjectNestedNulls rejects MessagePack nil in value-typed collection elements or map values
func rejectIndexedObjectNestedNulls(slot int64, raw []byte, fieldType reflect.Type, label, fieldName string) error {
	switch fieldType.Kind() {
	case reflect.Slice, reflect.Array:
		if fieldType.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		var elements []codec.Raw
		if err := DecodeMsgpack(raw, &elements); err != nil {
			return fmt.Errorf("decode %s slot %d (%s) raw elements: %w", label, slot, fieldName, err)
		}
		for index := range elements {
			if isRawMessagePackNil(elements[index]) && !typeCanRepresentNil(fieldType.Elem()) {
				return fmt.Errorf("decode %s slot %d (%s)[%d]: nil is not valid for %s", label, slot, fieldName, index, fieldType.Elem())
			}
		}
	case reflect.Map:
		if typeCanRepresentNil(fieldType.Elem()) {
			return nil
		}
		count, err := messagePackMapLength(raw)
		if err != nil {
			return fmt.Errorf("decode %s slot %d (%s) map header: %w", label, slot, fieldName, err)
		}
		rawMapType := reflect.MapOf(fieldType.Key(), codecRawType)
		rawMapPointer := reflect.New(rawMapType)
		if err := DecodeMsgpack(raw, rawMapPointer.Interface()); err != nil {
			return fmt.Errorf("decode %s slot %d (%s) raw map: %w", label, slot, fieldName, err)
		}
		rawMap := rawMapPointer.Elem()
		if int64(rawMap.Len()) != count {
			return fmt.Errorf("decode %s slot %d (%s): map contains duplicate keys", label, slot, fieldName)
		}
		iterator := rawMap.MapRange()
		for iterator.Next() {
			if isRawMessagePackNil(iterator.Value().Interface().(codec.Raw)) {
				return fmt.Errorf("decode %s slot %d (%s): nil is not valid for map value", label, slot, fieldName)
			}
		}
	}
	return nil
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
