package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// determineCmd represents the determine command
var determineCmd = &cobra.Command{
	Use:   "determine [file/directory]",
	Short: "Determine file types",
	Long: `Determine the types of files in a directory or a single file.
This command analyzes files and provides detailed information about their types,
formats, game compatibility, and other metadata.

Use the --strict flag to enforce strict file type determination based on content
rather than relying on file extensions.

Examples:
  MeidoSerialization determine example.menu
  MeidoSerialization determine --strict ./mods_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if isDirectory(path) {
			fmt.Printf("Analyzing directory: %s\n", path)
			// 检测是只读探查，认不出来的文件本身就是一种结果，不应该让整批命令以失败结束
			// Detection is a read-only probe and a file that cannot be identified is itself a result, so it must not fail the whole batch
			return processDirectoryConcurrent(path, determineFileTypeTolerant, fileTypeFilter)
		}

		return processFile(path, determineFileType)
	},
}

// determineFileTypeTolerant 在目录模式下报告单个文件的检测失败但不让整批命令失败
// determineFileTypeTolerant reports a per-file detection failure in directory mode without failing the whole batch
func determineFileTypeTolerant(path string) error {
	if err := determineFileType(path); err != nil {
		fmt.Printf("Cannot determine %s: %v\n", path, err)
	}
	return nil
}

// init 保留文件类型检测命令的初始化扩展点
// init retains the initialization extension point for the file-type detection command
func init() {
	// Add any command-specific flags here
}
