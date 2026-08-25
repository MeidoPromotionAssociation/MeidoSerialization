package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/kcesfixtures"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func TestFileTypeServiceRecognizesRealKCESFilesByContent(t *testing.T) {
	testdata := filepath.Join("..", "..", "testdata")
	modelPath := kcesfixtures.TextAssetPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.model")
	menuAssetsPath := kcesfixtures.TextAssetPath(t, "cm3d2_eyes.aba", "cm3d2_eyes.menuassets")
	dbConfPath := kcesfixtures.TextAssetPath(t, "partsmeta.aba", "default_hairf.dbconf")
	db2ConfPath := kcesfixtures.TextAssetPath(t, "partsmeta.aba", "default_hairf.db2conf")
	ikColliderPath := kcesfixtures.TextAssetPath(t, "system.aba", "maidIKCollider.ikcol")
	hitCheckPath := kcesfixtures.TextAssetPath(t, "system.aba", "IK.hitcheck")
	nativeJSONPath := kcesfixtures.TextAssetPath(t, "parts_bv001.aba", "crc2_Underwear038_pants.undressdat")

	tests := []struct {
		name     string
		path     string
		typeName string
		format   string
		version  int32
	}{
		{name: "content_table", path: filepath.Join(testdata, "KCES", "cm3d2_eyes.ct"), typeName: "ct", format: COM3D2Service.FormatBinary, version: 1000},
		{name: "unityfs", path: filepath.Join(testdata, "KCES", "cm3d2_eyes.aba"), typeName: "aba", format: COM3D2Service.FormatBinary, version: 7},
		{name: "model", path: modelPath, typeName: "model", format: COM3D2Service.FormatBinary, version: 1001},
		{name: "menuassets", path: menuAssetsPath, typeName: "menuassets", format: COM3D2Service.FormatBinary, version: 1000},
		{name: "dynamic_bone", path: dbConfPath, typeName: "dbconf", format: COM3D2Service.FormatBinary, version: 1000},
		{name: "msgpack_string", path: db2ConfPath, typeName: "db2conf", format: COM3D2Service.FormatBinary},
		{name: "ik_collider", path: ikColliderPath, typeName: "ikcol", format: COM3D2Service.FormatBinary, version: 1000},
		{name: "hitcheck", path: hitCheckPath, typeName: "hitcheck", format: COM3D2Service.FormatBinary},
		{name: "native_json_text", path: nativeJSONPath, typeName: "undressdat", format: COM3D2Service.FormatJSON},
	}

	service := &FileTypeService{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, matched, err := service.TryFileTypeDetermine(test.path)
			if err != nil {
				t.Fatalf("TryFileTypeDetermine(%q): %v", test.path, err)
			}
			if !matched {
				t.Fatalf("TryFileTypeDetermine(%q) did not match", test.path)
			}
			if info.FileType != test.typeName || info.StorageFormat != test.format || info.Game != COM3D2Service.GameKCES {
				t.Fatalf("info = %+v, want type=%q format=%q game=%q", info, test.typeName, test.format, COM3D2Service.GameKCES)
			}
			if info.Version != test.version {
				t.Fatalf("version = %d, want %d (info=%+v)", info.Version, test.version, info)
			}
		})
	}
}

func TestFileTypeServicePreservesLegacySharedExtensions(t *testing.T) {
	service := &FileTypeService{}
	for _, path := range []string{
		filepath.Join("..", "..", "testdata", "test.model"),
		filepath.Join("..", "..", "testdata", "test.preset"),
	} {
		info, matched, err := service.TryFileTypeDetermine(path)
		if err != nil {
			t.Fatalf("TryFileTypeDetermine(%q): %v", path, err)
		}
		if matched {
			t.Fatalf("legacy file %q was misidentified as KCES: %+v", path, info)
		}
	}
}

