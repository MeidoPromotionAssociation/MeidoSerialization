package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArcService(t *testing.T) {
	s := &ArcService{}
	arcPath := "../../testdata/test.arc"
	if _, err := os.Stat(arcPath); os.IsNotExist(err) {
		t.Skip("test.arc not found, skipping test")
	}

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

	files := s.GetFileList(arc)
	filesRepack := s.GetFileList(arcRepack)
	if len(files) != len(filesRepack) {
		t.Errorf("File count mismatch: %d != %d", len(files), len(filesRepack))
	}
}
