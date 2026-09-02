package ct

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

func TestCatalogRejectsUnsupportedWidths(t *testing.T) {
	for _, fields := range [][]interface{}{
		{int64(0), int64(0), int64(0), int64(0), nil, nil, uint64(0), int64(0)},
		{int64(0), int64(0), int64(0), int64(0), nil, nil, uint64(0), int64(0), false, []interface{}{}, []interface{}{}, []interface{}{}, nil},
	} {
		wire, err := msgpack.EncodeMsgpack(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeCatalog(wire); err == nil || !strings.Contains(err.Error(), "indexed-array width") {
			t.Fatalf("unsupported catalog width error = %v", err)
		}
	}
}

func TestCatalogUsesTypedNullability(t *testing.T) {
	wire, err := msgpack.EncodeMsgpack([]interface{}{
		int64(1000), int64(CatalogTypeParts), int64(PackageTypePlugin), int64(0),
		nil, nil, uint64(0), int64(0), false,
		[]interface{}{"bundle.aba", nil},
		[]interface{}{nil, ".menuassets"},
		[]interface{}{nil, []interface{}{int64(0), nil, uint64(9)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := DecodeCatalog(wire)
	if err != nil {
		t.Fatalf("DecodeCatalog: %v", err)
	}
	if value.Name != nil || value.SubName != nil || value.ResourceFileNames[1] != nil || value.ExtensionList[0] != nil ||
		value.Items[0] != nil || value.Items[1] == nil || value.Items[1].Name != nil {
		t.Fatalf("typed nullability changed: %+v", value)
	}
	reencoded, err := EncodeCatalog(value)
	if err != nil {
		t.Fatalf("EncodeCatalog: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatalf("catalog wire changed: got %x want %x", reencoded, wire)
	}
}

func TestCatalogAndExtensionRejectRootTrailingData(t *testing.T) {
	wire := []byte{0xc0, 1, 2, 3}
	if _, err := DecodeCatalog(wire); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("catalog trailing-data error = %v", err)
	}
	if _, err := DecodeExtensionNameList(wire); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("extension trailing-data error = %v", err)
	}
}

func TestExtensionNameListRejectsUnsupportedWidths(t *testing.T) {
	for _, fields := range [][]interface{}{
		{nil},
		{nil, []interface{}{}, nil},
	} {
		wire, err := msgpack.EncodeMsgpack(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeExtensionNameList(wire); err == nil || !strings.Contains(err.Error(), "indexed-array width") {
			t.Fatalf("unsupported ExtensionNameList width error = %v", err)
		}
	}
}

func TestCatalogRoundTripsBothAssetBundleWidths(t *testing.T) {
	paths, err := filepath.Glob("../../../testdata/KCES/*.ct")
	if err != nil {
		t.Fatal(err)
	}
	kces2Paths, err := filepath.Glob("../../../testdata/KCES2/*.ct")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, kces2Paths...)
	if len(paths) == 0 {
		t.Skip("no .ct test files found")
	}

	widths := make(map[int32]int)
	sharedBundleNames, contentHashes := 0, 0
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", filepath.Base(path), err)
		}
		table, err := ReadContentTable(file)
		file.Close()
		if err != nil {
			t.Fatalf("read %s: %v", filepath.Base(path), err)
		}
		wire, err := decodeContentTableMessagePackFile(table, "catalog")
		if err != nil {
			continue
		}
		catalog, err := DecodeCatalog(wire)
		if err != nil {
			t.Fatalf("decode %s catalog: %v", filepath.Base(path), err)
		}
		if catalog == nil {
			continue
		}
		if catalog.Kind != CatalogKindAssetBundle {
			continue
		}
		widths[catalog.IndexedArrayWidth]++

		switch catalog.IndexedArrayWidth {
		case assetBundleCatalogLegacyWidth:
			if catalog.ResourceFileBuildAssetBundleNames != nil || catalog.ContentHash != nil {
				t.Fatalf("%s width %d carries version 1001 fields: %+v", filepath.Base(path), catalog.IndexedArrayWidth, catalog)
			}
		case assetBundleCatalogCurrentWidth:
			if catalog.ResourceFileBuildAssetBundleNames != nil {
				sharedBundleNames++
				if len(catalog.ResourceFileBuildAssetBundleNames) != len(catalog.ResourceFileNames) {
					t.Fatalf("%s has %d build names for %d resource files", filepath.Base(path), len(catalog.ResourceFileBuildAssetBundleNames), len(catalog.ResourceFileNames))
				}
			}
			if catalog.ContentHash != nil {
				contentHashes++
				if len(*catalog.ContentHash) != 64 {
					t.Fatalf("%s contentHash = %q", filepath.Base(path), *catalog.ContentHash)
				}
			}
		default:
			t.Fatalf("%s decoded width %d", filepath.Base(path), catalog.IndexedArrayWidth)
		}

		reencoded, err := EncodeCatalog(catalog)
		if err != nil {
			t.Fatalf("encode %s catalog: %v", filepath.Base(path), err)
		}
		if !bytes.Equal(reencoded, wire) {
			t.Fatalf("%s catalog wire changed: got %d bytes want %d bytes", filepath.Base(path), len(reencoded), len(wire))
		}

		editing, err := json.Marshal(catalog)
		if err != nil {
			t.Fatalf("marshal %s catalog: %v", filepath.Base(path), err)
		}
		var restored AssetBundleCatalog
		if err := json.Unmarshal(editing, &restored); err != nil {
			t.Fatalf("unmarshal %s catalog: %v", filepath.Base(path), err)
		}
		if !reflect.DeepEqual(&restored, catalog) {
			t.Fatalf("%s catalog changed through editing JSON", filepath.Base(path))
		}
	}

	if widths[assetBundleCatalogLegacyWidth] == 0 || widths[assetBundleCatalogCurrentWidth] == 0 {
		t.Fatalf("game samples do not cover both AssetBundleCatalog widths: %v", widths)
	}
	if sharedBundleNames == 0 || contentHashes == 0 {
		t.Fatalf("no game sample exercised the version 1001 fields: buildNames=%d contentHashes=%d", sharedBundleNames, contentHashes)
	}
	t.Logf("catalog widths %v with %d shared-bundle name lists and %d content hashes", widths, sharedBundleNames, contentHashes)
}

func TestCatalogRejectsVersion1001FieldsInLegacyWidth(t *testing.T) {
	name := "width.menuassets"
	catalogName := "width"
	resourceName := "width.aba"
	buildName := "width{1}.aba"
	contentHash := strings.Repeat("a", 64)
	base := func() *AssetBundleCatalog {
		return &AssetBundleCatalog{
			Kind:              CatalogKindAssetBundle,
			Version:           1001,
			CatalogType:       CatalogTypeParts,
			PackageType:       PackageTypePlugin,
			Name:              &catalogName,
			Hash:              HashStringIgnoreCase(resourceName),
			ResourceFileNames: []*string{&resourceName},
			Items:             []*CatalogItem{{ResourceIndex: 0, Name: &name, Hash: HashStringIgnoreCase(name)}},
		}
	}

	legacy := base()
	legacy.ContentHash = &contentHash
	if _, err := EncodeCatalog(legacy); err == nil || !strings.Contains(err.Error(), "contentHash requires indexedArrayWidth 14") {
		t.Fatalf("legacy width with contentHash error = %v", err)
	}
	legacy = base()
	legacy.ResourceFileBuildAssetBundleNames = []*string{&buildName}
	if _, err := EncodeCatalog(legacy); err == nil || !strings.Contains(err.Error(), "resourceFileBuildAssetBundleNames requires indexedArrayWidth 14") {
		t.Fatalf("legacy width with build names error = %v", err)
	}

	current := base()
	current.IndexedArrayWidth = assetBundleCatalogCurrentWidth
	current.ResourceFileBuildAssetBundleNames = []*string{&buildName}
	current.ContentHash = &contentHash
	wire, err := EncodeCatalog(current)
	if err != nil {
		t.Fatalf("EncodeCatalog: %v", err)
	}
	decoded, err := DecodeCatalog(wire)
	if err != nil {
		t.Fatalf("DecodeCatalog: %v", err)
	}
	if !reflect.DeepEqual(decoded, current) {
		t.Fatalf("catalog changed during round-trip:\ngot =%+v\nwant=%+v", decoded, current)
	}

	unknown := base()
	unknown.IndexedArrayWidth = 13
	if _, err := EncodeCatalog(unknown); err == nil || !strings.Contains(err.Error(), "is not an AssetBundleCatalog width known from the game") {
		t.Fatalf("unknown declared width error = %v", err)
	}

	virtual := &AssetBundleCatalog{
		Kind: CatalogKindVirtualAsset, Version: 1000, Name: &catalogName,
		VirtualItems: []*VirtualCatalogItem{{AssetPath: &name, Name: &name, Hash: HashStringIgnoreCase(name)}},
		ContentHash:  &contentHash,
	}
	if _, err := EncodeCatalog(virtual); err == nil || !strings.Contains(err.Error(), "AssetBundle-only fields") {
		t.Fatalf("virtualAsset catalog with contentHash error = %v", err)
	}
}
