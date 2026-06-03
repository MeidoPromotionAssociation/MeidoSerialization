package binaryio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestEndianWriterPrimitivesUseConfiguredByteOrder(t *testing.T) {
	var buf bytes.Buffer
	w := NewEndianWriter(&buf, binary.BigEndian)

	mustWrite(t, w.WriteUInt16(0x0102))
	mustWrite(t, w.WriteUInt32(0x03040506))
	mustWrite(t, w.WriteInt64(0x0708090a0b0c0d0e))

	want := []byte{
		0x01, 0x02,
		0x03, 0x04, 0x05, 0x06,
		0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e,
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("bytes = % x, want % x", buf.Bytes(), want)
	}
}

func TestEndianWriterAlignedString(t *testing.T) {
	var buf bytes.Buffer
	w := NewEndianWriter(&buf, binary.LittleEndian)

	mustWrite(t, w.WriteAlignedString("abc"))
	mustWrite(t, w.WriteByte(0xff))

	want := []byte{
		0x03, 0x00, 0x00, 0x00,
		'a', 'b', 'c', 0x00,
		0xff,
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("bytes = % x, want % x", buf.Bytes(), want)
	}
}

func TestEndianWriterPropagatesWriteError(t *testing.T) {
	wantErr := errors.New("write failed")
	w := NewEndianWriter(errorWriter{err: wantErr}, binary.LittleEndian)

	if err := w.WriteUInt32(1); !errors.Is(err, wantErr) {
		t.Fatalf("WriteUInt32 error = %v, want %v", err, wantErr)
	}
}

func TestEndianWriterDetectsShortWrite(t *testing.T) {
	w := NewEndianWriter(shortWriter{}, binary.LittleEndian)

	if err := w.WriteUInt32(1); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("WriteUInt32 error = %v, want io.ErrShortWrite", err)
	}
}

func mustWrite(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}

func TestAlignOffset(t *testing.T) {
	tests := []struct {
		n         int
		alignment int
		want      int
	}{
		{n: 0, alignment: 16, want: 0},
		{n: 1, alignment: 16, want: 16},
		{n: 16, alignment: 16, want: 16},
		{n: 17, alignment: 16, want: 32},
		{n: 5, alignment: 0, want: 5},
	}

	for _, tt := range tests {
		if got := AlignOffset(tt.n, tt.alignment); got != tt.want {
			t.Fatalf("AlignOffset(%d, %d) = %d, want %d", tt.n, tt.alignment, got, tt.want)
		}
	}
}
