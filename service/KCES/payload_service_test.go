package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestPayloadService_DynamicBoneJSONRoundTrip(t *testing.T) {
	input, err := serializationKCES.EncodeDynamicBoneStatusFile(&serializationKCES.DynamicBoneStatus{
		Version:    1000,
		Damping:    0.5,
		Elasticity: 0.25,
		Gravity:    serializationKCES.Vector3{Y: -0.05},
	})
	if err != nil {
		t.Fatalf("EncodeDynamicBoneStatusFile: %v", err)
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample.dbconf")
	jsonPath := inputPath + ".json"
	outPath := filepath.Join(tmpDir, "out.dbconf")
	if err := os.WriteFile(inputPath, input, 0644); err != nil {
		t.Fatal(err)
	}

	service := &PayloadService{}
	if err := service.ConvertPayloadToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertPayloadToJson: %v", err)
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var env serializationKCES.KCESPayloadEnvelope
	if err := json.Unmarshal(jsonData, &env); err != nil {
		t.Fatalf("parse json output: %v", err)
	}
	if env.Kind != serializationKCES.PayloadKindDynamicBoneStatus || env.DynamicBone == nil {
		t.Fatalf("unexpected payload envelope: %+v", env)
	}
	env.DynamicBone.Damping = 0.75
	edited, err := json.MarshalIndent(&env, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, edited, 0644); err != nil {
		t.Fatal(err)
	}

	if err := service.ConvertJsonToPayload(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToPayload: %v", err)
	}
	outData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := serializationKCES.DecodeDynamicBoneStatusFile(outData)
	if err != nil {
		t.Fatalf("DecodeDynamicBoneStatusFile: %v", err)
	}
	if decoded.Damping != 0.75 {
		t.Fatalf("edited damping got %v", decoded.Damping)
	}
}

func TestPayloadService_ExportCMJSONVariantsRoundTrip(t *testing.T) {
	dynamicJSON := []byte(`{"version":1000,"damping":0.6,"DampingKeyFrames":[],"elasticity":0.1,"ElasticityKeyFrames":[],"stiffness":0.1,"StiffnessKeyFrames":[],"inert":0,"InertKeyFrames":[],"radius":0,"RadiusKeyFrames":[],"endLength":0,"endOffset":{"x":0,"y":0,"z":0},"gravity":{"x":0,"y":-0.05,"z":0},"force":{"x":0,"y":0,"z":0},"freezeAxis":0}`)
	colliderJSON := []byte(`{"version":1000,"StatusJsonStrList":[],"limbEnableList":[]}`)
	tests := []struct {
		name        string
		extension   string
		wire        []byte
		wantStorage string
	}{
		{name: "dbconf", extension: ".dbconf", wire: dynamicJSON, wantStorage: serializationKCES.PayloadStorageExportCMUnityJSON},
		{name: "dbcol", extension: ".dbcol", wire: colliderJSON, wantStorage: serializationKCES.PayloadStorageExportCMUnityJSON},
		{name: "dslcol", extension: ".dslcol", wire: appendServiceDotNetString(nil, colliderJSON), wantStorage: serializationKCES.PayloadStorageExportCMDotNetStringJSON},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input"+test.extension)
			jsonPath := input + ".json"
			output := filepath.Join(dir, "output"+test.extension)
			if err := os.WriteFile(input, test.wire, 0644); err != nil {
				t.Fatal(err)
			}

			service := &PayloadService{}
			if err := service.ConvertPayloadToJson(TestConversionContext, input, jsonPath, TestConversionMaxOutput); err != nil {
				t.Fatalf("ConvertPayloadToJson() error = %v", err)
			}
			var envelope serializationKCES.KCESPayloadEnvelope
			if err := json.Unmarshal(mustReadServiceTestFile(t, jsonPath), &envelope); err != nil {
				t.Fatalf("unmarshal editing JSON: %v", err)
			}
			if envelope.Format != serializationKCES.PayloadFormatKCESExportCM || envelope.StorageVariant != test.wantStorage {
				t.Fatalf("editing envelope format/storage = %q/%q", envelope.Format, envelope.StorageVariant)
			}

			if err := service.ConvertJsonToPayload(TestConversionContext, jsonPath, output, TestConversionMaxOutput); err != nil {
				t.Fatalf("ConvertJsonToPayload() error = %v", err)
			}
			if got := mustReadServiceTestFile(t, output); !reflect.DeepEqual(got, test.wire) {
				t.Fatalf("service round trip changed ExportCM wire:\n got  %x\n want %x", got, test.wire)
			}
		})
	}
}

func appendServiceDotNetString(dst, value []byte) []byte {
	length := len(value)
	for length >= 0x80 {
		dst = append(dst, byte(length)|0x80)
		length >>= 7
	}
	dst = append(dst, byte(length))
	return append(dst, value...)
}

