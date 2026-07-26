package ct

import (
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
)

func TestEncodeCatalog_PreservesGameLookupValues(t *testing.T) {
	valid := func() *AssetBundleCatalog {
		firstName := "a.menuassets"
		secondName := "z.menuassets"
		catalogName := "validation_test"
		resourceName := "validation_test.aba"
		extension := ".menuassets"
		first := CatalogItem{ResourceIndex: 0, Name: &firstName, Hash: HashStringIgnoreCase(firstName)}
		second := CatalogItem{ResourceIndex: 0, Name: &secondName, Hash: HashStringIgnoreCase(secondName)}
		if first.Hash > second.Hash {
			first, second = second, first
		}
		return &AssetBundleCatalog{
			Kind:              CatalogKindAssetBundle,
			Version:           1000,
			CatalogType:       CatalogTypeParts,
			PackageType:       PackageTypePlugin,
			Name:              &catalogName,
			Hash:              HashStringIgnoreCase("validation_test.aba"),
			ResourceFileNames: []*string{&resourceName},
			ExtensionList:     []*string{&extension},
			Items:             []*CatalogItem{&first, &second},
		}
	}

	encodedNil, err := EncodeCatalog(nil)
	if err != nil || !reflect.DeepEqual(encodedNil, []byte{0xc0}) {
		t.Fatalf("EncodeCatalog(nil) = %x, %v", encodedNil, err)
	}
	if decodedNil, err := DecodeCatalog(encodedNil); err != nil || decodedNil != nil {
		t.Fatalf("DecodeCatalog(nil root) = %#v, %v", decodedNil, err)
	}

	tests := []struct {
		name string
		edit func(*AssetBundleCatalog)
	}{
		{name: "unknown_catalog_bits", edit: func(c *AssetBundleCatalog) { c.CatalogType = 1 << 20 }},
		{name: "unknown_package", edit: func(c *AssetBundleCatalog) { c.PackageType = 99 }},
		{name: "no_resources", edit: func(c *AssetBundleCatalog) { c.ResourceFileNames = nil }},
		{name: "bad_resource_index", edit: func(c *AssetBundleCatalog) { c.Items[0].ResourceIndex = 1 }},
		{name: "wrong_item_hash", edit: func(c *AssetBundleCatalog) { c.Items[0].Hash++ }},
		{name: "unsorted_items", edit: func(c *AssetBundleCatalog) { c.Items[0], c.Items[1] = c.Items[1], c.Items[0] }},
		{name: "duplicate_extension", edit: func(c *AssetBundleCatalog) {
			first := ".menuassets"
			second := ".MENUASSETS"
			c.ExtensionList = []*string{&first, &second}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			catalog := valid()
			tc.edit(catalog)
			encoded, err := EncodeCatalog(catalog)
			if err != nil {
				t.Fatalf("EncodeCatalog rejected a wire-representable value: %v", err)
			}
			decoded, err := DecodeCatalog(encoded)
			if err != nil {
				t.Fatalf("DecodeCatalog: %v", err)
			}
			if !reflect.DeepEqual(decoded, catalog) {
				t.Fatalf("catalog changed during round-trip:\ngot =%+v\nwant=%+v", decoded, catalog)
			}
		})
	}
}

func TestEncodeCatalog_PreservesVersionWithoutMutatingInput(t *testing.T) {
	name := "canonical.menuassets"
	catalogName := "canonical"
	resourceName := "canonical.aba"
	catalog := &AssetBundleCatalog{
		Kind:              CatalogKindAssetBundle,
		CatalogType:       CatalogTypeParts,
		PackageType:       PackageTypePlugin,
		Name:              &catalogName,
		Hash:              HashStringIgnoreCase("canonical.aba"),
		ResourceFileNames: []*string{&resourceName},
		Items:             []*CatalogItem{{ResourceIndex: 0, Name: &name, Hash: HashStringIgnoreCase(name)}},
	}
	encoded, err := EncodeCatalog(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalog: %v", err)
	}
	if catalog.Version != 0 {
		t.Fatalf("EncodeCatalog mutated input version to %d", catalog.Version)
	}
	decoded, err := DecodeCatalog(encoded)
	if err != nil {
		t.Fatalf("DecodeCatalog: %v", err)
	}
	if decoded.Version != 0 {
		t.Fatalf("encoded version got %d, want preserved 0", decoded.Version)
	}
}

func TestCatalogDecodersRejectMalformedEntries(t *testing.T) {
	malformedCatalog, err := msgpack.EncodeMsgpack([]interface{}{
		int64(1000), int64(CatalogTypeParts), int64(PackageTypePlugin), int64(0),
		"bad", "", uint64(1), int64(0), false,
		[]interface{}{"bad.aba"}, []interface{}{".menuassets"},
		[]interface{}{[]interface{}{int64(0), "bad.menuassets", int64(-1)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCatalog(malformedCatalog); err == nil || !strings.Contains(err.Error(), "CatalogItem") || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("malformed catalog item error got %v", err)
	}

	malformedList, err := msgpack.EncodeMsgpack([]interface{}{".menuassets", []interface{}{[]interface{}{"bad.menuassets", int64(-1)}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeExtensionNameList(malformedList); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("malformed extension hash error got %v", err)
	}
}

func TestEncodeExtensionNameList_PreservesNamesAndHashes(t *testing.T) {
	encodedNil, err := EncodeExtensionNameList(nil)
	if err != nil || !reflect.DeepEqual(encodedNil, []byte{0xc0}) {
		t.Fatalf("EncodeExtensionNameList(nil) = %x, %v", encodedNil, err)
	}
	if decodedNil, err := DecodeExtensionNameList(encodedNil); err != nil || decodedNil != nil {
		t.Fatalf("DecodeExtensionNameList(nil root) = %#v, %v", decodedNil, err)
	}
	lists := []*ExtensionNameList{
		{
			Extension: catalogValidationStringPointer(".menuassets"),
			Data: []*ExtensionNamePack{
				{Name: catalogValidationStringPointer("test.menuassets"), Hash: HashStringIgnoreCase("test.menuassets") + 1},
				{Name: catalogValidationStringPointer("test.menuassets"), Hash: 0},
			},
		},
		{Extension: catalogValidationStringPointer(""), Data: nil},
	}
	for _, list := range lists {
		encoded, err := EncodeExtensionNameList(list)
		if err != nil {
			t.Fatalf("EncodeExtensionNameList rejected wire-representable values: %v", err)
		}
		decoded, err := DecodeExtensionNameList(encoded)
		if err != nil {
			t.Fatalf("DecodeExtensionNameList: %v", err)
		}
		if !reflect.DeepEqual(decoded, list) {
			t.Fatalf("extension-name list changed during round-trip:\ngot =%+v\nwant=%+v", decoded, list)
		}
	}
}

func catalogValidationStringPointer(value string) *string { return &value }
