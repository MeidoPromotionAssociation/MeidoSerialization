package conversionio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestLimitWriterHonorsExactBoundary(t *testing.T) {
	var output bytes.Buffer
	w := &LimitWriter{Context: context.Background(), Writer: &output, Remaining: 4}
	if n, err := w.Write([]byte("1234")); err != nil || n != 4 {
		t.Fatalf("boundary write = (%d, %v)", n, err)
	}
	if n, err := w.Write([]byte("5")); !errors.Is(err, ErrOutputLimitExceeded) || n != 0 {
		t.Fatalf("overflow write = (%d, %v)", n, err)
	}
	if got := output.String(); got != "1234" {
		t.Fatalf("output = %q", got)
	}
}

func TestLimitWriterHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	w := &LimitWriter{Context: ctx, Writer: &output, Remaining: 4}
	if n, err := w.Write([]byte("1")); !errors.Is(err, context.Canceled) || n != 0 {
		t.Fatalf("canceled write = (%d, %v)", n, err)
	}
	if output.Len() != 0 {
		t.Fatalf("canceled write emitted %d bytes", output.Len())
	}
}

func TestEncodeStopsAtOutputLimit(t *testing.T) {
	_, err := Encode(context.Background(), 3, func(w io.Writer) error {
		_, err := w.Write([]byte("1234"))
		return err
	})
	if !errors.Is(err, ErrOutputLimitExceeded) {
		t.Fatalf("Encode error = %v", err)
	}
}
