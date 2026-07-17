package aba

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTryConvertMeshToCRMesh_Sample(t *testing.T) {
	bundle, f := openAbaSample(t, "parts_personal002.aba")
	defer f.Close()

	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		fileData, err := bundle.GetFileData(i)
		if err != nil {
			t.Fatalf("GetFileData: %v", err)
		}
		af, err := ReadAssetsFile(fileData)
		if err != nil {
			t.Fatalf("ReadAssetsFile: %v", err)
		}
		for _, info := range af.Metadata.AssetInfos {
			if info.TypeId != ClassIDMesh {
				continue
			}
			out, err := af.TryConvertMeshToCRMesh(&info, func(msg string) { t.Log(msg) })
			if err != nil {
				t.Fatalf("TryConvertMeshToCRMesh pathId=%d: %v", info.PathId, err)
			}
			if len(out) < 12 || out[0] != 11 || string(out[1:12]) != "CR_MOD_MESH" {
				t.Fatalf("invalid crmesh prefix: % x", out[:min(len(out), 12)])
			}
			t.Logf("crmesh bytes=%d", len(out))
			return
		}
	}
	t.Fatal("no Mesh found in sample")
}

func TestTryConvertMeshToCRMeshMatchesAbaExtractorArtifact(t *testing.T) {
	bundle, file := openAbaSample(t, "cm3d2_megane002.aba")
	defer file.Close()

	wantPath := filepath.Join("..", "..", "..", "testdata", "kces_assets", "cm3d2_megane002.mmesh.crmesh")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read AbaExtractor artifact: %v", err)
	}

	for directoryIndex, directory := range bundle.BlockInfo.DirectoryInfos {
		if !directory.IsSerialized() {
			continue
		}
		fileData, err := bundle.GetFileData(directoryIndex)
		if err != nil {
			t.Fatalf("GetFileData(%s): %v", directory.Name, err)
		}
		af, err := ReadAssetsFile(fileData)
		if err != nil {
			t.Fatalf("ReadAssetsFile(%s): %v", directory.Name, err)
		}
		for assetIndex := range af.Metadata.AssetInfos {
			info := &af.Metadata.AssetInfos[assetIndex]
			if info.TypeId != ClassIDMesh {
				continue
			}
			root, err := af.ReadAssetValue(info)
			if err != nil {
				t.Fatalf("read mesh PathID %d: %v", info.PathId, err)
			}
			name, _ := root.Field("m_Name").String()
			if name != "cm3d2_megane002.mmesh" {
				continue
			}
			got, err := af.TryConvertMeshToCRMesh(info, nil)
			if err != nil {
				t.Fatalf("convert mesh PathID %d: %v", info.PathId, err)
			}
			if !bytes.Equal(got, want) {
				firstDiff := 0
				for firstDiff < len(got) && firstDiff < len(want) && got[firstDiff] == want[firstDiff] {
					firstDiff++
				}
				t.Fatalf("CR_MOD_MESH differs from AbaExtractor artifact at byte %d: got %d bytes, want %d", firstDiff, len(got), len(want))
			}
			return
		}
	}
	t.Fatal("cm3d2_megane002 Mesh was not found")
}
