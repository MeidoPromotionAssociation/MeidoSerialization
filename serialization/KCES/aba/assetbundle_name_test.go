package aba

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio"
)

// TestSerializedFileNameMatchesKnownVectors 固定 CAB 目录条目名算法，向量取自 Unity 官方测试包与 KCES 本体资源包
func TestSerializedFileNameMatchesKnownVectors(t *testing.T) {
	cases := []struct {
		assetBundleName string
		want            string
	}{
		// Unity 官方 UnityDataTools 测试包的 AssetBundle 对象 m_Name 为 assetbundle，基名不带后缀
		{"assetbundle", "CAB-5d40f7cad7c871cf2ad2af19ac542994"},
		// KCES 本体资源包，基名带 .aba 后缀
		{"parts.aba", "CAB-ba1a9170892fd14ab310868893203cbd"},
		{"bg.aba", "CAB-b769febcfc2ef27ea66c634c7477567c"},
		{"motion.aba", "CAB-12ed7c5157ada71f12000dc71106795d"},
		{"sound.aba", "CAB-5ab279ec5d4506637750d0d4bddb8e53"},
		{"language{639150577351412466}.aba", "CAB-c47a682f0ccd09998607d81e987685d5"},
	}
	for _, c := range cases {
		if got := SerializedFileName(c.assetBundleName); got != c.want {
			t.Errorf("SerializedFileName(%q) = %q, want %q", c.assetBundleName, got, c.want)
		}
	}
}

// TestSerializedFileNameMatchesSampleEntries 在每个可用样本上验证目录条目名等于 m_AssetBundleName 的 MD4 摘要
// 该断言只使用样本自身记录的基名，因此不依赖样本的来源
// TestSerializedFileNameMatchesSampleEntries verifies on every available sample that the directory entry name is the MD4 digest of m_AssetBundleName
// The assertion only uses the base name recorded inside the sample, so it does not depend on where the sample came from
func TestSerializedFileNameMatchesSampleEntries(t *testing.T) {
	for _, filePath := range smallAbaTestFiles(t) {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			abaFile, err := ReadAba(f)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "encrypted") {
					t.Skipf("encrypted sample: %v", err)
				}
				t.Fatalf("ReadAba: %v", err)
			}

			checked := 0
			for i := range abaFile.BlockInfo.DirectoryInfos {
				dir := &abaFile.BlockInfo.DirectoryInfos[i]
				if !dir.IsSerialized() {
					continue
				}
				data, err := abaFile.GetFileData(int64(i))
				if err != nil {
					t.Fatalf("GetFileData(%q): %v", dir.Name, err)
				}
				af, err := ReadAssetsFile(data)
				if err != nil {
					t.Fatalf("ReadAssetsFile(%q): %v", dir.Name, err)
				}
				for j := range af.Metadata.AssetInfos {
					info := &af.Metadata.AssetInfos[j]
					if info.TypeId != ClassIDAssetBundle {
						continue
					}
					baseName := sampleAssetBundleBaseName(t, af, info)
					if got := SerializedFileName(baseName); got != dir.Name {
						t.Fatalf("directory entry %q, want SerializedFileName(%q) = %q", dir.Name, baseName, got)
					}
					checked++
				}
			}
			if checked == 0 {
				t.Skip("sample has no AssetBundle object")
			}
		})
	}
}

// sampleAssetBundleBaseName 用样本自带的 TypeTree 解码 AssetBundle 对象的 m_AssetBundleName
func sampleAssetBundleBaseName(t *testing.T, af *AssetsFile, info *AssetInfo) string {
	t.Helper()
	var tree *TypeTreeType
	for i := range af.Metadata.TypeTreeTypes {
		candidate := &af.Metadata.TypeTreeTypes[i]
		if candidate.TypeId == ClassIDAssetBundle && len(candidate.Nodes) > 0 {
			tree = candidate
			break
		}
	}
	if tree == nil {
		t.Fatalf("AssetBundle TypeTree not found for PathID %d", info.PathId)
	}
	data, err := af.GetAssetData(info)
	if err != nil {
		t.Fatalf("GetAssetData(%d): %v", info.PathId, err)
	}
	r := binaryio.NewEndianReader(data, binary.LittleEndian)
	root, next, err := readTypeTreeValue(tree, r, 0)
	if err != nil {
		t.Fatalf("readTypeTreeValue(%d): %v", info.PathId, err)
	}
	if next != int64(len(tree.Nodes)) {
		t.Fatalf("AssetBundle TypeTree stopped at node %d/%d", next, len(tree.Nodes))
	}
	baseName, ok := root.Field("m_AssetBundleName").String()
	if !ok {
		t.Fatalf("AssetBundle PathID %d has no m_AssetBundleName", info.PathId)
	}
	return baseName
}
