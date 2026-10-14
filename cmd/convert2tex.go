package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	compressTex bool
	forcePng    bool
)

// convert2texCmd represents the convert2tex command
var convert2texCmd = &cobra.Command{
	Use:   "convert2tex [file/directory]",
	Short: "Convert image files to .tex",
	Long: `Convert image files (e.g., .png) to .tex format.
This command can process a single file or all files in a directory.

Use --compress for DXT compression.

Examples:
  MeidoSerialization convert2tex example.png
  MeidoSerialization convert2tex example.jpg --compress
  MeidoSerialization convert2tex example.png --forcePng false
  MeidoSerialization convert2tex ./images_directory
  MeidoSerialization convert2tex ./images_directory --compress --forcePng false`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		processor := func(filePath string) error {
			return convertToTex(filePath, compressTex, forcePng)
		}

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			// 只按后缀预筛，避免在遍历中对每个文件都启动一次 ImageMagick
			// 真正受支持的格式仍由 convertToTex 中的 tools.IsSupportedImageType 裁定
			// Pre-filter by suffix alone so the traversal does not start ImageMagick once per file
			// The formats that are actually supported are still decided by tools.IsSupportedImageType inside convertToTex
			return processDirectory(path, processor, func(p string) bool {
				return fileTypeFilter(p) && isImageCandidateFile(p)
			})
		}

		return processFile(path, processor)
	},
}

// init 注册 TEX 压缩和 PNG 载荷参数
// init registers the TEX compression and PNG payload flags
func init() {
	// Add command-specific flags here
	convert2texCmd.Flags().BoolVarP(&compressTex, "compress", "c", false, "Enable DXT compression for .tex files")
	convert2texCmd.Flags().BoolVarP(&forcePng, "forcePng", "f", true, "Force use of png (lossless) for data in .tex")
}
