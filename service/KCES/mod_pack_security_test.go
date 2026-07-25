package KCES

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeModAssetPathRejectsUnsafePortablePaths(t *testing.T) {
	for _, input := range []string{
		"",
		"../outside.bin",
		"dir/../outside.bin",
		`dir\..\outside.bin`,
		"/absolute.bin",
		`\absolute.bin`,
		`C:\outside.bin`,
		`C:relative.bin`,
		`\\server\share\outside.bin`,
		"dir//file.bin",
		`dir\\file.bin`,
		"./file.bin",
		"file.bin\x00ignored",
		"file.bin:stream",
	} {
		t.Run(strings.ReplaceAll(input, "\\", "_"), func(t *testing.T) {
			if got, err := normalizeModAssetPath(input); err == nil {
				t.Fatalf("normalizeModAssetPath(%q) unexpectedly returned %q", input, got)
			}
		})
	}

	want := filepath.Join("nested", "asset.bin")
	for _, input := range []string{"nested/asset.bin", `nested\asset.bin`} {
		if got, err := normalizeModAssetPath(input); err != nil || got != want {
			t.Fatalf("normalizeModAssetPath(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestPackModManifestRejectsEscapingAssetPathsWithoutOutputs(t *testing.T) {
	baseDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.bin")
	if err := os.WriteFile(outsidePath, []byte("outside secret"), 0644); err != nil {
		t.Fatal(err)
	}

	paths := []string{
		"../outside.bin",
		`..\outside.bin`,
		outsidePath,
		`C:\outside.bin`,
		`\\server\share\outside.bin`,
		"nested//asset.bin",
	}
	for i, assetPath := range paths {
		name := "unsafe_" + strings.Repeat("x", i+1)
		t.Run(name, func(t *testing.T) {
			manifest := ModManifest{
				Name: name, CatalogType: "Parts", PackageType: "Plugin",
				Assets: []ModAsset{{Name: "asset.menuassets", Path: assetPath, Kind: "textasset"}},
			}
			err := packModManifest(manifest, baseDir, baseDir)
			if err == nil || !strings.Contains(err.Error(), "unsafe source path") {
				t.Fatalf("packModManifest(%q) error = %v, want unsafe source path", assetPath, err)
			}
			assertPackPairAbsent(t, baseDir, name)
		})
	}
}

func TestPackModManifestRejectsOversizedInMemorySourceBeforeReading(t *testing.T) {
	baseDir := t.TempDir()
	source := filepath.Join(baseDir, "huge.bin")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxPackInMemoryAssetSize + 1); err != nil {
		f.Close()
		t.Skipf("filesystem cannot create sparse oversized fixture: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	manifest := ModManifest{
		Name:        "oversized_source",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets:      []ModAsset{{Name: "huge.menuassets", Path: "huge.bin", Kind: "textasset"}},
	}
	err = packModManifest(manifest, baseDir, baseDir)
	if err == nil || !strings.Contains(err.Error(), "exceeds in-memory packing limit") {
		t.Fatalf("packModManifest error = %v, want early size rejection", err)
	}
	assertPackPairAbsent(t, baseDir, manifest.Name)
}

func TestPackModManifestConfinesAssetReadsAndIgnoresLegacySidecars(t *testing.T) {
	t.Run("asset symlink", func(t *testing.T) {
		baseDir := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.bin")
		if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(baseDir, "asset.bin")
		makeTestSymlinkOrSkip(t, outside, link)

		manifest := testTextManifest("asset_link", "asset.bin")
		err := packModManifest(manifest, baseDir, baseDir)
		if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "symlink") &&
			!strings.Contains(strings.ToLower(err.Error()), "reparse")) {
			t.Fatalf("packModManifest error = %v, want symlink/reparse rejection", err)
		}
		assertPackPairAbsent(t, baseDir, manifest.Name)
	})

	t.Run("metadata sidecar symlink is ignored", func(t *testing.T) {
		baseDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(baseDir, "asset.bin"), []byte("asset"), 0644); err != nil {
			t.Fatal(err)
		}
		outsideMeta := filepath.Join(t.TempDir(), "outside.meta.json")
		if err := os.WriteFile(outsideMeta, []byte(`{"pathId":123}`), 0644); err != nil {
			t.Fatal(err)
		}
		makeTestSymlinkOrSkip(t, outsideMeta, filepath.Join(baseDir, "asset.bin.meta.json"))

		manifest := testTextManifest("sidecar_link", "asset.bin")
		if err := packModManifest(manifest, baseDir, baseDir); err != nil {
			t.Fatalf("packModManifest read an ignored sidecar: %v", err)
		}
		for _, extension := range []string{".ct", ".aba"} {
			if _, err := os.Stat(filepath.Join(baseDir, manifest.Name+extension)); err != nil {
				t.Fatalf("missing %s output: %v", extension, err)
			}
		}
	})
}

