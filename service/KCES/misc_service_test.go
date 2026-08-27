package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/kcesfixtures"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestMiscService_HitCheckRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample.hitcheck")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out.hitcheck")

	hitCheck := serializationKCES.NewHitCheck()
	hitCheck.Entries = []serializationKCES.HitCheckEntry{
		{
			Type:      1,
			Radius:    0.5,
			RadiusSqr: 0.25,
			Name:      "Sphere",
			Parent:    "Bip01 Head",
			Position:  serializationKCES.Vector3{X: 1, Y: 2, Z: 3},
			SKRT:      0,
			RL:        1,
		},
	}
	encoded, err := serializationKCES.EncodeHitCheck(hitCheck)
	if err != nil {
		t.Fatalf("EncodeHitCheck: %v", err)
	}
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	service := &MiscService{}
	if err := service.ConvertMiscToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertMiscToJson: %v", err)
	}
	if err := service.ConvertJsonToMisc(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToMisc: %v", err)
	}
	decoded, err := serializationKCES.DecodeHitCheck(mustReadTestFile(t, outputPath))
	if err != nil {
		t.Fatalf("DecodeHitCheck output: %v", err)
	}
	if len(decoded.Entries) != 1 || decoded.Entries[0].Parent != "Bip01 Head" || decoded.Entries[0].Type != 1 {
		t.Fatalf("unexpected decoded hitcheck: %+v", decoded)
	}
}

func TestMiscService_UndressDataRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample.undressdat")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out.undressdat")

	// WearSetuper 用 JsonUtility.FromJson<ArchiveTarget> 读取 .undressdat，因此这里保留真实的
	// 游戏侧成员名，包括必须与缺失区分开的空数组
	// WearSetuper reads .undressdat with JsonUtility.FromJson<ArchiveTarget>, so keep the real
	// game-side member names here, including the empty arrays that must stay distinct from absent ones
	input := []byte("{\n    \"format\": \"1.2.2\",\n    \"editVer\": 13,\n    \"dataGroup\": [\n        {\n            \"label\": \"Group_0000\",\n            \"layer\": 0,\n            \"weights\": [],\n            \"indices\": [7, 11]\n        }\n    ]\n}\n")
	if err := os.WriteFile(inputPath, input, 0644); err != nil {
		t.Fatal(err)
	}

	service := &MiscService{}
	if err := service.ConvertMiscToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertMiscToJson: %v", err)
	}
	editing, err := serializationKCES.DecodeKCESUndressData(mustReadTestFile(t, jsonPath))
	if err != nil {
		t.Fatalf("decode editing document: %v", err)
	}
	if editing.Format == nil || *editing.Format != "1.2.2" || editing.DataGroup == nil || len(*editing.DataGroup) != 1 {
		t.Fatalf("unexpected editing document: %+v", editing)
	}

	if err := service.ConvertJsonToMisc(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToMisc: %v", err)
	}
	// 编辑 JSON 的根就是资源文档本身，因此原生输出与输入的成员集合完全一致
	// The editing JSON root is the resource document itself, so the native output carries exactly the input member set
	if got := string(mustReadTestFile(t, outputPath)); got != "{\n    \"format\": \"1.2.2\",\n    \"editVer\": 13,\n    \"dataGroup\": [\n        {\n            \"label\": \"Group_0000\",\n            \"weights\": [],\n            \"layer\": 0,\n            \"indices\": [\n                7,\n                11\n            ]\n        }\n    ]\n}" {
		t.Fatalf("unexpected native output: %q", got)
	}
}

func TestMiscService_NSONRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "dance_enabled_list.nson")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out.nson")

	// CsvManager.CreateCsvIdManager feeds .nson TextAssets to
	// JsonUtility.FromJson<CsvIdManager>. Keep the real game-side field names in
	// this probe while preserving support for other JSON-shaped .nson TextAssets.
	input := []byte(`{"version":1000,"_ids":[1,2,2147483647]}`)
	if err := os.WriteFile(inputPath, input, 0644); err != nil {
		t.Fatal(err)
	}

	service := &MiscService{}
	if err := service.ConvertMiscToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertMiscToJson: %v", err)
	}
	var document json.RawMessage
	if err := json.Unmarshal(mustReadTestFile(t, jsonPath), &document); err != nil {
		t.Fatalf("unmarshal editing document: %v", err)
	}
	var gotDocument, wantDocument interface{}
	if err := json.Unmarshal(document, &gotDocument); err != nil {
		t.Fatalf("unmarshal editing document body: %v", err)
	}
	if err := json.Unmarshal(input, &wantDocument); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotDocument, wantDocument) {
		t.Fatalf("unexpected NSON editing document: %s", document)
	}

	if err := service.ConvertJsonToMisc(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToMisc: %v", err)
	}
	decoded, err := serializationKCES.DecodeKCESJSONText(mustReadTestFile(t, outputPath), ".nson")
	if err != nil {
		t.Fatalf("DecodeKCESJSONText output: %v", err)
	}
	if string(decoded) != string(input) {
		t.Fatalf("unexpected NSON document: %s", decoded)
	}
}

func TestMiscService_FixedSamplesJSONRoundTrip(t *testing.T) {
	pathsByExt := fixedMiscServiceSamplesByExt(t)
	service := &MiscService{}
	for ext, paths := range pathsByExt {
		ext := ext
		paths := paths
		t.Run(ext, func(t *testing.T) {
			for _, sample := range paths {
				sample := sample
				t.Run(filepath.Base(sample), func(t *testing.T) {
					name := filepath.Base(sample)
					tmpDir := t.TempDir()
					jsonPath := filepath.Join(tmpDir, name+".json")
					outPath := filepath.Join(tmpDir, name)
					if err := service.ConvertMiscToJson(TestConversionContext, sample, jsonPath, TestConversionMaxOutput); err != nil {
						t.Fatalf("ConvertMiscToJson: %v", err)
					}
					if err := service.ConvertJsonToMisc(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
						t.Fatalf("ConvertJsonToMisc: %v", err)
					}
					want, err := service.ReadMiscFile(sample)
					if err != nil {
						t.Fatalf("ReadMiscFile sample: %v", err)
					}
					got, err := service.ReadMiscFile(outPath)
					if err != nil {
						t.Fatalf("ReadMiscFile output: %v", err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("service misc JSON round-trip changed %s: got %#v, want %#v", name, got, want)
					}
				})
			}
		})
	}
}

func fixedMiscServiceSamplesByExt(t *testing.T) map[string][]string {
	t.Helper()
	paths := kcesfixtures.MiscSamplePaths(t)
	pathsByExt := map[string][]string{}
	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".hitcheck", ".undressdat", ".undresspdat":
			pathsByExt[ext] = append(pathsByExt[ext], path)
		default:
			t.Fatalf("unexpected fixed misc sample suffix %q for %s", ext, filepath.Base(path))
		}
	}
	for _, ext := range []string{".hitcheck", ".undressdat", ".undresspdat"} {
		if len(pathsByExt[ext]) == 0 {
			t.Fatalf("no fixed misc samples with suffix %s", ext)
		}
	}
	return pathsByExt
}

func mustReadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
