package utilities

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// ReadByte 读 1 字节
func ReadByte(r io.Reader) (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r, b[:])
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// ReadBytes 读取 n 个字节
func ReadBytes(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

// ReadInt32 读取 4 字节 int32（little-endian）
func ReadInt32(r io.Reader) (int32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf[:])), nil
}

// ReadUInt32 读取 4 字节无符号整数 (little-endian)
func ReadUInt32(r io.Reader) (uint32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

// ReadUInt16 读取 2 字节无符号整数 (little-endian)
func ReadUInt16(r io.Reader) (uint16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(buf[:]), nil
}

// ReadFloat32 从 r 中读取 4 个字节，以 little-endian 解码成 float32
// 在 C# 中是 BinaryReader.ReadSingle
func ReadFloat32(r io.Reader) (float32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint32(buf[:])
	return math.Float32frombits(bits), nil
}

// ReadString 读取 C# BinaryWriter.WriteString 格式的字符串
// 完全匹配 .NET 4.8 的实现逻辑
// 需要注意这个 7BitEncode 虽然与 LEB128 类似，但不是完全相同
func ReadString(r io.Reader) (string, error) {
	// 读取字符串的字节长度（不是字符长度）
	stringLength, err := Read7BitEncodedInt(r)
	if err != nil {
		return "", err
	}

	// 检查长度有效性
	if stringLength < 0 {
		return "", fmt.Errorf("invalid string length: %d", stringLength)
	}

	if stringLength == 0 {
		return "", nil
	}

	// 对于 Go 来说，由于 string 本身就是 UTF-8，我们可以简化处理
	// 直接读取所有字节并转换为字符串
	buffer := make([]byte, stringLength)
	_, err = io.ReadFull(r, buffer)
	if err != nil {
		if err == io.EOF {
			return "", errors.New("unexpected end of stream while reading string")
		}
		return "", err
	}

	return string(buffer), nil
}

// ReadBool 读取一个字节，返回 bool，如果字节非 0 则返回 true，否则返回 false
// 对应 C# 中的 BinaryReader.ReadBoolean
func ReadBool(r io.Reader) (bool, error) {
	b, err := ReadByte(r)
	if err != nil {
		return false, fmt.Errorf("read bool failed: %w", err)
	}
	return b != 0, nil
}

// -------------------- Float2 / Float3 / Float4 / Float4x4 --------------------

func ReadFloat2(r io.Reader) ([2]float32, error) {
	var arr [2]float32
	for i := 0; i < 2; i++ {
		f, err := ReadFloat32(r)
		if err != nil {
			return arr, err
		}
		arr[i] = f
	}
	return arr, nil
}

func ReadFloat3(r io.Reader) ([3]float32, error) {
	var arr [3]float32
	for i := 0; i < 3; i++ {
		f, err := ReadFloat32(r)
		if err != nil {
			return arr, err
		}
		arr[i] = f
	}
	return arr, nil
}

func ReadFloat4(r io.Reader) ([4]float32, error) {
	var arr [4]float32
	for i := 0; i < 4; i++ {
		f, err := ReadFloat32(r)
		if err != nil {
			return arr, err
		}
		arr[i] = f
	}
	return arr, nil
}

func ReadFloat4x4(r io.Reader) ([16]float32, error) {
	var arr [16]float32
	for i := 0; i < 16; i++ {
		f, err := ReadFloat32(r)
		if err != nil {
			return arr, err
		}
		arr[i] = f
	}
	return arr, nil
}

// -------------------- Peek --------------------

// PeekByte 偷看下一个字节，不移动读取指针。
// 这里需要一个带缓冲、可 Peek 的包装，如 bufio.Reader。
func PeekByte(r io.Reader) (byte, error) {
	br, ok := r.(interface {
		Peek(int) ([]byte, error)
	})
	if !ok {
		// 如果没有实现 Peek，需要自己封装一个 bufio.Reader
		return 0, fmt.Errorf("PeekByte: the reader is not peekable, wrap it with bufio.Reader first")
	}
	bytes, err := br.Peek(1)
	if err != nil {
		return 0, err
	}
	return bytes[0], nil
}

// PeekString 读取下一个字符串（LEB128 + UTF-8），但不消耗它。
// 因此下次再从同一个 reader 中读取时，会得到相同的数据。
func PeekString(rs io.ReadSeeker) (string, error) {
	// 记录当前指针
	startPos, err := rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}

	// 尝试读取字符串
	str, err := ReadString(rs)
	if err != nil {
		// 如果出错了也回退
		_, _ = rs.Seek(startPos, io.SeekStart)
		return "", err
	}

	// 读完后回退到之前的位置
	_, err = rs.Seek(startPos, io.SeekStart)
	if err != nil {
		return "", err
	}

	return str, nil
}
