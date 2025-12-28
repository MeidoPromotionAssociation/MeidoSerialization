package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTexService(t *testing.T) {
	s := &TexService{}
	inputPath := "../../testdata/test.tex"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.tex not found, skipping test")
	}

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

	t.Logf("tex version: %v", tex.Version)
	t.Logf("tex TextureFormat: %v", tex.TextureFormat)

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
		t.Errorf("ConvertTexToImageAndWrite warning (might need ImageMagick): %v", err)
	} else {
		// 4. Test ConvertImageToTexAndWrite
		err = s.ConvertImageToTexAndWrite(imagePath, "test_back", true, true, backPath)
		if err != nil {
			t.Errorf("ConvertImageToTexAndWrite failed: %v", err)
		}
	}

	// Optional: Re-read and verify
	texRepack, err := s.ReadTexFile(outputPath)
	if err != nil {
		t.Fatalf("Read re-written tex failed: %v", err)
	}
	if texRepack.Signature != tex.Signature {
		t.Errorf("Signature mismatch: %s != %s", texRepack.Signature, tex.Signature)
	}
}

func TestTexService2(t *testing.T) {
	s := &TexService{}
	inputPath := "../../testdata/test2.tex"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skip("test.tex not found, skipping test")
	}

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

	t.Logf("tex version: %v", tex.Version)
	t.Logf("tex TextureFormat: %v", tex.TextureFormat)

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
		t.Errorf("ConvertTexToImageAndWrite warning (might need ImageMagick): %v", err)
	} else {
		// 4. Test ConvertImageToTexAndWrite
		err = s.ConvertImageToTexAndWrite(imagePath, "test_back", true, true, backPath)
		if err != nil {
			t.Errorf("ConvertImageToTexAndWrite failed: %v", err)
		}
	}

	// Optional: Re-read and verify
	texRepack, err := s.ReadTexFile(outputPath)
	if err != nil {
		t.Fatalf("Read re-written tex failed: %v", err)
	}
	if texRepack.Signature != tex.Signature {
		t.Errorf("Signature mismatch: %s != %s", texRepack.Signature, tex.Signature)
	}
}
