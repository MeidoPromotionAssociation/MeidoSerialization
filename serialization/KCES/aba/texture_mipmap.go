package aba

import (
	"fmt"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/tools"
)

// 本文件生成 RGBA32 mip 链
// 生成顺序与 Unity 的 Texture2D image data 布局一致，即全分辨率在前、逐级折半、1×1 收尾
// This file generates RGBA32 mip chains
// The order matches the Unity Texture2D image data layout, which starts at full resolution, halves level by level, and ends at 1×1

// MaxMipCount 返回指定尺寸贴图可用的最大 mip 层数，即从全分辨率逐级折半直到 1×1 的层数
// MaxMipCount returns the maximum mip level count for a texture of the given size, counting down from full resolution to 1×1
func MaxMipCount(width int64, height int64) int32 {
	if width <= 0 || height <= 0 {
		return 0
	}
	largest := width
	if height > largest {
		largest = height
	}
	count := int32(1)
	for largest > 1 {
		largest >>= 1
		count++
	}
	return count
}

// MipLevelDimensions 返回指定 mip 级的像素尺寸，每级折半且不小于 1
// MipLevelDimensions returns the pixel dimensions of the given mip level, halving each step and never going below 1
func MipLevelDimensions(width int64, height int64, level int32) (int64, int64) {
	if level <= 0 {
		return width, height
	}
	// 位移量达到 63 时结果必然为 1，提前返回可避免 64 位以上的未定义移位
	// A shift of 63 already forces the result to 1, so returning early avoids undefined shifts beyond 64 bits
	if level >= 63 {
		return 1, 1
	}
	shift := uint(level)
	levelWidth := width >> shift
	levelHeight := height >> shift
	if levelWidth < 1 {
		levelWidth = 1
	}
	if levelHeight < 1 {
		levelHeight = 1
	}
	return levelWidth, levelHeight
}

// TextureFormatBlockBytes 返回块压缩纹理格式每块的字节数，非块压缩格式返回 0
// TextureFormatBlockBytes returns the byte size of one block for a block-compressed texture format, or 0 for a format that is not block compressed
func TextureFormatBlockBytes(format int32) int64 {
	switch format {
	case TextureFormatDXT1:
		return 8
	case TextureFormatDXT5:
		return 16
	default:
		return 0
	}
}

// TextureLevelSize 返回指定纹理格式在指定 mip 级的字节数
// 块压缩格式按 4×4 块计算且块数向上取整，因此边长不足 4 的级仍占一整块
// TextureLevelSize returns the byte size of the given texture format at the given mip level
// Block-compressed formats count 4×4 blocks with the grid rounded up, so a level shorter than four texels still occupies a full block
func TextureLevelSize(format int32, width int64, height int64, level int32) int64 {
	levelWidth, levelHeight := MipLevelDimensions(width, height, level)
	if blockBytes := TextureFormatBlockBytes(format); blockBytes > 0 {
		return ((levelWidth + 3) / 4) * ((levelHeight + 3) / 4) * blockBytes
	}
	return levelWidth * levelHeight * 4
}

// TextureChainSize 返回指定纹理格式整条 mip 链的字节总数，即各级字节数之和
// TextureChainSize returns the total byte count of a whole mip chain for the given texture format, that is the sum of every level
func TextureChainSize(format int32, width int64, height int64, levelCount int32) int64 {
	total := int64(0)
	for level := int32(0); level < levelCount; level++ {
		total += TextureLevelSize(format, width, height, level)
	}
	return total
}

// GenerateMipChain 以盒式滤波逐级折半，返回自上而下行序的 RGBA32 mip 链，第 0 级是原图副本，层数由 levelCount 指定
// levelCount 必须落在 1 到 MaxMipCount 之间，传 MaxMipCount 即得到直到 1×1 的完整链
// 盒式滤波直接对存储值求平均，与 Unity 在 Linear 色彩空间贴图上生成 mip 的做法一致
// GenerateMipChain halves the image level by level with a box filter and returns an RGBA32 mip chain in top-down row order, with level 0 copied from the source and the depth given by levelCount
// levelCount must fall between 1 and MaxMipCount, and passing MaxMipCount yields the full chain down to 1×1
// The box filter averages stored values directly, matching how Unity generates mips for Linear color space textures
func GenerateMipChain(width int64, height int64, rgba []byte, levelCount int32) ([][]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid mip chain dimensions %dx%d", width, height)
	}
	if width > maxMipDimension || height > maxMipDimension {
		return nil, fmt.Errorf("mip chain dimensions %dx%d exceed the supported range", width, height)
	}
	if int64(len(rgba)) != width*height*4 {
		return nil, fmt.Errorf("mip chain source size %d does not match %dx%d RGBA32 (%d bytes)", len(rgba), width, height, width*height*4)
	}
	if maxLevels := MaxMipCount(width, height); levelCount < 1 || levelCount > maxLevels {
		return nil, fmt.Errorf("mip level count %d is outside 1..%d for %dx%d", levelCount, maxLevels, width, height)
	}

	levels := make([][]byte, 0, levelCount)
	levels = append(levels, append([]byte(nil), rgba...))
	for level := int32(1); level < levelCount; level++ {
		previous := levels[level-1]
		previousWidth, previousHeight := MipLevelDimensions(width, height, level-1)
		levelWidth, levelHeight := MipLevelDimensions(width, height, level)
		levels = append(levels, downsampleBoxRGBA32(previousWidth, previousHeight, previous, levelWidth, levelHeight))
	}
	return levels, nil
}

