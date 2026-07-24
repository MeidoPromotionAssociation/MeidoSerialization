package ct

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/ugorji/go/codec"
)

func TestVirtualDirectoryFixedLayoutRoundTrip(t *testing.T) {
	root := []interface{}{
		int64(1000),
		map[string]interface{}{
			"empty": []interface{}{
				int64(1001),
				map[string]interface{}{},
				map[string]interface{}{},
			},
			"nested": []interface{}{
				int64(1002),
				map[string]interface{}{
					"leaf": []interface{}{
						int64(1003),
						map[string]interface{}{},
						map[string]interface{}{},
					},
				},
				map[string]interface{}{
					"empty.bin": []interface{}{int64(HeaderSize), int64(0)},
				},
			},
		},
		map[string]interface{}{},
	}
	table, err := ReadContentTable(bytes.NewReader(makeVirtualDirectoryTestContainer(t, encodeVirtualDirectoryTestValue(t, root))))
	if err != nil {
		t.Fatalf("ReadContentTable() error = %v", err)
	}
	if table.Version != 1000 {
		t.Fatalf("Version = %d, want 1000", table.Version)
	}
	wantDirectories := map[string]VirtualDirectoryMetadata{
		"empty":       {Version: 1001},
		"nested":      {Version: 1002},
		"nested/leaf": {Version: 1003},
	}
	if len(table.Directories) != len(wantDirectories) {
		t.Fatalf("Directories = %#v, want %#v", table.Directories, wantDirectories)
	}
	for path, want := range wantDirectories {
		if got, ok := table.Directories[path]; !ok || got != want {
			t.Fatalf("Directories[%q] = %#v, exists=%v, want %#v", path, got, ok, want)
		}
	}
	if got, ok := table.Files["nested/empty.bin"]; !ok || got != (VirtualFile{Position: HeaderSize, Size: 0}) {
		t.Fatalf("Files[nested/empty.bin] = %#v, exists=%v", got, ok)
	}

	var rewritten bytes.Buffer
	if err := WriteContentTable(&rewritten, table); err != nil {
		t.Fatalf("WriteContentTable() error = %v", err)
	}
	roundTrip, err := ReadContentTable(bytes.NewReader(rewritten.Bytes()))
	if err != nil {
		t.Fatalf("ReadContentTable(round trip) error = %v", err)
	}
	if roundTrip.Version != table.Version || len(roundTrip.Directories) != len(table.Directories) || len(roundTrip.Files) != len(table.Files) {
		t.Fatalf("round-trip table changed:\n got  %#v\n want %#v", roundTrip, table)
	}
	for path, want := range table.Directories {
		if got := roundTrip.Directories[path]; got != want {
			t.Fatalf("round-trip Directories[%q] = %#v, want %#v", path, got, want)
		}
	}
}

func TestVirtualDirectoryRejectsUnsupportedShapes(t *testing.T) {
	emptyMap := map[string]interface{}{}
	validFile := []interface{}{int64(HeaderSize), int64(0)}
	validDirectory := func() []interface{} {
		return []interface{}{int64(1000), emptyMap, emptyMap}
	}
	tests := []struct {
		name    string
		root    []interface{}
		message string
	}{
		{name: "short root", root: []interface{}{int64(1000), emptyMap}, message: "root indexed-array width 2, expected 3"},
		{name: "high root slot", root: []interface{}{int64(1000), emptyMap, emptyMap, nil}, message: "root indexed-array width 4, expected 3"},
		{name: "nil root directories", root: []interface{}{int64(1000), nil, emptyMap}, message: "allDirectorys must not be nil"},
		{name: "nil root files", root: []interface{}{int64(1000), emptyMap, nil}, message: "allFiles must not be nil"},
		{name: "short child", root: []interface{}{int64(1000), map[string]interface{}{"bad": []interface{}{int64(1000), emptyMap}}, emptyMap}, message: "indexed-array width 2, expected 3"},
		{name: "high child slot", root: []interface{}{int64(1000), map[string]interface{}{"bad": append(validDirectory(), nil)}, emptyMap}, message: "indexed-array width 4, expected 3"},
		{name: "nil child directories", root: []interface{}{int64(1000), map[string]interface{}{"bad": []interface{}{int64(1000), nil, emptyMap}}, emptyMap}, message: "allDirectorys must not be nil"},
		{name: "nil child files", root: []interface{}{int64(1000), map[string]interface{}{"bad": []interface{}{int64(1000), emptyMap, nil}}, emptyMap}, message: "allFiles must not be nil"},
		{name: "short file", root: []interface{}{int64(1000), emptyMap, map[string]interface{}{"bad.bin": []interface{}{int64(HeaderSize)}}}, message: "VirtualFile indexed-array width 1, expected 2"},
		{name: "high file slot", root: []interface{}{int64(1000), emptyMap, map[string]interface{}{"bad.bin": append(validFile, nil)}}, message: "VirtualFile indexed-array width 3, expected 2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire := makeVirtualDirectoryTestContainer(t, encodeVirtualDirectoryTestValue(t, test.root))
			_, err := ReadContentTable(bytes.NewReader(wire))
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("ReadContentTable() error = %v, want substring %q", err, test.message)
			}
		})
	}
}

func TestVirtualDirectoryRejectsTrailingMessagePackData(t *testing.T) {
	metadata := encodeVirtualDirectoryTestValue(t, []interface{}{
		int64(1000),
		map[string]interface{}{},
		map[string]interface{}{},
	})
	metadata = append(metadata, 0xc0)
	_, err := ReadContentTable(bytes.NewReader(makeVirtualDirectoryTestContainer(t, metadata)))
	if err == nil || !strings.Contains(err.Error(), "trailing MessagePack bytes") {
		t.Fatalf("ReadContentTable() error = %v, want trailing MessagePack bytes", err)
	}
}

func encodeVirtualDirectoryTestValue(t *testing.T, value interface{}) []byte {
	t.Helper()
	h := &codec.MsgpackHandle{}
	h.Raw = true
	var metadata []byte
	if err := codec.NewEncoderBytes(&metadata, h).Encode(value); err != nil {
		t.Fatalf("encode VirtualDirectory metadata: %v", err)
	}
	return metadata
}

func makeVirtualDirectoryTestContainer(t *testing.T, metadata []byte) []byte {
	t.Helper()
	compressed, err := CompressLz4BlockArray(metadata)
	if err != nil {
		t.Fatalf("compress VirtualDirectory metadata: %v", err)
	}
	wire := append(append([]byte(nil), FileSignature...), SerializeTypeMsgPack)
	wire = append(wire, compressed...)
	var footer [4]byte
	binary.LittleEndian.PutUint32(footer[:], uint32(len(compressed)))
	return append(wire, footer[:]...)
}
