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
- If the file is a JSON file that corresponds to a MOD file, it will be converted back to MOD

This command can process a single file or all files in a directory.

Examples:
  meido convert example.menu
  meido convert example.menu.json
  meido convert ./mixed_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectory(path, convertFile, func(p string) bool {
				return isModFile(p) || isModJsonFile(p)
			})
		}

		return processFile(path, convertFile)
	},
}

func init() {
	// Add any command-specific flags here
}
