package KCES

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAbaServiceUnpackAbaExportsOnlyIndependentResourceFiles(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "parts_personal_om015_gp003.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	outDir := filepath.Join(t.TempDir(), "unpacked")
	if err := (&AbaService{}).UnpackAba(sample, outDir); err != nil {
		t.Fatal(err)
	}
	files := validateCanonicalDirectoryFiles(t, outDir, nil)
	if len(files) == 0 {
		t.Fatal("unpacked directory is empty")
	}
	typeDirectories := make(map[string]bool)
	for _, relativePath := range files {
		lower := strings.ToLower(relativePath)
		for _, suffix := range []string{".meta.json", ".typetree.json", ".ress", ".resource", ".resources", ".png"} {
			if strings.HasSuffix(lower, suffix) {
				t.Fatalf("unpacked directory contains forbidden artifact %q", relativePath)
			}
		}
		parts := strings.Split(filepath.ToSlash(relativePath), "/")
		if len(parts) < 2 {
			t.Fatalf("resource %q is not stored below its Unity type directory", relativePath)
		}
		typeDirectories[parts[0]] = true
	}
	for _, required := range []string{"TextAsset", "Texture2D", "Mesh", "Sprite"} {
		if !typeDirectories[required] {
			t.Errorf("sample did not produce %s resources", required)
		}
	}
}
