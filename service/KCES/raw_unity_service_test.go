package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

func TestRawUnityObjectService_FixedSamplesJSONRoundTrip(t *testing.T) {
	paths := rawUnityBytesSamples(t)
	service := &RawUnityObjectService{}
	for _, inputPath := range paths {
		inputPath := inputPath
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			raw, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("read fixed raw Unity sample: %v", err)
			}
			wantKind, wantClassID, ok := rawUnitySampleKindFromSuffix(inputPath)
			if !ok {
				t.Fatalf("no raw Unity suffix expectation for %s", filepath.Base(inputPath))
			}

			envelope, err := service.ReadRawUnityObjectFile(inputPath)
			if err != nil {
				t.Fatalf("ReadRawUnityObjectFile: %v", err)
			}
			if envelope.Format != RawUnityObjectFormat {
				t.Fatalf("format got %q, want %q", envelope.Format, RawUnityObjectFormat)
			}
			if envelope.ClassID != wantClassID || envelope.TypeName != unityClassName(wantClassID) || envelope.Kind != wantKind {
				t.Fatalf("unexpected raw Unity classification: got %+v, want class=%d type=%q kind=%q", envelope, wantClassID, unityClassName(wantClassID), wantKind)
			}
			if envelope.PathID == 0 {
				t.Fatalf("expected fixed sidecar PathID")
			}
			meta := readAssetMeta(inputPath)
			if envelope.PathID != meta.PathID || envelope.LoadName != meta.LoadName {
				t.Fatalf("envelope meta got pathId=%d loadName=%q, want pathId=%d loadName=%q", envelope.PathID, envelope.LoadName, meta.PathID, meta.LoadName)
			}
			decoded, err := base64.StdEncoding.DecodeString(envelope.DataBase64)
			if err != nil {
				t.Fatalf("decode dataBase64: %v", err)
			}
			if !bytes.Equal(decoded, raw) {
				t.Fatalf("envelope bytes changed")
			}

			tmpDir := t.TempDir()
			jsonPath := filepath.Join(tmpDir, filepath.Base(inputPath)+".json")
			outPath := filepath.Join(tmpDir, filepath.Base(inputPath))
			if err := service.ConvertRawUnityObjectToJson(inputPath, jsonPath); err != nil {
				t.Fatalf("ConvertRawUnityObjectToJson: %v", err)
			}
			if !IsKCESRawUnityBytesJSONFile(jsonPath) {
				t.Fatalf("converted JSON was not detected as raw Unity JSON")
			}
			var jsonEnvelope RawUnityObjectEnvelope
			if err := json.Unmarshal(mustReadServiceFile(t, jsonPath), &jsonEnvelope); err != nil {
				t.Fatalf("decode raw Unity JSON: %v", err)
			}
			if !reflect.DeepEqual(&jsonEnvelope, envelope) {
				t.Fatalf("JSON envelope changed: got %#v, want %#v", &jsonEnvelope, envelope)
			}
			if err := service.ConvertJsonToRawUnityObject(jsonPath, outPath); err != nil {
				t.Fatalf("ConvertJsonToRawUnityObject: %v", err)
			}
			roundTrip, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("read round-trip output: %v", err)
			}
			if !bytes.Equal(roundTrip, raw) {
				t.Fatalf("round-trip bytes changed")
			}
			roundTripMeta := readAssetMeta(outPath)
			if roundTripMeta.PathID != envelope.PathID || roundTripMeta.LoadName != envelope.LoadName {
				t.Fatalf("round-trip meta got %+v, want pathId=%d loadName=%q", roundTripMeta, envelope.PathID, envelope.LoadName)
			}
			roundTripEnvelope, err := service.ReadRawUnityObjectFile(outPath)
			if err != nil {
				t.Fatalf("ReadRawUnityObjectFile round-trip: %v", err)
			}
			if !reflect.DeepEqual(roundTripEnvelope, envelope) {
				t.Fatalf("round-trip envelope changed: got %#v, want %#v", roundTripEnvelope, envelope)
			}
		})
	}
}

