package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestIndependentPayloadServicesRoundTrip(t *testing.T) {
	tests := []struct {
		extension string
		toJSON    func(inputPath string, outputPath string) error
		toNative  func(inputPath string, outputPath string) error
	}{
		{serializationKCES.KCESDBConfExtension, func(inputPath string, outputPath string) error {
			return (&DBConfService{}).ConvertDBConfToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&DBConfService{}).ConvertJsonToDBConf(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESDBColExtension, func(inputPath string, outputPath string) error {
			return (&DBColService{}).ConvertDBColToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&DBColService{}).ConvertJsonToDBCol(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESDB2ConfExtension, func(inputPath string, outputPath string) error {
			return (&DB2ConfService{}).ConvertDB2ConfToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&DB2ConfService{}).ConvertJsonToDB2Conf(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESDSBConfExtension, func(inputPath string, outputPath string) error {
			return (&DSBConfService{}).ConvertDSBConfToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&DSBConfService{}).ConvertJsonToDSBConf(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESDSB2ConfExtension, func(inputPath string, outputPath string) error {
			return (&DSB2ConfService{}).ConvertDSB2ConfToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&DSB2ConfService{}).ConvertJsonToDSB2Conf(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESDSLConfExtension, func(inputPath string, outputPath string) error {
			return (&DSLConfService{}).ConvertDSLConfToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&DSLConfService{}).ConvertJsonToDSLConf(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESDSL2ConfExtension, func(inputPath string, outputPath string) error {
			return (&DSL2ConfService{}).ConvertDSL2ConfToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&DSL2ConfService{}).ConvertJsonToDSL2Conf(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESDSLColExtension, func(inputPath string, outputPath string) error {
			return (&DSLColService{}).ConvertDSLColToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&DSLColService{}).ConvertJsonToDSLCol(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESIKColExtension, func(inputPath string, outputPath string) error {
			return (&IKColService{}).ConvertIKColToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&IKColService{}).ConvertJsonToIKCol(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESIKColBytesExtension, func(inputPath string, outputPath string) error {
			return (&IKColBytesService{}).ConvertIKColBytesToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&IKColBytesService{}).ConvertJsonToIKColBytes(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
		{serializationKCES.KCESLimbColExtension, func(inputPath string, outputPath string) error {
			return (&LimbColService{}).ConvertLimbColToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}, func(inputPath string, outputPath string) error {
			return (&LimbColService{}).ConvertJsonToLimbCol(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.extension, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input"+test.extension)
			jsonPath := inputPath + ".json"
			outputPath := filepath.Join(dir, "output"+test.extension)
			envelope := independentPayloadEnvelope(t, test.extension)
			wire, err := serializationKCES.EncodeKCESPayload(envelope)
			if err != nil {
				t.Fatalf("EncodeKCESPayload: %v", err)
			}
			if err := os.WriteFile(inputPath, wire, 0644); err != nil {
				t.Fatal(err)
			}
			if err := test.toJSON(inputPath, jsonPath); err != nil {
				t.Fatalf("convert to JSON: %v", err)
			}
			var editing serializationKCES.KCESPayloadEnvelope
			if err := json.Unmarshal(mustReadServiceTestFile(t, jsonPath), &editing); err != nil {
				t.Fatalf("decode editing JSON: %v", err)
			}
			if editing.Extension != test.extension {
				t.Fatalf("editing extension = %q", editing.Extension)
			}
			if err := test.toNative(jsonPath, outputPath); err != nil {
				t.Fatalf("convert to native: %v", err)
			}
			decoded, err := serializationKCES.DecodeKCESPayload(mustReadServiceTestFile(t, outputPath), test.extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload: %v", err)
			}
			if decoded.Extension != test.extension || decoded.Kind != envelope.Kind {
				t.Fatalf("decoded envelope = %+v", decoded)
			}
		})
	}
}

func TestIndependentPartsServicesRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		samplePath string
		extension  string
		toJSON     func(inputPath string, outputPath string) error
		toNative   func(inputPath string, outputPath string) error
		readNative func(path string) error
	}{
		{
			name:       "menuassets",
			samplePath: filepath.Join("..", "..", "testdata", "kces_parts", "parts_personal002.menuassets"),
			extension:  menuAssetsExtension,
			toJSON: func(inputPath string, outputPath string) error {
				return (&MenuAssetsService{}).ConvertMenuAssetsToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			toNative: func(inputPath string, outputPath string) error {
				return (&MenuAssetsService{}).ConvertJsonToMenuAssets(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			readNative: func(path string) error {
				_, err := (&MenuAssetsService{}).ReadMenuAssetsFile(path)
				return err
			},
		},
		{
			name:       "materialassets",
			samplePath: filepath.Join("..", "..", "testdata", "kces_parts", "parts_personal002.materialassets"),
			extension:  materialAssetsExtension,
			toJSON: func(inputPath string, outputPath string) error {
				return (&MaterialAssetsService{}).ConvertMaterialAssetsToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			toNative: func(inputPath string, outputPath string) error {
				return (&MaterialAssetsService{}).ConvertJsonToMaterialAssets(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			readNative: func(path string) error {
				_, err := (&MaterialAssetsService{}).ReadMaterialAssetsFile(path)
				return err
			},
		},
		{
			name:       "pmatassets",
			samplePath: filepath.Join("..", "..", "testdata", "kces_parts", "partsmeta.pmatassets"),
			extension:  priorityMaterialAssetsExtension,
			toJSON: func(inputPath string, outputPath string) error {
				return (&PriorityMaterialAssetsService{}).ConvertPriorityMaterialAssetsToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			toNative: func(inputPath string, outputPath string) error {
				return (&PriorityMaterialAssetsService{}).ConvertJsonToPriorityMaterialAssets(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			readNative: func(path string) error {
				_, err := (&PriorityMaterialAssetsService{}).ReadPriorityMaterialAssetsFile(path)
				return err
			},
		},
		{
			name:       "model",
			samplePath: filepath.Join("..", "..", "testdata", "kces_parts", "hair_twin019.model"),
			extension:  modelExtension,
			toJSON: func(inputPath string, outputPath string) error {
				return (&ModelService{}).ConvertModelToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			toNative: func(inputPath string, outputPath string) error {
				return (&ModelService{}).ConvertJsonToModel(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			readNative: func(path string) error {
				_, err := (&ModelService{}).ReadModelFile(path)
				return err
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if _, err := os.Stat(test.samplePath); err != nil {
				t.Fatalf("fixed sample missing: %v", err)
			}
			dir := t.TempDir()
			jsonPath := filepath.Join(dir, "editing"+test.extension+".json")
			outputPath := filepath.Join(dir, "output"+test.extension)
			if err := test.toJSON(test.samplePath, jsonPath); err != nil {
				t.Fatalf("convert to JSON: %v", err)
			}
			if err := test.toNative(jsonPath, outputPath); err != nil {
				t.Fatalf("convert to native: %v", err)
			}
			if err := test.readNative(outputPath); err != nil {
				t.Fatalf("read native output: %v", err)
			}
		})
	}
}

func TestIndependentMiscServicesRoundTrip(t *testing.T) {
	hitCheckWire, err := serializationKCES.EncodeHitCheck(serializationKCES.NewHitCheck())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		extension  string
		wire       []byte
		toJSON     func(inputPath string, outputPath string) error
		toNative   func(inputPath string, outputPath string) error
		readNative func(path string) error
	}{
		{
			name:      "hitcheck",
			extension: hitCheckExtension,
			wire:      hitCheckWire,
			toJSON: func(inputPath string, outputPath string) error {
				return (&HitCheckService{}).ConvertHitCheckToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			toNative: func(inputPath string, outputPath string) error {
				return (&HitCheckService{}).ConvertJsonToHitCheck(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			readNative: func(path string) error {
				_, err := (&HitCheckService{}).ReadHitCheckFile(path)
				return err
			},
		},
		{
			name:      "undressdat",
			extension: serializationKCES.KCESUndressDataExtension,
			wire:      []byte(`{"editVer":13,"items":["a"]}`),
			toJSON: func(inputPath string, outputPath string) error {
				return (&UndressDataService{}).ConvertUndressDataToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			toNative: func(inputPath string, outputPath string) error {
				return (&UndressDataService{}).ConvertJsonToUndressData(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			readNative: func(path string) error {
				_, err := (&UndressDataService{}).ReadUndressDataFile(path)
				return err
			},
		},
		{
			name:      "undresspdat",
			extension: serializationKCES.KCESUndressPartsDataExtension,
			wire:      []byte(`{"editVer":13,"parts":["body"]}`),
			toJSON: func(inputPath string, outputPath string) error {
				return (&UndressPartsDataService{}).ConvertUndressPartsDataToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			toNative: func(inputPath string, outputPath string) error {
				return (&UndressPartsDataService{}).ConvertJsonToUndressPartsData(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			readNative: func(path string) error {
				_, err := (&UndressPartsDataService{}).ReadUndressPartsDataFile(path)
				return err
			},
		},
		{
			name:      "nson",
			extension: serializationKCES.KCESNSONExtension,
			wire:      []byte(`{"version":1000,"_ids":[1,2]}`),
			toJSON: func(inputPath string, outputPath string) error {
				return (&NSONService{}).ConvertNSONToJson(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			toNative: func(inputPath string, outputPath string) error {
				return (&NSONService{}).ConvertJsonToNSON(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			},
			readNative: func(path string) error {
				_, err := (&NSONService{}).ReadNSONFile(path)
				return err
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input"+test.extension)
			jsonPath := inputPath + ".json"
			outputPath := filepath.Join(dir, "output"+test.extension)
			if err := os.WriteFile(inputPath, test.wire, 0644); err != nil {
				t.Fatal(err)
			}
			if err := test.toJSON(inputPath, jsonPath); err != nil {
				t.Fatalf("convert to JSON: %v", err)
			}
			if err := test.toNative(jsonPath, outputPath); err != nil {
				t.Fatalf("convert to native: %v", err)
			}
			if err := test.readNative(outputPath); err != nil {
				t.Fatalf("read native output: %v", err)
			}
		})
	}
}

func TestIndependentSharedDataServicesRoundTrip(t *testing.T) {
	t.Run("psk", func(t *testing.T) {
		samplePath := filepath.Join("..", "..", "testdata", "kces_assets", "default_skirt.psk")
		dir := t.TempDir()
		jsonPath := filepath.Join(dir, "default_skirt.psk.json")
		outputPath := filepath.Join(dir, "default_skirt.psk")
		service := &PskService{}
		if err := service.ConvertPskToJson(TestConversionContext, samplePath, jsonPath, TestConversionMaxOutput); err != nil {
			t.Fatal(err)
		}
		if err := service.ConvertJsonToPsk(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
			t.Fatal(err)
		}
		if _, err := service.ReadPskFile(outputPath); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("nei", func(t *testing.T) {
		samplePath := filepath.Join("..", "..", "testdata", "kces_assets", "edit_pose_enabled_list.nei")
		dir := t.TempDir()
		csvPath := filepath.Join(dir, "edit_pose_enabled_list.csv")
		outputPath := filepath.Join(dir, "edit_pose_enabled_list.nei")
		service := &NeiService{}
		if err := service.ConvertNeiToCSV(samplePath, csvPath); err != nil {
			t.Fatal(err)
		}
		if err := service.ConvertCSVToNei(csvPath, outputPath); err != nil {
			t.Fatal(err)
		}
		if _, err := service.ReadNeiFile(outputPath); err != nil {
			t.Fatal(err)
		}
	})
}

func independentPayloadEnvelope(t *testing.T, extension string) *serializationKCES.KCESPayloadEnvelope {
	t.Helper()
	descriptor, ok := serializationKCES.DescribeKCESPayload(extension)
	if !ok {
		t.Fatalf("missing payload descriptor for %s", extension)
	}
	value := &serializationKCES.KCESPayloadEnvelope{
		Format:         serializationKCES.PayloadFormatKCESMessagePack,
		Extension:      descriptor.Extension,
		StorageVariant: serializationKCES.PayloadStorageInt32LZ4MessagePack,
		Kind:           descriptor.Kind,
	}
	switch descriptor.Kind {
	case serializationKCES.PayloadKindDynamicBoneStatus:
		value.DynamicBone = serializationKCES.NewDynamicBoneStatus()
	case serializationKCES.PayloadKindJSONString:
		value.JSON = json.RawMessage(`{"version":1000}`)
	case serializationKCES.PayloadKindColliderPackage:
		value.ColliderPackage = &serializationKCES.ColliderPackage{Version: 1000, Colliders: []*serializationKCES.ColliderRef{}, LimbEnableList: []*serializationKCES.ColliderState{}}
	case serializationKCES.PayloadKindLimbCollider:
		value.LimbCollider = &serializationKCES.LimbColliderPackage{Version: 1000, Items: []*serializationKCES.LimbColliderItem{}}
	case serializationKCES.PayloadKindIKCollider:
		value.IKCollider = &serializationKCES.IKColliderPackage{Version: 1000, Groups: []*serializationKCES.IKColliderGroup{}}
	case serializationKCES.PayloadKindClothParams:
		value.ClothParams = serializationKCES.NewClothParams()
	default:
		t.Fatalf("unsupported native payload kind %q", descriptor.Kind)
	}
	return value
}

func TestPersetServiceDirectRoundTrip(t *testing.T) {
	core := mustKCESPresetCoreForServiceTest(t)
	name := "direct-perset"
	wire, err := serializationKCES.EncodeKCESPreset(&serializationKCES.KCESPreset{
		ContainerVersion: 1000,
		Thumbnail:        []byte("png"),
		MaidData:         core,
		Meta:             &serializationKCES.KCESPresetMeta{Version: 1000, Data: map[string]*string{"presetName": &name}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.perset")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(dir, "output.perset")
	if err := os.WriteFile(inputPath, wire, 0644); err != nil {
		t.Fatal(err)
	}
	service := &PersetService{}
	if err := service.ConvertPersetToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	if err := service.ConvertJsonToPerset(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReadPersetFile(outputPath); err != nil {
		t.Fatal(err)
	}
}

func TestExtensionSpecificUnityFSReaders(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "parts_personal_om015_gp003.aba")
	data, err := os.ReadFile(sample)
	if err != nil {
		t.Skipf("sample not available: %v", err)
	}
	dir := t.TempDir()
	assetBGPath := filepath.Join(dir, "sample.asset_bg")
	assetScenePath := filepath.Join(dir, "sample.asset_scene")
	if err := os.WriteFile(assetBGPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetScenePath, data, 0644); err != nil {
		t.Fatal(err)
	}
	_, assetBGFile, err := (&AssetBGService{}).ReadAssetBG(assetBGPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := assetBGFile.Close(); err != nil {
		t.Fatal(err)
	}
	_, assetSceneFile, err := (&AssetSceneService{}).ReadAssetScene(assetScenePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := assetSceneFile.Close(); err != nil {
		t.Fatal(err)
	}
}
