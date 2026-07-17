package ct

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestDecodeCatalogSupportsTenSlotVirtualAssetCatalog(t *testing.T) {
	itemName := "local_test.menuassets"
	raw := []interface{}{
		int64(1000),
		int64(CatalogTypeParts),
		int64(PackageTypePlugin),
		int64(7),
		"local_test",
		"",
		HashStringIgnoreCase("local_test"),
		int64(123456789),
		[]interface{}{`.menuassets`},
		[]interface{}{
			[]interface{}{
				"Assets/GameData/parts/local_test/local_test.menuassets",
				itemName,
				HashStringIgnoreCase(itemName),
			},
		},
	}
	messagePack, err := EncodeMsgpack(raw)
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}

	assertCatalog := func(t *testing.T, catalog *AssetBundleCatalog) {
		t.Helper()
		if catalog.Kind != CatalogKindVirtualAsset {
			t.Fatalf("kind = %q, want %q", catalog.Kind, CatalogKindVirtualAsset)
		}
		if catalog.Name != "local_test" || len(catalog.ExtensionList) != 1 {
			t.Fatalf("unexpected catalog: %+v", catalog)
		}
		if len(catalog.VirtualItems) != 1 || catalog.VirtualItems[0].AssetPath != "Assets/GameData/parts/local_test/local_test.menuassets" {
			t.Fatalf("unexpected virtual items: %+v", catalog.VirtualItems)
		}
		if catalog.Items != nil || catalog.ResourceFileNames != nil || catalog.IsEncrypted {
			t.Fatalf("virtual catalog leaked assetBundle fields: %+v", catalog)
		}
	}

	t.Run("messagepack", func(t *testing.T) {
		catalog, err := DecodeCatalog(messagePack)
		if err != nil {
			t.Fatalf("DecodeCatalog: %v", err)
		}
		assertCatalog(t, catalog)
	})

	t.Run("content table", func(t *testing.T) {
		compressed, err := CompressLz4BlockArray(messagePack)
		if err != nil {
			t.Fatalf("CompressLz4BlockArray: %v", err)
		}
		table := &ContentTable{Version: 1000}
		table.AddFile("catalog", compressed)

		catalog, err := DecodeCatalogFromCt(table)
		if err != nil {
			t.Fatalf("DecodeCatalogFromCt: %v", err)
		}
		assertCatalog(t, catalog)
	})
}

func TestVirtualAssetCatalogSyntheticContentTableRoundTrip(t *testing.T) {
	catalog := validVirtualCatalogForTest()
	catalogData, err := EncodeCatalog(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalog: %v", err)
	}
	var catalogWire []interface{}
	if err := DecodeMsgpack(catalogData, &catalogWire); err != nil {
		t.Fatalf("DecodeMsgpack encoded catalog: %v", err)
	}
	if len(catalogWire) != 10 {
		t.Fatalf("virtual catalog wire slots = %d, want exactly 10", len(catalogWire))
	}

	extensionList := &ExtensionNameList{
		Extension: ".menuassets",
		Data: []ExtensionNamePack{
			{Name: catalog.VirtualItems[0].Name, Hash: catalog.VirtualItems[0].Hash},
			{Name: catalog.VirtualItems[1].Name, Hash: catalog.VirtualItems[1].Hash},
		},
	}
	extensionData, err := EncodeExtensionNameList(extensionList)
	if err != nil {
		t.Fatalf("EncodeExtensionNameList: %v", err)
	}
	compressedCatalog, err := CompressLz4BlockArray(catalogData)
	if err != nil {
		t.Fatalf("Compress catalog: %v", err)
	}
	compressedExtension, err := CompressLz4BlockArray(extensionData)
	if err != nil {
		t.Fatalf("Compress extension list: %v", err)
	}

	table := &ContentTable{Version: 1000}
	table.AddFile("catalog", compressedCatalog)
	table.AddFile(".menuassets", compressedExtension)
	var wire bytes.Buffer
	if err := WriteContentTable(&wire, table); err != nil {
		t.Fatalf("WriteContentTable: %v", err)
	}
	roundTripTable, err := ReadContentTable(bytes.NewReader(wire.Bytes()))
	if err != nil {
		t.Fatalf("ReadContentTable: %v", err)
	}
	decoded, err := DecodeCatalogFromCt(roundTripTable)
	if err != nil {
		t.Fatalf("DecodeCatalogFromCt: %v", err)
	}
	if !reflect.DeepEqual(decoded, catalog) {
		t.Fatalf("catalog changed after CT round-trip\ngot:  %+v\nwant: %+v", decoded, catalog)
	}
	decodedExtension, err := DecodeExtensionNameListFromCt(roundTripTable, ".menuassets")
	if err != nil {
		t.Fatalf("DecodeExtensionNameListFromCt: %v", err)
	}
	if !reflect.DeepEqual(decodedExtension, extensionList) {
		t.Fatalf("extension list changed after CT round-trip\ngot:  %+v\nwant: %+v", decodedExtension, extensionList)
	}
}

