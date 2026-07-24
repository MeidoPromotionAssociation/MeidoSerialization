package KCES

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestKCESBridgeSessionTypedRoundTrip(t *testing.T) {
	sessionID := "会话-A"
	value := &KCESBridgeSession{
		Format:           KCESBridgeSessionFormat,
		ContainerVersion: -7,
		ContainerDirectories: map[string]ct.VirtualDirectoryMetadata{
			"empty":  {Version: 1001},
			"future": {Version: -7},
		},
		SessionData: KCESBridgeSessionData{
			Version:         -9,
			SessionID:       sessionID,
			HideMenuFileIDs: []uint64{0, 128, 256, ^uint64(0)},
		},
		ExtraFiles: map[string][]byte{
			"future/blob": {0x00, 0xff, 0x7f},
			"zero":        nil,
		},
	}
	encoded, err := EncodeKCESBridgeSession(value)
	if err != nil {
		t.Fatalf("EncodeKCESBridgeSession() error = %v", err)
	}
	decoded, err := DecodeKCESBridgeSession(encoded)
	if err != nil {
		t.Fatalf("DecodeKCESBridgeSession() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("round trip changed:\n got  %#v\n want %#v", decoded, value)
	}

	table, err := ct.ReadContentTable(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	rawSessionID, err := table.GetFileData(kcesBridgeSessionIDFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rawSessionID, []byte(sessionID)) {
		t.Fatalf("session_id = %x, want %x", rawSessionID, []byte(sessionID))
	}
}

func TestKCESBridgeSessionRequiresMatchingUTF8SessionID(t *testing.T) {
	id := "typed-id"
	sessionData, err := encodeKCESBridgeSessionData(&KCESBridgeSessionData{
		Version:         0,
		SessionID:       id,
		HideMenuFileIDs: []uint64{},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		sessionID []byte
		want      string
	}{
		{name: "mismatch", sessionID: []byte("different"), want: "does not match"},
		{name: "invalid UTF-8", sessionID: []byte{0xff}, want: "not valid UTF-8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire := makeBridgeSessionVirtualDirectory(t, 1000, sessionData, test.sessionID, nil)
			if _, err := DecodeKCESBridgeSession(wire); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeKCESBridgeSession() error = %v, want %q", err, test.want)
			}
		})
	}

	nilIDData := []byte{0x93, 0x00, 0xc0, 0x90}
	if _, err := DecodeKCESBridgeSession(makeBridgeSessionVirtualDirectory(t, 1000, nilIDData, nil, nil)); err == nil || !strings.Contains(err.Error(), "must not be null") {
		t.Fatalf("nil sessionId error = %v", err)
	}
}

func TestKCESBridgeSessionRejectsUnsupportedSessionDataShapes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "nil root", data: []byte{0xc0}, want: "root must not be nil"},
		{name: "short root", data: []byte{0x92, 0x00, 0xa1, 'x'}, want: "indexed-array width 2, expected 3"},
		{name: "high slot", data: []byte{0x94, 0x00, 0xa1, 'x', 0x90, 0xc0}, want: "indexed-array width 4, expected 3"},
		{name: "trailing data", data: []byte{0x93, 0x00, 0xa1, 'x', 0x90, 0xc0}, want: "trailing data"},
		{name: "value nil version", data: []byte{0x93, 0xc0, 0xa1, 'x', 0x90}, want: "version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire := makeBridgeSessionVirtualDirectory(t, 1000, test.data, []byte("x"), nil)
			if _, err := DecodeKCESBridgeSession(wire); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeKCESBridgeSession() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestKCESBridgeSessionRequiresReservedFiles(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string][]byte
		wantAbsent string
	}{
		{name: "missing session_data", files: map[string][]byte{"session_id": []byte("x")}, wantAbsent: "session_data"},
		{name: "missing session_id", files: map[string][]byte{"session_data": []byte{0x93, 0x00, 0xa1, 'x', 0x90}}, wantAbsent: "session_id"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize)}
			for name, data := range test.files {
				if err := table.AddFile(name, data); err != nil {
					t.Fatal(err)
				}
			}
			var wire bytes.Buffer
			if err := ct.WriteContentTable(&wire, table); err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeKCESBridgeSession(wire.Bytes()); err == nil || !strings.Contains(err.Error(), test.wantAbsent) {
				t.Fatalf("DecodeKCESBridgeSession() error = %v, want %q", err, test.wantAbsent)
			}
		})
	}
}

func TestEncodeKCESBridgeSessionValidation(t *testing.T) {
	valid := NewKCESBridgeSession("x")
	invalidUTF8 := string([]byte{0xff})
	tests := []struct {
		name  string
		value *KCESBridgeSession
	}{
		{name: "nil"},
		{name: "wrong format", value: &KCESBridgeSession{Format: "future", SessionData: valid.SessionData}},
		{name: "invalid UTF-8", value: &KCESBridgeSession{Format: KCESBridgeSessionFormat, SessionData: KCESBridgeSessionData{SessionID: invalidUTF8}}},
		{name: "reserved extra", value: &KCESBridgeSession{Format: KCESBridgeSessionFormat, SessionData: valid.SessionData, ExtraFiles: map[string][]byte{"session_id": {1}}}},
		{name: "unsafe extra path", value: &KCESBridgeSession{Format: KCESBridgeSessionFormat, SessionData: valid.SessionData, ExtraFiles: map[string][]byte{"../escape": {1}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := EncodeKCESBridgeSession(test.value); err == nil {
				t.Fatal("invalid bridge session unexpectedly encoded")
			}
		})
	}
}

func TestKCESBridgeMenuFileIDMatchesGameCallbackVectors(t *testing.T) {
	tests := map[string]uint64{
		`C:\mods\FOO.MATE`:   15816416568107797580,
		`D:/parts/BAR.model`: 6262680505836588403,
		``:                   5376480941607332326,
		`dir/日本語.anything`:   10815372602820151964,
		`C:\mixed\I.MENU`:    5744843728918987537,
	}
	for input, want := range tests {
		if got := KCESBridgeMenuFileID(input); got != want {
			t.Errorf("KCESBridgeMenuFileID(%q) = %d, want %d", input, got, want)
		}
	}
}

func makeBridgeSessionVirtualDirectory(t *testing.T, version int32, sessionData, sessionID []byte, extra map[string][]byte) []byte {
	t.Helper()
	table := &ct.ContentTable{Version: version, Raw: make([]byte, ct.HeaderSize)}
	if err := table.AddFile(kcesBridgeSessionDataFile, append([]byte(nil), sessionData...)); err != nil {
		t.Fatal(err)
	}
	if err := table.AddFile(kcesBridgeSessionIDFile, append([]byte(nil), sessionID...)); err != nil {
		t.Fatal(err)
	}
	for name, data := range extra {
		if err := table.AddFile(name, append([]byte(nil), data...)); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := ct.WriteContentTable(&out, table); err != nil {
		t.Fatalf("write test VirtualDirectory: %v", err)
	}
	return out.Bytes()
}
