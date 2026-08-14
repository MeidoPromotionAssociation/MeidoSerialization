package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd 输出应用版本和 Git 提交
// versionCmd prints the application version and Git commit
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Get version",
	Long:  "Get version of MeidoSerialization",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("MeidoSerialization %s\n", applicationVersion())
		fmt.Printf("Build Date: %s\n", buildDate)
		fmt.Printf("Git Commit: %s\n", shortBuildCommit())
	},
}

// init 保留版本命令的初始化扩展点
// init retains the initialization extension point for the version command
func init() {}
