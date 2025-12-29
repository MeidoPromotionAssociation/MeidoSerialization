package arc

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestNewArc(t *testing.T) {
	fs := New("test_arc")
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
	fs := New("test")

	// 1. Create file
	data := []byte("hello world")
	path := "dir1/subdir/test.txt"
	f := fs.CreateFile(path, data)
	if f == nil {
		t.Fatalf("failed to create file %s", path)
	}

	// 2. Find file
	found := fs.GetFile(path)
	if found == nil {
		t.Fatalf("failed to find file %s", path)
	}
	d, _ := found.Ptr.Data()
	if !bytes.Equal(d, data) {
		t.Errorf("content mismatch: %s != %s", string(d), string(data))
	}

	// 3. Copy file
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

	// 4. List files
	list := fs.GetFileList()
	if len(list) != 2 {
		t.Errorf("expected 2 files, got %d", len(list))
	}

	// 5. Delete file
	if !fs.DeleteFile(path) {
		t.Error("failed to delete file")
	}
	if fs.GetFile(path) != nil {
		t.Error("file still exists after deletion")
	}
	if len(fs.GetFileList()) != 1 {
		t.Error("file count mismatch after deletion")
	}
}

func TestMerge(t *testing.T) {
	fs1 := New("fs1")
	fs1.CreateFile("common.txt", []byte("v1"))
	fs1.CreateFile("only1.txt", []byte("1"))

	fs2 := New("fs2")
	fs2.CreateFile("common.txt", []byte("v2"))
	fs2.CreateFile("only2.txt", []byte("2"))

	fs1.MergeFrom(fs2, false)

	if len(fs1.GetFileList()) != 3 {
		t.Errorf("expected 3 files after merge, got %d", len(fs1.GetFileList()))
	}

	// common.txt should be overwritten by default if not keeping dupes
	f := fs1.GetFile("common.txt")
	d, _ := f.Ptr.Data()
	if !bytes.Equal(d, []byte("v2")) {
		t.Errorf("expected common.txt to be v2, got %s", string(d))
	}
}

func TestArc(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/test*.arc")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			arc, err := ReadArc(filePath)
			if err != nil {
				t.Fatalf("failed to read arc: %v", err)
			}

			if arc == nil || arc.Root == nil {
				t.Fatal("arc or root is nil")
			}

			// Test Dump
			tempArc := filepath.Join(t.TempDir(), "test_output.arc")
			err = arc.Dump(tempArc)
			if err != nil {
				t.Fatalf("failed to dump arc: %v", err)
			}

			// Re-read
			arc2, err := ReadArc(tempArc)
			if err != nil {
				t.Fatalf("failed to re-read dumped arc: %v", err)
			}

			files1 := arc.GetFileList()
			files2 := arc2.GetFileList()

			if len(files1) != len(files2) {
				t.Errorf("file count mismatch: %d != %d", len(files1), len(files2))
			}

			// Compare content of first few files
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
		})
	}
}