func rawUnityBytesSamples(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "kces_assets", "*.bytes"))
	if err != nil {
		t.Fatalf("glob raw Unity samples: %v", err)
	}
	if len(paths) == 0 {
		t.Skip("no fixed raw Unity .bytes samples found")
	}
	for _, path := range paths {
		if !IsKCESRawUnityBytesFile(path) {
			t.Fatalf("fixed .bytes sample was not routed as raw Unity: %s", filepath.Base(path))
		}
		if _, err := os.Stat(assetMetaPath(path)); err != nil {
			t.Fatalf("fixed .bytes sample is missing meta sidecar %s: %v", assetMetaPath(path), err)
		}
	}
	return paths
}

func rawUnitySampleKindFromSuffix(path string) (kind string, classID int32, ok bool) {
	lower := strings.ToLower(filepath.Base(path))
	for _, candidate := range []struct {
		suffix string
		kind   string
		class  int32
	}{
		{".texture2d.bytes", "rawtexture2d", aba.ClassIDTexture2D},
		{".texture.bytes", "rawtexture2d", aba.ClassIDTexture2D},
		{".tex.bytes", "rawtexture2d", aba.ClassIDTexture2D},
		{".sprite.bytes", "sprite", aba.ClassIDSprite},
		{".mmesh.bytes", "mesh", aba.ClassIDMesh},
		{".partsatlas.bytes", "spriteatlas", aba.ClassIDSpriteAtlas},
		{".partsassets.bytes", "spriteatlas", aba.ClassIDSpriteAtlas},
		{".anm.bytes", "animationclip", aba.ClassIDAnimationClip},
		{".monoscript.bytes", "monoscript", aba.ClassIDMonoScript},
		{".monobehaviour.bytes", "monobehaviour", aba.ClassIDMonoBehaviour},
		{".material.bytes", "material", aba.ClassIDMaterial},
		{".shader.bytes", "shader", aba.ClassIDShader},
		{".audioclip.bytes", "audioclip", aba.ClassIDAudioClip},
		{".font.bytes", "font", aba.ClassIDFont},
	} {
		if strings.HasSuffix(lower, candidate.suffix) {
			return candidate.kind, candidate.class, true
		}
	}
	return "", 0, false
}

func TestRawUnityObjectService_JSONRoundTripPreservesTypeTreeSidecar(t *testing.T) {
	sourceBundle := filepath.Join("..", "..", "testdata", "aba", "cm3d2_megane002.aba")
	if _, err := os.Stat(sourceBundle); err != nil {
		t.Skipf("sample not found: %v", err)
	}

	tmpDir := t.TempDir()
	unpackDir := filepath.Join(tmpDir, "unpacked")
	abaService := &AbaService{}
	if err := abaService.UnpackAba(sourceBundle, unpackDir); err != nil {
		t.Fatalf("UnpackAba: %v", err)
	}

	inputPath := filepath.Join(unpackDir, "Texture2D", "cm3d2_megane002.tex.bytes")
	if _, err := os.Stat(typeTreeSidecarPath(inputPath)); err != nil {
		t.Fatalf("expected TypeTree sidecar: %v", err)
	}

	service := &RawUnityObjectService{}
	jsonPath := filepath.Join(tmpDir, "cm3d2_megane002.tex.bytes.json")
	if err := service.ConvertRawUnityObjectToJson(inputPath, jsonPath); err != nil {
		t.Fatalf("ConvertRawUnityObjectToJson: %v", err)
	}

	var envelope RawUnityObjectEnvelope
	if err := json.Unmarshal(mustReadServiceFile(t, jsonPath), &envelope); err != nil {
		t.Fatalf("decode raw Unity JSON: %v", err)
	}
	if envelope.TypeTree == nil || envelope.TypeTree.Format != RawUnityTypeTreeFormat || envelope.TypeTree.Value == nil {
		t.Fatalf("missing TypeTree in raw Unity envelope: %+v", envelope.TypeTree)
	}

	outPath := filepath.Join(tmpDir, "roundtrip.tex.bytes")
	if err := service.ConvertJsonToRawUnityObject(jsonPath, outPath); err != nil {
		t.Fatalf("ConvertJsonToRawUnityObject: %v", err)
	}
	restored, err := readRawUnityTypeTreeSidecar(outPath)
	if err != nil {
		t.Fatalf("read restored TypeTree sidecar: %v", err)
	}
	if restored.Format != RawUnityTypeTreeFormat || restored.ClassID != aba.ClassIDTexture2D || restored.Value == nil {
		t.Fatalf("restored TypeTree sidecar incomplete: %+v", restored)
	}
}

