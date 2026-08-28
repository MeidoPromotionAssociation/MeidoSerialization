package KCES

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
)

const assetBGExtension = ".asset_bg"

// AssetBGService 专门处理 COM3D2.5 背景资源使用且与 KCES ABA 共用 codec 的 .asset_bg UnityFS 文件 / AssetBGService handles COM3D2.5 .asset_bg UnityFS files that share the KCES ABA codec
type AssetBGService struct{}

// IsAssetBGFile 判断路径是否为原生 .asset_bg 文件
// IsAssetBGFile reports whether a path names a native .asset_bg file
func IsAssetBGFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), assetBGExtension)
}

// ReadAssetBG 直接打开并解析 .asset_bg UnityFS 文件，返回的文件句柄由调用者关闭
// ReadAssetBG directly opens and parses an .asset_bg UnityFS file, and the caller closes the returned file handle
func (s *AssetBGService) ReadAssetBG(path string) (*aba.Aba, *os.File, error) {
	if !IsAssetBGFile(path) {
		return nil, nil, fmt.Errorf("not an .asset_bg file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open .asset_bg file failed: %w", err)
	}
	bundle, err := aba.ReadAba(file)
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("parse .asset_bg file failed: %w", err)
	}
	return bundle, file, nil
}

// ListAssetBG 列出 .asset_bg 中所有 SerializedFile 的资源对象
// ListAssetBG lists resource objects from every SerializedFile in an .asset_bg file
func (s *AssetBGService) ListAssetBG(path string) ([]aba.AssetEntry, error) {
	bundle, file, err := s.ReadAssetBG(path)
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
			return nil, fmt.Errorf("read serialized .asset_bg entry %q at directory index %d: %w", directory.Name, directoryIndex, err)
		}
		assetsFile, err := aba.ReadAssetsFile(data)
		if err != nil {
			return nil, fmt.Errorf("parse serialized .asset_bg entry %q at directory index %d: %w", directory.Name, directoryIndex, err)
		}
		entries = append(entries, assetsFile.GetAssetEntries()...)
	}
	return entries, nil
}

// UnpackAssetBG 将受支持的 .asset_bg 提取为不含 sidecar 和外部流文件的纯资源目录
// UnpackAssetBG extracts a supported .asset_bg file into a plain resource directory without sidecars or external stream files
func (s *AssetBGService) UnpackAssetBG(path string, outDir string) error {
	return unpackUnityFSBundlePureDirectory(path, outDir, s.ReadAssetBG)
}
