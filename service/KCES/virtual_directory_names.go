package KCES

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// 这些是 VirtualDirectory.AdvancedController 使用的前九个一一对应映射，KCES 1.34.4 的 ReplaceTexts 包含未使用的第十个字形，但 InvalidFileNameChars 中没有对应条目
// These are the first nine one-to-one mappings used by VirtualDirectory.AdvancedController, while ReplaceTexts contains an unused tenth glyph in KCES 1.34.4 with no corresponding InvalidFileNameChars entry
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
	// 在游戏中 ❾ 表示单个 VirtualDirectory 键内的斜杠，本库公开以斜杠分隔的扁平路径映射，因此重建目录树时会将其转换为等效的路径分隔符
	// In the game ❾ represents a slash inside a single VirtualDirectory key, while this library exposes a flattened slash-separated path map and therefore converts it to an equivalent path separator when rebuilding the tree
	"❾", "/",
)

// virtualDirectoryNameToExtractionPath 对扁平化 VirtualDirectory 路径的每个组成部分应用游戏的 Windows 安全名称映射，然后执行常规的路径穿越、ADS 和保留名称提取检查
// virtualDirectoryNameToExtractionPath applies the game's Windows-safe name mapping to each flattened VirtualDirectory path component and then performs the normal traversal, ADS, and reserved-name extraction checks
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

// extractionPathToVirtualDirectoryName 反向转换游戏从 Windows 目录创建 VirtualDirectory 时使用的名称映射
// extractionPathToVirtualDirectoryName reverses the name mapping used by the game when creating a VirtualDirectory from a Windows directory
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
