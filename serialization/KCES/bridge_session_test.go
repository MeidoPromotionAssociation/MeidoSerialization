package KCES

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestDecodeKCESBridgeSessionHandWrittenWireAndLosslessExtras(t *testing.T) {
	sessionID := "会话-A"
	// Hand-written Standard MessagePack:
	// array(5) [version=0, sessionId, HashSet<ulong>, future map, future array].
	sessionData := []byte{0x95, 0x00, 0xa8}
	sessionData = append(sessionData, []byte(sessionID)...)
	sessionData = append(sessionData,
		0x94,
		0x00,
		0xcc, 0x80,
		0xcd, 0x01, 0x00,
		0xcf, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x81, 0xa1, 'x', 0x92, 0xc3, 0xc0,
		0x92, 0x01, 0x02,
	)
	binaryData := makeBridgeSessionVirtualDirectory(t, 1000, sessionData, []byte(sessionID), map[string][]byte{
		"future/blob": {0x00, 0xff, 0x7f},
		"zero":        nil,
	})

	got, err := DecodeKCESBridgeSession(binaryData)
	if err != nil {
		t.Fatalf("DecodeKCESBridgeSession: %v", err)
	}
	if got.Format != KCESBridgeSessionFormat || got.ContainerVersion != 1000 {
		t.Fatalf("header = %#v", got)
	}
	if got.SessionData == nil {
		t.Fatal("sessionData is nil")
	}
	if got.SessionData.Version != 0 || got.SessionData.SessionID != sessionID {
		t.Fatalf("sessionData header = %#v", got.SessionData)
	}
	wantIDs := []uint64{0, 128, 256, ^uint64(0)}
	if !reflect.DeepEqual(got.SessionData.HideMenuFileIDs, wantIDs) {
		t.Fatalf("hideMenuFileIds = %#v, want %#v", got.SessionData.HideMenuFileIDs, wantIDs)
	}
	wantFuture := [][]byte{
		{0x81, 0xa1, 'x', 0x92, 0xc3, 0xc0},
		{0x92, 0x01, 0x02},
	}
	if !reflect.DeepEqual(got.SessionData.FutureSlots, wantFuture) {
		t.Fatalf("futureSlots = %x, want %x", got.SessionData.FutureSlots, wantFuture)
	}
	if !reflect.DeepEqual(got.ExtraFiles, map[string][]byte{
		"future/blob": {0x00, 0xff, 0x7f},
		"zero":        nil,
	}) {
		t.Fatalf("extraFiles = %#v", got.ExtraFiles)
	}

	before := cloneBridgeSessionForTest(got)
	reencoded, err := EncodeKCESBridgeSession(got)
	if err != nil {
		t.Fatalf("EncodeKCESBridgeSession: %v", err)
	}
	if !reflect.DeepEqual(got, before) {
		t.Fatalf("encoding mutated caller:\n got  %#v\n want %#v", got, before)
	}
	again, err := DecodeKCESBridgeSession(reencoded)
	if err != nil {
		t.Fatalf("decode re-encoded session: %v", err)
	}
	if !reflect.DeepEqual(again, got) {
		t.Fatalf("semantic round trip changed session:\n got  %#v\n want %#v", again, got)
	}

	table, err := ct.ReadContentTable(bytes.NewReader(reencoded))
	if err != nil {
		t.Fatal(err)
	}
	rawSessionData, err := table.GetFileData("session_data")
	if err != nil {
		t.Fatal(err)
	}
	if len(rawSessionData) == 0 || rawSessionData[0] != 0x95 {
		t.Fatalf("session_data is not a bare Standard MessagePack array(5): %x", rawSessionData)
	}
	rawSessionID, err := table.GetFileData("session_id")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rawSessionID, []byte(sessionID)) {
		t.Fatalf("raw session_id = %x, want %x", rawSessionID, []byte(sessionID))
	}
}

