package KCES

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func TestGP03BridgeServiceJSONRoundTripAndFileTypeProbe(t *testing.T) {
	dir := t.TempDir()
	value := sourceConstructedServiceBridge(t)
	native, err := serializationKCES.EncodeGP03Bridge(value)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "gp03_export_maid.brd")
	jsonPath := inputPath + ".json"
	outputPath := filepath.Join(dir, "output.brd")
	if err := os.WriteFile(inputPath, native, 0644); err != nil {
		t.Fatal(err)
	}
	renamedPath := filepath.Join(dir, "renamed-bridge.bin")
	if err := os.WriteFile(renamedPath, native, 0644); err != nil {
		t.Fatal(err)
	}

	service := &GP03BridgeService{}
	if err := service.ConvertBridgeToJSON(inputPath, jsonPath); err != nil {
		t.Fatalf("ConvertBridgeToJSON: %v", err)
	}
	var envelope serializationKCES.GP03BridgeFile
	if err := json.Unmarshal(mustReadTestFile(t, jsonPath), &envelope); err != nil {
		t.Fatalf("unmarshal bridge JSON: %v", err)
	}
	if envelope.Format != serializationKCES.KCESGP03BridgeFormat || envelope.Signature != serializationKCES.GP03BridgeSignature || envelope.Version != serializationKCES.GP03BridgeVersion {
		t.Fatalf("unexpected bridge JSON envelope: %+v", envelope)
	}
	if !bytes.Equal(envelope.LegacyPreset, value.LegacyPreset) || !bytes.Equal(envelope.CurrentPreset, value.CurrentPreset) {
		t.Fatal("bridge JSON did not preserve embedded preset blobs")
	}

	for _, path := range []string{inputPath, renamedPath, jsonPath} {
		info, matched, probeErr := (&FileTypeService{}).TryFileTypeDetermine(path)
		if probeErr != nil || !matched {
			t.Fatalf("bridge probe %q: matched=%v info=%+v err=%v", path, matched, info, probeErr)
		}
		wantStorage := COM3D2Service.FormatBinary
		wantSignature := serializationKCES.GP03BridgeSignature
		if strings.HasSuffix(strings.ToLower(path), ".json") {
			wantStorage = COM3D2Service.FormatJSON
			wantSignature = serializationKCES.KCESGP03BridgeFormat
		}
		if info.FileType != "brd" || info.StorageFormat != wantStorage || info.Game != COM3D2Service.GameKCES || info.Signature != wantSignature || info.Version != serializationKCES.GP03BridgeVersion {
			t.Fatalf("bridge info for %q = %+v", path, info)
		}
	}

	// Service-side JSON readers consistently accept an optional UTF-8 BOM.
	withBOM := append([]byte{0xef, 0xbb, 0xbf}, mustReadTestFile(t, jsonPath)...)
	if err := os.WriteFile(jsonPath, withBOM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.ConvertJSONToBridge(jsonPath, outputPath); err != nil {
		t.Fatalf("ConvertJSONToBridge: %v", err)
	}
	decodedInput, err := serializationKCES.DecodeGP03Bridge(native)
	if err != nil {
		t.Fatal(err)
	}
	decodedOutput, err := service.ReadBridgeFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedOutput, decodedInput) {
		t.Fatalf("service round-trip changed bridge:\ngot =%+v\nwant=%+v", decodedOutput, decodedInput)
	}
	if !bytes.Equal(mustReadTestFile(t, outputPath), native) {
		t.Fatal("service round-trip changed deterministic bridge wire")
	}
}

