package stream

import (
	"encoding/binary"
	"io"
	"math"
)

// BinaryWriter 提供向流中写入基本类型的功能
type BinaryWriter struct {
	W      io.Writer
	buffer [64]byte // 用于写入基本类型的临时缓冲区
}

// NewBinaryWriter 创建一个新的 BinaryWriter
func NewBinaryWriter(w io.Writer) *BinaryWriter {
	return &BinaryWriter{W: w}
}

// WriteBool 写入布尔值
func (bw *BinaryWriter) WriteBool(value bool) error {
	if value {
		bw.buffer[0] = 1
	} else {
		bw.buffer[0] = 0
	}
	_, err := bw.W.Write(bw.buffer[:1])
	return err
}

// WriteByte 写入单个字节
func (bw *BinaryWriter) WriteByte(value byte) error {
	bw.buffer[0] = value
	_, err := bw.W.Write(bw.buffer[:1])
	return err
}

// WriteSByte 写入有符号字节
func (bw *BinaryWriter) WriteSByte(value int8) error {
	return bw.WriteByte(byte(value))
}

// WriteInt16 写入 2 字节有符号整数 (little-endian)
func (bw *BinaryWriter) WriteInt16(value int16) error {
	binary.LittleEndian.PutUint16(bw.buffer[:2], uint16(value))
	_, err := bw.W.Write(bw.buffer[:2])
	return err
}

// WriteUInt16 写入 2 字节无符号整数 (little-endian)
func (bw *BinaryWriter) WriteUInt16(value uint16) error {
	binary.LittleEndian.PutUint16(bw.buffer[:2], value)
	_, err := bw.W.Write(bw.buffer[:2])
	return err
}

// WriteInt32 写入 4 字节有符号整数 (little-endian)
func (bw *BinaryWriter) WriteInt32(value int32) error {
	binary.LittleEndian.PutUint32(bw.buffer[:4], uint32(value))
	_, err := bw.W.Write(bw.buffer[:4])
	return err
}

// WriteUInt32 写入 4 字节无符号整数 (little-endian)
func (bw *BinaryWriter) WriteUInt32(value uint32) error {
	binary.LittleEndian.PutUint32(bw.buffer[:4], value)
	_, err := bw.W.Write(bw.buffer[:4])
	return err
}

// WriteInt64 写入 8 字节有符号整数 (little-endian)
func (bw *BinaryWriter) WriteInt64(value int64) error {
	binary.LittleEndian.PutUint64(bw.buffer[:8], uint64(value))
	_, err := bw.W.Write(bw.buffer[:8])
	return err
}

// WriteUInt64 写入 8 字节无符号整数 (little-endian)
func (bw *BinaryWriter) WriteUInt64(value uint64) error {
	binary.LittleEndian.PutUint64(bw.buffer[:8], value)
	_, err := bw.W.Write(bw.buffer[:8])
	return err
}

// WriteFloat32 写入 4 字节浮点数 (little-endian)
func (bw *BinaryWriter) WriteFloat32(value float32) error {
	bits := math.Float32bits(value)
	binary.LittleEndian.PutUint32(bw.buffer[:4], bits)
	_, err := bw.W.Write(bw.buffer[:4])
	return err
}

// WriteFloat64 写入 8 字节浮点数 (little-endian)
func (bw *BinaryWriter) WriteFloat64(value float64) error {
	bits := math.Float64bits(value)
	binary.LittleEndian.PutUint64(bw.buffer[:8], bits)
	_, err := bw.W.Write(bw.buffer[:8])
	return err
}

// WriteString 写入长度前缀的 UTF-8 字符串（与 C# BinaryWriter 兼容）
func (bw *BinaryWriter) WriteString(value string) error {
	// 计算 UTF-8 字节长度
	length := len(value)

	// 写入 7-bit 编码的长度
	if err := bw.write7BitEncodedInt(length); err != nil {
		return err
	}

	// 写入字符串字节
	if length > 0 {
		_, err := bw.W.Write([]byte(value))
		return err
	}

	return nil
}

// write7BitEncodedInt 写入 7-bit 编码的整数
func (bw *BinaryWriter) write7BitEncodedInt(value int) error {
	v := uint32(value)
	for v >= 0x80 {
		if err := bw.WriteByte(byte(v | 0x80)); err != nil {
			return err
		}
		v >>= 7
	}
	return bw.WriteByte(byte(v))
}

// WriteBytes 写入字节数组
func (bw *BinaryWriter) WriteBytes(value []byte) error {
	_, err := bw.W.Write(value)
	return err
}

// -------------------- Float2 / Float3 / Float4 / Float4x4 --------------------

// WriteFloat2 写入 2 个连续的 float32 (Vector2)
func (bw *BinaryWriter) WriteFloat2(arr [2]float32) error {
	binary.LittleEndian.PutUint32(bw.buffer[0:4], math.Float32bits(arr[0]))
	binary.LittleEndian.PutUint32(bw.buffer[4:8], math.Float32bits(arr[1]))
	_, err := bw.W.Write(bw.buffer[:8])
	return err
}

// WriteFloat3 写入 3 个连续的 float32 (Vector3)
func (bw *BinaryWriter) WriteFloat3(arr [3]float32) error {
	binary.LittleEndian.PutUint32(bw.buffer[0:4], math.Float32bits(arr[0]))
	binary.LittleEndian.PutUint32(bw.buffer[4:8], math.Float32bits(arr[1]))
	binary.LittleEndian.PutUint32(bw.buffer[8:12], math.Float32bits(arr[2]))
	_, err := bw.W.Write(bw.buffer[:12])
	return err
}

// WriteFloat4 写入 4 个连续的 float32 (Vector4/Quaternion)
func (bw *BinaryWriter) WriteFloat4(arr [4]float32) error {
	binary.LittleEndian.PutUint32(bw.buffer[0:4], math.Float32bits(arr[0]))
	binary.LittleEndian.PutUint32(bw.buffer[4:8], math.Float32bits(arr[1]))
	binary.LittleEndian.PutUint32(bw.buffer[8:12], math.Float32bits(arr[2]))
	binary.LittleEndian.PutUint32(bw.buffer[12:16], math.Float32bits(arr[3]))
	_, err := bw.W.Write(bw.buffer[:16])
	return err
}

// WriteFloat4x4 写入 16 个连续的 float32 (4x4 Matrix)
func (bw *BinaryWriter) WriteFloat4x4(arr [16]float32) error {
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint32(bw.buffer[i*4:i*4+4], math.Float32bits(arr[i]))
	}
	_, err := bw.W.Write(bw.buffer[:64])
	return err
}

// Flush 刷新底层写入器（如果支持）
func (bw *BinaryWriter) Flush() error {
	if flusher, ok := bw.W.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}
