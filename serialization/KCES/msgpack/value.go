package msgpack

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ugorji/go/codec"
)

// 本文件实现 MessagePack 值的通用树模型，以及该值树的线格式编解码与 JSON 表示
// 解码复用本包的 DecodeMsgpack，ugorji 把载荷还原成 nil、bool、int64、uint64、float64、string、[]byte、[]any、map[any]any 与 codec.RawExt
// 值树在此基础上补回三样 ugorji 的 interface 解码会丢掉的信息：float32 宽度、映射键顺序、整数符号归一化
// float32 靠推断而不是记录：ugorji 把 0xca 精确加宽成 float64，因此「能由 float32 精确往返」等价于「线格式原来写的是 0xca」，写出时按同一判据还原宽度
// 值树不解释业务字段，因此 JSON 的根就是值树本身，没有额外封套：文档里不出现任何本库自造的元数据
// This file implements the generic tree model of MessagePack values along with its wire codec and JSON representation
// Decoding reuses this package's DecodeMsgpack, and ugorji restores a payload into nil, bool, int64, uint64, float64, string, []byte, []any, map[any]any, and codec.RawExt
// The value tree adds back the three facts ugorji's interface decoding drops: float32 width, map key order, and integer sign normalisation
// float32 width is inferred rather than recorded: ugorji widens 0xca exactly, so "float64 exactly round-trips through float32" is equivalent to "the wire form wrote 0xca", and encoding restores the width by the same test
// The tree does not interpret domain fields, so the JSON root is the value tree itself with no surrounding envelope and no metadata invented by this library

// Kind 表示 MessagePack 值的类型
// Kind identifies the type of a MessagePack value
type Kind uint8

const (
	// KindNil 表示 MessagePack nil
	// KindNil is MessagePack nil
	KindNil Kind = iota
	// KindBool 表示布尔值
	// KindBool is a boolean
	KindBool
	// KindInt 表示有符号整数，仅用于负数，非负数一律用 KindUint
	// KindInt is a signed integer used only for negative values, because non-negative values are always KindUint
	KindInt
	// KindUint 表示无符号整数
	// KindUint is an unsigned integer
	KindUint
	// KindFloat32 表示能由 float32 精确表示的数，写出为 0xca
	// KindFloat32 is a number float32 represents exactly and is written as 0xca
	KindFloat32
	// KindFloat64 表示 float32 无法精确表示的数，写出为 0xcb
	// KindFloat64 is a number float32 cannot represent exactly and is written as 0xcb
	KindFloat64
	// KindString 表示 UTF-8 字符串
	// KindString is a UTF-8 string
	KindString
	// KindArray 表示数组
	// KindArray is an array
	KindArray
	// KindMap 表示映射，键可以是整数或字符串
	// KindMap is a map whose keys may be integers or strings
	KindMap
	// KindBin 表示二进制数据
	// KindBin is binary data
	KindBin
	// KindExt 表示扩展类型
	// KindExt is an extension value
	KindExt
)

// String 返回类型的可读名称，仅用于错误信息
// String returns the readable kind name, used only in error messages
func (kind Kind) String() string {
	switch kind {
	case KindNil:
		return "nil"
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	case KindUint:
		return "uint"
	case KindFloat32:
		return "float32"
	case KindFloat64:
		return "float64"
	case KindString:
		return "string"
	case KindArray:
		return "array"
	case KindMap:
		return "map"
	case KindBin:
		return "bin"
	case KindExt:
		return "ext"
	default:
		return fmt.Sprintf("kind(%d)", uint8(kind))
	}
}

// MapEntry 是映射中的一个键值对，切片顺序即写出顺序
// ugorji 还原的映射是 Go map，本身没有顺序，因此解码时按键排序，保证同一份输入总得到同一份输出
// MapEntry is one key-value pair of a map, and the slice order is the written order
// The maps ugorji restores are Go maps without any order, so decoding sorts the keys to make the same input always produce the same output
type MapEntry struct {
	Key   Value // 键 / Key
	Value Value // 值 / Value
}

// Ext 是 MessagePack 扩展类型的类型号与载荷
// Ext is the type code and payload of a MessagePack extension
type Ext struct {
	Type int8   // 扩展类型号 / Extension type code
	Data []byte // 扩展载荷 / Extension payload
}

// Value 是 MessagePack 值树的一个节点，按 Kind 选用对应字段
// Value is one node of the MessagePack value tree and uses the field selected by Kind
type Value struct {
	Kind    Kind       // 节点类型 / Node kind
	Bool    bool       // KindBool 的有效字段 / Field valid for KindBool
	Int     int64      // KindInt 的有效字段 / Field valid for KindInt
	Uint    uint64     // KindUint 的有效字段 / Field valid for KindUint
	Float32 float32    // KindFloat32 的有效字段 / Field valid for KindFloat32
	Float64 float64    // KindFloat64 的有效字段 / Field valid for KindFloat64
	Str     string     // KindString 的有效字段 / Field valid for KindString
	Array   []Value    // KindArray 的有效字段 / Field valid for KindArray
	Map     []MapEntry // KindMap 的有效字段 / Field valid for KindMap
	Bin     []byte     // KindBin 的有效字段 / Field valid for KindBin
	Ext     Ext        // KindExt 的有效字段 / Field valid for KindExt
}