func TestRawUnityObjectService_TypeDirectorySampleJSONRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	typeDir := filepath.Join(tmpDir, "Type_95")
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		t.Fatal(err)
	}

	inputPath := filepath.Join(typeDir, "type95_internal.bytes")
	raw := []byte{1, 0, 0, 0, 95, 0, 0, 0, 2, 0, 0, 0}
	if err := os.WriteFile(inputPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeAssetMeta(inputPath, -95, "type95_internal"); err != nil {
		t.Fatal(err)
	}

	service := &RawUnityObjectService{}
	envelope, err := service.ReadRawUnityObjectFile(inputPath)
	if err != nil {
		t.Fatalf("ReadRawUnityObjectFile: %v", err)
	}
	if envelope.ClassID != 95 || envelope.TypeName != "Type_95" || envelope.Kind != "type_95" {
		t.Fatalf("unexpected Type_95 envelope: %+v", envelope)
	}
	if envelope.PathID != -95 || envelope.LoadName != "type95_internal" {
		t.Fatalf("unexpected Type_95 meta: %+v", envelope)
	}

	jsonPath := inputPath + ".json"
	if err := service.ConvertRawUnityObjectToJson(inputPath, jsonPath); err != nil {
		t.Fatalf("ConvertRawUnityObjectToJson: %v", err)
	}
	var decoded RawUnityObjectEnvelope
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("decode raw Unity JSON: %v", err)
	}
	if decoded.ClassID != 95 || decoded.Kind != "type_95" {
		t.Fatalf("decoded Type_95 JSON got %+v", decoded)
	}
}

func TestRawUnityObjectService_TypeDirectoryKnownNativeKinds(t *testing.T) {
	tmpDir := t.TempDir()
	raw := []byte{4, 0, 0, 0, 'n', 'a', 'm', 'e', 0, 0, 0, 0}
	samples := []struct {
		dir      string
		classID  int32
		typeName string
		kind     string
	}{
		{"GameObject", aba.ClassIDGameObject, "GameObject", "gameobject"},
		{"Transform", aba.ClassIDTransform, "Transform", "transform"},
		{"Material", aba.ClassIDMaterial, "Material", "material"},
		{"MeshRenderer", aba.ClassIDMeshRenderer, "MeshRenderer", "meshrenderer"},
		{"MeshFilter", aba.ClassIDMeshFilter, "MeshFilter", "meshfilter"},
		{"Shader", aba.ClassIDShader, "Shader", "shader"},
		{"AudioClip", aba.ClassIDAudioClip, "AudioClip", "audioclip"},
		{"MonoBehaviour", aba.ClassIDMonoBehaviour, "MonoBehaviour", "monobehaviour"},
		{"MonoScript", aba.ClassIDMonoScript, "MonoScript", "monoscript"},
		{"Font", aba.ClassIDFont, "Font", "font"},
	}

	service := &RawUnityObjectService{}
	for i, sample := range samples {
		sample := sample
		t.Run(sample.dir, func(t *testing.T) {
			dir := filepath.Join(tmpDir, sample.dir)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			inputPath := filepath.Join(dir, "sample.bytes")
			if err := os.WriteFile(inputPath, raw, 0644); err != nil {
				t.Fatal(err)
			}
			if err := writeAssetMeta(inputPath, int64(-3000-i), sample.dir); err != nil {
				t.Fatal(err)
			}

			envelope, err := service.ReadRawUnityObjectFile(inputPath)
			if err != nil {
				t.Fatalf("ReadRawUnityObjectFile: %v", err)
			}
			if envelope.ClassID != sample.classID || envelope.TypeName != sample.typeName || envelope.Kind != sample.kind {
				t.Fatalf("unexpected envelope: got %+v, want class=%d type=%q kind=%q", envelope, sample.classID, sample.typeName, sample.kind)
			}
		})
	}
}

func mustReadServiceFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
