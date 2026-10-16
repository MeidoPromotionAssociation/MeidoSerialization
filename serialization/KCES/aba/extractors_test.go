package aba

import (
	"encoding/binary"
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

			abaFile, err := ReadAba(f)
			if err != nil {
				if isEncryptedError(err) {
					t.Skipf("skipping encrypted file: %v", err)
				}
				t.Fatalf("ReadAba failed: %v", err)
			}

			for i, dir := range abaFile.BlockInfo.DirectoryInfos {
				if !dir.IsSerialized() {
					continue
				}
				data, err := abaFile.GetFileData(int64(i))
				if err != nil {
					continue
				}
				af, err := ReadAssetsFile(data)
				if err != nil {
					continue
				}

				entries := af.GetAssetEntries()
				t.Logf("Aba has %d entries:", len(entries))
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

// TestClassIdToNameAnimatorFamily 固定 Animator 系列 ID 与名字的对应，避免再次把 91/95 记反
// TestClassIdToNameAnimatorFamily pins the Animator family IDs to their names so 91/95 cannot be swapped again
func TestClassIdToNameAnimatorFamily(t *testing.T) {
	tests := []struct {
		classID int32
		want    string
	}{
		{ClassIDAnimatorController, "AnimatorController"},
		{ClassIDAnimator, "Animator"},
		{ClassIDAnimatorOverrideController, "AnimatorOverrideController"},
	}
	for _, test := range tests {
		if got := classIdToName(test.classID); got != test.want {
			t.Errorf("classIdToName(%d) = %q, want %q", test.classID, got, test.want)
		}
	}

	// 三个 ID 必须互不相同，且 91/95 不能被写成对方的常量值
	if ClassIDAnimatorController != 91 || ClassIDAnimator != 95 || ClassIDAnimatorOverrideController != 221 {
		t.Fatalf("Animator family IDs changed: controller=%d animator=%d override=%d",
			ClassIDAnimatorController, ClassIDAnimator, ClassIDAnimatorOverrideController)
	}

	// 未收录的 ID 仍必须退化为 Type_<id>，不能被误映射成已知类型
	if got := classIdToName(999999); got != "Type_999999" {
		t.Errorf("classIdToName(999999) = %q, want %q", got, "Type_999999")
	}
}

func TestGetTextAssetDataRejectsNegativeAlignedNameLength(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[:4], ^uint32(0))
	af := &AssetsFile{
		Header: AssetsFileHeader{Version: 22, FileSize: int64(len(data))},
		Data:   data,
	}
	info := &AssetInfo{TypeId: ClassIDTextAsset, ByteSize: uint32(len(data))}
	if _, _, err := af.GetTextAssetData(info); err == nil {
		t.Fatal("negative TextAsset m_Name length unexpectedly decoded")
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
