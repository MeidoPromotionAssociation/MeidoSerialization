package ct

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReadContentTable(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/aba/*.ct")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no .ct test files found")
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			defer f.Close()

			ct, err := ReadContentTable(f)
			if err != nil {
				t.Fatalf("ReadContentTable failed: %v", err)
			}

			if ct.Version == 0 {
				t.Error("version is 0")
			}
			if len(ct.Files) == 0 {
				t.Error("Files is empty")
			}

			t.Logf("Version: %d, Files: %d", ct.Version, len(ct.Files))
			for name, vf := range ct.Files {
				t.Logf("  %q: position=%d size=%d", name, vf.Position, vf.Size)

				raw, err := ct.GetFileData(name)
				if err != nil {
					t.Errorf("GetFileData(%q) failed: %v", name, err)
					continue
				}
				if len(raw) != vf.Size {
					t.Errorf("GetFileData(%q): got %d bytes, want %d", name, len(raw), vf.Size)
				}
			}
		})
	}
}

func TestReadContentTable_AllFiles(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/aba/*.ct")
	if err != nil {
		t.Fatal(err)
	}

	success := 0
	for _, filePath := range files {
		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		ct, err := ReadContentTable(f)
		f.Close()
		if err == nil && ct.Version > 0 && len(ct.Files) > 0 {
			success++
		} else if err != nil {
			fmt.Printf("  FAIL %s: %v\n", filepath.Base(filePath), err)
		}
	}

	fmt.Printf("Successfully parsed %d/%d .ct files\n", success, len(files))
	if success == 0 && len(files) > 0 {
		t.Error("failed to parse any .ct files")
	}
	if success < len(files) {
		t.Errorf("only %d/%d files parsed successfully", success, len(files))
	}
}

func TestWriteContentTable_RoundTrip(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/aba/*.ct")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no .ct test files found")
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			defer f.Close()

			ct, err := ReadContentTable(f)
			if err != nil {
				t.Fatalf("ReadContentTable failed: %v", err)
			}

			var buf bytes.Buffer
			if err := WriteContentTable(&buf, ct); err != nil {
				t.Fatalf("WriteContentTable failed: %v", err)
			}

			ct2, err := ReadContentTable(&buf)
			if err != nil {
				t.Fatalf("re-read failed: %v", err)
			}

			if ct2.Version != ct.Version {
				t.Errorf("version mismatch: got %d, want %d", ct2.Version, ct.Version)
			}
			if len(ct2.Files) != len(ct.Files) {
				t.Errorf("file count mismatch: got %d, want %d", len(ct2.Files), len(ct.Files))
			}

			for name := range ct.Files {
				orig, err := ct.GetFileData(name)
				if err != nil {
					t.Errorf("original GetFileData(%q) failed: %v", name, err)
					continue
				}
				rewritten, err := ct2.GetFileData(name)
				if err != nil {
					t.Errorf("rewritten GetFileData(%q) failed: %v", name, err)
					continue
				}
				if !bytes.Equal(orig, rewritten) {
					t.Errorf("data mismatch for %q: orig %d bytes, rewritten %d bytes", name, len(orig), len(rewritten))
				}
			}
		})
	}
}