func TestPayloadService_ColliderPackageRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample.dbcol")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out.dbcol")

	env := &serializationKCES.KCESPayloadEnvelope{
		Format:         serializationKCES.PayloadFormatKCESMessagePack,
		Extension:      ".dbcol",
		LengthPrefixed: true,
		StorageVariant: serializationKCES.PayloadStorageInt32LZ4MessagePack,
		Kind:           serializationKCES.PayloadKindColliderPackage,
		ColliderPackage: &serializationKCES.ColliderPackage{
			Version: 1000,
			Colliders: []serializationKCES.ColliderRef{{
				Type: 2,
				Collider: &serializationKCES.ColliderSphere{
					ColliderObject: serializationKCES.ColliderObject{
						Version:       1000,
						ParentName:    "Bip01 Neck",
						SelfName:      "Collider",
						LocalRotation: serializationKCES.Vector4{W: 1},
						LocalScale:    serializationKCES.Vector3{X: 1, Y: 1, Z: 1},
						Bound:         serializationKCES.ColliderBoundOutside,
					},
					Radius: 0.05,
				},
			}},
			LimbEnableList: []serializationKCES.ColliderState{{Version: 1000, LimbType: 0, IsEnable: true}},
		},
	}
	encoded, err := serializationKCES.EncodeKCESPayload(env)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	service := &PayloadService{}
	if err := service.ConvertPayloadToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertPayloadToJson: %v", err)
	}
	jsonBytes := mustReadServiceTestFile(t, jsonPath)
	if !strings.Contains(string(jsonBytes), `"limbEnableList"`) || strings.Contains(string(jsonBytes), `"states"`) {
		t.Fatalf("collider JSON should use limbEnableList, got %s", string(jsonBytes))
	}
	var decodedJSON serializationKCES.KCESPayloadEnvelope
	if err := json.Unmarshal(jsonBytes, &decodedJSON); err != nil {
		t.Fatalf("unmarshal payload json: %v", err)
	}
	if decodedJSON.Kind != serializationKCES.PayloadKindColliderPackage || decodedJSON.ColliderPackage == nil {
		t.Fatalf("unexpected JSON envelope: %+v", decodedJSON)
	}
	if len(decodedJSON.ColliderPackage.LimbEnableList) != 1 {
		t.Fatalf("limb enable list not populated from JSON: %+v", decodedJSON.ColliderPackage)
	}

	if err := service.ConvertJsonToPayload(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToPayload: %v", err)
	}
	roundTrip, err := serializationKCES.DecodeKCESPayload(mustReadServiceTestFile(t, outputPath), ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload output: %v", err)
	}
	c0, ok := roundTrip.ColliderPackage.Colliders[0].Collider.(*serializationKCES.ColliderSphere)
	if !ok || c0 == nil || c0.ParentName != "Bip01 Neck" {
		t.Fatalf("unexpected round-trip collider package: %+v", roundTrip)
	}
}

func TestPayloadService_ClothParamsRoundTrip(t *testing.T) {
	for _, ext := range []string{".dsbconf", ".dslconf"} {
		ext := ext
		t.Run(ext, func(t *testing.T) {
			testPayloadServiceClothParamsRoundTrip(t, ext)
		})
	}
}

func testPayloadServiceClothParamsRoundTrip(t *testing.T, ext string) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample"+ext)
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out"+ext)

	env := &serializationKCES.KCESPayloadEnvelope{
		Format:         serializationKCES.PayloadFormatKCESMessagePack,
		Extension:      ext,
		LengthPrefixed: true,
		StorageVariant: serializationKCES.PayloadStorageInt32LZ4MessagePack,
		Kind:           serializationKCES.PayloadKindClothParams,
		ClothParams: &serializationKCES.ClothParams{
			Radius:                         serializationKCES.BezierParam{StartValue: 0.02, EndValue: 0.04, UseEndValue: true},
			Mass:                           serializationKCES.BezierParam{StartValue: 1, EndValue: 1},
			UseGravity:                     true,
			Gravity:                        serializationKCES.BezierParam{StartValue: -9.8, EndValue: -9.8},
			UseDrag:                        true,
			Drag:                           serializationKCES.BezierParam{StartValue: 0.02, EndValue: 0.02, UseEndValue: true},
			UseMaxVelocity:                 true,
			MaxVelocity:                    serializationKCES.BezierParam{StartValue: 3, EndValue: 3},
			WorldMoveInfluence:             serializationKCES.BezierParam{StartValue: 0.5, EndValue: 0.5},
			WorldRotationInfluence:         serializationKCES.BezierParam{StartValue: 0.5, EndValue: 0.5},
			DisableDistance:                20,
			DisableFadeDistance:            5,
			UseClampDistanceRatio:          true,
			ClampDistanceMinRatio:          0.7,
			ClampDistanceMaxRatio:          1.1,
			UsePenetration:                 true,
			PenetrationMode:                serializationKCES.ClothPenetrationModeColliderPenetration,
			PenetrationAxis:                serializationKCES.ClothPenetrationAxisInverseZ,
			PenetrationConnectDistance:     serializationKCES.BezierParam{StartValue: 0.2, EndValue: 0.3, UseEndValue: true},
			PenetrationDistance:            serializationKCES.BezierParam{StartValue: 0.1, EndValue: 0.2, UseEndValue: true},
			PenetrationRadius:              serializationKCES.BezierParam{StartValue: 0.3, EndValue: 1, UseEndValue: true},
			UseLineAvarageRotation:         true,
			GravityDirection:               serializationKCES.Vector3{Y: 1},
			MaxMoveSpeed:                   10,
			MaxRotationSpeed:               360,
			ResetStabilizationTime:         0.1,
			ClampRotationVelocityLimit:     1,
			ClampRotationVelocityInfluence: 0.2,
		},
	}
	encoded, err := serializationKCES.EncodeKCESPayload(env)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	service := &PayloadService{}
	if err := service.ConvertPayloadToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertPayloadToJson: %v", err)
	}
	var decodedJSON serializationKCES.KCESPayloadEnvelope
	if err := json.Unmarshal(mustReadServiceTestFile(t, jsonPath), &decodedJSON); err != nil {
		t.Fatalf("unmarshal payload json: %v", err)
	}
	if decodedJSON.Kind != serializationKCES.PayloadKindClothParams || decodedJSON.ClothParams == nil {
		t.Fatalf("unexpected JSON envelope: %+v", decodedJSON)
	}
	decodedJSON.ClothParams.UsePenetration = false
	edited, err := json.MarshalIndent(&decodedJSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, edited, 0644); err != nil {
		t.Fatal(err)
	}

	if err := service.ConvertJsonToPayload(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToPayload: %v", err)
	}
	roundTrip, err := serializationKCES.DecodeKCESPayload(mustReadServiceTestFile(t, outputPath), ext)
	if err != nil {
		t.Fatalf("DecodeKCESPayload output: %v", err)
	}
	if roundTrip.ClothParams == nil || roundTrip.ClothParams.UsePenetration {
		t.Fatalf("unexpected round-trip cloth params: %+v", roundTrip)
	}
}

