package stream

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// BinaryReader 提供从流中读取 C# 基本类型的功能
// 共享缓冲，性能更高，不支持并发
type BinaryReader struct {
	R      io.Reader
	buffer [64]byte // 用于读取基本类型的临时缓冲区
}

// NewBinaryReader 创建一个新的 BinaryReader
func NewBinaryReader(r io.Reader) *BinaryReader {
	return &BinaryReader{R: r}
}

// ReadBool 读取布尔值
func (br *BinaryReader) ReadBool() (bool, error) {
	b, err := br.ReadByte()
	if err != nil {
		return false, err
	}
	return b != 0, nil
}

// ReadByte 读取单个字节
func (br *BinaryReader) ReadByte() (byte, error) {
	_, err := io.ReadFull(br.R, br.buffer[:1])
	if err != nil {
		return 0, err
	}
	return br.buffer[0], nil
}

// ReadSByte 读取有符号字节
func (br *BinaryReader) ReadSByte() (int8, error) {
	b, err := br.ReadByte()
	return int8(b), err
}

// ReadInt16 读取 2 字节有符号整数 (little-endian)
func (br *BinaryReader) ReadInt16() (int16, error) {
	_, err := io.ReadFull(br.R, br.buffer[:2])
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(br.buffer[:2])), nil
}

// ReadUInt16 读取 2 字节无符号整数 (little-endian)
func (br *BinaryReader) ReadUInt16() (uint16, error) {
	_, err := io.ReadFull(br.R, br.buffer[:2])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(br.buffer[:2]), nil
}

// ReadInt32 读取 4 字节有符号整数 (little-endian)
func (br *BinaryReader) ReadInt32() (int32, error) {
	_, err := io.ReadFull(br.R, br.buffer[:4])
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(br.buffer[:4])), nil
}

// ReadUInt32 读取 4 字节无符号整数 (little-endian)
func (br *BinaryReader) ReadUInt32() (uint32, error) {
	_, err := io.ReadFull(br.R, br.buffer[:4])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(br.buffer[:4]), nil
}

// ReadInt64 读取 8 字节有符号整数 (little-endian)
func (br *BinaryReader) ReadInt64() (int64, error) {
	_, err := io.ReadFull(br.R, br.buffer[:8])
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(br.buffer[:8])), nil
}

// ReadUInt64 读取 8 字节无符号整数 (little-endian)
func (br *BinaryReader) ReadUInt64() (uint64, error) {
	_, err := io.ReadFull(br.R, br.buffer[:8])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(br.buffer[:8]), nil
}

// ReadFloat32 读取 4 字节浮点数 (little-endian)
func (br *BinaryReader) ReadFloat32() (float32, error) {
	_, err := io.ReadFull(br.R, br.buffer[:4])
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint32(br.buffer[:4])
	return math.Float32frombits(bits), nil
}

// ReadFloat64 读取 8 字节浮点数 (little-endian)
func (br *BinaryReader) ReadFloat64() (float64, error) {
	_, err := io.ReadFull(br.R, br.buffer[:8])
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint64(br.buffer[:8])
	return math.Float64frombits(bits), nil
}

// ReadString 读取长度前缀的 UTF-8 字符串（与 C# BinaryWriter 兼容）
func (br *BinaryReader) ReadString() (string, error) {
	// 读取 7-bit 编码的长度
	length, err := br.read7BitEncodedInt()
	if err != nil {
		return "", err
	}

	if length < 0 {
		return "", errors.New("invalid string length")
	}

	if length == 0 {
		return "", nil
	}

	// 读取字符串字节
	buf := make([]byte, length)
	_, err = io.ReadFull(br.R, buf)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// read7BitEncodedInt 读取 7-bit 编码的整数
func (br *BinaryReader) read7BitEncodedInt() (int, error) {
	count := 0
	shift := 0

	for {
		if shift == 5*7 {
			return 0, errors.New("format error: bad 7-bit int32")
		}

		b, err := br.ReadByte()
		if err != nil {
			return 0, err
		}

		count |= int(b&0x7F) << shift
		shift += 7

		if (b & 0x80) == 0 {
			break
		}
	}

	return count, nil
}

// ReadBytes 读取指定数量的字节
func (br *BinaryReader) ReadBytes(count int) ([]byte, error) {
	if count < 0 {
		return nil, errors.New("count cannot be negative")
	}

	if count == 0 {
		return []byte{}, nil
	}

	buf := make([]byte, count)
	_, err := io.ReadFull(br.R, buf)
	return buf, err
}

// -------------------- Float2 / Float3 / Float4 / Float4x4 --------------------

// ReadFloat2 读取 2 个连续的 float32 (Vector2)
func (br *BinaryReader) ReadFloat2() ([2]float32, error) {
	var arr [2]float32
	_, err := io.ReadFull(br.R, br.buffer[:8])
	if err != nil {
		return arr, err
	}
	arr[0] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[0:4]))
	arr[1] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[4:8]))
	return arr, nil
}

