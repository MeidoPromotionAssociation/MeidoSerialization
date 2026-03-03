package cmd

import (
	"github.com/spf13/cobra"
)

// listArcCmd represents the listArc command
var listArcCmd = &cobra.Command{
	Use:   "listArc [file]",
	Short: "List files inside a .arc archive",
	Long: `List all files inside a .arc archive.

Examples:
  MeidoSerialization listArc example.arc`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return listArcFiles(args[0])
	},
}
