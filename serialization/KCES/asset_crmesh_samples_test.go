package KCES

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestKCESAssetCRMeshSamples(t *testing.T) {
	for _, path := range assetSamplePathsBySuffix(t, ".crmesh") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data := readAssetSampleFile(t, path)
			if len(data) < 12 || data[0] != 11 || string(data[1:12]) != "CR_MOD_MESH" {
				t.Fatalf("invalid crmesh prefix: % x", data[:min(len(data), 12)])
			}
			var mesh aba.PackedMesh
			if err := ct.DecodeMsgpack(data[12:], &mesh); err != nil {
				t.Fatalf("decode crmesh payload: %v", err)
			}
			payload, err := ct.EncodeMsgpack(&mesh)
			if err != nil {
				t.Fatalf("encode crmesh payload: %v", err)
			}
			var decoded aba.PackedMesh
			if err := ct.DecodeMsgpack(payload, &decoded); err != nil {
				t.Fatalf("re-decode crmesh payload: %v", err)
			}
			if !reflect.DeepEqual(&decoded, &mesh) {
				t.Fatalf("crmesh changed after decode/encode/decode: got %#v, want %#v", decoded, mesh)
			}
		})
	}
}
