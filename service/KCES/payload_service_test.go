package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
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
	var status serializationKCES.DynamicBoneStatus
	if err := json.Unmarshal(jsonData, &status); err != nil {
		t.Fatalf("parse json output: %v", err)
	}
	if status.Version != 1000 || status.Elasticity != 0.25 {
		t.Fatalf("unexpected payload root: %+v", status)
	}
	status.Damping = 0.75
	edited, err := json.MarshalIndent(&status, "", "  ")
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

func TestPayloadService_RejectsExportCMJSONVariants(t *testing.T) {
	dynamicJSON := []byte(`{"version":1000,"damping":0.6,"DampingKeyFrames":[],"elasticity":0.1,"ElasticityKeyFrames":[],"stiffness":0.1,"StiffnessKeyFrames":[],"inert":0,"InertKeyFrames":[],"radius":0,"RadiusKeyFrames":[],"endLength":0,"endOffset":{"x":0,"y":0,"z":0},"gravity":{"x":0,"y":-0.05,"z":0},"force":{"x":0,"y":0,"z":0},"freezeAxis":0}`)
	colliderJSON := []byte(`{"version":1000,"StatusJsonStrList":[],"limbEnableList":[]}`)
	tests := []struct {
		name      string
		extension string
		wire      []byte
	}{
		{name: "dbconf", extension: ".dbconf", wire: dynamicJSON},
		{name: "dbcol", extension: ".dbcol", wire: colliderJSON},
		{name: "dslcol", extension: ".dslcol", wire: appendServiceDotNetString(nil, colliderJSON)},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input"+test.extension)
			jsonPath := input + ".json"
			if err := os.WriteFile(input, test.wire, 0644); err != nil {
				t.Fatal(err)
			}

			service := &PayloadService{}
			err := service.ConvertPayloadToJson(TestConversionContext, input, jsonPath, TestConversionMaxOutput)
			if err == nil {
				t.Fatal("ConvertPayloadToJson() accepted an ExportCM sidecar")
			}
			if !strings.Contains(err.Error(), "ExportCM") {
				t.Fatalf("ConvertPayloadToJson() error = %v, want the ExportCM sidecar explanation", err)
			}
			if _, statErr := os.Stat(jsonPath); statErr == nil {
				t.Fatal("ConvertPayloadToJson() wrote editing JSON for a rejected ExportCM sidecar")
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

	pkg := &serializationKCES.ColliderPackage{
		Version: 1000,
		Colliders: []*serializationKCES.ColliderRef{{
			Type: 2,
			Collider: &serializationKCES.ColliderSphere{
				ColliderObject: serializationKCES.ColliderObject{
					Version:       1000,
					ParentName:    testStringPointer("Bip01 Neck"),
					SelfName:      testStringPointer("Collider"),
					LocalRotation: serializationKCES.Vector4{W: 1},
					LocalScale:    serializationKCES.Vector3{X: 1, Y: 1, Z: 1},
					Bound:         serializationKCES.ColliderBoundOutside,
				},
				Radius: 0.05,
			},
		}},
		LimbEnableList: []*serializationKCES.ColliderState{{Version: 1000, LimbType: 0, IsEnable: true}},
	}
	encoded, err := serializationKCES.EncodeKCESPayload(pkg, ".dbcol")
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
	var decodedJSON serializationKCES.ColliderPackage
	if err := json.Unmarshal(jsonBytes, &decodedJSON); err != nil {
		t.Fatalf("unmarshal payload json: %v", err)
	}
	if len(decodedJSON.LimbEnableList) != 1 {
		t.Fatalf("limb enable list not populated from JSON: %+v", decodedJSON)
	}

	if err := service.ConvertJsonToPayload(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToPayload: %v", err)
	}
	roundTrip, err := serializationKCES.DecodeKCESPayload(mustReadServiceTestFile(t, outputPath), ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload output: %v", err)
	}
	roundTripPackage, ok := roundTrip.(*serializationKCES.ColliderPackage)
	if !ok || roundTripPackage == nil {
		t.Fatalf("unexpected round-trip payload root: %#v", roundTrip)
	}
	c0, ok := roundTripPackage.Colliders[0].Collider.(*serializationKCES.ColliderSphere)
	if !ok || c0 == nil || testStringValue(c0.ParentName) != "Bip01 Neck" {
		t.Fatalf("unexpected round-trip collider package: %+v", roundTripPackage)
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

	params := &serializationKCES.ClothParams{
		Radius:                         &serializationKCES.BezierParam{StartValue: 0.02, EndValue: 0.04, UseEndValue: true},
		Mass:                           &serializationKCES.BezierParam{StartValue: 1, EndValue: 1},
		UseGravity:                     true,
		Gravity:                        &serializationKCES.BezierParam{StartValue: -9.8, EndValue: -9.8},
		UseDrag:                        true,
		Drag:                           &serializationKCES.BezierParam{StartValue: 0.02, EndValue: 0.02, UseEndValue: true},
		UseMaxVelocity:                 true,
		MaxVelocity:                    &serializationKCES.BezierParam{StartValue: 3, EndValue: 3},
		WorldMoveInfluence:             &serializationKCES.BezierParam{StartValue: 0.5, EndValue: 0.5},
		WorldRotationInfluence:         &serializationKCES.BezierParam{StartValue: 0.5, EndValue: 0.5},
		DisableDistance:                20,
		DisableFadeDistance:            5,
		UseClampDistanceRatio:          true,
		ClampDistanceMinRatio:          0.7,
		ClampDistanceMaxRatio:          1.1,
		UsePenetration:                 true,
		PenetrationMode:                serializationKCES.ClothPenetrationModeColliderPenetration,
		PenetrationAxis:                serializationKCES.ClothPenetrationAxisInverseZ,
		PenetrationConnectDistance:     &serializationKCES.BezierParam{StartValue: 0.2, EndValue: 0.3, UseEndValue: true},
		PenetrationDistance:            &serializationKCES.BezierParam{StartValue: 0.1, EndValue: 0.2, UseEndValue: true},
		PenetrationRadius:              &serializationKCES.BezierParam{StartValue: 0.3, EndValue: 1, UseEndValue: true},
		UseLineAvarageRotation:         true,
		GravityDirection:               serializationKCES.Vector3{Y: 1},
		MaxMoveSpeed:                   10,
		MaxRotationSpeed:               360,
		ResetStabilizationTime:         0.1,
		ClampRotationVelocityLimit:     1,
		ClampRotationVelocityInfluence: 0.2,
	}
	encoded, err := serializationKCES.EncodeKCESPayload(params, ext)
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
	var decodedJSON serializationKCES.ClothParams
	if err := json.Unmarshal(mustReadServiceTestFile(t, jsonPath), &decodedJSON); err != nil {
		t.Fatalf("unmarshal payload json: %v", err)
	}
	if !decodedJSON.UsePenetration {
		t.Fatalf("unexpected JSON payload root: %+v", decodedJSON)
	}
	decodedJSON.UsePenetration = false
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
	roundTripParams, ok := roundTrip.(*serializationKCES.ClothParams)
	if !ok || roundTripParams == nil || roundTripParams.UsePenetration {
		t.Fatalf("unexpected round-trip cloth params: %#v", roundTrip)
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
					if name == "default_accmimi_col.dbcol" || name == "default_yure_col.dbcol" {
						err := service.ConvertPayloadToJson(TestConversionContext, sample, filepath.Join(t.TempDir(), name+".json"), TestConversionMaxOutput)
						if err == nil || !strings.Contains(err.Error(), "sparse slot 13 must be nil") {
							t.Fatalf("ConvertPayloadToJson error = %v, want non-nil undeclared MaidProp slot rejection", err)
						}
						return
					}
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
	paths := kcesfixtures.PayloadSamplePaths(t)
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
