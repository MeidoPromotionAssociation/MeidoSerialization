package aba

import (
	"os"
	"testing"
)

func TestSpriteAtlasDebugSample(t *testing.T) {
	if os.Getenv("KCES_DEBUG_SPRITE") == "" {
		t.Skip("debug helper")
	}
	bundle, f := openAbaSample(t, "parts_personal002.aba")
	defer f.Close()
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		fileData, err := bundle.GetFileData(i)
		if err != nil {
			t.Fatal(err)
		}
		af, err := ReadAssetsFile(fileData)
		if err != nil {
			t.Fatal(err)
		}
		for _, info := range af.Metadata.AssetInfos {
			if info.TypeId != ClassIDSprite && info.TypeId != ClassIDSpriteAtlas {
				continue
			}
			root, err := af.ReadAssetValue(&info)
			if err != nil {
				t.Fatalf("ReadAssetValue %d: %v", info.TypeId, err)
			}
			name, _ := root.Field("m_Name").String()
			t.Logf("type=%d name=%s path=%d", info.TypeId, name, info.PathId)
			dumpValue(t, root, "  ", 0, 5)
		}
	}
}

func dumpValue(t *testing.T, v *TypeTreeValue, indent string, depth, maxDepth int) {
	if v == nil || depth > maxDepth {
		return
	}
	t.Logf("%s%s %s value=%T children=%d", indent, v.TypeName, v.Name, v.Value, len(v.Children))
	for _, child := range v.Children {
		dumpValue(t, child, indent+"  ", depth+1, maxDepth)
	}
}
