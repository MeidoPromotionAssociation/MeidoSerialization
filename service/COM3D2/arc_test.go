package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArcService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.arc")
	if err != nil {
		t.Fatal(err)
	}

	s := &ArcService{}
	for _, arcPath := range files {
		t.Run(filepath.Base(arcPath), func(t *testing.T) {
			tempDir := t.TempDir()
			unpackDir := filepath.Join(tempDir, "unpack")
			repackPath := filepath.Join(tempDir, "repack.arc")

			// 1. Test ReadArc
			start := time.Now()
			arc, err := s.ReadArc(arcPath)
			t.Logf("ReadArc took %v", time.Since(start))
			if err != nil {
				if strings.Contains(err.Error(), "unsupported") {
					t.Skipf("skipping %s: %v", arcPath, err)
				}
				t.Fatalf("ReadArc failed: %v", err)
			}
			if arc == nil {
				t.Fatal("ReadArc returned nil")
			}

			// 2. Test UnpackArc
			start = time.Now()
			err = s.UnpackArc(arcPath, unpackDir)
			t.Logf("UnpackArc took %v", time.Since(start))
			if err != nil {
				t.Fatalf("UnpackArc failed: %v", err)
			}

			// 3. Test PackArc
			start = time.Now()
			err = s.PackArc(unpackDir, repackPath)
			t.Logf("PackArc took %v", time.Since(start))
			if err != nil {
				t.Fatalf("PackArc failed: %v", err)
			}

			// 4. Test Read repackaged arc
			start = time.Now()
			arcRepack, err := s.ReadArc(repackPath)
			t.Logf("ReadRepack took %v", time.Since(start))
			if err != nil {
				t.Fatalf("Read repackaged arc failed: %v", err)
			}

			filesList := s.GetFileList(arc)
			filesRepack := s.GetFileList(arcRepack)
			t.Logf("File count: %d", len(filesList))
			if len(filesList) != len(filesRepack) {
				t.Errorf("File count mismatch: %d != %d", len(filesList), len(filesRepack))
			}
		})
	}
}

func TestArcServiceLogic(t *testing.T) {
	s := &ArcService{}
	fs := s.NewArc("test")

	// Test CreateFile
	data := []byte("service test data")
	path := "test/file.bin"
	err := s.CreateFile(fs, path, data)
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	// Test GetFileList
	list := s.GetFileList(fs)
	expectedPath := filepath.FromSlash(path)
	if len(list) != 1 || list[0] != expectedPath {
		t.Errorf("unexpected file list: %v, expected: [%s]", list, expectedPath)
	}

	// Test ExtractFile
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "extracted.bin")
	err = s.ExtractFile(fs, path, outPath)
	if err != nil {
		t.Fatalf("ExtractFile failed: %v", err)
	}
	readData, _ := os.ReadFile(outPath)
	if !bytes.Equal(readData, data) {
		t.Error("extracted data mismatch")
	}

	// Test CopyFile
	copyPath := "test/copy.bin"
	err = s.CopyFile(fs, path, copyPath)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}
	if len(s.GetFileList(fs)) != 2 {
		t.Error("CopyFile did not increase file count")
	}

	// Test MergeArc
	fs2 := s.NewArc("other")
	s.CreateFile(fs2, "other.txt", []byte("other"))
	s.MergeArc(fs2, fs, false)
	if len(s.GetFileList(fs)) != 3 {
		t.Errorf("MergeArc failed, count: %d", len(s.GetFileList(fs)))
	}

	// Test DeleteFile
	err = s.DeleteFile(fs, path)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
	if len(s.GetFileList(fs)) != 2 {
		t.Error("DeleteFile did not decrease file count")
	}
}

func TestArcServiceReadArcKeepsDataAccessible(t *testing.T) {
	s := &ArcService{}
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	arcPath := filepath.Join(tempDir, "sample.arc")
	extractPath := filepath.Join(tempDir, "out", "nested", "sample.txt")
	sourcePath := filepath.Join(sourceDir, "nested", "sample.txt")
	expected := []byte("lazy arc pointers must remain readable")

	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(sourcePath, expected, 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	if err := s.PackArc(sourceDir, arcPath); err != nil {
		t.Fatalf("PackArc failed: %v", err)
	}

	loaded, err := s.ReadArc(arcPath)
	if err != nil {
		t.Fatalf("ReadArc failed: %v", err)
	}

	if err := s.ExtractFile(loaded, "nested/sample.txt", extractPath); err != nil {
		t.Fatalf("ExtractFile after ReadArc failed: %v", err)
	}

	actual, err := os.ReadFile(extractPath)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("extracted data mismatch: got %q want %q", actual, expected)
	}
}

func TestArcServiceReadArcLazyRequiresOpenCloser(t *testing.T) {
	s := &ArcService{}
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	arcPath := filepath.Join(tempDir, "sample.arc")
	sourcePath := filepath.Join(sourceDir, "nested", "sample.txt")
	expected := []byte("lazy arc pointers should fail after close")

	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(sourcePath, expected, 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	if err := s.PackArc(sourceDir, arcPath); err != nil {
		t.Fatalf("PackArc failed: %v", err)
	}

	loaded, closer, err := s.ReadArcLazy(arcPath)
	if err != nil {
		t.Fatalf("ReadArcLazy failed: %v", err)
	}

	file := loaded.GetFile("nested/sample.txt")
	if file == nil {
		_ = closer.Close()
		t.Fatal("expected file in archive")
	}

	if data, err := file.Ptr.Data(); err != nil {
		_ = closer.Close()
		t.Fatalf("Ptr.Data before close failed: %v", err)
	} else if !bytes.Equal(data, expected) {
		_ = closer.Close()
		t.Fatalf("lazy data mismatch before close: got %q want %q", data, expected)
	}

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close lazy reader: %v", err)
	}

	if _, err := file.Ptr.Data(); err == nil {
		t.Fatal("expected lazy read to fail after closer is closed")
	}
}