// FlipTextureMipChain 对 mip 链的每一级分别垂直翻转，用于在自上而下行序与 Unity 自下而上行序之间转换
// RGBA32 逐行翻转，DXT1 与 DXT5 逐块翻转，块压缩格式的翻转同样无损
// FlipTextureMipChain vertically flips every mip level on its own, converting between top-down row order and Unity's bottom-up row order
// RGBA32 levels are flipped row by row while DXT1 and DXT5 levels are flipped block by block, which stays lossless for block-compressed formats too
func FlipTextureMipChain(format int32, width int64, height int64, mips [][]byte) ([][]byte, error) {
	blockBytes := TextureFormatBlockBytes(format)
	if blockBytes == 0 && format != TextureFormatRGBA32 {
		return nil, fmt.Errorf("unsupported texture format %d for mip chain flipping", format)
	}
	flipped := make([][]byte, len(mips))
	for level, data := range mips {
		levelWidth, levelHeight := MipLevelDimensions(width, height, int32(level))
		if blockBytes == 0 {
			flipped[level] = FlipRGBA32Rows(levelWidth, levelHeight, data)
			continue
		}
		if levelWidth > math.MaxInt32 || levelHeight > math.MaxInt32 {
			return nil, fmt.Errorf("mip level %d dimensions %dx%d exceed Int32 range", level, levelWidth, levelHeight)
		}
		levelFlipped, err := tools.FlipDXTBlocksVertically(data, int32(levelWidth), int32(levelHeight), blockBytes)
		if err != nil {
			return nil, fmt.Errorf("flip mip level %d: %w", level, err)
		}
		flipped[level] = levelFlipped
	}
	return flipped, nil
}

// maxMipDimension 限制单边尺寸，使盒式滤波的中间累加与目标缓冲分配都不会溢出
// maxMipDimension caps a single dimension so that neither the box filter accumulator nor the destination allocation can overflow
const maxMipDimension int64 = 1 << 20

// downsampleBoxRGBA32 按面积平均把 RGBA32 图像缩放到目标尺寸，源区域按目标像素等分，因此折半与非整数比例都能正确覆盖
// downsampleBoxRGBA32 area-averages an RGBA32 image down to the target size, splitting the source region evenly per destination pixel so both halving and non-integer ratios cover the whole source
func downsampleBoxRGBA32(sourceWidth int64, sourceHeight int64, source []byte, targetWidth int64, targetHeight int64) []byte {
	target := make([]byte, targetWidth*targetHeight*4)
	for targetY := int64(0); targetY < targetHeight; targetY++ {
		sourceYStart := targetY * sourceHeight / targetHeight
		sourceYEnd := (targetY + 1) * sourceHeight / targetHeight
		if sourceYEnd <= sourceYStart {
			sourceYEnd = sourceYStart + 1
		}
		for targetX := int64(0); targetX < targetWidth; targetX++ {
			sourceXStart := targetX * sourceWidth / targetWidth
			sourceXEnd := (targetX + 1) * sourceWidth / targetWidth
			if sourceXEnd <= sourceXStart {
				sourceXEnd = sourceXStart + 1
			}
			var red, green, blue, alpha, samples uint64
			for sourceY := sourceYStart; sourceY < sourceYEnd; sourceY++ {
				rowStart := sourceY * sourceWidth * 4
				for sourceX := sourceXStart; sourceX < sourceXEnd; sourceX++ {
					offset := rowStart + sourceX*4
					red += uint64(source[offset])
					green += uint64(source[offset+1])
					blue += uint64(source[offset+2])
					alpha += uint64(source[offset+3])
					samples++
				}
			}
			targetOffset := (targetY*targetWidth + targetX) * 4
			half := samples / 2
			target[targetOffset] = uint8((red + half) / samples)
			target[targetOffset+1] = uint8((green + half) / samples)
			target[targetOffset+2] = uint8((blue + half) / samples)
			target[targetOffset+3] = uint8((alpha + half) / samples)
		}
	}
	return target
}
