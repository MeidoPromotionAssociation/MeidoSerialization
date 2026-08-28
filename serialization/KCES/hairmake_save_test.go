package KCES

import (
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

func TestHairMakeSaveRoundTripPreservesStringKeyStructure(t *testing.T) {
	guid := "hair-guid"
	value := &HairMakeSave{
		MixHair: &MixHairBuildData{
			Version: 3, GUID: &guid, Type: 2, Category: 4,
			Bunches: []*BunchDeformedData{{
				HairID: &guid, BunchNo: 7,
				Position: Vector3{X: 1, Y: 2, Z: 3}, Rotation: Vector4{W: 1}, Scale: Vector3{X: 1, Y: 1, Z: 1},
				DeformList: []*BunchDeformData{{
					BaseBox: &BaseDeformBoxData{Version: 1, DivisionCount: 4, IgnoreTriangles: []int32{1, 2}},
					Param: &BunchDeformParameter{
						Version: 1, DeformFlags: 31, LengthPowers: []float32{0.25},
						WidthH: []Vector2{{X: 1, Y: 2}}, MovePowers: []Vector3{{Z: 3}},
					},
				}},
			}},
		},
		SilhouettePreset: &HairSilhouettePresetParameter{Number: 2, Version: 9, Deform: 0.5, Blend: 0.25},
		SilhouetteData:   &HairSilhouetteParameter{XDivisionPoints: []int32{1, 2}, DeformValues: []Vector3{{X: 1}}},
		BuildVersion:     13500,
		GameVersion:      349000,
	}

	wire, err := EncodeHairMakeSave(value)
	if err != nil {
		t.Fatalf("EncodeHairMakeSave: %v", err)
	}
	var root map[string]codec.Raw
	if err := msgpack.DecodeMsgpack(wire, &root); err != nil {
		t.Fatalf("decode HairMakeSave root map: %v", err)
	}
	for _, key := range []string{"hair", "preset", "silhouette", "build_ver", "game_ver"} {
		if _, ok := root[key]; !ok {
			t.Fatalf("HairMakeSave root is missing key %q", key)
		}
	}
	if len(root) != 5 {
		t.Fatalf("HairMakeSave root key count = %d, want 5", len(root))
	}

	decoded, err := DecodeHairMakeSave(wire)
	if err != nil {
		t.Fatalf("DecodeHairMakeSave: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("HairMakeSave round trip mismatch\n got: %#v\nwant: %#v", decoded, value)
	}
}
