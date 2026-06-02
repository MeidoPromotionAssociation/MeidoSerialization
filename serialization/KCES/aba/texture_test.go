package aba

import "testing"

func TestGetTexture2DData_Sample(t *testing.T) {
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
			if info.TypeId != ClassIDTexture2D {
				continue
			}
			tex, err := af.GetTexture2DDataRange(&info, bundle.GetFileDataRangeByName)
			if err != nil {
				if root, rootErr := af.ReadAssetValue(&info); rootErr == nil {
					for _, child := range root.Children {
						if child == nil {
							continue
						}
						t.Logf("field %s %s children=%d value=%T", child.TypeName, child.Name, len(child.Children), child.Value)
					}
				}
				t.Fatalf("GetTexture2DData pathId=%d: %v", info.PathId, err)
			}
			if tex.Name == "" || tex.Width <= 0 || tex.Height <= 0 || len(tex.ImageData) == 0 {
				t.Fatalf("bad texture data: name=%q size=%dx%d data=%d", tex.Name, tex.Width, tex.Height, len(tex.ImageData))
			}
			root, err := af.ReadAssetValue(&info)
			if err != nil {
				t.Fatalf("ReadAssetValue: %v", err)
			}
			imageData := root.Field("image data")
			if imageData == nil {
				imageData = root.Field("m_ImageData")
			}
			if imageData != nil {
				raw, ok := imageData.Bytes()
				if !ok {
					t.Fatalf("image data field did not expose bytes")
				}
				if len(imageData.Children) != 0 {
					t.Fatalf("byte array should not allocate per-byte children: bytes=%d children=%d", len(raw), len(imageData.Children))
				}
			}
			t.Logf("%s: %dx%d format=%d data=%d", tex.Name, tex.Width, tex.Height, tex.TextureFormat, len(tex.ImageData))
			return
		}
	}
	t.Fatal("no Texture2D found in sample")
}
