package ct

import (
	"bytes"
	"reflect"
	"sort"
	"testing"
)

func TestDecodeCatalogSupportsTenSlotVirtualAssetCatalog(t *testing.T) {
	itemName := "local_test.menuassets"
	raw := []interface{}{
		int64(1000), int64(CatalogTypeParts), int64(PackageTypePlugin), int64(7),
		"local_test", "", HashStringIgnoreCase("local_test"), int64(123456789),
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
	catalog, err := DecodeCatalog(messagePack)
	if err != nil {
		t.Fatalf("DecodeCatalog: %v", err)
	}
	if catalog.Kind != CatalogKindVirtualAsset || catalog.Name == nil || *catalog.Name != "local_test" || len(catalog.ExtensionList) != 1 {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	if len(catalog.VirtualItems) != 1 || catalog.VirtualItems[0] == nil || catalog.VirtualItems[0].AssetPath == nil ||
		*catalog.VirtualItems[0].AssetPath != "Assets/GameData/parts/local_test/local_test.menuassets" {
		t.Fatalf("unexpected virtual items: %+v", catalog.VirtualItems)
	}
	if catalog.Items != nil || catalog.ResourceFileNames != nil || catalog.IsEncrypted {
		t.Fatalf("virtual catalog leaked assetBundle fields: %+v", catalog)
	}
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

	extension := ".menuassets"
	extensionList := &ExtensionNameList{
		Extension: &extension,
		Data: []*ExtensionNamePack{
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
	if err := table.AddFile("catalog", compressedCatalog); err != nil {
		t.Fatal(err)
	}
	if err := table.AddFile(extension, compressedExtension); err != nil {
		t.Fatal(err)
	}
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
	decodedExtension, err := DecodeExtensionNameListFromCt(roundTripTable, extension)
	if err != nil {
		t.Fatalf("DecodeExtensionNameListFromCt: %v", err)
	}
	if !reflect.DeepEqual(decodedExtension, extensionList) {
		t.Fatalf("extension list changed after CT round-trip\ngot:  %+v\nwant: %+v", decodedExtension, extensionList)
	}
}

func TestCatalogKindValidation(t *testing.T) {
	virtual := validVirtualCatalogForTest()
	if _, err := EncodeCatalog(virtual); err != nil {
		t.Fatalf("valid virtual catalog: %v", err)
	}

	unknown := *virtual
	unknown.Kind = CatalogKind("future")
	if _, err := EncodeCatalog(&unknown); err == nil {
		t.Fatal("unknown catalog kind was accepted")
	}

	bundleOnly := *virtual
	resourceName := "local.aba"
	bundleOnly.ResourceFileNames = []*string{&resourceName}
	if _, err := EncodeCatalog(&bundleOnly); err == nil {
		t.Fatal("virtual catalog accepted AssetBundle-only fields")
	}
}

func validVirtualCatalogForTest() *AssetBundleCatalog {
	paths := []string{
		"Assets/GameData/parts/local/a.menuassets",
		"Assets/GameData/parts/local/z.menuassets",
	}
	names := []string{"a.menuassets", "z.menuassets"}
	items := []*VirtualCatalogItem{
		{AssetPath: &paths[0], Name: &names[0], Hash: HashStringIgnoreCase(names[0])},
		{AssetPath: &paths[1], Name: &names[1], Hash: HashStringIgnoreCase(names[1])},
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Hash < items[j].Hash })
	name := "local"
	extension := ".menuassets"
	return &AssetBundleCatalog{
		Kind:          CatalogKindVirtualAsset,
		Version:       1000,
		CatalogType:   CatalogTypeParts,
		PackageType:   PackageTypePlugin,
		Priority:      3,
		Name:          &name,
		Hash:          HashStringIgnoreCase(name),
		CreateTime:    123456789,
		ExtensionList: []*string{&extension},
		VirtualItems:  items,
	}
}
