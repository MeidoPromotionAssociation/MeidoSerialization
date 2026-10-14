package aba

import (
	"bytes"
	"testing"
)

func TestMaxMipCount(t *testing.T) {
	tests := []struct {
		width  int64
		height int64
		want   int32
	}{
		{width: 1, height: 1, want: 1},
		{width: 2, height: 2, want: 2},
		{width: 4, height: 4, want: 3},
		{width: 256, height: 32, want: 9},
		{width: 512, height: 256, want: 10},
		{width: 1024, height: 1024, want: 11},
		{width: 1024, height: 4, want: 11},
		{width: 2048, height: 2048, want: 12},
		{width: 0, height: 4, want: 0},
		{width: 4, height: -1, want: 0},
	}
	for _, tc := range tests {
		if got := MaxMipCount(tc.width, tc.height); got != tc.want {
			t.Errorf("MaxMipCount(%d, %d) = %d, want %d", tc.width, tc.height, got, tc.want)
		}
	}
}

func TestMipLevelDimensions(t *testing.T) {
	tests := []struct {
		level      int32
		wantWidth  int64
		wantHeight int64
	}{
		{level: 0, wantWidth: 1024, wantHeight: 4},
		{level: 1, wantWidth: 512, wantHeight: 2},
		{level: 2, wantWidth: 256, wantHeight: 1},
		{level: 10, wantWidth: 1, wantHeight: 1},
		{level: 63, wantWidth: 1, wantHeight: 1},
	}
	for _, tc := range tests {
		width, height := MipLevelDimensions(1024, 4, tc.level)
		if width != tc.wantWidth || height != tc.wantHeight {
			t.Errorf("MipLevelDimensions(1024, 4, %d) = %dx%d, want %dx%d", tc.level, width, height, tc.wantWidth, tc.wantHeight)
		}
	}
}

func TestMipChainSizeMatchesLevelSum(t *testing.T) {
	const width, height = int64(1024), int64(1024)
	levelCount := MaxMipCount(width, height)
	sum := int64(0)
	for level := int32(0); level < levelCount; level++ {
		sum += MipLevelSize(width, height, level)
	}
	if got := MipChainSize(width, height, levelCount); got != sum {
		t.Fatalf("MipChainSize = %d, want %d", got, sum)
	}
	// 1024x1024 RGBA32 的完整链，最后两级受 1 像素下限影响不再按 1/4 递减
	// The full RGBA32 chain of a 1024x1024 texture, whose last two levels stop shrinking by 1/4 once they reach one pixel
	if got, want := MipChainSize(width, height, levelCount), int64(5592404); got != want {
		t.Fatalf("full RGBA32 chain = %d, want %d", got, want)
	}
}

func TestGenerateMipChainLevelSizes(t *testing.T) {
	const width, height = int64(8), int64(8)
	source := make([]byte, width*height*4)
	for i := range source {
		source[i] = byte(i)
	}
	mips, err := GenerateMipChain(width, height, source, MaxMipCount(width, height))
	if err != nil {
		t.Fatalf("GenerateMipChain: %v", err)
	}
	if len(mips) != 4 {
		t.Fatalf("level count = %d, want 4", len(mips))
	}
	for level, want := range []int{8 * 8 * 4, 4 * 4 * 4, 2 * 2 * 4, 1 * 4} {
		if len(mips[level]) != want {
			t.Errorf("level %d size = %d, want %d", level, len(mips[level]), want)
		}
	}
	if !bytes.Equal(mips[0], source) {
		t.Fatal("level 0 is not a copy of the source")
	}
	source[0] = 0xff
	if mips[0][0] == 0xff {
		t.Fatal("level 0 aliases the source buffer")
	}
}

func TestGenerateMipChainAveragesBoxFilter(t *testing.T) {
	source := []byte{
		10, 20, 30, 40,
		50, 60, 70, 80,
		90, 100, 110, 120,
		130, 140, 150, 160,
	}
	mips, err := GenerateMipChain(2, 2, source, 2)
	if err != nil {
		t.Fatalf("GenerateMipChain: %v", err)
	}
	if want := []byte{70, 80, 90, 100}; !bytes.Equal(mips[1], want) {
		t.Fatalf("level 1 = %v, want %v", mips[1], want)
	}
}

func TestGenerateMipChainHalvesNonPowerOfTwo(t *testing.T) {
	const width, height = int64(5), int64(3)
	source := make([]byte, width*height*4)
	for i := range source {
		source[i] = 255
	}
	mips, err := GenerateMipChain(width, height, source, MaxMipCount(width, height))
	if err != nil {
		t.Fatalf("GenerateMipChain: %v", err)
	}
	if len(mips) != 3 {
		t.Fatalf("level count = %d, want 3", len(mips))
	}
	for level, mip := range mips {
		levelWidth, levelHeight := MipLevelDimensions(width, height, int32(level))
		if int64(len(mip)) != levelWidth*levelHeight*4 {
			t.Errorf("level %d size = %d, want %d", level, len(mip), levelWidth*levelHeight*4)
		}
		for _, value := range mip {
			if value != 255 {
				t.Fatalf("level %d averaged a uniform image to %d", level, value)
			}
		}
	}
}

