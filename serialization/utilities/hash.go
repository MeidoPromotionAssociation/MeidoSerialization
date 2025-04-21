package utilities

import (
	"crypto/sha256"
	"encoding/binary"
	"hash/fnv"
)

// GetStringHashInt32 计算字符串的哈希值并返回一个 int32 类型的值。
// 这个函数使用了 FNV-1a 哈希算法，将字符串转换为一个 32 位的哈希值。
// 由于 FNV-1a 哈希算法的输出是一个 64 位的哈希值，这里将其转换为 32 位。
func GetStringHashInt32(s string) int32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int32(h.Sum32()) //Cast to int32, possibly returning a negative number
}

// GetStringHashSHA256 计算字符串的哈希值并返回一个 int32 类型的值。
// 这个函数使用了 SHA-256 哈希算法，将字符串转换为一个 32 位的哈希值。
// 由于 SHA-256 哈希算法的输出是一个 256 位的哈希值，这里只取前 32 位作为返回值。
func GetStringHashSHA256(s string) int32 {
	h := sha256.New()
	h.Write([]byte(s))
	sum := h.Sum(nil)
	return int32(binary.BigEndian.Uint32(sum[:4])) // Take the first 32 digits
}
