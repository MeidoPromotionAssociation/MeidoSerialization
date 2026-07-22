package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func TestSystemDataServiceJSONRoundTripAndFileType(t *testing.T) {
	colors := make(map[int]int, 9)
	for i := 0; i <= 8; i++ {
		colors[i] = i * 10
	}
	presetID := "service-color-preset"
	baseMenu := "hairf001_i_.menu"
	layerName := "hair"
	viewName := "Hair"
	colorPreset, err := serializationKCES.NewColorPreset("12345678-1234-1234-1234-123456789abc")
	if err != nil {
		t.Fatal(err)
	}
	colorPreset.ID = &presetID
	colorPreset.BaseMenuFile = &baseMenu
	colorPreset.UserCreated = true
	colorPreset.IsAdvancedMode = true
	colorPreset.ColorPackList = []*serializationKCES.ColorPresetColorPack{{
		Version:            serializationKCES.ColorPresetPackVersion,
		MPNs:               []int{158},
		LayerName:          &layerName,
		ViewName:           &viewName,
		Type:               serializationKCES.ColorPresetPackColorAndAlpha,
		Alpha:              0.625,
		AllowedMPNOverride: true,
		ColorList: []*serializationKCES.ColorPresetLayerFreeColor{{
			Version: serializationKCES.ColorPresetColorVersion,
			BaseColor: &serializationKCES.ColorPresetFreeColor{
				Version: serializationKCES.ColorPresetColorVersion, Hue: 10, Saturation: 20, Brightness: 30, Contrast: 40,
			},
			ShadowColor: &serializationKCES.ColorPresetFreeColor{
				Version: serializationKCES.ColorPresetColorVersion, Hue: 50, Saturation: 60, Brightness: 70, Contrast: 80,
			},
			ShadowRate: 90,
		}},
		GradationColorList: []*serializationKCES.ColorPresetGradationColor{},
	}}
	value := &serializationKCES.KCESSystemData{
		Format:  serializationKCES.KCESSystemDataFormat,
		Version: 1000,
		EditData: []serializationKCES.KCESEditDataFile{
			{
				Path:         "EditData/PaletteColorSave0",
				Kind:         serializationKCES.KCESEditDataPaletteColor,
				PaletteColor: &serializationKCES.PaletteColorSaveData{Color: colors, IsSave: 1},
			},
			{
				Path:        "EditData/color_preset/hairf/service-color-preset",
				Kind:        serializationKCES.KCESEditDataColorPreset,
				ColorPreset: colorPreset,
			},
		},
		ExtraFiles: map[string][]byte{"EditData/future": {1, 2, 3}},
	}
	binary, err := serializationKCES.EncodeKCESSystemData(value)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "system.dat")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(dir, "roundtrip", "system.dat")
	if err := os.WriteFile(inputPath, binary, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		t.Fatal(err)
	}

	service := &SystemDataService{}
	if err := service.ConvertSystemDataToJSON(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertSystemDataToJSON: %v", err)
	}
	jsonData := mustReadTestFile(t, jsonPath)
	if !bytes.Contains(jsonData, []byte(`"kind": "color-preset"`)) || !bytes.Contains(jsonData, []byte(`"colorPreset"`)) || !bytes.Contains(jsonData, []byte(`"instanceGuid": "12345678-1234-1234-1234-123456789abc"`)) {
		t.Fatalf("generated JSON did not expose the color-preset union:\n%s", jsonData)
	}
	if err := service.ConvertJSONToSystemData(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJSONToSystemData: %v", err)
	}
	got, err := serializationKCES.DecodeKCESSystemData(mustReadTestFile(t, outputPath))
	if err != nil {
		t.Fatalf("DecodeKCESSystemData output: %v", err)
	}
	want, err := serializationKCES.DecodeKCESSystemData(binary)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("service round trip changed system.dat: got %#v, want %#v", got, want)
	}
	if !IsKCESSystemDataFile(inputPath) || !IsKCESSystemDataJSONFile(jsonPath) {
		t.Fatal("system.dat file predicates did not recognize generated files")
	}

	for _, path := range []string{inputPath, jsonPath} {
		info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(path)
		if err != nil || !matched {
			t.Fatalf("TryFileTypeDetermine(%q): matched=%v info=%+v err=%v", path, matched, info, err)
		}
		if info.FileType != "system" || info.Game != COM3D2Service.GameKCES || info.Version != 1000 {
			t.Fatalf("system info = %+v", info)
		}
	}

	// Content recognition also identifies a renamed system VirtualDirectory by
	// its EditData subtree, not just by the conventional basename.
	renamed := filepath.Join(dir, "renamed.vd")
	if err := os.WriteFile(renamed, binary, 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(renamed)
	if err != nil || !matched || info.FileType != "system" {
		t.Fatalf("renamed system.dat: matched=%v info=%+v err=%v", matched, info, err)
	}
}

func TestSystemDataServiceStrictEditingJSON(t *testing.T) {
	valid := []byte(`{"format":"kces-system-data","version":1000}`)
	if _, err := decodeKCESSystemDataEditingJSON(valid); err != nil {
		t.Fatalf("valid minimal system JSON: %v", err)
	}
	for name, data := range map[string][]byte{
		"missing marker": []byte(`{"version":1000}`),
		"wrong marker":   []byte(`{"format":"future","version":1000}`),
		"unknown field":  []byte(`{"format":"kces-system-data","version":1000,"future":1}`),
		"trailing value": append(append([]byte(nil), valid...), []byte(` {}`)...),
		"invalid UTF-8":  append([]byte(`{"format":"kces-system-data","version":1000,"extraFiles":{"x":"`), 0xff, '"', '}', '}'),
		"null":           []byte(`null`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeKCESSystemDataEditingJSON(data); err == nil {
				t.Fatalf("accepted invalid system JSON %q", data)
			}
		})
	}

	bom := append([]byte{0xef, 0xbb, 0xbf}, valid...)
	if _, err := decodeKCESSystemDataEditingJSON(bom); err != nil {
		t.Fatalf("UTF-8 BOM: %v", err)
	}
}

func TestFileTypeSystemDataValidatesKnownEditDataPayload(t *testing.T) {
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize)}
	table.AddFile("EditData/PaletteColorSave0", []byte{0x93, 0x80})
	var binary bytes.Buffer
	if err := ct.WriteContentTable(&binary, table); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "system.dat")
	if err := os.WriteFile(path, binary.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(path)
	if !matched || err == nil || !strings.Contains(err.Error(), "PaletteColorSave0") {
		t.Fatalf("matched=%v info=%+v err=%v", matched, info, err)
	}
	if info.FileType != COM3D2Service.UnknownFileType {
		t.Fatalf("malformed system.dat was assigned type %q", info.FileType)
	}
}
