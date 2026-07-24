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
				if int32(len(raw)) != vf.Size {
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

func TestDecodeVirtualDirectoryNestedFiles(t *testing.T) {
	table := &ContentTable{
		Raw: make([]byte, HeaderSize+8),
	}
	dirArray := []interface{}{
		int64(ctVersion),
		map[string]interface{}{
			"nested": []interface{}{
				int64(ctVersion),
				map[string]interface{}{},
				map[string]interface{}{
					"child.bin": []interface{}{int64(HeaderSize), int64(3)},
				},
			},
		},
		map[string]interface{}{
			"root.bin": []interface{}{int64(HeaderSize + 3), int64(5)},
		},
	}
	data, err := EncodeMsgpack(dirArray)
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}

	if err := table.decodeVirtualDirectory(data); err != nil {
		t.Fatalf("decodeVirtualDirectory: %v", err)
	}
	if len(table.Files) != 2 {
		t.Fatalf("file count got %d, want 2: %#v", len(table.Files), table.Files)
	}
	if vf := table.Files["nested/child.bin"]; vf.Position != HeaderSize || vf.Size != 3 {
		t.Fatalf("nested file got %+v, want position=%d size=3", vf, HeaderSize)
	}
	if vf := table.Files["root.bin"]; vf.Position != HeaderSize+3 || vf.Size != 5 {
		t.Fatalf("root file got %+v, want position=%d size=5", vf, HeaderSize+3)
	}
}

func TestWriteContentTable_NestedFilesRoundTrip(t *testing.T) {
	raw := append(make([]byte, HeaderSize), []byte("abcdef")...)
	copy(raw[:7], FileSignature)
	raw[7] = SerializeTypeMsgPack
	table := &ContentTable{
		Version: ctVersion,
		Raw:     raw,
		Files: map[string]VirtualFile{
			"catalog":         {Position: HeaderSize, Size: 3},
			"nested/file.bin": {Position: HeaderSize + 3, Size: 3},
		},
	}

	var buf bytes.Buffer
	if err := WriteContentTable(&buf, table); err != nil {
		t.Fatalf("WriteContentTable: %v", err)
	}
	decoded, err := ReadContentTable(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadContentTable: %v", err)
	}
	for name, want := range map[string][]byte{
		"catalog":         []byte("abc"),
		"nested/file.bin": []byte("def"),
	} {
		got, err := decoded.GetFileData(name)
		if err != nil {
			t.Fatalf("GetFileData(%q): %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("GetFileData(%q) got %q, want %q", name, got, want)
		}
	}
}

func TestWriteContentTableIsDeterministic(t *testing.T) {
	raw := append(make([]byte, HeaderSize), []byte("abcdefgh")...)
	copy(raw[:7], FileSignature)
	raw[7] = SerializeTypeMsgPack
	table := &ContentTable{
		Version: ctVersion,
		Raw:     raw,
		Files: map[string]VirtualFile{
			"z":        {Position: HeaderSize, Size: 2},
			"a":        {Position: HeaderSize + 2, Size: 2},
			"nested/z": {Position: HeaderSize + 4, Size: 2},
			"nested/a": {Position: HeaderSize + 6, Size: 2},
		},
	}

	var first bytes.Buffer
	if err := WriteContentTable(&first, table); err != nil {
		t.Fatal(err)
	}
	for iteration := int64(0); iteration < 64; iteration++ {
		var next bytes.Buffer
		if err := WriteContentTable(&next, table); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(next.Bytes(), first.Bytes()) {
			t.Fatalf("encoding %d changed ContentTable bytes", iteration)
		}
	}
}
