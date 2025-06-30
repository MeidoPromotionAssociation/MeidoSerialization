package utilities

import (
	"crypto/sha256"
	"encoding/binary"
	"hash/crc32"
	"hash/fnv"
)

// GetStringHashFNV1a 计算字符串的哈希值并返回一个 int32 类型的值。
// 这个函数使用了 FNV-1a 哈希算法，将字符串转换为一个 32 位的哈希值。
func GetStringHashFNV1a(s string) (int32, error) {
	h := fnv.New32a()
	_, err := h.Write([]byte(s))
	if err != nil {
		return 0, err
	}
	return int32(h.Sum32()), nil //Cast to int32, possibly returning a negative number
}

// GetStringHashSHA256 计算字符串的哈希值并返回一个 int32 类型的值。
// 这个函数使用了 SHA-256 哈希算法，将字符串转换为一个 32 位的哈希值。
// 由于 SHA-256 哈希算法的输出是一个 256 位的哈希值，这里只取前 32 位作为返回值。
func GetStringHashSHA256(s string) (int32, error) {
	h := sha256.New()
	_, err := h.Write([]byte(s))
	if err != nil {
		return 0, err
	}
	sum := h.Sum(nil)
	return int32(binary.BigEndian.Uint32(sum[:4])), nil // Take the first 32 digits
}

// GetStringHashCRC32 计算字符串的哈希值并返回一个 int32 类型的值。
// 这个函数使用了 CRC-32 哈希算法，将字符串转换为一个 32 位的哈希值。
func GetStringHashCRC32(s string) (int32, error) {
	h := crc32.NewIEEE()
	_, err := h.Write([]byte(s))
	if err != nil {
		return 0, err
	}
	return int32(h.Sum32()), nil
}
