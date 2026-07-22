package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func mustKCESPresetCoreForServiceTest(t *testing.T) *serializationKCES.KCESPresetCore {
	t.Helper()
	core, err := serializationKCES.NewKCESPresetCore()
	if err != nil {
		t.Fatalf("NewKCESPresetCore: %v", err)
	}
	return core
}

func TestKCESPresetServiceDistinguishesLegacyAndCurrentFormats(t *testing.T) {
	legacy := filepath.Join("..", "..", "testdata", "test.preset")
	if IsKCESPresetFile(legacy) {
		t.Fatalf("legacy CM3D2_PRESET %q was misidentified as a current KCES preset", legacy)
	}

	tempDir := t.TempDir()
	currentPath := filepath.Join(tempDir, "current.preset")
	jsonPath := currentPath + ".json"
	backPath := filepath.Join(tempDir, "back.preset")
	core := mustKCESPresetCoreForServiceTest(t)

	encoded, err := serializationKCES.EncodeKCESPreset(&serializationKCES.KCESPreset{
		ContainerVersion: 1000,
		Thumbnail:        []byte("png"),
		MaidData:         core,
		Meta:             &serializationKCES.KCESPresetMeta{Version: 1000, Data: map[string]string{"presetName": "service"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESPresetFile(currentPath) {
		t.Fatal("current VirtualDirectory preset was not detected")
	}
	persetPath := filepath.Join(tempDir, "early.perset")
	if err := os.WriteFile(persetPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESPresetFile(persetPath) {
		t.Fatal("early KCES .perset VirtualDirectory was not detected")
	}

	service := &PresetService{}
	if err := service.ConvertPresetToJson(TestConversionContext, currentPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertPresetToJson: %v", err)
	}
	if !IsKCESPresetJSONFile(jsonPath) {
		t.Fatal("KCES preset JSON marker was not detected")
	}
	persetJSONPath := persetPath + ".json"
	if err := os.WriteFile(persetJSONPath, mustReadFile(t, jsonPath), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESPresetJSONFile(persetJSONPath) {
		t.Fatal("early KCES .perset.json marker was not detected")
	}
	if err := service.ConvertJsonToPreset(TestConversionContext, jsonPath, backPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToPreset: %v", err)
	}
	back, err := service.ReadPresetFile(backPath)
	if err != nil {
		t.Fatalf("ReadPresetFile(back): %v", err)
	}
	if !bytes.Equal(back.MaidData.PropData, core.PropData) || back.Meta.Data["presetName"] != "service" {
		t.Fatalf("service round-trip mismatch: %+v", back)
	}

	// Windows-authored UTF-8 JSON commonly carries a BOM. Detection and the
	// actual converter must agree on accepting it.
	bomJSONPath := filepath.Join(tempDir, "bom.preset.json")
	bomBackPath := filepath.Join(tempDir, "bom-back.preset")
	bomJSON := append([]byte{0xef, 0xbb, 0xbf}, mustReadFile(t, jsonPath)...)
	if err := os.WriteFile(bomJSONPath, bomJSON, 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESPresetJSONFile(bomJSONPath) {
		t.Fatal("BOM-prefixed KCES preset JSON marker was not detected")
	}
	if err := service.ConvertJsonToPreset(TestConversionContext, bomJSONPath, bomBackPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToPreset(BOM): %v", err)
	}
	if _, err := service.ReadPresetFile(bomBackPath); err != nil {
		t.Fatalf("ReadPresetFile(BOM round trip): %v", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
