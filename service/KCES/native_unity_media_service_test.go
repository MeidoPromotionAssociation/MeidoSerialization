package KCES

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

func writeTestPNG(t *testing.T, path string, width int32, height int32) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	want := make([]byte, 0, int(width)*int(height)*4)
	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			pixel := color.NRGBA{R: uint8(10 + x*40), G: uint8(20 + y*40), B: uint8(30 + x + y), A: 255}
			img.SetNRGBA(int(x), int(y), pixel)
			want = append(want, pixel.R, pixel.G, pixel.B, pixel.A)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write PNG: %v", err)
	}
	return want
}

func TestConvertImageToTexture2DRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "sample.png")
	texPath := filepath.Join(tmpDir, "sample.tex")
	want := writeTestPNG(t, pngPath, 3, 2)

	service := &NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2D(context.Background(), pngPath, texPath, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2D: %v", err)
	}
	if !IsKCESNativeTexture2DFile(texPath) {
		t.Fatal("output is not detected as a native Texture2D file")
	}

	data, err := os.ReadFile(texPath)
	if err != nil {
		t.Fatal(err)
	}
	object, err := aba.ReadTexture2D(data)
	if err != nil {
		t.Fatalf("ReadTexture2D: %v", err)
	}
	af, info, err := object.AssetsFileView()
	if err != nil {
		t.Fatal(err)
	}
	tex, err := af.GetTexture2DDataRange(info, nil)
	if err != nil {
		t.Fatalf("GetTexture2DDataRange: %v", err)
	}
	if tex.Name != "sample.tex" {
		t.Errorf("m_Name got %q, want %q", tex.Name, "sample.tex")
	}
	if tex.Width != 3 || tex.Height != 2 {
		t.Errorf("dimensions got %dx%d, want 3x2", tex.Width, tex.Height)
	}
	if tex.TextureFormat != aba.TextureFormatRGBA32 {
		t.Errorf("texture format got %d, want RGBA32", tex.TextureFormat)
	}
	if tex.MipCount != 1 {
		t.Errorf("mip count got %d, want 1", tex.MipCount)
	}
	if !bytes.Equal(tex.ImageData, want) {
		t.Errorf("image data mismatch: got %v, want %v", tex.ImageData, want)
	}
}

func TestConvertImageToTexture2DNameFollowsPackRules(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "icon.png")
	outputPath := filepath.Join(tmpDir, "icon.texture2d")
	writeTestPNG(t, pngPath, 1, 1)

	service := &NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2D(context.Background(), pngPath, outputPath, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2D: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	object, err := aba.ReadTexture2D(data)
	if err != nil {
		t.Fatalf("ReadTexture2D: %v", err)
	}
	af, info, err := object.AssetsFileView()
	if err != nil {
		t.Fatal(err)
	}
	tex, err := af.GetTexture2DDataRange(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tex.Name != "icon" {
		t.Errorf("m_Name got %q, want %q for a .texture2d output", tex.Name, "icon")
	}
}

func TestConvertImageToTexture2DRespectsOutputLimit(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "big.png")
	writeTestPNG(t, pngPath, 8, 8)

	service := &NativeUnityMediaService{}
	err := service.ConvertImageToTexture2D(context.Background(), pngPath, filepath.Join(tmpDir, "big.tex"), 16)
	if err == nil {
		t.Fatal("undersized output limit was accepted")
	}
}
