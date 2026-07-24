package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestPresetService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.preset")
	if err != nil {
		t.Fatal(err)
	}

	s := &PresetService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, "test.preset")
			jsonPath := filepath.Join(tempDir, "test.preset.json")
			backPath := filepath.Join(tempDir, "test_back.preset")

			// 1. Test ReadPresetFile
			preset, err := s.ReadPresetFile(inputPath)
			if err != nil {
				t.Fatalf("ReadPresetFile failed: %v", err)
			}
			if preset == nil {
				t.Fatal("ReadPresetFile returned nil")
			}

			// 2. Test WritePresetFile
			err = s.WritePresetFile(outputPath, preset)
			if err != nil {
				t.Fatalf("WritePresetFile failed: %v", err)
			}

			// 3. Test ConvertPresetToJson
			err = s.ConvertPresetToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput)
			if err != nil {
				t.Fatalf("ConvertPresetToJson failed: %v", err)
			}

			// 4. Test ConvertJsonToPreset
			err = s.ConvertJsonToPreset(TestConversionContext, jsonPath, backPath, TestConversionMaxOutput)
			if err != nil {
				t.Fatalf("ConvertJsonToPreset failed: %v", err)
			}

			// Re-read and verify consistency
			presetBack, err := s.ReadPresetFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted preset failed: %v", err)
			}
			if !reflect.DeepEqual(preset, presetBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			presetRepack, err := s.ReadPresetFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written preset failed: %v", err)
			}
			if !reflect.DeepEqual(preset, presetRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}

func TestPresetServiceExpandsCOM3D25EmbeddedPresetData(t *testing.T) {
	crcOpaque, err := serializationKCES.NewKCESPreset()
	if err != nil {
		t.Fatalf("NewKCESPreset: %v", err)
	}
	crcOpaque.ContainerVersion = 901
	crcOpaque.MaidData.Version = 902
	crcPreset, err := serializationKCES.ExpandKCESPreset(crcOpaque)
	if err != nil {
		t.Fatalf("ExpandKCESPreset: %v", err)
	}
	colorPresetID := "embedded-color"
	warpointName := "future-warpoint"
	flagSource := "COM3D2.5"
	embeddedColorPreset, err := serializationKCES.NewColorPreset("12345678-1234-1234-1234-123456789abc")
	if err != nil {
		t.Fatalf("NewColorPreset: %v", err)
	}
	crcPreset.MaidData.PropData.Properties = []serializationKCES.KCESPresetNamedProperty{{
		Key: "hairf",
		Property: serializationKCES.KCESPresetProperty{
			Signature: serializationKCES.KCESPresetPropertySignature,
			Version:   serializationKCES.KCESPresetPropertyVersion,
			Name:      "hairf",
			Base: serializationKCES.KCESPresetPropBase{
				Type:    "None",
				SubType: "None",
				EditBaseData: &serializationKCES.KCESPresetEditBaseData{
					Version: serializationKCES.KCESPresetEditBaseDataVersion,
					ColorPreset: &serializationKCES.KCESPresetEditColorPreset{
						ID:               &colorPresetID,
						SerializedPreset: embeddedColorPreset,
					},
					Flags: map[string]*string{"source": &flagSource},
				},
				SubProperties: []*serializationKCES.KCESPresetSubProperty{{
					Number:                    1,
					DefaultHokuroTattooSlotID: "none",
					EditUnitData: &serializationKCES.KCESPresetEditUnitData{
						Version:      serializationKCES.KCESPresetEditUnitDataVersion,
						PositionX:    1.25,
						PositionY:    -2.5,
						WarpointName: &warpointName,
					},
					Base: serializationKCES.KCESPresetPropBase{Type: "None", SubType: "None"},
				}},
			},
		},
	}}

	value := &serializationCOM3D2.Preset{
		Signature:  serializationCOM3D2.PresetSignature,
		Version:    34800,
		PresetType: serializationCOM3D2.PresetTypeAll,
		PresetPropertyList: &serializationCOM3D2.PresetPropertyList{
			Signature:        serializationCOM3D2.PresetPropertyListSignature,
			Version:          2002,
			PresetProperties: map[string]serializationCOM3D2.PresetProperty{},
			PartsColorOther: &serializationCOM3D2.MultiColor{
				Signature: serializationCOM3D2.MultiColorSignature,
				Version:   34800,
				PartCount: 9,
				PartNames: []string{"EYE_L"},
				PartsColors: []serializationCOM3D2.PartsColor{{
					IsUse: true, MainHue: 123, ShadowContrast: 456,
				}},
			},
			CRCPreset: crcPreset,
		},
		MultiColor: &serializationCOM3D2.MultiColor{
			Signature: serializationCOM3D2.MultiColorSignature,
			Version:   34800,
		},
		BodyProperty: &serializationCOM3D2.BodyProperty{
			Signature: serializationCOM3D2.BodyPropertySignature,
			Version:   34800,
		},
	}

	dir := t.TempDir()
	nativePath := filepath.Join(dir, "typed.preset")
	jsonPath := nativePath + ".json"
	backPath := filepath.Join(dir, "typed-back.preset")
	service := &PresetService{}
	if err := service.WritePresetFile(nativePath, value); err != nil {
		t.Fatalf("WritePresetFile: %v", err)
	}
	if err := service.ConvertPresetToJson(TestConversionContext, nativePath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertPresetToJson: %v", err)
	}
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"PartsColorOtherBin", "CRCPresetBin", "serializeBinary", `"propData":"`, `"colorData":"`, `"bodyData":"`, `"editBaseData":"`, `"editUnitData":"`} {
		if bytes.Contains(jsonData, []byte(forbidden)) {
			t.Fatalf("COM3D2.5 preset JSON contains unresolved field %q: %s", forbidden, jsonData)
		}
	}
	for _, required := range []string{`"PartsColorOther":{`, `"CRCPreset":{`, `"propData":{`, `"colorData":{`, `"bodyData":{`, `"editBaseData":{`, `"editUnitData":{`, `"serializedPreset":{`} {
		if !bytes.Contains(jsonData, []byte(required)) {
			t.Fatalf("COM3D2.5 preset JSON lacks typed field %s: %s", required, jsonData)
		}
	}
	if err := service.ConvertJsonToPreset(TestConversionContext, jsonPath, backPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToPreset: %v", err)
	}
	back, err := service.ReadPresetFile(backPath)
	if err != nil {
		t.Fatalf("ReadPresetFile(back): %v", err)
	}
	if back.PresetPropertyList.PartsColorOther == nil || back.PresetPropertyList.PartsColorOther.PartsColors[0].ShadowContrast != 456 {
		t.Fatalf("PartsColorOther round trip = %+v", back.PresetPropertyList.PartsColorOther)
	}
	if back.PresetPropertyList.CRCPreset == nil || back.PresetPropertyList.CRCPreset.ContainerVersion != 901 || back.PresetPropertyList.CRCPreset.MaidData.Version != 902 || back.PresetPropertyList.CRCPreset.MaidData.PropData == nil {
		t.Fatalf("CRCPreset round trip = %+v", back.PresetPropertyList.CRCPreset)
	}
}