func TestKCESBridgeSessionPreservesVersionsOnCopies(t *testing.T) {
	// The library does not invoke the game's OnBeforeSerialize callbacks.
	sessionData := []byte{0x93, 0xff, 0xa1, 'x', 0x90} // data version -1
	binaryData := makeBridgeSessionVirtualDirectory(t, 1234, sessionData, []byte("x"), nil)
	decoded, err := DecodeKCESBridgeSession(binaryData)
	if err != nil {
		t.Fatalf("decode legacy/future versions: %v", err)
	}
	if decoded.ContainerVersion != 1234 || decoded.SessionData.Version != -1 {
		t.Fatalf("decoded versions = container %d, data %d", decoded.ContainerVersion, decoded.SessionData.Version)
	}
	before := cloneBridgeSessionForTest(decoded)
	encoded, err := EncodeKCESBridgeSession(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, before) {
		t.Fatal("encoding mutated the caller")
	}
	again, err := DecodeKCESBridgeSession(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if again.ContainerVersion != 1234 || again.SessionData.Version != -1 {
		t.Fatalf("versions changed = container %d, data %d", again.ContainerVersion, again.SessionData.Version)
	}
}

func TestKCESBridgeSessionPreservesContainerWireMetadata(t *testing.T) {
	containerFieldCount := 4
	fileFieldCount := 3
	sessionFieldCount := 3
	value := &KCESBridgeSession{
		ContainerVersion:     -7,
		ContainerFieldCount:  &containerFieldCount,
		ContainerFutureSlots: [][]byte{{0xcc, 0x08}},
		ContainerDirectories: map[string]ct.VirtualDirectoryMetadata{
			"future": {Versionless: true},
			"empty":  {Version: -7},
		},
		ContainerVirtualFiles: map[string]ct.VirtualFileMetadata{
			"session_id": {FieldCount: &fileFieldCount, FutureSlots: [][]byte{{0xd4, 0x01, 0x7f}}},
		},
		SessionData: &KCESBridgeSessionData{
			FieldCount:      &sessionFieldCount,
			Version:         -9,
			SessionID:       "id",
			HideMenuFileIDs: []uint64{},
		},
		SessionIDFileData: []byte("id"),
		ExtraFiles:        map[string][]byte{"future/blob": {1, 2, 3}},
	}
	encoded, err := EncodeKCESBridgeSession(value)
	if err != nil {
		t.Fatalf("EncodeKCESBridgeSession() error = %v", err)
	}
	decoded, err := DecodeKCESBridgeSession(encoded)
	if err != nil {
		t.Fatalf("DecodeKCESBridgeSession() error = %v", err)
	}
	if decoded.ContainerVersion != value.ContainerVersion || !reflect.DeepEqual(decoded.ContainerFieldCount, value.ContainerFieldCount) || !reflect.DeepEqual(decoded.ContainerFutureSlots, value.ContainerFutureSlots) || !reflect.DeepEqual(decoded.ContainerDirectories, value.ContainerDirectories) || !reflect.DeepEqual(decoded.ContainerVirtualFiles, value.ContainerVirtualFiles) {
		t.Fatalf("container metadata changed:\n got  %+v\n want %+v", decoded, value)
	}
}

func TestDecodeKCESBridgeSessionPreservesIndependentSessionIDFiles(t *testing.T) {
	tests := []struct {
		name      string
		dataID    []byte
		rawID     []byte
		wantID    string
		wantIDNil bool
	}{
		{name: "mismatch", dataID: []byte{0xa1, 'a'}, rawID: []byte("b"), wantID: "a"},
		{name: "nil MessagePack session id", dataID: []byte{0xc0}, rawID: []byte{}, wantIDNil: true},
		{name: "empty session id", dataID: []byte{0xa0}, rawID: []byte{}},
		{name: "invalid raw UTF-8", dataID: []byte{0xa1, 'x'}, rawID: []byte{0xff}, wantID: "x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sessionData := append([]byte{0x93, 0x00}, test.dataID...)
			sessionData = append(sessionData, 0x90)
			binaryData := makeBridgeSessionVirtualDirectory(t, 1000, sessionData, test.rawID, nil)
			got, err := DecodeKCESBridgeSession(binaryData)
			if err != nil {
				t.Fatalf("DecodeKCESBridgeSession: %v", err)
			}
			if got.SessionData.SessionID != test.wantID || got.SessionData.SessionIDIsNil != test.wantIDNil || !bytes.Equal(got.SessionIDFileData, test.rawID) {
				t.Fatalf("independent session IDs changed: %#v raw=%x", got.SessionData, got.SessionIDFileData)
			}
			reencoded, err := EncodeKCESBridgeSession(got)
			if err != nil {
				t.Fatal(err)
			}
			again, err := DecodeKCESBridgeSession(reencoded)
			if err != nil || !reflect.DeepEqual(again, got) {
				t.Fatalf("round trip = %#v, %v; want %#v", again, err, got)
			}
		})
	}

	invalidData := []byte{0x93, 0x00, 0xa1, 0xff, 0x90}
	invalidDecoded, err := DecodeKCESBridgeSession(makeBridgeSessionVirtualDirectory(t, 1000, invalidData, nil, nil))
	if err != nil {
		t.Fatalf("MessagePack-CSharp-readable malformed UTF-8 was rejected: %v", err)
	}
	if invalidDecoded.SessionData.SessionID != "\uFFFD" {
		t.Fatalf("invalid UTF-8 replacement = %q, want U+FFFD", invalidDecoded.SessionData.SessionID)
	}
}

