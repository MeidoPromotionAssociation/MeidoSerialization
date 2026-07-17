package aba

import (
	"fmt"
	"math"
)

func int32WireLength(field string, length uint64) (int32, error) {
	if length > uint64(math.MaxInt32) {
		return 0, fmt.Errorf("%s %d exceeds Int32 wire range", field, length)
	}
	return int32(length), nil
}

func uint32WireLength(field string, length uint64) (uint32, error) {
	if length > uint64(math.MaxUint32) {
		return 0, fmt.Errorf("%s %d exceeds UInt32 wire range", field, length)
	}
	return uint32(length), nil
}
