package binaryio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// EndianReader 以可配置字节序从内存字节切片读取基础类型 / EndianReader reads primitive values from an in-memory byte slice with a configurable byte order.
type EndianReader struct {
	data  []byte
	pos   int
	order binary.ByteOrder
}

// NewEndianReader 创建从偏移 0 开始读取的 EndianReader / NewEndianReader creates an EndianReader starting at offset 0.
func NewEndianReader(data []byte, order binary.ByteOrder) *EndianReader {
	return NewEndianReaderAt(data, 0, order)
}

// NewEndianReaderAt 创建从指定偏移开始读取的 EndianReader / NewEndianReaderAt creates an EndianReader starting at the requested offset.
func NewEndianReaderAt(data []byte, pos int, order binary.ByteOrder) *EndianReader {
	if order == nil {
		order = binary.LittleEndian
	}
	return &EndianReader{data: data, pos: pos, order: order}
}

// Pos 返回当前读取偏移 / Pos returns the current read offset.
func (r *EndianReader) Pos() int {
	return r.pos
}

// Len 返回底层数据长度 / Len returns the length of the backing data.
func (r *EndianReader) Len() int {
	return len(r.data)
}

// Remaining 返回从当前偏移开始仍可读取的字节数 / Remaining returns the number of readable bytes from the current offset.
func (r *EndianReader) Remaining() int {
	if r.pos < 0 || r.pos >= len(r.data) {
		return 0
	}
	return len(r.data) - r.pos
}

// ByteOrder 返回当前数值字节序 / ByteOrder returns the current numeric byte order.
func (r *EndianReader) ByteOrder() binary.ByteOrder {
	return r.order
}

// SetByteOrder 修改后续读取使用的数值字节序 / SetByteOrder changes the numeric byte order used by subsequent reads.
func (r *EndianReader) SetByteOrder(order binary.ByteOrder) {
	if order != nil {
		r.order = order
	}
}

// ReadByte 读取单个字节 / ReadByte reads one byte.
func (r *EndianReader) ReadByte() (byte, error) {
	if err := r.require(1); err != nil {
		return 0, err
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

// ReadBool 将单个字节读取为 bool / ReadBool reads one byte as a boolean.
func (r *EndianReader) ReadBool() (bool, error) {
	b, err := r.ReadByte()
	return b != 0, err
}

// ReadInt8 读取单字节有符号整数 / ReadInt8 reads one signed byte.
func (r *EndianReader) ReadInt8() (int8, error) {
	b, err := r.ReadByte()
	return int8(b), err
}

// ReadUInt16 按当前字节序读取 uint16 / ReadUInt16 reads a uint16 using the configured byte order.
func (r *EndianReader) ReadUInt16() (uint16, error) {
	if err := r.require(2); err != nil {
		return 0, err
	}
	v := r.order.Uint16(r.data[r.pos:])
	r.pos += 2
	return v, nil
}

// ReadInt16 按当前字节序读取 int16 / ReadInt16 reads an int16 using the configured byte order.
func (r *EndianReader) ReadInt16() (int16, error) {
	v, err := r.ReadUInt16()
	return int16(v), err
}

// ReadUInt32 按当前字节序读取 uint32 / ReadUInt32 reads a uint32 using the configured byte order.
func (r *EndianReader) ReadUInt32() (uint32, error) {
	if err := r.require(4); err != nil {
		return 0, err
	}
	v := r.order.Uint32(r.data[r.pos:])
	r.pos += 4
	return v, nil
}

// ReadInt32 按当前字节序读取 int32 / ReadInt32 reads an int32 using the configured byte order.
func (r *EndianReader) ReadInt32() (int32, error) {
	v, err := r.ReadUInt32()
	return int32(v), err
}

// ReadUInt64 按当前字节序读取 uint64 / ReadUInt64 reads a uint64 using the configured byte order.
func (r *EndianReader) ReadUInt64() (uint64, error) {
	if err := r.require(8); err != nil {
		return 0, err
	}
	v := r.order.Uint64(r.data[r.pos:])
	r.pos += 8
	return v, nil
}

// ReadInt64 按当前字节序读取 int64 / ReadInt64 reads an int64 using the configured byte order.
func (r *EndianReader) ReadInt64() (int64, error) {
	v, err := r.ReadUInt64()
	return int64(v), err
}

// ReadFloat32 按当前字节序读取 float32 / ReadFloat32 reads a float32 using the configured byte order.
func (r *EndianReader) ReadFloat32() (float32, error) {
	v, err := r.ReadUInt32()
	return math.Float32frombits(v), err
}

// ReadFloat64 按当前字节序读取 float64 / ReadFloat64 reads a float64 using the configured byte order.
func (r *EndianReader) ReadFloat64() (float64, error) {
	v, err := r.ReadUInt64()
	return math.Float64frombits(v), err
}

// ReadFull 将 len(buf) 个字节复制到 buf / ReadFull copies len(buf) bytes into buf.
func (r *EndianReader) ReadFull(buf []byte) error {
	if err := r.require(len(buf)); err != nil {
		return err
	}
	copy(buf, r.data[r.pos:r.pos+len(buf)])
	r.pos += len(buf)
	return nil
}

// ReadBytes 读取 n 个字节并返回新切片 / ReadBytes reads n bytes into a new slice.
func (r *EndianReader) ReadBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("negative byte count: %d", n)
	}
	if n > 512*1024*1024 {
		return nil, fmt.Errorf("byte count too large: %d", n)
	}
	if err := r.require(n); err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	copy(buf, r.data[r.pos:r.pos+n])
	r.pos += n
	return buf, nil
}

