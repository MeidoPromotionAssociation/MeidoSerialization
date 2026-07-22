package KCES

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// KCES 各格式共用的 MessagePack 根值、LZ4 Block Array 和基础值转换辅助函数
// MessagePack-root, LZ4 Block Array, and primitive conversion helpers shared by KCES formats

// messagePackTrailingCarrier 由根格式化结果后可能存在 MessagePack-CSharp 未读字节的顶层游戏记录实现，这些字节位于 indexed object 之外，不能成为另一个结构体槽位
// messagePackTrailingCarrier is implemented by top-level game records whose formatter result may be followed by bytes left unread by MessagePack-CSharp, which are outside the indexed object and must never become another struct slot
type messagePackTrailingCarrier interface {
	getMessagePackTrailing() []byte
	setMessagePackTrailing([]byte)
}

// messagePackRootCarrier 由需要保留 MessagePack nil 根值状态的顶层记录实现
// messagePackRootCarrier is implemented by top-level records that preserve MessagePack nil-root state
type messagePackRootCarrier interface {
	getMessagePackRootNil() bool
	setMessagePackRootNil(bool)
}

// MessagePackRootMetadata 与 ct 共用，使所有 MessagePack 模型采用相同的 JSON 和线格式注释结构
// MessagePackRootMetadata is shared with ct so all MessagePack-facing models use one JSON and wire annotation shape
type MessagePackRootMetadata = ct.MessagePackRootMetadata

// IndexedObjectMetadata 记录已解码 MessagePack-CSharp int-key 数组的准确宽度，以及比当前类型模型更新的原始槽位
// IndexedObjectMetadata records a decoded MessagePack-CSharp int-key array's exact width and raw slots newer than this library's typed model
type IndexedObjectMetadata = ct.TypedIndexedObjectMetadata

// messagePackRootTrailingAfterParsed 核对解析器消耗长度并返回首个根值后的尾部字节
// messagePackRootTrailingAfterParsed verifies the parser's consumed length and returns bytes after the first root value
func messagePackRootTrailingAfterParsed(data []byte, parsed int, name string) ([]byte, error) {
	if parsed < 0 || parsed > len(data) {
		return nil, fmt.Errorf("decode %s parser consumed invalid byte count %d of %d", name, parsed, len(data))
	}
	root, trailing, err := ct.SplitFirstMsgpackValue(data)
	if err != nil {
		return nil, fmt.Errorf("decode %s root MessagePack value: %w", name, err)
	}
	if len(root) != parsed {
		return nil, fmt.Errorf("decode %s parser consumed %d bytes but MessagePack library reports a %d-byte root value", name, parsed, len(root))
	}
	return trailing, nil
}

// encodeNilMessagePackRootIfRequested 在请求 nil 根值且没有有效载荷时写出 nil 与保留尾部
// encodeNilMessagePackRootIfRequested writes a nil root and preserved trailing bytes when requested and no payload is populated
func encodeNilMessagePackRootIfRequested(metadata MessagePackRootMetadata, hasPayload bool, name string) ([]byte, bool, error) {
	if !metadata.RootNil {
		return nil, false, nil
	}
	if hasPayload {
		return nil, true, fmt.Errorf("%s rootNil would discard populated object fields", name)
	}
	out := make([]byte, 1, 1+len(metadata.TrailingData))
	out[0] = 0xc0
	out = append(out, metadata.TrailingData...)
	return out, true, nil
}

// appendMessagePackRootTrailing 将保留的根值尾部字节追加到编码结果
// appendMessagePackRootTrailing appends preserved root trailing bytes to encoded data
func appendMessagePackRootTrailing(root []byte, metadata MessagePackRootMetadata) []byte {
	return append(root, metadata.TrailingData...)
}

// decodeCompressedMsgpack 解压 LZ4 Block Array，解码首个 MessagePack 根值并保留根值状态与尾部
// decodeCompressedMsgpack decompresses an LZ4 Block Array, decodes the first MessagePack root, and preserves root state and trailing bytes
func decodeCompressedMsgpack(data []byte, out interface{}, name string) error {
	decompressed, err := ct.DecompressLz4BlockArray(data)
	if err != nil {
		return fmt.Errorf("decompress %s: %w", name, err)
	}

	root, trailing, err := ct.SplitFirstMsgpackValue(decompressed)
	if err != nil {
		return fmt.Errorf("split %s root msgpack: %w", name, err)
	}
	if carrier, ok := out.(messagePackRootCarrier); ok {
		carrier.setMessagePackRootNil(false)
		if len(root) == 1 && root[0] == 0xc0 {
			resetMessagePackRootValue(out)
			carrier.setMessagePackRootNil(true)
			if trailingCarrier, ok := out.(messagePackTrailingCarrier); ok {
				trailingCarrier.setMessagePackTrailing(append([]byte(nil), trailing...))
			}
			return nil
		}
	}
	if err := ct.DecodeMsgpack(root, out); err != nil {
		return fmt.Errorf("decode %s msgpack: %w", name, err)
	}
	if carrier, ok := out.(messagePackTrailingCarrier); ok {
		carrier.setMessagePackTrailing(append([]byte(nil), trailing...))
	}
	return nil
}

