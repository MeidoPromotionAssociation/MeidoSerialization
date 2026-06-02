package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestMiscService_HitCheckRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample.hitcheck")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out.hitcheck")

	zero := int32(0)
	encoded, err := serializationKCES.EncodeHitCheck(&serializationKCES.HitCheck{
		Header: 1,
		Entries: []serializationKCES.HitCheckEntry{
			{
				Radius:     0.5,
				RadiusSqr:  0.25,
				ShapeName:  "Sphere",
				BoneName:   "Bip01 Head",
				Position:   serializationKCES.Vector3{X: 1, Y: 2, Z: 3},
				TargetType: 0,
				Side:       1,
				Tail:       &zero,
			},
		},
	})
	if err != nil {
		t.Fatalf("EncodeHitCheck: %v", err)
	}
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	service := &MiscService{}
	if err := service.ConvertMiscToJson(inputPath, jsonPath); err != nil {
		t.Fatalf("ConvertMiscToJson: %v", err)
	}
	if err := service.ConvertJsonToMisc(jsonPath, outputPath); err != nil {
		t.Fatalf("ConvertJsonToMisc: %v", err)
	}
	decoded, err := serializationKCES.DecodeHitCheck(mustReadTestFile(t, outputPath))
	if err != nil {
		t.Fatalf("DecodeHitCheck output: %v", err)
	}
	if decoded.Header != 1 || len(decoded.Entries) != 1 || decoded.Entries[0].BoneName != "Bip01 Head" {
		t.Fatalf("unexpected decoded hitcheck: %+v", decoded)
	}
}

func TestMiscService_JSONTextRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample.undressdat")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out.undressdat")

	input := []byte("{\n  \"editVer\": 13,\n  \"items\": [\"a\", \"b\"]\n}\n")
	if err := os.WriteFile(inputPath, input, 0644); err != nil {
		t.Fatal(err)
	}

	service := &MiscService{}
	if err := service.ConvertMiscToJson(inputPath, jsonPath); err != nil {
		t.Fatalf("ConvertMiscToJson: %v", err)
	}
	var envelope serializationKCES.KCESJSONText
	if err := json.Unmarshal(mustReadTestFile(t, jsonPath), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.Extension != ".undressdat" || len(envelope.JSON) == 0 {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}

	if err := service.ConvertJsonToMisc(jsonPath, outputPath); err != nil {
		t.Fatalf("ConvertJsonToMisc: %v", err)
	}
	decoded, err := serializationKCES.DecodeKCESJSONText(mustReadTestFile(t, outputPath), ".undressdat")
	if err != nil {
		t.Fatalf("DecodeKCESJSONText output: %v", err)
	}
	if string(decoded.JSON) != `{"editVer":13,"items":["a","b"]}` {
		t.Fatalf("unexpected JSON payload: %s", decoded.JSON)
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
					if err := service.ConvertMiscToJson(sample, jsonPath); err != nil {
						t.Fatalf("ConvertMiscToJson: %v", err)
					}
					if err := service.ConvertJsonToMisc(jsonPath, outPath); err != nil {
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
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "kces_misc", "*"))
	if err != nil {
		t.Fatalf("glob fixed misc samples: %v", err)
	}
	if len(paths) == 0 {
		t.Skip("no fixed misc samples found")
	}
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
