package KCES

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
)

func TestKCESAssetPskSamples(t *testing.T) {
	for _, path := range assetSamplePathsBySuffix(t, ".psk") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data := readAssetSampleFile(t, path)
			psk, err := serializationCOM3D2.ReadPsk(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("ReadPsk: %v", err)
			}
			var buf bytes.Buffer
			if err := psk.Dump(&buf); err != nil {
				t.Fatalf("Dump PSK: %v", err)
			}
			decoded, err := serializationCOM3D2.ReadPsk(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("re-read PSK: %v", err)
			}
			if !reflect.DeepEqual(decoded, psk) {
				t.Fatalf("PSK changed after decode/encode/decode: got %#v, want %#v", decoded, psk)
			}
		})
	}
}
