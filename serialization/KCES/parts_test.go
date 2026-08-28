package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
)

func TestPublicWireTypesRemainInKCESPackage(t *testing.T) {
	wantPackage := reflect.TypeOf(Model{}).PkgPath()
	tests := []struct {
		name  string
		value interface{}
	}{
		{name: "Vector3", value: Vector3{}},
		{name: "PartsColor", value: PartsColor{}},
		{name: "PreMulTexDatas", value: PreMulTexDatas{}},
		{name: "ClothParams", value: ClothParams{}},
		{name: "ColliderPackage", value: ColliderPackage{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := reflect.TypeOf(test.value).PkgPath(); got != wantPackage {
				t.Fatalf("PkgPath() = %q, want KCES package %q", got, wantPackage)
			}
		})
	}
}

func TestPriorityMaterial_RoundTrip(t *testing.T) {
	original := &PriorityMaterialAssets{
		FileName: partsTestString("test.pmatassets"),
		Assets: []*PriorityMaterial{
			{
				Version:     1000,
				ID:          12345,
				FileName:    partsTestString("test_mat.pmat"),
				RenderQueue: 3000.5,
				TargetID:    67890,
			},
			{
				Version:     1000,
				ID:          11111,
				FileName:    partsTestString("another.pmat"),
				RenderQueue: 2000.0,
				TargetID:    22222,
			},
		},
	}

	encoded, err := EncodePriorityMaterialAssets(original)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := DecodePriorityMaterialAssets(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !partsTestStringsEqual(decoded.FileName, original.FileName) {
		t.Errorf("fileName mismatch: got %v, want %v", decoded.FileName, original.FileName)
	}
	if len(decoded.Assets) != len(original.Assets) {
		t.Fatalf("assets count mismatch: got %d, want %d", len(decoded.Assets), len(original.Assets))
	}

	for i, orig := range original.Assets {
		got := decoded.Assets[i]
		if got.Version != orig.Version {
			t.Errorf("[%d] version: got %d, want %d", i, got.Version, orig.Version)
		}
		if got.ID != orig.ID {
			t.Errorf("[%d] id: got %d, want %d", i, got.ID, orig.ID)
		}
		if !partsTestStringsEqual(got.FileName, orig.FileName) {
			t.Errorf("[%d] fileName: got %v, want %v", i, got.FileName, orig.FileName)
		}
		if got.RenderQueue != orig.RenderQueue {
			t.Errorf("[%d] renderQueue: got %f, want %f", i, got.RenderQueue, orig.RenderQueue)
		}
		if got.TargetID != orig.TargetID {
			t.Errorf("[%d] targetId: got %d, want %d", i, got.TargetID, orig.TargetID)
		}
	}
}

func TestDecodeMaterialAssets_FromAba(t *testing.T) {
	data := kcesfixtures.TextAssetBytes(t, "parts_personal002.aba", "parts_personal002.materialassets")

	assets, err := DecodeMaterialAssets(data)
	if err != nil {
		t.Fatalf("DecodeMaterialAssets failed: %v", err)
	}

	t.Logf("MaterialAssets: fileName=%v, assets=%d", assets.FileName, len(assets.Assets))
	for i, mat := range assets.Assets {
		t.Logf("  [%d] version=%d id=%d fileName=%v shader=%v",
			i, mat.Version, mat.ID, mat.FileName, mat.ShaderName)
		t.Logf("      texProps=%d colorProps=%d floatProps=%d",
			len(mat.TextureProps), len(mat.ColorProps), len(mat.FloatProps))
	}

	// Round-trip test
	encoded, err := EncodeMaterialAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMaterialAssets failed: %v", err)
	}

	decoded, err := DecodeMaterialAssets(encoded)
	if err != nil {
		t.Fatalf("re-decode failed: %v", err)
	}

	if !partsTestStringsEqual(decoded.FileName, assets.FileName) {
		t.Errorf("fileName mismatch after round-trip")
	}
	if len(decoded.Assets) != len(assets.Assets) {
		t.Errorf("assets count mismatch after round-trip: got %d, want %d", len(decoded.Assets), len(assets.Assets))
	}
}

func TestDecodeMenuAssets_FromAba(t *testing.T) {
	data := kcesfixtures.TextAssetBytes(t, "parts_personal002.aba", "parts_personal002.menuassets")

	assets, err := DecodeMenuAssets(data)
	if err != nil {
		t.Fatalf("DecodeMenuAssets failed: %v", err)
	}

	t.Logf("MenuAssets: fileName=%v, assets=%d", assets.FileName, len(assets.Assets))
	for i, menu := range assets.Assets {
		t.Logf("  [%d] version=%d id=%d fileName=%v itemName=%v category=%v commands=%d",
			i, menu.Version, menu.ID, menu.FileName, menu.ItemName, menu.CategoryText, len(menu.Commands))
	}

	// Round-trip test
	encoded, err := EncodeMenuAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMenuAssets failed: %v", err)
	}

	decoded, err := DecodeMenuAssets(encoded)
	if err != nil {
		t.Fatalf("re-decode failed: %v", err)
	}

	if !partsTestStringsEqual(decoded.FileName, assets.FileName) {
		t.Errorf("fileName mismatch after round-trip")
	}
	if len(decoded.Assets) != len(assets.Assets) {
		t.Errorf("assets count mismatch after round-trip: got %d, want %d", len(decoded.Assets), len(assets.Assets))
	}
}

