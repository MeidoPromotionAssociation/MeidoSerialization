package aba

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/tools"
)

// dxtTestSplitImage 生成上半部分纯红、下半部分纯蓝的 RGBA32 图像
// 这种图像垂直方向不对称，而 DXT1 与 DXT5 都能精确表示纯色块，因此特别适合验证方向处理
// dxtTestSplitImage builds a top-down RGBA32 image whose top half is solid red and bottom half solid blue
// The image is vertically asymmetric while DXT1 and DXT5 both represent solid color blocks exactly, which makes it ideal for checking orientation handling
func dxtTestSplitImage(width int64, height int64) []byte {
	pixels := make([]byte, width*height*4)
	for y := int64(0); y < height; y++ {
		for x := int64(0); x < width; x++ {
			offset := (y*width + x) * 4
			if y < height/2 {
				pixels[offset] = 0xFF
			} else {
				pixels[offset+2] = 0xFF
			}
			pixels[offset+3] = 0xFF
		}
	}
	return pixels
}

func TestCompressRGBA32MipChainToDXTLevelSizes(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required to compress into DXT: %v", err)
	}
	const width, height = int64(16), int64(16)
	mips, err := GenerateMipChain(width, height, dxtTestSplitImage(width, height), MaxMipCount(width, height))
	if err != nil {
		t.Fatalf("GenerateMipChain: %v", err)
	}

	for _, tc := range []struct {
		name   string
		dxt5   bool
		format int32
	}{
		{name: "dxt1", dxt5: false, format: TextureFormatDXT1},
		{name: "dxt5", dxt5: true, format: TextureFormatDXT5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compressed, err := CompressRGBA32MipChainToDXT(mips, width, height, tc.dxt5)
			if err != nil {
				t.Fatalf("CompressRGBA32MipChainToDXT: %v", err)
			}
			if len(compressed) != len(mips) {
				t.Fatalf("level count = %d, want %d", len(compressed), len(mips))
			}
			for level, data := range compressed {
				want := TextureLevelSize(tc.format, width, height, int32(level))
				if int64(len(data)) != want {
					t.Errorf("level %d = %d bytes, want %d", level, len(data), want)
				}
			}
		})
	}
}

func TestCompressRGBA32MipChainToDXTRejectsBadInput(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required to compress into DXT: %v", err)
	}
	if _, err := CompressRGBA32MipChainToDXT(nil, 4, 4, false); err == nil {
		t.Fatal("an empty mip chain was accepted")
	}
	if _, err := CompressRGBA32MipChainToDXT([][]byte{make([]byte, 15)}, 4, 4, false); err == nil {
		t.Fatal("a level with the wrong byte count was accepted")
	}
}

// TestCompressedTexture2DRoundTripKeepsOrientation 固定压缩路径的方向契约
// 导入时先按正立顺序压缩再翻成 Unity 行序，导出时反过来，因此重新导出的图像必须与输入同向
// 若任何一侧漏了翻转，导出的图像会上下颠倒，用上下半部分不同颜色的图像就能立刻发现
// TestCompressedTexture2DRoundTripKeepsOrientation pins the orientation contract of the compressed path
// Import compresses in upright order and then flips into the Unity row order while export reverses that, so a re-exported image must match the input orientation
// A missing flip on either side would turn the exported image upside down, which an image with differently colored halves exposes immediately
func TestCompressedTexture2DRoundTripKeepsOrientation(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required to compress into DXT: %v", err)
	}
	const width, height = int64(16), int64(16)
	mips, err := GenerateMipChain(width, height, dxtTestSplitImage(width, height), 1)
	if err != nil {
		t.Fatalf("GenerateMipChain: %v", err)
	}
	compressed, err := CompressRGBA32MipChainToDXT(mips, width, height, false)
	if err != nil {
		t.Fatalf("CompressRGBA32MipChainToDXT: %v", err)
	}

	object, err := NewNativeTexture2DObjectWithFormat("split.tex", width, height, TextureFormatDXT1, compressed)
	if err != nil {
		t.Fatalf("NewNativeTexture2DObjectWithFormat: %v", err)
	}
	encoded, err := WriteTexture2D(object)
	if err != nil {
		t.Fatalf("WriteTexture2D: %v", err)
	}

	readBack, err := ReadTexture2D(encoded)
	if err != nil {
		t.Fatalf("ReadTexture2D: %v", err)
	}
	af, info, err := readBack.AssetsFileView()
	if err != nil {
		t.Fatal(err)
	}
	tex, err := af.GetTexture2DDataRange(info, nil)
	if err != nil {
		t.Fatalf("GetTexture2DDataRange: %v", err)
	}
	if tex.TextureFormat != TextureFormatDXT1 {
		t.Fatalf("texture format = %d, want DXT1", tex.TextureFormat)
	}
	if tex.MipCount != 1 {
		t.Fatalf("mip count = %d, want 1", tex.MipCount)
	}
	if want := TextureLevelSize(TextureFormatDXT1, width, height, 0); int64(len(tex.ImageData)) != want {
		t.Fatalf("payload = %d bytes, want %d", len(tex.ImageData), want)
	}

	var exported bytes.Buffer
	if err := WriteTexturePNGTo(tex, &exported); err != nil {
		t.Fatalf("WriteTexturePNGTo: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(exported.Bytes()))
	if err != nil {
		t.Fatalf("decode exported PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != int(width) || bounds.Dy() != int(height) {
		t.Fatalf("exported size = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
	}

	topRed, _, topBlue, _ := img.At(bounds.Min.X, bounds.Min.Y).RGBA()
	bottomRed, _, bottomBlue, _ := img.At(bounds.Min.X, bounds.Max.Y-1).RGBA()
	if topRed <= topBlue {
		t.Fatalf("top row is not red: r=%d b=%d, the compressed payload is upside down", topRed, topBlue)
	}
	if bottomBlue <= bottomRed {
		t.Fatalf("bottom row is not blue: r=%d b=%d, the compressed payload is upside down", bottomRed, bottomBlue)
	}
}
