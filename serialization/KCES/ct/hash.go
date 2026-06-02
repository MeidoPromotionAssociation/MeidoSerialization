package ct

import (
	"unicode"
)

// FNV-1a 64-bit 算法常量。
// 对应 C# AssetManager.GetHash 中的两个魔数：
//   - 14695981039346656037 = 0xCBF29CE484222325 (offset basis)
//   - 1099511628211        = 0x100000001B3      (prime)
const (
	fnv1aOffsetBasis uint64 = 14695981039346656037
	fnv1aPrime       uint64 = 1099511628211
)

// HashBytes 计算字节序列的 FNV-1a 64-bit 哈希。
// 对应 C# AssetManager.GetHash(byte[])
//
//	空输入返回 0
func HashBytes(bytes []byte) uint64 {
	if len(bytes) == 0 {
		return 0
	}
	hash := fnv1aOffsetBasis
	for _, b := range bytes {
		hash ^= uint64(b)
		hash *= fnv1aPrime
	}
	return hash
}

// HashString 计算字符串的 FNV-1a 64-bit 哈希（区分大小写）。
// 对应 C# AssetManager.GetHash(string)：先 UTF-8 编码再哈希
//
//	空字符串返回 0
func HashString(text string) uint64 {
	if len(text) == 0 {
		return 0
	}
	return HashBytes([]byte(text))
}

// HashStringIgnoreCase 计算字符串的 FNV-1a 64-bit 哈希（忽略大小写）。
// 对应 C# AssetManager.GetHashIgnoreCase(string)。
//
// 算法：逐 char 处理（C# 中 char 是 UTF-16 code unit），ASCII 大写转小写后按 UTF-8 编码哈希。
// 非 ASCII 字符使用 ToLowerInvariant 转小写后再 UTF-8 编码。
//
// 注意：本实现按 rune 迭代而非 UTF-16 code unit 迭代，
// 对 BMP 内字符（包括 KCES 资源名中常见的日文）行为与 C# 一致；
// 仅在出现 surrogate pair 字符（U+10000 及以上）时才会与 C# 实现存在差异。
//
//	空字符串返回 0
func HashStringIgnoreCase(text string) uint64 {
	if len(text) == 0 {
		return 0
	}
	hash := fnv1aOffsetBasis
	for _, r := range text {
		// ASCII：使用快速小写映射
		if r < 0x80 {
			b := byte(r)
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			hash ^= uint64(b)
			hash *= fnv1aPrime
			continue
		}

		lower := unicode.ToLower(r)

		// 按 UTF-8 编码逐字节哈希。
		// C# 实现仅处理到 BMP（U+0000..U+FFFF），对应 1~3 字节 UTF-8；
		// 我们额外处理 4 字节 UTF-8 以覆盖 supplementary 字符。
		switch {
		case lower < 0x800:
			hash ^= uint64(byte(0xC0 | (lower >> 6)))
			hash *= fnv1aPrime
			hash ^= uint64(byte(0x80 | (lower & 0x3F)))
			hash *= fnv1aPrime
		case lower < 0x10000:
			hash ^= uint64(byte(0xE0 | (lower >> 12)))
			hash *= fnv1aPrime
			hash ^= uint64(byte(0x80 | ((lower >> 6) & 0x3F)))
			hash *= fnv1aPrime
			hash ^= uint64(byte(0x80 | (lower & 0x3F)))
			hash *= fnv1aPrime
		default:
			hash ^= uint64(byte(0xF0 | (lower >> 18)))
			hash *= fnv1aPrime
			hash ^= uint64(byte(0x80 | ((lower >> 12) & 0x3F)))
			hash *= fnv1aPrime
			hash ^= uint64(byte(0x80 | ((lower >> 6) & 0x3F)))
			hash *= fnv1aPrime
			hash ^= uint64(byte(0x80 | (lower & 0x3F)))
			hash *= fnv1aPrime
		}
	}
	return hash
}