func TestGenerateMipChainRejectsInvalidInput(t *testing.T) {
	valid := make([]byte, 4*4*4)
	tests := []struct {
		name       string
		width      int64
		height     int64
		rgba       []byte
		levelCount int32
	}{
		{name: "zero_width", width: 0, height: 4, rgba: valid, levelCount: 1},
		{name: "negative_height", width: 4, height: -4, rgba: valid, levelCount: 1},
		{name: "wrong_source_size", width: 4, height: 4, rgba: make([]byte, 16), levelCount: 1},
		{name: "zero_levels", width: 4, height: 4, rgba: valid, levelCount: 0},
		{name: "too_many_levels", width: 4, height: 4, rgba: valid, levelCount: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := GenerateMipChain(tc.width, tc.height, tc.rgba, tc.levelCount); err == nil {
				t.Fatal("GenerateMipChain unexpectedly succeeded")
			}
		})
	}
}

func TestFlipRGBA32MipChainFlipsEveryLevel(t *testing.T) {
	const width, height = int64(4), int64(2)
	mips, err := GenerateMipChain(width, height, make([]byte, width*height*4), 2)
	if err != nil {
		t.Fatalf("GenerateMipChain: %v", err)
	}
	for level, mip := range mips {
		levelWidth, levelHeight := MipLevelDimensions(width, height, int32(level))
		for i := range mip {
			mip[i] = byte(i)
		}
		flipped := FlipRGBA32MipChain(width, height, mips)[level]
		if got := FlipRGBA32Rows(levelWidth, levelHeight, flipped); !bytes.Equal(got, mip) {
			t.Fatalf("level %d flip is not its own inverse", level)
		}
	}
}

func TestEncodeTexture2DDataWithMipChain(t *testing.T) {
	tt := sampleTypeTree(t, "parts_bcc4_gp003_2.aba", ClassIDTexture2D)
	mips, err := GenerateMipChain(4, 4, make([]byte, 4*4*4), 3)
	if err != nil {
		t.Fatalf("GenerateMipChain: %v", err)
	}
	total := int64(0)
	for level, mip := range mips {
		total += int64(len(mip))
		mips[level] = bytes.Repeat([]byte{byte(level + 1)}, len(mip))
	}
	data, err := encodeTexture2DData("2022.3.35f", "mip_test.tex", 4, 4, mips)
	if err != nil {
		t.Fatalf("encodeTexture2DData: %v", err)
	}
	root := decodeObjectWithTypeTree(t, &tt, data)

	assertTypeTreeInt(t, root, "m_MipCount", 3)
	assertTypeTreeInt(t, root, "m_CompleteImageSize", total)
	assertTypeTreeInt(t, root, "m_TextureFormat", int64(TextureFormatRGBA32))
	assertTypeTreeInt(t, root, "m_Width", 4)
	assertTypeTreeInt(t, root, "m_Height", 4)

	imageData, ok := root.Field("image data").Bytes()
	if !ok {
		t.Fatal("image data missing")
	}
	want := make([]byte, 0, total)
	for _, mip := range mips {
		want = append(want, mip...)
	}
	if !bytes.Equal(imageData, want) {
		t.Fatalf("image data = %v, want the concatenated mip chain", imageData)
	}
}

func TestEncodeTexture2DDataRejectsBadMipChain(t *testing.T) {
	valid := make([]byte, 4*4*4)
	tests := []struct {
		name string
		mips [][]byte
	}{
		{name: "no_levels", mips: nil},
		{name: "too_many_levels", mips: [][]byte{valid, make([]byte, 2*2*4), make([]byte, 1*4), make([]byte, 1*4)}},
		{name: "wrong_level_size", mips: [][]byte{valid, make([]byte, 2*2*4-4)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := encodeTexture2DData("2022.3.35f", "bad.tex", 4, 4, tc.mips); err == nil {
				t.Fatal("encodeTexture2DData unexpectedly succeeded")
			}
		})
	}

	if _, err := encodeTexture2DData("2022.3.35f", "single.tex", 4, 4, [][]byte{valid}); err != nil {
		t.Fatalf("single level chain should be accepted, got %v", err)
	}
}

func TestNativeTexture2DMipCountRoundTrip(t *testing.T) {
	rgba := make([]byte, 4*4*4)
	mips, err := GenerateMipChain(4, 4, rgba, 3)
	if err != nil {
		t.Fatalf("GenerateMipChain: %v", err)
	}
	object, err := NewNativeTexture2DObjectWithMips("mip.tex", 4, 4, mips)
	if err != nil {
		t.Fatalf("NewNativeTexture2DObjectWithMips: %v", err)
	}
	data, err := WriteTexture2D(object)
	if err != nil {
		t.Fatalf("WriteTexture2D: %v", err)
	}
	got, err := NativeTexture2DMipCount(data)
	if err != nil {
		t.Fatalf("NativeTexture2DMipCount: %v", err)
	}
	if got != 3 {
		t.Fatalf("mip count = %d, want 3", got)
	}

	single, err := NewNativeTexture2DObject("single.tex", 4, 4, rgba)
	if err != nil {
		t.Fatalf("NewNativeTexture2DObject: %v", err)
	}
	singleData, err := WriteTexture2D(single)
	if err != nil {
		t.Fatalf("WriteTexture2D: %v", err)
	}
	if got, err := NativeTexture2DMipCount(singleData); err != nil || got != 1 {
		t.Fatalf("single mip count = %d err=%v, want 1", got, err)
	}
}

func TestNativeTexture2DMipCountRejectsGarbage(t *testing.T) {
	if _, err := NativeTexture2DMipCount([]byte("not a native Unity object")); err == nil {
		t.Fatal("NativeTexture2DMipCount accepted garbage input")
	}
}
