package ct

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/ugorji/go/codec"
)

func TestVirtualDirectoryPreservesNestedWireMetadata(t *testing.T) {
	rootFuture := codec.Raw{0xd4, 0x2a, 0x7f}
	emptyFuture := codec.Raw{0xcc, 0x01}
	legacyFuture := codec.Raw{0x81, 0xa1, 'x', 0x92, 0xc0, 0xc3}
	fileFuture := codec.Raw{0xd6, 0x03, 0, 0, 0, 7}

	root := []interface{}{
		int64(-3),
		map[string]interface{}{
			"emptyNil": []interface{}{int64(-5), nil, nil, emptyFuture},
			"legacy":   []interface{}{map[string]interface{}{}, nil, legacyFuture},
			"filesOnly": []interface{}{
				int64(77),
				map[string]interface{}{
					"short.bin": []interface{}{int64(HeaderSize), int64(0), fileFuture},
				},
			},
			"short": []interface{}{int64(0)},
		},
		nil,
		rootFuture,
	}
	wire := makeVirtualDirectoryTestContainer(t, root)

	table, err := ReadContentTable(bytes.NewReader(wire))
	if err != nil {
		t.Fatalf("ReadContentTable() error = %v", err)
	}
	if table.Version != -3 || table.Versionless || table.FilesOnly || !table.FilesNil || table.FieldCount == nil || *table.FieldCount != 4 || !reflect.DeepEqual(table.FutureSlots, [][]byte{rootFuture}) {
		t.Fatalf("root metadata changed: %+v", table)
	}
	if len(table.Directories) != 4 {
		t.Fatalf("empty/nested directories were lost: %#v", table.Directories)
	}
	if got := table.Directories["emptyNil"]; got.Version != -5 || !got.DirectoriesNil || !got.FilesNil || got.FieldCount == nil || *got.FieldCount != 4 || !reflect.DeepEqual(got.FutureSlots, [][]byte{emptyFuture}) {
		t.Fatalf("emptyNil metadata = %#v", got)
	}
	if got := table.Directories["legacy"]; !got.Versionless || got.FilesOnly || got.DirectoriesNil || !got.FilesNil || got.FieldCount == nil || *got.FieldCount != 3 || !reflect.DeepEqual(got.FutureSlots, [][]byte{legacyFuture}) {
		t.Fatalf("legacy metadata = %#v", got)
	}
	if got := table.Directories["filesOnly"]; got.Version != 77 || !got.FilesOnly || got.Versionless || got.FieldCount != nil {
		t.Fatalf("filesOnly metadata = %#v", got)
	}
	if got := table.Directories["short"]; got.Version != 0 || got.Versionless || got.FieldCount == nil || *got.FieldCount != 1 {
		t.Fatalf("short metadata = %#v", got)
	}
	file, ok := table.Files["filesOnly/short.bin"]
	if !ok || file.Position != HeaderSize || file.Size != 0 || file.FieldCount == nil || *file.FieldCount != 3 || !reflect.DeepEqual(file.FutureSlots, [][]byte{fileFuture}) {
		t.Fatalf("future VirtualFile metadata = %#v, exists=%v", file, ok)
	}

	var rewritten bytes.Buffer
	if err := WriteContentTable(&rewritten, table); err != nil {
		t.Fatalf("WriteContentTable() error = %v", err)
	}
	roundTrip, err := ReadContentTable(bytes.NewReader(rewritten.Bytes()))
	if err != nil {
		t.Fatalf("ReadContentTable(round trip) error = %v", err)
	}
	if roundTrip.Version != table.Version || roundTrip.Versionless != table.Versionless || roundTrip.FilesOnly != table.FilesOnly || roundTrip.DirectoriesNil != table.DirectoriesNil || roundTrip.FilesNil != table.FilesNil || !reflect.DeepEqual(roundTrip.FieldCount, table.FieldCount) || !reflect.DeepEqual(roundTrip.FutureSlots, table.FutureSlots) {
		t.Fatalf("root shape changed:\n got  %+v\n want %+v", roundTrip, table)
	}
	if !reflect.DeepEqual(roundTrip.Directories, table.Directories) {
		t.Fatalf("directory metadata changed:\n got  %#v\n want %#v", roundTrip.Directories, table.Directories)
	}
	if !reflect.DeepEqual(roundTrip.Files, table.Files) {
		t.Fatalf("VirtualFile metadata changed:\n got  %#v\n want %#v", roundTrip.Files, table.Files)
	}
}

func TestVirtualDirectoryPreservesShortVirtualFiles(t *testing.T) {
	for _, fields := range [][]interface{}{
		{},
		{int64(HeaderSize)},
	} {
		root := []interface{}{
			int64(0),
			map[string]interface{}{},
			map[string]interface{}{"empty.bin": fields},
		}
		wire := makeVirtualDirectoryTestContainer(t, root)
		table, err := ReadContentTable(bytes.NewReader(wire))
		if err != nil {
			t.Fatalf("ReadContentTable(%v) error = %v", fields, err)
		}
		file := table.Files["empty.bin"]
		if file.FieldCount == nil || *file.FieldCount != int32(len(fields)) || file.Size != 0 {
			t.Fatalf("short VirtualFile %v decoded as %#v", fields, file)
		}
		var out bytes.Buffer
		if err := WriteContentTable(&out, table); err != nil {
			t.Fatalf("WriteContentTable(%v) error = %v", fields, err)
		}
		roundTrip, err := ReadContentTable(bytes.NewReader(out.Bytes()))
		if err != nil {
			t.Fatalf("ReadContentTable(round trip %v) error = %v", fields, err)
		}
		if got := roundTrip.Files["empty.bin"]; got.FieldCount == nil || *got.FieldCount != int32(len(fields)) || got.Size != 0 {
			t.Fatalf("short VirtualFile width changed: %#v", got)
		}
	}
}

func TestVirtualDirectoryRejectsMalformedFutureSlot(t *testing.T) {
	fieldCount := int32(4)
	table := &ContentTable{
		Version:     0,
		FieldCount:  &fieldCount,
		FutureSlots: [][]byte{{0x92, 0x01}},
		Raw:         make([]byte, HeaderSize),
		Files:       map[string]VirtualFile{},
	}
	var out bytes.Buffer
	if err := WriteContentTable(&out, table); err == nil {
		t.Fatal("WriteContentTable unexpectedly accepted a truncated future slot")
	}
	if out.Len() != 0 {
		t.Fatalf("writer received %d bytes before future-slot validation failed", out.Len())
	}
}

func makeVirtualDirectoryTestContainer(t *testing.T, root []interface{}) []byte {
	t.Helper()
	h := &codec.MsgpackHandle{}
	h.Raw = true
	var metadata []byte
	if err := codec.NewEncoderBytes(&metadata, h).Encode(root); err != nil {
		t.Fatalf("encode VirtualDirectory metadata: %v", err)
	}
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