func TestFileTypeServiceRecognizesCurrentPresetAndJSONMarkers(t *testing.T) {
	dir := t.TempDir()
	service := &FileTypeService{}
	presetName := "probe"
	preset := &serializationKCES.KCESPreset{
		ContainerVersion: 1000,
		Thumbnail:        []byte{0x89, 'P', 'N', 'G', 1},
		MaidData:         mustKCESPresetCoreForServiceTest(t),
		Meta:             &serializationKCES.KCESPresetMeta{Version: 1000, Data: map[string]*string{"presetName": &presetName}},
	}
	encoded, err := serializationKCES.EncodeKCESPreset(preset)
	if err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(dir, "current.preset")
	if err := os.WriteFile(currentPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := service.TryFileTypeDetermine(currentPath)
	if err != nil || !matched {
		t.Fatalf("current preset probe: matched=%v info=%+v err=%v", matched, info, err)
	}
	if info.FileType != "preset" || info.Game != COM3D2Service.GameKCES || info.Version != 1000 {
		t.Fatalf("current preset info = %+v", info)
	}

	jsonPath := currentPath + ".json"
	if err := (&PresetService{}).ConvertPresetToJson(TestConversionContext, currentPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	jsonInfo, matched, err := service.TryFileTypeDetermine(jsonPath)
	if err != nil || !matched {
		t.Fatalf("current preset JSON probe: matched=%v info=%+v err=%v", matched, jsonInfo, err)
	}
	if jsonInfo.FileType != "preset" || jsonInfo.StorageFormat != COM3D2Service.FormatJSON || jsonInfo.Game != COM3D2Service.GameKCES {
		t.Fatalf("current preset JSON info = %+v", jsonInfo)
	}
}

func TestFileTypeServiceRecognizesNSONAndEditingEnvelope(t *testing.T) {
	dir := t.TempDir()
	service := &FileTypeService{}
	nativePath := filepath.Join(dir, "dance_enabled_list.nson")
	if err := os.WriteFile(nativePath, []byte(`{"version":1000,"_ids":[1,2,3]}`), 0644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		path string
	}{
		{name: "native", path: nativePath},
		{name: "editing envelope", path: nativePath + ".json"},
	} {
		if test.name == "editing envelope" {
			if err := (&MiscService{}).ConvertMiscToJson(TestConversionContext, nativePath, test.path, TestConversionMaxOutput); err != nil {
				t.Fatalf("ConvertMiscToJson: %v", err)
			}
		}
		t.Run(test.name, func(t *testing.T) {
			info, matched, err := service.TryFileTypeDetermine(test.path)
			if err != nil || !matched {
				t.Fatalf("TryFileTypeDetermine: matched=%v info=%+v err=%v", matched, info, err)
			}
			if info.FileType != "nson" || info.StorageFormat != COM3D2Service.FormatJSON || info.Game != COM3D2Service.GameKCES {
				t.Fatalf("NSON info = %+v", info)
			}
		})
	}
}

func TestFileTypeServiceRejectsMalformedKCESOnlyCandidates(t *testing.T) {
	service := &FileTypeService{}
	for _, name := range []string{"bad.ct", "bad.aba", "bad.dbconf", "bad.menuassets", "bad.hitcheck", "bad.perset"} {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte("plain text,not a KCES payload"), 0644); err != nil {
			t.Fatal(err)
		}
		info, matched, err := service.TryFileTypeDetermine(path)
		if !matched || err == nil {
			t.Fatalf("malformed %q: matched=%v info=%+v err=%v", name, matched, info, err)
		}
		if info.FileType != COM3D2Service.UnknownFileType {
			t.Fatalf("malformed %q was assigned type %q", name, info.FileType)
		}
	}
}

func TestFileTypeServiceStrictlyValidatesJSONEnvelopeContents(t *testing.T) {
	service := &FileTypeService{}
	dir := t.TempDir()

	envelope, err := (&CtService{}).ReadCtEnvelope(filepath.Join("..", "..", "testdata", "KCES", "cm3d2_eyes.ct"))
	if err != nil {
		t.Fatal(err)
	}
	envelope.Files = append(envelope.Files, CtEnvelopeFile{Name: "future-invalid", DataBase64: "%%%not-base64%%%"})
	invalidCT, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	ctJSONPath := filepath.Join(dir, "bad.ct.json")
	if err := os.WriteFile(ctJSONPath, invalidCT, 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := service.TryFileTypeDetermine(ctJSONPath)
	if !matched || err == nil || info.FileType != COM3D2Service.UnknownFileType {
		t.Fatalf("invalid CT JSON: matched=%v info=%+v err=%v", matched, info, err)
	}

	for name, body := range map[string]string{
		"bad-base64.bytes.json": `{"format":"kces-unity-raw-object","dataBase64":"%%%"}`,
		"empty.bytes.json":      `{"format":"kces-unity-raw-object","dataBase64":""}`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		info, matched, err := service.TryFileTypeDetermine(path)
		if !matched || err == nil || info.FileType != COM3D2Service.UnknownFileType {
			t.Fatalf("invalid raw Unity JSON %q: matched=%v info=%+v err=%v", name, matched, info, err)
		}
	}
}

func TestFileTypeServiceAcceptsUTF8BOMAndLongJSONWhitespace(t *testing.T) {
	dir := t.TempDir()
	preset := &serializationKCES.KCESPreset{
		ContainerVersion: 1000,
		Thumbnail:        []byte("png"),
		MaidData:         mustKCESPresetCoreForServiceTest(t),
	}
	data, err := json.Marshal(preset)
	if err != nil {
		t.Fatal(err)
	}
	prefixed := append([]byte{0xef, 0xbb, 0xbf}, []byte(" \r\n\t")...)
	prefixed = append(prefixed, make([]byte, 5000)...)
	for i := 7; i < len(prefixed); i++ {
		prefixed[i] = ' '
	}
	prefixed = append(prefixed, data...)
	path := filepath.Join(dir, "bom.preset.json")
	if err := os.WriteFile(path, prefixed, 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(path)
	if err != nil || !matched || info.FileType != "preset" || info.StorageFormat != COM3D2Service.FormatJSON {
		t.Fatalf("BOM/whitespace JSON probe: matched=%v info=%+v err=%v", matched, info, err)
	}
}
