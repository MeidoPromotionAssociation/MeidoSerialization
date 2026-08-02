package KCES

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestPartsService_MenuAssetsJSONRoundTrip(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "kces_parts", "parts_personal002.menuassets")

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "parts_personal002.menuassets.json")
	outPath := filepath.Join(tmpDir, "parts_personal002.menuassets")

	service := &PartsService{}
	if err := service.ConvertPartsToJson(TestConversionContext, sample, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertPartsToJson failed: %v", err)
	}
	if err := service.ConvertJsonToParts(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
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
	if testStringValue(assets.FileName) != "parts_personal002.menuassets" {
		t.Errorf("fileName: got %q", testStringValue(assets.FileName))
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
	if err := service.ConvertPartsToJson(TestConversionContext, sample, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertPartsToJson failed: %v", err)
	}
	if err := service.ConvertJsonToParts(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
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
	if testStringValue(model.FileName) != "hair_twin019.model" {
		t.Errorf("fileName: got %q", testStringValue(model.FileName))
	}
	if len(model.TransData) == 0 {
		t.Errorf("transData is empty")
	}
}

func TestPartsServiceWritersRecalculateLookupFields(t *testing.T) {
	dir := t.TempDir()
	menuFileName := "MixedCase.KCMENU"
	exportedGUID := "ABCDEF01-2345-6789-ABCD-EF0123456789"
	menu := serializationKCES.NewKCES2Menu()
	menu.FileName = &menuFileName
	menu.ID = 1
	menu.GUID = 2
	menu.HairMake = serializationKCES.NewHairMake()
	menu.HairMake.ExportedGUID = &exportedGUID
	menuPath := filepath.Join(dir, "parts.menuassets")
	if err := (&MenuAssetsService{}).WriteMenuAssetsFile(menuPath, &serializationKCES.MenuAssets{Assets: []*serializationKCES.Menu{menu}}); err != nil {
		t.Fatalf("WriteMenuAssetsFile: %v", err)
	}
	menuWire, err := os.ReadFile(menuPath)
	if err != nil {
		t.Fatal(err)
	}
	menuOutput, err := serializationKCES.DecodeMenuAssets(menuWire)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := menuOutput.Assets[0].ID, ct.HashStringIgnoreCase(menuFileName); got != want {
		t.Fatalf("menu ID = %d, want %d", got, want)
	}
	if got, want := menuOutput.Assets[0].GUID, ct.HashStringIgnoreCase(exportedGUID); got != want {
		t.Fatalf("menu GUID = %d, want %d", got, want)
	}
	if menu.ID != 1 || menu.GUID != 2 {
		t.Fatalf("WriteMenuAssetsFile mutated input IDs: ID=%d GUID=%d", menu.ID, menu.GUID)
	}

	materialFileName := "MixedCase.Mate"
	material := serializationKCES.NewKCES2Material()
	material.FileName = &materialFileName
	material.ID = 1
	materialPath := filepath.Join(dir, "parts.materialassets")
	if err := (&MaterialAssetsService{}).WriteMaterialAssetsFile(materialPath, &serializationKCES.MaterialAssets{Assets: []*serializationKCES.Material{material}}); err != nil {
		t.Fatalf("WriteMaterialAssetsFile: %v", err)
	}
	materialWire, err := os.ReadFile(materialPath)
	if err != nil {
		t.Fatal(err)
	}
	materialOutput, err := serializationKCES.DecodeMaterialAssets(materialWire)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := materialOutput.Assets[0].ID, ct.HashString(materialFileName); got != want {
		t.Fatalf("material ID = %d, want %d", got, want)
	}
	if material.ID != 1 {
		t.Fatalf("WriteMaterialAssetsFile mutated input ID: %d", material.ID)
	}

	modelFileName := "stale.model"
	model := &serializationKCES.Model{FileName: &modelFileName, ID: 1}
	modelPath := filepath.Join(dir, "written.model")
	if err := (&ModelService{}).WriteModelFile(modelPath, model); err != nil {
		t.Fatalf("WriteModelFile: %v", err)
	}
	modelWire, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	modelOutput, err := serializationKCES.DecodeModel(modelWire)
	if err != nil {
		t.Fatal(err)
	}
	writtenFileName := filepath.Base(modelPath)
	if modelOutput.FileName == nil || *modelOutput.FileName != writtenFileName {
		t.Fatalf("model fileName = %v, want %q", modelOutput.FileName, writtenFileName)
	}
	if got, want := modelOutput.ID, ct.HashString(writtenFileName); got != want {
		t.Fatalf("model ID = %d, want %d", got, want)
	}
	if model.ID != 1 || model.FileName == nil || *model.FileName != modelFileName {
		t.Fatalf("WriteModelFile mutated input: %+v", model)
	}
}

func TestPartsService_PriorityMaterialAssetsJSONRoundTrip(t *testing.T) {
	assets := &serializationKCES.PriorityMaterialAssets{
		FileName: testStringPointer("test.pmatassets"),
		Assets: []*serializationKCES.PriorityMaterial{
			{
				Version:     1000,
				ID:          12345,
				FileName:    testStringPointer("test.pmat"),
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
	if err := service.ConvertPartsToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertPartsToJson failed: %v", err)
	}
	if err := service.ConvertJsonToParts(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
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
	if testStringValue(decoded.FileName) != testStringValue(assets.FileName) {
		t.Errorf("fileName: got %q, want %q", testStringValue(decoded.FileName), testStringValue(assets.FileName))
	}
	if len(decoded.Assets) != 1 {
		t.Fatalf("asset count: got %d, want 1", len(decoded.Assets))
	}
	if decoded.Assets[0] == nil || testStringValue(decoded.Assets[0].FileName) != "test.pmat" {
		t.Errorf("asset fileName: got %q", testStringValue(decoded.Assets[0].FileName))
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
					if err := service.ConvertPartsToJson(TestConversionContext, sample, jsonPath, TestConversionMaxOutput); err != nil {
						t.Fatalf("ConvertPartsToJson: %v", err)
					}
					if err := service.ConvertJsonToParts(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
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
					want, err = canonicalizeExpectedParts(want)
					if err != nil {
						t.Fatalf("canonicalize expected parts: %v", err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("service parts JSON round-trip changed %s: got %#v, want %#v", name, got, want)
					}
				})
			}
		})
	}
}

func TestPartsService_RootNullRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		ext    string
		decode func([]byte) (any, error)
	}{
		{name: "menuassets", ext: ".menuassets", decode: func(data []byte) (any, error) {
			return serializationKCES.DecodeMenuAssets(data)
		}},
		{name: "materialassets", ext: ".materialassets", decode: func(data []byte) (any, error) {
			return serializationKCES.DecodeMaterialAssets(data)
		}},
		{name: "pmatassets", ext: ".pmatassets", decode: func(data []byte) (any, error) {
			return serializationKCES.DecodePriorityMaterialAssets(data)
		}},
		{name: "model", ext: ".model", decode: func(data []byte) (any, error) {
			return serializationKCES.DecodeModel(data)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := encodePartsJSON(test.ext, []byte("null"))
			if err != nil {
				t.Fatalf("encodePartsJSON: %v", err)
			}
			decoded, err := test.decode(encoded)
			if err != nil {
				t.Fatalf("decode encoded root nil: %v", err)
			}
			if value := reflect.ValueOf(decoded); !value.IsValid() || value.Kind() != reflect.Pointer || !value.IsNil() {
				t.Fatalf("decoded root = %#v, want nil", decoded)
			}
		})
	}
}

