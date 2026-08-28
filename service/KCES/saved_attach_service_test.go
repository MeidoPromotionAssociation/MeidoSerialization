package KCES

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/COM3D2"
)

func TestSavedAttachServiceJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	partName := "hat-point"
	targetName := "Bip01 Head"
	value := &serializationKCES.SavedAttachFile{
		Signature: serializationKCES.SavedAttachSignature,
		Version:   serializationKCES.SavedAttachFileVersion,
		Items: []serializationKCES.SavedAttachData{{
			Version:                serializationKCES.SavedAttachRecordVersion,
			PartName:               &partName,
			Enabled:                true,
			MyRID:                  1,
			MySlotID:               "accHat",
			TargetRID:              2,
			TargetSlotID:           "body",
			TargetSlotNo:           3,
			TargetAttachPointName:  &targetName,
			TargetVertexCount:      4,
			TargetVertexIndex:      5,
			NewAttachVertexIndices: []int32{6, 7},
			PRS2: &serializationKCES.SavedAttachPosRotScale{
				Scale:    serializationKCES.Vector3{X: 1, Y: 1, Z: 1},
				Rotation: serializationKCES.Vector4{W: 1},
			},
			BoneAttachedHierarchy: map[string]serializationKCES.SavedAttachPosRotScale{
				"Bip01": {Rotation: serializationKCES.Vector4{W: 1}},
			},
			BoneAttachEdited: true,
		}},
	}
	data, err := serializationKCES.EncodeSavedAttach(value)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "sample.sad")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(dir, "output.sad")
	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	service := &SavedAttachService{}
	if err := service.ConvertSavedAttachToJSON(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertSavedAttachToJSON: %v", err)
	}
	var envelope serializationKCES.SavedAttachFile
	if err := json.Unmarshal(mustReadTestFile(t, jsonPath), &envelope); err != nil {
		t.Fatalf("unmarshal editing JSON: %v", err)
	}
	if envelope.Signature != serializationKCES.SavedAttachSignature {
		t.Fatalf("unexpected editing envelope: %+v", envelope)
	}
	for _, path := range []string{inputPath, jsonPath} {
		info, matched, probeErr := (&FileTypeService{}).TryFileTypeDetermine(path)
		if probeErr != nil || !matched {
			t.Fatalf("saved-attach probe %q: matched=%v info=%+v err=%v", path, matched, info, probeErr)
		}
		wantFormat := COM3D2Service.FormatBinary
		wantSignature := serializationKCES.SavedAttachSignature
		if strings.HasSuffix(strings.ToLower(path), ".json") {
			wantFormat = COM3D2Service.FormatJSON
			wantSignature = serializationKCES.KCESSavedAttachFormat
		}
		if info.FileType != "sad" || info.StorageFormat != wantFormat || info.Game != COM3D2Service.GameKCES || info.Signature != wantSignature || info.Version != serializationKCES.SavedAttachFileVersion {
			t.Fatalf("saved-attach info for %q = %+v", path, info)
		}
	}
	if err := service.ConvertJSONToSavedAttach(TestConversionContext, jsonPath, outputPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJSONToSavedAttach: %v", err)
	}
	decodedInput, err := serializationKCES.DecodeSavedAttach(data)
	if err != nil {
		t.Fatal(err)
	}
	decodedOutput, err := service.ReadSavedAttachFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedOutput, decodedInput) {
		t.Fatalf("service round-trip changed value:\ngot =%+v\nwant=%+v", decodedOutput, decodedInput)
	}
	if !bytes.Equal(mustReadTestFile(t, outputPath), data) {
		t.Fatal("service round-trip changed deterministic saved-attach wire")
	}
}

func TestSavedAttachServiceRejectsTrailingData(t *testing.T) {
	dir := t.TempDir()
	value := serializationKCES.NewSavedAttachFile()
	wire, err := serializationKCES.EncodeSavedAttach(value)
	if err != nil {
		t.Fatal(err)
	}

	inputPath := filepath.Join(dir, "metadata.sad")
	jsonPath := inputPath + ".json"
	wire = append(wire, 0xde, 0xad, 0xbe, 0xef)
	if err := os.WriteFile(inputPath, wire, 0644); err != nil {
		t.Fatal(err)
	}
	service := &SavedAttachService{}
	if err := service.ConvertSavedAttachToJSON(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err == nil {
		t.Fatal("saved-attach trailing data was accepted")
	}
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Fatalf("failed conversion created JSON output: %v", err)
	}
}

