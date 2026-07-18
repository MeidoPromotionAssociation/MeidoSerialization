package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// AbaService 提供 .aba 文件 Unity AssetBundle 的读取、列出和提取操作 / AbaService provides read, list, and extraction operations for .aba Unity AssetBundle files
type AbaService struct{}

const (
	abaBundleMetaFileName = ".meido-aba.meta.json"
	abaBundleMetaFormat   = "kces-unityfs-bundle"
)

// abaBundleMeta preserves the UnityFS header context which cannot be inferred
// from a directory containing only non-serialized bundle entries.
type abaBundleMeta struct {
	Format            string `json:"format"`
	BundleVersion     uint32 `json:"bundleVersion"`
	GenerationVersion string `json:"generationVersion"`
	EngineVersion     string `json:"engineVersion"`
}

// rawAssetMeta 表示原始 Unity 对象 sidecar 元数据 / rawAssetMeta represents sidecar metadata for a raw Unity object
type rawAssetMeta struct {
	PathID                int64   `json:"pathId"`                          // Unity PathID / Unity PathID
	LoadName              string  `json:"loadName,omitempty"`              // AssetBundle m_Container 加载名 / AssetBundle m_Container load name
	UnityVersion          string  `json:"unityVersion,omitempty"`          // SerializedFile 元数据中的 Unity 版本 / Unity version from SerializedFile metadata
	EngineVersion         string  `json:"engineVersion,omitempty"`         // UnityFS header 中的引擎版本 / Engine version from the UnityFS header
	TargetPlatform        *uint32 `json:"targetPlatform,omitempty"`        // SerializedFile 目标平台；指针用于区分缺失与显式 0 / SerializedFile target platform; pointer distinguishes absent from explicit zero
	BundleVersion         uint32  `json:"bundleVersion,omitempty"`         // UnityFS 文件格式版本（KCES 样本为 7 或 8）/ UnityFS format version, 7 or 8 in KCES samples
	GenerationVersion     string  `json:"generationVersion,omitempty"`     // UnityFS generation version / UnityFS generation version
	SerializedFileVersion uint32  `json:"serializedFileVersion,omitempty"` // SerializedFile 格式版本 / SerializedFile format version
}

// ReadAba 读取 .aba 文件并返回 Bundle
func (s *AbaService) ReadAba(path string) (*aba.Bundle, *os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open .aba file failed: %w", err)
	}

	bundle, err := aba.ReadBundle(f)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("parse .aba file failed: %w", err)
	}
	return bundle, f, nil
}

// ListAba 列出 .aba 文件中的所有资源
func (s *AbaService) ListAba(path string) ([]aba.AssetEntry, error) {
	bundle, f, err := s.ReadAba(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var allEntries []aba.AssetEntry
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(i)
		if err != nil {
			return nil, fmt.Errorf("read serialized bundle file %q at directory index %d: %w", dir.Name, i, err)
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return nil, fmt.Errorf("parse serialized bundle file %q at directory index %d: %w", dir.Name, i, err)
		}
		allEntries = append(allEntries, af.GetAssetEntries()...)
	}
	return allEntries, nil
}

