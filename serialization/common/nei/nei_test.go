package nei

import (
	"bytes"
	"math"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func mustWrite(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

// buildWire encrypts a NEI table whose cells hold the given raw bytes in index order.
func buildWire(t *testing.T, cols uint32, rows uint32, cells [][]byte) []byte {
	t.Helper()
	var plain bytes.Buffer
	writer := stream.NewBinaryWriter(&plain)
	mustWrite(t, writer.WriteBytes(Signature))
	mustWrite(t, writer.WriteUInt32(cols))
	mustWrite(t, writer.WriteUInt32(rows))

	var offset uint32
	for _, cell := range cells {
		if len(cell) == 0 {
			mustWrite(t, writer.WriteUInt32(0))
			mustWrite(t, writer.WriteUInt32(0))
			continue
		}
		length := uint32(len(cell)) + 1
		mustWrite(t, writer.WriteUInt32(offset))
		mustWrite(t, writer.WriteUInt32(length))
		offset += length
	}
	for _, cell := range cells {
		if len(cell) == 0 {
			continue
		}
		mustWrite(t, writer.WriteBytes(cell))
		mustWrite(t, writer.WriteByte(0))
	}

	encrypted, err := encryptData(plain.Bytes(), Key, nil)
	if err != nil {
		t.Fatalf("encrypt fixture: %v", err)
	}
	return encrypted
}

func TestReadUsesCellOffsetsAndRejectsImpossibleDimensions(t *testing.T) {
	decoded, err := Read(bytes.NewReader(buildWire(t, 2, 1, [][]byte{[]byte("B"), []byte("A")})), nil)
	if err != nil {
		t.Fatalf("Read(offset fixture): %v", err)
	}
	if !reflect.DeepEqual(decoded.Data, [][]string{{"B", "A"}}) {
		t.Fatalf("offset-indexed cells = %#v, want B/A", decoded.Data)
	}

	var plain bytes.Buffer
	writer := stream.NewBinaryWriter(&plain)
	mustWrite(t, writer.WriteBytes(Signature))
	mustWrite(t, writer.WriteUInt32(^uint32(0)))
	mustWrite(t, writer.WriteUInt32(2))
	impossible, err := encryptData(plain.Bytes(), Key, nil)
	if err != nil {
		t.Fatalf("encrypt fixture: %v", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Read panicked for impossible dimensions: %v", recovered)
		}
	}()
	if _, err := Read(bytes.NewReader(impossible), nil); err == nil {
		t.Fatal("Read accepted dimensions whose index table cannot fit")
	}
}

func TestDumpPreservesExplicitDimensionsAndRejectsShapeMismatch(t *testing.T) {
	empty := &Table{Rows: 0, Cols: 0, Data: [][]string{}}
	var encoded bytes.Buffer
	if err := empty.Dump(&encoded); err != nil {
		t.Fatalf("Dump(empty): %v", err)
	}
	if empty.Rows != 0 || empty.Cols != 0 || len(empty.Data) != 0 {
		t.Fatalf("Dump mutated empty table: %#v", empty)
	}
	decoded, err := Read(bytes.NewReader(encoded.Bytes()), nil)
	if err != nil {
		t.Fatalf("Read(empty): %v", err)
	}
	if decoded.Rows != 0 || decoded.Cols != 0 || len(decoded.Data) != 0 {
		t.Fatalf("empty table changed: %#v", decoded)
	}

	invalid := &Table{Rows: 1, Cols: 1, Data: [][]string{{"kept", "extra"}}}
	snapshot := &Table{Rows: invalid.Rows, Cols: invalid.Cols, Data: [][]string{append([]string(nil), invalid.Data[0]...)}}
	encoded.Reset()
	if err := invalid.Dump(&encoded); err == nil {
		t.Fatal("Dump accepted Data whose shape does not match explicit Rows/Cols")
	}
	if !reflect.DeepEqual(invalid, snapshot) {
		t.Fatalf("Dump mutated invalid table\n got: %#v\nwant: %#v", invalid, snapshot)
	}
	if encoded.Len() != 0 {
		t.Fatalf("invalid table wrote %d bytes", encoded.Len())
	}
}

func TestZeroWidthRowsUseStoredDimensionWithoutRowAllocation(t *testing.T) {
	value := &Table{Rows: math.MaxUint32, Cols: 0}
	var encoded bytes.Buffer
	if err := value.Dump(&encoded); err != nil {
		t.Fatalf("Dump(zero-width max rows): %v", err)
	}
	decoded, err := Read(bytes.NewReader(encoded.Bytes()), nil)
	if err != nil {
		t.Fatalf("Read(zero-width max rows): %v", err)
	}
	if decoded.Rows != math.MaxUint32 || decoded.Cols != 0 || len(decoded.Data) != 0 {
		t.Fatalf("zero-width dimensions changed: %#v", decoded)
	}
}

func TestReadDetectsCellTextEncodingPerGame(t *testing.T) {
	// 大カテゴリ / 身体 as COM3D2 writes them (Shift-JIS) and as KCES writes them (UTF-8).
	shiftJISCells := [][]byte{
		{0x91, 0xE5, 0x83, 0x4A, 0x83, 0x65, 0x83, 0x53, 0x83, 0x8A},
		{0x90, 0x67, 0x91, 0xCC},
	}
	utf8Cells := [][]byte{
		{0xE5, 0xA4, 0xA7, 0xE3, 0x82, 0xAB, 0xE3, 0x83, 0x86, 0xE3, 0x82, 0xB4, 0xE3, 0x83, 0xAA},
		{0xE8, 0xBA, 0xAB, 0xE4, 0xBD, 0x93},
	}
	asciiCells := [][]byte{[]byte("Body"), []byte("Head")}

	tests := []struct {
		name         string
		cells        [][]byte
		wantEncoding TextEncoding
		wantData     []string
	}{
		{"COM3D2 Shift-JIS", shiftJISCells, TextEncodingShiftJIS, []string{"大カテゴリ", "身体"}},
		{"KCES UTF-8", utf8Cells, TextEncodingUTF8, []string{"大カテゴリ", "身体"}},
		{"ASCII only", asciiCells, TextEncodingShiftJIS, []string{"Body", "Head"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := Read(bytes.NewReader(buildWire(t, 2, 1, tt.cells)), nil)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if decoded.TextEncoding != tt.wantEncoding {
				t.Errorf("TextEncoding = %q, want %q", decoded.TextEncoding, tt.wantEncoding)
			}
			if !reflect.DeepEqual(decoded.Data, [][]string{tt.wantData}) {
				t.Errorf("Data = %#v, want %#v", decoded.Data, tt.wantData)
			}
		})
	}
}

func TestDumpWritesCellsInSelectedTextEncoding(t *testing.T) {
	table := &Table{Rows: 1, Cols: 1, Data: [][]string{{"身体"}}}

	for _, tt := range []struct {
		encoding  TextEncoding
		wantBytes []byte
	}{
		{TextEncodingShiftJIS, []byte{0x90, 0x67, 0x91, 0xCC}},
		{TextEncodingUTF8, []byte{0xE8, 0xBA, 0xAB, 0xE4, 0xBD, 0x93}},
		{"", []byte{0x90, 0x67, 0x91, 0xCC}},
	} {
		t.Run(string(tt.encoding), func(t *testing.T) {
			table.TextEncoding = tt.encoding
			var encoded bytes.Buffer
			if err := table.Dump(&encoded); err != nil {
				t.Fatalf("Dump: %v", err)
			}
			plain, err := decryptData(encoded.Bytes(), Key)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if !bytes.Contains(plain, tt.wantBytes) {
				t.Fatalf("cell bytes %X do not contain %X", plain, tt.wantBytes)
			}

			decoded, err := Read(bytes.NewReader(encoded.Bytes()), nil)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if decoded.Data[0][0] != "身体" {
				t.Errorf("round-trip value = %q, want 身体", decoded.Data[0][0])
			}
		})
	}
}

func TestDumpRejectsInvalidCellsForTextEncoding(t *testing.T) {
	invalidUTF8 := &Table{Rows: 1, Cols: 1, Data: [][]string{{string([]byte{0xFF, 0xFE})}}, TextEncoding: TextEncodingUTF8}
	if err := invalidUTF8.Dump(&bytes.Buffer{}); err == nil {
		t.Error("Dump accepted a cell that is not valid UTF-8 for a UTF-8 table")
	}

	unknown := &Table{Rows: 1, Cols: 1, Data: [][]string{{"身体"}}, TextEncoding: "EUC-JP"}
	if err := unknown.Dump(&bytes.Buffer{}); err == nil {
		t.Error("Dump accepted an unknown text encoding")
	}
}
