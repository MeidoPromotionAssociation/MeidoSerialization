package KCES

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func TestExportNameMapServiceSourceConstructedRoundTripAndProbe(t *testing.T) {
	dir := t.TempDir()
	nativePath := filepath.Join(dir, "export_map.enm")
	jsonPath := nativePath + ".json"
	backPath := filepath.Join(dir, "back", "export_map.enm")
	if err := os.MkdirAll(filepath.Dir(backPath), 0755); err != nil {
		t.Fatal(err)
	}
	want := &serializationKCES.KCESExportNameMap{
		Version: serializationKCES.KCESExportNameMapVersion,
		Entries: []serializationKCES.KCESExportNameMapEntry{
			{InternalName: "GP03_EXPORT_BODY.MATE", FileName: "1.MATE"},
			{InternalName: "gp03_export_hair.menu", FileName: "0.menu"},
		},
	}
	native, err := serializationKCES.EncodeKCESExportNameMap(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nativePath, native, 0644); err != nil {
		t.Fatal(err)
	}

	if !IsKCESExportNameMapFile(nativePath) {
		t.Fatal("native .enm was not routed")
	}
	service := &ExportNameMapService{}
	if err := service.ConvertExportNameMapToJSON(TestConversionContext, nativePath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	if !IsKCESExportNameMapJSONFile(jsonPath) {
		t.Fatal("editing .enm.json marker was not routed")
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, append([]byte{0xef, 0xbb, 0xbf}, jsonData...), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESExportNameMapJSONFile(jsonPath) {
		t.Fatal("BOM-prefixed .enm.json was not routed")
	}
	if err := service.ConvertJSONToExportNameMap(TestConversionContext, jsonPath, backPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	back, err := os.ReadFile(backPath)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := serializationKCES.DecodeKCESExportNameMap(back)
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := []serializationKCES.KCESExportNameMapEntry{
		{InternalName: "GP03_EXPORT_BODY.MATE", FileName: "1.MATE"},
		{InternalName: "gp03_export_hair.menu", FileName: "0.menu"},
	}
	if !reflect.DeepEqual(decoded.Entries, wantEntries) {
		t.Fatalf("round-trip entries = %+v, want %+v", decoded.Entries, wantEntries)
	}

	for _, path := range []string{nativePath, jsonPath} {
		info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(path)
		if err != nil || !matched {
			t.Fatalf("probe %q: matched=%v err=%v", path, matched, err)
		}
		if info.FileType != "enm" || info.StorageFormat != COM3D2Service.FormatJSON || info.Game != COM3D2Service.GameKCES || info.Version != serializationKCES.KCESExportNameMapVersion {
			t.Fatalf("probe %q = %+v", path, info)
		}
	}
}

func TestExportNameMapRoutingRejectsMalformedNativeAndNullEntries(t *testing.T) {
	dir := t.TempDir()
	badNative := filepath.Join(dir, "renamed.enm")
	if err := os.WriteFile(badNative, []byte(`{"version":1000,"serializeData":null}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESExportNameMapFile(badNative) {
		t.Fatal("all non-JSON .enm paths should be native candidates")
	}
	_, matched, err := (&FileTypeService{}).TryFileTypeDetermine(badNative)
	if !matched || err == nil {
		t.Fatalf("malformed native probe: matched=%v err=%v, want matched validation error", matched, err)
	}
	jsonPath := badNative + ".json"
	service := &ExportNameMapService{}
	if err := service.ConvertExportNameMapToJSON(TestConversionContext, badNative, jsonPath, TestConversionMaxOutput); err == nil {
		t.Fatal("ConvertExportNameMapToJSON accepted null serializeData")
	}

	nilJSON := filepath.Join(dir, "renamed.enm.json")
	if err := os.WriteFile(nilJSON, []byte(`{"version":1000,"entries":null}`), 0644); err != nil {
		t.Fatal(err)
	}
	if IsKCESExportNameMapJSONFile(nilJSON) {
		t.Fatal("editing route accepted a null entry list")
	}
	_, matched, err = (&FileTypeService{}).TryFileTypeDetermine(nilJSON)
	if !matched || err == nil {
		t.Fatalf("null-entry editing probe: matched=%v err=%v, want matched validation error", matched, err)
	}
	if err := service.ConvertJSONToExportNameMap(TestConversionContext, nilJSON, filepath.Join(dir, "back.enm"), TestConversionMaxOutput); err == nil {
		t.Fatal("ConvertJSONToExportNameMap accepted a null entry list")
	}
}
