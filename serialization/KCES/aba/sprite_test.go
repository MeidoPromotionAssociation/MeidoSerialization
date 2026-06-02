package aba

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

func TestGetSpriteExport_Sample(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick not available: %v", err)
	}

	bundle, f := openAbaSample(t, "parts_personal002.aba")
	defer f.Close()

	files := make(map[string]*AssetsFile)
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		fileData, err := bundle.GetFileData(i)
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
	resolver := BundleAssetResolver(files)

	for _, af := range files {
		for _, info := range af.Metadata.AssetInfos {
			if info.TypeId != ClassIDSprite {
				continue
			}
			sprite, err := af.GetSpriteExportRange(&info, resolver, bundle.GetFileDataRangeByName)
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
			if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > sprite.Texture.Width || cfg.Height > sprite.Texture.Height {
				t.Fatalf("unexpected sprite png size for %s: %dx%d", sprite.Name, cfg.Width, cfg.Height)
			}
			return
		}
	}
	t.Fatal("no Sprite found in sample")
}
