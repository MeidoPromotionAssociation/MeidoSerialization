package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPMatService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.pmat")
	if err != nil {
		t.Fatal(err)
	}

	s := &PMatService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, "test.pmat")
			jsonPath := filepath.Join(tempDir, "test.pmat.json")
			backPath := filepath.Join(tempDir, "test_back.pmat")

			// 1. Test ReadPMatFile
			pmat, err := s.ReadPMatFile(inputPath)
			if err != nil {
				t.Fatalf("ReadPMatFile failed: %v", err)
			}
			if pmat == nil {
				t.Fatal("ReadPMatFile returned nil")
			}

			// 2. Test WritePMatFile
			err = s.WritePMatFile(outputPath, pmat)
			if err != nil {
				t.Fatalf("WritePMatFile failed: %v", err)
			}

			// 3. Test ConvertPMatToJson
			err = s.ConvertPMatToJson(inputPath, jsonPath)
			if err != nil {
				t.Fatalf("ConvertPMatToJson failed: %v", err)
			}

			// 4. Test ConvertJsonToPMat
			err = s.ConvertJsonToPMat(jsonPath, backPath)
			if err != nil {
				t.Fatalf("ConvertJsonToPMat failed: %v", err)
			}

			// Re-read and verify consistency
			pmatBack, err := s.ReadPMatFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted pmat failed: %v", err)
			}
			// Sync Hash because service layer recalculates it during WritePMatFile
			pmat.Hash = pmatBack.Hash
			if !reflect.DeepEqual(pmat, pmatBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			pmatRepack, err := s.ReadPMatFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written pmat failed: %v", err)
			}
			pmat.Hash = pmatRepack.Hash
			if !reflect.DeepEqual(pmat, pmatRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}