// DecodeValue 把一个完整的 MessagePack 根值解码为值树，并拒绝任何尾部字节
// 压缩分帧不属于值树的职责，调用方需要先自行解开
// DecodeValue decodes one complete MessagePack root value into a value tree and rejects any trailing bytes
// Compression framing is not the value tree's concern, so callers have to undo it first
func DecodeValue(data []byte) (Value, error) {
	var decoded any
	if err := DecodeMsgpack(data, &decoded); err != nil {
		return Value{}, fmt.Errorf("decode MessagePack value: %w", err)
	}
	value, err := valueFromAny(decoded, "$")
	if err != nil {
		return Value{}, fmt.Errorf("decode MessagePack value: %w", err)
	}
	return value, nil
}

// EncodeValue 把值树编码为 MessagePack 线格式，整数一律使用能表示该值的最短形式
// EncodeValue encodes a value tree into MessagePack wire form and always uses the shortest integer form that can represent each value
func EncodeValue(value Value) ([]byte, error) {
	encoded, err := appendMessagePackValue(make([]byte, 0, 256), value)
	if err != nil {
		return nil, fmt.Errorf("encode MessagePack value: %w", err)
	}
	return encoded, nil
}

// valueFromAny 把 ugorji 的 interface 解码结果转换成值树
// path 只用于错误信息，指出出问题的节点位置
// valueFromAny converts a ugorji interface decoding result into a value tree
// The path argument is used only in error messages and names the failing node
func valueFromAny(decoded any, path string) (Value, error) {
	switch typed := decoded.(type) {
	case nil:
		return Value{Kind: KindNil}, nil
	case bool:
		return Value{Kind: KindBool, Bool: typed}, nil
	case int64:
		// 非负的 fixint 也会解成 int64，统一归一化成无符号，写出时才是最短形式
		// A non-negative fixint also decodes to int64, so it is normalised to unsigned to keep the written form shortest
		if typed >= 0 {
			return Value{Kind: KindUint, Uint: uint64(typed)}, nil
		}
		return Value{Kind: KindInt, Int: typed}, nil
	case uint64:
		return Value{Kind: KindUint, Uint: typed}, nil
	case float64:
		return valueFromFloat64(typed), nil
	case string:
		// ugorji 的 ValidateUnicode 只覆盖部分解码路径，JSON 必须能承载字符串，因此这里再校验一次
		// ugorji's ValidateUnicode covers only some decoding paths, and JSON has to carry the string, so it is validated again here
		if !utf8.ValidString(typed) {
			return Value{}, fmt.Errorf("%s: MessagePack string is not valid UTF-8", path)
		}
		return Value{Kind: KindString, Str: typed}, nil
	case []byte:
		return Value{Kind: KindBin, Bin: typed}, nil
	case codec.RawExt:
		// ugorji 用 uint64 表示扩展类型号，MessagePack 的线格式只有一字节
		// 类型号 -1 是 MessagePack 时间戳，ugorji 会直接解成 time.Time 并落到下面的兜底分支
		// ugorji carries the extension type code in a uint64 while the MessagePack wire form has a single byte
		// Type code -1 is the MessagePack timestamp, which ugorji decodes into time.Time and which therefore falls into the default branch below
		if typed.Tag > math.MaxUint8 {
			return Value{}, fmt.Errorf("%s: extension type code %d is outside the single-byte range", path, typed.Tag)
		}
		return Value{Kind: KindExt, Ext: Ext{Type: int8(byte(typed.Tag)), Data: typed.Data}}, nil
	case []any:
		items := make([]Value, len(typed))
		for index, item := range typed {
			converted, err := valueFromAny(item, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return Value{}, err
			}
			items[index] = converted
		}
		return Value{Kind: KindArray, Array: items}, nil
	case map[any]any:
		return valueFromMap(typed, path)
	default:
		return Value{}, fmt.Errorf("%s: unsupported MessagePack value %T", path, decoded)
	}
}

// valueFromMap 把 ugorji 还原的映射转换成按序排列的键值对切片
// valueFromMap converts a map restored by ugorji into an ordered slice of key-value pairs
func valueFromMap(decoded map[any]any, path string) (Value, error) {
	entries := make([]MapEntry, 0, len(decoded))
	for rawKey, rawValue := range decoded {
		key, err := valueFromAny(rawKey, path)
		if err != nil {
			return Value{}, err
		}
		switch key.Kind {
		case KindInt, KindUint, KindString:
		default:
			return Value{}, fmt.Errorf("%s: unsupported map key of kind %s", path, key.Kind)
		}
		value, err := valueFromAny(rawValue, path)
		if err != nil {
			return Value{}, err
		}
		entries = append(entries, MapEntry{Key: key, Value: value})
	}
	sortMapEntries(entries)
	return Value{Kind: KindMap, Map: entries}, nil
}

