package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/application"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/KCES"
	"github.com/spf13/cobra"
)

var texture2dMipModeFlag string

var convert2texture2dCmd = &cobra.Command{
	Use:   "convert2texture2d [file/directory]",
	Short: "Convert image files to native KCES Texture2D",
	Long: `Convert PNG or JPEG images back to native KCES Texture2D primary files.
The texture is rebuilt from scratch as inline RGBA32 data.
Every native Texture2D file with the same base name (.tex or .texture2d) is
overwritten so packAba picks up the change, and a .tex is created when there is
none yet.
This command can process a single file or all matching images in a directory.

Use --mipmap to control the mip chain:
  auto  probe the file being overwritten and keep its level count (default)
  none  write a single level
  full  generate the full chain down to 1x1

Typical texture editing workflow:
  unpackAba mod.aba -> convert2image Texture2D\foo.tex -> edit foo.png ->
  convert2texture2d foo.png -> packAba

Examples:
  MeidoSerialization convert2texture2d example.png
  MeidoSerialization convert2texture2d example.png --mipmap full
  MeidoSerialization convert2texture2d ./mod_unpacked/Texture2D`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		mipMode, err := parseTexture2DMipMode(texture2dMipModeFlag)
		if err != nil {
			return err
		}
		processor := func(filePath string) error {
			return convertImageToNativeTexture2D(filePath, mipMode)
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

// init 注册 mipmap 生成策略参数
// init registers the mipmap generation strategy flag
func init() {
	convert2texture2dCmd.Flags().StringVar(&texture2dMipModeFlag, "mipmap", "auto", "Mipmap handling: auto (probe the file being overwritten and keep its level count), none, or full")
}

// parseTexture2DMipMode 把命令行取值解析为 mipmap 生成策略
// CLI 词汇与内部词汇的解耦点只在这里，改动取值或底层枚举只需改这一处
// parseTexture2DMipMode parses the command line value into a mipmap generation strategy
// The CLI vocabulary is decoupled from the internal vocabulary only here, so changing the accepted values or the underlying enum only touches this function
func parseTexture2DMipMode(value string) (KCESService.Texture2DMipMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto":
		return KCESService.Texture2DMipModeAuto, nil
	case "none":
		return KCESService.Texture2DMipModeNone, nil
	case "full":
		return KCESService.Texture2DMipModeFull, nil
	default:
		return 0, fmt.Errorf("unsupported mipmap mode %q; use auto, none, or full", value)
	}
}

// convertImageToNativeTexture2D 将 PNG 或 JPEG 图像重建为 KCES 原生 Texture2D 独立对象文件
// 同基名可能同时存在 .tex 与 .texture2d 两个原生 Texture2D，此时把图像写进每一个，否则只更新其中一个会让另一个留着旧内容
// 一个都不存在时按同基名 .tex 新建
// convertImageToNativeTexture2D rebuilds a PNG or JPEG image into a native KCES Texture2D standalone object file
// One base name may hold both a .tex and a .texture2d native Texture2D, so the image is written into each of them, otherwise updating only one would leave the other with stale content
// When neither exists, a .tex with the same base name is created
func convertImageToNativeTexture2D(path string, mipMode KCESService.Texture2DMipMode) error {
	targets := existingNativeTexture2DPaths(path)
	if len(targets) == 0 {
		targets = []string{trimLastExtension(path) + ".tex"}
	}
	for _, target := range targets {
		if err := convertImageToNativeTexture2DAt(path, target, mipMode); err != nil {
			return err
		}
	}
	return nil
}

// convertImageToNativeTexture2DAt 将 PNG 或 JPEG 图像按指定 mipmap 策略重建为指定路径的 KCES 原生 Texture2D
// convertImageToNativeTexture2DAt rebuilds a PNG or JPEG image into a native KCES Texture2D at an explicit path using the requested mipmap strategy
func convertImageToNativeTexture2DAt(path string, outputPath string, mipMode KCESService.Texture2DMipMode) error {
	if !isRGBA32DecodableImageFile(path) {
		return fmt.Errorf("not a PNG or JPEG image file: %s", path)
	}
	service := &KCESService.NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2DWithMipMode(context.Background(), path, outputPath, mipMode, application.DefaultMaxOutputBytes); err != nil {
		return fmt.Errorf("failed to convert %s to native Texture2D: %w", path, err)
	}
	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
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
