package binaryio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestEndianReaderPrimitivesUseConfiguredByteOrder(t *testing.T) {
	r := NewEndianReader([]byte{
		0x01, 0x02,
		0x03, 0x04, 0x05, 0x06,
		0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e,
	}, binary.BigEndian)

	u16, err := r.ReadUInt16()
	if err != nil {
		t.Fatalf("ReadUInt16: %v", err)
	}
	if u16 != 0x0102 {
		t.Fatalf("ReadUInt16 = %#x, want 0x0102", u16)
	}

	u32, err := r.ReadUInt32()
	if err != nil {
		t.Fatalf("ReadUInt32: %v", err)
	}
	if u32 != 0x03040506 {
		t.Fatalf("ReadUInt32 = %#x, want 0x03040506", u32)
	}

	u64, err := r.ReadUInt64()
	if err != nil {
		t.Fatalf("ReadUInt64: %v", err)
	}
	if u64 != 0x0708090a0b0c0d0e {
		t.Fatalf("ReadUInt64 = %#x, want 0x0708090a0b0c0d0e", u64)
	}
}

func TestEndianReaderAlignedString(t *testing.T) {
	data := []byte{
		0x03, 0x00, 0x00, 0x00,
		'a', 'b', 'c', 0x00,
		0xff,
	}
	r := NewEndianReader(data, binary.LittleEndian)

	s, err := r.ReadAlignedString()
	if err != nil {
		t.Fatalf("ReadAlignedString: %v", err)
	}
	if s != "abc" {
		t.Fatalf("ReadAlignedString = %q, want abc", s)
	}
	if r.Pos() != 8 {
		t.Fatalf("Pos = %d, want 8", r.Pos())
	}
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0xff {
		t.Fatalf("ReadByte = %#x, want 0xff", b)
	}
}

func TestReadNullString(t *testing.T) {
	r := NewEndianReader([]byte("CAB\x00tail"), binary.LittleEndian)
	s, err := r.ReadNullString()
	if err != nil {
		t.Fatalf("EndianReader.ReadNullString: %v", err)
	}
	if s != "CAB" {
		t.Fatalf("EndianReader.ReadNullString = %q, want CAB", s)
	}

	s, err = ReadNullString(bytes.NewReader([]byte("UnityFS\x00rest")))
	if err != nil {
		t.Fatalf("ReadNullString: %v", err)
	}
	if s != "UnityFS" {
		t.Fatalf("ReadNullString = %q, want UnityFS", s)
	}
}

func TestEndianReaderBounds(t *testing.T) {
	r := NewEndianReaderAt([]byte{0x01}, 2, binary.LittleEndian)
	if _, err := r.ReadByte(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadByte error = %v, want io.ErrUnexpectedEOF", err)
	}
	if _, err := r.ReadNullString(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadNullString error = %v, want io.ErrUnexpectedEOF", err)
	}
}