// valueFromFloat64 按能否由 float32 精确往返决定浮点数的宽度
// 0xca 被 ugorji 精确加宽成 float64，因此该判据能把原本的单精度值还原成 float32，而真正需要双精度的值保持 float64
// valueFromFloat64 selects the float width by whether float32 round-trips the value exactly
// 0xca is widened exactly by ugorji, so this test restores originally single-precision values as float32 while values that genuinely need double precision stay float64
func valueFromFloat64(value float64) Value {
	narrow := float32(value)
	if float64(narrow) == value {
		return Value{Kind: KindFloat32, Float32: narrow}
	}
	return Value{Kind: KindFloat64, Float64: value}
}

// sortMapEntries 按数值键升序、字符串键字典序排列键值对，数值键排在字符串键之前
// sortMapEntries orders key-value pairs by ascending numeric keys and then lexicographic string keys, with numeric keys before string keys
func sortMapEntries(entries []MapEntry) {
	sort.SliceStable(entries, func(left, right int) bool {
		return compareMapKeys(entries[left].Key, entries[right].Key) < 0
	})
}

// compareMapKeys 为排序提供键的全序，返回负值表示 left 排在 right 之前
// compareMapKeys provides the total order used for sorting and returns a negative value when left sorts before right
func compareMapKeys(left, right Value) int {
	leftRank, rightRank := mapKeyRank(left), mapKeyRank(right)
	if leftRank != rightRank {
		return leftRank - rightRank
	}
	switch leftRank {
	case 0:
		return compareNumericKeys(left, right)
	case 1:
		return strings.Compare(left.Str, right.Str)
	default:
		return strings.Compare(keyDebugString(left), keyDebugString(right))
	}
}

// mapKeyRank 给出键的排序分组，数值键为 0、字符串键为 1、其余为 2
// mapKeyRank gives the sort group of a key: 0 for numeric keys, 1 for string keys, and 2 for everything else
func mapKeyRank(key Value) int {
	switch key.Kind {
	case KindInt, KindUint:
		return 0
	case KindString:
		return 1
	default:
		return 2
	}
}

// compareNumericKeys 按数值比较两个整数键，负数排在最前
// compareNumericKeys compares two integer keys by value with negative keys first
func compareNumericKeys(left, right Value) int {
	leftNegative, leftMagnitude := numericKeyMagnitude(left)
	rightNegative, rightMagnitude := numericKeyMagnitude(right)
	if leftNegative != rightNegative {
		if leftNegative {
			return -1
		}
		return 1
	}
	switch {
	case leftMagnitude < rightMagnitude:
		if leftNegative {
			return 1
		}
		return -1
	case leftMagnitude > rightMagnitude:
		if leftNegative {
			return -1
		}
		return 1
	default:
		return 0
	}
}

// numericKeyMagnitude 返回整数键是否为负数以及其绝对值，MinInt64 也不会溢出
// numericKeyMagnitude reports whether an integer key is negative along with its absolute value, without overflowing on MinInt64
func numericKeyMagnitude(key Value) (bool, uint64) {
	if key.Kind == KindUint {
		return false, key.Uint
	}
	if key.Int >= 0 {
		return false, uint64(key.Int)
	}
	return true, uint64(-(key.Int + 1)) + 1
}

// keyDebugString 为无法排序的键提供稳定字符串，只用于非数值非字符串键的比较
// keyDebugString gives a stable string for keys that are neither numeric nor string, used only to order them
func keyDebugString(key Value) string {
	return key.Kind.String() + ":" + strconv.Quote(key.Str)
}

