package KCES

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
)

// NativeUnityMediaService 提供独立 Unity 媒体对象到通用查看格式的转换 / NativeUnityMediaService converts standalone Unity media objects to common viewing formats
type NativeUnityMediaService struct{}

// Texture2DMipMode 决定 convert2texture2d 重建原生 Texture2D 时写入多少 mip 层 / Texture2DMipMode decides how many mip levels convert2texture2d writes when rebuilding a native Texture2D
type Texture2DMipMode int

const (
	// Texture2DMipModeAuto 探测目标原生 Texture2D 声明的 mip 层数并沿用，探测不到时写单层 / Texture2DMipModeAuto probes the mip level count declared by the target native Texture2D and reuses it, writing a single level when nothing can be probed
	Texture2DMipModeAuto Texture2DMipMode = iota
	// Texture2DMipModeNone 强制写单层 / Texture2DMipModeNone forces a single level
	Texture2DMipModeNone
	// Texture2DMipModeFull 强制生成直到 1×1 的完整 mip 链 / Texture2DMipModeFull forces the full mip chain down to 1×1
	Texture2DMipModeFull
)

// Texture2DRebuildOptions 控制从图像重建原生 Texture2D 的方式 / Texture2DRebuildOptions controls how a native Texture2D is rebuilt from an image
type Texture2DRebuildOptions struct {
	MipMode  Texture2DMipMode // mip 层数策略 / Mip level strategy
	Compress bool             // 是否压成 DXT1 或 DXT5 / Whether to compress into DXT1 or DXT5
}

// IsKCESNativeTexture2DFile 判断文件是否为带内嵌 TypeTree 的 Texture2D 主文件
// IsKCESNativeTexture2DFile reports whether a file is a primary Texture2D file with an embedded TypeTree
func IsKCESNativeTexture2DFile(path string) bool {
	return isKCESNativeUnityClassFile(path, aba.ClassIDTexture2D)
}

// IsKCESNativeMeshFile 判断文件是否为带内嵌 TypeTree 的 Mesh 主文件
// IsKCESNativeMeshFile reports whether a file is a Mesh primary file with an embedded TypeTree
func IsKCESNativeMeshFile(path string) bool {
	return isKCESNativeUnityClassFile(path, aba.ClassIDMesh)
}

// IsKCESNativeSpriteFile 判断文件是否为带内嵌 TypeTree 的 Sprite 主文件
// IsKCESNativeSpriteFile reports whether a file is a Sprite primary file with an embedded TypeTree
func IsKCESNativeSpriteFile(path string) bool {
	return isKCESNativeUnityClassFile(path, aba.ClassIDSprite)
}

// IsKCESNativeAnimationClipFile 判断文件是否为带内嵌 TypeTree 的 AnimationClip 主文件
// IsKCESNativeAnimationClipFile reports whether a file is an AnimationClip primary file with an embedded TypeTree
func IsKCESNativeAnimationClipFile(path string) bool {
	return isKCESNativeUnityClassFile(path, aba.ClassIDAnimationClip)
}

// IsKCESNativeAudioClipFile 判断文件是否为带内嵌 TypeTree 的 AudioClip 主文件
// IsKCESNativeAudioClipFile reports whether a file is an AudioClip primary file with an embedded TypeTree
func IsKCESNativeAudioClipFile(path string) bool {
	return isKCESNativeUnityClassFile(path, aba.ClassIDAudioClip)
}

// ConvertTexture2DToImage 将独立 Texture2D 主文件导出为 PNG 或 DDS，不修改主文件
// ConvertTexture2DToImage exports a standalone Texture2D primary file as PNG or DDS without modifying the primary file
func (s *NativeUnityMediaService) ConvertTexture2DToImage(ctx context.Context, inputPath string, outputPath string, format string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	object, err := aba.ReadTexture2D(data)
	if err != nil {
		return fmt.Errorf("read native Texture2D %q: %w", inputPath, err)
	}
	af, info, err := object.AssetsFileView()
	if err != nil {
		return err
	}
	texture, err := af.GetTexture2DDataRange(info, nil)
	if err != nil {
		return fmt.Errorf("decode native Texture2D %q: %w", inputPath, err)
	}
	format = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(outputPath)), ".")
	}
	var output []byte
	switch format {
	case "png":
		output, err = aba.TexturePNGBytes(texture)
	case "dds":
		output, err = aba.TextureDDSBytes(texture)
	default:
		return fmt.Errorf("native Texture2D output format %q is unsupported; use png or dds", format)
	}
	if err != nil {
		return fmt.Errorf("encode native Texture2D as %s: %w", format, err)
	}
	return writeNativeUnityMediaOutput(ctx, outputPath, output, maxOutputBytes)
}

