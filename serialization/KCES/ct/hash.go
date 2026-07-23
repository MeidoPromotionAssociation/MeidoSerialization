package ct

import (
	"unicode"
	"unicode/utf16"
)

// FNV-1a 64-bit 算法常量
// 对应 C# AssetManager.GetHash 中的偏移基数 14695981039346656037 和质数 1099511628211
// FNV-1a 64-bit algorithm constants
// These correspond to the offset basis 14695981039346656037 and prime 1099511628211 in C# AssetManager.GetHash
const (
	fnv1aOffsetBasis uint64 = 14695981039346656037
	fnv1aPrime       uint64 = 1099511628211
)

// HashBytes 计算字节序列的 FNV-1a 64-bit 哈希，空输入返回零
// 对应 C# AssetManager.GetHash(byte[])
// HashBytes computes the FNV-1a 64-bit hash of a byte sequence and returns zero for empty input
// It corresponds to C# AssetManager.GetHash(byte[])
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

// HashString 对字符串 UTF-8 编码后计算区分大小写的 FNV-1a 64-bit 哈希，空字符串返回零
// 对应 C# AssetManager.GetHash(string)：先 UTF-8 编码再哈希
// HashString computes a case-sensitive FNV-1a 64-bit hash after UTF-8 encoding a string and returns zero for an empty string
// It corresponds to C# AssetManager.GetHash(string), which encodes as UTF-8 before hashing
func HashString(text string) uint64 {
	if len(text) == 0 {
		return 0
	}
	return HashBytes([]byte(text))
}

// HashStringIgnoreCase 计算与 C# AssetManager.GetHashIgnoreCase(string) 一致的忽略大小写 FNV-1a 64-bit 哈希，空字符串返回零
// 算法逐个处理 UTF-16 code unit，ASCII 大写先转小写，非 ASCII code unit 使用 ToLowerInvariant 后按游戏分支写入两个或三个 UTF-8 形式字节
// HashStringIgnoreCase computes the case-insensitive FNV-1a 64-bit hash matching C# AssetManager.GetHashIgnoreCase(string) and returns zero for an empty string
// The algorithm processes UTF-16 code units individually, lower-cases ASCII directly, and applies ToLowerInvariant to non-ASCII code units before writing two or three UTF-8-form bytes through the game branches
func HashStringIgnoreCase(text string) uint64 {
	if len(text) == 0 {
		return 0
	}
	hash := fnv1aOffsetBasis
	for _, codeUnit := range utf16.Encode([]rune(text)) {
		// ASCII 使用快速小写映射
		// ASCII uses the fast lower-case mapping
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

		// 此处刻意不合并 surrogate pair，游戏按 char 逐个 code unit 处理，因此 U+10000 以上字符会作为两个 surrogate 分别进入三字节分支
		// Surrogate pairs are deliberately not combined because the game processes each char code unit separately, so characters above U+10000 enter the three-byte branch as two surrogates
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
