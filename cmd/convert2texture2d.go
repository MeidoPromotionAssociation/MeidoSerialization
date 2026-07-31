package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
	"github.com/spf13/cobra"
)

var convert2texture2dCmd = &cobra.Command{
	Use:   "convert2texture2d [file/directory]",
	Short: "Convert image files to native KCES Texture2D",
	Long: `Convert PNG or JPEG images back to native KCES Texture2D primary files.
The texture is rebuilt from scratch as inline single-mip RGBA32 data.
When a native Texture2D file with the same base name (.tex or .texture2d) exists
in the same directory, it is overwritten so packAba picks up the change.
This command can process a single file or all matching images in a directory.

Typical texture editing workflow:
  unpackAba mod.aba -> convert2image Texture2D\foo.tex -> edit foo.png ->
  convert2texture2d foo.png -> packAba

Examples:
  MeidoSerialization convert2texture2d example.png
  MeidoSerialization convert2texture2d ./mod_unpacked/Texture2D`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		processor := func(filePath string) error {
			return convertImageToNativeTexture2D(filePath)
		}
		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectoryConcurrent(path, processor, func(candidate string) bool {
				return fileTypeFilter(candidate) && isRGBA32DecodableImageFile(candidate)
			})
		}
		return processFile(path, processor)
	},
}

// convertImageToNativeTexture2D 将 PNG 或 JPEG 图像重建为 KCES 原生 Texture2D 独立对象文件
// convertImageToNativeTexture2D rebuilds a PNG or JPEG image into a native KCES Texture2D standalone object file
func convertImageToNativeTexture2D(path string) error {
	if !isRGBA32DecodableImageFile(path) {
		return fmt.Errorf("not a PNG or JPEG image file: %s", path)
	}
	outputPath := nativeTexture2DOutputPath(path)
	service := &KCESService.NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2D(context.Background(), path, outputPath, application.DefaultMaxOutputBytes); err != nil {
		return fmt.Errorf("failed to convert %s to native Texture2D: %w", path, err)
	}
	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// nativeTexture2DOutputPath 优先覆盖同目录同基名的现有原生 Texture2D 文件以便重新打包生效，否则输出同基名 .tex
// nativeTexture2DOutputPath prefers overwriting an existing native Texture2D file with the same base name so repacking picks up the change, and otherwise emits a .tex file with the same base name
func nativeTexture2DOutputPath(imagePath string) string {
	base := strings.TrimSuffix(imagePath, filepath.Ext(imagePath))
	for _, extension := range []string{".tex", ".texture2d"} {
		candidate := base + extension
		if KCESService.IsKCESNativeTexture2DFile(candidate) {
			return candidate
		}
	}
	return base + ".tex"
}

// isRGBA32DecodableImageFile 判断路径是否为可直接解码成 RGBA32 的 PNG 或 JPEG 图像
// isRGBA32DecodableImageFile reports whether a path names a PNG or JPEG image that decodes directly to RGBA32
func isRGBA32DecodableImageFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return true
	default:
		return false
	}
}
