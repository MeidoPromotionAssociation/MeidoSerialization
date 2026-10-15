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

func readTestTexture2D(t *testing.T, path string) *aba.Texture2DData {
	t.Helper()
	data, err := os.ReadFile(path)
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
	wantSize := aba.TextureChainSize(tex.TextureFormat, int64(tex.Width), int64(tex.Height), tex.MipCount)
	if int64(len(tex.ImageData)) != wantSize {
		t.Fatalf("image data is %d bytes but a %d level chain of %dx%d format %d needs %d", len(tex.ImageData), tex.MipCount, tex.Width, tex.Height, tex.TextureFormat, wantSize)
	}
	return tex
}

func readTestMipCount(t *testing.T, path string) int32 {
	t.Helper()
	return readTestTexture2D(t, path).MipCount
}

func writeTestPNGWithAlpha(t *testing.T, path string, width int32, height int32, alpha uint8) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			img.SetNRGBA(int(x), int(y), color.NRGBA{R: uint8(10 + x*40), G: uint8(20 + y*40), B: uint8(30 + x + y), A: alpha})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write PNG: %v", err)
	}
}

func TestConvertImageToTexture2DCompressPicksFormatByAlpha(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required to compress into DXT: %v", err)
	}
	tests := []struct {
		name       string
		alpha      uint8
		wantFormat int32
	}{
		{name: "fully_opaque_uses_dxt1", alpha: 255, wantFormat: aba.TextureFormatDXT1},
		{name: "partial_alpha_uses_dxt5", alpha: 128, wantFormat: aba.TextureFormatDXT5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pngPath := filepath.Join(tmpDir, "sample.png")
			texPath := filepath.Join(tmpDir, "sample.tex")
			writeTestPNGWithAlpha(t, pngPath, 16, 16, tc.alpha)

			service := &NativeUnityMediaService{}
			options := Texture2DRebuildOptions{MipMode: Texture2DMipModeAuto, Compress: true}
			if err := service.ConvertImageToTexture2DWithOptions(context.Background(), pngPath, texPath, options, 1<<20); err != nil {
				t.Fatalf("ConvertImageToTexture2DWithOptions: %v", err)
			}
			if got := readTestTexture2D(t, texPath).TextureFormat; got != tc.wantFormat {
				t.Fatalf("texture format = %d, want %d", got, tc.wantFormat)
			}
		})
	}
}

func TestConvertImageToTexture2DCompressKeepsMipCount(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required to compress into DXT: %v", err)
	}
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "sample.png")
	texPath := filepath.Join(tmpDir, "sample.tex")
	writeTestPNGWithAlpha(t, pngPath, 16, 16, 255)

	service := &NativeUnityMediaService{}
	options := Texture2DRebuildOptions{MipMode: Texture2DMipModeFull, Compress: true}
	if err := service.ConvertImageToTexture2DWithOptions(context.Background(), pngPath, texPath, options, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2DWithOptions: %v", err)
	}
	tex := readTestTexture2D(t, texPath)
	if tex.TextureFormat != aba.TextureFormatDXT1 {
		t.Fatalf("texture format = %d, want DXT1", tex.TextureFormat)
	}
	if tex.MipCount != aba.MaxMipCount(16, 16) {
		t.Fatalf("mip count = %d, want %d", tex.MipCount, aba.MaxMipCount(16, 16))
	}
}

func TestConvertImageToTexture2DMipModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     Texture2DMipMode
		wantMips int32
	}{
		{name: "none", mode: Texture2DMipModeNone, wantMips: 1},
		{name: "full", mode: Texture2DMipModeFull, wantMips: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pngPath := filepath.Join(tmpDir, "sample.png")
			texPath := filepath.Join(tmpDir, "sample.tex")
			writeTestPNG(t, pngPath, 8, 8)

			service := &NativeUnityMediaService{}
			if err := service.ConvertImageToTexture2DWithOptions(context.Background(), pngPath, texPath, Texture2DRebuildOptions{MipMode: tc.mode}, 1<<20); err != nil {
				t.Fatalf("ConvertImageToTexture2DWithOptions: %v", err)
			}
			if got := readTestMipCount(t, texPath); got != tc.wantMips {
				t.Fatalf("mip count = %d, want %d", got, tc.wantMips)
			}
		})
	}
}

