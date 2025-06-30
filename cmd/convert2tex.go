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
			return processDirectory(path, processor, isImageFile)
		}

		return processFile(path, processor)
	},
}

func init() {
	// Add command-specific flags here
	convert2texCmd.Flags().BoolVarP(&compressTex, "compress", "c", false, "Enable DXT compression for .tex files")
	convert2texCmd.Flags().BoolVarP(&forcePng, "forcePng", "f", true, "Force use of png (lossless) for data in .tex")
}