// cloneSlicePreserveNil 复制切片并保留 nil 与空切片的区别
// cloneSlicePreserveNil clones a slice while preserving the distinction between nil and empty
func cloneSlicePreserveNil[T any](src []T) []T {
	if src == nil {
		return nil
	}
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}

// encodeCompressedMsgpack 编码 indexed MessagePack 根值及保留尾部并使用 LZ4 Block Array 压缩
// encodeCompressedMsgpack encodes an indexed MessagePack root with preserved trailing bytes and compresses it as an LZ4 Block Array
func encodeCompressedMsgpack(v interface{}, name string) ([]byte, error) {
	var encoded []byte
	if carrier, ok := v.(messagePackRootCarrier); ok && carrier.getMessagePackRootNil() {
		if messagePackRootHasPayload(v) {
			return nil, fmt.Errorf("encode %s: rootNil would discard populated object fields", name)
		}
		encoded = []byte{0xc0}
	} else {
		var err error
		selfer, ok := indexedMessagePackSelfer(v)
		if !ok {
			return nil, fmt.Errorf("encode %s: %T does not implement the indexed MessagePack codec", name, v)
		}
		encoded, err = ct.EncodeIndexedMsgpack(selfer)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", name, err)
		}
	}
	if carrier, ok := v.(messagePackTrailingCarrier); ok {
		encoded = append(encoded, carrier.getMessagePackTrailing()...)
	}

	compressed, err := ct.CompressLz4BlockArray(encoded)
	if err != nil {
		return nil, fmt.Errorf("compress %s: %w", name, err)
	}
	return compressed, nil
}

// indexedMessagePackSelfer 获取值实现的 codec.Selfer，必要时为可寻址结构体副本创建指针
// indexedMessagePackSelfer obtains the codec.Selfer implemented by a value, creating a pointer to an addressable struct copy when necessary
func indexedMessagePackSelfer(value interface{}) (codec.Selfer, bool) {
	if selfer, ok := value.(codec.Selfer); ok {
		return selfer, true
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return nil, false
	}
	copyPointer := reflect.New(rv.Type())
	copyPointer.Elem().Set(rv)
	selfer, ok := copyPointer.Interface().(codec.Selfer)
	return selfer, ok
}

// resetMessagePackRootValue 将可设置的非 nil 指针目标清零
// resetMessagePackRootValue zeroes the target of a settable non-nil pointer
func resetMessagePackRootValue(value interface{}) {
	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Ptr && !rv.IsNil() && rv.Elem().CanSet() {
		rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
	}
}

// messagePackRootHasPayload 判断根对象是否含有不能被 nil 根值表示的有效字段
// messagePackRootHasPayload reports whether a root object contains populated fields that a nil root cannot represent
func messagePackRootHasPayload(value interface{}) bool {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Ptr || rv.IsNil() {
		return false
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return !rv.IsZero()
	}
	indexedType := reflect.TypeOf(IndexedObjectMetadata{})
	indexedPtrType := reflect.PointerTo(indexedType)
	for index := 0; index < rv.NumField(); index++ {
		fieldType := rv.Type().Field(index)
		field := rv.Field(index)
		if fieldType.Name == "_struct" {
			continue
		}
		if fieldType.Type == indexedType {
			metadata := field.Interface().(IndexedObjectMetadata)
			if indexedObjectMetadataHasPayload(metadata) {
				return true
			}
			continue
		}
		if fieldType.Type == indexedPtrType {
			if !field.IsNil() {
				metadata := field.Interface().(*IndexedObjectMetadata)
				if indexedObjectMetadataHasPayload(*metadata) {
					return true
				}
			}
			continue
		}
		if strings.Split(fieldType.Tag.Get("codec"), ",")[0] == "-" || fieldType.PkgPath != "" {
			continue
		}
		if !field.IsZero() {
			return true
		}
	}
	return false
}

// indexedObjectMetadataHasPayload 判断 indexed-object 元数据是否记录了任何非默认线格式状态
// indexedObjectMetadataHasPayload reports whether indexed-object metadata records any nondefault wire state
func indexedObjectMetadataHasPayload(metadata IndexedObjectMetadata) bool {
	return metadata.FieldCount != nil || len(metadata.FutureSlots) != 0 || len(metadata.NilSlots) != 0 || len(metadata.NullElements) != 0 || len(metadata.NullMapValueKeys) != 0
}

