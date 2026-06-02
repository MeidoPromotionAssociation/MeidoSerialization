package KCES

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestCatalogItemsResolveToAbaAssets(t *testing.T) {
	ctFiles, err := filepath.Glob(filepath.Join("..", "..", "testdata", "aba", "*.ct"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ctFiles) == 0 {
		t.Skip("no .ct samples found")
	}

	checked := 0
	for _, ctPath := range ctFiles {
		t.Run(filepath.Base(ctPath), func(t *testing.T) {
			f, err := os.Open(ctPath)
			if err != nil {
				t.Fatalf("open .ct: %v", err)
			}
			defer f.Close()

			table, err := ct.ReadContentTable(f)
			if err != nil {
				t.Fatalf("ReadContentTable: %v", err)
			}
			catalog, err := ct.DecodeCatalogFromCt(table)
			if err != nil {
				t.Fatalf("DecodeCatalogFromCt: %v", err)
			}
			if len(catalog.Items) == 0 {
				t.Fatalf("catalog has no items")
			}

			resourceAssets := make([]map[uint64]int32, len(catalog.ResourceFileNames))
			for i, resourceName := range catalog.ResourceFileNames {
				resourcePath := filepath.Join(filepath.Dir(ctPath), resourceName)
				if _, err := os.Stat(resourcePath); err != nil {
					t.Skipf("resource bundle %s not available: %v", resourcePath, err)
				}
				assets, err := collectAbaAssetTypes(resourcePath)
				if err != nil {
					if isEncryptedAbaError(err) {
						t.Skipf("resource bundle %s is encrypted and cannot be inspected: %v", resourceName, err)
					}
					t.Fatalf("collect assets from %s: %v", resourceName, err)
				}
				resourceAssets[i] = assets
			}

			for _, item := range catalog.Items {
				if item.ResourceIndex < 0 || item.ResourceIndex >= len(resourceAssets) {
					t.Fatalf("catalog item %q resourceIndex=%d out of bounds", item.Name, item.ResourceIndex)
				}
				if !catalogItemShouldResolveToAsset(item) {
					continue
				}
				if _, ok := resourceAssets[item.ResourceIndex][item.Hash]; !ok {
					t.Fatalf("catalog item %q not found in resource %q", item.Name, catalog.ResourceFileNames[item.ResourceIndex])
				}
			}
			checked++
		})
	}
	if checked == 0 {
		t.Skip("no catalog/resource pairs were available")
	}
}

func collectAbaAssetTypes(path string) (map[uint64]int32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bundle, err := aba.ReadBundle(f)
	if err != nil {
		return nil, err
	}
	assetTypes := map[uint64]int32{}
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(i)
		if err != nil {
			return nil, err
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return nil, err
		}
		for _, entry := range af.GetAssetEntries() {
			if entry.Name == "" {
				continue
			}
			assetTypes[ct.HashStringIgnoreCase(entry.Name)] = entry.TypeId
			if entry.TypeId == aba.ClassIDSpriteAtlas && !strings.HasSuffix(strings.ToLower(entry.Name), ".partsatlas") {
				assetTypes[ct.HashStringIgnoreCase(entry.Name+".partsatlas")] = entry.TypeId
			}
		}
	}
	return assetTypes, nil
}

func catalogItemShouldResolveToAsset(item ct.CatalogItem) bool {
	ext := strings.ToLower(filepath.Ext(item.Name))
	return ext != "" && ext != ".null"
}

func isEncryptedAbaError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "encrypted .aba file") || strings.Contains(msg, ".aba file is encrypted")
}
