package KCES

import (
	"bytes"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestCompressedMessagePackRootTrailingDataRoundTrip(t *testing.T) {
	tail := []byte{0xde, 0xad, 0xbe, 0xef, 0xc1}
	tests := []struct {
		name          string
		rootSlots     int
		encodeInitial func() ([]byte, error)
		decodeEncode  func([]byte) ([]byte, []byte, error)
	}{
		{
			name:      "MaterialAssets",
			rootSlots: 2,
			encodeInitial: func() ([]byte, error) {
				return EncodeMaterialAssets(&MaterialAssets{TrailingData: append([]byte(nil), tail...)})
			},
			decodeEncode: func(data []byte) ([]byte, []byte, error) {
				value, err := DecodeMaterialAssets(data)
				if err != nil {
					return nil, nil, err
				}
				encoded, err := EncodeMaterialAssets(value)
				return value.TrailingData, encoded, err
			},
		},
		{
			name:      "MenuAssets",
			rootSlots: 2,
			encodeInitial: func() ([]byte, error) {
				return EncodeMenuAssets(&MenuAssets{TrailingData: append([]byte(nil), tail...)})
			},
			decodeEncode: func(data []byte) ([]byte, []byte, error) {
				value, err := DecodeMenuAssets(data)
				if err != nil {
					return nil, nil, err
				}
				encoded, err := EncodeMenuAssets(value)
				return value.TrailingData, encoded, err
			},
		},
		{
			name:      "Model",
			rootSlots: 11,
			encodeInitial: func() ([]byte, error) {
				return EncodeModel(&Model{TrailingData: append([]byte(nil), tail...)})
			},
			decodeEncode: func(data []byte) ([]byte, []byte, error) {
				value, err := DecodeModel(data)
				if err != nil {
					return nil, nil, err
				}
				encoded, err := EncodeModel(value)
				return value.TrailingData, encoded, err
			},
		},
		{
			name:      "ModelAssets",
			rootSlots: 2,
			encodeInitial: func() ([]byte, error) {
				return EncodeModelAssets(&ModelAssets{TrailingData: append([]byte(nil), tail...)})
			},
			decodeEncode: func(data []byte) ([]byte, []byte, error) {
				value, err := DecodeModelAssets(data)
				if err != nil {
					return nil, nil, err
				}
				encoded, err := EncodeModelAssets(value)
				return value.TrailingData, encoded, err
			},
		},
		{
			name:      "PriorityMaterialAssets",
			rootSlots: 2,
			encodeInitial: func() ([]byte, error) {
				return EncodePriorityMaterialAssets(&PriorityMaterialAssets{TrailingData: append([]byte(nil), tail...)})
			},
			decodeEncode: func(data []byte) ([]byte, []byte, error) {
				value, err := DecodePriorityMaterialAssets(data)
				if err != nil {
					return nil, nil, err
				}
				encoded, err := EncodePriorityMaterialAssets(value)
				return value.TrailingData, encoded, err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := tc.encodeInitial()
			if err != nil {
				t.Fatalf("initial encode: %v", err)
			}
			assertCompressedRootAndTail(t, encoded, tc.rootSlots, tail)

			decodedTail, reencoded, err := tc.decodeEncode(encoded)
			if err != nil {
				t.Fatalf("decode/re-encode: %v", err)
			}
			if !bytes.Equal(decodedTail, tail) {
				t.Fatalf("decoded trailingData = % x, want % x", decodedTail, tail)
			}
			assertCompressedRootAndTail(t, reencoded, tc.rootSlots, tail)
		})
	}
}

func TestModelAssetsRejectsStandaloneModelTrailingData(t *testing.T) {
	_, err := EncodeModelAssets(&ModelAssets{Assets: []Model{{TrailingData: []byte{1}}}})
	if err == nil {
		t.Fatal("nested standalone Model trailingData was silently discarded")
	}
}

func TestCompressedMessagePackNilRootRoundTrip(t *testing.T) {
	tail := []byte{0xde, 0xad, 0xc1}
	compressed, err := ct.CompressLz4BlockArray(append([]byte{0xc0}, tail...))
	if err != nil {
		t.Fatalf("compress nil root: %v", err)
	}
	tests := []struct {
		name         string
		decodeEncode func([]byte) (bool, []byte, []byte, error)
	}{
		{
			name: "MaterialAssets",
			decodeEncode: func(data []byte) (bool, []byte, []byte, error) {
				value, err := DecodeMaterialAssets(data)
				if err != nil {
					return false, nil, nil, err
				}
				encoded, err := EncodeMaterialAssets(value)
				return value.RootNil, value.TrailingData, encoded, err
			},
		},
		{
			name: "MenuAssets",
			decodeEncode: func(data []byte) (bool, []byte, []byte, error) {
				value, err := DecodeMenuAssets(data)
				if err != nil {
					return false, nil, nil, err
				}
				encoded, err := EncodeMenuAssets(value)
				return value.RootNil, value.TrailingData, encoded, err
			},
		},
		{
			name: "Model",
			decodeEncode: func(data []byte) (bool, []byte, []byte, error) {
				value, err := DecodeModel(data)
				if err != nil {
					return false, nil, nil, err
				}
				encoded, err := EncodeModel(value)
				return value.RootNil, value.TrailingData, encoded, err
			},
		},
		{
			name: "ModelAssets",
			decodeEncode: func(data []byte) (bool, []byte, []byte, error) {
				value, err := DecodeModelAssets(data)
				if err != nil {
					return false, nil, nil, err
				}
				encoded, err := EncodeModelAssets(value)
				return value.RootNil, value.TrailingData, encoded, err
			},
		},
		{
			name: "PriorityMaterialAssets",
			decodeEncode: func(data []byte) (bool, []byte, []byte, error) {
				value, err := DecodePriorityMaterialAssets(data)
				if err != nil {
					return false, nil, nil, err
				}
				encoded, err := EncodePriorityMaterialAssets(value)
				return value.RootNil, value.TrailingData, encoded, err
			},
		},
		{
			name: "KCESPresetCore",
			decodeEncode: func(data []byte) (bool, []byte, []byte, error) {
				value := &KCESPresetCore{}
				if err := decodeCompressedMsgpack(data, value, "test preset core"); err != nil {
					return false, nil, nil, err
				}
				encoded, err := encodeCompressedMsgpack(value, "test preset core")
				return value.RootNil, value.TrailingData, encoded, err
			},
		},
		{
			name: "KCESPresetMeta",
			decodeEncode: func(data []byte) (bool, []byte, []byte, error) {
				value := &KCESPresetMeta{}
				if err := decodeCompressedMsgpack(data, value, "test preset meta"); err != nil {
					return false, nil, nil, err
				}
				encoded, err := encodeCompressedMsgpack(value, "test preset meta")
				return value.RootNil, value.TrailingData, encoded, err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootNil, decodedTail, reencoded, err := test.decodeEncode(compressed)
			if err != nil {
				t.Fatalf("decode/re-encode nil root: %v", err)
			}
			if !rootNil {
				t.Fatal("nil root marker was not retained")
			}
			if !bytes.Equal(decodedTail, tail) {
				t.Fatalf("decoded trailingData = % x, want % x", decodedTail, tail)
			}
			decompressed, err := ct.DecompressLz4BlockArray(reencoded)
			if err != nil {
				t.Fatalf("decompress re-encoded nil root: %v", err)
			}
			want := append([]byte{0xc0}, tail...)
			if !bytes.Equal(decompressed, want) {
				t.Fatalf("re-encoded nil root = % x, want % x", decompressed, want)
			}
		})
	}
}

func TestCompressedMessagePackNilRootRejectsDiscardedPayload(t *testing.T) {
	if _, err := EncodeMaterialAssets(&MaterialAssets{RootNil: true, FileName: "lost.materialassets"}); err == nil {
		t.Fatal("rootNil silently discarded populated MaterialAssets fields")
	}
	zero := 0
	if _, err := EncodeMenuAssets(&MenuAssets{
		RootNil: true,
		IndexedObjectMetadata: &IndexedObjectMetadata{
			FieldCount: &zero,
		},
	}); err == nil {
		t.Fatal("rootNil silently discarded indexed-object width metadata")
	}
	if _, err := EncodeModelAssets(&ModelAssets{Assets: []Model{{RootNil: true}}}); err == nil {
		t.Fatal("nested standalone Model rootNil was silently discarded")
	}
}

func assertCompressedRootAndTail(t *testing.T, compressed []byte, rootSlots int, tail []byte) {
	t.Helper()
	decompressed, err := ct.DecompressLz4BlockArray(compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	var root []interface{}
	consumed, err := ct.DecodeMsgpackWithConsumed(decompressed, &root)
	if err != nil {
		t.Fatalf("decode root: %v", err)
	}
	if len(root) != rootSlots {
		t.Fatalf("root array slots = %d, want %d; codec metadata leaked into wire", len(root), rootSlots)
	}
	if !bytes.Equal(decompressed[consumed:], tail) {
		t.Fatalf("wire trailing bytes = % x, want % x", decompressed[consumed:], tail)
	}
}
