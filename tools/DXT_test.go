package tools

import (
	"bytes"
	"testing"
)

func TestFlipDXTBlocksVerticallyIsItsOwnInverse(t *testing.T) {
	for _, blockBytes := range []int64{8, 16} {
		// 8×8 有 2×2 个块，内容与块结构不对称，因此翻转必须真的改变载荷
		data := make([]byte, 2*2*blockBytes)
		for i := range data {
			data[i] = byte(i * 7)
		}
		flipped, err := FlipDXTBlocksVertically(data, 8, 8, blockBytes)
		if err != nil {
			t.Fatalf("blockBytes=%d: %v", blockBytes, err)
		}
		if bytes.Equal(flipped, data) {
			t.Fatalf("blockBytes=%d: flip did not change a block-asymmetric payload", blockBytes)
		}
		restored, err := FlipDXTBlocksVertically(flipped, 8, 8, blockBytes)
		if err != nil {
			t.Fatalf("blockBytes=%d second flip: %v", blockBytes, err)
		}
		if !bytes.Equal(restored, data) {
			t.Fatalf("blockBytes=%d: flip is not its own inverse", blockBytes)
		}
	}
}

func TestFlipDXT1BlockKeepsEndpointsAndReversesIndexRows(t *testing.T) {
	// 单个 4×4 DXT1 块：颜色端点必须原样保留，四个索引字节必须倒序
	data := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0x01, 0x02, 0x03, 0x04}
	flipped, err := FlipDXTBlocksVertically(data, 4, 4, 8)
	if err != nil {
		t.Fatalf("FlipDXTBlocksVertically: %v", err)
	}
	want := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0x04, 0x03, 0x02, 0x01}
	if !bytes.Equal(flipped, want) {
		t.Fatalf("flipped = % x, want % x", flipped, want)
	}
}

func TestFlipDXT5BlockKeepsEndpointsAndReversesAlphaRows(t *testing.T) {
	// 单个 4×4 DXT5 块：Alpha 端点与颜色端点原样保留，Alpha 索引行与颜色索引行倒序
	// 每行四个 3 位 Alpha 索引，行 r 落在第 12r 位，因此第 0 行取 0x001、第 1 行取 0x002、第 2 行取 0x004、第 3 行取 0x008
	var data [16]byte
	data[0], data[1] = 0x11, 0x22
	alphaBits := uint64(0x001) | uint64(0x002)<<12 | uint64(0x004)<<24 | uint64(0x008)<<36
	for i := 0; i < 6; i++ {
		data[2+i] = byte(alphaBits >> (8 * i))
	}
	data[8], data[9], data[10], data[11] = 0x33, 0x44, 0x55, 0x66
	data[12], data[13], data[14], data[15] = 0x0A, 0x0B, 0x0C, 0x0D

	flipped, err := FlipDXTBlocksVertically(data[:], 4, 4, 16)
	if err != nil {
		t.Fatalf("FlipDXTBlocksVertically: %v", err)
	}
	if flipped[0] != 0x11 || flipped[1] != 0x22 {
		t.Fatalf("alpha endpoints changed: % x", flipped[:2])
	}
	if !bytes.Equal(flipped[8:12], []byte{0x33, 0x44, 0x55, 0x66}) {
		t.Fatalf("color endpoints changed: % x", flipped[8:12])
	}
	if !bytes.Equal(flipped[12:16], []byte{0x0D, 0x0C, 0x0B, 0x0A}) {
		t.Fatalf("color index rows not reversed: % x", flipped[12:16])
	}

	var gotAlpha uint64
	for i := 0; i < 6; i++ {
		gotAlpha |= uint64(flipped[2+i]) << (8 * i)
	}
	wantAlpha := uint64(0x008) | uint64(0x004)<<12 | uint64(0x002)<<24 | uint64(0x001)<<36
	if gotAlpha != wantAlpha {
		t.Fatalf("alpha index rows not reversed: got %#x, want %#x", gotAlpha, wantAlpha)
	}
}

func TestFlipDXTBlocksVerticallyRejectsBadInput(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		width      int32
		height     int32
		blockBytes int64
	}{
		{name: "unsupported_block_size", data: make([]byte, 8), width: 4, height: 4, blockBytes: 12},
		{name: "zero_width", data: make([]byte, 8), width: 0, height: 4, blockBytes: 8},
		{name: "negative_height", data: make([]byte, 8), width: 4, height: -4, blockBytes: 8},
		{name: "short_payload", data: make([]byte, 7), width: 4, height: 4, blockBytes: 8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := FlipDXTBlocksVertically(tc.data, tc.width, tc.height, tc.blockBytes); err == nil {
				t.Fatal("FlipDXTBlocksVertically unexpectedly succeeded")
			}
		})
	}
}
