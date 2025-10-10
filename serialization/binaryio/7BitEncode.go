package binaryio

import (
	"errors"
	"io"
)

// Read7BitEncodedInt 读取 C# 格式的 7-bit encoded int
// 完全匹配 .NET 4.8 的实现逻辑
func Read7BitEncodedInt(reader io.Reader) (int32, error) {
	var count int32
	var shift uint

	for {
		// 检查是否超过最大字节数（5 字节）
		if shift == 5*7 { // 5 bytes max per Int32
			return 0, errors.New("format exception: bad 7-bit encoded int32")
		}

		// 读取一个字节
		b, err := ReadByte(reader)
		if err != nil {
			return 0, err
		}

		// 将低 7 位加入结果
		count |= int32(b&0x7F) << shift
		shift += 7

		// 如果最高位为 0，结束读取
		if (b & 0x80) == 0 {
			break
		}
	}

	return count, nil
}

// Write7BitEncodedInt 写入 C# 格式的 7-bit encoded int
// 完全匹配 .NET 4.8 的实现：支持负数，使用无符号转换
func Write7BitEncodedInt(writer io.Writer, value int32) error {
	// 转换为无符号数以支持负数（与 C# 源码一致）
	v := uint32(value)
	for v >= 0x80 {
		err := WriteByte(writer, byte(v|0x80))
		if err != nil {
			return err
		}
		v >>= 7
	}
	return WriteByte(writer, byte(v))
}

// Get7BitEncodedIntSize 计算编码一个 int32 值所需的字节数
// 与 Write7BitEncodedInt 保持一致的编码逻辑
func Get7BitEncodedIntSize(value int32) int {
	// 转换为无符号数以支持负数（与 C# 源码一致）
	v := uint32(value)

	// 至少需要1个字节
	size := 1

	// 计算需要多少个额外字节（每个字节可以编码7位）
	for v >= 0x80 {
		size++
		v >>= 7
	}

	return size
}
