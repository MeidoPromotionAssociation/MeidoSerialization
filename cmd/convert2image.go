package cmd

import (
	"fmt"

	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
	"github.com/spf13/cobra"
)

var (
	outputFormat string
)

// convert2imageCmd represents the convert2image command
var convert2imageCmd = &cobra.Command{
	Use:   "convert2image [file/directory]",
	Short: "Convert .tex, Texture2D, and Sprite files to images",
	Long: `Convert .tex and native KCES Texture2D files to PNG or DDS, or native Sprite files to PNG.
This command can process a single file or all files in a directory.

Default output format is .png

Examples:
  MeidoSerialization convert2image example.tex
  MeidoSerialization convert2image example.tex --format jpg
  MeidoSerialization convert2image ./textures_directory
  MeidoSerialization convert2image ./textures_directory --format webp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		processor := func(filePath string) error {
			return convertToImage(filePath, outputFormat)
		}

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectoryConcurrent(path, processor, func(p string) bool {
				return fileTypeFilter(p) && (isTexFile(p) || KCESService.IsKCESNativeTexture2DFile(p) || KCESService.IsKCESNativeSpriteFile(p))
			})
		}

		return processFile(path, processor)
	},
}

// init 注册图像输出格式参数
// init registers the image output format flag
func init() {
	convert2imageCmd.Flags().StringVarP(&outputFormat, "format", "f", "png", "Output image format (png or dds for native Texture2D; png for native Sprite)")
}
