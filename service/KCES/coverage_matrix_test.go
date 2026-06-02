package KCES

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestKCESKnownExtensionRoutingMatrix(t *testing.T) {
	testdata := filepath.Join("..", "..", "testdata")

	assertRouteSamples(t, filepath.Join(testdata, "aba", "*.ct"), ".ct", IsKCESCtFile)
	assertGeneratedCtJSONRouteSamples(t, filepath.Join(testdata, "aba", "*.ct"))
	assertRouteSamples(t, filepath.Join(testdata, "kces_parts", "*"), "parts", IsKCESPartsFile)
	assertRouteSamples(t, filepath.Join(testdata, "kces_payload", "*"), "payload", IsKCESPayloadFile)
	assertRouteSamples(t, filepath.Join(testdata, "kces_misc", "*"), "misc", IsKCESMiscFile)
	assertRouteSamples(t, filepath.Join(testdata, "kces_assets", "*.psk"), ".psk", IsKCESDataFile)
	assertRouteSamples(t, filepath.Join(testdata, "kces_assets", "*.bytes"), ".bytes", IsKCESRawUnityBytesFile)
	assertAssetDataUnsupportedSamples(t, filepath.Join(testdata, "kces_assets", "*"))
}

func assertGeneratedCtJSONRouteSamples(t *testing.T, pattern string) {
	t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob .ct route samples %s: %v", pattern, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no .ct route samples for .ct.json")
	}
	service := &CtService{}
	t.Run(".ct.json", func(t *testing.T) {
		for _, path := range paths {
			path := path
			t.Run(filepath.Base(path)+".json", func(t *testing.T) {
				jsonPath := filepath.Join(t.TempDir(), filepath.Base(path)+".json")
				if err := service.ConvertCtToJson(path, jsonPath); err != nil {
					t.Fatalf("ConvertCtToJson: %v", err)
				}
				if !IsKCESCtJSONFile(jsonPath) {
					t.Fatalf(".ct JSON was not routed as supported: %s", jsonPath)
				}
			})
		}
	})
}

func assertRouteSamples(t *testing.T, pattern string, group string, ok func(string) bool) {
	t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob route samples %s: %v", pattern, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no route samples for %s", group)
	}
	t.Run(group, func(t *testing.T) {
		for _, path := range paths {
			path := path
			if strings.HasSuffix(strings.ToLower(path), ".meta.json") || strings.HasSuffix(strings.ToLower(path), ".typetree.json") {
				continue
			}
			t.Run(filepath.Base(path), func(t *testing.T) {
				if !ok(path) {
					t.Fatalf("%s was not routed as supported: %s", group, path)
				}
			})
		}
	})
}

func assertAssetDataUnsupportedSamples(t *testing.T, pattern string) {
	t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob asset route samples %s: %v", pattern, err)
	}
	t.Run("assets-not-data", func(t *testing.T) {
		for _, path := range paths {
			path := path
			lower := strings.ToLower(path)
			if strings.HasSuffix(lower, ".meta.json") || strings.HasSuffix(lower, ".typetree.json") || strings.HasSuffix(lower, ".psk") {
				continue
			}
			t.Run(filepath.Base(path), func(t *testing.T) {
				if IsKCESDataFile(path) {
					t.Fatalf("non-.psk KCES asset was routed as shared data: %s", path)
				}
			})
		}
	})
}
