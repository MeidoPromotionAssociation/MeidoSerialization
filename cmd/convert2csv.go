package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// convert2csvCmd represents the convert2csv command
var convert2csvCmd = &cobra.Command{
	Use:   "convert2csv [file/directory]",
	Short: "Convert .nei files to CSV",
	Long: `Convert .nei files (encrypted Shift-JIS CSV) to .csv format.
This command can process a single file or all files in a directory.

The CSV is written with UTF-8-BOM encoding, using ',' as the separator.

Examples:
  MeidoSerialization convert2csv example.nei
  MeidoSerialization convert2csv ./nei_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		processor := func(filePath string) error {
			return convertToCsv(filePath)
		}

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectory(path, processor, isNeiFile)
		}

		return processFile(path, processor)
	},
}

func init() {
	// No specific flags for this command
}
