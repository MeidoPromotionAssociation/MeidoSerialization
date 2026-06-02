package aba

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Unity 类型 ID 常量（用于资源分类）
const (
	TypeIdGameObject    = 1
	TypeIdTransform     = 4
	TypeIdMaterial      = 21
	TypeIdMeshRenderer  = 23
	TypeIdTexture2D     = 28
	TypeIdMeshFilter    = 33
	TypeIdMesh          = 43
	TypeIdShader        = 48
	TypeIdTextAsset     = 49
	TypeIdAnimationClip = 74
	TypeIdMonoBehaviour = 114
	TypeIdMonoScript    = 115
	TypeIdSprite        = 213
	TypeIdSpriteAtlas   = 687078895
)

func TestReadAssetsFile(t *testing.T) {
	files := smallAbaTestFiles(t)

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			defer f.Close()

			bundle, err := ReadBundle(f)
			if err != nil {
				if isEncryptedError(err) {
					t.Skipf("skipping encrypted file: %v", err)
				}
				t.Fatalf("ReadBundle failed: %v", err)
			}

			// 解析 bundle 内的第一个 SerializedFile
			for i, dir := range bundle.BlockInfo.DirectoryInfos {
				if !dir.IsSerialized() {
					continue
				}
				data, err := bundle.GetFileData(i)
				if err != nil {
					t.Errorf("GetFileData failed: %v", err)
					continue
				}

				af, err := ReadAssetsFile(data)
				if err != nil {
					t.Errorf("ReadAssetsFile(%q) failed: %v", dir.Name, err)
					continue
				}

				t.Logf("AssetsFile: %s", dir.Name)
				t.Logf("  Unity Version: %s", af.Metadata.UnityVersion)
				t.Logf("  Format Version: %d", af.Header.Version)
				t.Logf("  Types: %d, Assets: %d", len(af.Metadata.TypeTreeTypes), len(af.Metadata.AssetInfos))

				// 统计每种类型的资源数量
				typeCount := make(map[int32]int)
				for _, asset := range af.Metadata.AssetInfos {
					typeCount[asset.TypeId]++
				}
				for tid, cnt := range typeCount {
					t.Logf("  Type %d: %d assets", tid, cnt)
				}

				// 验证可以读取每个资源的原始数据
				for j, info := range af.Metadata.AssetInfos {
					if _, err := af.GetAssetData(&info); err != nil {
						t.Errorf("GetAssetData(%d) failed: %v", j, err)
					}
				}
			}
		})
	}
}

func TestReadAssetsFile_Summary(t *testing.T) {
	files := smallAbaTestFiles(t)

	totalSuccess := 0
	totalAssets := 0
	for _, filePath := range files {
		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		bundle, err := ReadBundle(f)
		if err != nil {
			f.Close()
			continue
		}

		for i, dir := range bundle.BlockInfo.DirectoryInfos {
			if !dir.IsSerialized() {
				continue
			}
			data, err := bundle.GetFileData(i)
			if err != nil {
				fmt.Printf("  FAIL GetFileData %s/%s: %v\n", filepath.Base(filePath), dir.Name, err)
				continue
			}
			af, err := ReadAssetsFile(data)
			if err != nil {
				fmt.Printf("  FAIL %s/%s: %v\n", filepath.Base(filePath), dir.Name, err)
				continue
			}
			totalSuccess++
			totalAssets += len(af.Metadata.AssetInfos)
		}
		f.Close()
	}

	fmt.Printf("Successfully parsed %d AssetsFiles, %d total assets\n", totalSuccess, totalAssets)
	if totalSuccess == 0 {
		t.Error("failed to parse any AssetsFile")
	}
}