// appendMessagePackValue 将一个值追加为 MessagePack 线格式，整数一律使用能表示该值的最短形式
// 非负整数始终写作无符号形式，与 MessagePack-CSharp 的 WriteInt32 与 WriteInt64 一致
// appendMessagePackValue appends one value in MessagePack wire form and always uses the shortest integer form that can represent it
// Non-negative integers are always written unsigned, matching MessagePack-CSharp WriteInt32 and WriteInt64
func appendMessagePackValue(dst []byte, value Value) ([]byte, error) {
	switch value.Kind {
	case KindNil:
		return append(dst, 0xc0), nil
	case KindBool:
		if value.Bool {
			return append(dst, 0xc3), nil
		}
		return append(dst, 0xc2), nil
	case KindInt:
		return appendSignedInt(dst, value.Int), nil
	case KindUint:
		return appendUnsignedInt(dst, value.Uint), nil
	case KindFloat32:
		dst = append(dst, 0xca)
		return binary.BigEndian.AppendUint32(dst, math.Float32bits(value.Float32)), nil
	case KindFloat64:
		// 能由 float32 精确表示的值仍按单精度写出，与解码侧的宽度推断对称
		// A value float32 represents exactly is still written as single precision, mirroring the width inference on the decoding side
		if narrow := float32(value.Float64); float64(narrow) == value.Float64 {
			dst = append(dst, 0xca)
			return binary.BigEndian.AppendUint32(dst, math.Float32bits(narrow)), nil
		}
		dst = append(dst, 0xcb)
		return binary.BigEndian.AppendUint64(dst, math.Float64bits(value.Float64)), nil
	case KindString:
		dst = appendStringHeader(dst, len(value.Str))
		return append(dst, value.Str...), nil
	case KindArray:
		if int64(len(value.Array)) > math.MaxInt32 {
			return nil, fmt.Errorf("MessagePack array length %d exceeds Int32", len(value.Array))
		}
		dst = WriteArrayHeader(dst, int64(len(value.Array)))
		for index, item := range value.Array {
			next, err := appendMessagePackValue(dst, item)
			if err != nil {
				return nil, fmt.Errorf("array[%d]: %w", index, err)
			}
			dst = next
		}
		return dst, nil
	case KindMap:
		if int64(len(value.Map)) > math.MaxInt32 {
			return nil, fmt.Errorf("MessagePack map length %d exceeds Int32", len(value.Map))
		}
		dst = appendMapHeader(dst, len(value.Map))
		for index, entry := range value.Map {
			next, err := appendMessagePackValue(dst, entry.Key)
			if err != nil {
				return nil, fmt.Errorf("map[%d] key: %w", index, err)
			}
			next, err = appendMessagePackValue(next, entry.Value)
			if err != nil {
				return nil, fmt.Errorf("map[%d] value: %w", index, err)
			}
			dst = next
		}
		return dst, nil
	case KindBin:
		dst = appendBinHeader(dst, len(value.Bin))
		return append(dst, value.Bin...), nil
	case KindExt:
		dst = appendExtHeader(dst, value.Ext.Type, len(value.Ext.Data))
		return append(dst, value.Ext.Data...), nil
	default:
		return nil, fmt.Errorf("unsupported MessagePack value kind %s", value.Kind)
	}
}

// appendUnsignedInt 以最短无符号形式追加非负整数
// appendUnsignedInt appends a non-negative integer in the shortest unsigned form
func appendUnsignedInt(dst []byte, value uint64) []byte {
	switch {
	case value <= 0x7f:
		return append(dst, byte(value))
	case value <= math.MaxUint8:
		return append(dst, 0xcc, byte(value))
	case value <= math.MaxUint16:
		dst = append(dst, 0xcd)
		return binary.BigEndian.AppendUint16(dst, uint16(value))
	case value <= math.MaxUint32:
		dst = append(dst, 0xce)
		return binary.BigEndian.AppendUint32(dst, uint32(value))
	default:
		dst = append(dst, 0xcf)
		return binary.BigEndian.AppendUint64(dst, value)
	}
}

// appendSignedInt 以最短有符号形式追加整数，非负值改用无符号形式
// appendSignedInt appends an integer in the shortest signed form and switches non-negative values to the unsigned form
func appendSignedInt(dst []byte, value int64) []byte {
	if value >= 0 {
		return appendUnsignedInt(dst, uint64(value))
	}
	switch {
	case value >= -32:
		return append(dst, byte(int8(value)))
	case value >= math.MinInt8:
		return append(dst, 0xd0, byte(int8(value)))
	case value >= math.MinInt16:
		dst = append(dst, 0xd1)
		return binary.BigEndian.AppendUint16(dst, uint16(int16(value)))
	case value >= math.MinInt32:
		dst = append(dst, 0xd2)
		return binary.BigEndian.AppendUint32(dst, uint32(int32(value)))
	default:
		dst = append(dst, 0xd3)
		return binary.BigEndian.AppendUint64(dst, uint64(value))
	}
}

// appendStringHeader 以最短形式追加字符串长度头
// appendStringHeader appends a string length header in the shortest form
func appendStringHeader(dst []byte, length int) []byte {
	switch {
	case length <= 0x1f:
		return append(dst, 0xa0|byte(length))
	case length <= math.MaxUint8:
		return append(dst, 0xd9, byte(length))
	case length <= math.MaxUint16:
		dst = append(dst, 0xda)
		return binary.BigEndian.AppendUint16(dst, uint16(length))
	default:
		dst = append(dst, 0xdb)
		return binary.BigEndian.AppendUint32(dst, uint32(length))
	}
}

// appendMapHeader 以最短形式追加映射长度头
// appendMapHeader appends a map length header in the shortest form
func appendMapHeader(dst []byte, length int) []byte {
	switch {
	case length <= 0x0f:
		return append(dst, 0x80|byte(length))
	case length <= math.MaxUint16:
		dst = append(dst, 0xde)
		return binary.BigEndian.AppendUint16(dst, uint16(length))
	default:
		dst = append(dst, 0xdf)
		return binary.BigEndian.AppendUint32(dst, uint32(length))
	}
}

// appendBinHeader 以最短形式追加二进制长度头
// appendBinHeader appends a binary length header in the shortest form
func appendBinHeader(dst []byte, length int) []byte {
	switch {
	case length <= math.MaxUint8:
		return append(dst, 0xc4, byte(length))
	case length <= math.MaxUint16:
		dst = append(dst, 0xc5)
		return binary.BigEndian.AppendUint16(dst, uint16(length))
	default:
		dst = append(dst, 0xc6)
		return binary.BigEndian.AppendUint32(dst, uint32(length))
	}
}

