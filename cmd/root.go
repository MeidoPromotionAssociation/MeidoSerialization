package cmd

import (
	"github.com/spf13/cobra"
)

var (
	strictMode bool
	fileType   string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "MeidoSerialization",
	Short: "MeidoSerialization CLI tool",
	Long: `MeidoSerialization CLI tool for converting between COM3D2 MOD files and JSON.
This tool can convert MOD files to JSON, JSON files to MOD files, and determine file types.
(For .tex, it is converted to image, for .nei, it is converted to .csv)

Supported file types include: .menu, .mate, .pmat, .col, .phy, .psk, .tex, .anm, .model, .nei

Github: https://github.com/MeidoPromotionAssociation/MeidoSerialization



MeidoSerialization CLI 工具，用于在 COM3D2 MOD 文件和 JSON 之间进行转换。
此工具可以将 MOD 文件转换为 JSON，也可以将 JSON 文件转换为 MOD 文件，并可识别文件类型。
（对于 .tex 则是转换为图片，对于 .nei 则是转换为 .csv)

支持的文件类型包括：.menu、.mate、.pmat、.col、.phy、.psk、.tex、.anm、.model、.nei

中文说明请查看在线文档：
Github：https://github.com/MeidoPromotionAssociation/MeidoSerialization
`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help
		err := cmd.Help()
		if err != nil {
			return
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	// Add global flags
	RootCmd.PersistentFlags().BoolVarP(&strictMode, "strict", "s", false, "Use strict mode for file type determination")
	RootCmd.PersistentFlags().StringVarP(&fileType, "type", "t", "", "Filter by file type (menu, mate, pmat, col, phy, psk, anm, model, tex, nei, csv, image) or '<type>.json' (e.g., 'menu.json')")

	// Add subcommands
	RootCmd.AddCommand(convertCmd)
	RootCmd.AddCommand(convert2jsonCmd)
	RootCmd.AddCommand(convert2modCmd)
	RootCmd.AddCommand(determineCmd)
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(convert2texCmd)
	RootCmd.AddCommand(convert2imageCmd)
	RootCmd.AddCommand(convert2neiCmd)
	RootCmd.AddCommand(convert2csvCmd)
}
