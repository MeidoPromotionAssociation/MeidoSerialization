package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// convert2jsonCmd represents the convert2json command
var convert2jsonCmd = &cobra.Command{
	Use:   "convert2json [file/directory]",
	Short: "Convert MOD files to JSON",
	Long: `Convert MOD files to JSON format.
This command can process a single file or all files in a directory.
Supported file types include: .menu, .mate, .pmat, .col, .phy, .psk, .tex, .anm, and .model.

Examples:
  meido convert2json example.menu
  meido convert2json ./mods_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectory(path, convertToJson, isModFile)
		}

		return processFile(path, convertToJson)
	},
}

func init() {
	// Add any command-specific flags here
}
