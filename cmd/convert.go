package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// convertNeiFormatFlag 保存 convert 自动命令选择的 NEI 输出格式名称
// 空值与 auto 表示沿用即将被覆盖的同名 .nei 的格式
// convertNeiFormatFlag holds the NEI output format name selected for the automatic convert command
// An empty value and auto reuse the format of the same-named .nei that is about to be overwritten
var convertNeiFormatFlag string

// convertCmd represents the convert command
var convertCmd = &cobra.Command{
	Use:   "convert [file/directory]",
	Short: "Auto-detect and convert files",
	Long: `Auto-detect and convert files between native and common formats.
This command automatically determines the direction of conversion:

- If the file is a native MOD file, it will be converted to editing JSON
- If the file is editing JSON that corresponds to a native MOD file, it will be converted back
- If the file is a tex, native Texture2D, or native Sprite file, it will be converted to an image
- If the file is an image, it will be written back in the format of its same-named sibling:
  a native KCES Texture2D when one exists, otherwise a COM3D2 .tex
- If the file is a nei file, it will be converted to csv
- If the file is a csv file, it will be converted to nei
- If the file is an arc file, it will be unpacked

COM3D2 reads .nei cell text as Shift-JIS while KCES reads it as UTF-8, and a CSV keeps no trace
of which game it came from. Converting to .nei therefore reuses the format of the .nei that is
about to be overwritten, and --format com3d2 or --format kces overrides that choice.

This command can process a single file or all files in a directory.

This method does not perform concurrent operations when processing folders and is therefore slower than separate commands (convert2mod convert2json...).

Examples:
  MeidoSerialization convert example.menu
  MeidoSerialization convert example.menu.json
  MeidoSerialization convert example.tex
  MeidoSerialization convert example.nei
  MeidoSerialization convert example.csv --format kces
  MeidoSerialization convert example.arc
  MeidoSerialization convert ./mixed_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		encoding, err := parseNeiOutputFormat(convertNeiFormatFlag)
		if err != nil {
			return err
		}

		processor := func(filePath string) error {
			return convertFile(filePath, encoding)
		}

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectory(path, processor, func(p string) bool {
				// 廉价判断放在前面，避免对根本不能转换的文件启动 ImageMagick
				// The cheap check comes first so ImageMagick is not started for files that cannot be converted
				if !isAutoConvertibleFile(p) {
					// 显式按图片类型筛选时让 ImageMagick 最终裁定，代价是每个文件一次 identify
					// An explicit image type filter lets ImageMagick have the final say at the cost of one identify per file
					return isImageTypeFilter() && fileTypeFilter(p)
				}
				return fileTypeFilter(p)
			})
		}

		return processFile(path, processor)
	},
}

// init 注册自动转换命令的 NEI 输出格式选项
// init registers the NEI output format option for the automatic conversion command
func init() {
	convertCmd.Flags().StringVarP(&convertNeiFormatFlag, "format", "f", "auto",
		"Format of an output .nei: 'auto' reuses the format of the .nei being overwritten, or pass 'com3d2' (Shift-JIS) or 'kces' (UTF-8)")
}