// UnpackAba 将 .aba 文件中的所有资源提取到指定目录
func (s *AbaService) UnpackAba(abaPath string, outDir string) error {
	bundle, f, err := s.ReadAba(abaPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if outDir == "" {
		outDir = abaPath + "_unpacked"
	}
	rawBundlePaths := make(map[int]string)
	claimedOutputPaths := make(map[string]string)
	if err := claimExtractionPaths(claimedOutputPaths, "UnityFS bundle metadata", abaBundleMetaFileName); err != nil {
		return err
	}
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if dir.IsSerialized() {
			continue
		}
		relPath, err := normalizeExtractionPath(dir.Name)
		if err != nil {
			return fmt.Errorf("unsafe bundle file name %q: %w", dir.Name, err)
		}
		if err := claimExtractionPaths(claimedOutputPaths, "bundle file "+dir.Name, relPath); err != nil {
			return err
		}
		rawBundlePaths[i] = relPath
	}

	serialized := make(map[int]*aba.AssetsFile)
	serializedByName := make(map[string]*aba.AssetsFile)
	containerMaps := make(map[int]map[int64]string)
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(i)
		if err != nil {
			return fmt.Errorf("read file %q from bundle failed: %w", dir.Name, err)
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return fmt.Errorf("parse AssetsFile %q failed: %w", dir.Name, err)
		}
		serialized[i] = af
		serializedByName[dir.Name] = af
		serializedByName[filepath.Base(dir.Name)] = af
		containerNames, err := af.GetAssetBundleContainerMap()
		if err != nil {
			return fmt.Errorf("read AssetBundle container map from %q: %w", dir.Name, err)
		}
		containerMaps[i] = containerNames
	}
	resolver := aba.BundleAssetResolver(serializedByName)
	streamResolver := bundle.GetFileDataRangeByName
	root, err := openExtractionRoot(outDir)
	if err != nil {
		return err
	}
	defer root.Close()
	bundleMetaData, err := json.MarshalIndent(abaBundleMeta{
		Format:            abaBundleMetaFormat,
		BundleVersion:     bundle.Header.Version,
		GenerationVersion: bundle.Header.GenerationVersion,
		EngineVersion:     bundle.Header.EngineVersion,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal UnityFS bundle metadata: %w", err)
	}
	bundleMetaData = append(bundleMetaData, '\n')
	if err := root.WriteFile(abaBundleMetaFileName, bundleMetaData, 0644); err != nil {
		return fmt.Errorf("write UnityFS bundle metadata: %w", err)
	}

	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			// 非序列化文件（如多 GiB 的 .resS）使用范围 API 流式提取，
			// 避免 GetFileData 的单次内存上限和整文件分配。
			if err := writeRawBundleDirectory(root, bundle, dir, rawBundlePaths[i]); err != nil {
				return fmt.Errorf("write raw bundle file %q: %w", dir.Name, err)
			}
			continue
		}

		af := serialized[i]
		if af == nil {
			return fmt.Errorf("serialized bundle file %q at directory index %d was not loaded", dir.Name, i)
		}
		if err := unpackAssetsFile(root, dir.Name, bundle, af, containerMaps[i], resolver, streamResolver, claimedOutputPaths); err != nil {
			return err
		}
	}
	return nil
}

func writeRawBundleDirectory(root *extractionRoot, bundle *aba.Bundle, dir aba.DirectoryInfo, relPath string) error {
	if root == nil || bundle == nil {
		return fmt.Errorf("nil extraction root or bundle")
	}
	if dir.DecompressedSize < 0 {
		return fmt.Errorf("negative directory size %d", dir.DecompressedSize)
	}
	const chunkSize int64 = 64 << 20
	return root.WriteFileStream(relPath, 0644, func(out *os.File) error {
		for offset := int64(0); offset < dir.DecompressedSize; {
			size := dir.DecompressedSize - offset
			if size > chunkSize {
				size = chunkSize
			}
			data, err := bundle.GetFileDataRangeByName(dir.Name, offset, size)
			if err != nil {
				return fmt.Errorf("read range [%d,%d): %w", offset, offset+size, err)
			}
			if int64(len(data)) != size {
				return fmt.Errorf("range [%d,%d) returned %d bytes", offset, offset+size, len(data))
			}
			written := 0
			for written < len(data) {
				n, err := out.Write(data[written:])
				if err != nil {
					return err
				}
				if n <= 0 {
					return fmt.Errorf("zero-progress write after %d of %d range bytes", written, len(data))
				}
				written += n
			}
			offset += size
		}
		return nil
	})
}