func TestDecodeKCESBridgeSessionPreservesNilEmptyAndDuplicateIDArrays(t *testing.T) {
	emptySet := makeBridgeSessionVirtualDirectory(t, 1000, []byte{0x93, 0x00, 0xa1, 'x', 0x90}, []byte("x"), nil)
	decoded, err := DecodeKCESBridgeSession(emptySet)
	if err != nil {
		t.Fatalf("decode empty HashSet: %v", err)
	}
	if decoded.SessionData.HideMenuFileIDs == nil || len(decoded.SessionData.HideMenuFileIDs) != 0 {
		t.Fatalf("empty HashSet distinction was lost: %#v", decoded.SessionData.HideMenuFileIDs)
	}

	duplicates := makeBridgeSessionVirtualDirectory(t, 1000, []byte{0x93, 0x00, 0xa1, 'x', 0x93, 0x02, 0x01, 0x02}, []byte("x"), nil)
	decoded, err = DecodeKCESBridgeSession(duplicates)
	if err != nil {
		t.Fatalf("decode duplicate HashSet wire items: %v", err)
	}
	if want := []uint64{2, 1, 2}; !reflect.DeepEqual(decoded.SessionData.HideMenuFileIDs, want) {
		t.Fatalf("duplicate IDs changed = %#v, want %#v", decoded.SessionData.HideMenuFileIDs, want)
	}

	nilSet := makeBridgeSessionVirtualDirectory(t, 1000, []byte{0x93, 0x00, 0xa1, 'x', 0xc0}, []byte("x"), nil)
	decoded, err = DecodeKCESBridgeSession(nilSet)
	if err != nil || decoded.SessionData.HideMenuFileIDs != nil {
		t.Fatalf("nil ID collection = %#v, %v", decoded, err)
	}
}

func TestDecodeKCESBridgeSessionUInt64AndMalformedWireBoundaries(t *testing.T) {
	validIDs := []byte{
		0x96,
		0x7f,
		0xcc, 0x80,
		0xcd, 0x01, 0x00,
		0xce, 0x80, 0x00, 0x00, 0x00,
		0xcf, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xd0, 0x01,
	}
	wire := append([]byte{0x93, 0x00, 0xa1, 'x'}, validIDs...)
	decoded, err := DecodeKCESBridgeSession(makeBridgeSessionVirtualDirectory(t, 1000, wire, []byte("x"), nil))
	if err != nil {
		t.Fatalf("decode UInt64 marker boundaries: %v", err)
	}
	want := []uint64{127, 128, 256, 1 << 31, 1 << 63, 1}
	if !reflect.DeepEqual(decoded.SessionData.HideMenuFileIDs, want) {
		t.Fatalf("UInt64 values = %#v, want %#v", decoded.SessionData.HideMenuFileIDs, want)
	}

	cases := map[string][]byte{
		"version UInt32 overflow": {0x93, 0xce, 0x80, 0x00, 0x00, 0x00, 0xa1, 'x', 0x90},
		"negative UInt64":         {0x93, 0x00, 0xa1, 'x', 0x91, 0xff},
		"wrong set type":          {0x93, 0x00, 0xa1, 'x', 0x80},
		"truncated set":           {0x93, 0x00, 0xa1, 'x', 0x92, 0x01},
		"collection count bomb":   {0x93, 0x00, 0xa1, 'x', 0xdd, 0xff, 0xff, 0xff, 0xff},
		"invalid future marker":   {0x94, 0x00, 0xa1, 'x', 0x90, 0xc1},
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			binaryData := makeBridgeSessionVirtualDirectory(t, 1000, data, []byte("x"), nil)
			if _, err := DecodeKCESBridgeSession(binaryData); err == nil {
				t.Fatalf("malformed session_data %x unexpectedly decoded", data)
			}
		})
	}

	for name, data := range map[string][]byte{
		"root nil":            {0xc0},
		"empty indexed array": {0x90},
		"short indexed array": {0x92, 0x00, 0xa1, 'x'},
	} {
		t.Run(name, func(t *testing.T) {
			decoded, err := DecodeKCESBridgeSession(makeBridgeSessionVirtualDirectory(t, 1000, data, []byte("independent"), nil))
			if err != nil {
				t.Fatalf("decode representable short form: %v", err)
			}
			encoded, err := EncodeKCESBridgeSession(decoded)
			if err != nil {
				t.Fatalf("encode representable short form: %v", err)
			}
			again, err := DecodeKCESBridgeSession(encoded)
			if err != nil || !reflect.DeepEqual(again, decoded) {
				t.Fatalf("short form round trip = %#v, %v; want %#v", again, err, decoded)
			}
		})
	}
}

