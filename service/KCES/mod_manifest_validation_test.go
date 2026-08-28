package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
)

func TestParseCatalogTypeCoversGameEnumAndFlags(t *testing.T) {
	tests := map[string]ct.CatalogType{
		"Unknown":          ct.CatalogTypeUnknown,
		"Language":         ct.CatalogTypeLanguage,
		"Product":          ct.CatalogTypeProduct,
		"Movie":            ct.CatalogTypeMovie,
		"Script":           ct.CatalogTypeScript,
		"Sound":            ct.CatalogTypeSound,
		"Voice":            ct.CatalogTypeVoice,
		"Csv":              ct.CatalogTypeCsv,
		"System":           ct.CatalogTypeSystem,
		"Bg":               ct.CatalogTypeBg,
		"Motion":           ct.CatalogTypeMotion,
		"PartsMeta":        ct.CatalogTypePartsMeta,
		"Parts":            ct.CatalogTypeParts,
		"Parts|PartsMeta":  ct.CatalogTypeParts | ct.CatalogTypePartsMeta,
		"Sound, Voice":     ct.CatalogTypeSound | ct.CatalogTypeVoice,
		"Language+Product": ct.CatalogTypeLanguage | ct.CatalogTypeProduct,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := parseCatalogType(input)
			if err != nil {
				t.Fatalf("parseCatalogType: %v", err)
			}
			if got != want {
				t.Fatalf("parseCatalogType(%q) got %d, want %d", input, got, want)
			}
		})
	}
	for _, input := range []string{"", "Parts|Parts", "not-a-type"} {
		if _, err := parseCatalogType(input); err == nil {
			t.Errorf("parseCatalogType(%q) unexpectedly succeeded", input)
		}
	}
}

func TestParsePackageTypeCoversGameEnum(t *testing.T) {
	tests := map[string]ct.CatalogPackageType{
		"Base":        ct.PackageTypeBase,
		"Plugin":      ct.PackageTypePlugin,
		"PluginPatch": ct.PackageTypePluginPatch,
		"BasePatch":   ct.PackageTypeBasePatch,
		"ExtraBase":   ct.PackageTypeExtraBase,
		"ExtraPatch":  ct.PackageTypeExtraPatch,
	}
	for input, want := range tests {
		got, err := parsePackageType(input)
		if err != nil || got != want {
			t.Errorf("parsePackageType(%q) got %d, %v; want %d", input, got, err, want)
		}
	}
	for _, input := range []string{"", "plugni"} {
		if _, err := parsePackageType(input); err == nil {
			t.Errorf("parsePackageType(%q) unexpectedly succeeded", input)
		}
	}
}

func TestPackModManifestRejectsInvalidManifestBeforeWriting(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "first.bin"), []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "second.bin"), []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		manifest ModManifest
		want     string
	}{
		{
			name: "unsafe_output_name",
			manifest: ModManifest{Name: "../escape", CatalogType: "Parts", PackageType: "Plugin",
				Assets: []ModAsset{{Name: "one.menuassets", Path: "first.bin", Kind: "textasset"}}},
			want: "invalid name",
		},
		{
			name: "unknown_catalog_type",
			manifest: ModManifest{Name: "bad_catalog", CatalogType: "Prats", PackageType: "Plugin",
				Assets: []ModAsset{{Name: "one.menuassets", Path: "first.bin", Kind: "textasset"}}},
			want: "unsupported catalogType",
		},
		{
			name: "unknown_package_type",
			manifest: ModManifest{Name: "bad_package", CatalogType: "Parts", PackageType: "Plugni",
				Assets: []ModAsset{{Name: "one.menuassets", Path: "first.bin", Kind: "textasset"}}},
			want: "unsupported packageType",
		},
		{
			name: "patch_without_subname",
			manifest: ModManifest{Name: "bad_patch", CatalogType: "Parts", PackageType: "PluginPatch",
				Assets: []ModAsset{{Name: "one.menuassets", Path: "first.bin", Kind: "textasset"}}},
			want: "subName is required",
		},
		{
			name: "unknown_asset_kind",
			manifest: ModManifest{Name: "bad_kind", CatalogType: "Parts", PackageType: "Plugin",
				Assets: []ModAsset{{Name: "one.menuassets", Path: "first.bin", Kind: "sprtie"}}},
			want: "unsupported kind",
		},
		{
			name: "duplicate_catalog_name",
			manifest: ModManifest{Name: "duplicate_name", CatalogType: "Parts", PackageType: "Plugin",
				Assets: []ModAsset{
					{Name: "Same.menuassets", Path: "first.bin", Kind: "textasset"},
					{Name: "same.MENUASSETS", Path: "second.bin", Kind: "textasset"},
				}},
			want: "same case-insensitive catalog hash",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := packModManifest(tc.manifest, tmpDir, tmpDir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("packModManifest error got %v, want %q", err, tc.want)
			}
			for _, extension := range []string{".ct", ".aba"} {
				path := filepath.Join(tmpDir, filepath.Base(tc.manifest.Name)+extension)
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("invalid manifest left output %q (stat error %v)", path, statErr)
				}
			}
		})
	}
}