// unpackAssetsFile extracts the required representation of every user asset
// in one SerializedFile. Raw Unity objects and TextAsset scripts are required;
// PNG previews are best-effort derivatives and never replace the raw
// object as the source used for repacking.
func unpackAssetsFile(root *extractionRoot, sourceName string, bundle *aba.Bundle, af *aba.AssetsFile, containerNames map[int64]string, resolver aba.AssetResolver, streamResolver aba.BundleFileRangeResolver, claimedOutputPaths map[string]string) error {
	if root == nil {
		return fmt.Errorf("extract AssetsFile %q: nil extraction root", sourceName)
	}
	if af == nil {
		return fmt.Errorf("extract AssetsFile %q: nil AssetsFile", sourceName)
	}

	entries := af.GetAssetEntries()
	for _, entry := range entries {
		info := findInfo(af, entry.PathId)
		if info == nil {
			return fmt.Errorf("extract asset from %q: PathID %d (%s) has no AssetInfo", sourceName, entry.PathId, entry.TypeName)
		}

		// The AssetBundle object is the SerializedFile's structural m_Container
		// index. Its mapping was already decoded above and is represented by each
		// extracted object's loadName metadata; PackService regenerates it.
		if entry.TypeId == aba.ClassIDAssetBundle {
			continue
		}

		loadName := containerNames[entry.PathId]
		assetBaseName := entry.Name
		if assetBaseName == "" {
			assetBaseName = fmt.Sprintf("asset_%d", entry.PathId)
		}

		switch entry.TypeId {
		case aba.ClassIDTextAsset:
			name, script, err := af.GetTextAssetData(info)
			if err != nil {
				return fmt.Errorf("read TextAsset PathID %d from %q: %w", entry.PathId, sourceName, err)
			}
			if name == "" {
				name = assetBaseName
			}
			relPath := filepath.Join("TextAsset", sanitizeName(name))
			if err := claimExtractedAssetPaths(claimedOutputPaths, relPath, fmt.Sprintf("TextAsset PathID %d from %s", entry.PathId, sourceName), false); err != nil {
				return err
			}
			if err := writeExtractedAsset(root, relPath, script, entry, loadName, bundle, af, info, false); err != nil {
				return fmt.Errorf("write TextAsset %q (PathID %d) from %q: %w", name, entry.PathId, sourceName, err)
			}

		case aba.ClassIDTexture2D:
			assetData, err := af.GetAssetData(info)
			if err != nil {
				return fmt.Errorf("read Texture2D %q (PathID %d) from %q: %w", assetBaseName, entry.PathId, sourceName, err)
			}
			rawPath := filepath.Join("Texture2D", textureRawFileName(assetBaseName))
			if err := claimExtractedAssetPaths(claimedOutputPaths, rawPath, fmt.Sprintf("Texture2D PathID %d from %s", entry.PathId, sourceName), true); err != nil {
				return err
			}
			if err := writeExtractedAsset(root, rawPath, assetData, entry, loadName, bundle, af, info, true); err != nil {
				return fmt.Errorf("write Texture2D %q (PathID %d) from %q: %w", assetBaseName, entry.PathId, sourceName, err)
			}
			if tex, err := af.GetTexture2DDataRange(info, streamResolver); err == nil {
				pngPath := filepath.Join("Texture2D", sanitizeName(assetBaseName)+".png")
				if claimExtractionPaths(claimedOutputPaths, fmt.Sprintf("optional Texture2D preview PathID %d from %s", entry.PathId, sourceName), pngPath) == nil {
					_ = root.WriteFileStream(pngPath, 0644, func(f *os.File) error {
						return aba.WriteTexturePNGTo(tex, f)
					})
				}
			}

		case aba.ClassIDSprite:
			assetData, err := af.GetAssetData(info)
			if err != nil {
				return fmt.Errorf("read Sprite %q (PathID %d) from %q: %w", assetBaseName, entry.PathId, sourceName, err)
			}
			rawPath := filepath.Join("Sprite", sanitizeName(assetBaseName)+".sprite.bytes")
			if err := claimExtractedAssetPaths(claimedOutputPaths, rawPath, fmt.Sprintf("Sprite PathID %d from %s", entry.PathId, sourceName), true); err != nil {
				return err
			}
			if err := writeExtractedAsset(root, rawPath, assetData, entry, loadName, bundle, af, info, true); err != nil {
				return fmt.Errorf("write Sprite %q (PathID %d) from %q: %w", assetBaseName, entry.PathId, sourceName, err)
			}
			if sprite, err := af.GetSpriteExportRange(info, resolver, streamResolver); err == nil {
				pngPath := filepath.Join("Sprite", sanitizeName(assetBaseName)+".png")
				if claimExtractionPaths(claimedOutputPaths, fmt.Sprintf("optional Sprite preview PathID %d from %s", entry.PathId, sourceName), pngPath) == nil {
					_ = root.WriteFileStream(pngPath, 0644, func(f *os.File) error {
						return aba.WriteSpritePNGTo(sprite, f)
					})
				}
			}

		case aba.ClassIDMesh:
			assetData, err := af.GetAssetData(info)
			if err != nil {
				return fmt.Errorf("read Mesh %q (PathID %d) from %q: %w", assetBaseName, entry.PathId, sourceName, err)
			}
			rawPath := filepath.Join("Mesh", sanitizeName(assetBaseName)+".bytes")
			if err := claimExtractedAssetPaths(claimedOutputPaths, rawPath, fmt.Sprintf("Mesh PathID %d from %s", entry.PathId, sourceName), true); err != nil {
				return err
			}
			if err := writeExtractedAsset(root, rawPath, assetData, entry, loadName, bundle, af, info, true); err != nil {
				return fmt.Errorf("write Mesh %q (PathID %d) from %q: %w", assetBaseName, entry.PathId, sourceName, err)
			}
		default:
			assetData, err := af.GetAssetData(info)
			if err != nil {
				return fmt.Errorf("read %s asset %q (PathID %d) from %q: %w", entry.TypeName, assetBaseName, entry.PathId, sourceName, err)
			}
			typeName := entry.TypeName
			if typeName == "" {
				typeName = fmt.Sprintf("Type_%d", entry.TypeId)
			}
			relPath := filepath.Join(typeName, sanitizeName(assetBaseName)+".bytes")
			if err := claimExtractedAssetPaths(claimedOutputPaths, relPath, fmt.Sprintf("%s PathID %d from %s", typeName, entry.PathId, sourceName), true); err != nil {
				return err
			}
			if err := writeExtractedAsset(root, relPath, assetData, entry, loadName, bundle, af, info, true); err != nil {
				return fmt.Errorf("write %s asset %q (PathID %d) from %q: %w", typeName, assetBaseName, entry.PathId, sourceName, err)
			}
		}
	}
	return nil
}

