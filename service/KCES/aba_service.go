package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// AbaService 提供 .aba 文件的读取、列出和提取操作；文件内容使用 Unity AssetBundle UnityFS 格式。
//
// AbaService provides read, list, and extraction operations for .aba files using the Unity AssetBundle UnityFS format.
type AbaService struct{}

const (
	abaMetaFileName = ".meido-aba.meta.json"
	abaMetaFormat   = "kces-unityfs-aba"
)

// abaMeta preserves the UnityFS header context which cannot be inferred
// from a directory containing only non-serialized .aba entries.
type abaMeta struct {
	Format            string `json:"format"`
	AbaVersion        uint32 `json:"abaVersion"`
	GenerationVersion string `json:"generationVersion"`
	EngineVersion     string `json:"engineVersion"`
}

// rawAssetMeta 表示原始 Unity 对象的 sidecar 元数据。
//
// rawAssetMeta represents sidecar metadata for a raw Unity object.
type rawAssetMeta struct {
	PathID                int64   `json:"pathId"`                          // Unity PathID / Unity PathID
	LoadName              string  `json:"loadName,omitempty"`              // AssetBundle m_Container 加载名 / AssetBundle m_Container load name
	UnityVersion          string  `json:"unityVersion,omitempty"`          // SerializedFile 元数据中的 Unity 版本 / Unity version from SerializedFile metadata
	EngineVersion         string  `json:"engineVersion,omitempty"`         // UnityFS header 中的引擎版本 / Engine version from the UnityFS header
	TargetPlatform        *uint32 `json:"targetPlatform,omitempty"`        // SerializedFile 目标平台；指针用于区分缺失与显式 0 / SerializedFile target platform; pointer distinguishes absent from explicit zero
	AbaVersion            uint32  `json:"abaVersion,omitempty"`            // UnityFS 文件格式版本（KCES 样本为 7 或 8）/ UnityFS format version, 7 or 8 in KCES samples
	GenerationVersion     string  `json:"generationVersion,omitempty"`     // UnityFS generation version / UnityFS generation version
	SerializedFileVersion uint32  `json:"serializedFileVersion,omitempty"` // SerializedFile 格式版本 / SerializedFile format version
}

// ReadAba 读取 .aba 文件并返回 Aba。
//
// ReadAba reads an .aba file and returns its Aba representation.
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

// ListAba 列出 .aba 文件中的所有资源
func (s *AbaService) ListAba(path string) ([]aba.AssetEntry, error) {
	abaFile, f, err := s.ReadAba(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var allEntries []aba.AssetEntry
	for i, dir := range abaFile.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := abaFile.GetFileData(int64(i))
		if err != nil {
			return nil, fmt.Errorf("read serialized .aba entry %q at directory index %d: %w", dir.Name, i, err)
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return nil, fmt.Errorf("parse serialized .aba entry %q at directory index %d: %w", dir.Name, i, err)
		}
		allEntries = append(allEntries, af.GetAssetEntries()...)
	}
	return allEntries, nil
}

