package tools

import "fmt"

// 本文件提供 DXT 块压缩纹理的垂直翻转
// Unity 把贴图按自下而上行序存储，标准 DDS 按自上而下行序存储，两者相差一次垂直翻转
// 块压缩格式的翻转无法按行直接搬移，必须反转块行顺序再翻转每个块内部的四行索引
// This file provides the vertical flip for DXT block-compressed textures
// Unity stores textures bottom-up while standard DDS stores them top-down, so the two differ by one vertical flip
// A block-compressed payload cannot be flipped row by row, so the flip reverses the block row order and then flips the four index rows inside every block

// FlipDXTBlocksVertically 垂直翻转 DXT1 或 DXT5 载荷，用于在自上而下行序与 Unity 自下而上行序之间转换
// 该变换是自身的逆运算，因此导出与导入共用同一函数
// DXT1 与 DXT5 的块内数据都是按行存放的索引，所以翻转可以完全无损，不需要解压重压
// 尺寸不是 4 的倍数时边缘块的填充决定了结果是否精确，调用方应尽量传入 4 的倍数尺寸
// FlipDXTBlocksVertically vertically flips a DXT1 or DXT5 payload, converting between top-down row order and Unity's bottom-up row order
// The transform is its own inverse, so export and import share the same function
// Both DXT1 and DXT5 store their block payload as per-row indices, so the flip stays completely lossless and needs no decode and re-encode
// Padding in the edge blocks decides whether the result is exact when a dimension is not a multiple of four, so callers should prefer multiples of four
func FlipDXTBlocksVertically(data []byte, width int32, height int32, blockBytes int64) ([]byte, error) {
	if blockBytes != 8 && blockBytes != 16 {
		return nil, fmt.Errorf("unsupported DXT block size %d, want 8 for DXT1 or 16 for DXT5", blockBytes)
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid DXT dimensions: %dx%d", width, height)
	}

	blocksWide := (int64(width) + 3) / 4
	blocksHigh := (int64(height) + 3) / 4
	expectedLen := blocksWide * blocksHigh * blockBytes
	if int64(len(data)) != expectedLen {
		return nil, fmt.Errorf("unexpected DXT data size: got %d bytes, want %d for %dx%d with %d-byte blocks", len(data), expectedLen, width, height, blockBytes)
	}

	flipped := make([]byte, len(data))
	rowStride := blocksWide * blockBytes
	for blockY := int64(0); blockY < blocksHigh; blockY++ {
		srcRowOffset := blockY * rowStride
		dstRowOffset := (blocksHigh - 1 - blockY) * rowStride
		for blockX := int64(0); blockX < blocksWide; blockX++ {
			srcBlock := data[srcRowOffset+blockX*blockBytes : srcRowOffset+blockX*blockBytes+blockBytes]
			dstBlock := flipped[dstRowOffset+blockX*blockBytes : dstRowOffset+blockX*blockBytes+blockBytes]
			if blockBytes == 8 {
				flipDXT1Block(dstBlock, srcBlock)
			} else {
				flipDXT5Block(dstBlock, srcBlock)
			}
		}
	}

	return flipped, nil
}

// flipDXT1Block 垂直翻转一个 DXT1 块中的四行颜色索引
// DXT1 块的前 4 字节是颜色端点，后 4 字节是十六个 2 位索引，每行四个索引占一字节
// flipDXT1Block vertically flips the four color-index rows in one DXT1 block
// A DXT1 block holds the color endpoints in its first four bytes and sixteen 2-bit indices in the last four, with one byte per row of four indices
func flipDXT1Block(dst []byte, src []byte) {
	copy(dst[:4], src[:4])
	dst[4] = src[7]
	dst[5] = src[6]
	dst[6] = src[5]
	dst[7] = src[4]
}

// flipDXT5Block 垂直翻转一个 DXT5 块中的 Alpha 索引行和颜色索引行
// DXT5 块的前 2 字节是 Alpha 端点，接着 6 字节是十六个 3 位 Alpha 索引，后 8 字节与 DXT1 的颜色部分相同
// flipDXT5Block vertically flips the alpha-index and color-index rows in one DXT5 block
// A DXT5 block holds the alpha endpoints in its first two bytes, sixteen 3-bit alpha indices in the next six, and a DXT1-shaped color section in the last eight
func flipDXT5Block(dst []byte, src []byte) {
	copy(dst[:2], src[:2])

	// 十六个 3 位 Alpha 索引紧凑存放，每行四个索引占十二位，跨字节边界因此先拼成一个大整数再按行切分
	// The sixteen 3-bit alpha indices are packed tightly with one row of four indices per twelve bits, so they straddle byte boundaries and are reassembled into one integer before being split by row
	var alphaBits uint64
	for i := int64(0); i < 6; i++ {
		alphaBits |= uint64(src[2+i]) << (8 * i)
	}

	var rows [4]uint16
	for row := int64(0); row < 4; row++ {
		rows[row] = uint16((alphaBits >> (12 * row)) & 0x0FFF)
	}

	flippedAlphaBits := uint64(rows[3]) |
		(uint64(rows[2]) << 12) |
		(uint64(rows[1]) << 24) |
		(uint64(rows[0]) << 36)
	for i := int64(0); i < 6; i++ {
		dst[2+i] = byte(flippedAlphaBits >> (8 * i))
	}

	copy(dst[8:12], src[8:12])
	dst[12] = src[15]
	dst[13] = src[14]
	dst[14] = src[13]
	dst[15] = src[12]
}
