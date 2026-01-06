package arc

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// DataHasher replicates the C# DataHasher behavior
// Seeds and keys come from the C# implementation
var (
	seedA uint32 = 0x84222325
	seedB uint32 = 0xCBF29CE4
	keyA  uint32 = 0x00000100
	keyB  uint32 = 0x000001B3
)

// hashBytes computes the custom 64-bit hash over bytes
func hashBytes(data []byte, _seedA uint32, _seedB uint32, _keyA uint32, _keyB uint32) uint64 {
	if _seedA == 0 {
		_seedA = seedA
	}
	if _seedB == 0 {
		_seedB = seedB
	}
	if _keyA == 0 {
		_keyA = keyA
	}
	if _keyB == 0 {
		_keyB = keyB
	}

	sa := _seedA
	sb := _seedB
	for i := 0; i < len(data); i++ {
		sa ^= uint32(data[i])

		t0 := sa * _keyA
		t1 := sb * _keyB

		mul := uint64(sa) * uint64(_keyB)
		sa = uint32(mul)
		sb = uint32(mul>>32) + t0 + t1
	}
	sa ^= sb
	return (uint64(sb) << 32) | uint64(sa)
}

// utf16le encodes string (UTF-8) to UTF-16LE bytes
func utf16le(s string) []byte {
	r := []rune(s)
	u := utf16.Encode(r)
	out := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(out[i*2:], v)
	}
	return out
}

// NameHashUTF16 computes name hash (lowercased input) as UTF-16LE
func NameHashUTF16(name string) uint64 {
	return hashBytes(utf16le(strings.ToLower(name)), 0, 0, 0, 0)
}

// NameHashUTF8 computes name hash (lowercased input) as UTF-8
func NameHashUTF8(name string) uint64 {
	return hashBytes([]byte(strings.ToLower(name)), 0, 0, 0, 0)
}

// UniqueIDHash computes the unique id hash of a full name encoded as UTF-16LE without lowercasing
func UniqueIDHash(fullName string) uint64 {
	return hashBytes(utf16le(fullName), 0, 0, 0, 0)
}