// decodeRawMsgpackArray 将通用数组重新编码后解码进强类型 indexed object
// decodeRawMsgpackArray re-encodes a generic array and decodes it into a typed indexed object
func decodeRawMsgpackArray(arr []interface{}, out interface{}, name string) error {
	encoded, err := ct.EncodeMsgpack(arr)
	if err != nil {
		return fmt.Errorf("encode raw %s array: %w", name, err)
	}
	if err := ct.DecodeMsgpack(encoded, out); err != nil {
		return fmt.Errorf("decode raw %s array: %w", name, err)
	}
	return nil
}

// toIntVal 将支持的 MessagePack 或 JSON 数值转换为平台 int 并检查范围
// toIntVal converts a supported MessagePack or JSON number to platform int with range checking
func toIntVal(v interface{}) (int, bool) {
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	switch n := v.(type) {
	case json.Number:
		i, err := strconv.ParseInt(n.String(), 10, 0)
		return int(i), err == nil
	case int64:
		if n < int64(minInt) || n > int64(maxInt) {
			return 0, false
		}
		return int(n), true
	case uint64:
		if n > uint64(maxInt) {
			return 0, false
		}
		return int(n), true
	case int:
		return n, true
	case uint:
		if uint64(n) > uint64(maxInt) {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}

// toUint64Val 将支持的非负 MessagePack 或 JSON 数值转换为 UInt64
// toUint64Val converts a supported nonnegative MessagePack or JSON number to UInt64
func toUint64Val(v interface{}) (uint64, bool) {
	switch n := v.(type) {
	case json.Number:
		u, err := strconv.ParseUint(n.String(), 10, 64)
		return u, err == nil
	case uint64:
		return n, true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case uint:
		return uint64(n), true
	}
	return 0, false
}

// toFloat32 将支持的 MessagePack 或 JSON 数值转换为 Float32
// toFloat32 converts a supported MessagePack or JSON number to Float32
func toFloat32(v interface{}) (float32, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(n.String(), 32)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return float32(f), true
	case float64:
		// MessagePackReader.ReadSingle 接受浮点值并执行 CLR 转换，NaN、无穷和溢出为 Single 无穷的有限 double 仍是可表示的运行时结果
		// MessagePackReader.ReadSingle accepts floating values and performs the CLR conversion, with NaN, infinities, and finite double values overflowing to Single infinity remaining representable runtime results
		return float32(n), true
	case float32:
		return n, true
	case int64:
		return float32(n), true
	case uint64:
		return float32(n), true
	}
	return 0, false
}

// toFloat64 将支持的 MessagePack 或 JSON 数值转换为 Float64
// toFloat64 converts a supported MessagePack or JSON number to Float64
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(n.String(), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

// toInt64Val 将支持的 MessagePack 或 JSON 整数转换为 Int64 并检查范围
// toInt64Val converts a supported MessagePack or JSON integer to Int64 with range checking
func toInt64Val(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := strconv.ParseInt(n.String(), 10, 64)
		return i, err == nil
	case int64:
		return n, true
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	case uint:
		if uint64(n) > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	}
	return 0, false
}

// toBool 将值转换为 bool，并在类型不匹配时返回 false
// toBool converts a value to bool and reports false when the type does not match
func toBool(v interface{}) (bool, bool) {
	if b, ok := v.(bool); ok {
		return b, true
	}
	return false, false
}

// toStringVal 将值转换为字符串，并在类型不匹配时返回 false
// toStringVal converts a value to string and reports false when the type does not match
func toStringVal(v interface{}) (string, bool) {
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

// padSlice 将通用数组以 nil 补齐到至少指定长度
// padSlice pads a generic array with nil values to at least the requested length
func padSlice(arr []interface{}, size int) []interface{} {
	if len(arr) >= size {
		return arr
	}
	padded := make([]interface{}, size)
	copy(padded, arr)
	return padded
}

// float32ToUint32Bits 用于 MessagePack 中 float32 的精确编码
// float32ToUint32Bits returns the exact Float32 bits used for MessagePack encoding
func float32ToUint32Bits(f float32) uint32 {
	return math.Float32bits(f)
}

// jsonNumberForFloat 将浮点数格式化为保留浮点语义的 JSON 数字
// jsonNumberForFloat formats a floating-point value as a JSON number that retains floating-point semantics
func jsonNumberForFloat(v float64, bitSize int) json.Number {
	s := strconv.FormatFloat(v, 'g', -1, bitSize)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return json.Number(s)
}
