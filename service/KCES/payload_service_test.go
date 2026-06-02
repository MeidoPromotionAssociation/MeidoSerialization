package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	if err := service.ConvertPayloadToJson(inputPath, jsonPath); err != nil {
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

	if err := service.ConvertJsonToPayload(jsonPath, outPath); err != nil {
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

func TestPayloadService_ColliderPackageRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample.dbcol")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out.dbcol")

	env := &serializationKCES.KCESPayloadEnvelope{
		Format:         serializationKCES.PayloadFormatKCESMessagePack,
		Extension:      ".dbcol",
		LengthPrefixed: true,
		Kind:           serializationKCES.PayloadKindColliderPackage,
		ColliderPackage: &serializationKCES.ColliderPackage{
			Version: 1000,
			Colliders: []serializationKCES.ColliderRef{{
				Type: 2,
				Collider: serializationKCES.ColliderObject{
					Version:       1000,
					ParentName:    "Bip01 Neck",
					SelfName:      "Collider",
					LocalRotation: serializationKCES.Vector4{W: 1},
					LocalScale:    serializationKCES.Vector3{X: 1, Y: 1, Z: 1},
					Tail:          []interface{}{int64(0), 0.05},
				},
			}},
			States: []serializationKCES.ColliderState{{Version: 1000, Index: 0, Enabled: true}},
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
	if err := service.ConvertPayloadToJson(inputPath, jsonPath); err != nil {
		t.Fatalf("ConvertPayloadToJson: %v", err)
	}
	var decodedJSON serializationKCES.KCESPayloadEnvelope
	if err := json.Unmarshal(mustReadServiceTestFile(t, jsonPath), &decodedJSON); err != nil {
		t.Fatalf("unmarshal payload json: %v", err)
	}
	if decodedJSON.Kind != serializationKCES.PayloadKindColliderPackage || decodedJSON.ColliderPackage == nil {
		t.Fatalf("unexpected JSON envelope: %+v", decodedJSON)
	}

	if err := service.ConvertJsonToPayload(jsonPath, outputPath); err != nil {
		t.Fatalf("ConvertJsonToPayload: %v", err)
	}
	roundTrip, err := serializationKCES.DecodeKCESPayload(mustReadServiceTestFile(t, outputPath), ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload output: %v", err)
	}
	if roundTrip.ColliderPackage == nil || roundTrip.ColliderPackage.Colliders[0].Collider.ParentName != "Bip01 Neck" {
		t.Fatalf("unexpected round-trip collider package: %+v", roundTrip)
	}
}

func TestPayloadService_ClothParamsRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "sample.dsbconf")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(tmpDir, "out.dsbconf")

	env := &serializationKCES.KCESPayloadEnvelope{
		Format:         serializationKCES.PayloadFormatKCESMessagePack,
		Extension:      ".dsbconf",
		LengthPrefixed: true,
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
	if err := service.ConvertPayloadToJson(inputPath, jsonPath); err != nil {
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

	if err := service.ConvertJsonToPayload(jsonPath, outputPath); err != nil {
		t.Fatalf("ConvertJsonToPayload: %v", err)
	}
	roundTrip, err := serializationKCES.DecodeKCESPayload(mustReadServiceTestFile(t, outputPath), ".dsbconf")
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
					if err := service.ConvertPayloadToJson(sample, jsonPath); err != nil {
						t.Fatalf("ConvertPayloadToJson: %v", err)
					}
					if err := service.ConvertJsonToPayload(jsonPath, outPath); err != nil {
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
	for _, ext := range []string{".dbconf", ".dbcol", ".db2conf", ".dsbconf", ".dsb2conf", ".dsl2conf", ".dslcol", ".ikcol", ".limbcol"} {
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