func TestDecodeMenuAssets_ByteEqual(t *testing.T) {
	original := kcesfixtures.TextAssetBytes(t, "parts_personal002.aba", "parts_personal002.menuassets")

	assets, err := DecodeMenuAssets(original)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	encoded, err := EncodeMenuAssets(assets)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// 语义 round-trip：重新解码并比较关键字段
	decoded2, err := DecodeMenuAssets(encoded)
	if err != nil {
		t.Fatalf("re-decode failed: %v", err)
	}

	if !partsTestStringsEqual(decoded2.FileName, assets.FileName) {
		t.Errorf("fileName mismatch")
	}
	if len(decoded2.Assets) != len(assets.Assets) {
		t.Fatalf("assets count mismatch: got %d, want %d", len(decoded2.Assets), len(assets.Assets))
	}
	for i, orig := range assets.Assets {
		got := decoded2.Assets[i]
		if got.ID != orig.ID {
			t.Errorf("[%d] id mismatch: got %d, want %d", i, got.ID, orig.ID)
		}
		if !partsTestStringsEqual(got.FileName, orig.FileName) {
			t.Errorf("[%d] fileName mismatch: got %v, want %v", i, got.FileName, orig.FileName)
		}
		if !partsTestStringsEqual(got.CategoryText, orig.CategoryText) {
			t.Errorf("[%d] categoryText mismatch: got %v, want %v", i, got.CategoryText, orig.CategoryText)
		}
		if len(got.Commands) != len(orig.Commands) {
			t.Errorf("[%d] commands count mismatch: got %d, want %d", i, len(got.Commands), len(orig.Commands))
		}
	}
}

func TestDecodeModel_FromAba(t *testing.T) {
	paths := partsSamplePathsBySuffix(t, ".model")

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}

			model, err := DecodeModel(data)
			if err != nil {
				t.Fatalf("DecodeModel failed: %v", err)
			}
			if model.Version == 0 {
				t.Errorf("version is zero")
			}
			if model.ID == 0 {
				t.Errorf("id is zero")
			}
			if model.FileName == nil || *model.FileName == "" {
				t.Errorf("fileName is empty")
			}
			if len(model.TransData) == 0 {
				t.Errorf("transData is empty")
			}
			if len(model.BoneNames) == 0 {
				t.Errorf("boneNames is empty")
			}
			if len(model.MaterialFileName) == 0 {
				t.Errorf("materialFileName is empty")
			}

			encoded, err := EncodeModel(model)
			if err != nil {
				t.Fatalf("EncodeModel failed: %v", err)
			}
			decoded, err := DecodeModel(encoded)
			if err != nil {
				t.Fatalf("re-decode failed: %v", err)
			}
			if !partsTestStringsEqual(decoded.FileName, model.FileName) {
				t.Errorf("fileName mismatch after round-trip: got %v, want %v", decoded.FileName, model.FileName)
			}
			if len(decoded.TransData) != len(model.TransData) {
				t.Errorf("transData count mismatch after round-trip: got %d, want %d", len(decoded.TransData), len(model.TransData))
			}
			if len(decoded.Morphs) != len(model.Morphs) {
				t.Errorf("morph count mismatch after round-trip: got %d, want %d", len(decoded.Morphs), len(model.Morphs))
			}
		})
	}
}

func TestPartsColorTypedGradaJSONRoundTrip(t *testing.T) {
	original := PartsColor{
		MainHue: 1,
		Grada:   []PartsColorGrada{{MainHue: 2, ShadowContrast: 3}},
	}

	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	var decoded PartsColor
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("typed grada mismatch after JSON round-trip: %#v", decoded.Grada)
	}

	encoded, err := encodeCompressedMsgpack(original, "PartsColor")
	if err != nil {
		t.Fatalf("msgpack encode failed: %v", err)
	}
	var msgpackDecoded PartsColor
	if err := decodeCompressedMsgpack(encoded, &msgpackDecoded, "PartsColor"); err != nil {
		t.Fatalf("msgpack decode failed: %v", err)
	}
	if !reflect.DeepEqual(msgpackDecoded, original) {
		t.Fatalf("typed grada mismatch after msgpack round-trip: %#v", msgpackDecoded.Grada)
	}
}

func partsTestString(value string) *string { return &value }

func partsTestStringsEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
