package KCES

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// These are the first nine one-to-one mappings used by
// VirtualDirectory.AdvancedController. ReplaceTexts contains an unused tenth
// glyph in KCES 1.34.4; there is no corresponding InvalidFileNameChars entry.
var virtualDirectoryWindowsNameEscaper = strings.NewReplacer(
	`"`, "❶",
	"<", "❷",
	">", "❸",
	"|", "❹",
	":", "❺",
	"*", "❻",
	"?", "❼",
	`\`, "❽",
)

var virtualDirectoryWindowsNameUnescaper = strings.NewReplacer(
	"❶", `"`,
	"❷", "<",
	"❸", ">",
	"❹", "|",
	"❺", ":",
	"❻", "*",
	"❼", "?",
	"❽", `\`,
	// ❾ represents a slash inside a single VirtualDirectory key in the game.
	// This library exposes a flattened slash-separated path map, so it becomes
	// an equivalent path separator when the tree is rebuilt.
	"❾", "/",
)

// virtualDirectoryNameToExtractionPath applies the game's Windows-safe name
// mapping to each flattened VirtualDirectory path component, then subjects the
// result to the normal traversal/ADS/reserved-name extraction checks.
func virtualDirectoryNameToExtractionPath(name string) (string, error) {
	if name == "" || !utf8.ValidString(name) || strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("invalid VirtualDirectory path")
	}
	portable := strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(portable, "/") || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("absolute or volume-qualified VirtualDirectory path is not allowed")
	}
	parts := strings.Split(portable, "/")
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("unsafe VirtualDirectory path component %q", part)
		}
		parts[i] = virtualDirectoryWindowsNameEscaper.Replace(part)
	}
	return normalizeExtractionPath(strings.Join(parts, "/"))
}

// extractionPathToVirtualDirectoryName reverses the mapping used by the game
// when it creates a VirtualDirectory from a Windows directory.
func extractionPathToVirtualDirectoryName(name string) (string, error) {
	rel, err := normalizeExtractionPath(name)
	if err != nil {
		return "", err
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i := range parts {
		parts[i] = virtualDirectoryWindowsNameUnescaper.Replace(parts[i])
	}
	result := strings.Join(parts, "/")
	if result == "" || strings.HasPrefix(result, "/") || strings.Contains(result, "//") {
		return "", fmt.Errorf("invalid decoded VirtualDirectory path %q", result)
	}
	for _, part := range strings.Split(result, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("unsafe decoded VirtualDirectory path component %q", part)
		}
	}
	return result, nil
}
