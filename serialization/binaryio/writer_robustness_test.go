package binaryio

import (
	"errors"
	"io"
	"testing"
)

func TestPrimitiveWritersDetectShortWrites(t *testing.T) {
	tests := map[string]func(io.Writer) error{
		"byte":   func(w io.Writer) error { return WriteByte(w, 1) },
		"bytes":  func(w io.Writer) error { return WriteBytes(w, []byte{1, 2}) },
		"int32":  func(w io.Writer) error { return WriteInt32(w, 1) },
		"string": func(w io.Writer) error { return WriteString(w, "x") },
	}
	for name, write := range tests {
		t.Run(name, func(t *testing.T) {
			if err := write(shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("error = %v, want io.ErrShortWrite", err)
			}
		})
	}
}
