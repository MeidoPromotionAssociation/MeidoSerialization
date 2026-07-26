package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

func TestIndexedObjectsRejectUnsupportedWidths(t *testing.T) {
	tests := []struct {
		name string
		wire []byte
		want string
	}{
		{name: "short vector", wire: []byte{0x92, 0x01, 0x02}, want: "indexed-array width 2"},
		{name: "long vector", wire: []byte{0x94, 0x01, 0x02, 0x03, 0x04}, want: "indexed-array width 4"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var value Vector3
			if err := msgpack.DecodeMsgpack(test.wire, &value); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeMsgpack() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestIndexedObjectsRejectTrailingRootData(t *testing.T) {
	wire := append([]byte{0x93, 0x01, 0x02, 0x03}, 0xc0)
	var value Vector3
	if err := msgpack.DecodeMsgpack(wire, &value); err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("DecodeMsgpack() error = %v, want trailing data", err)
	}
}

func TestSparseSlotsRejectNonNilValues(t *testing.T) {
	// MenuAssets key 24 is a fixed sparse slot in the game's current layout
	menu := make([]interface{}, 31)
	menu[0] = int64(1005)
	menu[1] = uint64(1)
	menu[2] = uint64(2)
	menu[7] = int64(0)
	menu[8] = uint64(0)
	menu[9] = false
	menu[10] = false
	menu[11] = false
	menu[12] = []interface{}{}
	menu[15] = uint64(0)
	menu[16] = map[uint64]interface{}{}
	menu[19] = uint64(0)
	menu[20] = uint64(0)
	menu[22] = false
	menu[23] = int64(0)
	menu[24] = int64(1)
	menu[25] = uint64(0)
	menu[26] = false
	menu[29] = int64(0)
	menu[30] = int64(0)
	wire := compressIndexedTestValue(t, []interface{}{"menuassets", []interface{}{menu}})
	if _, err := DecodeMenuAssets(wire); err == nil || !strings.Contains(err.Error(), "sparse slot") {
		t.Fatalf("DecodeMenuAssets() error = %v, want sparse-slot rejection", err)
	}
}

func TestUnknownColliderTagIsRejected(t *testing.T) {
	// The first slot is the discriminator used by the collider union
	data := lengthPrefixedIndexedTestValue(t, []interface{}{
		int64(1000),
		[]interface{}{
			[]interface{}{int64(99), []interface{}{}},
		},
		nil,
	})
	if _, err := DecodeKCESPayload(data, ".dbcol"); err == nil || !strings.Contains(err.Error(), "unsupported collider type") {
		t.Fatalf("DecodeKCESPayload() error = %v, want unsupported collider type", err)
	}
}

func compressIndexedTestValue(t *testing.T, value interface{}) []byte {
	t.Helper()
	h := &codec.MsgpackHandle{}
	h.Raw = true
	h.MaxDepth = 256
	var messagePack []byte
	if err := codec.NewEncoderBytes(&messagePack, h).Encode(value); err != nil {
		t.Fatalf("encode indexed test MessagePack: %v", err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(messagePack)
	if err != nil {
		t.Fatalf("compress indexed test MessagePack: %v", err)
	}
	return compressed
}

func lengthPrefixedIndexedTestValue(t *testing.T, value interface{}) []byte {
	t.Helper()
	return AddLengthPrefix(compressIndexedTestValue(t, value))
}

func decodeCompressedIndexedTestArray(t *testing.T, data []byte) []codec.Raw {
	t.Helper()
	messagePack, err := msgpack.DecompressLz4BlockArray(data)
	if err != nil {
		t.Fatalf("decompress indexed test wire: %v", err)
	}
	return decodeIndexedTestArray(t, messagePack)
}

func decodeLengthPrefixedIndexedTestArray(t *testing.T, data []byte) []codec.Raw {
	t.Helper()
	compressed, _, err := StripLengthPrefix(data)
	if err != nil {
		t.Fatalf("strip indexed test length prefix: %v", err)
	}
	return decodeCompressedIndexedTestArray(t, compressed)
}

func decodeIndexedTestArray(t *testing.T, data []byte) []codec.Raw {
	t.Helper()
	if len(data) == 0 {
		data = []byte{0xc0}
	}
	var slots []codec.Raw
	if err := msgpack.DecodeMsgpack(data, &slots); err != nil {
		t.Fatalf("decode indexed test array: %v; wire=% x", err, data)
	}
	return slots
}

func rawMessagePackEqual(got codec.Raw, want []byte) bool {
	if len(got) == 0 {
		got = codec.Raw{0xc0}
	}
	if len(want) == 0 {
		want = []byte{0xc0}
	}
	return string(got) == string(want)
}

func assertRawNil(t *testing.T, raw codec.Raw, label string) {
	t.Helper()
	if !rawMessagePackEqual(raw, []byte{0xc0}) {
		t.Fatalf("%s = % x, want nil", label, raw)
	}
}

func colliderCapsuleIndexedTestValue(version int32) []interface{} {
	return []interface{}{
		int64(version), "parent", "name",
		[]interface{}{float32(0), float32(0), float32(0)},
		[]interface{}{float32(0), float32(0), float32(0), float32(1)},
		[]interface{}{float32(1), float32(1), float32(1)},
		[]interface{}{float32(0), float32(0), float32(0)},
		int64(0), int64(VectorTypeY), false,
		float32(0.5), float32(0.5), float32(0),
	}
}

func colliderMaidPropIndexedTestValue(version int32) []interface{} {
	value := append([]interface{}(nil), colliderCapsuleIndexedTestValue(version)...)
	value = append(value, nil, nil, nil)
	return append(value,
		[]interface{}{},
		[]interface{}{float32(0), float32(0), float32(0)},
		[]interface{}{}, float32(1),
		[]interface{}{}, float32(1),
		[]interface{}{}, []interface{}{}, []interface{}{},
	)
}
