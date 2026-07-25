package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// AbaService 提供 ABA 文件的读取、列出和纯目录提取操作 / AbaService provides reading, listing, and pure-directory extraction for ABA files
type AbaService struct{}

// rawAssetMeta 保存独立原始 Unity 对象编辑接口使用的 metadata，不参与 ABA 纯目录往返 / rawAssetMeta stores metadata for the individual raw Unity object editing API and does not participate in ABA pure-directory round trips
type rawAssetMeta struct {
	PathID                int64   `json:"pathId"`                          // Unity PathID / Unity PathID
	LoadName              string  `json:"loadName,omitempty"`              // AssetBundle m_Container 加载名 / AssetBundle m_Container load name
	UnityVersion          string  `json:"unityVersion,omitempty"`          // SerializedFile 元数据中的 Unity 版本 / Unity version from SerializedFile metadata
	EngineVersion         string  `json:"engineVersion,omitempty"`         // UnityFS header 中的引擎版本 / Engine version from the UnityFS header
	TargetPlatform        *uint32 `json:"targetPlatform,omitempty"`        // SerializedFile 目标平台 / SerializedFile target platform
	AbaVersion            uint32  `json:"abaVersion,omitempty"`            // UnityFS 文件格式版本 / UnityFS format version
	GenerationVersion     string  `json:"generationVersion,omitempty"`     // UnityFS generation 版本 / UnityFS generation version
	SerializedFileVersion uint32  `json:"serializedFileVersion,omitempty"` // SerializedFile 格式版本 / SerializedFile format version
}

// ReadAba 打开并解析 ABA 文件，返回值中的文件句柄由调用者关闭
// ReadAba opens and parses an ABA file, and the caller closes the returned file handle
func (s *AbaService) ReadAba(path string) (*aba.Aba, *os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open .aba file failed: %w", err)
	}

	abaFile, err := aba.ReadAba(f)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("parse .aba file failed: %w", err)
	}
	return abaFile, f, nil
}

// ListAba 列出 ABA 中所有 SerializedFile 的资源对象
// ListAba lists resource objects from every SerializedFile in an ABA
func (s *AbaService) ListAba(path string) ([]aba.AssetEntry, error) {
	abaFile, f, err := s.ReadAba(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var allEntries []aba.AssetEntry
	for directoryIndex, dir := range abaFile.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := abaFile.GetFileData(int64(directoryIndex))
		if err != nil {
			return nil, fmt.Errorf("read serialized .aba entry %q at directory index %d: %w", dir.Name, directoryIndex, err)
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return nil, fmt.Errorf("parse serialized .aba entry %q at directory index %d: %w", dir.Name, directoryIndex, err)
		}
		allEntries = append(allEntries, af.GetAssetEntries()...)
	}
	return allEntries, nil
}

// UnpackAba 将受支持的 KCES ABA 提取为不含 metadata、预览图和外部流文件的纯资源目录
// UnpackAba extracts a supported KCES ABA into a plain resource directory without metadata, previews, or external stream files
func (s *AbaService) UnpackAba(abaPath string, outDir string) error {
	return s.unpackAbaPureDirectory(abaPath, outDir)
}

// claimExtractionPaths 原子登记规范输出路径并拒绝大小写不敏感的冲突
// claimExtractionPaths atomically claims normalized output paths and rejects case-insensitive conflicts
func claimExtractionPaths(claims map[string]string, owner string, paths ...string) error {
	if claims == nil {
		return nil
	}
	// claim 保存一次原子路径登记的规范键和显示路径 / claim stores the normalized key and display path for one atomic path claim
	type claim struct {
		key  string // 大小写折叠后的冲突键 / Case-folded conflict key
		path string // 规范相对路径 / Normalized relative path
	}
	pending := make([]claim, 0, len(paths))
	for _, candidate := range paths {
		relPath, err := normalizeExtractionPath(candidate)
		if err != nil {
			return fmt.Errorf("invalid output path %q for %s: %w", candidate, owner, err)
		}
		key := strings.ToLower(relPath)
		if previous, exists := claims[key]; exists {
			return fmt.Errorf("output path %q for %s conflicts with %s", relPath, owner, previous)
		}
		for _, existing := range pending {
			if existing.key == key {
				return fmt.Errorf("output path %q is used more than once by %s", relPath, owner)
			}
		}
		pending = append(pending, claim{key: key, path: relPath})
	}
	for _, item := range pending {
		claims[item.key] = owner
	}
	return nil
}

// writeAssetMeta 写入独立原始对象编辑接口使用的最小 metadata
// writeAssetMeta writes minimal metadata used by the individual raw-object editing API
func writeAssetMeta(assetPath string, pathID int64, loadName string) error {
	return writeRawAssetMeta(assetPath, rawAssetMeta{PathID: pathID, LoadName: loadName})
}

// writeRawAssetMeta 写入独立原始对象的完整 metadata
// writeRawAssetMeta writes complete metadata for an individual raw object
func writeRawAssetMeta(assetPath string, meta rawAssetMeta) error {
	data, err := marshalRawAssetMeta(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(assetMetaPath(assetPath), data, 0644)
}

// readAssetMeta 读取独立原始对象的 metadata，读取失败时返回零值供只读展示回退
// readAssetMeta reads metadata for an individual raw object and returns a zero value as a read-only display fallback on failure
func readAssetMeta(assetPath string) rawAssetMeta {
	meta, err := readAssetMetaStrict(assetPath)
	if err != nil {
		return rawAssetMeta{}
	}
	return meta
}

// marshalRawAssetMeta 编码独立原始对象编辑接口的 metadata
// marshalRawAssetMeta encodes metadata for the individual raw-object editing API
func marshalRawAssetMeta(meta rawAssetMeta) ([]byte, error) {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// assetMetaPath 返回独立原始对象编辑接口的 metadata 路径
// assetMetaPath returns the metadata path used by the individual raw-object editing API
func assetMetaPath(assetPath string) string {
	return assetPath + ".meta.json"
}

// textureRawFileName 为原始 Texture2D 对象生成无歧义文件名
// textureRawFileName creates an unambiguous file name for a raw Texture2D object
func textureRawFileName(name string) string {
	safeName := sanitizeName(name)
	if filepath.Ext(safeName) == "" {
		return safeName + ".texture2d.bytes"
	}
	return safeName + ".bytes"
}

// sanitizeName 将 Unity 对象名称转换为跨平台安全的单个文件名组件
// sanitizeName converts a Unity object name into one cross-platform-safe file-name component
func sanitizeName(name string) string {
	var result strings.Builder
	result.Grow(len(name))
	for _, character := range name {
		if character < 0x20 || strings.ContainsRune(`/\:*?"<>|`, character) {
			result.WriteByte('_')
		} else {
			result.WriteRune(character)
		}
	}
	safe := result.String()
	if safe == "" {
		return "unnamed"
	}
	for strings.HasSuffix(safe, ".") || strings.HasSuffix(safe, " ") {
		safe = safe[:len(safe)-1] + "_"
	}
	if safe == "." || safe == ".." || isWindowsReservedName(safe) {
		safe = "_" + safe
	}
	return safe
}
