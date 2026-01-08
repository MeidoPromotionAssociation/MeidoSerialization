package arc

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewArc(t *testing.T) {
	fs := NewArc("test_arc")
	if fs.Name != "test_arc" {
		t.Errorf("expected name test_arc, got %s", fs.Name)
	}
	if fs.Root == nil {
		t.Fatal("expected root dir, got nil")
	}
	if len(fs.Root.Files) != 0 || len(fs.Root.Dirs) != 0 {
		t.Error("expected empty root")
	}
}

func TestArcFileSystem(t *testing.T) {
	fs := NewArc("test")

	// 1. Create file
	start := time.Now()
	data := []byte("hello world")
	path := "dir1/subdir/test.txt"
	f := fs.CreateFile(path, data)
	if f == nil {
		t.Fatalf("failed to create file %s", path)
	}
	t.Logf("1. Create file took %v", time.Since(start))

	// 2. Find file
	start = time.Now()
	found := fs.GetFile(path)
	if found == nil {
		t.Fatalf("failed to find file %s", path)
	}
	d, _ := found.Ptr.Data()
	if !bytes.Equal(d, data) {
		t.Errorf("content mismatch: %s != %s", string(d), string(data))
	}
	t.Logf("2. Find file took %v", time.Since(start))

	// 3. Copy file
	start = time.Now()
	dstPath := "dir2/copy.txt"
	err := fs.CopyFile(path, dstPath)
	if err != nil {
		t.Fatalf("failed to copy file: %v", err)
	}
	foundCopy := fs.GetFile(dstPath)
	if foundCopy == nil {
		t.Fatal("copy not found")
	}
	d2, _ := foundCopy.Ptr.Data()
	if !bytes.Equal(d2, data) {
		t.Error("copy content mismatch")
	}
	t.Logf("3. Copy file took %v", time.Since(start))

	// 4. List files
	start = time.Now()
	list := fs.GetFileList()
	if len(list) != 2 {
		t.Errorf("expected 2 files, got %d", len(list))
	}
	t.Logf("4. List files took %v", time.Since(start))

	// 5. Delete file
	start = time.Now()
	if !fs.DeleteFile(path) {
		t.Error("failed to delete file")
	}
	if fs.GetFile(path) != nil {
		t.Error("file still exists after deletion")
	}
	if len(fs.GetFileList()) != 1 {
		t.Error("file count mismatch after deletion")
	}
	t.Logf("5. Delete file took %v", time.Since(start))
}

func TestMerge(t *testing.T) {
	fs1 := NewArc("fs1")
	fs1.CreateFile("common.txt", []byte("v1"))
	fs1.CreateFile("only1.txt", []byte("1"))

	fs2 := NewArc("fs2")
	fs2.CreateFile("common.txt", []byte("v2"))
	fs2.CreateFile("only2.txt", []byte("2"))

	fs1.MergeFrom(fs2, false)

	if len(fs1.GetFileList()) != 3 {
		t.Errorf("expected 3 files after merge, got %d", len(fs1.GetFileList()))
	}

	// common.txt should be overwritten by default if not keeping dupes
	f := fs1.GetFile("common.txt")
	if f == nil {
		t.Errorf("common.txt not found")
		return
	}
	d, _ := f.Ptr.Data()
	if !bytes.Equal(d, []byte("v2")) {
		t.Errorf("expected common.txt to be v2, got %s", string(d))
	}
}

func TestArc(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/*.arc")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("cannot open .arc file: %v", err)
			}
			defer f.Close()

			start := time.Now()
			arc, err := ReadArc(f)
			if err != nil {
				if strings.Contains(err.Error(), "unsupported") {
					t.Skipf("skipping %s: %v", filepath.Base(filePath), err)
				}
				t.Fatalf("failed to read arc: %v", err)
			}
			t.Logf("ReadArc took %v", time.Since(start))

			if arc == nil || arc.Root == nil {
				t.Fatal("arc or root is nil")
			}

			// Test Dump
			tempArcPath := filepath.Join(t.TempDir(), "test_output.arc")
			start = time.Now()
			err = arc.Dump(tempArcPath)
			if err != nil {
				t.Fatalf("failed to dump arc: %v", err)
			}
			t.Logf("Dump took %v", time.Since(start))

			// Re-read
			f2, err := os.Open(tempArcPath)
			if err != nil {
				t.Fatalf("cannot open .arc file: %v", err)
			}
			defer f2.Close()
			start = time.Now()
			arc2, err := ReadArc(f2)
			if err != nil {
				t.Fatalf("failed to re-read dumped arc: %v", err)
			}
			t.Logf("Re-read took %v", time.Since(start))

			files1 := arc.GetFileList()
			files2 := arc2.GetFileList()

			if len(files1) != len(files2) {
				t.Errorf("file count mismatch: %d != %d", len(files1), len(files2))
			}

			// Compare content of first few files
			start = time.Now()
			for i := 0; i < len(files1) && i < 5; i++ {
				f1 := arc.GetFile(files1[i])
				f2 := arc2.GetFile(files1[i])
				if f2 == nil {
					t.Errorf("file %s missing in re-read arc", files1[i])
					continue
				}
				d1, _ := f1.Ptr.Data()
				d2, _ := f2.Ptr.Data()
				if !bytes.Equal(d1, d2) {
					t.Errorf("content mismatch for file %s", files1[i])
				}
			}
			t.Logf("Compare took %v", time.Since(start))

			if !isArcEqual(arc, arc2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}

func isArcEqual(a, b *Arc) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if !reflect.DeepEqual(a.CompressGlobs, b.CompressGlobs) {
		return false
	}
	return isDirEqual(a.Root, b.Root)
}

func isDirEqual(a, b *Dir) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if len(a.Dirs) != len(b.Dirs) || len(a.Files) != len(b.Files) {
		return false
	}
	for name, d1 := range a.Dirs {
		d2, ok := b.Dirs[name]
		if !ok || !isDirEqual(d1, d2) {
			return false
		}
	}
	for name, f1 := range a.Files {
		f2, ok := b.Files[name]
		if !ok || !isFileEqual(f1, f2) {
			return false
		}
	}
	return true
}

func isFileEqual(a, b *File) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	p1, p2 := a.Ptr, b.Ptr
	if p1.Size() != p2.Size() || p1.RawSize() != p2.RawSize() || p1.Compressed() != p2.Compressed() {
		return false
	}
	p1Data, err := p1.Data()
	if err != nil {
		return false
	}
	p2Data, err := p2.Data()
	if err != nil {
		return false
	}
	if !reflect.DeepEqual(p1Data, p2Data) {
		return false
	}
	return true
}
