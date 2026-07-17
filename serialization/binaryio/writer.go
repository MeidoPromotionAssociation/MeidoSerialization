package binaryio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// WriteByte 写单字节
func WriteByte(w io.Writer, b byte) error {
	return writeFull(w, []byte{b})
}

// WriteBytes 写多个字节
func WriteBytes(w io.Writer, bs []byte) error {
	return writeFull(w, bs)
}

func writeFull(w io.Writer, data []byte) error {
	if w == nil {
		return fmt.Errorf("binaryio: nil writer")
	}
	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

// WriteBool 写一个字节，如果 b 为 true 则写入 1，否则写入 0
func WriteBool(w io.Writer, v bool) error {
	var b byte
	if v {
		b = 1
	}
	return WriteByte(w, b)
}

// WriteInt8 写入 1 字节 int8
func WriteInt8(w io.Writer, value int8) error {
	return WriteByte(w, byte(value))
}

// WriteInt16 写入 2 字节有符号整数 (little-endian)
func WriteInt16(w io.Writer, value int16) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(value))
	return writeFull(w, buf[:])
}

// WriteUInt16 写入一个16位无符号整数(little-endian)
func WriteUInt16(w io.Writer, val uint16) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], val)
	return writeFull(w, buf[:])
}

// WriteInt32 写一个 4 字节 int32（little-endian）
func WriteInt32(w io.Writer, v int32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(v))
	return writeFull(w, buf[:])
}

// WriteUInt32 写入一个 4 字节 uint32(little-endian)
func WriteUInt32(w io.Writer, val uint32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], val)
	return writeFull(w, buf[:])
}

// WriteInt64 写入 8 字节有符号整数 (little-endian)
func WriteInt64(w io.Writer, value int64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(value))
	return writeFull(w, buf[:])
}

// WriteUInt64 写入 8 字节无符号整数 (little-endian)
func WriteUInt64(w io.Writer, value uint64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], value)
	return writeFull(w, buf[:])
}

// WriteFloat32 写一个 float32 (4 bytes, little-endian)
// 在 C# 中是 BinaryWriter.WriteSingle
func WriteFloat32(w io.Writer, val float32) error {
	var buf [4]byte
	bits := math.Float32bits(val)
	binary.LittleEndian.PutUint32(buf[:], bits)
	return writeFull(w, buf[:])
}

// WriteFloat64 写入 8 字节浮点数 (little-endian)
func WriteFloat64(w io.Writer, value float64) error {
	var buf [8]byte
	bits := math.Float64bits(value)
	binary.LittleEndian.PutUint64(buf[:], bits)
	return writeFull(w, buf[:])
}

// WriteString 写入 C# BinaryWriter.WriteString 格式的字符串
// 完全匹配 .NET 4.8 的实现逻辑
// 需要注意这个 7BitEncode 虽然与 LEB128 类似，但不是完全相同
func WriteString(w io.Writer, s string) error {
	// 将字符串转换为 UTF-8 字节数组
	buffer := []byte(s)
	if uint64(len(buffer)) > uint64(math.MaxInt32) {
		return fmt.Errorf("string byte length %d exceeds Int32", len(buffer))
	}
	// 写入字节长度（不是字符长度）
	err := Write7BitEncodedInt(w, int32(len(buffer)))
	if err != nil {
		return err
	}
	// 写入实际的字节数据
	return writeFull(w, buffer)
}

// -------------------- Float2 / Float3 / Float4 / Float4x4 --------------------

func WriteFloat2(w io.Writer, arr [2]float32) error {
	for i := 0; i < 2; i++ {
		if err := WriteFloat32(w, arr[i]); err != nil {
			return err
		}
	}
	return nil
}

func WriteFloat3(w io.Writer, arr [3]float32) error {
	for i := 0; i < 3; i++ {
		if err := WriteFloat32(w, arr[i]); err != nil {
			return err
		}
	}
	return nil
}

func WriteFloat4(w io.Writer, arr [4]float32) error {
	for i := 0; i < 4; i++ {
		if err := WriteFloat32(w, arr[i]); err != nil {
			return err
		}
	}
	return nil
}

func WriteFloat4x4(w io.Writer, arr [16]float32) error {
	for i := 0; i < 16; i++ {
		if err := WriteFloat32(w, arr[i]); err != nil {
			return err
		}
	}
	return nil
}
