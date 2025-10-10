package stream

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
