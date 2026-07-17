package binaryio

import (
	"errors"
	"io"
)

// Read7BitEncodedInt 读取 C# 格式的 7-bit encoded int
// 匹配 .NET 4.8 的实现逻辑
func Read7BitEncodedInt(reader io.Reader) (int32, error) {
	var count uint32
	for byteIndex := 0; byteIndex < 5; byteIndex++ {
		// 读取一个字节
		b, err := ReadByte(reader)
		if err != nil {
			return 0, err
		}
		// An Int32 has only four meaningful bits in the fifth byte. This
		// still accepts all values emitted by Write7BitEncodedInt, including
		// negative ones, while rejecting encodings whose high bits would be
		// silently discarded by an Int32 shift.
		if byteIndex == 4 && b&0xF0 != 0 {
			return 0, errors.New("format exception: bad 7-bit encoded int32")
		}

		// 将低 7 位加入结果
		count |= uint32(b&0x7F) << (7 * byteIndex)

		// 如果最高位为 0，结束读取
		if (b & 0x80) == 0 {
			return int32(count), nil
		}
	}
	return 0, errors.New("format exception: bad 7-bit encoded int32")
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
