package ct

import (
	"unicode"
	"unicode/utf16"
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
// 算法：逐 UTF-16 code unit 处理（C# 中 char 是 UTF-16 code unit），ASCII 大写转小写后哈希。
// 非 ASCII code unit 使用 ToLowerInvariant 转小写后按 C# 代码的 UTF-8 分支写入 2 或 3 字节。
//
//	空字符串返回 0
func HashStringIgnoreCase(text string) uint64 {
	if len(text) == 0 {
		return 0
	}
	hash := fnv1aOffsetBasis
	for _, codeUnit := range utf16.Encode([]rune(text)) {
		// ASCII：使用快速小写映射
		if codeUnit < 0x80 {
			b := byte(codeUnit)
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			hash ^= uint64(b)
			hash *= fnv1aPrime
			continue
		}

		lower := uint16(unicode.ToLower(rune(codeUnit)))

		// 这里刻意不合并 surrogate pair。游戏按 char 逐个 code unit 处理，
		// 因此 U+10000 以上字符会作为两个 surrogate 分别进入 3 字节分支。
		if lower < 0x800 {
			hash ^= uint64(byte(0xC0 | (lower >> 6)))
			hash *= fnv1aPrime
			hash ^= uint64(byte(0x80 | (lower & 0x3F)))
			hash *= fnv1aPrime
			continue
		}
		hash ^= uint64(byte(0xE0 | (lower >> 12)))
		hash *= fnv1aPrime
		hash ^= uint64(byte(0x80 | ((lower >> 6) & 0x3F)))
		hash *= fnv1aPrime
		hash ^= uint64(byte(0x80 | (lower & 0x3F)))
		hash *= fnv1aPrime
	}
	return hash
}