// ReadFloat3 读取 3 个连续的 float32 (Vector3)
func (br *BinaryReader) ReadFloat3() ([3]float32, error) {
	var arr [3]float32
	_, err := io.ReadFull(br.R, br.buffer[:12])
	if err != nil {
		return arr, err
	}
	arr[0] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[0:4]))
	arr[1] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[4:8]))
	arr[2] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[8:12]))
	return arr, nil
}

// ReadFloat4 读取 4 个连续的 float32 (Vector4/Quaternion)
func (br *BinaryReader) ReadFloat4() ([4]float32, error) {
	var arr [4]float32
	_, err := io.ReadFull(br.R, br.buffer[:16])
	if err != nil {
		return arr, err
	}
	arr[0] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[0:4]))
	arr[1] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[4:8]))
	arr[2] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[8:12]))
	arr[3] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[12:16]))
	return arr, nil
}

// ReadFloat4x4 读取 16 个连续的 float32 (4x4 Matrix)
func (br *BinaryReader) ReadFloat4x4() ([16]float32, error) {
	var arr [16]float32
	_, err := io.ReadFull(br.R, br.buffer[:64])
	if err != nil {
		return arr, err
	}
	for i := 0; i < 16; i++ {
		arr[i] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[i*4 : i*4+4]))
	}
	return arr, nil
}

// -------------------- Peek 操作 --------------------

// Peeker 定义了实现了 Peek 方法的接口，bufio.Reader 实现了该接口。
type Peeker interface {
	Peek(n int) ([]byte, error)
	io.Reader
}

// PeekByte 偷看下一个字节，不移动读取指针。
// 要求 reader 实现 Peeker 接口，例如 bufio.Reader。
func (br *BinaryReader) PeekByte() (byte, error) {
	peeker, ok := br.R.(Peeker)
	if !ok {
		return 0, errors.New("peekByte: underlying reader does not support Peek (wrap it with bufio.NewReader)")
	}

	b, err := peeker.Peek(1)
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
func (br *BinaryReader) PeekString() (string, error) {
	peeker, ok := br.R.(Peeker)
	if !ok {
		return "", errors.New("peekString: underlying reader does not support Peek (wrap it with bufio.NewReader)")
	}

	var stringLength int // 解析出的字符串长度
	var shift int        // 当前位偏移量
	var prefixLen int    // 长度前缀占用了多少个字节

	for {
		// Peek 字节直到找到 7-bit 整数的结尾
		// 每次多 Peek 一个字节来检查
		bSlice, err := peeker.Peek(prefixLen + 1)
		if err != nil {
			return "", err
		}

		b := bSlice[prefixLen] // 获取刚刚 Peek 到的最后一个字节
		prefixLen++

		// 7-Bit 解码
		stringLength |= (int(b) & 0x7F) << shift
		shift += 7

		// 如果最高位是 0，说明整数结束
		if (b & 0x80) == 0 {
			break
		}

		if prefixLen >= 5 {
			return "", errors.New("peekString: string length prefix too large")
		}
	}

	// 检查长度有效性
	if stringLength < 0 {
		return "", fmt.Errorf("peekString: invalid string length: %d", stringLength)
	}
	if stringLength == 0 {
		return "", nil
	}

	// Peek 完整的数据 (长度前缀 + 字符串内容)
	totalLen := prefixLen + stringLength

	data, err := peeker.Peek(totalLen)
	if err != nil {
		if errors.Is(err, bufio.ErrBufferFull) {
			return "", fmt.Errorf("peekString: string length (%d) exceeds buffer size (increase buffer size)", totalLen)
		}
		return "", err
	}

	// 拷贝一次，确保后续读取不影响结果
	return string(data[prefixLen:]), nil
}
