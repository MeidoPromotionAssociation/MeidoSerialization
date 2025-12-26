package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	outputPathFlag string
)

// unpackArcCmd represents the unpack command
var unpackArcCmd = &cobra.Command{
	Use:   "unpackArc [file/directory]",
	Short: "unpackArc .arc files to directories",
	Long: `unpackArc .arc files to directories.
This command can process a single file or all .arc files in a directory.

Examples:
  MeidoSerialization unpackArc example.arc
  MeidoSerialization unpackArc example.arc -o ./output_dir
  MeidoSerialization unpackArc ./arc_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			// When processing a directory, we use outputPathFlag as the base directory if provided
			processor := unpackArc
			if outputPathFlag != "" {
				processor = func(p string) error {
					// Join base output dir with arc filename
					rel, _ := filepath.Rel(path, p)
					target := filepath.Join(outputPathFlag, rel+"_unpacked")
					return unpackArcTo(p, target)
				}
			}

			return processDirectoryConcurrent(path, processor, func(p string) bool {
				return fileTypeFilter(p) && isArcFile(p)
			})
		}

		return unpackArcTo(path, outputPathFlag)
	},
}

func init() {
	unpackArcCmd.Flags().StringVarP(&outputPathFlag, "output", "o", "", "Output directory path")
}
