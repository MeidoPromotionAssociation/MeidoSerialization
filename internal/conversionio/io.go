package conversionio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// ErrOutputLimitExceeded reports that a conversion attempted to emit more
// data than its caller-authorized output budget.
var ErrOutputLimitExceeded = errors.New("conversion output limit exceeded")

// Check normalizes a nil context and reports cancellation before a conversion
// starts another unit of work.
func Check(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return ctx.Err()
}

// Reader checks the conversion context before every underlying read.
type Reader struct {
	Context context.Context
	Reader  io.Reader
}

func (r *Reader) Read(p []byte) (int, error) {
	if err := Check(r.Context); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

// LimitWriter checks cancellation and enforces a hard byte limit before each
// write reaches the wrapped writer.
type LimitWriter struct {
	Context   context.Context
	Writer    io.Writer
	Remaining int64
}

// Budget shares one output allowance across a conversion's primary file and
// any sidecars it emits.
type Budget struct {
	Context   context.Context
	Remaining int64
}

func NewBudget(ctx context.Context, maxBytes int64) (*Budget, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("positive conversion output limit is required")
	}
	if err := Check(ctx); err != nil {
		return nil, err
	}
	return &Budget{Context: ctx, Remaining: maxBytes}, nil
}

func (b *Budget) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if b == nil {
		return fmt.Errorf("conversion output budget is nil")
	}
	if int64(len(data)) > b.Remaining {
		return fmt.Errorf("%w: need %d bytes, remaining budget is %d", ErrOutputLimitExceeded, len(data), b.Remaining)
	}
	if err := WriteFile(b.Context, path, data, perm, b.Remaining); err != nil {
		return err
	}
	b.Remaining -= int64(len(data))
	return nil
}

func (w *LimitWriter) Write(p []byte) (int, error) {
	if err := Check(w.Context); err != nil {
		return 0, err
	}
	if w.Writer == nil {
		return 0, fmt.Errorf("conversion output writer is nil")
	}
	if w.Remaining < 0 || int64(len(p)) > w.Remaining {
		return 0, ErrOutputLimitExceeded
	}
	n, err := w.Writer.Write(p)
	w.Remaining -= int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

// ReadFile reads a conversion input while observing ctx between filesystem
// reads. Format decoders may still contain indivisible CPU-bound operations;
// callers should also check ctx before and after invoking those decoders.
func ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := Check(ctx); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var data bytes.Buffer
	_, copyErr := io.Copy(&data, &Reader{Context: ctx, Reader: f})
	closeErr := f.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := Check(ctx); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// Encode runs an encoder against a context-aware, size-limited memory buffer.
// It is useful for serializers that only expose an io.Writer API.
func Encode(ctx context.Context, maxBytes int64, encode func(io.Writer) error) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("positive conversion output limit is required")
	}
	if encode == nil {
		return nil, fmt.Errorf("conversion encoder is nil")
	}
	if err := Check(ctx); err != nil {
		return nil, err
	}
	var data bytes.Buffer
	w := &LimitWriter{Context: ctx, Writer: &data, Remaining: maxBytes}
	if err := encode(w); err != nil {
		return nil, err
	}
	if err := Check(ctx); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// File is an output file whose Write method observes a conversion context and
// enforces a hard byte limit.
type File struct {
	file   *os.File
	writer *LimitWriter
}

func CreateFile(ctx context.Context, path string, perm fs.FileMode, maxBytes int64) (*File, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("positive conversion output limit is required")
	}
	if err := Check(ctx); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return nil, err
	}
	return &File{
		file:   f,
		writer: &LimitWriter{Context: ctx, Writer: f, Remaining: maxBytes},
	}, nil
}

func (f *File) Write(p []byte) (int, error) {
	if f == nil || f.writer == nil {
		return 0, fmt.Errorf("conversion output file is nil")
	}
	return f.writer.Write(p)
}

func (f *File) Close() error {
	if f == nil || f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

// WriteFile writes one output with cancellation and a hard size limit.
func WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode, maxBytes int64) error {
	if maxBytes <= 0 {
		return fmt.Errorf("positive conversion output limit is required")
	}
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("%w: need %d bytes, limit is %d", ErrOutputLimitExceeded, len(data), maxBytes)
	}
	f, err := CreateFile(ctx, path, perm, maxBytes)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return Check(ctx)
}
