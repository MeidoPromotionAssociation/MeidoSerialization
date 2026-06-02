package KCES

import (
	"bytes"
	"image/png"
	"path/filepath"
	"testing"
)

func TestKCESAssetPNGSamples(t *testing.T) {
	for _, path := range assetSamplePathsBySuffix(t, ".png") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data := readAssetSampleFile(t, path)
			cfg, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("DecodeConfig PNG: %v", err)
			}
			if cfg.Width <= 0 || cfg.Height <= 0 {
				t.Fatalf("invalid PNG dimensions: %dx%d", cfg.Width, cfg.Height)
			}
		})
	}
}
