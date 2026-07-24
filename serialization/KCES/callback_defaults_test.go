package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestDecodeDynamicBoneStatusRejectsUnknownShortArray(t *testing.T) {
	msgpackData, err := ct.EncodeMsgpack([]interface{}{int64(999)})
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}
	compressed, err := ct.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatalf("CompressLz4BlockArray: %v", err)
	}

	_, err = DecodeKCESPayload(AddLengthPrefix(compressed), ".dbconf")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "width") {
		t.Fatalf("DecodeKCESPayload() error = %v, want unsupported indexed-array width", err)
	}
}

func TestPartsAssetsEncodersPreserveNilAssets(t *testing.T) {
	menuAssets := &MenuAssets{}
	modelAssets := &ModelAssets{}
	materialAssets := &MaterialAssets{}
	priorityMaterialAssets := &PriorityMaterialAssets{}
	tests := []struct {
		name          string
		encode        func() ([]byte, error)
		inputStillNil func() bool
		decodeIsNil   func([]byte) (bool, error)
	}{
		{
			name:          "menu",
			encode:        func() ([]byte, error) { return EncodeMenuAssets(menuAssets) },
			inputStillNil: func() bool { return menuAssets.Assets == nil },
			decodeIsNil: func(data []byte) (bool, error) {
				decoded, err := DecodeMenuAssets(data)
				return err == nil && decoded.Assets == nil, err
			},
		},
		{
			name:          "model",
			encode:        func() ([]byte, error) { return EncodeModelAssets(modelAssets) },
			inputStillNil: func() bool { return modelAssets.Assets == nil },
			decodeIsNil: func(data []byte) (bool, error) {
				decoded, err := DecodeModelAssets(data)
				return err == nil && decoded.Assets == nil, err
			},
		},
		{
			name:          "material",
			encode:        func() ([]byte, error) { return EncodeMaterialAssets(materialAssets) },
			inputStillNil: func() bool { return materialAssets.Assets == nil },
			decodeIsNil: func(data []byte) (bool, error) {
				decoded, err := DecodeMaterialAssets(data)
				return err == nil && decoded.Assets == nil, err
			},
		},
		{
			name:          "priority material",
			encode:        func() ([]byte, error) { return EncodePriorityMaterialAssets(priorityMaterialAssets) },
			inputStillNil: func() bool { return priorityMaterialAssets.Assets == nil },
			decodeIsNil: func(data []byte) (bool, error) {
				decoded, err := DecodePriorityMaterialAssets(data)
				return err == nil && decoded.Assets == nil, err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := test.encode()
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !test.inputStillNil() {
				t.Fatal("encoding mutated caller assetArray")
			}
			isNil, err := test.decodeIsNil(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !isNil {
				t.Fatal("assetArray was silently changed from nil to empty")
			}
		})
	}
}

func TestMaterialAssetsEncoderPreservesNilPropertyArrays(t *testing.T) {
	assets := &MaterialAssets{Assets: []*Material{{}}}
	encoded, err := EncodeMaterialAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMaterialAssets: %v", err)
	}
	if assets.Assets[0].TextureProps != nil || assets.Assets[0].ColorProps != nil ||
		assets.Assets[0].VectorProps != nil || assets.Assets[0].FloatProps != nil {
		t.Fatalf("encoding mutated input: %+v", assets.Assets[0])
	}

	decoded, err := DecodeMaterialAssets(encoded)
	if err != nil {
		t.Fatalf("DecodeMaterialAssets: %v", err)
	}
	material := decoded.Assets[0]
	if material.TextureProps != nil || material.ColorProps != nil ||
		material.VectorProps != nil || material.FloatProps != nil {
		t.Fatalf("nil material property arrays were silently changed: %+v", material)
	}
}

func TestModelEncodersPreserveNilMembers(t *testing.T) {
	assertNil := func(t *testing.T, model *Model) {
		t.Helper()
		if model == nil {
			t.Fatal("model unexpectedly decoded as null")
		}
		if model.TransData != nil || model.BoneNames != nil || model.MaterialFileName != nil || model.Morphs != nil {
			t.Fatalf("nil model arrays were silently changed: %+v", model)
		}
		if model.SkinThick != nil {
			t.Fatal("nil skinThick was silently synthesized")
		}
	}
	assertInputUntouched := func(t *testing.T, model *Model) {
		t.Helper()
		if model == nil {
			t.Fatal("input model unexpectedly null")
		}
		if model.TransData != nil || model.BoneNames != nil || model.MaterialFileName != nil ||
			model.Morphs != nil || model.SkinThick != nil {
			t.Fatalf("encoding mutated input: %+v", model)
		}
	}

	t.Run("single model", func(t *testing.T) {
		model := &Model{}
		encoded, err := EncodeModel(model)
		if err != nil {
			t.Fatalf("EncodeModel: %v", err)
		}
		assertInputUntouched(t, model)
		decoded, err := DecodeModel(encoded)
		if err != nil {
			t.Fatalf("DecodeModel: %v", err)
		}
		assertNil(t, decoded)
	})

	t.Run("model assets", func(t *testing.T) {
		assets := &ModelAssets{Assets: []*Model{{}}}
		encoded, err := EncodeModelAssets(assets)
		if err != nil {
			t.Fatalf("EncodeModelAssets: %v", err)
		}
		assertInputUntouched(t, assets.Assets[0])
		decoded, err := DecodeModelAssets(encoded)
		if err != nil {
			t.Fatalf("DecodeModelAssets: %v", err)
		}
		assertNil(t, decoded.Assets[0])
	})
}