func TestAssetBundleCatalogLegacyJSONAndWireStayCompatible(t *testing.T) {
	name := "legacy.menuassets"
	legacy := &AssetBundleCatalog{
		Version:           1000,
		CatalogType:       CatalogTypeParts,
		PackageType:       PackageTypePlugin,
		Name:              "legacy",
		Hash:              HashStringIgnoreCase("legacy.aba"),
		ResourceFileNames: []string{"legacy.aba"},
		ExtensionList:     []string{".menuassets"},
		Items:             []CatalogItem{{ResourceIndex: 0, Name: name, Hash: HashStringIgnoreCase(name)}},
	}
	legacyJSON, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("Marshal legacy catalog: %v", err)
	}
	if bytes.Contains(legacyJSON, []byte(`"kind"`)) {
		t.Fatalf("zero-kind legacy JSON unexpectedly changed shape: %s", legacyJSON)
	}
	var fromLegacyJSON AssetBundleCatalog
	if err := json.Unmarshal(legacyJSON, &fromLegacyJSON); err != nil {
		t.Fatalf("Unmarshal legacy catalog: %v", err)
	}

	legacyWire, err := EncodeCatalog(&fromLegacyJSON)
	if err != nil {
		t.Fatalf("EncodeCatalog legacy kind: %v", err)
	}
	explicit := fromLegacyJSON
	explicit.Kind = CatalogKindAssetBundle
	explicitWire, err := EncodeCatalog(&explicit)
	if err != nil {
		t.Fatalf("EncodeCatalog explicit kind: %v", err)
	}
	if !bytes.Equal(legacyWire, explicitWire) {
		t.Fatal("legacy empty kind and explicit assetBundle kind produced different wire data")
	}
	var raw []interface{}
	if err := DecodeMsgpack(legacyWire, &raw); err != nil {
		t.Fatalf("DecodeMsgpack: %v", err)
	}
	if len(raw) != 12 {
		t.Fatalf("assetBundle catalog wire slots = %d, want exactly 12", len(raw))
	}
	decoded, err := DecodeCatalog(legacyWire)
	if err != nil {
		t.Fatalf("DecodeCatalog: %v", err)
	}
	if decoded.Kind != CatalogKindAssetBundle {
		t.Fatalf("decoded kind = %q, want %q", decoded.Kind, CatalogKindAssetBundle)
	}
}

