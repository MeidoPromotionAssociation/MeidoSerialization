package KCES

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// NativeUnityMediaService 提供独立 Unity 媒体对象到通用查看格式的转换 / NativeUnityMediaService converts standalone Unity media objects to common viewing formats
type NativeUnityMediaService struct{}

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
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	data, err := readConversionFile(ctx, inputPath)
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
	if err := writeConversionFile(ctx, outputPath, output, maxOutputBytes); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

// ExtractAudioClip 将独立 AudioClip 中已内联的原始音频载荷写出，不进行有损转码
// ExtractAudioClip writes the inline encoded payload from a standalone AudioClip without lossy transcoding
func (s *NativeUnityMediaService) ExtractAudioClip(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	data, err := readConversionFile(ctx, inputPath)
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
	if err := writeConversionFile(ctx, outputPath, audioData, maxOutputBytes); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

// DetectAudioClipExtension 读取独立 AudioClip 的内联载荷并返回建议扩展名
// DetectAudioClipExtension reads a standalone AudioClip's inline payload and returns its suggested extension
func (s *NativeUnityMediaService) DetectAudioClipExtension(ctx context.Context, inputPath string) (string, error) {
	data, err := readConversionFile(ctx, inputPath)
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
