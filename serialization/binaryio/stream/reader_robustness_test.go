package stream

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestReadBytesHugeTruncatedInputIsReadProgressively(t *testing.T) {
	reader := NewBinaryReader(bytes.NewReader([]byte{0x42}))
	data, err := reader.ReadBytes(1<<31 - 1)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadBytes error = %v, want io.ErrUnexpectedEOF", err)
	}
	if !bytes.Equal(data, []byte{0x42}) {
		t.Fatalf("ReadBytes partial data = %x", data)
	}
}

func TestReadStringHasNoArbitrarySizeLimit(t *testing.T) {
	var wire bytes.Buffer
	w := NewBinaryWriter(&wire)
	if err := w.write7BitEncodedInt(10*1024*1024 + 1); err != nil {
		t.Fatalf("write length: %v", err)
	}
	_, err := NewBinaryReader(bytes.NewReader(wire.Bytes())).ReadString()
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadString error = %v, want truncated-data error rather than a size limit", err)
	}
}

func TestReadStringRejectsLengthOutsideNonNegativeInt32(t *testing.T) {
	// 0x80000000 encoded as a 7-bit integer. A .NET string length is a
	// non-negative Int32, so this marker cannot describe a string.
	wire := []byte{0x80, 0x80, 0x80, 0x80, 0x08}
	if _, err := NewBinaryReader(bytes.NewReader(wire)).ReadString(); err == nil {
		t.Fatal("ReadString accepted a length outside non-negative Int32")
	}
}
