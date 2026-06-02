package aba

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAssetEntries(t *testing.T) {
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

			for i, dir := range bundle.BlockInfo.DirectoryInfos {
				if !dir.IsSerialized() {
					continue
				}
				data, err := bundle.GetFileData(i)
				if err != nil {
					continue
				}
				af, err := ReadAssetsFile(data)
				if err != nil {
					continue
				}

				entries := af.GetAssetEntries()
				t.Logf("Bundle has %d entries:", len(entries))
				for _, e := range entries {
					t.Logf("  PathId=%d Type=%s Name=%q Size=%d", e.PathId, e.TypeName, e.Name, e.Size)
				}

				// 验证 TextAsset 解析
				for _, e := range entries {
					if e.TypeId != ClassIDTextAsset {
						continue
					}
					info := findAssetInfo(af, e.PathId)
					if info == nil {
						t.Errorf("AssetInfo not found for PathId=%d", e.PathId)
						continue
					}
					name, script, err := af.GetTextAssetData(info)
					if err != nil {
						t.Errorf("GetTextAssetData failed for PathId=%d: %v", e.PathId, err)
						continue
					}
					t.Logf("  TextAsset %q: %d bytes script", name, len(script))
				}
			}
		})
	}
}

func findAssetInfo(af *AssetsFile, pathId int64) *AssetInfo {
	for i, info := range af.Metadata.AssetInfos {
		if info.PathId == pathId {
			return &af.Metadata.AssetInfos[i]
		}
	}
	return nil
}