func TestPackModManifestPreservesPatchSubName(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "patch.bin"), []byte("patch"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := ModManifest{
		Name:        "parts_validation_2",
		SubName:     "validation",
		CatalogType: "Parts",
		PackageType: "PluginPatch",
		Priority:    2,
		Assets: []ModAsset{{
			Name: "patch.menuassets", Path: "patch.bin", Kind: "textasset",
		}},
	}
	if err := packModManifest(manifest, tmpDir, tmpDir); err != nil {
		t.Fatalf("packModManifest: %v", err)
	}
	f, err := os.Open(filepath.Join(tmpDir, manifest.Name+".ct"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	table, err := ct.ReadContentTable(f)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.PackageType != ct.PackageTypePluginPatch || testStringValue(catalog.SubName) != manifest.SubName {
		t.Fatalf("patch catalog got package=%d subName=%q", catalog.PackageType, testStringValue(catalog.SubName))
	}
}

func TestPackModManifestWritesFixedUnityContract(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "asset.bin"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := ModManifest{
		Name:        "manifest_version",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{{
			Name: "asset.menuassets", Path: "asset.bin", Kind: "textasset",
		}},
	}
	if err := packModManifest(manifest, tmpDir, tmpDir); err != nil {
		t.Fatalf("packModManifest: %v", err)
	}
	abaBytes, err := os.ReadFile(filepath.Join(tmpDir, manifest.Name+".aba"))
	if err != nil {
		t.Fatal(err)
	}
	abaFile, err := aba.ReadAba(bytes.NewReader(abaBytes))
	if err != nil {
		t.Fatal(err)
	}
	if abaFile.Header.Version != defaultKCESAbaVersion || abaFile.Header.EngineVersion != defaultKCESUnityVersion {
		t.Fatalf(".aba header got version=%d engine=%q", abaFile.Header.Version, abaFile.Header.EngineVersion)
	}
	serialized, err := abaFile.GetFileData(0)
	if err != nil {
		t.Fatal(err)
	}
	af, err := aba.ReadAssetsFile(serialized)
	if err != nil {
		t.Fatal(err)
	}
	if af.Metadata.UnityVersion != defaultKCESUnityVersion || af.Metadata.TargetPlatform != defaultKCESTargetPlatform {
		t.Fatalf("SerializedFile UnityVersion got %q", af.Metadata.UnityVersion)
	}
}

func TestPackModManifestIgnoresLegacyPreferredPathID(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{"first.bin", "second.bin"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, name+".meta.json"), []byte(`{"pathId":42}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := ModManifest{
		Name:        "duplicate_path_id",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{
			{Name: "first.menuassets", Path: "first.bin", Kind: "textasset"},
			{Name: "second.menuassets", Path: "second.bin", Kind: "textasset"},
		},
	}
	if err := packModManifest(manifest, tmpDir, tmpDir); err != nil {
		t.Fatalf("packModManifest: %v", err)
	}
	bundleBytes, err := os.ReadFile(filepath.Join(tmpDir, manifest.Name+".aba"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadAba(bytes.NewReader(bundleBytes))
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatal(err)
	}
	af, err := aba.ReadAssetsFile(serialized)
	if err != nil {
		t.Fatal(err)
	}
	want, err := buildCanonicalPathIDs([]string{"first.bin", "second.bin"})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int64]bool)
	for _, info := range af.Metadata.AssetInfos {
		if info.TypeId == aba.ClassIDTextAsset {
			seen[info.PathId] = true
		}
	}
	if !seen[want["first.bin"]] || !seen[want["second.bin"]] || seen[42] {
		t.Fatalf("TextAsset PathIDs = %v, want canonical IDs %v and no legacy ID 42", seen, want)
	}
}
