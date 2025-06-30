package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	outputFormat string
)

// convert2imageCmd represents the convert2image command
var convert2imageCmd = &cobra.Command{
	Use:   "convert2image [file/directory]",
	Short: "Convert .tex files to image files",
	Long: `Convert .tex files to a specified image format.
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
			return processDirectory(path, processor, isTexFile)
		}

		return processFile(path, processor)
	},
}

func init() {
	// Add command-specific flags here
	convert2imageCmd.Flags().StringVarP(&outputFormat, "format", "f", "png", "Output image format (e.g., png, jpg, webp)")
}
