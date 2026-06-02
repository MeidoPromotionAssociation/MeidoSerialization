package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
)

func TestDataService_PskUsesFixedKCESSample(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "kces_assets", "default_skirt.psk")
	if _, err := os.Stat(sample); err != nil {
		t.Fatalf("fixed KCES .psk sample missing: %v", err)
	}

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "default_skirt.psk.json")
	outPath := filepath.Join(tmpDir, "default_skirt.psk")

	service := &DataService{}
	if err := service.ConvertDataToJson(sample, jsonPath); err != nil {
		t.Fatalf("ConvertDataToJson: %v", err)
	}
	var decodedJSON serializationCOM3D2.Psk
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &decodedJSON); err != nil {
		t.Fatalf("unmarshal .psk json: %v", err)
	}
	if decodedJSON.Signature == "" || decodedJSON.Version == 0 {
		t.Fatalf("invalid .psk json: %+v", decodedJSON)
	}

	if err := service.ConvertJsonToData(jsonPath, outPath); err != nil {
		t.Fatalf("ConvertJsonToData: %v", err)
	}
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	roundTrip, err := serializationCOM3D2.ReadPsk(f)
	if err != nil {
		t.Fatalf("ReadPsk round-trip: %v", err)
	}
	if roundTrip.Signature != decodedJSON.Signature || roundTrip.Version != decodedJSON.Version {
		t.Fatalf("round-trip changed header: got %+v want signature=%q version=%d", roundTrip, decodedJSON.Signature, decodedJSON.Version)
	}
}

func TestDataService_NeiUsesFixedKCESSample(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "kces_assets", "edit_pose_enabled_list.nei")
	if _, err := os.Stat(sample); err != nil {
		t.Fatalf("fixed KCES .nei sample missing: %v", err)
	}

	referenceService := &COM3D2Service.NeiService{}
	want, err := referenceService.ReadNeiFile(sample)
	if err != nil {
		t.Fatalf("ReadNeiFile sample: %v", err)
	}
	if want.Rows == 0 || want.Cols == 0 || len(want.Data) == 0 {
		t.Fatalf("empty .nei sample: %+v", want)
	}

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "edit_pose_enabled_list.csv")
	outPath := filepath.Join(tmpDir, "edit_pose_enabled_list.nei")

	service := &DataService{}
	if err := service.ConvertNeiToCSV(sample, csvPath); err != nil {
		t.Fatalf("ConvertNeiToCSV: %v", err)
	}
	if err := service.ConvertCSVToNei(csvPath, outPath); err != nil {
		t.Fatalf("ConvertCSVToNei: %v", err)
	}

	got, err := referenceService.ReadNeiFile(outPath)
	if err != nil {
		t.Fatalf("ReadNeiFile round-trip: %v", err)
	}
	if got.Rows != want.Rows || got.Cols != want.Cols || !reflect.DeepEqual(got.Data, want.Data) {
		t.Fatalf("round-trip changed .nei data: got rows=%d cols=%d data=%v want rows=%d cols=%d data=%v", got.Rows, got.Cols, got.Data, want.Rows, want.Cols, want.Data)
	}
}
