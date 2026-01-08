package arc

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// FilePointer represents an interface for managing file data within an ARC file system.
type FilePointer interface {
	Compressed() bool      // Compressed indicates if the file data is compressed.
	Data() ([]byte, error) // Data retrieves the file data as a byte slice.
	RawSize() uint32       // RawSize returns the uncompressed file size in bytes.
	Size() uint32          // Size returns the file size in bytes, respecting compression if applied.
}

// MemoryPointer represents a structure that encapsulates a memory buffer as a slice of bytes.
type MemoryPointer struct {
	data []byte
}

// NewMemoryPointer creates a new MemoryPointer instance with a copy of the provided byte slice.
func NewMemoryPointer(data []byte) *MemoryPointer {
	return &MemoryPointer{data: append([]byte(nil), data...)}
}

func (m *MemoryPointer) Compressed() bool      { return false }
func (m *MemoryPointer) Data() ([]byte, error) { return append([]byte(nil), m.data...), nil }
func (m *MemoryPointer) RawSize() uint32       { return uint32(len(m.data)) }
func (m *MemoryPointer) Size() uint32          { return uint32(len(m.data)) }

// MemoryPointerCompressed represents a data structure for managing compressed data and its raw size.
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
	reader      *stream.BinaryReader // reader is a binary stream reader used to lazily load file data from an .arc file.
	offset      int64                // offset specifies the byte offset within the .arc file where the file's data starts.
	initialized bool                 // initialized indicates whether the ArcPointer has read and cached the required metadata from the .arc file.
	compressed  bool                 // compressed indicates whether the file data is stored in a compressed format within the .arc file.
	raw         uint32               // raw represents the uncompressed size of the file data in bytes as read from the .arc file header.
	size        uint32               // size represents the size of the file data in bytes, as specified in the .arc file header.
	dataOff     int64                // dataOff specifies the byte offset within the file where the actual file data begins after the header has been parsed.
}

func NewArcPointer(reader *stream.BinaryReader, offset int64) *ArcPointer {
	return &ArcPointer{reader: reader, offset: offset}
}

// ensure initializes the ArcPointer by loading data from the underlying binary stream if it is not already initialized.
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