// appendExtHeader 追加扩展类型头，载荷长度恰为 1、2、4、8 或 16 时使用定长形式
// appendExtHeader appends an extension header and uses the fixed-length form when the payload is exactly 1, 2, 4, 8, or 16 bytes
func appendExtHeader(dst []byte, typeCode int8, length int) []byte {
	switch length {
	case 1:
		return append(dst, 0xd4, byte(typeCode))
	case 2:
		return append(dst, 0xd5, byte(typeCode))
	case 4:
		return append(dst, 0xd6, byte(typeCode))
	case 8:
		return append(dst, 0xd7, byte(typeCode))
	case 16:
		return append(dst, 0xd8, byte(typeCode))
	}
	switch {
	case length <= math.MaxUint8:
		return append(dst, 0xc7, byte(length), byte(typeCode))
	case length <= math.MaxUint16:
		dst = append(dst, 0xc8)
		dst = binary.BigEndian.AppendUint16(dst, uint16(length))
		return append(dst, byte(typeCode))
	default:
		dst = append(dst, 0xc9)
		dst = binary.BigEndian.AppendUint32(dst, uint32(length))
		return append(dst, byte(typeCode))
	}
}

// 值树的 JSON 表示采用朴素形式，只在 MessagePack 独有且 JSON 无法自然表达的类型上引入标记对象：
// nil 写作 null，布尔写作 true 或 false，整数写作数字，浮点数写作数字，字符串写作字符串，数组写作数组
// 映射写作对象，键按值树顺序排列，能解析为规范十进制整数的键在回写时还原成整数键
// 二进制写作 {"$bin":"<base64>"}，扩展类型写作 {"$ext":{"type":<类型号>,"data":"<base64>"}}
// 浮点数的宽度不写在 JSON 里，而是靠十进制写法本身携带：float32 用最短单精度写法，float64 用最短双精度写法，两者冲突时改用 17 位有效数字
// The JSON form of the value tree is plain and only introduces tagged objects for MessagePack types JSON cannot express naturally:
// nil becomes null, bool becomes true or false, integers become numbers, floats become numbers, strings become strings, and arrays become arrays
// A map becomes an object whose keys follow the value tree order, and a key that parses as a canonical decimal integer is restored as an integer key when writing back
// Binary becomes {"$bin":"<base64>"} and an extension becomes {"$ext":{"type":<code>,"data":"<base64>"}}
// Float width is not written into the JSON but carried by the decimal spelling itself: float32 uses the shortest single-precision spelling, float64 uses the shortest double-precision spelling, and 17 significant digits are used when the two collide

const (
	// binJSONKey 是二进制值的标记键
	// binJSONKey is the tag key of a binary value
	binJSONKey = "$bin"
	// extJSONKey 是扩展类型的标记键
	// extJSONKey is the tag key of an extension value
	extJSONKey = "$ext"
	// extTypeJSONKey 是扩展类型对象的类型号字段
	// extTypeJSONKey is the type-code field of an extension object
	extTypeJSONKey = "type"
	// extDataJSONKey 是扩展类型对象的载荷字段
	// extDataJSONKey is the payload field of an extension object
	extDataJSONKey = "data"
)

// DecodeValueJSON 解析值树的 JSON 表示为值树
// DecodeValueJSON parses the JSON representation of a value tree into a value tree
func DecodeValueJSON(data []byte) (Value, error) {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	if len(trimmed) == 0 {
		return Value{}, fmt.Errorf("MessagePack value JSON is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	value, err := decodeJSONValue(decoder, "$")
	if err != nil {
		return Value{}, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return Value{}, fmt.Errorf("MessagePack value JSON has more than one root value")
		}
		return Value{}, fmt.Errorf("MessagePack value JSON trailing content: %w", err)
	}
	return value, nil
}

// EncodeValueJSON 将值树写为两空格缩进并以换行结尾的 UTF-8 JSON
// EncodeValueJSON writes the value tree as UTF-8 JSON indented with two spaces and terminated by a newline
func EncodeValueJSON(value Value) ([]byte, error) {
	indented, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal MessagePack value JSON: %w", err)
	}
	return append(indented, '\n'), nil
}

// decodeJSONValue 从 JSON 词法流读取一个值，对象按书写顺序保留键
// decodeJSONValue reads one value from the JSON token stream and keeps object keys in their written order
func decodeJSONValue(decoder *json.Decoder, path string) (Value, error) {
	token, err := decoder.Token()
	if err != nil {
		return Value{}, fmt.Errorf("%s: %w", path, err)
	}
	switch typed := token.(type) {
	case nil:
		return Value{Kind: KindNil}, nil
	case bool:
		return Value{Kind: KindBool, Bool: typed}, nil
	case string:
		return Value{Kind: KindString, Str: typed}, nil
	case json.Number:
		return decodeJSONNumber(typed, path)
	case json.Delim:
		switch typed {
		case '[':
			return decodeJSONArray(decoder, path)
		case '{':
			return decodeJSONObject(decoder, path)
		}
		return Value{}, fmt.Errorf("%s: unexpected JSON delimiter %q", path, typed)
	default:
		return Value{}, fmt.Errorf("%s: unsupported JSON token %v", path, token)
	}
}

