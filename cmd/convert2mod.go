package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
)

// convert2modCmd represents the convert2mod command
var convert2modCmd = &cobra.Command{
	Use:   "convert2mod [file/directory]",
	Short: "Convert JSON files to MOD",
	Long: `Convert JSON files back to MOD format.
This command can process a single file or all files in a directory.
It will convert files like .menu.json back to .menu, .mate.json back to .mate, etc.
KCES parts JSON files are also supported: .menuassets.json, .materialassets.json,
.pmatassets.json, and KCES .model.json.

Not supported: .tex.json
  please use convert2tex instead

Examples:
  MeidoSerialization convert2mod example.menu.json
  MeidoSerialization convert2mod ./json_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectoryConcurrent(path, convertToMod, func(p string) bool {
				return fileTypeFilter(p) && (isModJsonFile(p) || KCESService.IsKCESPartsJSONFile(p) || KCESService.IsKCESPayloadJSONFile(p) || KCESService.IsKCESMiscJSONFile(p) || KCESService.IsKCESDataJSONFile(p) || KCESService.IsKCESRawUnityBytesJSONFile(p) || KCESService.IsKCESCtJSONFile(p))
			})
		}

		return processFile(path, convertToMod)
	},
}

func init() {
	// Add any command-specific flags here
}
