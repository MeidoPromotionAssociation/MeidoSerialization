package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKCESExportCMMatAliasCommandRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gp03_export_body[0][0]skin.mat")
	want, err := os.ReadFile(filepath.Join("..", "testdata", "test.mate"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, want, 0644); err != nil {
		t.Fatal(err)
	}

	output, err := executeCommand(RootCmd, "determine", "--strict", path)
	if err != nil {
		t.Fatalf("determine --strict .mat: %v\n%s", err, output)
	}
	for _, marker := range []string{"Type: mate", "Format: binary", "Game: COM3D2", "Signature: CM3D2_MATERIAL"} {
		if !strings.Contains(output, marker) {
			t.Fatalf("determine output lacks %q:\n%s", marker, output)
		}
	}

	output, err = executeCommand(RootCmd, "convert2json", "--strict", "--type", "mate", path)
	if err != nil {
		t.Fatalf("convert2json --type mate .mat: %v\n%s", err, output)
	}
	jsonPath := path + ".json"
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("missing .mat.json output: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	output, err = executeCommand(RootCmd, "convert2mod", "--strict", "--type", "mat.json", jsonPath)
	if err != nil {
		t.Fatalf("convert2mod --type mat.json: %v\n%s", err, output)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing reconstructed .mat: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf(".mat command round trip changed bytes: got %d bytes, want %d", len(got), len(want))
	}
}

func TestKCESExportCMMatAliasNonStrictFilters(t *testing.T) {
	dir := t.TempDir()
	nativePath := filepath.Join(dir, "export.mat")
	jsonPath := nativePath + ".json"
	for _, path := range []string{nativePath, jsonPath} {
		if err := os.WriteFile(path, []byte("placeholder"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	oldStrict, oldType := strictMode, fileType
	t.Cleanup(func() {
		strictMode = oldStrict
		fileType = oldType
	})
	strictMode = false

	for _, selector := range []string{"mate", "mat"} {
		fileType = selector
		if !fileTypeFilter(nativePath) || fileTypeFilter(jsonPath) {
			t.Fatalf("--type %s did not treat .mat as native mate alias", selector)
		}
	}
	for _, selector := range []string{"mate.json", "mat.json"} {
		fileType = selector
		if fileTypeFilter(nativePath) || !fileTypeFilter(jsonPath) {
			t.Fatalf("--type %s did not treat .mat.json as mate editing alias", selector)
		}
	}
}
