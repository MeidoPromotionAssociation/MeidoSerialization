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

// KCES 各格式共用的 MessagePack 根值、LZ4 Block Array 和基础值转换辅助函数。
//
// MessagePack-root, LZ4 Block Array, and primitive conversion helpers shared by KCES formats.

// messagePackTrailingCarrier 由顶层游戏记录实现，用于保存在 MessagePack-CSharp 格式化器结果之后仍未读取的字节。这些字节位于索引对象之外，不能成为额外的结构体槽位。
//
// messagePackTrailingCarrier is implemented by top-level game records whose
// formatter result may be followed by bytes that MessagePack-CSharp leaves
// unread. The bytes are outside the indexed object itself and therefore must
// never become another struct slot.
type messagePackTrailingCarrier interface {
	getMessagePackTrailing() []byte
	setMessagePackTrailing([]byte)
}

type messagePackRootCarrier interface {
	getMessagePackRootNil() bool
	setMessagePackRootNil(bool)
}

// MessagePackRootMetadata 与 ct 共用，使所有面向 MessagePack 的模型采用一致的 JSON 和 wire 元数据形状。
//
// MessagePackRootMetadata is shared with ct so all MessagePack-facing models
// use one JSON/wire annotation shape.
type MessagePackRootMetadata = ct.MessagePackRootMetadata

// IndexedObjectMetadata 保存解码后的 MessagePack-CSharp 整数键数组的精确宽度、未来原始槽位以及可空元素标记。
//
// IndexedObjectMetadata records a decoded MessagePack-CSharp int-key array's
// exact width, future raw slots, and nullable-element markers.
type IndexedObjectMetadata = ct.TypedIndexedObjectMetadata

func messagePackRootTrailingAfterParsed(data []byte, parsed int64, name string) ([]byte, error) {
	if parsed < 0 || parsed > int64(len(data)) {
		return nil, fmt.Errorf("decode %s parser consumed invalid byte count %d of %d", name, parsed, len(data))
	}
	root, trailing, err := ct.SplitFirstMsgpackValue(data)
	if err != nil {
		return nil, fmt.Errorf("decode %s root MessagePack value: %w", name, err)
	}
	if int64(len(root)) != parsed {
		return nil, fmt.Errorf("decode %s parser consumed %d bytes but MessagePack library reports a %d-byte root value", name, parsed, len(root))
	}
	return trailing, nil
}

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

func appendMessagePackRootTrailing(root []byte, metadata MessagePackRootMetadata) []byte {
	return append(root, metadata.TrailingData...)
}

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
	if trailingCarrier, ok := out.(messagePackTrailingCarrier); ok {
		trailingCarrier.setMessagePackTrailing(append([]byte(nil), trailing...))
	}
	return nil
}

func cloneSlicePreserveNil[T any](src []T) []T {
	if src == nil {
		return nil
	}
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}

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

func resetMessagePackRootValue(value interface{}) {
	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Ptr && !rv.IsNil() && rv.Elem().CanSet() {
		rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
	}
}

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

func indexedObjectMetadataHasPayload(metadata IndexedObjectMetadata) bool {
	return metadata.FieldCount != nil || len(metadata.FutureSlots) != 0 || len(metadata.NilSlots) != 0 || len(metadata.NullElements) != 0 || len(metadata.NullMapValueKeys) != 0
}

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

func toFloat32(v interface{}) (float32, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(n.String(), 32)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return float32(f), true
	case float64:
		// MessagePackReader.ReadSingle accepts floating values and performs the
		// CLR conversion. NaN, infinities, and finite double values that overflow
		// to a Single infinity are still representable runtime results.
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

func toBool(v interface{}) (bool, bool) {
	if b, ok := v.(bool); ok {
		return b, true
	}
	return false, false
}

func toStringVal(v interface{}) (string, bool) {
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

func padSlice(arr []interface{}, size int64) []interface{} {
	if int64(len(arr)) >= size {
		return arr
	}
	padded := make([]interface{}, size)
	copy(padded, arr)
	return padded
}

// float32ToUint32Bits 用于 MessagePack 中 float32 的精确编码
func float32ToUint32Bits(f float32) uint32 {
	return math.Float32bits(f)
}

func jsonNumberForFloat(v float64, bitSize int64) json.Number {
	s := strconv.FormatFloat(v, 'g', -1, int(bitSize))
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return json.Number(s)
}
