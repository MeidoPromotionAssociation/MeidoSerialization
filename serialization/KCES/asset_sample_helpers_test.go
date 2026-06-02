package KCES

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKCESAssetSamplesHaveKnownSuffixes(t *testing.T) {
	knownSuffixes := map[string]struct{}{
		".nei":    {},
		".psk":    {},
		".png":    {},
		".crmesh": {},
		".bytes":  {},
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
	paths, err := filepath.Glob(filepath.Join(kcesAssetSampleDir(), "*"))
	if err != nil {
		t.Fatalf("glob asset samples: %v", err)
	}

	var samples []string
	for _, path := range paths {
		name := strings.ToLower(filepath.Base(path))
		if strings.HasSuffix(name, ".meta.json") || strings.HasSuffix(name, ".typetree.json") {
			continue
		}
		samples = append(samples, path)
	}
	if len(samples) == 0 {
		t.Skip("no asset samples found in testdata/kces_assets")
	}
	return samples
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

func readKCESAssetSample(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(kcesAssetSampleDir(), name)
	return readAssetSampleFile(t, path)
}

func kcesAssetSampleDir() string {
	return filepath.Join("..", "..", "testdata", "kces_assets")
}