// ConvertImageToTexture2D 将 PNG 或 JPEG 图像重建为内联 RGBA32 的独立 Texture2D 主文件，mip 层数默认探测并沿用目标原有设置，资源名按输出文件名推断并与纯目录打包规则一致
// ConvertImageToTexture2D rebuilds a PNG or JPEG image into a standalone Texture2D primary file with inline RGBA32 data, probing and keeping the mip level count of the existing target by default, and the resource name inferred from the output file name matches the pure-directory packing rules
func (s *NativeUnityMediaService) ConvertImageToTexture2D(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	return s.ConvertImageToTexture2DWithOptions(ctx, inputPath, outputPath, Texture2DRebuildOptions{MipMode: Texture2DMipModeAuto}, maxOutputBytes)
}

// ConvertImageToTexture2DWithOptions 将 PNG 或 JPEG 图像重建为独立 Texture2D 主文件，并按选项决定 mip 层数与是否压缩
// 无论是否压缩，像素都先解码成 RGBA32，再按同一套规则生成 mip 链，因此两条路径的 mip 内容完全一致
// 原始块压缩格式无法保留，BC7 也无法写出，压缩时只能落到 DXT1 或 DXT5
// ConvertImageToTexture2DWithOptions rebuilds a PNG or JPEG image into a standalone Texture2D primary file and picks the mip level count and whether to compress from the options
// Pixels are always decoded to RGBA32 first and the mip chain is generated by the same rules either way, so both paths carry identical mips
// The original block-compressed format cannot be kept and BC7 cannot be written at all, so compression can only land on DXT1 or DXT5
func (s *NativeUnityMediaService) ConvertImageToTexture2DWithOptions(ctx context.Context, inputPath string, outputPath string, options Texture2DRebuildOptions, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	width, height, rgba, err := decodeImageToRGBA32(data)
	if err != nil {
		return fmt.Errorf("decode image %q: %w", inputPath, err)
	}
	levelCount, err := resolveTexture2DMipLevelCount(options.MipMode, outputPath, width, height)
	if err != nil {
		return err
	}
	mips, err := aba.GenerateMipChain(width, height, rgba, levelCount)
	if err != nil {
		return fmt.Errorf("generate mip chain for %q: %w", inputPath, err)
	}
	format := int32(aba.TextureFormatRGBA32)
	if options.Compress {
		dxt5 := hasTranslucentPixel(rgba)
		mips, err = aba.CompressRGBA32MipChainToDXT(mips, width, height, dxt5)
		if err != nil {
			return fmt.Errorf("compress %q: %w", inputPath, err)
		}
		if dxt5 {
			format = aba.TextureFormatDXT5
		} else {
			format = aba.TextureFormatDXT1
		}
	}
	name := inferAssetNameForPack(filepath.Base(outputPath))
	object, err := aba.NewNativeTexture2DObjectWithFormat(name, width, height, format, mips)
	if err != nil {
		return fmt.Errorf("build native Texture2D %q: %w", name, err)
	}
	output, err := aba.WriteTexture2D(object)
	if err != nil {
		return fmt.Errorf("encode native Texture2D %q: %w", name, err)
	}
	return writeNativeUnityMediaOutput(ctx, outputPath, output, maxOutputBytes)
}

// hasTranslucentPixel 判断 RGBA32 像素里是否存在非完全不透明的像素
// 存在时 DXT1 的一位 Alpha 会丢掉半透明，因此压缩应改用 DXT5
// hasTranslucentPixel reports whether any RGBA32 pixel is not fully opaque
// When one exists, the single-bit alpha of DXT1 would drop the partial transparency, so compression should use DXT5 instead
func hasTranslucentPixel(rgba []byte) bool {
	for i := 3; i < len(rgba); i += 4 {
		if rgba[i] != 0xFF {
			return true
		}
	}
	return false
}

