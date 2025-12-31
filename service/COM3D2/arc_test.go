package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
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
			arc, err := s.ReadArc(arcPath)
			if err != nil {
				t.Fatalf("ReadArc failed: %v", err)
			}
			if arc == nil {
				t.Fatal("ReadArc returned nil")
			}

			// 2. Test UnpackArc
			err = s.UnpackArc(arcPath, unpackDir)
			if err != nil {
				t.Fatalf("UnpackArc failed: %v", err)
			}

			// 3. Test PackArc
			err = s.PackArc(unpackDir, repackPath)
			if err != nil {
				t.Fatalf("PackArc failed: %v", err)
			}

			// 4. Test Read repackaged arc
			arcRepack, err := s.ReadArc(repackPath)
			if err != nil {
				t.Fatalf("Read repackaged arc failed: %v", err)
			}

			filesList := s.GetFileList(arc)
			filesRepack := s.GetFileList(arcRepack)
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
