package KCES

import (
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

func rawArrayWidth(t *testing.T, data []byte) int {
	t.Helper()
	var values []codec.Raw
	if err := msgpack.DecodeMsgpack(data, &values); err != nil {
		t.Fatalf("decode raw array: %v", err)
	}
	return len(values)
}

func nestedCompressedArrayWidth(t *testing.T, data []byte, indexes ...int) int {
	t.Helper()
	raw, err := msgpack.DecompressLz4BlockArray(data)
	if err != nil {
		t.Fatalf("DecompressLz4BlockArray: %v", err)
	}
	var current codec.Raw = raw
	for _, index := range indexes {
		var values []codec.Raw
		if err := msgpack.DecodeMsgpack(current, &values); err != nil {
			t.Fatalf("decode nested array: %v", err)
		}
		if index < 0 || index >= len(values) {
			t.Fatalf("nested array index %d outside width %d", index, len(values))
		}
		current = values[index]
	}
	var values []codec.Raw
	if err := msgpack.DecodeMsgpack(current, &values); err != nil {
		t.Fatalf("decode target array: %v", err)
	}
	return len(values)
}
