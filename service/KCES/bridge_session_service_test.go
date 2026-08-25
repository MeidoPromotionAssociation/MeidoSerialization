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
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func TestBridgeSessionServiceStrictJSONRoundTripAndUInt64(t *testing.T) {
	sessionID := "session-日本語"
	value := &serializationKCES.KCESBridgeSession{
		ContainerVersion: serializationKCES.KCESBridgeSessionContainerVersion,
		SessionData: serializationKCES.KCESBridgeSessionData{
			Version:         serializationKCES.KCESBridgeSessionDataVersion,
			SessionID:       sessionID,
			HideMenuFileIDs: []uint64{0, 1 << 63, ^uint64(0)},
		},
		ExtraFiles: map[string][]byte{
			"future/data": {0, 1, 2, 3},
		},
	}
	native, err := serializationKCES.EncodeKCESBridgeSession(value)
	if err != nil {
		t.Fatal(err)
	}
	want, err := serializationKCES.DecodeKCESBridgeSession(native)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "bridge_session.vd")
	jsonPath := filepath.Join(dir, "bridge_session.vd.json")
	output := filepath.Join(dir, "roundtrip.vd")
	if err := os.WriteFile(input, native, 0644); err != nil {
		t.Fatal(err)
	}

	service := &BridgeSessionService{}
	if err := service.ConvertBridgeSessionToJSON(TestConversionContext, input, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertBridgeSessionToJSON: %v", err)
	}
	editingJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(editingJSON, []byte("18446744073709551615")) {
		t.Fatalf("UInt64 max was not emitted exactly as a JSON integer:\n%s", editingJSON)
	}
	if bytes.Contains(editingJSON, []byte("1.844674")) {
		t.Fatalf("UInt64 max was rounded/scientific-notation encoded:\n%s", editingJSON)
	}
	if err := service.ConvertJSONToBridgeSession(TestConversionContext, jsonPath, output, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJSONToBridgeSession: %v", err)
	}
	roundTripBytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	got, err := serializationKCES.DecodeKCESBridgeSession(roundTripBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("service round trip changed session:\n got  %#v\n want %#v", got, want)
	}
}