func TestDecodeKCESBridgeSessionRequiresReservedFiles(t *testing.T) {
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize)}
	table.AddFile("session_id", []byte("x"))
	var binaryData bytes.Buffer
	if err := ct.WriteContentTable(&binaryData, table); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESBridgeSession(binaryData.Bytes()); err == nil || !strings.Contains(err.Error(), "session_data") {
		t.Fatalf("missing session_data error = %v", err)
	}

	table = &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize)}
	table.AddFile("session_data", []byte{0x93, 0x00, 0xa1, 'x', 0x90})
	binaryData.Reset()
	if err := ct.WriteContentTable(&binaryData, table); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESBridgeSession(binaryData.Bytes()); err == nil || !strings.Contains(err.Error(), "session_id") {
		t.Fatalf("missing session_id error = %v", err)
	}
}

func TestKCESBridgeMenuFileIDMatchesGameCallbackVectors(t *testing.T) {
	tests := map[string]uint64{
		`C:\mods\FOO.MATE`:   15816416568107797580, // foo.menu
		`D:/parts/BAR.model`: 6262680505836588403,  // bar.menu
		``:                   5376480941607332326,  // .menu
		`dir/日本語.anything`:   10815372602820151964, // 日本語.menu
		`C:\mixed\I.MENU`:    5744843728918987537,  // i.menu
	}
	for input, want := range tests {
		if got := KCESBridgeMenuFileID(input); got != want {
			t.Errorf("KCESBridgeMenuFileID(%q) = %d (0x%x), want %d (0x%x)", input, got, got, want, want)
		}
	}
}

func TestEncodeKCESBridgeSessionDoesNotApplyIgnoredNameCallback(t *testing.T) {
	names := []string{`C:\mods\FOO.MATE`, `D:/other/foo.model`, `dir/日本語.anything`}
	value := &KCESBridgeSession{
		Format:           KCESBridgeSessionFormat,
		ContainerVersion: 4321,
		SessionData: &KCESBridgeSessionData{
			Version:           99,
			SessionID:         "session-1",
			HideMenuFileIDs:   nil,
			HideMenuFileNames: &names,
		},
		ExtraFiles: map[string][]byte{"future": {1, 2, 3}},
	}
	before := cloneBridgeSessionForTest(value)
	encoded, err := EncodeKCESBridgeSession(value)
	if err != nil {
		t.Fatalf("encode names annotation: %v", err)
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatal("name hashing mutated the caller")
	}
	decoded, err := DecodeKCESBridgeSession(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.SessionData.HideMenuFileIDs != nil {
		t.Fatalf("encoder derived IDs from ignored names: %#v", decoded.SessionData.HideMenuFileIDs)
	}
	if decoded.SessionData.HideMenuFileNames != nil {
		t.Fatalf("ignored names unexpectedly appeared on the wire: %#v", decoded.SessionData.HideMenuFileNames)
	}

	matchingNames := []string{`FOO.MATE`}
	matching := validBridgeSessionForTest()
	matching.SessionData.HideMenuFileIDs = []uint64{15816416568107797580}
	matching.SessionData.HideMenuFileNames = &matchingNames
	matching.SessionData.HideMenuFileIDs = []uint64{1}
	wire, err := EncodeKCESBridgeSession(matching)
	if err != nil {
		t.Fatalf("opaque IDs should not be checked against ignored names: %v", err)
	}
	got, err := DecodeKCESBridgeSession(wire)
	if err != nil || !reflect.DeepEqual(got.SessionData.HideMenuFileIDs, []uint64{1}) {
		t.Fatalf("explicit IDs changed: %#v, %v", got, err)
	}

	for name, valid := range map[string]*KCESBridgeSession{
		"nil sessionData": {Format: KCESBridgeSessionFormat},
		"empty sessionId": {Format: KCESBridgeSessionFormat, SessionData: &KCESBridgeSessionData{HideMenuFileIDs: []uint64{}}},
		"nil IDs":         {Format: KCESBridgeSessionFormat, SessionData: &KCESBridgeSessionData{SessionID: "x"}},
		"duplicate IDs":   {Format: KCESBridgeSessionFormat, SessionData: &KCESBridgeSessionData{SessionID: "x", HideMenuFileIDs: []uint64{1, 1}}},
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := EncodeKCESBridgeSession(valid)
			if err != nil {
				t.Fatalf("representable value rejected: %v", err)
			}
			if _, err := DecodeKCESBridgeSession(encoded); err != nil {
				t.Fatalf("representable value did not round trip: %v", err)
			}
		})
	}

	tests := map[string]*KCESBridgeSession{
		"nil": nil,
		"wrong format": {
			Format: "future-format", SessionData: validBridgeSessionForTest().SessionData,
		},
		"invalid future slot": {
			Format: KCESBridgeSessionFormat, SessionData: &KCESBridgeSessionData{SessionID: "x", HideMenuFileIDs: []uint64{}, FutureSlots: [][]byte{{0x01, 0x02}}},
		},
		"empty future slot": {
			Format: KCESBridgeSessionFormat, SessionData: &KCESBridgeSessionData{SessionID: "x", HideMenuFileIDs: []uint64{}, FutureSlots: [][]byte{{}}},
		},
		"reserved extra": {
			Format: KCESBridgeSessionFormat, SessionData: validBridgeSessionForTest().SessionData, ExtraFiles: map[string][]byte{"session_id": {1}},
		},
		"unsafe extra path": {
			Format: KCESBridgeSessionFormat, SessionData: validBridgeSessionForTest().SessionData, ExtraFiles: map[string][]byte{"../escape": {1}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := EncodeKCESBridgeSession(test); err == nil {
				t.Fatal("invalid bridge session unexpectedly encoded")
			}
		})
	}
}

