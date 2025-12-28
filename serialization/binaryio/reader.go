package binaryio

import (
	"bufio"
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

// ReadBool 读取一个字节，返回 bool，如果字节非 0 则返回 true，否则返回 false
// 对应 C# 中的 BinaryReader.ReadBoolean
func ReadBool(r io.Reader) (bool, error) {
	b, err := ReadByte(r)
	if err != nil {
		return false, fmt.Errorf("read bool failed: %w", err)
	}
	return b != 0, nil
}

// ReadInt8 读取 1 字节有符号整数
func ReadInt8(r io.Reader) (int8, error) {
	var buf [1]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int8(buf[0]), nil
}

// ReadInt16 读取 2 字节有符号整数 (little-endian)
func ReadInt16(r io.Reader) (int16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf[:])), nil
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

// ReadInt64 读取 8 字节有符号整数 (little-endian)
func ReadInt64(r io.Reader) (int64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf[:])), nil
}

// ReadUInt64 读取 8 字节无符号整数 (little-endian)
func ReadUInt64(r io.Reader) (uint64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
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

// ReadFloat64 读取 8 字节浮点数 (little-endian)
func ReadFloat64(r io.Reader) (float64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint64(buf[:])
	return math.Float64frombits(bits), nil
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

// Peeker 定义了实现了 Peek 方法的接口，bufio.Reader 实现了该接口。
type Peeker interface {
	Peek(n int) ([]byte, error)
	io.Reader
}

// PeekByte 偷看下一个字节，不移动读取指针。
// 要求 reader 实现 Peeker 接口，例如 bufio.Reader。
func PeekByte(r io.Reader) (byte, error) {
	br, ok := r.(Peeker)
	if !ok {
		return 0, fmt.Errorf("peekByte: the reader is not peekable, wrap it with bufio.Reader first")
	}

	b, err := br.Peek(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// PeekString 读取下一个字符串（7BitEncode + UTF-8），但不消耗它。
// 要求 reader 实现 Peeker 接口，例如 bufio.Reader。
//
// 注意：如果字符串长度超过了 bufio.Reader 的缓冲区大小（默认 4KB），
// Peek 操作会返回 bufio.ErrBufferFull。
// 如果需要处理超大字符串，请在创建 bufio.Reader 时指定更大的缓冲区大小
func PeekString(r io.Reader) (string, error) {
	br, ok := r.(Peeker)
	if !ok {
		return "", fmt.Errorf("peekString: the reader is not peekable, wrap it with bufio.Reader first")
	}

	var stringLength int // 解析出的字符串长度
	var shift int        // 当前位偏移量
	var prefixLen int    // 长度前缀占用了多少个字节

	for {
		// Peek 字节直到找到 7-bit 整数的结尾
		// 每次多 Peek 一个字节来检查
		bSlice, err := br.Peek(prefixLen + 1)
		if err != nil {
			return "", err
		}

		b := bSlice[prefixLen]
		prefixLen++

		// 7-Bit 解码逻辑
		stringLength |= (int(b) & 0x7F) << shift
		shift += 7

		// 如果最高位是 0，说明整数结束
		if (b & 0x80) == 0 {
			break
		}

		if prefixLen >= 5 {
			return "", fmt.Errorf("peekString: string length prefix too large")
		}
	}

	// 2. 检查长度有效性
	if stringLength < 0 {
		return "", fmt.Errorf("peekString: invalid string length: %d", stringLength)
	}
	if stringLength == 0 {
		return "", nil
	}

	totalLen := prefixLen + stringLength

	// 尝试 Peek 所有数据
	data, err := br.Peek(totalLen)
	if err != nil {
		if errors.Is(err, bufio.ErrBufferFull) {
			return "", fmt.Errorf("peekString: string length (%d) exceeds buffer size (increase buffer size)", totalLen)
		}
		return "", err
	}

	// 拷贝一次，确保后续读取不影响结果
	return string(data[prefixLen:]), nil
}
