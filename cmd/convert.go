package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// convertCmd represents the convert command
var convertCmd = &cobra.Command{
	Use:   "convert [file/directory]",
	Short: "Auto-detect and convert files",
	Long: `Auto-detect and convert files between MOD and JSON formats.
This command automatically determines the direction of conversion:

- If the file is a MOD file, it will be converted to JSON
- If the file is a JSON file that corresponds to a MOD file, it will be converted to MOD
- If the file is a tex file, it will be converted to image
- If the file is an image file, it will be converted to tex
- If the file is a nei file, it will be converted to csv
- If the file is a csv file, it will be converted to nei

This command can process a single file or all files in a directory.

Examples:
  MeidoSerialization convert example.menu
  MeidoSerialization convert example.menu.json
  MeidoSerialization convert example.tex
  MeidoSerialization convert example.nei
  MeidoSerialization convert ./mixed_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectory(path, convertFile, func(p string) bool {
				if !fileTypeFilter(p) {
					return false
				}
				return isModJsonFile(p) || isModFile(p) || isTexFile(p) || isImageFile(p) || isNeiFile(p) || isCsvFile(p)
			})
		}

		return processFile(path, convertFile)
	},
}

func init() {
	// Add any command-specific flags here
}