func TestKCESBridgeSessionFutureSlotDepthLimit(t *testing.T) {
	deep := bytes.Repeat([]byte{0x91}, simpleEditDataMaxDepth+1)
	deep = append(deep, 0xc0)
	value := validBridgeSessionForTest()
	value.SessionData.FutureSlots = [][]byte{deep}
	if _, err := EncodeKCESBridgeSession(value); err == nil {
		t.Fatal("over-deep future MessagePack slot unexpectedly encoded")
	}
}

func makeBridgeSessionVirtualDirectory(t *testing.T, version int, sessionData, sessionID []byte, extra map[string][]byte) []byte {
	t.Helper()
	table := &ct.ContentTable{Version: version, Raw: make([]byte, ct.HeaderSize)}
	table.AddFile("session_data", append([]byte(nil), sessionData...))
	table.AddFile("session_id", append([]byte(nil), sessionID...))
	for name, data := range extra {
		table.AddFile(name, append([]byte(nil), data...))
	}
	var out bytes.Buffer
	if err := ct.WriteContentTable(&out, table); err != nil {
		t.Fatalf("write test VirtualDirectory: %v", err)
	}
	return out.Bytes()
}

func validBridgeSessionForTest() *KCESBridgeSession {
	return &KCESBridgeSession{
		Format:           KCESBridgeSessionFormat,
		ContainerVersion: KCESBridgeSessionContainerVersion,
		SessionData: &KCESBridgeSessionData{
			Version:         KCESBridgeSessionDataVersion,
			SessionID:       "x",
			HideMenuFileIDs: []uint64{},
		},
	}
}

func cloneBridgeSessionForTest(value *KCESBridgeSession) *KCESBridgeSession {
	if value == nil {
		return nil
	}
	clone := *value
	if value.SessionData != nil {
		data := *value.SessionData
		data.HideMenuFileIDs = append([]uint64(nil), value.SessionData.HideMenuFileIDs...)
		if value.SessionData.HideMenuFileIDs != nil && data.HideMenuFileIDs == nil {
			data.HideMenuFileIDs = []uint64{}
		}
		if value.SessionData.HideMenuFileNames != nil {
			names := append([]string(nil), *value.SessionData.HideMenuFileNames...)
			if *value.SessionData.HideMenuFileNames != nil && names == nil {
				names = []string{}
			}
			data.HideMenuFileNames = &names
		}
		if value.SessionData.FutureSlots != nil {
			data.FutureSlots = make([][]byte, len(value.SessionData.FutureSlots))
			for i, slot := range value.SessionData.FutureSlots {
				data.FutureSlots[i] = append([]byte(nil), slot...)
			}
		}
		clone.SessionData = &data
	}
	if value.ExtraFiles != nil {
		clone.ExtraFiles = make(map[string][]byte, len(value.ExtraFiles))
		for name, data := range value.ExtraFiles {
			clone.ExtraFiles[name] = append([]byte(nil), data...)
		}
	}
	return &clone
}
