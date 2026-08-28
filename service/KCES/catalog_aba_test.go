package KCES

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
)

func TestCatalogItemsResolveToAbaAssets(t *testing.T) {
	ctFiles, err := filepath.Glob(filepath.Join("..", "..", "testdata", "KCES", "*.ct"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ctFiles) == 0 {
		t.Skip("no .ct samples found")
	}

	type catalogAbaCase struct {
		name      string
		catalog   *ct.AssetBundleCatalog
		resources []string
	}
	var cases []catalogAbaCase
	for _, ctPath := range ctFiles {
		f, err := os.Open(ctPath)
		if err != nil {
			t.Fatalf("open .ct: %v", err)
		}
		table, err := ct.ReadContentTable(f)
		f.Close()
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
		resources := make([]string, len(catalog.ResourceFileNames))
		missingResource := false
		for i, resourceName := range catalog.ResourceFileNames {
			if resourceName == nil {
				continue
			}
			resourcePath := filepath.Join(filepath.Dir(ctPath), *resourceName)
			if _, err := os.Stat(resourcePath); err != nil {
				missingResource = true
				break
			}
			resources[i] = resourcePath
		}
		if missingResource {
			continue
		}
		cases = append(cases, catalogAbaCase{
			name:      filepath.Base(ctPath),
			catalog:   catalog,
			resources: resources,
		})
	}
	if len(cases) == 0 {
		t.Skip("no catalog/resource pairs were available")
	}

	for _, sample := range cases {
		sample := sample
		t.Run(sample.name, func(t *testing.T) {
			resourceAssets := make([]map[uint64]int32, len(sample.resources))
			for i, resourcePath := range sample.resources {
				if resourcePath == "" {
					continue
				}
				assets, err := collectAbaAssetTypes(resourcePath)
				if err != nil {
					if isEncryptedAbaError(err) {
						t.Skipf("resource .aba file %s is encrypted and cannot be inspected: %v", filepath.Base(resourcePath), err)
					}
					t.Fatalf("collect assets from %s: %v", filepath.Base(resourcePath), err)
				}
				resourceAssets[i] = assets
			}

			for _, item := range sample.catalog.Items {
				if item == nil {
					continue
				}
				if item.ResourceIndex < 0 || int64(item.ResourceIndex) >= int64(len(resourceAssets)) {
					t.Fatalf("catalog item %q resourceIndex=%d out of bounds", testStringValue(item.Name), item.ResourceIndex)
				}
				if !catalogItemShouldResolveToAsset(item) {
					continue
				}
				if _, ok := resourceAssets[item.ResourceIndex][item.Hash]; !ok {
					t.Fatalf("catalog item %q not found in resource %q", testStringValue(item.Name), testStringValue(sample.catalog.ResourceFileNames[item.ResourceIndex]))
				}
			}
		})
	}
}

func collectAbaAssetTypes(path string) (map[uint64]int32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	abaFile, err := aba.ReadAba(f)
	if err != nil {
		return nil, err
	}
	assetTypes := map[uint64]int32{}
	for i, dir := range abaFile.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := abaFile.GetFileData(int64(i))
		if err != nil {
			return nil, err
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return nil, err
		}
		for assetIndex := range af.Metadata.AssetInfos {
			bundleInfo := &af.Metadata.AssetInfos[assetIndex]
			if bundleInfo.TypeId != aba.ClassIDAssetBundle {
				continue
			}
			containerEntries, err := af.GetAssetBundleContainerEntries(bundleInfo)
			if err != nil {
				return nil, fmt.Errorf("read AssetBundle container: %w", err)
			}
			for _, containerEntry := range containerEntries {
				if containerEntry.FileID != 0 || containerEntry.PathID == 0 || containerEntry.Name == "" {
					continue
				}
				target := af.GetAssetInfoByPathID(containerEntry.PathID)
				if target == nil {
					return nil, fmt.Errorf("AssetBundle container %q references missing PathID %d", containerEntry.Name, containerEntry.PathID)
				}
				assetTypes[ct.HashStringIgnoreCase(containerEntry.Name)] = target.TypeId
			}
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

func catalogItemShouldResolveToAsset(item *ct.CatalogItem) bool {
	if item == nil || item.Name == nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(*item.Name))
	return ext != "" && ext != ".null" && ext != ".meta"
}

func isEncryptedAbaError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "encrypted .aba file") || strings.Contains(msg, ".aba file is encrypted")
}
