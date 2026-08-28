package KCES

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
)

type catalogItemView struct {
	Name string
	Hash uint64
}

func decodeCatalogView(t *testing.T, path string) (*ct.AssetBundleCatalog, map[string][]catalogItemView) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	table, err := ct.ReadContentTable(f)
	if err != nil {
		t.Fatalf("read content table %s: %v", path, err)
	}
	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		t.Fatalf("decode catalog %s: %v", path, err)
	}
	lists := make(map[string][]catalogItemView)
	for _, extension := range catalog.ExtensionList {
		if extension == nil {
			t.Fatalf("%s: nil extension entry", path)
		}
		enl, err := ct.DecodeExtensionNameListFromCt(table, *extension)
		if err != nil {
			t.Fatalf("decode ExtensionNameList %q from %s: %v", *extension, path, err)
		}
		var views []catalogItemView
		for _, pack := range enl.Data {
			if pack == nil {
				continue
			}
			views = append(views, catalogItemView{Name: testStringValue(pack.Name), Hash: pack.Hash})
		}
		// Real catalogs order list entries by asset path while this library orders
		// them by hash, and the game accepts both, so compare order-insensitively.
		sort.Slice(views, func(i, j int) bool { return views[i].Hash < views[j].Hash })
		lists[*extension] = views
	}
	return catalog, lists
}

func catalogItemViews(catalog *ct.AssetBundleCatalog) []catalogItemView {
	var views []catalogItemView
	for _, item := range catalog.Items {
		if item == nil {
			continue
		}
		views = append(views, catalogItemView{Name: testStringValue(item.Name), Hash: item.Hash})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Hash < views[j].Hash })
	return views
}

// Official catalogs with known packaging mistakes that this library deliberately
// does not reproduce (stray desktop.ini, Unity .meta files, entries without a
// matching m_Container entry, or testdata pairs whose .ct references another .aba).
var knownIrregularCatalogSamples = map[string]string{
	"parts_bv001":                "official catalog lists crc2_underwear038 which has no m_Container entry in the .aba",
	"parts_personal_om015_gp003": "official catalog lists a stray desktop(.ini) entry not present in the .aba",
	"parts_stream001":            "official catalog lists Unity .meta editor files that are not real assets",
	"parts_dlc573_gp003":         "testdata .ct references parts_dlc577_gp003.aba, mismatching its own file name",
}

func TestGenerateCtFromAbaMatchesRealCatalogs(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "KCES")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("testdata/KCES unavailable: %v", err)
	}
	tested := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".aba") {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		realCtPath := filepath.Join(dir, base+".ct")
		if _, err := os.Stat(realCtPath); err != nil {
			continue
		}
		tested++
		t.Run(base, func(t *testing.T) {
			if reason, ok := knownIrregularCatalogSamples[base]; ok {
				t.Skip(reason)
			}
			outPath := filepath.Join(t.TempDir(), base+".ct")
			if err := (&CtService{}).GenerateCtFromAba(filepath.Join(dir, name), outPath); err != nil {
				if strings.Contains(err.Error(), "encrypted") {
					t.Skipf("sample is encrypted: %v", err)
				}
				t.Fatalf("GenerateCtFromAba: %v", err)
			}
			gotCatalog, gotLists := decodeCatalogView(t, outPath)
			wantCatalog, wantLists := decodeCatalogView(t, realCtPath)

			if gotCatalog.Hash != ct.HashStringIgnoreCase(base+".aba") {
				t.Errorf("generated catalog hash got %d, want hash of %q", gotCatalog.Hash, base+".aba")
			}
			// The game only uses the top-level hash as a patch dedup key. The
			// official KCES pipeline hashes "<name>.aba" while the COM3D2
			// converter pipeline hashes the bare name, so accept either.
			if wantCatalog.Hash != ct.HashStringIgnoreCase(base+".aba") && wantCatalog.Hash != ct.HashStringIgnoreCase(base) {
				t.Errorf("real catalog hash %d matches neither known scheme for %q", wantCatalog.Hash, base)
			}
			if got, want := testStringValues(gotCatalog.ResourceFileNames), testStringValues(wantCatalog.ResourceFileNames); !equalStringSlices(got, want) {
				t.Errorf("resource file names got %v, want %v", got, want)
			}
			if got, want := catalogItemViews(gotCatalog), catalogItemViews(wantCatalog); !equalItemViews(got, want) {
				t.Errorf("catalog items got %d entries, want %d entries\n got: %v\nwant: %v", len(got), len(want), got, want)
			}
			if got, want := testStringValues(gotCatalog.ExtensionList), testStringValues(wantCatalog.ExtensionList); !equalStringSlices(got, want) {
				t.Errorf("extension list got %v, want %v", got, want)
			}
			for extension, want := range wantLists {
				if !equalItemViews(gotLists[extension], want) {
					t.Errorf("ExtensionNameList %q got %v, want %v", extension, gotLists[extension], want)
				}
			}
		})
	}
	if tested == 0 {
		t.Skip("no paired .aba/.ct samples found")
	}
}

func TestGenerateCtFromAbaDefaultOutputPath(t *testing.T) {
	sourceDir := filepath.Join("..", "..", "testdata", "KCES")
	sourceEntries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Skipf("testdata/KCES unavailable: %v", err)
	}
	var sample string
	for _, entry := range sourceEntries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".aba") {
			sample = entry.Name()
			break
		}
	}
	if sample == "" {
		t.Skip("no .aba samples found")
	}
	data, err := os.ReadFile(filepath.Join(sourceDir, sample))
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	abaPath := filepath.Join(tmpDir, sample)
	if err := os.WriteFile(abaPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := (&CtService{}).GenerateCtFromAba(abaPath, ""); err != nil {
		t.Fatalf("GenerateCtFromAba: %v", err)
	}
	base := strings.TrimSuffix(sample, filepath.Ext(sample))
	if _, err := os.Stat(filepath.Join(tmpDir, base+".ct")); err != nil {
		t.Fatalf("default output %s.ct missing: %v", base, err)
	}
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalItemViews(a []catalogItemView, b []catalogItemView) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
