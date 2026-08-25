package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
)

func TestPublicDecodersRejectCLRInt32Overflow(t *testing.T) {
	menuWire, err := EncodeMenuAssets(newInt32TestMenuAssets())
	if err != nil {
		t.Fatalf("EncodeMenuAssets fixture: %v", err)
	}
	model := validModelForInt32Test()
	modelWire, err := EncodeModel(&model)
	if err != nil {
		t.Fatalf("EncodeModel fixture: %v", err)
	}

	for _, overflow := range []int64{int64(testMinInt32) - 1, int64(testMaxInt32) + 1} {
		overflow := overflow
		t.Run("menu", func(t *testing.T) {
			wire := mutateCompressedInt32TestRoot(t, menuWire, func(root []interface{}) {
				assets := requireInt32TestArray(t, root[1], "MenuAssets.assetArray")
				menu := requireInt32TestArray(t, assets[0], "MenuAssets.assetArray[0]")
				menu[0] = overflow
			})
			_, err := DecodeMenuAssets(wire)
			assertInt32DecodeError(t, err, "version")
		})

		t.Run("model", func(t *testing.T) {
			wire := mutateCompressedInt32TestRoot(t, modelWire, func(root []interface{}) {
				root[0] = overflow
			})
			_, err := DecodeModel(wire)
			assertInt32DecodeError(t, err, "version")
		})
	}
}

func TestPayloadDecoderRejectsCLRInt32Overflow(t *testing.T) {
	validWire, err := EncodeKCESPayload(NewDynamicBoneStatus(), ".dbconf")
	if err != nil {
		t.Fatalf("EncodeKCESPayload fixture: %v", err)
	}
	payload, prefixed, err := StripLengthPrefix(validWire)
	if err != nil {
		t.Fatalf("StripLengthPrefix fixture: %v", err)
	}
	if !prefixed {
		t.Fatal("dynamic-bone fixture is not length-prefixed")
	}

	for _, overflow := range []int64{int64(testMinInt32) - 1, int64(testMaxInt32) + 1} {
		mutated := mutateCompressedInt32TestRoot(t, payload, func(root []interface{}) {
			root[0] = overflow
		})
		_, err := DecodeKCESPayload(AddLengthPrefix(mutated), ".dbconf")
		assertInt32DecodeError(t, err, "version")
	}
}

func mutateCompressedInt32TestRoot(t *testing.T, wire []byte, mutate func([]interface{})) []byte {
	t.Helper()
	raw, err := msgpack.DecompressLz4BlockArray(wire)
	if err != nil {
		t.Fatalf("decompress Int32 test fixture: %v", err)
	}
	var root []interface{}
	if err := msgpack.DecodeMsgpack(raw, &root); err != nil {
		t.Fatalf("decode Int32 test fixture: %v", err)
	}
	mutate(root)
	encoded, err := msgpack.EncodeMsgpack(root)
	if err != nil {
		t.Fatalf("encode mutated Int32 test fixture: %v", err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(encoded)
	if err != nil {
		t.Fatalf("compress mutated Int32 test fixture: %v", err)
	}
	return compressed
}

func requireInt32TestArray(t *testing.T, value interface{}, path string) []interface{} {
	t.Helper()
	result, ok := value.([]interface{})
	if !ok {
		t.Fatalf("%s has type %T, want []interface{}", path, value)
	}
	return result
}

func assertInt32DecodeError(t *testing.T, err error, path string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "Int32") || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(path)) {
		t.Fatalf("decoder error = %v, want Int32 rejection at %q", err, path)
	}
}
