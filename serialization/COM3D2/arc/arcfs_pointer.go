package arc

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

type FilePointer interface {
	Compressed() bool
	Data() ([]byte, error)
	RawSize() uint32
	Size() uint32
}

type MemoryPointer struct {
	data []byte
}

func NewMemoryPointer(data []byte) *MemoryPointer {
	return &MemoryPointer{data: append([]byte(nil), data...)}
}

func (m *MemoryPointer) Compressed() bool      { return false }
func (m *MemoryPointer) Data() ([]byte, error) { return append([]byte(nil), m.data...), nil }
func (m *MemoryPointer) RawSize() uint32       { return uint32(len(m.data)) }
func (m *MemoryPointer) Size() uint32          { return uint32(len(m.data)) }

type MemoryPointerCompressed struct {
	data []byte
	raw  uint32
}

func NewMemoryPointerCompressed(compressed []byte, rawSize uint32) *MemoryPointerCompressed {
	return &MemoryPointerCompressed{data: append([]byte(nil), compressed...), raw: rawSize}
}

// NewMemoryPointerCompressedAuto tries to determine raw size by decompressing
func NewMemoryPointerCompressedAuto(compressed []byte) *MemoryPointerCompressed {
	dec, err := deflateDecompress(compressed)
	if err != nil {
		return &MemoryPointerCompressed{data: append([]byte(nil), compressed...), raw: 0}
	}
	return &MemoryPointerCompressed{data: append([]byte(nil), compressed...), raw: uint32(len(dec))}
}

func (m *MemoryPointerCompressed) Compressed() bool      { return true }
func (m *MemoryPointerCompressed) Data() ([]byte, error) { return append([]byte(nil), m.data...), nil }
func (m *MemoryPointerCompressed) RawSize() uint32       { return m.raw }
func (m *MemoryPointerCompressed) Size() uint32          { return uint32(len(m.data)) }

// ArcPointer lazily reads from an .arc file at a given offset
// The offset points to the start of the 16-byte per-file header
// [u32 compressed][u32 padding][u32 rawSize][u32 size] followed by data

type ArcPointer struct {
	reader      *stream.BinaryReader
	offset      int64
	initialized bool
	compressed  bool
	raw         uint32
	size        uint32
	dataOff     int64
}

func NewArcPointer(reader *stream.BinaryReader, offset int64) *ArcPointer {
	return &ArcPointer{reader: reader, offset: offset}
}

func (a *ArcPointer) ensure() error {
	if a.initialized {
		return nil
	}
	if _, err := a.reader.Seek(a.offset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to offset %d: %w", a.offset, err)
	}
	flag, err := a.reader.ReadUInt32()
	if err != nil {
		return fmt.Errorf("failed to read compressed flag: %w", err)
	}
	if _, err := a.reader.ReadUInt32(); err != nil { // padding
		return fmt.Errorf("failed to read padding: %w", err)
	}
	raw, err := a.reader.ReadUInt32()
	if err != nil {
		return fmt.Errorf("failed to read raw size: %w", err)
	}
	sz, err := a.reader.ReadUInt32()
	if err != nil {
		return fmt.Errorf("failed to read compressed size: %w", err)
	}
	a.raw = raw
	a.size = sz
	a.compressed = flag == 1
	pos, _ := a.reader.Seek(0, io.SeekCurrent)
	a.dataOff = pos
	a.initialized = true
	return nil
}

func (a *ArcPointer) Compressed() bool { _ = a.ensure(); return a.compressed }
func (a *ArcPointer) RawSize() uint32  { _ = a.ensure(); return a.raw }
func (a *ArcPointer) Size() uint32     { _ = a.ensure(); return a.size }

func (a *ArcPointer) Data() ([]byte, error) {
	if err := a.ensure(); err != nil {
		return nil, fmt.Errorf("failed to ensure pointer: %w", err)
	}
	if _, err := a.reader.Seek(a.dataOff, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to data offset: %w", err)
	}
	return a.reader.ReadBytes(int(a.size))
}

// deflateCompress produces 0x78 0x5E header + raw DEFLATE stream (no trailer)
func deflateCompress(data []byte) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte(0x78)
	out.WriteByte(0x5E)
	w, err := flate.NewWriter(&out, flate.DefaultCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to create deflate writer: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close deflate writer: %w", err)
	}
	return out.Bytes(), nil
}

// deflateDecompress expects 0x78 0x5E + raw DEFLATE stream
func deflateDecompress(in []byte) ([]byte, error) {
	if len(in) < 2 {
		return nil, fmt.Errorf("invalid deflate payload")
	}
	r := flate.NewReader(bytes.NewReader(in[2:]))
	defer r.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		return nil, fmt.Errorf("failed to decompress deflate stream: %w", err)
	}
	return out.Bytes(), nil
}