func TestPayloadService_FixedSamplesJSONRoundTrip(t *testing.T) {
	pathsByExt := fixedPayloadServiceSamplesByExt(t)
	service := &PayloadService{}
	for ext, paths := range pathsByExt {
		ext := ext
		paths := paths
		t.Run(ext, func(t *testing.T) {
			for _, sample := range paths {
				sample := sample
				t.Run(filepath.Base(sample), func(t *testing.T) {
					name := filepath.Base(sample)
					tmpDir := t.TempDir()
					jsonPath := filepath.Join(tmpDir, name+".json")
					outPath := filepath.Join(tmpDir, name)
					if err := service.ConvertPayloadToJson(TestConversionContext, sample, jsonPath, TestConversionMaxOutput); err != nil {
						t.Fatalf("ConvertPayloadToJson: %v", err)
					}
					if err := service.ConvertJsonToPayload(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
						t.Fatalf("ConvertJsonToPayload: %v", err)
					}

					want, err := serializationKCES.DecodeKCESPayload(mustReadServiceTestFile(t, sample), name)
					if err != nil {
						t.Fatalf("DecodeKCESPayload sample: %v", err)
					}
					got, err := serializationKCES.DecodeKCESPayload(mustReadServiceTestFile(t, outPath), name)
					if err != nil {
						t.Fatalf("DecodeKCESPayload output: %v", err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("service payload JSON round-trip changed %s: got %#v, want %#v", name, got, want)
					}
				})
			}
		})
	}
}

func fixedPayloadServiceSamplesByExt(t *testing.T) map[string][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "kces_payload", "*"))
	if err != nil {
		t.Fatalf("glob fixed payload samples: %v", err)
	}
	if len(paths) == 0 {
		t.Skip("no fixed payload samples found")
	}
	pathsByExt := map[string][]string{}
	for _, path := range paths {
		ext := serializationKCES.NormalizeKCESPayloadExtension(path)
		if ext == "" {
			t.Fatalf("unexpected fixed payload sample suffix for %s", filepath.Base(path))
		}
		pathsByExt[ext] = append(pathsByExt[ext], path)
	}
	if len(pathsByExt[".dslconf"]) == 0 {
		source := pathsByExt[".dsbconf"]
		if len(source) == 0 {
			t.Fatalf("cannot synthesize .dslconf sample without a .dsbconf ClothParams sample")
		}
		data, err := os.ReadFile(source[0])
		if err != nil {
			t.Fatalf("read .dsbconf sample for .dslconf coverage: %v", err)
		}
		dslconfPath := filepath.Join(t.TempDir(), "default_sleeve.dslconf")
		if err := os.WriteFile(dslconfPath, data, 0644); err != nil {
			t.Fatalf("write synthesized .dslconf coverage sample: %v", err)
		}
		pathsByExt[".dslconf"] = append(pathsByExt[".dslconf"], dslconfPath)
	}
	for _, ext := range []string{".dbconf", ".dbcol", ".db2conf", ".dsbconf", ".dsb2conf", ".dslconf", ".dsl2conf", ".dslcol", ".ikcol", ".limbcol"} {
		if len(pathsByExt[ext]) == 0 {
			t.Fatalf("no fixed payload samples with suffix %s", ext)
		}
	}
	return pathsByExt
}

func mustReadServiceTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
