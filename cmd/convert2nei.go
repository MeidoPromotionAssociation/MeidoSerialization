package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// neiFormatFlag 保存 convert2nei 选择的输出格式名称
// 空值与 auto 表示沿用即将被覆盖的同名 .nei 的格式
// neiFormatFlag holds the output format name selected for convert2nei
// An empty value and auto reuse the format of the same-named .nei that is about to be overwritten
var neiFormatFlag string

// convert2neiCmd represents the convert2nei command
var convert2neiCmd = &cobra.Command{
	Use:   "convert2nei [file/directory]",
	Short: "Convert CSV files to .nei",
	Long: `Convert .csv files to .nei format (encrypted CSV).
This command can process a single file or all files in a directory.

The CSV must be written with UTF-8-BOM encoding, and using ',' as the separator.

A .nei table is read by exactly one game, and the two games disagree on how cell
text is stored: COM3D2 reads Shift-JIS (CP932) while KCES reads UTF-8. The container
carries no marker, so --format selects the game that will read the output.
By default the format of the .nei being overwritten is reused, and COM3D2 is used
when that file does not exist yet. Choosing the wrong game shows up as garbage
Japanese in-game.

Examples:
  MeidoSerialization convert2nei example.csv
  MeidoSerialization convert2nei example.csv --format kces
  MeidoSerialization convert2nei ./csv_directory --format com3d2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		encoding, err := parseNeiOutputFormat(neiFormatFlag)
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

// init 注册 CSV 转 NEI 命令的输出格式选项
// init registers the output format option for the CSV-to-NEI command
func init() {
	convert2neiCmd.Flags().StringVarP(&neiFormatFlag, "format", "f", "auto",
		"Format of the output .nei: 'auto' reuses the format of the .nei being overwritten, or pass 'com3d2' (Shift-JIS) or 'kces' (UTF-8)")
}
