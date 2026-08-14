package cmd

// buildVersion 是构建时通过 ldflags 注入的应用版本号
// buildVersion is the application version injected through ldflags at build time
var buildVersion = "dev"

// buildCommit 是构建时通过 ldflags 注入的 Git 提交哈希
// buildCommit is the Git commit hash injected through ldflags at build time
var buildCommit = "unknown"

// shortBuildCommit 将 Git 提交哈希截断为前 8 位
// shortBuildCommit truncates the Git commit hash to its first 8 characters
func shortBuildCommit() string {
	if len(buildCommit) > 8 {
		return buildCommit[:8]
	}
	return buildCommit
}
