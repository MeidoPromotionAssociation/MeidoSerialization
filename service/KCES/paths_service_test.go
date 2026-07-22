package KCES

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestPathsServiceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "paths.dat")
	jsonPath := inputPath + ".json"
	backPath := filepath.Join(dir, "back", "paths.dat")
	if err := os.MkdirAll(filepath.Dir(backPath), 0755); err != nil {
		t.Fatal(err)
	}
	want := serializationKCES.NewKCESPathsFile()
	want.Paths = []string{"system", "parts", "日本語"}
	data, err := serializationKCES.EncodeKCESPaths(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESPathsFile(inputPath) {
		t.Fatal("paths.dat was not routed")
	}

	service := &PathsService{}
	if err := service.ConvertPathsToJSON(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	if !IsKCESPathsJSONFile(jsonPath) {
		t.Fatal("paths.dat JSON marker was not routed")
	}
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	jsonData = append([]byte{0xef, 0xbb, 0xbf}, jsonData...)
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESPathsJSONFile(jsonPath) {
		t.Fatal("BOM-prefixed paths.dat JSON marker was not routed")
	}
	if err := service.ConvertJSONToPaths(TestConversionContext, jsonPath, backPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	backData, err := os.ReadFile(backPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := serializationKCES.DecodeKCESPaths(backData)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Paths, want.Paths) {
		t.Fatalf("paths = %v, want %v", got.Paths, want.Paths)
	}
}
