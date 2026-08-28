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

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/tools"
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
	if !bytes.Equal(tex.ImageData, aba.FlipRGBA32Rows(3, 2, want)) {
		t.Errorf("image data mismatch: got %v, want %v", tex.ImageData, aba.FlipRGBA32Rows(3, 2, want))
	}
	// Unity stores textures bottom-up, so the inline payload must be the vertical flip of the PNG row order.
	if bytes.Equal(tex.ImageData, want) {
		t.Error("image data kept PNG top-down row order instead of Unity bottom-up order")
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

func TestPackDirectoryRecognizesNativeTexture2DOutsideTypeDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	contentDir := filepath.Join(tmpDir, "mod_root")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatal(err)
	}
	pngPath := filepath.Join(tmpDir, "source.png")
	writeTestPNG(t, pngPath, 2, 2)
	texPath := filepath.Join(contentDir, "dress078___test_1.tex")
	service := &NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2D(context.Background(), pngPath, texPath, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2D: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "sample.menuassets"), []byte("menu"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := (&PackService{}).PackToAbaAndCt(contentDir, "native_root_pack"); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "native_root_pack.aba"))
	if err != nil {
		t.Fatal(err)
	}
	abaFile, err := aba.ReadAba(bytes.NewReader(abaData))
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := abaFile.GetFileData(0)
	if err != nil {
		t.Fatal(err)
	}
	af, err := aba.ReadAssetsFile(serialized)
	if err != nil {
		t.Fatal(err)
	}
	var texType int32
	for _, entry := range af.GetAssetEntries() {
		if entry.Name == "dress078___test_1.tex" {
			texType = entry.TypeId
		}
	}
	if texType != aba.ClassIDTexture2D {
		t.Fatalf("native .tex outside Texture2D directory got ClassID %d, want Texture2D", texType)
	}
}

func TestConvertImageToTexture2DAndBackPreservesOrientation(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick unavailable: %v", err)
	}
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "orientation.png")
	source := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	top := color.NRGBA{B: 255, A: 255}
	bottom := color.NRGBA{R: 255, A: 255}
	source.SetNRGBA(0, 0, top)
	source.SetNRGBA(1, 0, top)
	source.SetNRGBA(0, 1, bottom)
	source.SetNRGBA(1, 1, bottom)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	if err := os.WriteFile(sourcePath, encoded.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	service := &NativeUnityMediaService{}
	texPath := filepath.Join(tmpDir, "orientation.tex")
	if err := service.ConvertImageToTexture2D(context.Background(), sourcePath, texPath, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2D: %v", err)
	}
	roundTripPath := filepath.Join(tmpDir, "orientation_roundtrip.png")
	if err := service.ConvertTexture2DToImage(context.Background(), texPath, roundTripPath, "png", 1<<20); err != nil {
		t.Fatalf("ConvertTexture2DToImage: %v", err)
	}
	roundTripData, err := os.ReadFile(roundTripPath)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(roundTripData))
	if err != nil {
		t.Fatalf("decode round-trip PNG: %v", err)
	}
	for _, tc := range []struct {
		x    int
		y    int
		want color.NRGBA
	}{{0, 0, top}, {1, 0, top}, {0, 1, bottom}, {1, 1, bottom}} {
		r, g, b, a := decoded.At(tc.x, tc.y).RGBA()
		got := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
		if got != tc.want {
			t.Errorf("pixel (%d,%d) got %+v, want %+v", tc.x, tc.y, got, tc.want)
		}
	}
}
