package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/kcesfixtures"
)

func TestKCESExportedNeiSamples(t *testing.T) {
	for _, path := range kcesExportedAssetSamplePaths(t, ".nei") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data := readKCESExportedAssetSample(t, path)
			nei, err := ReadNei(bytes.NewReader(data), nil)
			if err != nil {
				t.Fatalf("ReadNei: %v", err)
			}
			var buf bytes.Buffer
			if err := nei.Dump(&buf); err != nil {
				t.Fatalf("Dump NEI: %v", err)
			}
			decoded, err := ReadNei(bytes.NewReader(buf.Bytes()), nil)
			if err != nil {
				t.Fatalf("re-read NEI: %v", err)
			}
			if !reflect.DeepEqual(decoded, nei) {
				t.Fatalf("NEI changed after decode/encode/decode: got %#v, want %#v", decoded, nei)
			}
		})
	}
}

func TestKCESExportedPskSamples(t *testing.T) {
	for _, path := range kcesExportedAssetSamplePaths(t, ".psk") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data := readKCESExportedAssetSample(t, path)
			psk, err := ReadPsk(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("ReadPsk: %v", err)
			}
			var buf bytes.Buffer
			if err := psk.Dump(&buf); err != nil {
				t.Fatalf("Dump PSK: %v", err)
			}
			decoded, err := ReadPsk(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("re-read PSK: %v", err)
			}
			if !reflect.DeepEqual(decoded, psk) {
				t.Fatalf("PSK changed after decode/encode/decode: got %#v, want %#v", decoded, psk)
			}
		})
	}
}

func kcesExportedAssetSamplePaths(t *testing.T, suffix string) []string {
	t.Helper()
	paths := kcesfixtures.AssetSamplePaths(t)
	var matches []string
	for _, path := range paths {
		name := strings.ToLower(filepath.Base(path))
		if strings.HasSuffix(name, ".meta.json") || strings.HasSuffix(name, ".typetree.json") {
			continue
		}
		if strings.ToLower(filepath.Ext(name)) == suffix {
			matches = append(matches, path)
		}
	}
	if len(matches) == 0 {
		t.Skipf("no KCES asset samples with suffix %s", suffix)
	}
	return matches
}

func readKCESExportedAssetSample(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read KCES asset sample %s: %v", path, err)
	}
	if len(data) == 0 {
		t.Fatalf("empty KCES asset sample %s", filepath.Base(path))
	}
	return data
}