func TestBridgeSessionServiceAcceptsBOMAndRejectsLooseJSON(t *testing.T) {
	sessionID := "x"
	valid := &serializationKCES.KCESBridgeSession{
		ContainerVersion: serializationKCES.KCESBridgeSessionContainerVersion,
		SessionData: serializationKCES.KCESBridgeSessionData{
			Version:         serializationKCES.KCESBridgeSessionDataVersion,
			SessionID:       sessionID,
			HideMenuFileIDs: []uint64{},
		},
	}
	validJSON, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	service := &BridgeSessionService{}

	bomPath := filepath.Join(dir, "bom.vd.json")
	bom := append([]byte{0xef, 0xbb, 0xbf}, validJSON...)
	if err := os.WriteFile(bomPath, bom, 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.ConvertJSONToBridgeSession(TestConversionContext, bomPath, filepath.Join(dir, "bom.vd"), TestConversionMaxOutput); err != nil {
		t.Fatalf("UTF-8 BOM should be accepted: %v", err)
	}

	mutate := func(base []byte, suffix string) []byte {
		trimmed := bytes.TrimSpace(base)
		trimmed = bytes.TrimSuffix(trimmed, []byte("}"))
		return append(append(trimmed, []byte(suffix)...), '}')
	}
	tests := map[string][]byte{
		"root null":              []byte("null"),
		"invalid UTF-8":          append(append([]byte(nil), validJSON...), 0xff),
		"unknown root field":     mutate(validJSON, `,"unknown":1`),
		"trailing JSON value":    append(append([]byte(nil), validJSON...), []byte(" {}")...),
		"removed format tag":     []byte(`{"format":"kces-bridge-session","containerVersion":1000,"sessionData":{"version":0,"sessionId":"x","hideMenuFileIds":[]}}`),
		"unknown nested field":   []byte(`{"containerVersion":1000,"sessionData":{"version":0,"sessionId":"x","hideMenuFileIds":[],"unknown":1}}`),
		"rounded UInt64":         []byte(`{"containerVersion":1000,"sessionData":{"version":0,"sessionId":"x","hideMenuFileIds":[18446744073709551616]}}`),
		"sessionId raw fallback": []byte(`{"containerVersion":1000,"sessionData":{"version":0,"sessionId":"x","hideMenuFileIds":[]},"sessionIdFileData":"eA=="}`),
		"null sessionData":       []byte(`{"containerVersion":1000,"sessionData":null}`),
		"null sessionId":         []byte(`{"containerVersion":1000,"sessionData":{"version":0,"sessionId":null,"hideMenuFileIds":[]}}`),
		"missing container":      []byte(`{"sessionData":{"version":0,"sessionId":"x","hideMenuFileIds":[]}}`),
		"missing data version":   []byte(`{"containerVersion":1000,"sessionData":{"sessionId":"x","hideMenuFileIds":[]}}`),
		"missing sessionId":      []byte(`{"containerVersion":1000,"sessionData":{"version":0,"hideMenuFileIds":[]}}`),
		"missing IDs":            []byte(`{"containerVersion":1000,"sessionData":{"version":0,"sessionId":"x"}}`),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(dir, strings.ReplaceAll(name, " ", "_")+".vd.json")
			output := input + ".vd"
			if err := os.WriteFile(input, data, 0644); err != nil {
				t.Fatal(err)
			}
			if err := service.ConvertJSONToBridgeSession(TestConversionContext, input, output, TestConversionMaxOutput); err == nil {
				t.Fatal("loose/invalid editing JSON unexpectedly converted")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("failed conversion created output file: %v", err)
			}
		})
	}

	accepted := map[string][]byte{
		"null IDs": []byte(`{"containerVersion":1000,"sessionData":{"version":0,"sessionId":"x","hideMenuFileIds":null}}`),
	}
	for name, data := range accepted {
		t.Run("representable "+name, func(t *testing.T) {
			input := filepath.Join(dir, "accepted_"+strings.ReplaceAll(name, " ", "_")+".vd.json")
			output := input + ".vd"
			if err := os.WriteFile(input, data, 0644); err != nil {
				t.Fatal(err)
			}
			if err := service.ConvertJSONToBridgeSession(TestConversionContext, input, output, TestConversionMaxOutput); err != nil {
				t.Fatalf("representable editing JSON rejected: %v", err)
			}
			binaryData, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := serializationKCES.DecodeKCESBridgeSession(binaryData); err != nil {
				t.Fatalf("converted representation did not decode: %v", err)
			}
		})
	}
}

func TestBridgeSessionServiceFilePredicates(t *testing.T) {
	for _, path := range []string{
		`C:\work\bridge_session.vd`,
		`C:\work\BRIDGE_SESSION.VD`,
	} {
		if !IsKCESBridgeSessionFile(path) {
			t.Errorf("IsKCESBridgeSessionFile(%q) = false", path)
		}
	}
	for _, path := range []string{
		`C:\work\other.vd`,
		`C:\work\bridge_session.vd.json`,
		`C:\work\prefix_bridge_session.vd`,
	} {
		if IsKCESBridgeSessionFile(path) {
			t.Errorf("IsKCESBridgeSessionFile(%q) = true", path)
		}
	}
	for _, path := range []string{
		`C:\work\bridge_session.vd.json`,
		`C:\work\BRIDGE_SESSION.VD.JSON`,
	} {
		if !IsKCESBridgeSessionJSONFile(path) {
			t.Errorf("IsKCESBridgeSessionJSONFile(%q) = false", path)
		}
	}
	for _, path := range []string{
		`C:\work\other.vd.json`,
		`C:\work\bridge_session.json`,
		`C:\work\bridge_session.vd`,
	} {
		if IsKCESBridgeSessionJSONFile(path) {
			t.Errorf("IsKCESBridgeSessionJSONFile(%q) = true", path)
		}
	}
}

