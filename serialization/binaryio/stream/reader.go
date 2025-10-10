package stream

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

// BinaryReader 提供从流中读取基本类型的功能
type BinaryReader struct {
	r      io.Reader
	buffer [64]byte // 用于读取基本类型的临时缓冲区
}

// NewBinaryReader 创建一个新的 BinaryReader
func NewBinaryReader(r io.Reader) *BinaryReader {
	return &BinaryReader{r: r}
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
	_, err := io.ReadFull(br.r, br.buffer[:1])
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
	_, err := io.ReadFull(br.r, br.buffer[:2])
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(br.buffer[:2])), nil
}

// ReadUInt16 读取 2 字节无符号整数 (little-endian)
func (br *BinaryReader) ReadUInt16() (uint16, error) {
	_, err := io.ReadFull(br.r, br.buffer[:2])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(br.buffer[:2]), nil
}

// ReadInt32 读取 4 字节有符号整数 (little-endian)
func (br *BinaryReader) ReadInt32() (int32, error) {
	_, err := io.ReadFull(br.r, br.buffer[:4])
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(br.buffer[:4])), nil
}

// ReadUInt32 读取 4 字节无符号整数 (little-endian)
func (br *BinaryReader) ReadUInt32() (uint32, error) {
	_, err := io.ReadFull(br.r, br.buffer[:4])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(br.buffer[:4]), nil
}

// ReadInt64 读取 8 字节有符号整数 (little-endian)
func (br *BinaryReader) ReadInt64() (int64, error) {
	_, err := io.ReadFull(br.r, br.buffer[:8])
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(br.buffer[:8])), nil
}

// ReadUInt64 读取 8 字节无符号整数 (little-endian)
func (br *BinaryReader) ReadUInt64() (uint64, error) {
	_, err := io.ReadFull(br.r, br.buffer[:8])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(br.buffer[:8]), nil
}

// ReadFloat32 读取 4 字节浮点数 (little-endian)
func (br *BinaryReader) ReadFloat32() (float32, error) {
	_, err := io.ReadFull(br.r, br.buffer[:4])
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint32(br.buffer[:4])
	return math.Float32frombits(bits), nil
}

// ReadFloat64 读取 8 字节浮点数 (little-endian)
func (br *BinaryReader) ReadFloat64() (float64, error) {
	_, err := io.ReadFull(br.r, br.buffer[:8])
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
	_, err = io.ReadFull(br.r, buf)
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
	_, err := io.ReadFull(br.r, buf)
	return buf, err
}

// -------------------- Float2 / Float3 / Float4 / Float4x4 --------------------

// ReadFloat2 读取 2 个连续的 float32 (Vector2)
func (br *BinaryReader) ReadFloat2() ([2]float32, error) {
	var arr [2]float32
	_, err := io.ReadFull(br.r, br.buffer[:8])
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
	_, err := io.ReadFull(br.r, br.buffer[:12])
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
	_, err := io.ReadFull(br.r, br.buffer[:16])
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
	_, err := io.ReadFull(br.r, br.buffer[:64])
	if err != nil {
		return arr, err
	}
	for i := 0; i < 16; i++ {
		arr[i] = math.Float32frombits(binary.LittleEndian.Uint32(br.buffer[i*4 : i*4+4]))
	}
	return arr, nil
}

// -------------------- Peek 操作 --------------------

// PeekByte 偷看下一个字节，不移动读取指针
// 如果底层 reader 不支持 Peek，会自动包装为 bufio.Reader
func (br *BinaryReader) PeekByte() (byte, error) {
	// 检查是否已经支持 Peek
	if peeker, ok := br.r.(interface{ Peek(int) ([]byte, error) }); ok {
		bytes, err := peeker.Peek(1)
		if err != nil {
			return 0, err
		}
		return bytes[0], nil
	}

	// 如果不支持，尝试将其包装为 bufio.Reader
	if _, ok := br.r.(*bufio.Reader); !ok {
		br.r = bufio.NewReader(br.r)
	}

	if peeker, ok := br.r.(interface{ Peek(int) ([]byte, error) }); ok {
		bytes, err := peeker.Peek(1)
		if err != nil {
			return 0, err
		}
		return bytes[0], nil
	}

	return 0, errors.New("PeekByte: reader does not support Peek")
}

// PeekString 偷看下一个字符串（7-bit 长度前缀 + UTF-8），不消耗数据
// 要求底层 reader 实现 io.ReadSeeker 接口
func (br *BinaryReader) PeekString() (string, error) {
	seeker, ok := br.r.(io.ReadSeeker)
	if !ok {
		return "", errors.New("PeekString: reader does not support Seek")
	}

	// 记录当前位置
	startPos, err := seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}

	// 尝试读取字符串
	str, err := br.ReadString()

	// 无论成功或失败，都回退到原位置
	_, seekErr := seeker.Seek(startPos, io.SeekStart)
	if seekErr != nil {
		return "", seekErr
	}

	return str, err
}