func TestPackServiceRejectsSymlinkOrReparseInput(t *testing.T) {
	inputDir := filepath.Join(t.TempDir(), "input")
	if err := os.Mkdir(inputDir, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.menuassets")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	makeTestSymlinkOrSkip(t, outside, filepath.Join(inputDir, "linked.menuassets"))

	err := (&PackService{}).PackToAbaAndCt(inputDir, "linked_pack")
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "symlink") &&
		!strings.Contains(strings.ToLower(err.Error()), "reparse")) {
		t.Fatalf("PackToAbaAndCt error = %v, want symlink/reparse rejection", err)
	}
	assertPackPairAbsent(t, filepath.Dir(inputDir), "linked_pack")
}

func TestWritePackOutputPairRollsBackSecondInstallFailure(t *testing.T) {
	for _, withExistingPair := range []bool{false, true} {
		t.Run(map[bool]string{false: "new pair", true: "replace pair"}[withExistingPair], func(t *testing.T) {
			dir := t.TempDir()
			ctPath := filepath.Join(dir, "pair.ct")
			abaPath := filepath.Join(dir, "pair.aba")
			if withExistingPair {
				if err := os.WriteFile(ctPath, []byte("old ct"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(abaPath, []byte("old aba"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			injected := false
			rename := func(root *os.Root, oldName, newName string) error {
				if !injected && strings.HasSuffix(oldName, ".tmp") && newName == "pair.aba" {
					injected = true
					return errors.New("injected second install failure")
				}
				return root.Rename(oldName, newName)
			}
			err := writePackOutputPairWithRename(
				dir,
				"pair.ct", []byte("new ct"),
				"pair.aba", []byte("new aba"),
				rename,
			)
			if err == nil || !strings.Contains(err.Error(), "injected second install failure") || !injected {
				t.Fatalf("writePackOutputPairWithRename error = %v", err)
			}

			if withExistingPair {
				assertFileBytes(t, ctPath, []byte("old ct"))
				assertFileBytes(t, abaPath, []byte("old aba"))
			} else {
				assertPackPairAbsent(t, dir, "pair")
			}
			assertNoPackWorkFiles(t, dir)
		})
	}
}

func TestWritePackOutputPairReplacesExistingPairAndRejectsUnsafeTarget(t *testing.T) {
	t.Run("successful replacement", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "pair.ct"), []byte("old ct"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "pair.aba"), []byte("old aba"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := writePackOutputPair(dir, "pair.ct", []byte("new ct"), "pair.aba", []byte("new aba")); err != nil {
			t.Fatalf("writePackOutputPair: %v", err)
		}
		assertFileBytes(t, filepath.Join(dir, "pair.ct"), []byte("new ct"))
		assertFileBytes(t, filepath.Join(dir, "pair.aba"), []byte("new aba"))
		assertNoPackWorkFiles(t, dir)
	})

	t.Run("directory target preflight", func(t *testing.T) {
		dir := t.TempDir()
		ctPath := filepath.Join(dir, "pair.ct")
		if err := os.WriteFile(ctPath, []byte("old ct"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, "pair.aba"), 0755); err != nil {
			t.Fatal(err)
		}
		err := writePackOutputPair(dir, "pair.ct", []byte("new ct"), "pair.aba", []byte("new aba"))
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("writePackOutputPair error = %v, want non-regular target", err)
		}
		assertFileBytes(t, ctPath, []byte("old ct"))
		assertNoPackWorkFiles(t, dir)
	})
}

func testTextManifest(name, path string) ModManifest {
	return ModManifest{
		Name: name, CatalogType: "Parts", PackageType: "Plugin",
		Assets: []ModAsset{{Name: "asset.menuassets", Path: path, Kind: "textasset"}},
	}
}

func makeTestSymlinkOrSkip(t *testing.T, oldName, newName string) {
	t.Helper()
	if err := os.Symlink(oldName, newName); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
}

func assertPackPairAbsent(t *testing.T, dir, base string) {
	t.Helper()
	for _, ext := range []string{".ct", ".aba"} {
		if _, err := os.Lstat(filepath.Join(dir, base+ext)); !os.IsNotExist(err) {
			t.Fatalf("unexpected output %q (Lstat error %v)", base+ext, err)
		}
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s bytes = %q, want %q", filepath.Base(path), got, want)
	}
}

func assertNoPackWorkFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".meido-pack-") {
			t.Fatalf("transaction work file was not cleaned: %s", entry.Name())
		}
	}
}
