package binaryio

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestReadBytesHugeTruncatedInputIsReadProgressively(t *testing.T) {
	data, err := ReadBytes(bytes.NewReader([]byte{0x42}), 1<<31-1)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadBytes error = %v, want io.ErrUnexpectedEOF", err)
	}
	if !bytes.Equal(data, []byte{0x42}) {
		t.Fatalf("ReadBytes partial data = %x", data)
	}
}

func TestReadStringHasNoArbitrarySizeLimit(t *testing.T) {
	var wire bytes.Buffer
	if err := Write7BitEncodedInt(&wire, 10*1024*1024+1); err != nil {
		t.Fatalf("write length: %v", err)
	}
	_, err := ReadString(bytes.NewReader(wire.Bytes()))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadString error = %v, want truncated-data error rather than a size limit", err)
	}
}

func TestReadStringRejectsLengthOutsideNonNegativeInt32(t *testing.T) {
	// 0x80000000 encoded as a 7-bit integer. A .NET string length is a
	// non-negative Int32, so this marker cannot describe a string.
	wire := []byte{0x80, 0x80, 0x80, 0x80, 0x08}
	if _, err := ReadString(bytes.NewReader(wire)); err == nil {
		t.Fatal("ReadString accepted a length outside non-negative Int32")
	}
}

func TestRead7BitEncodedIntRoundTripsNegativeValues(t *testing.T) {
	for _, value := range []int32{-1, -2, -1 << 31} {
		var wire bytes.Buffer
		if err := Write7BitEncodedInt(&wire, value); err != nil {
			t.Fatalf("Write7BitEncodedInt(%d): %v", value, err)
		}
		got, err := Read7BitEncodedInt(&wire)
		if err != nil {
			t.Fatalf("Read7BitEncodedInt(%d): %v", value, err)
		}
		if got != value {
			t.Fatalf("Read7BitEncodedInt round trip = %d, want %d", got, value)
		}
	}
}

func TestRead7BitEncodedIntRejectsBitsOutsideInt32(t *testing.T) {
	wire := []byte{0x80, 0x80, 0x80, 0x80, 0x10}
	if _, err := Read7BitEncodedInt(bytes.NewReader(wire)); err == nil {
		t.Fatal("Read7BitEncodedInt accepted bits outside Int32")
	}
}
