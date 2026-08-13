package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// neiTextEncodingFlag 保存 convert2nei 选择的单元格文本编码名称
// neiTextEncodingFlag holds the cell text encoding name selected for convert2nei
var neiTextEncodingFlag string

// convert2neiCmd represents the convert2nei command
var convert2neiCmd = &cobra.Command{
	Use:   "convert2nei [file/directory]",
	Short: "Convert CSV files to .nei",
	Long: `Convert .csv files to .nei format (encrypted CSV).
This command can process a single file or all files in a directory.

The CSV must be written with UTF-8-BOM encoding, and using ',' as the separator.

Cell text inside a .nei file is not stored in a single fixed encoding:
COM3D2 reads cells as Shift-JIS (CP932) while KCES reads them as UTF-8.
Use --encoding to select the game that will read the output, otherwise
Japanese text will show up as garbage in-game.

Examples:
  MeidoSerialization convert2nei example.csv
  MeidoSerialization convert2nei example.csv --encoding utf-8
  MeidoSerialization convert2nei ./csv_directory --encoding shift-jis`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		encoding, err := parseNeiTextEncoding(neiTextEncodingFlag)
		if err != nil {
			return err
		}

		processor := func(filePath string) error {
			return convertToNei(filePath, encoding)
		}

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectoryConcurrent(path, processor, func(p string) bool {
				return fileTypeFilter(p) && isCsvFile(p)
			})
		}

		return processFile(path, processor)
	},
}

// init 注册 CSV 转 NEI 命令的单元格文本编码选项
// init registers the cell text encoding option for the CSV-to-NEI command
func init() {
	convert2neiCmd.Flags().StringVar(&neiTextEncodingFlag, "encoding", "shift-jis",
		"Cell text encoding of the output .nei: 'shift-jis' (COM3D2) or 'utf-8' (KCES)")
}
