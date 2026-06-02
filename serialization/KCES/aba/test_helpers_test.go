package aba

import (
	"os"
	"path/filepath"
	"testing"
)

const abaHeavyTestEnv = "KCES_ABA_HEAVY_TESTS"

func smallAbaTestFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../../../testdata/aba/*.aba")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no .aba test files found")
	}
	if os.Getenv(abaHeavyTestEnv) != "" {
		return files
	}

	const maxDefaultSampleSize = int64(8 << 20)
	var small []string
	for _, filePath := range files {
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("stat %s: %v", filePath, err)
		}
		if info.Size() <= maxDefaultSampleSize {
			small = append(small, filePath)
		}
	}
	if len(small) == 0 {
		t.Skip("no small .aba test files found")
	}
	return small
}

func openAbaSample(t *testing.T, name string) (*Bundle, *os.File) {
	t.Helper()
	filePath := filepath.Join("..", "..", "..", "testdata", "aba", name)
	f, err := os.Open(filePath)
	if err != nil {
		t.Skipf("sample .aba not available: %v", err)
	}
	bundle, err := ReadBundle(f)
	if err != nil {
		f.Close()
		t.Fatalf("ReadBundle: %v", err)
	}
	return bundle, f
}
