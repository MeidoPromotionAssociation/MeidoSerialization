package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func TestKCES2PartServicesDeriveLookupFieldsFromDestinationPath(t *testing.T) {
	exportedGUID := "ABCDEF01-2345-6789-ABCD-EF0123456789"
	menuSourceName := "stale.kcmenu"
	menu := serializationKCES.NewKCES2Menu()
	menu.FileName = &menuSourceName
	menu.ID = 1
	menu.GUID = 2
	menu.HairMake = serializationKCES.NewHairMake()
	menu.HairMake.ExportedGUID = &exportedGUID

	materialSourceName := "stale-material"
	material := serializationKCES.NewKCES2Material()
	material.FileName = &materialSourceName
	material.ID = 3

	modelSourceName := "stale-model"
	model := serializationKCES.NewModel()
	model.FileName = &modelSourceName
	model.ID = 4

	tests := []struct {
		name        string
		outputName  string
		fileType    string
		version     int32
		value       any
		decode      func([]byte) (any, error)
		assertValue func(*testing.T, any, string)
		assertInput func(*testing.T)
	}{
		{
			name:       "kcmenu",
			outputName: "Mixed.KCMENU",
			fileType:   "kcmenu",
			version:    1005,
			value:      menu,
			decode: func(data []byte) (any, error) {
				return serializationKCES.DecodeKCMenu(data)
			},
			assertValue: func(t *testing.T, value any, path string) {
				t.Helper()
				actual := value.(*serializationKCES.Menu)
				wantFileName := filepath.Base(path)
				if actual.FileName == nil || *actual.FileName != wantFileName {
					t.Fatalf("fileName = %v, want %q", actual.FileName, wantFileName)
				}
				if got, want := actual.ID, ct.HashStringIgnoreCase(wantFileName); got != want {
					t.Fatalf("ID = %d, want %d", got, want)
				}
				if got, want := actual.GUID, ct.HashStringIgnoreCase(exportedGUID); got != want {
					t.Fatalf("GUID = %d, want %d", got, want)
				}
			},
			assertInput: func(t *testing.T) {
				t.Helper()
				if menu.FileName == nil || *menu.FileName != menuSourceName || menu.ID != 1 || menu.GUID != 2 {
					t.Fatalf("input menu was mutated: %+v", menu)
				}
			},
		},
		{
			name:       "kcmat",
			outputName: "Material.Name.KCMAT",
			fileType:   "kcmat",
			version:    1000,
			value:      material,
			decode: func(data []byte) (any, error) {
				return serializationKCES.DecodeKCMat(data)
			},
			assertValue: func(t *testing.T, value any, path string) {
				t.Helper()
				actual := value.(*serializationKCES.Material)
				fileName := filepath.Base(path)
				wantFileName := fileName[:len(fileName)-len(filepath.Ext(fileName))]
				if actual.FileName == nil || *actual.FileName != wantFileName {
					t.Fatalf("fileName = %v, want %q", actual.FileName, wantFileName)
				}
				if got, want := actual.ID, ct.HashString(wantFileName); got != want {
					t.Fatalf("ID = %d, want %d", got, want)
				}
			},
			assertInput: func(t *testing.T) {
				t.Helper()
				if material.FileName == nil || *material.FileName != materialSourceName || material.ID != 3 {
					t.Fatalf("input material was mutated: %+v", material)
				}
			},
		},
		{
			name:       "kcmodel",
			outputName: "Model.Name.KCMODEL",
			fileType:   "kcmodel",
			version:    1001,
			value:      model,
			decode: func(data []byte) (any, error) {
				return serializationKCES.DecodeKCModel(data)
			},
			assertValue: func(t *testing.T, value any, path string) {
				t.Helper()
				actual := value.(*serializationKCES.Model)
				fileName := filepath.Base(path)
				wantFileName := strings.ToLower(fileName[:len(fileName)-len(filepath.Ext(fileName))])
				if actual.FileName == nil || *actual.FileName != wantFileName {
					t.Fatalf("fileName = %v, want %q", actual.FileName, wantFileName)
				}
				if got, want := actual.ID, ct.HashString(wantFileName); got != want {
					t.Fatalf("ID = %d, want %d", got, want)
				}
			},
			assertInput: func(t *testing.T) {
				t.Helper()
				if model.FileName == nil || *model.FileName != modelSourceName || model.ID != 4 {
					t.Fatalf("input model was mutated: %+v", model)
				}
			},
		},
	}

	parts := &PartsService{}
	fileTypeService := &FileTypeService{}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, test.outputName)
			if !IsKCESPartsFile(outputPath) {
				t.Fatalf("IsKCESPartsFile rejected %s", outputPath)
			}
			if err := parts.WritePartsFile(outputPath, test.value); err != nil {
				t.Fatalf("WritePartsFile: %v", err)
			}
			wire, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			value, err := test.decode(wire)
			if err != nil {
				t.Fatal(err)
			}
			test.assertValue(t, value, outputPath)
			test.assertInput(t)

			info, matched, err := fileTypeService.TryFileTypeDetermine(outputPath)
			if err != nil || !matched {
				t.Fatalf("TryFileTypeDetermine: matched=%v info=%+v err=%v", matched, info, err)
			}
			if info.FileType != test.fileType || info.Game != COM3D2Service.GameKCES || info.Version != test.version {
				t.Fatalf("file type = %+v, want type=%q game=%q version=%d", info, test.fileType, COM3D2Service.GameKCES, test.version)
			}

			editingJSON, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			jsonPath := outputPath + ".json"
			if err := os.WriteFile(jsonPath, editingJSON, 0644); err != nil {
				t.Fatal(err)
			}
			if !IsKCESPartsJSONFile(jsonPath) {
				t.Fatalf("IsKCESPartsJSONFile rejected %s", jsonPath)
			}
			jsonInfo, matched, err := fileTypeService.TryFileTypeDetermine(jsonPath)
			if err != nil || !matched {
				t.Fatalf("TryFileTypeDetermine JSON: matched=%v info=%+v err=%v", matched, jsonInfo, err)
			}
			if jsonInfo.FileType != test.fileType || jsonInfo.StorageFormat != COM3D2Service.FormatJSON || jsonInfo.Game != COM3D2Service.GameKCES {
				t.Fatalf("JSON file type = %+v, want type=%q format=%q game=%q", jsonInfo, test.fileType, COM3D2Service.FormatJSON, COM3D2Service.GameKCES)
			}
			convertedPath := filepath.Join(dir, "converted_"+test.outputName)
			if err := parts.ConvertJsonToParts(TestConversionContext, jsonPath, convertedPath, TestConversionMaxOutput); err != nil {
				t.Fatalf("ConvertJsonToParts: %v", err)
			}
			convertedWire, err := os.ReadFile(convertedPath)
			if err != nil {
				t.Fatal(err)
			}
			converted, err := test.decode(convertedWire)
			if err != nil {
				t.Fatal(err)
			}
			test.assertValue(t, converted, convertedPath)
			test.assertInput(t)
		})
	}
}
