package mcpserver

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
)

// FilesystemMode controls which path vocabulary MCP tools expose. Restricted
// mode confines every operation to configured roots. Unrestricted mode accepts
// direct paths and therefore inherits all filesystem permissions of the server
// process.
type FilesystemMode string

const (
	FilesystemModeRestricted   FilesystemMode = "restricted"
	FilesystemModeUnrestricted FilesystemMode = "unrestricted"
)

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