// UnpackAba 将 .aba 文件中的所有资源提取到指定目录
func (s *AbaService) UnpackAba(abaPath string, outDir string) error {
	abaFile, f, err := s.ReadAba(abaPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if outDir == "" {
		outDir = abaPath + "_unpacked"
	}
	rawAbaPaths := make(map[int64]string)
	claimedOutputPaths := make(map[string]string)
	if err := claimExtractionPaths(claimedOutputPaths, "UnityFS .aba metadata", abaMetaFileName); err != nil {
		return err
	}
	for i, dir := range abaFile.BlockInfo.DirectoryInfos {
		directoryIndex := int64(i)
		if dir.IsSerialized() {
			continue
		}
		relPath, err := normalizeExtractionPath(dir.Name)
		if err != nil {
			return fmt.Errorf("unsafe .aba entry name %q: %w", dir.Name, err)
		}
		if err := claimExtractionPaths(claimedOutputPaths, ".aba entry "+dir.Name, relPath); err != nil {
			return err
		}
		rawAbaPaths[directoryIndex] = relPath
	}

	serialized := make(map[int64]*aba.AssetsFile)
	serializedByName := make(map[string]*aba.AssetsFile)
	containerMaps := make(map[int64]map[int64]string)
	for i, dir := range abaFile.BlockInfo.DirectoryInfos {
		directoryIndex := int64(i)
		if !dir.IsSerialized() {
			continue
		}
		data, err := abaFile.GetFileData(directoryIndex)
		if err != nil {
			return fmt.Errorf("read file %q from .aba failed: %w", dir.Name, err)
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return fmt.Errorf("parse AssetsFile %q failed: %w", dir.Name, err)
		}
		serialized[directoryIndex] = af
		serializedByName[dir.Name] = af
		serializedByName[filepath.Base(dir.Name)] = af
		containerNames, err := af.GetAssetBundleContainerMap()
		if err != nil {
			return fmt.Errorf("read AssetBundle container map from %q: %w", dir.Name, err)
		}
		containerMaps[directoryIndex] = containerNames
	}
	resolver := aba.AbaAssetResolver(serializedByName)
	streamResolver := abaFile.GetFileDataRangeByName
	root, err := openExtractionRoot(outDir)
	if err != nil {
		return err
	}
	defer root.Close()
	abaMetaData, err := json.MarshalIndent(abaMeta{
		Format:            abaMetaFormat,
		AbaVersion:        abaFile.Header.Version,
		GenerationVersion: abaFile.Header.GenerationVersion,
		EngineVersion:     abaFile.Header.EngineVersion,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal UnityFS .aba metadata: %w", err)
	}
	abaMetaData = append(abaMetaData, '\n')
	if err := root.WriteFile(abaMetaFileName, abaMetaData, 0644); err != nil {
		return fmt.Errorf("write UnityFS .aba metadata: %w", err)
	}

	for i, dir := range abaFile.BlockInfo.DirectoryInfos {
		directoryIndex := int64(i)
		if !dir.IsSerialized() {
			// 非序列化文件（如多 GiB 的 .resS）使用范围 API 流式提取，
			// 避免 GetFileData 的单次内存上限和整文件分配。
			if err := writeRawAbaDirectory(root, abaFile, dir, rawAbaPaths[directoryIndex]); err != nil {
				return fmt.Errorf("write raw .aba entry %q: %w", dir.Name, err)
			}
			continue
		}

		af := serialized[directoryIndex]
		if af == nil {
			return fmt.Errorf("serialized .aba entry %q at directory index %d was not loaded", dir.Name, i)
		}
		if err := unpackAssetsFile(root, dir.Name, abaFile, af, containerMaps[directoryIndex], resolver, streamResolver, claimedOutputPaths); err != nil {
			return err
		}
	}
	return nil
}

func writeRawAbaDirectory(root *extractionRoot, abaFile *aba.Aba, dir aba.DirectoryInfo, relPath string) error {
	if root == nil || abaFile == nil {
		return fmt.Errorf("nil extraction root or .aba")
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
			data, err := abaFile.GetFileDataRangeByName(dir.Name, offset, size)
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
func unpackAssetsFile(root *extractionRoot, sourceName string, abaFile *aba.Aba, af *aba.AssetsFile, containerNames map[int64]string, resolver aba.AssetResolver, streamResolver aba.AbaFileRangeResolver, claimedOutputPaths map[string]string) error {
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
			if err := writeExtractedAsset(root, relPath, script, entry, loadName, abaFile, af, info, false); err != nil {
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
			if err := writeExtractedAsset(root, rawPath, assetData, entry, loadName, abaFile, af, info, true); err != nil {
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
			if err := writeExtractedAsset(root, rawPath, assetData, entry, loadName, abaFile, af, info, true); err != nil {
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
			if err := writeExtractedAsset(root, rawPath, assetData, entry, loadName, abaFile, af, info, true); err != nil {
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
			if err := writeExtractedAsset(root, relPath, assetData, entry, loadName, abaFile, af, info, true); err != nil {
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

func writeExtractedAsset(root *extractionRoot, relPath string, data []byte, entry aba.AssetEntry, loadName string, abaFile *aba.Aba, af *aba.AssetsFile, info *aba.AssetInfo, includeTypeTree bool) error {
	if err := root.WriteFile(relPath, data, 0644); err != nil {
		return err
	}
	metaData, err := marshalRawAssetMeta(sourceAssetMeta(entry.PathId, loadName, abaFile, af))
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

func writeSourceAssetMeta(assetPath string, pathID int64, loadName string, abaFile *aba.Aba, af *aba.AssetsFile) error {
	return writeRawAssetMeta(assetPath, sourceAssetMeta(pathID, loadName, abaFile, af))
}

func sourceAssetMeta(pathID int64, loadName string, abaFile *aba.Aba, af *aba.AssetsFile) rawAssetMeta {
	meta := rawAssetMeta{PathID: pathID, LoadName: loadName}
	if abaFile != nil {
		meta.EngineVersion = abaFile.Header.EngineVersion
		meta.AbaVersion = abaFile.Header.Version
		meta.GenerationVersion = abaFile.Header.GenerationVersion
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
