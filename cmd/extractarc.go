package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	extractExt        string
	extractFile       string
	extractOutputFlag string
)

// extractArcCmd represents the extractArc command
var extractArcCmd = &cobra.Command{
	Use:   "extractArc [file/directory]",
	Short: "Extract files from a .arc archive by extension or file path",
	Long: `Extract files from a .arc archive selectively.
This command can process a single file or all .arc files in a directory.

Use --ext to extract all files with a given extension.
Use --file to extract a single file by its full path or filename within the archive (single .arc only).
If only a filename is given, the archive is searched for a matching entry.
Exactly one of --ext or --file must be provided.

Examples:
  MeidoSerialization extractArc example.arc --ext .menu
  MeidoSerialization extractArc example.arc --ext tex
  MeidoSerialization extractArc example.arc --file folder/texture.tex
  MeidoSerialization extractArc example.arc --file script.nei
  MeidoSerialization extractArc example.arc --ext .menu -o ./output_dir
  MeidoSerialization extractArc ./arc_directory --ext .tex`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		hasExt := extractExt != ""
		hasFile := extractFile != ""

		if !hasExt && !hasFile {
			return fmt.Errorf("either --ext or --file must be provided")
		}
		if hasExt && hasFile {
			return fmt.Errorf("--ext and --file cannot be used together")
		}

		if isDirectory(path) {
			if hasFile {
				return fmt.Errorf("--file cannot be used with a directory input")
			}

			fmt.Printf("Processing directory: %s\n", path)
			processor := func(p string) error {
				outDir := extractOutputFlag
				if outDir == "" {
					outDir = p + "_extracted"
				} else {
					rel, _ := filepath.Rel(path, p)
					outDir = filepath.Join(extractOutputFlag, rel+"_extracted")
				}
				return extractArcByExt(p, extractExt, outDir)
			}

			return processDirectoryConcurrent(path, processor, func(p string) bool {
				return fileTypeFilter(p) && isArcFile(p)
			})
		}

		outDir := extractOutputFlag
		if outDir == "" {
			outDir = path + "_extracted"
		}

		if hasExt {
			return extractArcByExt(path, extractExt, outDir)
		}
		return extractArcFile(path, extractFile, outDir)
	},
}

func init() {
	extractArcCmd.Flags().StringVarP(&extractExt, "ext", "e", "", "Extract all files with this extension (e.g., .menu, tex)")
	extractArcCmd.Flags().StringVarP(&extractFile, "file", "f", "", "Extract a single file by its full path or filename within the archive")
	extractArcCmd.Flags().StringVarP(&extractOutputFlag, "output", "o", "", "Output directory (default: <arcname>_extracted)")
}
