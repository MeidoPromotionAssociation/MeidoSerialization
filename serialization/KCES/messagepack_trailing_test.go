package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

func TestCompressedMessagePackDecodersRejectTrailingData(t *testing.T) {
	tests := []struct {
		name   string
		encode func() ([]byte, error)
		decode func([]byte) error
	}{
		{name: "MaterialAssets", encode: func() ([]byte, error) { return EncodeMaterialAssets(&MaterialAssets{Assets: []*Material{}}) }, decode: func(data []byte) error { _, err := DecodeMaterialAssets(data); return err }},
		{name: "MenuAssets", encode: func() ([]byte, error) { return EncodeMenuAssets(&MenuAssets{Assets: []*Menu{}}) }, decode: func(data []byte) error { _, err := DecodeMenuAssets(data); return err }},
		{name: "Model", encode: func() ([]byte, error) { return EncodeModel(&Model{}) }, decode: func(data []byte) error { _, err := DecodeModel(data); return err }},
		{name: "ModelAssets", encode: func() ([]byte, error) { return EncodeModelAssets(&ModelAssets{Assets: []*Model{}}) }, decode: func(data []byte) error { _, err := DecodeModelAssets(data); return err }},
		{name: "PriorityMaterialAssets", encode: func() ([]byte, error) {
			return EncodePriorityMaterialAssets(&PriorityMaterialAssets{Assets: []*PriorityMaterial{}})
		}, decode: func(data []byte) error { _, err := DecodePriorityMaterialAssets(data); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire, err := test.encode()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := msgpack.DecompressLz4BlockArray(wire)
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, 0xc0)
			wire, err = msgpack.CompressLz4BlockArray(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := test.decode(wire); err == nil || !strings.Contains(strings.ToLower(err.Error()), "trailing") {
				t.Fatalf("decode error = %v, want trailing-data rejection", err)
			}
		})
	}
}

func TestCompressedMessagePackDecodersRoundTripTypedNilRoots(t *testing.T) {
	wire, err := msgpack.CompressLz4BlockArray([]byte{0xc0})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		decode func([]byte) (bool, error)
		encode func() ([]byte, error)
	}{
		{name: "MaterialAssets", decode: func(data []byte) (bool, error) { value, err := DecodeMaterialAssets(data); return value == nil, err }, encode: func() ([]byte, error) { return EncodeMaterialAssets(nil) }},
		{name: "MenuAssets", decode: func(data []byte) (bool, error) { value, err := DecodeMenuAssets(data); return value == nil, err }, encode: func() ([]byte, error) { return EncodeMenuAssets(nil) }},
		{name: "Model", decode: func(data []byte) (bool, error) { value, err := DecodeModel(data); return value == nil, err }, encode: func() ([]byte, error) { return EncodeModel(nil) }},
		{name: "ModelAssets", decode: func(data []byte) (bool, error) { value, err := DecodeModelAssets(data); return value == nil, err }, encode: func() ([]byte, error) { return EncodeModelAssets(nil) }},
		{name: "PriorityMaterialAssets", decode: func(data []byte) (bool, error) {
			value, err := DecodePriorityMaterialAssets(data)
			return value == nil, err
		}, encode: func() ([]byte, error) { return EncodePriorityMaterialAssets(nil) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isNil, err := test.decode(wire)
			if err != nil || !isNil {
				t.Fatalf("decode nil root: isNil=%v error=%v", isNil, err)
			}
			encoded, err := test.encode()
			if err != nil {
				t.Fatalf("encode typed nil root: %v", err)
			}
			isNil, err = test.decode(encoded)
			if err != nil || !isNil {
				t.Fatalf("round-trip typed nil root: isNil=%v error=%v", isNil, err)
			}

			trailingWire, err := msgpack.CompressLz4BlockArray([]byte{0xc0, 0xc0})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := test.decode(trailingWire); err == nil || !strings.Contains(strings.ToLower(err.Error()), "trailing") {
				t.Fatalf("decode nil root plus trailing value error = %v, want trailing-data rejection", err)
			}
		})
	}
}
