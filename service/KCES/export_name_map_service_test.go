package KCES

import (
	"bytes"
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

func TestExportNameMapRoutingPreservesSchemaAnomaliesAndAcceptsNilEntries(t *testing.T) {
	dir := t.TempDir()
	badNative := filepath.Join(dir, "renamed.enm")
	if err := os.WriteFile(badNative, []byte(`{"version":1000,"serializeData":null}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESExportNameMapFile(badNative) {
		t.Fatal("all non-JSON .enm paths should be native candidates")
	}
	info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(badNative)
	if !matched || err != nil || info.FileType != "enm" || info.Version != 1000 {
		t.Fatalf("schema-anomalous native probe: matched=%v info=%+v err=%v", matched, info, err)
	}
	jsonPath := badNative + ".json"
	backPath := filepath.Join(dir, "back.enm")
	service := &ExportNameMapService{}
	if err := service.ConvertExportNameMapToJSON(TestConversionContext, badNative, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertExportNameMapToJSON() error = %v", err)
	}
	if err := service.ConvertJSONToExportNameMap(TestConversionContext, jsonPath, backPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJSONToExportNameMap() error = %v", err)
	}
	want, err := os.ReadFile(badNative)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(backPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("schema-anomalous native text changed: got %s want %s", got, want)
	}

	nilJSON := filepath.Join(dir, "renamed.enm.json")
	if err := os.WriteFile(nilJSON, []byte(`{"format":"kces-export-name-map","version":1000,"entries":null}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESExportNameMapJSONFile(nilJSON) {
		t.Fatal("editing route rejected a representable nil entry list")
	}
	info, matched, err = (&FileTypeService{}).TryFileTypeDetermine(nilJSON)
	if err != nil || !matched || info.FileType != "enm" || info.Version != 1000 {
		t.Fatalf("nil-entry editing probe: matched=%v info=%+v err=%v", matched, info, err)
	}
}
