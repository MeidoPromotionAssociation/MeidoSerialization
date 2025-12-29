package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestTexService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.tex")
	if err != nil {
		t.Fatal(err)
	}

	s := &TexService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, "test.tex")
			imagePath := filepath.Join(tempDir, "test.png")
			backPath := filepath.Join(tempDir, "test_back.tex")

			// 1. Test ReadTexFile
			tex, err := s.ReadTexFile(inputPath)
			if err != nil {
				t.Fatalf("ReadTexFile failed: %v", err)
			}
			if tex == nil {
				t.Fatal("ReadTexFile returned nil")
			}

			// 2. Test WriteTexFile
			err = s.WriteTexFile(outputPath, tex)
			if err != nil {
				t.Fatalf("WriteTexFile failed: %v", err)
			}

			// 3. Test ConvertTexToImageAndWrite
			// forcePng 为 true
			err = s.ConvertTexToImageAndWrite(inputPath, imagePath, true)
			if err != nil {
				// 如果环境没有设置好，可能会失败，但在有 testdata 的情况下应该尽量运行
				t.Logf("ConvertTexToImageAndWrite warning (might need ImageMagick): %v", err)
			} else {
				// 4. Test ConvertImageToTexAndWrite
				err = s.ConvertImageToTexAndWrite(imagePath, "test_back", true, true, backPath)
				if err != nil {
					t.Errorf("ConvertImageToTexAndWrite failed: %v", err)
				}
			}

			// Re-read and verify consistency
			texRepack, err := s.ReadTexFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written tex failed: %v", err)
			}
			if !reflect.DeepEqual(tex, texRepack) {
				t.Errorf("data mismatch after write and re-read")
			}
		})
	}
}
