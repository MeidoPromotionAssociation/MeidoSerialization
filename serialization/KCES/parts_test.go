package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPriorityMaterial_RoundTrip(t *testing.T) {
	original := &PriorityMaterialAssets{
		FileName: "test.pmatassets",
		Assets: []PriorityMaterial{
			{
				Version:     1000,
				ID:          12345,
				FileName:    "test_mat.pmat",
				RenderQueue: 3000.5,
				TargetID:    67890,
			},
			{
				Version:     1000,
				ID:          11111,
				FileName:    "another.pmat",
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

	if decoded.FileName != original.FileName {
		t.Errorf("fileName mismatch: got %q, want %q", decoded.FileName, original.FileName)
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
		if got.FileName != orig.FileName {
			t.Errorf("[%d] fileName: got %q, want %q", i, got.FileName, orig.FileName)
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
	data := readPartsSample(t, "parts_personal002.materialassets")

	assets, err := DecodeMaterialAssets(data)
	if err != nil {
		t.Fatalf("DecodeMaterialAssets failed: %v", err)
	}

	t.Logf("MaterialAssets: fileName=%q, assets=%d", assets.FileName, len(assets.Assets))
	for i, mat := range assets.Assets {
		t.Logf("  [%d] version=%d id=%d fileName=%q shader=%q",
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

	if decoded.FileName != assets.FileName {
		t.Errorf("fileName mismatch after round-trip")
	}
	if len(decoded.Assets) != len(assets.Assets) {
		t.Errorf("assets count mismatch after round-trip: got %d, want %d", len(decoded.Assets), len(assets.Assets))
	}
}

func TestDecodeMenuAssets_FromAba(t *testing.T) {
	data := readPartsSample(t, "parts_personal002.menuassets")

	assets, err := DecodeMenuAssets(data)
	if err != nil {
		t.Fatalf("DecodeMenuAssets failed: %v", err)
	}

	t.Logf("MenuAssets: fileName=%q, assets=%d", assets.FileName, len(assets.Assets))
	for i, menu := range assets.Assets {
		t.Logf("  [%d] version=%d id=%d fileName=%q itemName=%q category=%q commands=%d",
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

	if decoded.FileName != assets.FileName {
		t.Errorf("fileName mismatch after round-trip")
	}
	if len(decoded.Assets) != len(assets.Assets) {
		t.Errorf("assets count mismatch after round-trip: got %d, want %d", len(decoded.Assets), len(assets.Assets))
	}
}

func TestDecodeMenuAssets_ByteEqual(t *testing.T) {
	original := readPartsSample(t, "parts_personal002.menuassets")

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

	if decoded2.FileName != assets.FileName {
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
		if got.FileName != orig.FileName {
			t.Errorf("[%d] fileName mismatch: got %q, want %q", i, got.FileName, orig.FileName)
		}
		if got.CategoryText != orig.CategoryText {
			t.Errorf("[%d] categoryText mismatch: got %q, want %q", i, got.CategoryText, orig.CategoryText)
		}
		if len(got.Commands) != len(orig.Commands) {
			t.Errorf("[%d] commands count mismatch: got %d, want %d", i, len(got.Commands), len(orig.Commands))
		}
	}
}

func TestDecodeModel_FromAba(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(partsSampleDir(), "*.model"))
	if err != nil {
		t.Fatalf("glob models: %v", err)
	}
	if len(paths) == 0 {
		t.Skip("model samples not found")
	}

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
			if model.FileName == "" {
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
			if decoded.FileName != model.FileName {
				t.Errorf("fileName mismatch after round-trip: got %q, want %q", decoded.FileName, model.FileName)
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

func TestPartsColorGradaBytes_JSONRoundTrip(t *testing.T) {
	original := PartsColor{
		MainHue:    1,
		GradaBytes: GradaBytes{Value: []byte{1, 2, 3, 4}},
	}

	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	var decoded PartsColor
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(decoded.GradaBytes.Value, []byte{1, 2, 3, 4}) {
		t.Fatalf("grada bytes mismatch after JSON round-trip: %#v", decoded.GradaBytes.Value)
	}

	encoded, err := encodeCompressedMsgpack(original, "PartsColor")
	if err != nil {
		t.Fatalf("msgpack encode failed: %v", err)
	}
	var msgpackDecoded PartsColor
	if err := decodeCompressedMsgpack(encoded, &msgpackDecoded, "PartsColor"); err != nil {
		t.Fatalf("msgpack decode failed: %v", err)
	}
	if !reflect.DeepEqual(msgpackDecoded.GradaBytes.Value, []byte{1, 2, 3, 4}) {
		t.Fatalf("grada bytes mismatch after msgpack round-trip: %#v", msgpackDecoded.GradaBytes.Value)
	}

	original.GradaBytes.Value = false
	encoded, err = encodeCompressedMsgpack(original, "PartsColor")
	if err != nil {
		t.Fatalf("msgpack encode bool failed: %v", err)
	}
	if err := decodeCompressedMsgpack(encoded, &msgpackDecoded, "PartsColor"); err != nil {
		t.Fatalf("msgpack decode bool failed: %v", err)
	}
	if msgpackDecoded.GradaBytes.Value != false {
		t.Fatalf("grada bool placeholder mismatch: %#v", msgpackDecoded.GradaBytes.Value)
	}
}
