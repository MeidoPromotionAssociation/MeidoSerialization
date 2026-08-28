package arc

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

// FilePointer 表示用于管理 ARC 文件系统内文件数据的接口
// FilePointer represents an interface for managing file data within an ARC file system
type FilePointer interface {
	Compressed() bool      // Compressed 表示文件数据是否已压缩 / Compressed indicates if the file data is compressed
	Data() ([]byte, error) // Data 以字节切片形式获取文件数据 / Data retrieves the file data as a byte slice
	RawSize() uint32       // RawSize 返回未压缩文件的字节大小 / RawSize returns the uncompressed file size in bytes
	Size() uint32          // Size 返回文件的存储字节大小，并反映是否应用压缩 / Size returns the file size in bytes, respecting compression if applied
}

// MemoryPointer 表示将内存缓冲区封装为字节切片的结构
// MemoryPointer represents a structure that encapsulates a memory buffer as a slice of bytes
type MemoryPointer struct {
	data []byte // 未压缩的数据 / Uncompressed data
}

// NewMemoryPointer 创建一个复制所提供字节切片的新 MemoryPointer 实例
// NewMemoryPointer creates a new MemoryPointer instance with a copy of the provided byte slice
func NewMemoryPointer(data []byte) *MemoryPointer {
	return &MemoryPointer{data: append([]byte(nil), data...)}
}

// Compressed 始终返回 false，因为该指针保存未压缩数据
// Compressed always returns false because this pointer stores uncompressed data
func (m *MemoryPointer) Compressed() bool { return false }

// Data 返回未压缩数据的副本
// Data returns a copy of the uncompressed data
func (m *MemoryPointer) Data() ([]byte, error) { return append([]byte(nil), m.data...), nil }

// RawSize 返回未压缩数据的字节数
// RawSize returns the number of bytes in the uncompressed data
func (m *MemoryPointer) RawSize() uint32 { return uint32(len(m.data)) }

// Size 返回未压缩数据在 ARC 中的存储字节数
// Size returns the number of bytes used to store the uncompressed data in the ARC
func (m *MemoryPointer) Size() uint32 { return uint32(len(m.data)) }

// MemoryPointerCompressed 表示用于管理压缩数据及其原始大小的结构
// MemoryPointerCompressed represents a data structure for managing compressed data and its raw size
type MemoryPointerCompressed struct {
	data []byte // 已压缩的数据 / Compressed data
	raw  uint32 // 解压后的字节数 / Uncompressed byte count
}

// NewMemoryPointerCompressed 创建一个保存压缩数据的内存指针
// NewMemoryPointerCompressed creates a memory pointer for already compressed data
func NewMemoryPointerCompressed(compressed []byte, rawSize uint32) *MemoryPointerCompressed {
	return &MemoryPointerCompressed{data: append([]byte(nil), compressed...), raw: rawSize}
}

// NewMemoryPointerCompressedAuto 尝试通过解压数据确定原始大小，解压失败时将原始大小保留为零
// NewMemoryPointerCompressedAuto tries to determine raw size by decompressing and leaves it at zero when decompression fails
func NewMemoryPointerCompressedAuto(compressed []byte) *MemoryPointerCompressed {
	dec, err := deflateDecompress(compressed)
	if err != nil {
		return &MemoryPointerCompressed{data: append([]byte(nil), compressed...), raw: 0}
	}
	return &MemoryPointerCompressed{data: append([]byte(nil), compressed...), raw: uint32(len(dec))}
}

// Compressed 始终返回 true，因为该指针保存压缩数据
// Compressed always returns true because this pointer stores compressed data
func (m *MemoryPointerCompressed) Compressed() bool { return true }

// Data 返回压缩数据的副本
// Data returns a copy of the compressed data
func (m *MemoryPointerCompressed) Data() ([]byte, error) { return append([]byte(nil), m.data...), nil }

// RawSize 返回压缩数据解压后的大小
// RawSize returns the size of the compressed data after decompression
func (m *MemoryPointerCompressed) RawSize() uint32 { return m.raw }

// Size 返回压缩数据在 ARC 中的存储字节数
// Size returns the number of bytes used to store the compressed data in the ARC
func (m *MemoryPointerCompressed) Size() uint32 { return uint32(len(m.data)) }

// ArcPointer 从 ARC 文件中的给定偏移延迟读取数据
// 此偏移指向每个文件所用的 16 字节头部起始位置
// 头部依次为压缩标志、保留值、原始大小和存储大小，随后是文件数据
// ArcPointer lazily reads from an .arc file at a given offset
// The offset points to the start of the 16-byte per-file header
// [u32 compressed][u32 padding][u32 rawSize][u32 size] followed by data
type ArcPointer struct {
	reader      *stream.BinaryReader // 延迟读取所用的二进制读取器 / Binary reader used for lazy loading
	offset      int64                // 文件头起始偏移 / Offset of the file header
	initialized bool                 // 是否已读取并缓存文件头 / Whether the file header has been loaded and cached
	compressed  bool                 // 文件数据是否压缩 / Whether the file data is compressed
	raw         uint32               // 解压后的数据大小 / Uncompressed data size
	size        uint32               // ARC 中存储的数据大小 / Data size stored in the ARC
	dataOff     int64                // 文件数据起始偏移 / Offset of the file data
}

// NewArcPointer 创建一个从给定文件头偏移读取数据的延迟指针
// NewArcPointer creates a lazy pointer that reads data from the given file-header offset
func NewArcPointer(reader *stream.BinaryReader, offset int64) *ArcPointer {
	return &ArcPointer{reader: reader, offset: offset}
}

// ensure 在尚未初始化时从底层二进制流读取数据并初始化 ArcPointer
// ensure initializes the ArcPointer by loading data from the underlying binary stream if it is not already initialized
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
	// 跳过保留值
	// Skip padding
	if _, err := a.reader.ReadUInt32(); err != nil {
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
	pos, err := a.reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to determine data offset: %w", err)
	}
	a.dataOff = pos
	a.initialized = true
	return nil
}

// Compressed 返回文件头中的压缩标志，读取错误时返回当前缓存值
// Compressed returns the header compression flag and returns the cached value on read failure
func (a *ArcPointer) Compressed() bool { _ = a.ensure(); return a.compressed }

// RawSize 返回文件头记录的解压后大小
// RawSize returns the uncompressed size recorded in the file header
func (a *ArcPointer) RawSize() uint32 { _ = a.ensure(); return a.raw }

// Size 返回文件头记录的 ARC 存储大小
// Size returns the ARC storage size recorded in the file header
func (a *ArcPointer) Size() uint32 { _ = a.ensure(); return a.size }

// Data 根据已缓存的偏移和大小读取 ARC 中的数据字节
// Data reads the ARC data bytes using the cached offset and size
func (a *ArcPointer) Data() ([]byte, error) {
	if err := a.ensure(); err != nil {
		return nil, fmt.Errorf("failed to ensure pointer: %w", err)
	}
	if _, err := a.reader.Seek(a.dataOff, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to data offset: %w", err)
	}
	return a.reader.ReadBytes(int64(a.size))
}

// deflateCompress 生成 0x78 0x5E 头部和不带尾部的原始 DEFLATE 流
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
		_ = w.Close()
		return nil, fmt.Errorf("failed to write data: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close deflate writer: %w", err)
	}
	return out.Bytes(), nil
}

// deflateDecompress 要求输入由 0x78 0x5E 头部和原始 DEFLATE 流组成
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