func TestGP03BridgeServiceStrictJSONAndRouting(t *testing.T) {
	if !IsKCESGP03BridgeFile("sample.BRD") || IsKCESGP03BridgeFile("sample.brd.json") {
		t.Fatal("native .brd routing is incorrect")
	}
	if !IsKCESGP03BridgeJSONFile("sample.BRD.JSON") || IsKCESGP03BridgeJSONFile("sample.json") {
		t.Fatal(".brd.json routing is incorrect")
	}

	service := &GP03BridgeService{}
	badNativePath := filepath.Join(t.TempDir(), "bad.brd")
	if err := os.WriteFile(badNativePath, []byte("not a GP03 bridge"), 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, probeErr := (&FileTypeService{}).TryFileTypeDetermine(badNativePath)
	if !matched || probeErr == nil || info.FileType != COM3D2Service.UnknownFileType {
		t.Fatalf("malformed .brd probe: matched=%v info=%+v err=%v", matched, info, probeErr)
	}

	valid := sourceConstructedServiceBridge(t)
	validJSON, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]interface{}
	if err := json.Unmarshal(validJSON, &object); err != nil {
		t.Fatal(err)
	}
	delete(object, "legacyPreset")
	missing, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}

	unknown := append(validJSON[:len(validJSON)-1], []byte(`,"future":1}`)...)
	trailing := append(append([]byte(nil), validJSON...), []byte(` {}`)...)
	for name, body := range map[string][]byte{
		"unknown":      unknown,
		"trailing":     trailing,
		"invalid utf8": []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "bad.brd.json")
			outputPath := filepath.Join(dir, "bad.brd")
			if err := os.WriteFile(inputPath, body, 0644); err != nil {
				t.Fatal(err)
			}
			if err := service.ConvertJSONToBridge(inputPath, outputPath); err == nil {
				t.Fatal("malformed bridge JSON was accepted")
			}
			if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
				t.Fatalf("failed conversion left output file: %v", statErr)
			}
		})
	}
	for name, body := range map[string][]byte{
		"missing opaque blob": missing,
		"null opaque blobs":   []byte(`{"format":"kces-gp03-bridge","signature":"GP03_BRIDGE","version":2001,"guid":"g","legacyPreset":null,"currentPreset":null}`),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "opaque.brd.json")
			outputPath := filepath.Join(dir, "opaque.brd")
			if err := os.WriteFile(inputPath, body, 0644); err != nil {
				t.Fatal(err)
			}
			if err := service.ConvertJSONToBridge(inputPath, outputPath); err != nil {
				t.Fatalf("representable opaque blobs rejected: %v", err)
			}
		})
	}
}

func TestGP03BridgeServicePreservesAndReportsCOM3D2V2000(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gp03_export_reverse.brd")
	native, err := serializationKCES.EncodeGP03Bridge(&serializationKCES.GP03BridgeFile{
		Signature: serializationKCES.GP03BridgeSignature,
		Version:   serializationKCES.GP03BridgeCOM3D2Version,
		GUID:      "reverse-guid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, native, 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(path)
	if err != nil || !matched || info.FileType != "brd" || info.Version != serializationKCES.GP03BridgeCOM3D2Version {
		t.Fatalf("v2000 file type: matched=%v info=%+v err=%v", matched, info, err)
	}

	service := &GP03BridgeService{}
	jsonPath := path + ".json"
	if err := service.ConvertBridgeToJSON(path, jsonPath); err != nil {
		t.Fatal(err)
	}
	jsonInfo, matched, err := (&FileTypeService{}).TryFileTypeDetermine(jsonPath)
	if err != nil || !matched || jsonInfo.FileType != "brd" || jsonInfo.Version != serializationKCES.GP03BridgeCOM3D2Version {
		t.Fatalf("v2000 JSON file type: matched=%v info=%+v err=%v", matched, jsonInfo, err)
	}
	backPath := filepath.Join(dir, "reverse-back.brd")
	if err := service.ConvertJSONToBridge(jsonPath, backPath); err != nil {
		t.Fatal(err)
	}
	if back := mustReadTestFile(t, backPath); !bytes.Equal(back, native) {
		t.Fatal("v2000 service round-trip changed wire bytes")
	}
}

func sourceConstructedServiceBridge(t *testing.T) *serializationKCES.GP03BridgeFile {
	t.Helper()
	var legacy bytes.Buffer
	bw := stream.NewBinaryWriter(&legacy)
	for _, write := range []func() error{
		func() error { return bw.WriteString("CM3D2_PRESET") },
		func() error { return bw.WriteInt32(2001) },
		func() error { return bw.WriteInt32(2) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteString("CM3D2_MPROP_LIST") },
		func() error { return bw.WriteInt32(2001) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
	} {
		if err := write(); err != nil {
			t.Fatal(err)
		}
	}
	current, err := serializationKCES.EncodeKCESPreset(&serializationKCES.KCESPreset{
		ContainerVersion: 1000,
		Thumbnail:        []byte{1, 2, 3},
		MaidData:         mustKCESPresetCoreForServiceTest(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &serializationKCES.GP03BridgeFile{
		Format:        serializationKCES.KCESGP03BridgeFormat,
		Signature:     serializationKCES.GP03BridgeSignature,
		Version:       serializationKCES.GP03BridgeVersion,
		GUID:          "source-constructed-guid",
		LegacyPreset:  legacy.Bytes(),
		CurrentPreset: current,
	}
}
