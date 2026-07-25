package KCES

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// canonicalAssetPathForID 统一路径分隔符、大小写和清理规则
// canonicalAssetPathForID normalizes separators, case, and cleaning rules
func canonicalAssetPathForID(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" {
		return "", fmt.Errorf("canonical asset path is empty")
	}
	if strings.HasPrefix(name, "/") || path.IsAbs(name) {
		return "", fmt.Errorf("canonical asset path %q is absolute", name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("canonical asset path %q escapes the directory root", name)
	}
	return strings.ToLower(clean), nil
}

// canonicalAssetPathID 为规范相对路径计算稳定的非零 64 位 PathID
// canonicalAssetPathID computes a stable non-zero 64-bit PathID from a canonical relative path
func canonicalAssetPathID(canonicalPath string, salt uint64) int64 {
	seed := canonicalPath
	if salt != 0 {
		seed = fmt.Sprintf("%s#%d", canonicalPath, salt)
	}
	sum := sha256.Sum256([]byte("MeidoSerialization/KCES/PathID/" + seed))
	id := int64(binary.BigEndian.Uint64(sum[:8]))
	if id == 0 {
		id = 1
	}
	return id
}

// buildCanonicalPathIDs 按排序后的路径处理哈希碰撞，保证不同扫描顺序得到相同 PathID
// buildCanonicalPathIDs resolves hash collisions in sorted order so scan order cannot change PathIDs
func buildCanonicalPathIDs(paths []string) (map[string]int64, error) {
	canonical := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		name, err := canonicalAssetPathForID(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("duplicate canonical asset path %q", name)
		}
		seen[name] = struct{}{}
		canonical = append(canonical, name)
	}
	sort.Strings(canonical)
	result := make(map[string]int64, len(canonical))
	used := make(map[int64]string, len(canonical))
	for _, name := range canonical {
		for salt := uint64(0); ; salt++ {
			id := canonicalAssetPathID(name, salt)
			if previous, ok := used[id]; ok {
				if previous == name {
					return nil, fmt.Errorf("canonical asset path %q was assigned twice", name)
				}
				continue
			}
			used[id] = name
			result[name] = id
			break
		}
	}
	return result, nil
}

// canonicalOrdinalAssetName 在资源扩展名前插入固定宽度序号，无扩展名时追加到末尾
// canonicalOrdinalAssetName inserts a fixed-width ordinal before an asset extension or appends it when no extension exists
func canonicalOrdinalAssetName(name string, ordinal int64) string {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	if stem == "" {
		return fmt.Sprintf("%s_%04d", name, ordinal)
	}
	return fmt.Sprintf("%s_%04d%s", stem, ordinal, extension)
}
