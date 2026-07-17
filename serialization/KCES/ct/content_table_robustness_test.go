package ct

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestContentTableGetFileDataRejectsInvalidRangesWithoutPanic(t *testing.T) {
	tests := []VirtualFile{
		{Position: 8, Size: -1},
		{Position: -1, Size: 1},
		{Position: math.MaxInt64, Size: 1},
		{Position: 15, Size: 2},
	}
	for _, file := range tests {
		t.Run(fmt.Sprintf("position=%d,size=%d", file.Position, file.Size), func(t *testing.T) {
			table := &ContentTable{
				Raw:   make([]byte, 16),
				Files: map[string]VirtualFile{"x": file},
			}
			assertCTErrorWithoutPanic(t, func() error {
				_, err := table.GetFileData("x")
				return err
			}, "out of bounds")
		})
	}
}

func TestWriteContentTableRejectsInvalidFileRange(t *testing.T) {
	table := &ContentTable{
		Raw:   make([]byte, HeaderSize),
		Files: map[string]VirtualFile{"x": {Position: HeaderSize, Size: 1}},
	}
	assertCTErrorWithoutPanic(t, func() error {
		return WriteContentTable(&bytes.Buffer{}, table)
	}, "out of bounds")
}

func TestDecodeMsgpackFileReportsCorruptCompression(t *testing.T) {
	compressed, err := CompressLz4BlockArray(bytes.Repeat([]byte{0x91, 0x01}, 100))
	if err != nil {
		t.Fatalf("CompressLz4BlockArray: %v", err)
	}
	compressed = compressed[:len(compressed)-1]
	table := &ContentTable{
		Raw:   compressed,
		Files: map[string]VirtualFile{"catalog": {Position: 0, Size: len(compressed)}},
	}
	var out interface{}
	err = table.DecodeMsgpackFile("catalog", &out)
	if err == nil {
		t.Fatal("truncated compressed file unexpectedly decoded")
	}
	if !strings.Contains(err.Error(), `decompress content table file "catalog"`) {
		t.Fatalf("error %q does not retain decompression context", err)
	}
}

func TestContentTableDecodeMsgpackFile_PropagatesRecognizedEnvelopeErrors(t *testing.T) {
	raw, err := EncodeMsgpack([]interface{}{int64(1000), "payload long enough to produce a compressed envelope"})
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := CompressLz4BlockArray(raw)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := compressed[:len(compressed)-1]
	table := &ContentTable{
		Files: map[string]VirtualFile{"catalog": {Position: 0, Size: len(corrupt)}},
		Raw:   corrupt,
	}

	var out []interface{}
	err = table.DecodeMsgpackFile("catalog", &out)
	if err == nil {
		t.Fatal("corrupt recognized LZ4 envelope unexpectedly decoded")
	}
	if !strings.Contains(err.Error(), "decompress content table file \"catalog\"") {
		t.Fatalf("error lost decompression context: %v", err)
	}
}

func TestDecodeVirtualDirectoryRejectsMalformedFilesInsteadOfDroppingThem(t *testing.T) {
	tests := []struct {
		name string
		file string
		data interface{}
		want string
	}{
		{name: "wrong_size_type", file: "bad.bin", data: []interface{}{int64(HeaderSize), "one"}, want: "position/size"},
		{name: "non_string_map_key", file: "", data: map[interface{}]interface{}{int64(1): []interface{}{int64(HeaderSize), int64(1)}}, want: "map key must be string"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var files interface{}
			if tc.file == "" {
				files = tc.data
			} else {
				files = map[string]interface{}{tc.file: tc.data}
			}
			encoded, err := EncodeMsgpack([]interface{}{int64(ctVersion), map[string]interface{}{}, files})
			if err != nil {
				t.Fatal(err)
			}
			table := &ContentTable{}
			err = table.decodeVirtualDirectory(encoded)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("decode error got %v, want context %q", err, tc.want)
			}
			if len(table.Files) != 0 {
				t.Fatalf("malformed entry produced files: %#v", table.Files)
			}
		})
	}
}

