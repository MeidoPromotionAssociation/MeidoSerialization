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

// decodeCompressedMsgpack 解压 LZ4 Block Array 并严格解码唯一的完整 MessagePack 根值
// decodeCompressedMsgpack decompresses an LZ4 Block Array and strictly decodes its sole complete MessagePack root value
func decodeCompressedMsgpack(data []byte, out interface{}, name string) error {
	decompressed, err := ct.DecompressLz4BlockArray(data)
	if err != nil {
		return fmt.Errorf("decompress %s: %w", name, err)
	}
	if err := ct.DecodeMsgpack(decompressed, out); err != nil {
		return fmt.Errorf("decode %s msgpack: %w", name, err)
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

// encodeCompressedMsgpack 编码固定布局的 indexed MessagePack 根值并使用 LZ4 Block Array 压缩
// encodeCompressedMsgpack encodes a fixed-layout indexed MessagePack root and compresses it as an LZ4 Block Array
func encodeCompressedMsgpack(v interface{}, name string) ([]byte, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || ((rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface) && rv.IsNil()) {
		encoded, err := ct.EncodeMsgpack(nil)
		if err != nil {
			return nil, fmt.Errorf("encode %s null root: %w", name, err)
		}
		compressed, err := ct.CompressLz4BlockArray(encoded)
		if err != nil {
			return nil, fmt.Errorf("compress %s: %w", name, err)
		}
		return compressed, nil
	}
	selfer, ok := indexedMessagePackSelfer(v)
	if !ok {
		return nil, fmt.Errorf("encode %s: %T does not implement the indexed MessagePack codec", name, v)
	}
	encoded, err := ct.EncodeIndexedMsgpack(selfer)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", name, err)
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
func padSlice(arr []interface{}, size int64) []interface{} {
	if int64(len(arr)) >= size {
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
func jsonNumberForFloat(v float64, bitSize int64) json.Number {
	s := strconv.FormatFloat(v, 'g', -1, int(bitSize))
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return json.Number(s)
}
