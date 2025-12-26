package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	packOutputFlag string
)

// packArcCmd represents the packarc command
var packArcCmd = &cobra.Command{
	Use:   "packArc [directory]",
	Short: "Pack a directory into a .arc file",
	Long: `Pack a directory into a .arc file.
The folder structure will be preserved inside the ARC.

Examples:
  MeidoSerialization packArc ./my_folder
  MeidoSerialization packArc ./my_folder -o custom.arc`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if !isDirectory(path) {
			return fmt.Errorf("%s is not a directory", path)
		}

		outputPath := packOutputFlag
		if outputPath == "" {
			// Default to directory_name.arc
			abs, _ := filepath.Abs(path)
			name := filepath.Base(abs)
			// Remove common suffixes like _unpacked if they exist to be clean
			name = strings.TrimSuffix(name, "_unpacked")
			outputPath = name + ".arc"
		}

		return packArc(path, outputPath)
	},
}

func init() {
	packArcCmd.Flags().StringVarP(&packOutputFlag, "output", "o", "", "Output ARC file path")
}