func TestIsKCESPartsJSONFileAcceptsNullableModelRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nullable.model.json")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbf  null\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESPartsJSONFile(path) {
		t.Fatal("nullable KCES model JSON was not detected")
	}
}

// Compare the service's JSON route against a direct encoder route so both
// sides undergo the same known-field MessagePack canonicalization. Neither
// route invokes the game's version callback or rewrites stored versions.
func canonicalizeExpectedParts(value interface{}) (interface{}, error) {
	switch typed := value.(type) {
	case *serializationKCES.MenuAssets:
		encoded, err := serializationKCES.EncodeMenuAssets(typed)
		if err != nil {
			return nil, err
		}
		return serializationKCES.DecodeMenuAssets(encoded)
	case *serializationKCES.MaterialAssets:
		encoded, err := serializationKCES.EncodeMaterialAssets(typed)
		if err != nil {
			return nil, err
		}
		return serializationKCES.DecodeMaterialAssets(encoded)
	case *serializationKCES.PriorityMaterialAssets:
		encoded, err := serializationKCES.EncodePriorityMaterialAssets(typed)
		if err != nil {
			return nil, err
		}
		return serializationKCES.DecodePriorityMaterialAssets(encoded)
	case *serializationKCES.Model:
		encoded, err := serializationKCES.EncodeModel(typed)
		if err != nil {
			return nil, err
		}
		return serializationKCES.DecodeModel(encoded)
	default:
		return value, nil
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
