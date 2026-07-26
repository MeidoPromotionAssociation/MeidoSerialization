package KCES

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// AssetSceneService 专门处理 KCES 场景资源使用的 .asset_scene UnityFS 文件 / AssetSceneService handles .asset_scene UnityFS files used by KCES scene resources
type AssetSceneService struct{}

// IsKCESAssetSceneFile 判断路径是否为原生 .asset_scene 文件
// IsKCESAssetSceneFile reports whether a path names a native .asset_scene file
func IsKCESAssetSceneFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), aba.AssetSceneExtension)
}

// ReadAssetScene 直接打开并解析 .asset_scene UnityFS 文件，返回的文件句柄由调用者关闭
// ReadAssetScene directly opens and parses an .asset_scene UnityFS file, and the caller closes the returned file handle
func (s *AssetSceneService) ReadAssetScene(path string) (*aba.Aba, *os.File, error) {
	if !IsKCESAssetSceneFile(path) {
		return nil, nil, fmt.Errorf("not an .asset_scene file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open .asset_scene file failed: %w", err)
	}
	bundle, err := aba.ReadAba(file)
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("parse .asset_scene file failed: %w", err)
	}
	return bundle, file, nil
}

// ListAssetScene 列出 .asset_scene 中所有 SerializedFile 的资源对象
// ListAssetScene lists resource objects from every SerializedFile in an .asset_scene file
func (s *AssetSceneService) ListAssetScene(path string) ([]aba.AssetEntry, error) {
	bundle, file, err := s.ReadAssetScene(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var entries []aba.AssetEntry
	for directoryIndex, directory := range bundle.BlockInfo.DirectoryInfos {
		if !directory.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(int64(directoryIndex))
		if err != nil {
			return nil, fmt.Errorf("read serialized .asset_scene entry %q at directory index %d: %w", directory.Name, directoryIndex, err)
		}
		assetsFile, err := aba.ReadAssetsFile(data)
		if err != nil {
			return nil, fmt.Errorf("parse serialized .asset_scene entry %q at directory index %d: %w", directory.Name, directoryIndex, err)
		}
		entries = append(entries, assetsFile.GetAssetEntries()...)
	}
	return entries, nil
}

// UnpackAssetScene 将受支持的 .asset_scene 提取为不含 sidecar 和外部流文件的纯资源目录
// UnpackAssetScene extracts a supported .asset_scene file into a plain resource directory without sidecars or external stream files
func (s *AssetSceneService) UnpackAssetScene(path string, outDir string) error {
	return unpackUnityFSBundlePureDirectory(path, outDir, s.ReadAssetScene)
}
