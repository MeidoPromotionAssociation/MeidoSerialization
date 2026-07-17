package aba

import (
	"strings"
	"testing"
)

func testResolverAssetsFile(pathIDs ...int64) *AssetsFile {
	infos := make([]AssetInfo, len(pathIDs))
	for i, pathID := range pathIDs {
		infos[i] = AssetInfo{PathId: pathID}
	}
	return &AssetsFile{Metadata: AssetsMetadata{AssetInfos: infos}}
}

func TestDefaultAssetResolverRejectsNullAndNilReferences(t *testing.T) {
	assets := testResolverAssetsFile(7)
	if _, _, err := DefaultAssetResolver(nil, 0, 7); err == nil || !strings.Contains(err.Error(), "nil relative AssetsFile") {
		t.Fatalf("DefaultAssetResolver(nil, ...) error = %v, want nil relative AssetsFile", err)
	}
	if _, _, err := DefaultAssetResolver(assets, 0, 0); err == nil || !strings.Contains(err.Error(), "null PPtr") {
		t.Fatalf("DefaultAssetResolver(..., pathID=0) error = %v, want null PPtr", err)
	}
	resolvedAF, resolvedInfo, err := DefaultAssetResolver(assets, 0, 7)
	if err != nil {
		t.Fatalf("DefaultAssetResolver valid reference: %v", err)
	}
	if resolvedAF != assets || resolvedInfo == nil || resolvedInfo.PathId != 7 {
		t.Fatalf("DefaultAssetResolver valid reference = (%p, %#v), want original asset and PathID 7", resolvedAF, resolvedInfo)
	}
}

func TestBundleAssetResolverExternalNameMatching(t *testing.T) {
	relative := testResolverAssetsFile(1)
	relative.Metadata.ExternalFiles = []ExternalFile{{PathName: "archive:/Textures/Bar.assets"}}
	dependency := testResolverAssetsFile(42)

	// The key is an alias with different casing and an additional directory.
	resolver := BundleAssetResolver(map[string]*AssetsFile{
		"textures/BAR.ASSETS": dependency,
	})
	resolvedAF, resolvedInfo, err := resolver(relative, 1, 42)
	if err != nil {
		t.Fatalf("case-insensitive external lookup: %v", err)
	}
	if resolvedAF != dependency || resolvedInfo == nil || resolvedInfo.PathId != 42 {
		t.Fatalf("external lookup = (%p, %#v), want dependency and PathID 42", resolvedAF, resolvedInfo)
	}

	// Complete path matching wins over a same-named basename alias.
	other := testResolverAssetsFile(42)
	resolver = BundleAssetResolver(map[string]*AssetsFile{
		"Textures/Bar.assets": dependency,
		"Other/Bar.assets":    other,
		"Bar.assets":          other,
	})
	if resolvedAF, _, err = resolver(relative, 1, 42); err != nil || resolvedAF != dependency {
		t.Fatalf("complete external path precedence = (%p, %v), want Textures/Bar.assets file", resolvedAF, err)
	}

	// Multiple aliases for the same file are not ambiguous.
	resolver = BundleAssetResolver(map[string]*AssetsFile{
		"Bar.assets":            dependency,
		"other/path/Bar.assets": dependency,
	})
	if resolvedAF, _, err = resolver(relative, 1, 42); err != nil || resolvedAF != dependency {
		t.Fatalf("same-file external aliases = (%p, %v), want dependency without ambiguity", resolvedAF, err)
	}
}

func TestBundleAssetResolverRejectsAmbiguousExternalName(t *testing.T) {
	relative := testResolverAssetsFile(1)
	relative.Metadata.ExternalFiles = []ExternalFile{{PathName: "Textures/Bar.assets"}}
	first := testResolverAssetsFile(42)
	second := testResolverAssetsFile(42)
	resolver := BundleAssetResolver(map[string]*AssetsFile{
		"Bar.assets":       first,
		"other/Bar.assets": second,
	})
	if _, _, err := resolver(relative, 1, 42); err == nil || !strings.Contains(err.Error(), "matches 2 AssetsFiles") {
		t.Fatalf("ambiguous external lookup error = %v, want ambiguity error", err)
	}

	// A case-insensitive collision is also ambiguous when no exact key exists.
	resolver = BundleAssetResolver(map[string]*AssetsFile{
		"BAR.ASSETS": first,
		"bar.assets": second,
	})
	relative.Metadata.ExternalFiles[0].PathName = "Textures/BaR.assets"
	if _, _, err := resolver(relative, 1, 42); err == nil || !strings.Contains(err.Error(), "matches 2 AssetsFiles") {
		t.Fatalf("case-insensitive ambiguous lookup error = %v, want ambiguity error", err)
	}
}

func TestBundleAssetResolverRejectsAmbiguousFallbackAndNullPPtr(t *testing.T) {
	first := testResolverAssetsFile(99)
	second := testResolverAssetsFile(99)
	resolver := BundleAssetResolver(map[string]*AssetsFile{
		"first.assets":  first,
		"second.assets": second,
	})
	if _, _, err := resolver(nil, 1, 0); err == nil || !strings.Contains(err.Error(), "null PPtr") {
		t.Fatalf("null fallback PPtr error = %v, want null PPtr", err)
	}
	if _, _, err := resolver(nil, 1, 99); err == nil || !strings.Contains(err.Error(), "ambiguous across 2 AssetsFiles") {
		t.Fatalf("ambiguous fallback error = %v, want ambiguity error", err)
	}
}

func TestBundleAssetResolverDoesNotCrossFileFallbackAfterNamedDependencyMatch(t *testing.T) {
	relative := testResolverAssetsFile(1)
	relative.Metadata.ExternalFiles = []ExternalFile{{PathName: "Textures/Bar.assets"}}
	dependency := testResolverAssetsFile(7)
	unrelated := testResolverAssetsFile(42)
	resolver := BundleAssetResolver(map[string]*AssetsFile{
		"Textures/Bar.assets": dependency,
		"Other.assets":        unrelated,
	})
	if _, _, err := resolver(relative, 1, 42); err == nil || !strings.Contains(err.Error(), "not found in external file") {
		t.Fatalf("resolver error = %v, want missing object in named external file", err)
	}
}

func TestNormalizeBundleAssetPath(t *testing.T) {
	tests := map[string]string{
		"archive:/Textures\\Bar.assets": "Textures/Bar.assets",
		"archive://Textures/Bar.assets": "Textures/Bar.assets",
		"/Textures/./Bar.assets":        "Textures/Bar.assets",
		"../Bar.assets":                 "",
		".":                             "",
	}
	for input, want := range tests {
		if got := normalizeBundleAssetPath(input); got != want {
			t.Errorf("normalizeBundleAssetPath(%q) = %q, want %q", input, got, want)
		}
	}
}
