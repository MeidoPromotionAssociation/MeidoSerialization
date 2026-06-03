package binaryio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

// EndianWriter 以可配置字节序向底层 writer 写入基础类型 / EndianWriter writes primitive values into an underlying writer with a configurable byte order.
type EndianWriter struct {
	W       io.Writer
	order   binary.ByteOrder
	pos     int64
	scratch [8]byte
}

// NewEndianWriter 使用指定字节序创建 EndianWriter / NewEndianWriter creates a EndianWriter using the requested byte order.
func NewEndianWriter(w io.Writer, order binary.ByteOrder) *EndianWriter {
	if order == nil {
		order = binary.LittleEndian
	}
	bw := &EndianWriter{W: w, order: order}
	if l, ok := w.(interface{ Len() int }); ok {
		bw.pos = int64(l.Len())
	}
	return bw
}

// Len 返回当前已写入偏移 / Len returns the current written offset.
func (w *EndianWriter) Len() int {
	return int(w.pos)
}

// Bytes 在底层 writer 是 bytes.Buffer 时返回其字节，否则返回 nil / Bytes returns underlying bytes when the writer is a bytes.Buffer, otherwise nil.
func (w *EndianWriter) Bytes() []byte {
	if buf, ok := w.W.(*bytes.Buffer); ok {
		return buf.Bytes()
	}
	return nil
}

// ByteOrder 返回当前数值字节序 / ByteOrder returns the current numeric byte order.
func (w *EndianWriter) ByteOrder() binary.ByteOrder {
	return w.order
}

// SetByteOrder 修改后续写入使用的数值字节序 / SetByteOrder changes the numeric byte order used by subsequent writes.
func (w *EndianWriter) SetByteOrder(order binary.ByteOrder) {
	if order != nil {
		w.order = order
	}
}

// WriteByte 写入单个字节 / WriteByte writes one byte.
func (w *EndianWriter) WriteByte(value byte) error {
	w.scratch[0] = value
	return w.writeFull(w.scratch[:1])
}

// WriteBool 将 bool 写为单字节 / WriteBool writes a bool as one byte.
func (w *EndianWriter) WriteBool(value bool) error {
	if value {
		return w.WriteByte(1)
	}
	return w.WriteByte(0)
}

// WriteBytes 写入原始字节 / WriteBytes writes raw bytes.
func (w *EndianWriter) WriteBytes(value []byte) error {
	return w.writeFull(value)
}

// WriteZeroes 写入 count 个零字节 / WriteZeroes writes count zero bytes.
func (w *EndianWriter) WriteZeroes(count int) error {
	if count <= 0 {
		return nil
	}
	return w.WriteBytes(make([]byte, count))
}

// WriteNullString 写入字符串原始字节并追加 null 结尾 / WriteNullString writes raw string bytes followed by a null byte.
func (w *EndianWriter) WriteNullString(value string) error {
	if err := w.WriteBytes([]byte(value)); err != nil {
		return err
	}
	return w.WriteByte(0)
}

// WriteUInt16 按当前字节序写入 uint16 / WriteUInt16 writes a uint16 using the configured byte order.
func (w *EndianWriter) WriteUInt16(value uint16) error {
	w.order.PutUint16(w.scratch[:2], value)
	return w.WriteBytes(w.scratch[:2])
}

// WriteInt16 按当前字节序写入 int16 / WriteInt16 writes an int16 using the configured byte order.
func (w *EndianWriter) WriteInt16(value int16) error {
	return w.WriteUInt16(uint16(value))
}

// WriteUInt32 按当前字节序写入 uint32 / WriteUInt32 writes a uint32 using the configured byte order.
func (w *EndianWriter) WriteUInt32(value uint32) error {
	w.order.PutUint32(w.scratch[:4], value)
	return w.WriteBytes(w.scratch[:4])
}

// WriteInt32 按当前字节序写入 int32 / WriteInt32 writes an int32 using the configured byte order.
func (w *EndianWriter) WriteInt32(value int32) error {
	return w.WriteUInt32(uint32(value))
}

// WriteUInt64 按当前字节序写入 uint64 / WriteUInt64 writes a uint64 using the configured byte order.
func (w *EndianWriter) WriteUInt64(value uint64) error {
	w.order.PutUint64(w.scratch[:8], value)
	return w.WriteBytes(w.scratch[:8])
}

// WriteInt64 按当前字节序写入 int64 / WriteInt64 writes an int64 using the configured byte order.
func (w *EndianWriter) WriteInt64(value int64) error {
	return w.WriteUInt64(uint64(value))
}

// WriteFloat32 按当前字节序写入 float32 / WriteFloat32 writes a float32 using the configured byte order.
func (w *EndianWriter) WriteFloat32(value float32) error {
	return w.WriteUInt32(math.Float32bits(value))
}

// WriteFloat64 按当前字节序写入 float64 / WriteFloat64 writes a float64 using the configured byte order.
func (w *EndianWriter) WriteFloat64(value float64) error {
	return w.WriteUInt64(math.Float64bits(value))
}

// WriteAlignedString 写入 Unity 对齐字符串布局 / WriteAlignedString writes Unity's aligned string layout:
// uint32 字节长度、原始字节，然后填充到 4 字节边界 / uint32 byte length, raw bytes, then padding to a 4-byte boundary.
func (w *EndianWriter) WriteAlignedString(value string) error {
	data := []byte(value)
	if err := w.WriteUInt32(uint32(len(data))); err != nil {
		return err
	}
	if err := w.WriteBytes(data); err != nil {
		return err
	}
	return w.Align(4)
}

// Align 填充缓冲区到下一个对齐边界 / Align pads the buffer to the next alignment boundary.
func (w *EndianWriter) Align(alignment int) error {
	target := AlignOffset(w.Len(), alignment)
	return w.WriteZeroes(target - w.Len())
}

// AlignOffset 将 n 向上取整到下一个对齐边界 / AlignOffset returns n rounded up to the next alignment boundary.
func AlignOffset(n int, alignment int) int {
	if alignment <= 0 {
		return n
	}
	rem := n % alignment
	if rem == 0 {
		return n
	}
	return n + alignment - rem
}

// writeFull 写入完整字节切片并更新偏移 / writeFull writes the whole byte slice and updates the offset.
// 它会返回 nil writer、底层写入错误或短写错误 / It returns a nil-writer error, the underlying write error, or a short-write error.
func (w *EndianWriter) writeFull(p []byte) error {
	if w.W == nil {
		return errors.New("binaryio.EndianWriter: nil writer")
	}
	n, err := w.W.Write(p)
	w.pos += int64(n)
	if err != nil {
		return err
	}
	if n != len(p) {
		return io.ErrShortWrite
	}
	return nil
}
