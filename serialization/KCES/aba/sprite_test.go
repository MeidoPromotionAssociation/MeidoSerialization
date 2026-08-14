package aba

import (
	"bytes"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

func TestReadPPtrRejectsFileIDOutsideInt32(t *testing.T) {
	for _, fileID := range []int64{math.MinInt32 - 1, math.MaxInt32 + 1} {
		value := &TypeTreeValue{Children: []*TypeTreeValue{
			{Name: "m_FileID", Value: fileID},
			{Name: "m_PathID", Value: int64(1)},
		}}
		if _, ok := readPPtr(value); ok {
			t.Fatalf("readPPtr accepted file ID %d outside Int32", fileID)
		}
	}
}

func TestGetSpriteExport_Sample(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick not available: %v", err)
	}

	abaFile, f := openAbaSample(t, "parts_personal002.aba")
	defer f.Close()

	files := make(map[string]*AssetsFile)
	for i, dir := range abaFile.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		fileData, err := abaFile.GetFileData(int64(i))
		if err != nil {
			t.Fatal(err)
		}
		af, err := ReadAssetsFile(fileData)
		if err != nil {
			t.Fatal(err)
		}
		files[dir.Name] = af
		files[filepath.Base(dir.Name)] = af
	}
	resolver := AbaAssetResolver(files)

	for _, af := range files {
		for _, info := range af.Metadata.AssetInfos {
			if info.TypeId != ClassIDSprite {
				continue
			}
			sprite, err := af.GetSpriteExportRange(&info, resolver, abaFile.GetFileDataRangeByName)
			if err != nil {
				t.Fatalf("GetSpriteExport pathId=%d: %v", info.PathId, err)
			}
			if sprite.Texture == nil || sprite.Texture.Width != 256 || sprite.Texture.Height != 256 {
				t.Fatalf("unexpected texture for %s: %#v", sprite.Name, sprite.Texture)
			}
			if sprite.Rect.Width <= 0 || sprite.Rect.Height <= 0 {
				t.Fatalf("invalid sprite rect for %s: %+v", sprite.Name, sprite.Rect)
			}
			outPath := filepath.Join(t.TempDir(), sprite.Name+".png")
			if err := WriteSpritePNG(sprite, outPath); err != nil {
				t.Fatalf("WriteSpritePNG %s: %v", sprite.Name, err)
			}
			f, err := os.Open(outPath)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := png.DecodeConfig(f)
			f.Close()
			if err != nil {
				t.Fatalf("DecodeConfig %s: %v", sprite.Name, err)
			}
			if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width) > int64(sprite.Texture.Width) || int64(cfg.Height) > int64(sprite.Texture.Height) {
				t.Fatalf("unexpected sprite png size for %s: %dx%d", sprite.Name, cfg.Width, cfg.Height)
			}
			return
		}
	}
	t.Fatal("no Sprite found in sample")
}

func TestWriteSpritePNGToCropsLowerLeftOriginRectFromBottomUpTexture(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick not available: %v", err)
	}
	green := []byte{0, 255, 0, 255}
	red := []byte{255, 0, 0, 255}
	// Unity stores rows bottom-up, so the first two stored rows carry the green 2x2 patch at the lower-left corner.
	var payload []byte
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			if row < 2 && column < 2 {
				payload = append(payload, green...)
				continue
			}
			payload = append(payload, red...)
		}
	}
	sprite := &SpriteExport{
		Name: "lower_left",
		Texture: &Texture2DData{
			Name:          "orientation.tex",
			Width:         4,
			Height:        4,
			TextureFormat: TextureFormatRGBA32,
			MipCount:      1,
			ImageData:     payload,
		},
		Rect: SpriteRect{X: 0, Y: 0, Width: 2, Height: 2},
	}
	var out bytes.Buffer
	if err := WriteSpritePNGTo(sprite, &out); err != nil {
		t.Fatalf("WriteSpritePNGTo: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Fatalf("cropped sprite is %dx%d, want 2x2", bounds.Dx(), bounds.Dy())
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := decoded.At(x, y).RGBA()
			if g <= r || g <= b {
				t.Fatalf("pixel (%d,%d) is not green: r=%d g=%d b=%d", x, y, r>>8, g>>8, b>>8)
			}
		}
	}
}
