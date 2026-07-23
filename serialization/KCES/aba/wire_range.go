package aba

import (
	"fmt"
	"math"
)

// int32WireLength 将长度转换为 Int32 线格式值并拒绝超出范围的输入
// int32WireLength converts a length to its Int32 wire value and rejects out-of-range input
func int32WireLength(field string, length uint64) (int32, error) {
	if length > uint64(math.MaxInt32) {
		return 0, fmt.Errorf("%s %d exceeds Int32 wire range", field, length)
	}
	return int32(length), nil
}

// uint32WireLength 将长度转换为 UInt32 线格式值并拒绝超出范围的输入
// uint32WireLength converts a length to its UInt32 wire value and rejects out-of-range input
func uint32WireLength(field string, length uint64) (uint32, error) {
	if length > uint64(math.MaxUint32) {
		return 0, fmt.Errorf("%s %d exceeds UInt32 wire range", field, length)
	}
	return uint32(length), nil
}
