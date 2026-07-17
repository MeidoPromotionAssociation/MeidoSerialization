package COM3D2

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func TestNei(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.nei")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()

			nei, err := ReadNei(f, nil)
			if err != nil {
				t.Fatalf("failed to read nei: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = nei.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump nei: %v", err)
			}

			// Re-read from dumped buffer
			nei2, err := ReadNei(&buf, nil)
			if err != nil {
				t.Fatalf("failed to re-read dumped nei: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(nei, nei2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}

func TestReadNeiUsesCellOffsetsAndRejectsImpossibleDimensions(t *testing.T) {
	makeEncrypted := func(write func(*stream.BinaryWriter)) []byte {
		t.Helper()
		var plain bytes.Buffer
		writer := stream.NewBinaryWriter(&plain)
		write(writer)
		encrypted, err := encryptData(plain.Bytes(), NeiKey, nil)
		if err != nil {
			t.Fatalf("encrypt fixture: %v", err)
		}
		return encrypted
	}

	offsetWire := makeEncrypted(func(writer *stream.BinaryWriter) {
		countMustWrite(t, writer.WriteBytes(NeiSignature))
		countMustWrite(t, writer.WriteUInt32(2)) // columns
		countMustWrite(t, writer.WriteUInt32(1)) // rows
		countMustWrite(t, writer.WriteUInt32(2))
		countMustWrite(t, writer.WriteUInt32(2))
		countMustWrite(t, writer.WriteUInt32(0))
		countMustWrite(t, writer.WriteUInt32(2))
		countMustWrite(t, writer.WriteBytes([]byte{'B', 0, 'A', 0}))
	})
	decoded, err := ReadNei(bytes.NewReader(offsetWire), nil)
	if err != nil {
		t.Fatalf("ReadNei(offset fixture): %v", err)
	}
	if !reflect.DeepEqual(decoded.Data, [][]string{{"A", "B"}}) {
		t.Fatalf("offset-indexed cells = %#v, want A/B", decoded.Data)
	}

	impossible := makeEncrypted(func(writer *stream.BinaryWriter) {
		countMustWrite(t, writer.WriteBytes(NeiSignature))
		countMustWrite(t, writer.WriteUInt32(^uint32(0)))
		countMustWrite(t, writer.WriteUInt32(2))
	})
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("ReadNei panicked for impossible dimensions: %v", recovered)
		}
	}()
	if _, err := ReadNei(bytes.NewReader(impossible), nil); err == nil {
		t.Fatal("ReadNei accepted dimensions whose index table cannot fit")
	}
}

func TestNeiDumpPreservesExplicitDimensionsAndRejectsShapeMismatch(t *testing.T) {
	empty := &Nei{Rows: 0, Cols: 0, Data: [][]string{}}
	var encoded bytes.Buffer
	if err := empty.Dump(&encoded); err != nil {
		t.Fatalf("Dump(empty): %v", err)
	}
	if empty.Rows != 0 || empty.Cols != 0 || len(empty.Data) != 0 {
		t.Fatalf("Dump mutated empty table: %#v", empty)
	}
	decoded, err := ReadNei(bytes.NewReader(encoded.Bytes()), nil)
	if err != nil {
		t.Fatalf("ReadNei(empty): %v", err)
	}
	if decoded.Rows != 0 || decoded.Cols != 0 || len(decoded.Data) != 0 {
		t.Fatalf("empty table changed: %#v", decoded)
	}

	invalid := &Nei{Rows: 1, Cols: 1, Data: [][]string{{"kept", "extra"}}}
	snapshot := &Nei{Rows: invalid.Rows, Cols: invalid.Cols, Data: [][]string{append([]string(nil), invalid.Data[0]...)}}
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

func TestNeiZeroWidthRowsUseStoredDimensionWithoutRowAllocation(t *testing.T) {
	value := &Nei{Rows: math.MaxUint32, Cols: 0}
	var encoded bytes.Buffer
	if err := value.Dump(&encoded); err != nil {
		t.Fatalf("Dump(zero-width max rows): %v", err)
	}
	decoded, err := ReadNei(bytes.NewReader(encoded.Bytes()), nil)
	if err != nil {
		t.Fatalf("ReadNei(zero-width max rows): %v", err)
	}
	if decoded.Rows != math.MaxUint32 || decoded.Cols != 0 || len(decoded.Data) != 0 {
		t.Fatalf("zero-width dimensions changed: %#v", decoded)
	}
}
