package KCES

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func decodeCompressedMsgpack(data []byte, out interface{}, name string) error {
	decompressed, err := ct.DecompressLz4BlockArray(data)
	if err != nil {
		decompressed = data
	}

	if err := ct.DecodeMsgpack(decompressed, out); err != nil {
		return fmt.Errorf("decode %s msgpack: %w", name, err)
	}
	return nil
}

func encodeCompressedMsgpack(v interface{}, name string) ([]byte, error) {
	encoded, err := ct.EncodeMsgpack(v)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", name, err)
	}

	compressed, err := ct.CompressLz4BlockArray(encoded)
	if err != nil {
		return nil, fmt.Errorf("compress %s: %w", name, err)
	}
	return compressed, nil
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

func toIntVal(v interface{}) (int, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := strconv.ParseInt(n.String(), 10, 0)
		return int(i), err == nil
	case int64:
		return int(n), true
	case uint64:
		return int(n), true
	case int:
		return n, true
	case uint:
		return int(n), true
	}
	return 0, false
}

func toUint64Val(v interface{}) (uint64, bool) {
	switch n := v.(type) {
	case json.Number:
		u, err := strconv.ParseUint(n.String(), 10, 64)
		return u, err == nil
	case uint64:
		return n, true
	case int64:
		return uint64(n), true
	case int:
		return uint64(n), true
	case uint:
		return uint64(n), true
	}
	return 0, false
}

func toFloat32(v interface{}) (float32, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(n.String(), 32)
		return float32(f), err == nil
	case float64:
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
		return f, err == nil
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

func toInt64Val(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := strconv.ParseInt(n.String(), 10, 64)
		return i, err == nil
	case int64:
		return n, true
	case uint64:
		return int64(n), true
	case int:
		return int64(n), true
	case uint:
		return int64(n), true
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

func padSlice(arr []interface{}, size int) []interface{} {
	if len(arr) >= size {
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

func jsonNumberForFloat(v float64, bitSize int) json.Number {
	s := strconv.FormatFloat(v, 'g', -1, bitSize)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return json.Number(s)
}
