package aba

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/tools"
)

// 本文件把 RGBA32 像素压成 DXT1 或 DXT5 块
// 块编码本身交给 ImageMagick，因为 Go 生态没有可用的 DXT 编码库，而 ImageMagick 的 dds:compression 能直接产出这两种格式
// 本库只负责决定格式、逐级编码并剥掉 DDS 头，mip 链仍由 GenerateMipChain 生成，因此压缩与非压缩两条路径的 mip 完全一致
// This file compresses RGBA32 pixels into DXT1 or DXT5 blocks
// The block encoding itself is delegated to ImageMagick because the Go ecosystem has no usable DXT encoder while the ImageMagick dds:compression option produces both formats directly
// This library only picks the format, encodes level by level, and strips the DDS header, while the mip chain still comes from GenerateMipChain so the compressed and uncompressed paths share identical mips

// ddsHeaderSize 是 ImageMagick 写出的 DDS 文件头长度
// ddsHeaderSize is the length of the DDS header ImageMagick writes
const ddsHeaderSize = 128

// CompressRGBA32MipChainToDXT 把自上而下行序的 RGBA32 mip 链逐级压成 DXT1 或 DXT5 块
// 返回的各级仍与输入一一对应且保持自上而下行序，调用方还需要按 Unity 行序再翻转一次
// dxt5 为 false 时压成 DXT1，此时透明只保留一位 Alpha，因此调用方应先确认图像没有半透明像素
// CompressRGBA32MipChainToDXT compresses a top-down RGBA32 mip chain into DXT1 or DXT5 blocks level by level
// The returned levels match the input one for one and stay in top-down row order, so callers still have to flip them into the Unity row order
// A false dxt5 compresses to DXT1, which keeps only one bit of alpha, so callers should confirm the image has no partial transparency first
func CompressRGBA32MipChainToDXT(mips [][]byte, width int64, height int64, dxt5 bool) ([][]byte, error) {
	if len(mips) == 0 {
		return nil, fmt.Errorf("no mip levels to compress")
	}
	if err := tools.CheckMagick(); err != nil {
		return nil, err
	}
	compression := "dxt1"
	if dxt5 {
		compression = "dxt5"
	}

	compressed := make([][]byte, len(mips))
	for level, data := range mips {
		levelWidth, levelHeight := MipLevelDimensions(width, height, int32(level))
		if int64(len(data)) != levelWidth*levelHeight*4 {
			return nil, fmt.Errorf("mip level %d has %d bytes but %dx%d RGBA32 needs %d", level, len(data), levelWidth, levelHeight, levelWidth*levelHeight*4)
		}
		blocks, err := compressRGBA32ToDXTBlocks(data, levelWidth, levelHeight, compression)
		if err != nil {
			return nil, fmt.Errorf("compress mip level %d: %w", level, err)
		}
		compressed[level] = blocks
	}
	return compressed, nil
}

// compressRGBA32ToDXTBlocks 用 ImageMagick 把单级 RGBA32 像素压成 DXT 块并剥掉 DDS 头
// 单级编码而不是让 ImageMagick 生成 mip，是为了让压缩路径和非压缩路径共用同一条由本库生成的 mip 链
// compressRGBA32ToDXTBlocks compresses one RGBA32 level into DXT blocks through ImageMagick and strips the DDS header
// Encoding one level at a time rather than letting ImageMagick generate mips keeps the compressed path on the same library-generated mip chain as the uncompressed path
func compressRGBA32ToDXTBlocks(rgba []byte, width int64, height int64, compression string) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid level dimensions %dx%d", width, height)
	}
	cmd := exec.Command(
		"magick",
		"-size", fmt.Sprintf("%dx%d", width, height),
		"-depth", "8",
		"rgba:-",
		"-define", "dds:compression="+compression,
		// ImageMagick 的 dds:mipmaps 是「额外层数」，不写会按 2 的幂尺寸自动补出整条链
		// 写 0 才能只得到当前这一级，否则返回的载荷长度与本库按级累加的预期完全不符
		// The ImageMagick dds:mipmaps option counts extra levels, and leaving it out makes power-of-two sizes grow a whole chain
		// Only zero yields this single level, without which the returned payload length would not match the per-level sizes this library expects
		"-define", "dds:mipmaps=0",
		"dds:-",
	)
	tools.SetHideWindow(cmd)
	cmd.Stdin = bytes.NewReader(rgba)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("magick %s compression failed: %w, stderr: %s", compression, err, stderr.String())
	}
	dds := stdout.Bytes()
	if len(dds) <= ddsHeaderSize || string(dds[:4]) != "DDS " {
		return nil, fmt.Errorf("magick returned %d bytes without a DDS header", len(dds))
	}
	return append([]byte(nil), dds[ddsHeaderSize:]...), nil
}
