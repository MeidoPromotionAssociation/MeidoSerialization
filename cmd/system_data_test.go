package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestKCESSystemDataConvertCommandsAndFilters(t *testing.T) {
	colors := make(map[int32]int32, 9)
	for i := int32(0); i <= 8; i++ {
		colors[i] = i
	}
	want := &serializationKCES.KCESSystemData{
		Version:     1000,
		Directories: map[string]ct.VirtualDirectoryMetadata{"EditData": {Version: 1000}},
		EditData: []serializationKCES.KCESEditDataFile{{
			Path:         "EditData/PaletteColorSave0",
			Kind:         serializationKCES.KCESEditDataPaletteColor,
			PaletteColor: &serializationKCES.PaletteColorSaveData{Color: colors, IsSave: 1},
		}},
	}
	binary, err := serializationKCES.EncodeKCESSystemData(want)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "system.dat")
	jsonPath := path + ".json"
	if err := os.WriteFile(path, binary, 0644); err != nil {
		t.Fatal(err)
	}
	if !isModFile(path) {
		t.Fatal("isModFile did not recognize system.dat")
	}
	if err := convertToJson(path); err != nil {
		t.Fatalf("convertToJson: %v", err)
	}
	if !isModJsonFile(jsonPath) {
		t.Fatal("isModJsonFile did not recognize system.dat.json")
	}

	oldType, oldStrict := fileType, strictMode
	t.Cleanup(func() { fileType, strictMode = oldType, oldStrict })
	fileType = "system"
	strictMode = false
	if !fileTypeFilter(path) || fileTypeFilter(jsonPath) {
		t.Fatal("non-strict --type system filter mismatch")
	}
	fileType = "system.json"
	if fileTypeFilter(path) || !fileTypeFilter(jsonPath) {
		t.Fatal("non-strict --type system.json filter mismatch")
	}
	strictMode = true
	if fileTypeFilter(path) || !fileTypeFilter(jsonPath) {
		t.Fatal("strict --type system.json filter mismatch")
	}
	fileType = "system"
	if !fileTypeFilter(path) || fileTypeFilter(jsonPath) {
		t.Fatal("strict --type system filter mismatch")
	}

	if err := convertToMod(jsonPath); err != nil {
		t.Fatalf("convertToMod: %v", err)
	}
	got, err := serializationKCES.DecodeKCESSystemData(mustReadFile(t, path))
	if err != nil {
		t.Fatalf("DecodeKCESSystemData output: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CLI round trip changed system.dat: got %#v, want %#v", got, want)
	}
}

func TestKCESSystemDataMalformedEditingJSONReturnsValidationError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "system.dat.JSON")
	data := []byte(`{"version":1000,"future":1}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), data...)
	if err := convertToMod(path); err == nil {
		t.Fatal("convertToMod accepted unknown system.dat JSON field")
	}
	if got := mustReadFile(t, path); !reflect.DeepEqual(got, before) {
		t.Fatal("failed conversion modified its JSON input")
	}
}
