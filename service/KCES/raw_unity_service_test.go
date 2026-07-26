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
			if err := service.ConvertRawUnityObjectToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
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
			if err := service.ConvertJsonToRawUnityObject(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
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
	sourceAba := filepath.Join("..", "..", "testdata", "aba", "parts_personal_om015_gp003.aba")
	if _, err := os.Stat(sourceAba); err != nil {
		t.Skipf("sample not found: %v", err)
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "texture.tex.bytes")
	bundle, bundleFile, err := (&AbaService{}).ReadAba(sourceAba)
	if err != nil {
		t.Fatal(err)
	}
	defer bundleFile.Close()
	wroteFixture := false
	for directoryIndex, directory := range bundle.BlockInfo.DirectoryInfos {
		if !directory.IsSerialized() || wroteFixture {
			continue
		}
		serialized, err := bundle.GetFileData(int64(directoryIndex))
		if err != nil {
			t.Fatal(err)
		}
		af, err := aba.ReadAssetsFile(serialized)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range af.GetAssetEntries() {
			if entry.TypeId != aba.ClassIDTexture2D {
				continue
			}
			info := af.GetAssetInfoByPathID(entry.PathId)
			data, err := af.GetAssetData(info)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(inputPath, data, 0644); err != nil {
				t.Fatal(err)
			}
			if err := writeRawUnityTypeTreeSidecar(inputPath, af, info, entry, entry.Name); err != nil {
				t.Fatal(err)
			}
			wroteFixture = true
			break
		}
	}
	if !wroteFixture {
		t.Fatal("target sample has no Texture2D fixture")
	}

	service := &RawUnityObjectService{}
	jsonPath := filepath.Join(tmpDir, "texture.tex.bytes.json")
	if err := service.ConvertRawUnityObjectToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
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
	if err := service.ConvertJsonToRawUnityObject(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
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
	if err := service.ConvertRawUnityObjectToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
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

func TestRawUnityObjectService_NativeObjectJSONEditsTypeTreeValue(t *testing.T) {
	tmpDir := t.TempDir()
	directory := filepath.Join(tmpDir, "Material")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(directory, "sample.material")
	object := nativeUnityServiceTestObject(t, aba.ClassIDMaterial, "sample", 17)
	encoded, err := aba.EncodeNativeUnityObject(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESRawUnityBytesFile(inputPath) {
		t.Fatalf("native .material file was not detected")
	}

	service := &RawUnityObjectService{}
	jsonPath := inputPath + ".json"
	if err := service.ConvertRawUnityObjectToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	if !IsKCESRawUnityBytesJSONFile(jsonPath) {
		t.Fatalf("native object JSON was not detected")
	}
	var envelope RawUnityObjectEnvelope
	if err := json.Unmarshal(mustReadServiceFile(t, jsonPath), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Format != NativeUnityObjectJSONFormat || envelope.ReadOnly || envelope.SchemaBase64 == "" || envelope.TypeTree == nil || envelope.TypeTree.Value == nil {
		t.Fatalf("unexpected native object envelope: %+v", envelope)
	}
	custom := typeTreeJSONChild(envelope.TypeTree.Value, "m_Custom")
	if custom == nil {
		t.Fatalf("native object JSON has no m_Custom field")
	}
	custom.Value = float64(23)
	modifiedJSON, err := json.MarshalIndent(&envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, append(modifiedJSON, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(directory, "edited.material")
	if err := service.ConvertJsonToRawUnityObject(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outputPath + ".meta.json"); !os.IsNotExist(err) {
		t.Fatalf("native JSON conversion created metadata sidecar: %v", err)
	}
	if _, err := os.Stat(outputPath + ".typetree.json"); !os.IsNotExist(err) {
		t.Fatalf("native JSON conversion created TypeTree sidecar: %v", err)
	}
	restored, err := aba.ReadNativeUnityObject(mustReadServiceFile(t, outputPath))
	if err != nil {
		t.Fatal(err)
	}
	value, err := restored.DecodeValue()
	if err != nil {
		t.Fatal(err)
	}
	if got := value.Field("m_Custom").Value; got != int64(23) {
		t.Fatalf("edited m_Custom = %#v, want 23", got)
	}
}

func TestRawUnityObjectService_AllPromisedNativeClassesRoundTripEditableJSON(t *testing.T) {
	tests := []struct {
		path    string
		classID int32
	}{
		{path: "Mesh/sample.mmesh", classID: aba.ClassIDMesh},
		{path: "Texture2D/sample.texture2d", classID: aba.ClassIDTexture2D},
		{path: "Sprite/sample.sprite", classID: aba.ClassIDSprite},
		{path: "SpriteAtlas/sample.partsatlas", classID: aba.ClassIDSpriteAtlas},
		{path: "AnimationClip/sample.anm", classID: aba.ClassIDAnimationClip},
		{path: "Material/sample.material", classID: aba.ClassIDMaterial},
		{path: "MonoBehaviour/sample.monobehaviour", classID: aba.ClassIDMonoBehaviour},
	}
	service := &RawUnityObjectService{}
	for _, test := range tests {
		test := test
		t.Run(filepath.ToSlash(test.path), func(t *testing.T) {
			inputPath := filepath.Join(t.TempDir(), filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(inputPath), 0755); err != nil {
				t.Fatal(err)
			}
			object := nativeUnityServiceTestObject(t, test.classID, "sample", 17)
			encoded, err := aba.EncodeNativeUnityObject(object)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
				t.Fatal(err)
			}
			jsonPath := inputPath + ".json"
			if err := service.ConvertRawUnityObjectToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
				t.Fatal(err)
			}
			var envelope RawUnityObjectEnvelope
			if err := json.Unmarshal(mustReadServiceFile(t, jsonPath), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.ReadOnly || envelope.ClassID != test.classID || envelope.SchemaBase64 == "" || envelope.TypeTree == nil {
				t.Fatalf("unexpected editable envelope: %+v", envelope)
			}
			custom := typeTreeJSONChild(envelope.TypeTree.Value, "m_Custom")
			if custom == nil {
				t.Fatal("editable JSON has no m_Custom field")
			}
			custom.Value = float64(29)
			modified, err := json.Marshal(&envelope)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(jsonPath, modified, 0644); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(filepath.Dir(inputPath), "edited"+filepath.Ext(inputPath))
			if err := service.ConvertJsonToRawUnityObject(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
				t.Fatal(err)
			}
			restored, err := aba.ReadNativeUnityObject(mustReadServiceFile(t, outputPath))
			if err != nil {
				t.Fatal(err)
			}
			value, err := restored.DecodeValue()
			if err != nil {
				t.Fatal(err)
			}
			if restored.ClassID != test.classID || value.Field("m_Custom").Value != int64(29) {
				t.Fatalf("restored ClassID=%d m_Custom=%#v", restored.ClassID, value.Field("m_Custom").Value)
			}
		})
	}
}

func TestRawUnityObjectService_AudioClipJSONReplacesInlinePayload(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "AudioClip", "sample.audioclip")
	if err := os.MkdirAll(filepath.Dir(inputPath), 0755); err != nil {
		t.Fatal(err)
	}
	object := nativeUnityAudioServiceTestObject(t, []byte("OggSsource"))
	encoded, err := aba.EncodeNativeUnityObject(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}
	service := &RawUnityObjectService{}
	jsonPath := inputPath + ".json"
	if err := service.ConvertRawUnityObjectToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	var envelope RawUnityObjectEnvelope
	if err := json.Unmarshal(mustReadServiceFile(t, jsonPath), &envelope); err != nil {
		t.Fatal(err)
	}
	replacement := []byte("RIFF\x04\x00\x00\x00WAVE")
	envelope.ResourceDataBase64 = base64.StdEncoding.EncodeToString(replacement)
	modified, err := json.Marshal(&envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, modified, 0644); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(filepath.Dir(inputPath), "edited.audioclip")
	if err := service.ConvertJsonToRawUnityObject(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	restored, err := aba.ReadAudioClip(mustReadServiceFile(t, outputPath))
	if err != nil {
		t.Fatal(err)
	}
	audioData, err := restored.AudioData()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(audioData, replacement) {
		t.Fatalf("restored AudioClip payload = %q, want %q", audioData, replacement)
	}
}

func TestRawUnityObjectService_UnknownClassTypeTreeViewIsReadOnly(t *testing.T) {
	tmpDir := t.TempDir()
	directory := filepath.Join(tmpDir, "Type_95")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(directory, "sample.bytes")
	object := nativeUnityServiceTestObject(t, 95, "unknown", 7)
	encoded, err := aba.EncodeNativeUnityObject(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	service := &RawUnityObjectService{}
	envelope, err := service.ReadRawUnityObjectFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !envelope.ReadOnly || envelope.SchemaBase64 != "" || envelope.TypeTree == nil || envelope.TypeTree.Value == nil {
		t.Fatalf("unexpected unknown-class envelope: %+v", envelope)
	}
	jsonData, err := json.Marshal(&envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := decodeNativeUnityObjectEditingJSON(jsonData); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("read-only conversion error = %v", err)
	}
}

func TestEditableTypeTreeJSONDoesNotValidateDigestFields(t *testing.T) {
	source := bytes.Repeat([]byte{0x5a}, 1024)
	jsonValue := editableTypeTreeJSONValue(&aba.TypeTreeValue{TypeName: "TypelessData", Name: "m_Data", Value: source})
	jsonValue.Bytes.Length = 1
	jsonValue.Bytes.SHA256 = "deliberately-not-a-hash"
	restored, err := editableTypeTreeValueFromJSON(jsonValue)
	if err != nil {
		t.Fatal(err)
	}
	if data, ok := restored.Value.([]byte); !ok || !bytes.Equal(data, source) {
		t.Fatalf("restored bytes differ from editable data")
	}
}

func nativeUnityServiceTestObject(t *testing.T, classID int32, name string, custom int64) *aba.NativeUnityObject {
	t.Helper()
	var stringBuffer []byte
	stringOffset := func(value string) uint32 {
		offset := uint32(len(stringBuffer))
		stringBuffer = append(stringBuffer, value...)
		stringBuffer = append(stringBuffer, 0)
		return offset
	}
	tree := aba.TypeTreeType{
		TypeId:          classID,
		ScriptTypeIndex: -1,
		Nodes: []aba.TypeTreeNode{
			{Version: 1, Level: 0, TypeStrOff: stringOffset("Material"), NameStrOff: stringOffset("Base"), ByteSize: -1},
			{Version: 1, Level: 1, TypeStrOff: stringOffset("string"), NameStrOff: stringOffset("m_Name"), ByteSize: -1, MetaFlags: 0x4000},
			{Version: 1, Level: 1, TypeStrOff: stringOffset("int"), NameStrOff: stringOffset("m_Custom"), ByteSize: 4},
		},
		StringBuffer: stringBuffer,
	}
	object := &aba.NativeUnityObject{ClassID: classID, TypeTree: tree}
	value := &aba.TypeTreeValue{
		TypeName: "Material",
		Name:     "Base",
		Children: []*aba.TypeTreeValue{
			{TypeName: "string", Name: "m_Name", Value: name},
			{TypeName: "int", Name: "m_Custom", Value: custom},
		},
	}
	var err error
	object.Data, err = object.EncodeValue(value)
	if err != nil {
		t.Fatal(err)
	}
	return object
}

func nativeUnityAudioServiceTestObject(t *testing.T, audioData []byte) *aba.NativeUnityObject {
	t.Helper()
	var stringBuffer []byte
	stringOffset := func(value string) uint32 {
		offset := uint32(len(stringBuffer))
		stringBuffer = append(stringBuffer, value...)
		stringBuffer = append(stringBuffer, 0)
		return offset
	}
	tree := aba.TypeTreeType{
		TypeId:          aba.ClassIDAudioClip,
		ScriptTypeIndex: -1,
		Nodes: []aba.TypeTreeNode{
			{Level: 0, TypeStrOff: stringOffset("AudioClip"), NameStrOff: stringOffset("Base"), ByteSize: -1},
			{Level: 1, TypeStrOff: stringOffset("string"), NameStrOff: stringOffset("m_Name"), ByteSize: -1, MetaFlags: 0x4000},
			{Level: 1, TypeStrOff: stringOffset("StreamedResource"), NameStrOff: stringOffset("m_Resource"), ByteSize: -1},
			{Level: 2, TypeStrOff: stringOffset("string"), NameStrOff: stringOffset("m_Source"), ByteSize: -1, MetaFlags: 0x4000},
			{Level: 2, TypeStrOff: stringOffset("UInt64"), NameStrOff: stringOffset("m_Offset"), ByteSize: 8},
			{Level: 2, TypeStrOff: stringOffset("UInt64"), NameStrOff: stringOffset("m_Size"), ByteSize: 8},
		},
		StringBuffer: stringBuffer,
	}
	root := &aba.TypeTreeValue{
		TypeName: "AudioClip",
		Name:     "Base",
		Children: []*aba.TypeTreeValue{
			{TypeName: "string", Name: "m_Name", Value: "sample"},
			{TypeName: "StreamedResource", Name: "m_Resource", Children: []*aba.TypeTreeValue{
				{TypeName: "string", Name: "m_Source", Value: ""},
				{TypeName: "UInt64", Name: "m_Offset", Value: uint64(0)},
				{TypeName: "UInt64", Name: "m_Size", Value: uint64(len(audioData))},
			}},
		},
	}
	object := &aba.NativeUnityObject{ClassID: aba.ClassIDAudioClip, TypeTree: tree}
	var err error
	object.Data, err = object.EncodeValueWithTrailingData(root, audioData)
	if err != nil {
		t.Fatal(err)
	}
	return object
}

func typeTreeJSONChild(value *TypeTreeJSONValue, name string) *TypeTreeJSONValue {
	if value == nil {
		return nil
	}
	for _, child := range value.Children {
		if child != nil && child.Name == name {
			return child
		}
	}
	return nil
}

func mustReadServiceFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