func TestConvertImageToTexture2DDefaultsToNewFileSingleMip(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "fresh.png")
	texPath := filepath.Join(tmpDir, "fresh.tex")
	writeTestPNG(t, pngPath, 8, 8)

	service := &NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2D(context.Background(), pngPath, texPath, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2D: %v", err)
	}
	if got := readTestMipCount(t, texPath); got != 1 {
		t.Fatalf("mip count = %d, want 1 for a target that did not exist", got)
	}
}

func TestConvertImageToTexture2DPreservesExistingMipCount(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "sample.png")
	texPath := filepath.Join(tmpDir, "sample.tex")
	writeTestPNG(t, pngPath, 8, 8)

	service := &NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2DWithOptions(context.Background(), pngPath, texPath, Texture2DRebuildOptions{MipMode: Texture2DMipModeFull}, 1<<20); err != nil {
		t.Fatalf("seed with a full chain: %v", err)
	}
	if got := readTestMipCount(t, texPath); got != 4 {
		t.Fatalf("seeded mip count = %d, want 4", got)
	}

	if err := service.ConvertImageToTexture2D(context.Background(), pngPath, texPath, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2D: %v", err)
	}
	if got := readTestMipCount(t, texPath); got != 4 {
		t.Fatalf("preserved mip count = %d, want 4", got)
	}
}

func TestConvertImageToTexture2DClampsMipCountToImageSize(t *testing.T) {
	tmpDir := t.TempDir()
	largePath := filepath.Join(tmpDir, "sample.png")
	smallPath := filepath.Join(tmpDir, "small.png")
	texPath := filepath.Join(tmpDir, "sample.tex")
	writeTestPNG(t, largePath, 8, 8)
	writeTestPNG(t, smallPath, 2, 2)

	service := &NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2DWithOptions(context.Background(), largePath, texPath, Texture2DRebuildOptions{MipMode: Texture2DMipModeFull}, 1<<20); err != nil {
		t.Fatalf("seed with a full chain: %v", err)
	}
	if err := service.ConvertImageToTexture2D(context.Background(), smallPath, texPath, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2D: %v", err)
	}
	if got := readTestMipCount(t, texPath); got != 2 {
		t.Fatalf("clamped mip count = %d, want 2 for a 2x2 image", got)
	}
}

func TestConvertImageToTexture2DFallsBackToSingleMipOnUnreadableTarget(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "sample.png")
	texPath := filepath.Join(tmpDir, "sample.tex")
	writeTestPNG(t, pngPath, 4, 4)
	if err := os.WriteFile(texPath, []byte("not a native Unity object"), 0644); err != nil {
		t.Fatal(err)
	}

	service := &NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2D(context.Background(), pngPath, texPath, 1<<20); err != nil {
		t.Fatalf("ConvertImageToTexture2D: %v", err)
	}
	if got := readTestMipCount(t, texPath); got != 1 {
		t.Fatalf("mip count = %d, want 1 when the existing target cannot be probed", got)
	}
}

func TestConvertImageToTexture2DRejectsUnknownMipMode(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "sample.png")
	writeTestPNG(t, pngPath, 4, 4)

	service := &NativeUnityMediaService{}
	if err := service.ConvertImageToTexture2DWithOptions(context.Background(), pngPath, filepath.Join(tmpDir, "sample.tex"), Texture2DRebuildOptions{MipMode: Texture2DMipMode(99)}, 1<<20); err == nil {
		t.Fatal("an unknown mip mode was accepted")
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