func TestDecodeVirtualDirectoryHistoricalDirectoriesAndFiles(t *testing.T) {
	value := []interface{}{
		map[string]interface{}{
			"nested": []interface{}{
				map[string]interface{}{},
				map[string]interface{}{"child.bin": []interface{}{int64(HeaderSize), int64(1)}},
			},
		},
		map[string]interface{}{"root.bin": []interface{}{int64(HeaderSize + 1), int64(1)}},
	}
	encoded, err := EncodeMsgpack(value)
	if err != nil {
		t.Fatal(err)
	}
	table := &ContentTable{}
	if err := table.decodeVirtualDirectory(encoded); err != nil {
		t.Fatalf("decode historical VirtualDirectory: %v", err)
	}
	if !table.Versionless || table.Version != 0 {
		t.Fatalf("historical version state = versionless %v version %d, want true/0", table.Versionless, table.Version)
	}
	if _, ok := table.Files["nested/child.bin"]; !ok {
		t.Fatalf("nested historical file was lost: %#v", table.Files)
	}
	if _, ok := table.Files["root.bin"]; !ok {
		t.Fatalf("root historical file was lost: %#v", table.Files)
	}

	table.Raw = append(make([]byte, HeaderSize), 0x11, 0x22)
	var rewritten bytes.Buffer
	if err := WriteContentTable(&rewritten, table); err != nil {
		t.Fatalf("rewrite historical VirtualDirectory: %v", err)
	}
	roundTrip, err := ReadContentTable(bytes.NewReader(rewritten.Bytes()))
	if err != nil {
		t.Fatalf("read rewritten historical VirtualDirectory: %v", err)
	}
	if !roundTrip.Versionless || roundTrip.Version != 0 {
		t.Fatalf("historical form was upgraded: versionless %v version %d", roundTrip.Versionless, roundTrip.Version)
	}
	if got, err := roundTrip.GetFileData("nested/child.bin"); err != nil || !bytes.Equal(got, []byte{0x11}) {
		t.Fatalf("rewritten nested payload = %x, %v", got, err)
	}
	if got, err := roundTrip.GetFileData("root.bin"); err != nil || !bytes.Equal(got, []byte{0x22}) {
		t.Fatalf("rewritten root payload = %x, %v", got, err)
	}
}

func TestVirtualDirectoryRejectsUnsafePathsBeforeWriting(t *testing.T) {
	unsafeNames := []string{
		"../escape",
		"nested/../../escape",
		"/absolute/path",
		`C:\absolute\path`,
		`\\server\share\path`,
		"nested//file",
		"./file",
	}
	for _, name := range unsafeNames {
		t.Run(name, func(t *testing.T) {
			table := &ContentTable{Raw: []byte{1}, Files: map[string]VirtualFile{name: {Position: 0, Size: 1}}}
			var out bytes.Buffer
			err := WriteContentTable(&out, table)
			if err == nil || !strings.Contains(err.Error(), "invalid virtual file name") {
				t.Fatalf("WriteContentTable(%q) error got %v", name, err)
			}
			if out.Len() != 0 {
				t.Fatalf("writer received %d bytes before unsafe name was rejected", out.Len())
			}
		})
	}
}

func TestVirtualDirectoryAllowsGameColonNamesButExtractionLayerCanRejectThem(t *testing.T) {
	name := "MoveablePanelManager::SceneEdit::savedata"
	table := &ContentTable{
		Version: ctVersion,
		Raw:     make([]byte, HeaderSize),
		Files:   map[string]VirtualFile{},
	}
	table.AddFile(name, []byte("state"))
	var encoded bytes.Buffer
	if err := WriteContentTable(&encoded, table); err != nil {
		t.Fatalf("WriteContentTable(game colon name): %v", err)
	}
	decoded, err := ReadContentTable(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("ReadContentTable(game colon name): %v", err)
	}
	data, err := decoded.GetFileData(name)
	if err != nil || string(data) != "state" {
		t.Fatalf("colon-name payload = %q, err=%v", data, err)
	}
}

func TestContentTableReadAddReplaceWriteRoundTrip(t *testing.T) {
	original := &ContentTable{
		Version: ctVersion,
		Raw:     make([]byte, HeaderSize),
		Files:   map[string]VirtualFile{},
	}
	original.AddFile("old", []byte("old-data"))
	var first bytes.Buffer
	if err := WriteContentTable(&first, original); err != nil {
		t.Fatal(err)
	}

	read, err := ReadContentTable(bytes.NewReader(first.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	read.AddFile("new", []byte("new-data"))
	read.AddFile("old", []byte("replacement"))
	var second bytes.Buffer
	if err := WriteContentTable(&second, read); err != nil {
		t.Fatalf("WriteContentTable after AddFile: %v", err)
	}

	final, err := ReadContentTable(bytes.NewReader(second.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"old": "replacement", "new": "new-data"} {
		got, err := final.GetFileData(name)
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, err=%v; want %q", name, got, err, want)
		}
	}
}

func TestContentTableGetFileDataDoesNotExposeMetadataArea(t *testing.T) {
	table := &ContentTable{
		Raw:     make([]byte, 64),
		dataEnd: 32,
		Files:   map[string]VirtualFile{"metadata.bin": {Position: 31, Size: 2}},
	}
	if _, err := table.GetFileData("metadata.bin"); err == nil || !strings.Contains(err.Error(), "out of bounds") {
		t.Fatalf("metadata-overlap error got %v", err)
	}
}

func TestContentTableIntegerConversionsRejectOverflow(t *testing.T) {
	if _, ok := toInt(uint64(^uint(0)>>1) + 1); ok {
		t.Fatal("toInt accepted uint larger than MaxInt")
	}
	if _, ok := toInt64(uint64(^uint64(0)>>1) + 1); ok {
		t.Fatal("toInt64 accepted uint larger than MaxInt64")
	}
}

func assertCTErrorWithoutPanic(t *testing.T, fn func() error, want string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	err := fn()
	if err == nil {
		t.Fatal("expected an error")
	}
	if want != "" && !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}
