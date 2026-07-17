package KCES

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

func TestPublicDecodersRejectCLRInt32Overflow(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot carry an out-of-Int32 MessagePack value")
	}
	overflow := int(int64(math.MaxInt32) + 1)

	menuWire := mustEncodeUncheckedCompressed(t, &MenuAssets{Assets: []Menu{{
		Version:      overflow,
		CategoryText: "null_mpn",
		ColorSetText: "null_mpn",
	}}})
	materialWire := mustEncodeUncheckedCompressed(t, &MaterialAssets{Assets: []Material{{Version: overflow}}})
	priorityWire := mustEncodeUncheckedCompressed(t, &PriorityMaterialAssets{Assets: []PriorityMaterial{{Version: overflow}}})
	model := validModelForInt32Test()
	model.Version = overflow
	modelWire := mustEncodeUncheckedCompressed(t, &model)
	modelAssetsWire := mustEncodeUncheckedCompressed(t, &ModelAssets{Assets: []Model{model}})

	for _, test := range []struct {
		name   string
		decode func() error
		path   string
	}{
		{name: "menu assets", path: "assetArray[0].version", decode: func() error { _, err := DecodeMenuAssets(menuWire); return err }},
		{name: "material assets", path: "assetArray[0].version", decode: func() error { _, err := DecodeMaterialAssets(materialWire); return err }},
		{name: "priority material assets", path: "assetArray[0].version", decode: func() error { _, err := DecodePriorityMaterialAssets(priorityWire); return err }},
		{name: "priority material raw", path: "version", decode: func() error {
			_, err := DecodePriorityMaterial([]interface{}{int64(overflow), uint64(1), "x.pmat", float64(0), uint64(2)})
			return err
		}},
		{name: "model", path: "version", decode: func() error { _, err := DecodeModel(modelWire); return err }},
		{name: "model assets", path: "assetArray[0].version", decode: func() error { _, err := DecodeModelAssets(modelAssetsWire); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.decode()
			if err == nil || !strings.Contains(err.Error(), "Int32") || !strings.Contains(err.Error(), test.path) {
				t.Fatalf("decoder error=%v, want Int32 rejection at %q", err, test.path)
			}
		})
	}
}

func TestPresetDecoderRejectsCLRInt32Overflow(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot carry an out-of-Int32 MessagePack value")
	}
	overflow := int(int64(math.MaxInt32) + 1)
	validCore := *validKCESPresetCoreForTest(t)

	for _, test := range []struct {
		name string
		core KCESPresetCore
		meta *KCESPresetMeta
		path string
	}{
		{name: "maiddata", core: func() KCESPresetCore { value := validCore; value.Version = overflow; return value }(), path: "maidData.version"},
		{name: "meta", core: validCore, meta: &KCESPresetMeta{Version: overflow, Data: map[string]string{}}, path: "meta.version"},
	} {
		t.Run(test.name, func(t *testing.T) {
			wire := mustBuildUncheckedPreset(t, &test.core, test.meta)
			_, err := DecodeKCESPreset(wire)
			if err == nil || !strings.Contains(err.Error(), "Int32") || !strings.Contains(err.Error(), test.path) {
				t.Fatalf("DecodeKCESPreset error=%v, want Int32 rejection at %q", err, test.path)
			}
		})
	}
}

func TestPayloadDecodersRejectCLRInt32Overflow(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot carry an out-of-Int32 MessagePack value")
	}
	overflow := int(int64(math.MaxInt32) + 1)

	dynamic := NewDynamicBoneStatus()
	dynamic.Version = overflow
	cloth := NewClothParams()
	cloth.BendDistanceMaxCount = overflow
	generic := newGenericColliderInt32Envelope().ColliderPackage
	generic.Version = overflow
	limb := newLimbColliderInt32Envelope().LimbCollider
	limb.Version = overflow
	ik := newIKColliderInt32Envelope().IKCollider
	ik.Version = overflow

	for _, test := range []struct {
		name      string
		extension string
		value     codec.Selfer
		path      string
	}{
		{name: "dynamic bone", extension: ".dbconf", value: dynamic, path: "dynamicBoneStatus.version"},
		{name: "cloth", extension: ".dsbconf", value: cloth, path: "clothParams.bendDistanceMaxCount"},
		{name: "generic collider", extension: ".dbcol", value: generic, path: "colliderPackage.version"},
		{name: "limb collider", extension: ".limbcol", value: limb, path: "limbColliderPackage.version"},
		{name: "IK collider", extension: ".ikcol", value: ik, path: "ikColliderPackage.version"},
	} {
		t.Run(test.name, func(t *testing.T) {
			wire := mustEncodeUncheckedPayload(t, test.value)
			_, err := DecodeKCESPayload(wire, test.extension)
			if err == nil || !strings.Contains(err.Error(), "Int32") || !strings.Contains(err.Error(), test.path) {
				t.Fatalf("DecodeKCESPayload error=%v, want Int32 rejection at %q", err, test.path)
			}
		})
	}
}

func mustEncodeUncheckedCompressed(t *testing.T, value interface{}) []byte {
	t.Helper()
	wire, err := encodeCompressedMsgpack(value, "unchecked Int32 test")
	if err != nil {
		t.Fatalf("encode unchecked compressed value: %v", err)
	}
	return wire
}

func mustEncodeUncheckedPayload(t *testing.T, value codec.Selfer) []byte {
	t.Helper()
	msgpack, err := ct.EncodeIndexedMsgpack(value)
	if err != nil {
		t.Fatalf("encode unchecked payload MessagePack: %v", err)
	}
	compressed, err := ct.CompressLz4BlockArray(msgpack)
	if err != nil {
		t.Fatalf("compress unchecked payload: %v", err)
	}
	return AddLengthPrefix(compressed)
}

func mustBuildUncheckedPreset(t *testing.T, core *KCESPresetCore, meta *KCESPresetMeta) []byte {
	t.Helper()
	table := &ct.ContentTable{
		Version: kcesPresetVersion,
		Raw:     make([]byte, ct.HeaderSize),
		Files:   make(map[string]ct.VirtualFile),
	}
	table.AddFile(kcesPresetThumbnailFile, []byte("png"))
	table.AddFile(kcesPresetMaidDataFile, mustEncodeUncheckedCompressed(t, core))
	if meta != nil {
		table.AddFile(kcesPresetMetaFile, mustEncodeUncheckedCompressed(t, meta))
	}
	var out bytes.Buffer
	if err := ct.WriteContentTable(&out, table); err != nil {
		t.Fatalf("build unchecked preset: %v", err)
	}
	return out.Bytes()
}
