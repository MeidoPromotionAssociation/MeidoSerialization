package aba

import "testing"

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