func TestCatalogKindValidationAndIndexedWireMetadata(t *testing.T) {
	virtual := validVirtualCatalogForTest()
	bundleName := "bundle.menuassets"
	bundle := &AssetBundleCatalog{
		Kind:              CatalogKindAssetBundle,
		Version:           1000,
		CatalogType:       CatalogTypeParts,
		PackageType:       PackageTypePlugin,
		Name:              "bundle",
		Hash:              HashStringIgnoreCase("bundle.aba"),
		ResourceFileNames: []string{"bundle.aba"},
		ExtensionList:     []string{".menuassets"},
		Items:             []CatalogItem{{ResourceIndex: 0, Name: bundleName, Hash: HashStringIgnoreCase(bundleName)}},
	}

	tests := []struct {
		name string
		cat  func() *AssetBundleCatalog
		want string
	}{
		{
			name: "unknown kind",
			cat: func() *AssetBundleCatalog {
				value := *bundle
				value.Kind = CatalogKind("future")
				return &value
			},
			want: "unsupported catalog kind",
		},
		{
			name: "bundle with virtual items",
			cat: func() *AssetBundleCatalog {
				value := *bundle
				value.VirtualItems = virtual.VirtualItems
				return &value
			},
			want: "cannot contain virtualItems",
		},
		{
			name: "virtual with bundle items",
			cat: func() *AssetBundleCatalog {
				value := *virtual
				value.Items = bundle.Items
				return &value
			},
			want: "cannot contain assetBundle items",
		},
		{
			name: "virtual with resource names",
			cat: func() *AssetBundleCatalog {
				value := *virtual
				value.ResourceFileNames = []string{"local.aba"}
				return &value
			},
			want: "cannot contain resourceFileNames",
		},
		{
			name: "virtual encrypted",
			cat: func() *AssetBundleCatalog {
				value := *virtual
				value.IsEncrypted = true
				return &value
			},
			want: "cannot set isEncrypted",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := EncodeCatalog(test.cat())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("EncodeCatalog error = %v, want substring %q", err, test.want)
			}
		})
	}

	preserved := []struct {
		name string
		cat  func() *AssetBundleCatalog
	}{
		{
			name: "case-insensitive duplicate extension",
			cat: func() *AssetBundleCatalog {
				value := *virtual
				value.ExtensionList = []string{".menuassets", ".MENUASSETS"}
				return &value
			},
		},
		{
			name: "wrong virtual hash",
			cat: func() *AssetBundleCatalog {
				value := *virtual
				value.VirtualItems = append([]VirtualCatalogItem(nil), virtual.VirtualItems...)
				value.VirtualItems[0].Hash++
				return &value
			},
		},
		{
			name: "unsorted virtual items",
			cat: func() *AssetBundleCatalog {
				value := *virtual
				value.VirtualItems = append([]VirtualCatalogItem(nil), virtual.VirtualItems...)
				value.VirtualItems[0], value.VirtualItems[1] = value.VirtualItems[1], value.VirtualItems[0]
				return &value
			},
		},
	}
	for _, test := range preserved {
		t.Run("preserves "+test.name, func(t *testing.T) {
			catalog := test.cat()
			encoded, err := EncodeCatalog(catalog)
			if err != nil {
				t.Fatalf("EncodeCatalog rejected a wire-representable value: %v", err)
			}
			decoded, err := DecodeCatalog(encoded)
			if err != nil {
				t.Fatalf("DecodeCatalog: %v", err)
			}
			if !reflect.DeepEqual(decoded, catalog) {
				t.Fatalf("virtual catalog changed during round-trip:\ngot =%+v\nwant=%+v", decoded, catalog)
			}
		})
	}

	virtualWire, err := EncodeCatalog(virtual)
	if err != nil {
		t.Fatalf("EncodeCatalog virtual: %v", err)
	}
	var virtualRaw []interface{}
	if err := DecodeMsgpack(virtualWire, &virtualRaw); err != nil {
		t.Fatalf("DecodeMsgpack virtual: %v", err)
	}
	bundleWire, err := EncodeCatalog(bundle)
	if err != nil {
		t.Fatalf("EncodeCatalog bundle: %v", err)
	}
	var bundleRaw []interface{}
	if err := DecodeMsgpack(bundleWire, &bundleRaw); err != nil {
		t.Fatalf("DecodeMsgpack bundle: %v", err)
	}

	compatibleWires := []struct {
		name string
		raw  []interface{}
	}{
		{name: "nine slots", raw: append([]interface{}(nil), virtualRaw[:9]...)},
		{name: "eleven slots", raw: append(append([]interface{}(nil), virtualRaw...), nil)},
		{name: "thirteen slots", raw: append(append([]interface{}(nil), bundleRaw...), nil)},
	}
	virtualItem := append([]interface{}(nil), virtualRaw[9].([]interface{})[0].([]interface{})...)
	virtualItem = append(virtualItem, "extra")
	virtualItems := append([]interface{}(nil), virtualRaw[9].([]interface{})...)
	virtualItems[0] = virtualItem
	virtualExtraItem := append([]interface{}(nil), virtualRaw...)
	virtualExtraItem[9] = virtualItems
	compatibleWires = append(compatibleWires, struct {
		name string
		raw  []interface{}
	}{name: "virtual item extra slot", raw: virtualExtraItem})

	bundleItem := append([]interface{}(nil), bundleRaw[11].([]interface{})[0].([]interface{})...)
	bundleItem = append(bundleItem, "extra")
	bundleItems := append([]interface{}(nil), bundleRaw[11].([]interface{})...)
	bundleItems[0] = bundleItem
	bundleExtraItem := append([]interface{}(nil), bundleRaw...)
	bundleExtraItem[11] = bundleItems
	compatibleWires = append(compatibleWires, struct {
		name string
		raw  []interface{}
	}{name: "bundle item extra slot", raw: bundleExtraItem})

	for _, test := range compatibleWires {
		t.Run(test.name, func(t *testing.T) {
			data, err := EncodeMsgpack(test.raw)
			if err != nil {
				t.Fatalf("EncodeMsgpack: %v", err)
			}
			decoded, err := DecodeCatalog(data)
			if err != nil {
				t.Fatalf("DecodeCatalog rejected formatter-compatible wire: %v", err)
			}
			reencoded, err := EncodeCatalog(decoded)
			if err != nil {
				t.Fatalf("EncodeCatalog: %v", err)
			}
			again, err := DecodeCatalog(reencoded)
			if err != nil || !reflect.DeepEqual(again, decoded) {
				t.Fatalf("indexed metadata round trip = %#v, %v; want %#v", again, err, decoded)
			}
		})
	}
}

func validVirtualCatalogForTest() *AssetBundleCatalog {
	items := []VirtualCatalogItem{
		{
			AssetPath: "Assets/GameData/parts/local/a.menuassets",
			Name:      "a.menuassets",
			Hash:      HashStringIgnoreCase("a.menuassets"),
		},
		{
			AssetPath: "Assets/GameData/parts/local/z.menuassets",
			Name:      "z.menuassets",
			Hash:      HashStringIgnoreCase("z.menuassets"),
		},
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Hash < items[j].Hash })
	return &AssetBundleCatalog{
		Kind:          CatalogKindVirtualAsset,
		Version:       1000,
		CatalogType:   CatalogTypeParts,
		PackageType:   PackageTypePlugin,
		Priority:      3,
		Name:          "local",
		Hash:          HashStringIgnoreCase("local"),
		CreateTime:    123456789,
		ExtensionList: []string{".menuassets"},
		VirtualItems:  items,
	}
}