// decodeJSONNumber 按书写形式把 JSON 数字还原成整数或浮点值
// 含小数点或指数的是浮点数，否则是整数；浮点数在写法等于该值的最短单精度写法时按 float32 处理，与写出侧对称
// decodeJSONNumber restores a JSON number into an integer or floating value according to its spelling
// A number with a decimal point or an exponent is a float and anything else is an integer, and a float becomes float32 when its spelling equals the shortest single-precision spelling of the value, mirroring the writing side
func decodeJSONNumber(number json.Number, path string) (Value, error) {
	text := number.String()
	if !strings.ContainsAny(text, ".eE") {
		if value, err := strconv.ParseInt(text, 10, 64); err == nil {
			if value >= 0 {
				return Value{Kind: KindUint, Uint: uint64(value)}, nil
			}
			return Value{Kind: KindInt, Int: value}, nil
		}
		if value, err := strconv.ParseUint(text, 10, 64); err == nil {
			return Value{Kind: KindUint, Uint: value}, nil
		}
		return Value{}, fmt.Errorf("%s: integer %s is outside the Int64 and Uint64 ranges", path, text)
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return Value{}, fmt.Errorf("%s: parse float %s: %w", path, text, err)
	}
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return Value{}, fmt.Errorf("%s: float %s is not finite", path, text)
	}
	if formatFloat32(float32(value)) == text {
		return Value{Kind: KindFloat32, Float32: float32(value)}, nil
	}
	return Value{Kind: KindFloat64, Float64: value}, nil
}

// decodeJSONArray 读取 JSON 数组
// decodeJSONArray reads a JSON array
func decodeJSONArray(decoder *json.Decoder, path string) (Value, error) {
	items := make([]Value, 0, 8)
	for index := 0; decoder.More(); index++ {
		item, err := decodeJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return Value{}, err
		}
		items = append(items, item)
	}
	if _, err := decoder.Token(); err != nil {
		return Value{}, fmt.Errorf("%s: %w", path, err)
	}
	return Value{Kind: KindArray, Array: items}, nil
}

// decodeJSONObject 读取 JSON 对象，识别二进制与扩展类型的标记对象，其余按映射处理
// decodeJSONObject reads a JSON object, recognises the binary and extension tag objects, and treats everything else as a map
func decodeJSONObject(decoder *json.Decoder, path string) (Value, error) {
	entries := make([]MapEntry, 0, 8)
	seen := make(map[string]struct{}, 8)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return Value{}, fmt.Errorf("%s: %w", path, err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return Value{}, fmt.Errorf("%s: JSON object key is not a string", path)
		}
		if _, duplicate := seen[key]; duplicate {
			return Value{}, fmt.Errorf("%s: duplicate JSON object key %q", path, key)
		}
		seen[key] = struct{}{}
		value, err := decodeJSONValue(decoder, path+"."+key)
		if err != nil {
			return Value{}, err
		}
		entries = append(entries, MapEntry{Key: mapKeyValue(key), Value: value})
	}
	if _, err := decoder.Token(); err != nil {
		return Value{}, fmt.Errorf("%s: %w", path, err)
	}
	if tagged, ok, err := decodeTaggedObject(entries, path); ok || err != nil {
		return tagged, err
	}
	return Value{Kind: KindMap, Map: entries}, nil
}

// decodeTaggedObject 识别只有一个标记键的二进制或扩展类型对象
// decodeTaggedObject recognises a binary or extension object carrying exactly one tag key
func decodeTaggedObject(entries []MapEntry, path string) (Value, bool, error) {
	if len(entries) != 1 || entries[0].Key.Kind != KindString {
		return Value{}, false, nil
	}
	switch entries[0].Key.Str {
	case binJSONKey:
		if entries[0].Value.Kind != KindString {
			return Value{}, true, fmt.Errorf("%s: %s must be a base64 string", path, binJSONKey)
		}
		payload, err := base64.StdEncoding.DecodeString(entries[0].Value.Str)
		if err != nil {
			return Value{}, true, fmt.Errorf("%s: decode %s base64: %w", path, binJSONKey, err)
		}
		return Value{Kind: KindBin, Bin: payload}, true, nil
	case extJSONKey:
		return decodeExtObject(entries[0].Value, path)
	}
	return Value{}, false, nil
}