func TestSavedAttachServiceStrictJSONAndRouting(t *testing.T) {
	if !IsKCESSavedAttachFile("sample.SAD") || IsKCESSavedAttachFile("sample.sad.json") {
		t.Fatal("native .sad routing is incorrect")
	}
	if !IsKCESSavedAttachJSONFile("sample.SAD.JSON") || IsKCESSavedAttachJSONFile("sample.json") {
		t.Fatal(".sad.json routing is incorrect")
	}

	service := &SavedAttachService{}
	badNativePath := filepath.Join(t.TempDir(), "bad.sad")
	if err := os.WriteFile(badNativePath, []byte("not a saved attach file"), 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, probeErr := (&FileTypeService{}).TryFileTypeDetermine(badNativePath)
	if !matched || probeErr == nil || info.FileType != COM3D2Service.UnknownFileType {
		t.Fatalf("malformed .sad probe: matched=%v info=%+v err=%v", matched, info, probeErr)
	}
	for name, body := range map[string]string{
		"missing envelope":    `{}`,
		"removed format tag":  `{"format":"kces-saved-attach","signature":"SAVED_ATTACH_DATA","version":2000,"items":[]}`,
		"empty signature":     `{"signature":"","version":2000,"items":[]}`,
		"unknown":             `{"signature":"SAVED_ATTACH_DATA","version":2000,"items":[],"future":1}`,
		"trailing":            `{"signature":"SAVED_ATTACH_DATA","version":2000,"items":[]} {}`,
		"null record version": `{"signature":"SAVED_ATTACH_DATA","version":2000,"items":[{"version":null}]}`,
		"unsupported version": `{"signature":"SAVED_ATTACH_DATA","version":0,"items":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "bad.sad.json")
			outputPath := filepath.Join(dir, "bad.sad")
			if err := os.WriteFile(inputPath, append([]byte{0xef, 0xbb, 0xbf}, []byte(body)...), 0644); err != nil {
				t.Fatal(err)
			}
			err := service.ConvertJSONToSavedAttach(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
			if err == nil {
				t.Fatal("malformed saved-attach JSON was accepted")
			}
			if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
				t.Fatalf("failed conversion left output file: %v", statErr)
			}
		})
	}

	accepted := []struct {
		name  string
		body  string
		check func(*testing.T, *serializationKCES.SavedAttachFile)
	}{
		{
			name: "opaque slot IDs",
			body: `{"signature":"SAVED_ATTACH_DATA","version":2000,"items":[{"version":2001,"mySlotId":"acchat","targetSlotId":"future_Slot"}]}`,
			check: func(t *testing.T, value *serializationKCES.SavedAttachFile) {
				if len(value.Items) != 1 || value.Items[0].MySlotID != "acchat" || value.Items[0].TargetSlotID != "future_Slot" {
					t.Fatalf("opaque slot IDs changed: %+v", value.Items)
				}
			},
		},
		{
			name: "missing items",
			body: `{"signature":"SAVED_ATTACH_DATA","version":2000}`,
		},
		{
			name: "null items",
			body: `{"signature":"SAVED_ATTACH_DATA","version":2000,"items":null}`,
		},
	}
	for _, test := range accepted {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "representable.sad.json")
			outputPath := filepath.Join(dir, "representable.sad")
			if err := os.WriteFile(inputPath, append([]byte{0xef, 0xbb, 0xbf}, []byte(test.body)...), 0644); err != nil {
				t.Fatal(err)
			}
			if err := service.ConvertJSONToSavedAttach(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput); err != nil {
				t.Fatalf("wire-representable saved-attach JSON was rejected: %v", err)
			}
			value, err := service.ReadSavedAttachFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			if test.check != nil {
				test.check(t, value)
			}
		})
	}

	t.Run("invalid UTF-8", func(t *testing.T) {
		dir := t.TempDir()
		inputPath := filepath.Join(dir, "bad.sad.json")
		outputPath := filepath.Join(dir, "bad.sad")
		body := []byte(`{"signature":"SAVED_ATTACH_DATA","version":2000,"items":[{"version":2001,"partName":"`)
		body = append(body, 0xff)
		body = append(body, []byte(`","enabled":false,"myRid":0,"mySlotId":"body","targetRid":0,"targetSlotId":"body","targetSlotNo":0,"targetAttachPointName":null,"targetVertexCount":0,"targetVertexIndex":0,"newAttachVertexIndices":null,"prs2":null,"prs3":null,"boneAttachedHierarchy":null,"boneAttachEdited":false}]}`)...)
		if err := os.WriteFile(inputPath, body, 0644); err != nil {
			t.Fatal(err)
		}
		err := service.ConvertJSONToSavedAttach(TestConversionContext, inputPath, outputPath, TestConversionMaxOutput)
		if err == nil || !strings.Contains(err.Error(), "UTF-8") {
			t.Fatalf("invalid UTF-8 error=%v", err)
		}
		if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
			t.Fatalf("failed conversion left output file: %v", statErr)
		}
	})
}