// ReadNullString 从当前偏移读取 null 结尾字符串 / ReadNullString reads a null-terminated string from the current offset.
func (r *EndianReader) ReadNullString() (string, error) {
	if r.pos < 0 || r.pos > len(r.data) {
		return "", io.ErrUnexpectedEOF
	}
	start := r.pos
	for r.pos < len(r.data) {
		if r.data[r.pos] == 0 {
			s := string(r.data[start:r.pos])
			r.pos++
			return s, nil
		}
		r.pos++
	}
	return string(r.data[start:]), io.ErrUnexpectedEOF
}

// ReadAlignedString 读取 Unity 对齐字符串布局 / ReadAlignedString reads Unity's aligned string layout:
// int32 字节长度、原始字节，然后跳过到 4 字节边界 / int32 byte length, raw bytes, then padding to a 4-byte boundary.
func (r *EndianReader) ReadAlignedString() (string, error) {
	length, err := r.ReadInt32()
	if err != nil {
		return "", err
	}
	if length <= 0 {
		r.Align4()
		return "", nil
	}
	n := int(length)
	if err := r.require(n); err != nil {
		return "", err
	}
	s := string(r.data[r.pos : r.pos+n])
	r.pos += n
	r.Align4()
	return s, nil
}

// Skip 前移读取偏移，并在底层数据末尾截断 / Skip advances the read offset and clamps at the end of the backing data.
func (r *EndianReader) Skip(n int) {
	if n <= 0 {
		return
	}
	if r.pos < 0 {
		r.pos = 0
	}
	if n >= len(r.data)-r.pos {
		r.pos = len(r.data)
		return
	}
	r.pos += n
}

// Align 将读取偏移前移到下一个对齐边界 / Align advances the read offset to the next alignment boundary.
func (r *EndianReader) Align(alignment int) {
	if alignment <= 1 {
		return
	}
	rem := r.pos % alignment
	if rem != 0 {
		r.pos += alignment - rem
	}
}

// Align4 将读取偏移前移到下一个 4 字节边界 / Align4 advances the read offset to the next 4-byte boundary.
func (r *EndianReader) Align4() {
	r.Align(4)
}

// require 检查从当前偏移读取 n 个字节是否安全 / require checks whether n bytes can be read safely from the current offset.
func (r *EndianReader) require(n int) error {
	if n < 0 {
		return fmt.Errorf("negative byte count: %d", n)
	}
	if r.pos < 0 || r.pos > len(r.data) || n > len(r.data)-r.pos {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// ReadNullString 从 io.Reader 读取 null 结尾字符串 / ReadNullString reads a null-terminated string from an io.Reader.
func ReadNullString(r io.Reader) (string, error) {
	var buf []byte
	var b [1]byte
	for {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return "", err
		}
		if b[0] == 0 {
			return string(buf), nil
		}
		buf = append(buf, b[0])
	}
}