// decodeExtObject 识别 {"type":<类型号>,"data":"<base64>"} 形式的扩展类型对象
// decodeExtObject recognises an extension object of the form {"type":<code>,"data":"<base64>"}
func decodeExtObject(value Value, path string) (Value, bool, error) {
	if value.Kind != KindMap || len(value.Map) != 2 {
		return Value{}, true, fmt.Errorf("%s: %s must be an object with %q and %q", path, extJSONKey, extTypeJSONKey, extDataJSONKey)
	}
	var typeCode int64
	var payload []byte
	seenType, seenData := false, false
	for _, entry := range value.Map {
		if entry.Key.Kind != KindString {
			return Value{}, true, fmt.Errorf("%s: %s keys must be strings", path, extJSONKey)
		}
		switch entry.Key.Str {
		case extTypeJSONKey:
			switch entry.Value.Kind {
			case KindUint:
				if entry.Value.Uint > math.MaxInt8 {
					return Value{}, true, fmt.Errorf("%s: %s.%s %d is outside the Int8 range", path, extJSONKey, extTypeJSONKey, entry.Value.Uint)
				}
				typeCode = int64(entry.Value.Uint)
			case KindInt:
				if entry.Value.Int < math.MinInt8 {
					return Value{}, true, fmt.Errorf("%s: %s.%s %d is outside the Int8 range", path, extJSONKey, extTypeJSONKey, entry.Value.Int)
				}
				typeCode = entry.Value.Int
			default:
				return Value{}, true, fmt.Errorf("%s: %s.%s must be an integer", path, extJSONKey, extTypeJSONKey)
			}
			seenType = true
		case extDataJSONKey:
			if entry.Value.Kind != KindString {
				return Value{}, true, fmt.Errorf("%s: %s.%s must be a base64 string", path, extJSONKey, extDataJSONKey)
			}
			decoded, err := base64.StdEncoding.DecodeString(entry.Value.Str)
			if err != nil {
				return Value{}, true, fmt.Errorf("%s: decode %s.%s base64: %w", path, extJSONKey, extDataJSONKey, err)
			}
			payload = decoded
			seenData = true
		default:
			return Value{}, true, fmt.Errorf("%s: unknown %s field %q", path, extJSONKey, entry.Key.Str)
		}
	}
	if !seenType || !seenData {
		return Value{}, true, fmt.Errorf("%s: %s requires both %q and %q", path, extJSONKey, extTypeJSONKey, extDataJSONKey)
	}
	return Value{Kind: KindExt, Ext: Ext{Type: int8(typeCode), Data: payload}}, true, nil
}

// mapKeyValue 将 JSON 对象键还原成映射键，规范十进制整数还原为整数键
// 该规则使整数键的映射在 JSON 里保持可读，代价是形如 "123" 的字符串键会被写成整数键
// mapKeyValue restores a JSON object key into a map key, turning a canonical decimal integer back into an integer key
// The rule keeps integer-keyed maps readable in JSON at the cost of writing a string key spelled like "123" as an integer key
func mapKeyValue(key string) Value {
	if key == "" {
		return Value{Kind: KindString, Str: key}
	}
	if value, err := strconv.ParseInt(key, 10, 64); err == nil && strconv.FormatInt(value, 10) == key {
		if value >= 0 {
			return Value{Kind: KindUint, Uint: uint64(value)}
		}
		return Value{Kind: KindInt, Int: value}
	}
	if value, err := strconv.ParseUint(key, 10, 64); err == nil && strconv.FormatUint(value, 10) == key {
		return Value{Kind: KindUint, Uint: value}
	}
	return Value{Kind: KindString, Str: key}
}

// MarshalJSON 输出紧凑 JSON，映射键按值树中的顺序排列
// 缩进由 encoding/json 的 MarshalIndent 在外层完成，因此这里只保证键顺序与数字格式
// MarshalJSON emits compact JSON whose map keys follow the order held by the value tree
// Indentation is applied by encoding/json MarshalIndent on the outside, so this method only guarantees key order and number formatting
func (value Value) MarshalJSON() ([]byte, error) {
	return value.appendJSON(make([]byte, 0, 64))
}

// appendJSON 将值追加为紧凑 JSON
// appendJSON appends the value as compact JSON
func (value Value) appendJSON(dst []byte) ([]byte, error) {
	switch value.Kind {
	case KindNil:
		return append(dst, "null"...), nil
	case KindBool:
		if value.Bool {
			return append(dst, "true"...), nil
		}
		return append(dst, "false"...), nil
	case KindInt:
		return strconv.AppendInt(dst, value.Int, 10), nil
	case KindUint:
		return strconv.AppendUint(dst, value.Uint, 10), nil
	case KindFloat32:
		return append(dst, formatFloat32(value.Float32)...), nil
	case KindFloat64:
		return append(dst, formatFloat64(value.Float64)...), nil
	case KindString:
		encoded, err := json.Marshal(value.Str)
		if err != nil {
			return nil, fmt.Errorf("marshal JSON string: %w", err)
		}
		return append(dst, encoded...), nil
	case KindArray:
		dst = append(dst, '[')
		for index, item := range value.Array {
			if index > 0 {
				dst = append(dst, ',')
			}
			next, err := item.appendJSON(dst)
			if err != nil {
				return nil, fmt.Errorf("array[%d]: %w", index, err)
			}
			dst = next
		}
		return append(dst, ']'), nil
	case KindMap:
		dst = append(dst, '{')
		for index, entry := range value.Map {
			if index > 0 {
				dst = append(dst, ',')
			}
			key, err := entry.Key.mapKeyJSON()
			if err != nil {
				return nil, fmt.Errorf("map[%d]: %w", index, err)
			}
			encodedKey, err := json.Marshal(key)
			if err != nil {
				return nil, fmt.Errorf("map[%d] key: %w", index, err)
			}
			dst = append(dst, encodedKey...)
			dst = append(dst, ':')
			next, err := entry.Value.appendJSON(dst)
			if err != nil {
				return nil, fmt.Errorf("map[%d] value: %w", index, err)
			}
			dst = next
		}
		return append(dst, '}'), nil
	case KindBin:
		encodedKey, err := json.Marshal(binJSONKey)
		if err != nil {
			return nil, fmt.Errorf("marshal JSON string: %w", err)
		}
		encodedValue, err := json.Marshal(base64.StdEncoding.EncodeToString(value.Bin))
		if err != nil {
			return nil, fmt.Errorf("marshal JSON string: %w", err)
		}
		dst = append(dst, '{')
		dst = append(dst, encodedKey...)
		dst = append(dst, ':')
		dst = append(dst, encodedValue...)
		return append(dst, '}'), nil
	case KindExt:
		return value.appendExtJSON(dst)
	default:
		return nil, fmt.Errorf("unsupported MessagePack value kind %s", value.Kind)
	}
}

