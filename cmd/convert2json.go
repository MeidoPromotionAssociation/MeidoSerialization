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
Supported file types include: .menu, .mate/.mat, .pmat, .col, .phy, .psk, .anm, .model and .preset.
KCES parts payloads are also supported: .menuassets, .materialassets, .pmatassets,
.model, .kcmenu, .kcmat, and .kcmodel files.

Not supported: .tex
  please use convert2image instead

Examples:
  MeidoSerialization convert2json example.menu
  MeidoSerialization convert2json ./mods_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectoryConcurrent(path, convertToJson, func(p string) bool {
				return fileTypeFilter(p) && isConvertibleNativeFile(p)
			})
		}

		return processFile(path, convertToJson)
	},
}

// init 保留转编辑 JSON 命令的初始化扩展点
// init retains the initialization extension point for the editing-JSON conversion command
func init() {
	// Add any command-specific flags here
}
