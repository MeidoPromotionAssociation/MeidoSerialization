package KCES

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestPartsService_MenuAssetsJSONRoundTrip(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "kces_parts", "parts_personal002.menuassets")

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "parts_personal002.menuassets.json")
	outPath := filepath.Join(tmpDir, "parts_personal002.menuassets")

	service := &PartsService{}
	if err := service.ConvertPartsToJson(sample, jsonPath); err != nil {
		t.Fatalf("ConvertPartsToJson failed: %v", err)
	}
	if err := service.ConvertJsonToParts(jsonPath, outPath); err != nil {
		t.Fatalf("ConvertJsonToParts failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	assets, err := serializationKCES.DecodeMenuAssets(data)
	if err != nil {
		t.Fatalf("DecodeMenuAssets output failed: %v", err)
	}
	if assets.FileName != "parts_personal002.menuassets" {
		t.Errorf("fileName: got %q", assets.FileName)
	}
	if len(assets.Assets) != 4 {
		t.Errorf("asset count: got %d, want 4", len(assets.Assets))
	}
}

func TestPartsService_ModelJSONRoundTrip(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "kces_parts", "hair_twin019.model")

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "hair_twin019.model.json")
	outPath := filepath.Join(tmpDir, "hair_twin019.model")

	service := &PartsService{}
	if err := service.ConvertPartsToJson(sample, jsonPath); err != nil {
		t.Fatalf("ConvertPartsToJson failed: %v", err)
	}
	if err := service.ConvertJsonToParts(jsonPath, outPath); err != nil {
		t.Fatalf("ConvertJsonToParts failed: %v", err)
	}

	if !IsKCESModelFile(outPath) {
		t.Fatalf("output is not detected as KCES model")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	model, err := serializationKCES.DecodeModel(data)
	if err != nil {
		t.Fatalf("DecodeModel output failed: %v", err)
	}
	if model.FileName != "hair_twin019.model" {
		t.Errorf("fileName: got %q", model.FileName)
	}
	if len(model.TransData) == 0 {
		t.Errorf("transData is empty")
	}
}

func TestPartsService_PriorityMaterialAssetsJSONRoundTrip(t *testing.T) {
	assets := &serializationKCES.PriorityMaterialAssets{
		FileName: "test.pmatassets",
		Assets: []serializationKCES.PriorityMaterial{
			{
				Version:     1000,
				ID:          12345,
				FileName:    "test.pmat",
				RenderQueue: 3000,
				TargetID:    67890,
			},
		},
	}
	data, err := serializationKCES.EncodePriorityMaterialAssets(assets)
	if err != nil {
		t.Fatalf("EncodePriorityMaterialAssets failed: %v", err)
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.pmatassets")
	jsonPath := filepath.Join(tmpDir, "test.pmatassets.json")
	outPath := filepath.Join(tmpDir, "out.pmatassets")
	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	service := &PartsService{}
	if err := service.ConvertPartsToJson(inputPath, jsonPath); err != nil {
		t.Fatalf("ConvertPartsToJson failed: %v", err)
	}
	if err := service.ConvertJsonToParts(jsonPath, outPath); err != nil {
		t.Fatalf("ConvertJsonToParts failed: %v", err)
	}

	encoded, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	decoded, err := serializationKCES.DecodePriorityMaterialAssets(encoded)
	if err != nil {
		t.Fatalf("DecodePriorityMaterialAssets output failed: %v", err)
	}
	if decoded.FileName != assets.FileName {
		t.Errorf("fileName: got %q, want %q", decoded.FileName, assets.FileName)
	}
	if len(decoded.Assets) != 1 {
		t.Fatalf("asset count: got %d, want 1", len(decoded.Assets))
	}
	if decoded.Assets[0].FileName != "test.pmat" {
		t.Errorf("asset fileName: got %q", decoded.Assets[0].FileName)
	}
}

func TestPartsService_FixedSamplesJSONRoundTrip(t *testing.T) {
	pathsByExt := fixedPartsServiceSamplesByExt(t)
	service := &PartsService{}
	for ext, paths := range pathsByExt {
		ext := ext
		paths := paths
		t.Run(ext, func(t *testing.T) {
			for _, sample := range paths {
				sample := sample
				t.Run(filepath.Base(sample), func(t *testing.T) {
					tmpDir := t.TempDir()
					name := filepath.Base(sample)
					jsonPath := filepath.Join(tmpDir, name+".json")
					outPath := filepath.Join(tmpDir, name)
					if err := service.ConvertPartsToJson(sample, jsonPath); err != nil {
						t.Fatalf("ConvertPartsToJson: %v", err)
					}
					if err := service.ConvertJsonToParts(jsonPath, outPath); err != nil {
						t.Fatalf("ConvertJsonToParts: %v", err)
					}
					want, err := service.ReadPartsFile(sample)
					if err != nil {
						t.Fatalf("ReadPartsFile sample: %v", err)
					}
					got, err := service.ReadPartsFile(outPath)
					if err != nil {
						t.Fatalf("ReadPartsFile output: %v", err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("service parts JSON round-trip changed %s: got %#v, want %#v", name, got, want)
					}
				})
			}
		})
	}
}

func fixedPartsServiceSamplesByExt(t *testing.T) map[string][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "kces_parts", "*"))
	if err != nil {
		t.Fatalf("glob fixed parts samples: %v", err)
	}
	if len(paths) == 0 {
		t.Skip("no fixed parts samples found")
	}
	pathsByExt := map[string][]string{}
	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".menuassets", ".materialassets", ".model", ".pmatassets":
			pathsByExt[ext] = append(pathsByExt[ext], path)
		default:
			t.Fatalf("unexpected fixed parts sample suffix %q for %s", ext, filepath.Base(path))
		}
	}
	for _, ext := range []string{".menuassets", ".materialassets", ".model", ".pmatassets"} {
		if len(pathsByExt[ext]) == 0 {
			t.Fatalf("no fixed parts samples with suffix %s", ext)
		}
	}
	return pathsByExt
}