// appendExtJSON 将扩展类型追加为 {"$ext":{"type":<类型号>,"data":"<base64>"}}
// appendExtJSON appends an extension as {"$ext":{"type":<code>,"data":"<base64>"}}
func (value Value) appendExtJSON(dst []byte) ([]byte, error) {
	encodedKey, err := json.Marshal(extJSONKey)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON string: %w", err)
	}
	typeKey, err := json.Marshal(extTypeJSONKey)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON string: %w", err)
	}
	dataKey, err := json.Marshal(extDataJSONKey)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON string: %w", err)
	}
	encodedData, err := json.Marshal(base64.StdEncoding.EncodeToString(value.Ext.Data))
	if err != nil {
		return nil, fmt.Errorf("marshal JSON string: %w", err)
	}
	dst = append(dst, '{')
	dst = append(dst, encodedKey...)
	dst = append(dst, ':', '{')
	dst = append(dst, typeKey...)
	dst = append(dst, ':')
	dst = strconv.AppendInt(dst, int64(value.Ext.Type), 10)
	dst = append(dst, ',')
	dst = append(dst, dataKey...)
	dst = append(dst, ':')
	dst = append(dst, encodedData...)
	return append(dst, '}', '}'), nil
}

// mapKeyJSON 返回映射键在 JSON 对象中使用的字符串
// mapKeyJSON returns the string a map key uses inside a JSON object
func (value Value) mapKeyJSON() (string, error) {
	switch value.Kind {
	case KindString:
		return value.Str, nil
	case KindUint:
		return strconv.FormatUint(value.Uint, 10), nil
	case KindInt:
		return strconv.FormatInt(value.Int, 10), nil
	default:
		return "", fmt.Errorf("map key of kind %s cannot be represented in JSON", value.Kind)
	}
}

// formatFloat32 返回 float32 的最短十进制写法，并保证结果带小数点或指数，避免被读成整数
// formatFloat32 returns the shortest decimal spelling of a float32 and keeps a decimal point or an exponent so it is never read back as an integer
func formatFloat32(value float32) string {
	return ensureFloatSyntax(strconv.FormatFloat(float64(value), 'g', -1, 32))
}

// formatFloat64 返回 float64 的十进制写法
// 最短写法若会被读回 float32 且该值无法由 float32 精确表示，则改用 17 位有效数字，使 JSON 往返不改变数值
// 能由 float32 精确表示的值无法通过十进制写法与单精度区分，这类值按 float32 处理，数值本身不变
// formatFloat64 returns the decimal spelling of a float64
// When the shortest spelling would be read back as float32 while float32 cannot represent the value exactly, 17 significant digits are used so the JSON round trip keeps the number unchanged
// A value float32 represents exactly cannot be told apart from single precision through a decimal spelling, so such values are handled as float32 and their number stays unchanged
func formatFloat64(value float64) string {
	text := strconv.FormatFloat(value, 'g', -1, 64)
	if readsBackAsFloat32(text) {
		if narrow := float32(value); float64(narrow) != value {
			text = strconv.FormatFloat(value, 'g', 17, 64)
		}
	}
	return ensureFloatSyntax(text)
}

// readsBackAsFloat32 判断一段十进制写法在解码时会被判定成 float32
// readsBackAsFloat32 reports whether a decimal spelling is decoded as float32
func readsBackAsFloat32(text string) bool {
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return false
	}
	return formatFloat32(float32(parsed)) == ensureFloatSyntax(text)
}

// ensureFloatSyntax 为不含小数点与指数的写法补上 ".0"，使其仍被解析成浮点数
// ensureFloatSyntax appends ".0" to a spelling without a decimal point or an exponent so it is still parsed as a float
func ensureFloatSyntax(text string) string {
	if strings.ContainsAny(text, ".eE") {
		return text
	}
	return text + ".0"
}
