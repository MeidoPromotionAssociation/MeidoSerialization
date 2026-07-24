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
			catalogName := "<nil>"
			if cat.Name != nil {
				catalogName = *cat.Name
			}
			t.Logf("Catalog: version=%d type=%d pkg=%d priority=%d name=%q",
				cat.Version, cat.CatalogType, cat.PackageType, cat.Priority, catalogName)
			t.Logf("  ResourceFileNames: %v", cat.ResourceFileNames)
			t.Logf("  ExtensionList: %v", cat.ExtensionList)
			t.Logf("  Items: %d", len(cat.Items))

			for _, item := range cat.Items {
				if item == nil || item.Name == nil {
					t.Fatalf("catalog item or item name is null")
				}
				expectedHash := HashStringIgnoreCase(*item.Name)
				if item.Hash != expectedHash {
					t.Errorf("item %q: hash mismatch got=%d want=%d", *item.Name, item.Hash, expectedHash)
				}
			}

			// 验证每个 extensionList 条目都能在 .ct 中找到对应的 ExtensionNameList 文件
			for _, ext := range cat.ExtensionList {
				if ext == nil {
					t.Fatalf("catalog extension is null")
				}
				enl, err := DecodeExtensionNameListFromCt(table, *ext)
				if err != nil {
					t.Errorf("DecodeExtensionNameListFromCt(%q) failed: %v", *ext, err)
					continue
				}
				extensionName := "<nil>"
				if enl.Extension != nil {
					extensionName = *enl.Extension
				}
				t.Logf("  ExtensionNameList %q: extension=%q packs=%d", *ext, extensionName, len(enl.Data))

				for _, pack := range enl.Data {
					if pack == nil || pack.Name == nil {
						t.Fatalf("extension-name entry or name is null")
					}
					expectedHash := HashStringIgnoreCase(*pack.Name)
					if pack.Hash != expectedHash {
						t.Errorf("  pack %q: hash mismatch got=%d want=%d", *pack.Name, pack.Hash, expectedHash)
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

	// 与游戏 AssetManager.GetHashIgnoreCase 的 C# UTF-16 char 级实现对齐。
	// U+10000 以上字符必须按 surrogate pair 分别哈希，而不是按 Go rune 哈希。
	tests := map[string]uint64{
		"Test.menuassets":             8940223126534978995,
		"テスト.MENUASSETS":              17235742038489382243,
		"emoji_\U0001F600.menuassets": 6603427074542052611,
		"\U0001F600":                  5757361395789847182,
		"cjk_\U0002000B":              6750224190941313974,
	}
	for text, want := range tests {
		if got := HashStringIgnoreCase(text); got != want {
			t.Errorf("HashStringIgnoreCase(%q) got=%d want=%d", text, got, want)
		}
	}
}
