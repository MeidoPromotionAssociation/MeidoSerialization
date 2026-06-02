package ct

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeCatalogFromCt(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/aba/*.ct")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no .ct test files found")
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			defer f.Close()

			table, err := ReadContentTable(f)
			if err != nil {
				t.Fatalf("ReadContentTable failed: %v", err)
			}

			cat, err := DecodeCatalogFromCt(table)
			if err != nil {
				t.Fatalf("DecodeCatalogFromCt failed: %v", err)
			}

			if cat.Version == 0 {
				t.Error("catalog version is 0")
			}
			t.Logf("Catalog: version=%d type=%d pkg=%d priority=%d name=%q",
				cat.Version, cat.CatalogType, cat.PackageType, cat.Priority, cat.Name)
			t.Logf("  ResourceFileNames: %v", cat.ResourceFileNames)
			t.Logf("  ExtensionList: %v", cat.ExtensionList)
			t.Logf("  Items: %d", len(cat.Items))

			for _, item := range cat.Items {
				expectedHash := HashStringIgnoreCase(item.Name)
				if item.Hash != expectedHash {
					t.Errorf("item %q: hash mismatch got=%d want=%d", item.Name, item.Hash, expectedHash)
				}
			}

			// 验证每个 extensionList 条目都能在 .ct 中找到对应的 ExtensionNameList 文件
			for _, ext := range cat.ExtensionList {
				enl, err := DecodeExtensionNameListFromCt(table, ext)
				if err != nil {
					t.Errorf("DecodeExtensionNameListFromCt(%q) failed: %v", ext, err)
					continue
				}
				t.Logf("  ExtensionNameList %q: extension=%q packs=%d", ext, enl.Extension, len(enl.Data))

				for _, pack := range enl.Data {
					expectedHash := HashStringIgnoreCase(pack.Name)
					if pack.Hash != expectedHash {
						t.Errorf("  pack %q: hash mismatch got=%d want=%d", pack.Name, pack.Hash, expectedHash)
					}
				}
			}
		})
	}
}

func TestHashStringIgnoreCase(t *testing.T) {
	// 验证空字符串
	if HashStringIgnoreCase("") != 0 {
		t.Error("empty string should hash to 0")
	}

	// 验证大小写不敏感
	h1 := HashStringIgnoreCase("Test.menuassets")
	h2 := HashStringIgnoreCase("test.menuassets")
	h3 := HashStringIgnoreCase("TEST.MENUASSETS")
	if h1 != h2 || h2 != h3 {
		t.Errorf("case insensitive hash failed: %d %d %d", h1, h2, h3)
	}

	// 验证不同字符串产生不同 hash
	ha := HashStringIgnoreCase("abc")
	hb := HashStringIgnoreCase("def")
	if ha == hb {
		t.Error("different strings should have different hashes")
	}
}
