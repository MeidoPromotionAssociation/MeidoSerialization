package KCES

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
)

func TestKCESAssetSamplesHaveKnownSuffixes(t *testing.T) {
	knownSuffixes := map[string]struct{}{
		".nei":   {},
		".psk":   {},
		".png":   {},
		".bytes": {},
	}
	for _, path := range allKCESAssetSamplePaths(t) {
		suffix := kcesAssetSampleSuffix(path)
		if _, ok := knownSuffixes[suffix]; !ok {
			t.Fatalf("unexpected KCES asset sample suffix %q for %s", suffix, filepath.Base(path))
		}
	}
}

func assetSamplePathsBySuffix(t *testing.T, suffix string) []string {
	t.Helper()
	var matches []string
	for _, path := range allKCESAssetSamplePaths(t) {
		if kcesAssetSampleSuffix(path) == suffix {
			matches = append(matches, path)
		}
	}
	if len(matches) == 0 {
		t.Skipf("no KCES asset samples with suffix %s", suffix)
	}
	return matches
}

func allKCESAssetSamplePaths(t *testing.T) []string {
	t.Helper()
	return kcesfixtures.AssetSamplePaths(t)
}

func kcesAssetSampleSuffix(path string) string {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(name, ".bytes") {
		return ".bytes"
	}
	return strings.ToLower(filepath.Ext(name))
}

func readAssetSampleFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read asset sample %s: %v", path, err)
	}
	if len(data) == 0 {
		t.Fatalf("empty asset sample %s", filepath.Base(path))
	}
	return data
}
