package ct

import (
	"bytes"
	"strings"
	"testing"
)

const (
	testInt32Min = int64(-1 << 31)
	testInt32Max = int64(1<<31 - 1)
)

func TestToIntUsesCSharpInt32Range(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  int
		ok    bool
	}{
		{name: "signed min", value: testInt32Min, want: int(testInt32Min), ok: true},
		{name: "signed max", value: testInt32Max, want: int(testInt32Max), ok: true},
		{name: "unsigned max", value: uint64(testInt32Max), want: int(testInt32Max), ok: true},
		{name: "signed underflow", value: testInt32Min - 1, ok: false},
		{name: "signed overflow", value: testInt32Max + 1, ok: false},
		{name: "unsigned overflow", value: uint64(testInt32Max + 1), ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := toInt(test.value)
			if ok != test.ok || ok && got != test.want {
				t.Fatalf("toInt(%v) = (%d,%v), want (%d,%v)", test.value, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestCatalogCSharpInt32WireBounds(t *testing.T) {
	virtual := validCatalogForInt32Test(CatalogKindVirtualAsset)
	virtualData, err := EncodeCatalog(virtual)
	if err != nil {
		t.Fatalf("EncodeCatalog virtual: %v", err)
	}
	var virtualRaw []interface{}
	if err := DecodeMsgpack(virtualData, &virtualRaw); err != nil {
		t.Fatalf("DecodeMsgpack virtual: %v", err)
	}
	for _, field := range []struct {
		name string
		slot int
	}{
		{name: "version", slot: 0},
		{name: "priority", slot: 3},
	} {
		for _, value := range []int64{testInt32Min - 1, testInt32Max + 1} {
			t.Run("decode "+field.name, func(t *testing.T) {
				raw := append([]interface{}(nil), virtualRaw...)
				raw[field.slot] = value
				data, err := EncodeMsgpack(raw)
				if err != nil {
					t.Fatalf("EncodeMsgpack: %v", err)
				}
				_, err = DecodeCatalog(data)
				if err == nil || !strings.Contains(err.Error(), "Int32") {
					t.Fatalf("DecodeCatalog error = %v, want Int32 rejection for %d", err, value)
				}
			})
		}
	}

	for _, value := range []int64{testInt32Min, testInt32Max} {
		t.Run("encode priority boundary", func(t *testing.T) {
			catalog := *virtual
			catalog.Priority = int(value)
			data, err := EncodeCatalog(&catalog)
			if err != nil {
				t.Fatalf("EncodeCatalog priority %d: %v", value, err)
			}
			decoded, err := DecodeCatalog(data)
			if err != nil {
				t.Fatalf("DecodeCatalog priority %d: %v", value, err)
			}
			if int64(decoded.Priority) != value {
				t.Fatalf("priority = %d, want %d", decoded.Priority, value)
			}
		})
	}
	if int64(hostIntForTest(testInt32Max+1)) == testInt32Max+1 {
		for _, value := range []int64{testInt32Min - 1, testInt32Max + 1} {
			t.Run("encode priority overflow", func(t *testing.T) {
				catalog := *virtual
				catalog.Priority = hostIntForTest(value)
				_, err := EncodeCatalog(&catalog)
				if err == nil || !strings.Contains(err.Error(), "Int32") {
					t.Fatalf("EncodeCatalog error = %v, want Int32 rejection for %d", err, value)
				}
			})
		}
	}

	bundle := validCatalogForInt32Test(CatalogKindAssetBundle)
	bundleData, err := EncodeCatalog(bundle)
	if err != nil {
		t.Fatalf("EncodeCatalog bundle: %v", err)
	}
	var bundleRaw []interface{}
	if err := DecodeMsgpack(bundleData, &bundleRaw); err != nil {
		t.Fatalf("DecodeMsgpack bundle: %v", err)
	}
	for _, value := range []int64{testInt32Min - 1, testInt32Max + 1} {
		t.Run("decode resourceIndex", func(t *testing.T) {
			raw := append([]interface{}(nil), bundleRaw...)
			items := append([]interface{}(nil), raw[11].([]interface{})...)
			item := append([]interface{}(nil), items[0].([]interface{})...)
			item[0] = value
			items[0] = item
			raw[11] = items
			data, err := EncodeMsgpack(raw)
			if err != nil {
				t.Fatalf("EncodeMsgpack: %v", err)
			}
			_, err = DecodeCatalog(data)
			if err == nil || !strings.Contains(err.Error(), "Int32") {
				t.Fatalf("DecodeCatalog error = %v, want Int32 resourceIndex rejection for %d", err, value)
			}
		})
	}
	if int64(hostIntForTest(testInt32Max+1)) == testInt32Max+1 {
		catalog := *bundle
		catalog.Items = append([]CatalogItem(nil), bundle.Items...)
		catalog.Items[0].ResourceIndex = hostIntForTest(testInt32Max + 1)
		_, err := EncodeCatalog(&catalog)
		if err == nil || !strings.Contains(err.Error(), "Int32") {
			t.Fatalf("EncodeCatalog error = %v, want Int32 resourceIndex rejection", err)
		}
	}
}

func TestVirtualDirectoryCSharpInt32WireBounds(t *testing.T) {
	t.Run("version max round-trip", func(t *testing.T) {
		table := &ContentTable{Version: int(testInt32Max)}
		var wire bytes.Buffer
		if err := WriteContentTable(&wire, table); err != nil {
			t.Fatalf("WriteContentTable: %v", err)
		}
		decoded, err := ReadContentTable(bytes.NewReader(wire.Bytes()))
		if err != nil {
			t.Fatalf("ReadContentTable: %v", err)
		}
		if decoded.Version != int(testInt32Max) {
			t.Fatalf("version = %d, want %d", decoded.Version, testInt32Max)
		}
	})

	if int64(hostIntForTest(testInt32Max+1)) == testInt32Max+1 {
		for _, value := range []int64{testInt32Min - 1, testInt32Max + 1} {
			t.Run("write version overflow", func(t *testing.T) {
				var wire bytes.Buffer
				err := WriteContentTable(&wire, &ContentTable{Version: hostIntForTest(value)})
				if err == nil || !strings.Contains(err.Error(), "Int32") {
					t.Fatalf("WriteContentTable error = %v, want Int32 rejection for %d", err, value)
				}
			})
		}
	}

	for _, value := range []int64{testInt32Min - 1, testInt32Max + 1} {
		t.Run("decode version overflow", func(t *testing.T) {
			data, err := EncodeMsgpack([]interface{}{value, map[string]interface{}{}, map[string]interface{}{}})
			if err != nil {
				t.Fatalf("EncodeMsgpack: %v", err)
			}
			table := &ContentTable{}
			err = table.decodeVirtualDirectory(data)
			if err == nil || !strings.Contains(err.Error(), "Int32") {
				t.Fatalf("decodeVirtualDirectory error = %v, want Int32 rejection for %d", err, value)
			}
		})
	}
}

func TestVirtualFileSizeCSharpInt32WireBounds(t *testing.T) {
	for _, value := range []int64{testInt32Min, testInt32Max} {
		t.Run("decode boundary", func(t *testing.T) {
			file, err := decodeVirtualFile([]interface{}{int64(0), value})
			if err != nil {
				t.Fatalf("decodeVirtualFile(%d): %v", value, err)
			}
			if int64(file.Size) != value {
				t.Fatalf("size = %d, want %d", file.Size, value)
			}
		})
	}
	for _, value := range []int64{testInt32Min - 1, testInt32Max + 1} {
		t.Run("decode overflow", func(t *testing.T) {
			_, err := decodeVirtualFile([]interface{}{int64(0), value})
			if err == nil || !strings.Contains(err.Error(), "Int32") {
				t.Fatalf("decodeVirtualFile error = %v, want Int32 rejection for %d", err, value)
			}
		})
	}

	if int64(hostIntForTest(testInt32Max+1)) == testInt32Max+1 {
		for _, value := range []int64{testInt32Min - 1, testInt32Max + 1} {
			t.Run("write overflow", func(t *testing.T) {
				table := &ContentTable{
					Version: 1000,
					Files:   map[string]VirtualFile{"oversized": {Position: 0, Size: hostIntForTest(value)}},
				}
				var wire bytes.Buffer
				err := WriteContentTable(&wire, table)
				if err == nil || !strings.Contains(err.Error(), "Int32") {
					t.Fatalf("WriteContentTable error = %v, want Int32 size rejection for %d", err, value)
				}
			})
		}
	}
}

// hostIntForTest keeps out-of-Int32 values non-constant so this test file also
// compiles on 32-bit Go hosts, where converting those constants directly to
// int would be a compile-time overflow. Callers verify the round trip before
// relying on the converted value.
func hostIntForTest(value int64) int {
	return int(value)
}

func validCatalogForInt32Test(kind CatalogKind) *AssetBundleCatalog {
	catalog := &AssetBundleCatalog{
		Kind:        kind,
		Version:     1000,
		CatalogType: CatalogTypeParts,
		PackageType: PackageTypePlugin,
		Name:        "int32",
	}
	if kind == CatalogKindVirtualAsset {
		return catalog
	}
	itemName := "int32.menuassets"
	catalog.ResourceFileNames = []string{"int32.aba"}
	catalog.Items = []CatalogItem{{ResourceIndex: 0, Name: itemName, Hash: HashStringIgnoreCase(itemName)}}
	return catalog
}
