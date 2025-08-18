package cmd

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"
)

// versionCmd get app version
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Get version",
	Long:  "Get version of MeidoSerialization",

	Run: func(cmd *cobra.Command, args []string) {
		// Dynamically obtain version information
		info, ok := debug.ReadBuildInfo()
		version := "dev"
		commit := "unknown"
		buildDate := time.Now().Format("2006-01-02 15:04:05")

		if ok {
			// Get the version from the build information
			version = info.Main.Version
			if version == "(devel)" {
				version = "dev"
			}

			// Try getting the commit hash from the build info
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					commit = setting.Value
					if len(commit) > 8 {
						commit = commit[:8] // Only show the first 8 digits
					}
				} else if setting.Key == "vcs.time" {
					buildDate = setting.Value
				}
			}
		}

		fmt.Printf("MeidoSerialization %s\n", version)
		fmt.Printf("Build Date: %s\n", buildDate)
		fmt.Printf("Git Commit: %s\n", commit)
	},
}

func init() {}