func claimExtractedAssetPaths(claims map[string]string, relPath, owner string, includeTypeTree bool) error {
	paths := []string{relPath, relPath + ".meta.json"}
	if includeTypeTree {
		paths = append(paths, relPath+".typetree.json")
	}
	return claimExtractionPaths(claims, owner, paths...)
}

func claimExtractionPaths(claims map[string]string, owner string, paths ...string) error {
	if claims == nil {
		return nil
	}
	type claim struct {
		key  string
		path string
	}
	pending := make([]claim, 0, len(paths))
	for _, path := range paths {
		relPath, err := normalizeExtractionPath(path)
		if err != nil {
			return fmt.Errorf("invalid output path %q for %s: %w", path, owner, err)
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

func writeExtractedAsset(root *extractionRoot, relPath string, data []byte, entry aba.AssetEntry, loadName string, bundle *aba.Bundle, af *aba.AssetsFile, info *aba.AssetInfo, includeTypeTree bool) error {
	if err := root.WriteFile(relPath, data, 0644); err != nil {
		return err
	}
	metaData, err := marshalRawAssetMeta(sourceAssetMeta(entry.PathId, loadName, bundle, af))
	if err != nil {
		return fmt.Errorf("marshal asset metadata: %w", err)
	}
	if err := root.WriteFile(relPath+".meta.json", metaData, 0644); err != nil {
		return fmt.Errorf("write asset metadata: %w", err)
	}
	if includeTypeTree {
		typeTreeData, err := marshalRawUnityTypeTreeSidecar(af, info, entry, loadName)
		if err == nil {
			if err := root.WriteFile(relPath+".typetree.json", typeTreeData, 0644); err != nil {
				return fmt.Errorf("write TypeTree sidecar: %w", err)
			}
		}
	}
	return nil
}

func writeAssetMeta(assetPath string, pathID int64, loadName string) error {
	return writeRawAssetMeta(assetPath, rawAssetMeta{PathID: pathID, LoadName: loadName})
}

func writeSourceAssetMeta(assetPath string, pathID int64, loadName string, bundle *aba.Bundle, af *aba.AssetsFile) error {
	return writeRawAssetMeta(assetPath, sourceAssetMeta(pathID, loadName, bundle, af))
}

func sourceAssetMeta(pathID int64, loadName string, bundle *aba.Bundle, af *aba.AssetsFile) rawAssetMeta {
	meta := rawAssetMeta{PathID: pathID, LoadName: loadName}
	if bundle != nil {
		meta.EngineVersion = bundle.Header.EngineVersion
		meta.BundleVersion = bundle.Header.Version
		meta.GenerationVersion = bundle.Header.GenerationVersion
	}
	if af != nil {
		meta.UnityVersion = af.Metadata.UnityVersion
		meta.SerializedFileVersion = af.Header.Version
		targetPlatform := af.Metadata.TargetPlatform
		meta.TargetPlatform = &targetPlatform
	}
	return meta
}

func writeRawAssetMeta(assetPath string, meta rawAssetMeta) error {
	data, err := marshalRawAssetMeta(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(assetMetaPath(assetPath), data, 0644)
}

func marshalRawAssetMeta(meta rawAssetMeta) ([]byte, error) {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return data, nil
}

func assetMetaPath(assetPath string) string {
	return assetPath + ".meta.json"
}

func textureRawFileName(name string) string {
	safeName := sanitizeName(name)
	if filepath.Ext(safeName) == "" {
		return safeName + ".texture2d.bytes"
	}
	return safeName + ".bytes"
}

func findInfo(af *aba.AssetsFile, pathId int64) *aba.AssetInfo {
	for i, info := range af.Metadata.AssetInfos {
		if info.PathId == pathId {
			return &af.Metadata.AssetInfos[i]
		}
	}
	return nil
}

func sanitizeName(name string) string {
	// 替换跨平台文件系统不允许的字符，并规避 Windows 会折叠的
	// 结尾空格/点、设备名以及可被解释为目录的单点/双点名称。
	var result strings.Builder
	result.Grow(len(name))
	for _, c := range name {
		if c < 0x20 || strings.ContainsRune(`/\:*?"<>|`, c) {
			result.WriteByte('_')
		} else {
			result.WriteRune(c)
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
