package mcpserver

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
)

// FilesystemMode 控制 MCP 工具公开受限根目录路径还是直接文件系统路径 / FilesystemMode controls whether MCP tools expose confined root paths or direct filesystem paths
type FilesystemMode string

const (
	// FilesystemModeRestricted 将全部 MCP 文件操作限制在配置根目录内 / FilesystemModeRestricted confines all MCP file operations to configured roots
	FilesystemModeRestricted FilesystemMode = "restricted"
	// FilesystemModeUnrestricted 允许使用服务器进程账户可访问的直接路径 / FilesystemModeUnrestricted permits direct paths accessible to the server process account
	FilesystemModeUnrestricted FilesystemMode = "unrestricted"
)

// resolveFilesystemMode 根据显式配置和根目录存在情况选择一致的文件系统模式
// resolveFilesystemMode selects a consistent filesystem mode from explicit configuration and root availability
func resolveFilesystemMode(configured FilesystemMode, roots *application.RootSet) (FilesystemMode, error) {
	rootCount := 0
	if roots != nil {
		rootCount = len(roots.IDs())
	}
	if configured == "" {
		if rootCount != 0 {
			return FilesystemModeRestricted, nil
		}
		return FilesystemModeUnrestricted, nil
	}
	switch configured {
	case FilesystemModeRestricted:
		return configured, nil
	case FilesystemModeUnrestricted:
		if rootCount != 0 {
			return "", fmt.Errorf("unrestricted MCP filesystem mode cannot be combined with configured roots")
		}
		return configured, nil
	default:
		return "", fmt.Errorf("unsupported MCP filesystem mode %q", configured)
	}
}
