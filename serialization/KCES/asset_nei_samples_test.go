package KCES

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
)

func TestKCESAssetNeiSamples(t *testing.T) {
	for _, path := range assetSamplePathsBySuffix(t, ".nei") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data := readAssetSampleFile(t, path)
			nei, err := serializationCOM3D2.ReadNei(bytes.NewReader(data), nil)
			if err != nil {
				t.Fatalf("ReadNei: %v", err)
			}
			var buf bytes.Buffer
			if err := nei.Dump(&buf); err != nil {
				t.Fatalf("Dump NEI: %v", err)
			}
			decoded, err := serializationCOM3D2.ReadNei(bytes.NewReader(buf.Bytes()), nil)
			if err != nil {
				t.Fatalf("re-read NEI: %v", err)
			}
			if !reflect.DeepEqual(decoded, nei) {
				t.Fatalf("NEI changed after decode/encode/decode: got %#v, want %#v", decoded, nei)
			}
		})
	}
}