// resolveTexture2DMipLevelCount 按模式算出要写入的 mip 层数，auto 模式探测即将被覆盖的那个原生 Texture2D 声明的层数
// 探测不到时一律按单层处理，包括目标不存在、目标读不出来和目标解析失败三种情况
// 探测到的层数可能超过新图像尺寸允许的上限，此时收敛到上限
// resolveTexture2DMipLevelCount derives the mip level count to write for the requested mode, and the auto mode probes the count declared by the native Texture2D that is about to be overwritten
// Anything that cannot be probed falls back to a single level, covering a missing target, an unreadable one, and one that fails to parse
// The probed level count can exceed what the new image size allows and is clamped to that limit
func resolveTexture2DMipLevelCount(mipMode Texture2DMipMode, targetPath string, width int64, height int64) (int32, error) {
	maxLevels := aba.MaxMipCount(width, height)
	if maxLevels < 1 {
		return 0, fmt.Errorf("invalid Texture2D dimensions %dx%d", width, height)
	}
	switch mipMode {
	case Texture2DMipModeNone:
		return 1, nil
	case Texture2DMipModeFull:
		return maxLevels, nil
	case Texture2DMipModeAuto:
		existing, err := os.ReadFile(targetPath)
		if err != nil {
			return 1, nil
		}
		declared, err := aba.NativeTexture2DMipCount(existing)
		if err != nil {
			return 1, nil
		}
		if declared > maxLevels {
			return maxLevels, nil
		}
		return declared, nil
	default:
		return 0, fmt.Errorf("unsupported Texture2D mip mode %d", int(mipMode))
	}
}

// ExtractAudioClip 将独立 AudioClip 中已内联的原始音频载荷写出，不进行有损转码
// ExtractAudioClip writes the inline encoded payload from a standalone AudioClip without lossy transcoding
func (s *NativeUnityMediaService) ExtractAudioClip(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	object, err := aba.ReadAudioClip(data)
	if err != nil {
		return fmt.Errorf("read native AudioClip %q: %w", inputPath, err)
	}
	audioData, err := object.AudioData()
	if err != nil {
		return fmt.Errorf("extract native AudioClip %q: %w", inputPath, err)
	}
	return writeNativeUnityMediaOutput(ctx, outputPath, audioData, maxOutputBytes)
}

// DetectAudioClipExtension 读取独立 AudioClip 的内联载荷并返回建议扩展名
// DetectAudioClipExtension reads a standalone AudioClip's inline payload and returns its suggested extension
func (s *NativeUnityMediaService) DetectAudioClipExtension(ctx context.Context, inputPath string) (string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", inputPath, err)
	}
	object, err := aba.ReadAudioClip(data)
	if err != nil {
		return "", fmt.Errorf("read native AudioClip %q: %w", inputPath, err)
	}
	audioData, err := object.AudioData()
	if err != nil {
		return "", fmt.Errorf("inspect native AudioClip %q: %w", inputPath, err)
	}
	return NativeAudioClipExtension(audioData), nil
}

// writeNativeUnityMediaOutput 在上下文有效且大小不超限时写入媒体转换结果
// writeNativeUnityMediaOutput writes media conversion output while the context is active and the size remains within the limit
func writeNativeUnityMediaOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive native Unity media output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: native Unity media output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write native Unity media output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}

// NativeAudioClipExtension 根据内联载荷签名返回无需转码即可使用的建议扩展名
// NativeAudioClipExtension returns a suggested extension usable without transcoding based on the inline payload signature
func NativeAudioClipExtension(data []byte) string {
	switch {
	case len(data) >= 4 && string(data[:4]) == "OggS":
		return ".ogg"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE":
		return ".wav"
	case len(data) >= 4 && string(data[:4]) == "FSB5":
		return ".fsb"
	default:
		return ".audio"
	}
}

// isKCESNativeUnityClassFile 判断文件头是否声明指定 Unity ClassID
// isKCESNativeUnityClassFile reports whether a file header declares the requested Unity ClassID
func isKCESNativeUnityClassFile(path string, classID int32) bool {
	header, err := readNativeUnityObjectFileHeader(path)
	return err == nil && header.ClassID == classID
}

// readNativeUnityObjectFileHeader 只读取独立 Unity 对象的头和 TypeTree
// readNativeUnityObjectFileHeader reads only the header and TypeTree of a standalone Unity object
func readNativeUnityObjectFileHeader(path string) (*aba.NativeUnityObjectHeader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", path)
	}
	return aba.ReadNativeUnityObjectHeader(file, info.Size())
}
