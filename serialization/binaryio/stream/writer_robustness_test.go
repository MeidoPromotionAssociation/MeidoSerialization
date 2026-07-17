package stream

import (
	"errors"
	"io"
	"testing"
)

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}

func TestBinaryWriterDetectsShortWrites(t *testing.T) {
	tests := map[string]func(*BinaryWriter) error{
		"byte":     func(w *BinaryWriter) error { return w.WriteByte(1) },
		"bytes":    func(w *BinaryWriter) error { return w.WriteBytes([]byte{1, 2}) },
		"int32":    func(w *BinaryWriter) error { return w.WriteInt32(1) },
		"string":   func(w *BinaryWriter) error { return w.WriteString("x") },
		"float4x4": func(w *BinaryWriter) error { return w.WriteFloat4x4([16]float32{}) },
	}
	for name, write := range tests {
		t.Run(name, func(t *testing.T) {
			if err := write(NewBinaryWriter(shortWriter{})); !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("error = %v, want io.ErrShortWrite", err)
			}
		})
	}
}