func TestFileTypeServiceRecognizesBridgeSessionByPathContentAndJSONMarker(t *testing.T) {
	sessionID := "route-session"
	value := &serializationKCES.KCESBridgeSession{
		ContainerVersion: serializationKCES.KCESBridgeSessionContainerVersion,
		SessionData: serializationKCES.KCESBridgeSessionData{
			Version:         serializationKCES.KCESBridgeSessionDataVersion,
			SessionID:       sessionID,
			HideMenuFileIDs: []uint64{1, ^uint64(0)},
		},
	}
	native, err := serializationKCES.EncodeKCESBridgeSession(value)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	service := &FileTypeService{}

	for _, name := range []string{"bridge_session.vd", "renamed.bin"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, native, 0644); err != nil {
			t.Fatal(err)
		}
		info, matched, err := service.TryFileTypeDetermine(path)
		if err != nil || !matched {
			t.Fatalf("native probe %q: matched=%v info=%+v err=%v", name, matched, info, err)
		}
		if info.FileType != "bridge_session" || info.StorageFormat != COM3D2Service.FormatBinary || info.Game != COM3D2Service.GameKCES || info.Signature != "KCES_VIRTUAL_DIRECTORY" || info.Version != 1000 {
			t.Fatalf("native probe %q info = %+v", name, info)
		}
	}

	nativePath := filepath.Join(dir, "bridge_session.vd")
	jsonPath := nativePath + ".json"
	if err := (&BridgeSessionService{}).ConvertBridgeSessionToJSON(TestConversionContext, nativePath, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	recognizedPath := filepath.Join(dir, "bridge_session.vd.json")
	if err := os.WriteFile(recognizedPath, jsonData, 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := service.TryFileTypeDetermine(recognizedPath)
	if err != nil || !matched {
		t.Fatalf("JSON probe: matched=%v info=%+v err=%v", matched, info, err)
	}
	if info.FileType != "bridge_session" || info.StorageFormat != COM3D2Service.FormatJSON || info.Game != COM3D2Service.GameKCES || info.Signature != serializationKCES.KCESBridgeSessionFormat || info.Version != 1000 {
		t.Fatalf("JSON probe info = %+v", info)
	}

	// 编辑 JSON 的格式标记已移除，唯一的标识是 bridge_session.vd.json 双扩展名，
	// 因此改名后的文件不再被识别为 bridge session
	// The editing format marker was removed and the only marker is the bridge_session.vd.json double
	// extension, so a renamed file is no longer recognized as a bridge session
	renamedPath := filepath.Join(dir, "renamed.json")
	if err := os.WriteFile(renamedPath, jsonData, 0644); err != nil {
		t.Fatal(err)
	}
	if info, matched, err := service.TryFileTypeDetermine(renamedPath); matched && info.FileType == "bridge_session" {
		t.Fatalf("renamed bridge session editing JSON was still recognized: info=%+v err=%v", info, err)
	}
}

func TestFileTypeServiceBridgeSessionCandidatesAndOtherVirtualDirectories(t *testing.T) {
	dir := t.TempDir()
	service := &FileTypeService{}
	for name, data := range map[string][]byte{
		"bridge_session.vd":      []byte("not a VirtualDirectory"),
		"bridge_session.vd.json": []byte(`{"containerVersion":1000`),
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		info, matched, err := service.TryFileTypeDetermine(path)
		if !matched || err == nil || info.FileType != COM3D2Service.UnknownFileType {
			t.Fatalf("malformed candidate %q: matched=%v info=%+v err=%v", name, matched, info, err)
		}
	}

	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize)}
	table.AddFile("unrelated", []byte{1, 2, 3})
	var other bytes.Buffer
	if err := ct.WriteContentTable(&other, table); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "other.vd")
	if err := os.WriteFile(path, other.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := service.TryFileTypeDetermine(path)
	if err != nil || !matched || info.FileType != "virtualdirectory" {
		t.Fatalf("unrelated VirtualDirectory was misrouted: matched=%v info=%+v err=%v", matched, info, err)
	}
}
