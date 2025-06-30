package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// determineCmd represents the determine command
var determineCmd = &cobra.Command{
	Use:   "determine [file/directory]",
	Short: "Determine file types",
	Long: `Determine the types of files in a directory or a single file.
This command analyzes files and provides detailed information about their types,
formats, game compatibility, and other metadata.

Use the --strict flag to enforce strict file type determination based on content
rather than relying on file extensions.

Examples:
  MeidoSerialization determine example.menu
  MeidoSerialization determine --strict ./mods_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if isDirectory(path) {
			fmt.Printf("Analyzing directory: %s\n", path)
			return processDirectory(path, determineFileType, func(p string) bool {
				// Process all files when determining types
				return true
			})
		}

		return processFile(path, determineFileType)
	},
}

func init() {
	// Add any command-specific flags here
}
