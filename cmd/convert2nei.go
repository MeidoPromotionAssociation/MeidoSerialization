package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// convert2neiCmd represents the convert2nei command
var convert2neiCmd = &cobra.Command{
	Use:   "convert2nei [file/directory]",
	Short: "Convert CSV files to .nei",
	Long: `Convert .csv files to .nei format (encrypted Shift-JIS CSV).
This command can process a single file or all files in a directory.

The CSV must be written with UTF-8-BOM encoding, and using ',' as the separator.

Examples:
  MeidoSerialization convert2nei example.csv
  MeidoSerialization convert2nei ./csv_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		processor := func(filePath string) error {
			return convertToNei(filePath)
		}

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectory(path, processor, isCsvFile)
		}

		return processFile(path, processor)
	},
}

func init() {
	// No specific flags for this command
}
