package KCES

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

func TestMaterialAssetsPreservesNestedIndexedObjectWireMetadata(t *testing.T) {
	rootFuture := codec.Raw{0xd4, 0x31, 0x7f}
	materialFuture := codec.Raw{0x81, 0xa1, 'x', 0xcc, 0x80}
	textureFuture := codec.Raw{0xd6, 0x22, 0, 0, 0, 7}
	wire := compressIndexedTestValue(t, []interface{}{
		"container.materialassets",
		[]interface{}{
			[]interface{}{int64(17)},
			[]interface{}{
				int64(1000), uint64(2), "a.mate", "shader",
				[]interface{}{
					[]interface{}{int64(3)},
					[]interface{}{int64(4), "tex.png", float32(0), float32(0), float32(1), float32(1), textureFuture},
				},
				[]interface{}{}, []interface{}{}, []interface{}{}, materialFuture,
			},
		},
		rootFuture,
	})

	assets, err := DecodeMaterialAssets(wire)
	if err != nil {
		t.Fatalf("DecodeMaterialAssets: %v", err)
	}
	assertIndexedMetadata(t, assets.IndexedObjectMetadata, 3, rootFuture)
	if len(assets.Assets) != 2 {
		t.Fatalf("assetArray length = %d, want 2", len(assets.Assets))
	}
	assertIndexedMetadata(t, assets.Assets[0].IndexedObjectMetadata, 1)
	assertIndexedMetadata(t, assets.Assets[1].IndexedObjectMetadata, 9, materialFuture)
	if len(assets.Assets[1].TextureProps) != 2 {
		t.Fatalf("textureProps length = %d, want 2", len(assets.Assets[1].TextureProps))
	}
	assertIndexedMetadata(t, assets.Assets[1].TextureProps[0].IndexedObjectMetadata, 1)
	assertIndexedMetadata(t, assets.Assets[1].TextureProps[1].IndexedObjectMetadata, 7, textureFuture)

	reencoded, err := EncodeMaterialAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMaterialAssets: %v", err)
	}
	root := decodeCompressedIndexedTestArray(t, reencoded)
	if len(root) != 3 || !rawMessagePackEqual(root[2], rootFuture) {
		t.Fatalf("MaterialAssets root wire did not retain future slot: % x", root)
	}
	materials := decodeIndexedTestArray(t, root[1])
	if len(decodeIndexedTestArray(t, materials[0])) != 1 {
		t.Fatal("short Material was widened during re-encoding")
	}
	fullMaterial := decodeIndexedTestArray(t, materials[1])
	if len(fullMaterial) != 9 || !rawMessagePackEqual(fullMaterial[8], materialFuture) {
		t.Fatalf("Material future wire was not retained: % x", fullMaterial)
	}
	textures := decodeIndexedTestArray(t, fullMaterial[4])
	if len(decodeIndexedTestArray(t, textures[0])) != 1 {
		t.Fatal("short TextureProp was widened during re-encoding")
	}
	fullTexture := decodeIndexedTestArray(t, textures[1])
	if len(fullTexture) != 7 || !rawMessagePackEqual(fullTexture[6], textureFuture) {
		t.Fatalf("TextureProp future wire was not retained: % x", fullTexture)
	}

	assets.Assets[0].FileName = "would-be-lost.mate"
	if _, err := EncodeMaterialAssets(assets); err == nil || !strings.Contains(err.Error(), "would discard fileName") {
		t.Fatalf("short Material populated-field error = %v", err)
	}
}

func TestPriorityMaterialAssetsPreservesIndexedObjectWireMetadata(t *testing.T) {
	rootFuture := codec.Raw{0xcc, 0x80}
	itemFuture := codec.Raw{0xd4, 0x19, 0x01}
	wire := compressIndexedTestValue(t, []interface{}{
		"container.pmatassets",
		[]interface{}{
			[]interface{}{int64(1000), uint64(1), "a.pmat", float32(2450), uint64(2), itemFuture},
		},
		rootFuture,
	})
	assets, err := DecodePriorityMaterialAssets(wire)
	if err != nil {
		t.Fatalf("DecodePriorityMaterialAssets: %v", err)
	}
	assertIndexedMetadata(t, assets.IndexedObjectMetadata, 3, rootFuture)
	assertIndexedMetadata(t, assets.Assets[0].IndexedObjectMetadata, 6, itemFuture)

	reencoded, err := EncodePriorityMaterialAssets(assets)
	if err != nil {
		t.Fatalf("EncodePriorityMaterialAssets: %v", err)
	}
	root := decodeCompressedIndexedTestArray(t, reencoded)
	items := decodeIndexedTestArray(t, root[1])
	item := decodeIndexedTestArray(t, items[0])
	if len(root) != 3 || !rawMessagePackEqual(root[2], rootFuture) || len(item) != 6 || !rawMessagePackEqual(item[5], itemFuture) {
		t.Fatalf("PriorityMaterial metadata was not retained: root=% x item=% x", root, item)
	}
}

func TestModelPreservesDeepIndexedObjectWireMetadata(t *testing.T) {
	modelFuture := codec.Raw{0xd4, 0x41, 0x01}
	rotationFuture := codec.Raw{0x81, 0xa1, 'f', 0x92, 0xc0, 0xc3}
	groupFuture := codec.Raw{0xd6, 0x42, 0, 0, 0, 9}
	modelWire := []interface{}{
		int64(1001), uint64(9), "x.model", "x.mmesh", "model",
		[]interface{}{
			[]interface{}{
				"root", int64(-1), false,
				[]interface{}{float32(1)},
				[]interface{}{float32(0), float32(0), float32(0), float32(1), rotationFuture},
				[]interface{}{float32(1), float32(1), float32(1)},
			},
		},
		[]interface{}{"root"},
		[]interface{}{"x.mate"},
		[]interface{}{},
		[]interface{}{
			true,
			map[string]interface{}{
				"group": []interface{}{"group", "start", "end", int64(10), []interface{}{}, groupFuture},
			},
		},
		int64(0),
		modelFuture,
	}
	wire := compressIndexedTestValue(t, modelWire)
	model, err := DecodeModel(wire)
	if err != nil {
		t.Fatalf("DecodeModel: %v", err)
	}
	assertIndexedMetadata(t, model.IndexedObjectMetadata, 12, modelFuture)
	if len(model.TransData) != 1 {
		t.Fatalf("transData length = %d, want 1", len(model.TransData))
	}
	assertIndexedMetadata(t, model.TransData[0].Pos.IndexedObjectMetadata, 1)
	assertIndexedMetadata(t, model.TransData[0].Rot.IndexedObjectMetadata, 5, rotationFuture)
	group := model.SkinThick.Groups["group"]
	assertIndexedMetadata(t, group.IndexedObjectMetadata, 6, groupFuture)

	reencoded, err := EncodeModel(model)
	if err != nil {
		t.Fatalf("EncodeModel: %v", err)
	}
	root := decodeCompressedIndexedTestArray(t, reencoded)
	if len(root) != 12 || !rawMessagePackEqual(root[11], modelFuture) {
		t.Fatalf("Model root future slot was not retained: % x", root)
	}
	transforms := decodeIndexedTestArray(t, root[5])
	transform := decodeIndexedTestArray(t, transforms[0])
	if len(decodeIndexedTestArray(t, transform[3])) != 1 {
		t.Fatal("short Vector3 position was widened")
	}
	rotation := decodeIndexedTestArray(t, transform[4])
	if len(rotation) != 5 || !rawMessagePackEqual(rotation[4], rotationFuture) {
		t.Fatalf("Vector4 future slot was not retained: % x", rotation)
	}

	model.TransData[0].Pos.Y = 2
	if _, err := EncodeModel(model); err == nil || !strings.Contains(err.Error(), "would discard y") {
		t.Fatalf("short Vector3 populated-field error = %v", err)
	}
}

func TestMenuAssetsPreservesNestedIndexedObjectWireMetadata(t *testing.T) {
	rootFuture := codec.Raw{0xd4, 0x51, 0x01}
	menuFuture := codec.Raw{0xcc, 0xfe}
	commandFuture := codec.Raw{0xd6, 0x52, 0, 0, 0, 3}
	menu := make([]interface{}, 31)
	menu[0] = int64(1005)
	menu[12] = []interface{}{
		[]interface{}{int64(7)},
		[]interface{}{int64(8), []interface{}{"arg"}, commandFuture},
	}
	menu[16] = map[uint64]interface{}{}
	menu = append(menu, menuFuture)
	wire := compressIndexedTestValue(t, []interface{}{
		"container.menuassets",
		[]interface{}{menu},
		rootFuture,
	})

	assets, err := DecodeMenuAssets(wire)
	if err != nil {
		t.Fatalf("DecodeMenuAssets: %v", err)
	}
	assertIndexedMetadata(t, assets.IndexedObjectMetadata, 3, rootFuture)
	assertIndexedMetadata(t, assets.Assets[0].IndexedObjectMetadata, 32, menuFuture)
	assertIndexedMetadata(t, assets.Assets[0].Commands[0].IndexedObjectMetadata, 1)
	assertIndexedMetadata(t, assets.Assets[0].Commands[1].IndexedObjectMetadata, 3, commandFuture)

	reencoded, err := EncodeMenuAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMenuAssets: %v", err)
	}
	root := decodeCompressedIndexedTestArray(t, reencoded)
	menus := decodeIndexedTestArray(t, root[1])
	menuSlots := decodeIndexedTestArray(t, menus[0])
	commands := decodeIndexedTestArray(t, menuSlots[12])
	if len(root) != 3 || !rawMessagePackEqual(root[2], rootFuture) || len(menuSlots) != 32 || !rawMessagePackEqual(menuSlots[31], menuFuture) {
		t.Fatalf("Menu root/future metadata was not retained: root=% x menu=% x", root, menuSlots)
	}
	if len(decodeIndexedTestArray(t, commands[0])) != 1 {
		t.Fatal("short Command was widened")
	}
	fullCommand := decodeIndexedTestArray(t, commands[1])
	if len(fullCommand) != 3 || !rawMessagePackEqual(fullCommand[2], commandFuture) {
		t.Fatalf("Command future slot was not retained: % x", fullCommand)
	}
}

func TestPayloadModelsPreserveIndexedObjectWireMetadata(t *testing.T) {
	t.Run("DynamicBoneStatus", func(t *testing.T) {
		rootFuture := codec.Raw{0xd4, 0x61, 0x01}
		frameFuture := codec.Raw{0xd6, 0x62, 0, 0, 0, 4}
		root := make([]interface{}, 16)
		root[0] = int64(1000)
		root[2] = []interface{}{
			[]interface{}{float32(0.25)},
			[]interface{}{float32(0.5), float32(1), float32(2), float32(3), frameFuture},
		}
		root = append(root, rootFuture)
		wire := lengthPrefixedIndexedTestValue(t, root)

		env, err := DecodeKCESPayload(wire, ".dbconf")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		assertIndexedMetadata(t, env.DynamicBone.IndexedObjectMetadata, 17, rootFuture)
		assertIndexedMetadata(t, env.DynamicBone.DampingKeyFrames[0].IndexedObjectMetadata, 1)
		assertIndexedMetadata(t, env.DynamicBone.DampingKeyFrames[1].IndexedObjectMetadata, 5, frameFuture)

		reencoded, err := EncodeKCESPayload(env)
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		rootSlots := decodeLengthPrefixedIndexedTestArray(t, reencoded)
		frames := decodeIndexedTestArray(t, rootSlots[2])
		fullFrame := decodeIndexedTestArray(t, frames[1])
		if len(rootSlots) != 17 || !rawMessagePackEqual(rootSlots[16], rootFuture) || len(decodeIndexedTestArray(t, frames[0])) != 1 || len(fullFrame) != 5 || !rawMessagePackEqual(fullFrame[4], frameFuture) {
			t.Fatalf("DynamicBoneStatus metadata was not retained: root=% x frame=% x", rootSlots, fullFrame)
		}
	})

	t.Run("ClothParams", func(t *testing.T) {
		rootFuture := codec.Raw{0xd4, 0x63, 0x01}
		bezierFuture := codec.Raw{0xcc, 0xa5}
		root := make([]interface{}, 83)
		root[0] = []interface{}{float32(0.02)}
		root[1] = []interface{}{float32(1), float32(1), true, float32(0), false, bezierFuture}
		root = append(root, rootFuture)
		wire := lengthPrefixedIndexedTestValue(t, root)

		env, err := DecodeKCESPayload(wire, ".dsbconf")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		assertIndexedMetadata(t, env.ClothParams.IndexedObjectMetadata, 84, rootFuture)
		assertIndexedMetadata(t, env.ClothParams.Radius.IndexedObjectMetadata, 1)
		assertIndexedMetadata(t, env.ClothParams.Mass.IndexedObjectMetadata, 6, bezierFuture)

		reencoded, err := EncodeKCESPayload(env)
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		rootSlots := decodeLengthPrefixedIndexedTestArray(t, reencoded)
		mass := decodeIndexedTestArray(t, rootSlots[1])
		if len(rootSlots) != 84 || !rawMessagePackEqual(rootSlots[83], rootFuture) || len(decodeIndexedTestArray(t, rootSlots[0])) != 1 || len(mass) != 6 || !rawMessagePackEqual(mass[5], bezierFuture) {
			t.Fatalf("ClothParams metadata was not retained: root=% x mass=% x", rootSlots, mass)
		}
	})
}

func TestSparseIndexedSlotsRemainRawAndJSONEditable(t *testing.T) {
	t.Run("Menu Key24", func(t *testing.T) {
		gap := codec.Raw{0x82, 0xa1, 'x', 0x01, 0xa1, 'x', 0x02}
		menu := make([]interface{}, 31)
		menu[0] = int64(1005)
		menu[12] = []interface{}{}
		menu[16] = map[uint64]interface{}{}
		menu[24] = gap
		wire := compressIndexedTestValue(t, []interface{}{"menuassets", []interface{}{menu}})
		assets, err := DecodeMenuAssets(wire)
		if err != nil {
			t.Fatalf("DecodeMenuAssets: %v", err)
		}
		if !bytes.Equal(assets.Assets[0].Reserved24, gap) {
			t.Fatalf("reserved24 = % x, want % x", assets.Assets[0].Reserved24, gap)
		}

		jsonData, err := json.Marshal(assets)
		if err != nil {
			t.Fatalf("marshal MenuAssets JSON: %v", err)
		}
		var edited MenuAssets
		if err := json.Unmarshal(jsonData, &edited); err != nil {
			t.Fatalf("unmarshal MenuAssets JSON: %v", err)
		}
		if !bytes.Equal(edited.Assets[0].Reserved24, gap) {
			t.Fatalf("JSON reserved24 = % x, want % x", edited.Assets[0].Reserved24, gap)
		}
		reencoded, err := EncodeMenuAssets(&edited)
		if err != nil {
			t.Fatalf("EncodeMenuAssets: %v", err)
		}
		root := decodeCompressedIndexedTestArray(t, reencoded)
		menus := decodeIndexedTestArray(t, root[1])
		menuSlots := decodeIndexedTestArray(t, menus[0])
		if !rawMessagePackEqual(menuSlots[24], gap) {
			t.Fatalf("re-encoded Key(24) = % x, want % x", menuSlots[24], gap)
		}
	})

	t.Run("Cloth Key4 Key5 Key56", func(t *testing.T) {
		gap04 := codec.Raw{0xd4, 0x71, 0x01}
		gap05 := codec.Raw{0x82, 0xa1, 'k', 0x01, 0xa1, 'k', 0x02}
		gap56 := codec.Raw{0xcc, 0x80}
		root := make([]interface{}, 83)
		root[4], root[5], root[56] = gap04, gap05, gap56
		env, err := DecodeKCESPayload(lengthPrefixedIndexedTestValue(t, root), ".dsbconf")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		if !bytes.Equal(env.ClothParams.Reserved04, gap04) || !bytes.Equal(env.ClothParams.Reserved05, gap05) || !bytes.Equal(env.ClothParams.Reserved56, gap56) {
			t.Fatalf("decoded cloth gaps = % x / % x / % x", env.ClothParams.Reserved04, env.ClothParams.Reserved05, env.ClothParams.Reserved56)
		}
		reencoded, err := EncodeKCESPayload(env)
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		slots := decodeLengthPrefixedIndexedTestArray(t, reencoded)
		if !rawMessagePackEqual(slots[4], gap04) || !rawMessagePackEqual(slots[5], gap05) || !rawMessagePackEqual(slots[56], gap56) {
			t.Fatalf("re-encoded cloth gaps = % x / % x / % x", slots[4], slots[5], slots[56])
		}
	})
}

func TestSparseIndexedSlotRejectsMalformedRawMessagePack(t *testing.T) {
	assets := &MenuAssets{
		Assets: []Menu{{Reserved24: RawMessagePackSlot{0xc1}}},
	}
	if _, err := EncodeMenuAssets(assets); err == nil {
		t.Fatal("malformed raw sparse slot was accepted")
	}
}

func compressIndexedTestValue(t *testing.T, value interface{}) []byte {
	t.Helper()
	h := &codec.MsgpackHandle{}
	h.Raw = true
	h.MaxDepth = 256
	var messagePack []byte
	err := codec.NewEncoderBytes(&messagePack, h).Encode(value)
	if err != nil {
		t.Fatalf("encode indexed test MessagePack: %v", err)
	}
	compressed, err := ct.CompressLz4BlockArray(messagePack)
	if err != nil {
		t.Fatalf("compress indexed test MessagePack: %v", err)
	}
	return compressed
}

func lengthPrefixedIndexedTestValue(t *testing.T, value interface{}) []byte {
	t.Helper()
	return AddLengthPrefix(compressIndexedTestValue(t, value))
}

func decodeCompressedIndexedTestArray(t *testing.T, data []byte) []codec.Raw {
	t.Helper()
	messagePack, err := ct.DecompressLz4BlockArray(data)
	if err != nil {
		t.Fatalf("decompress indexed test wire: %v", err)
	}
	return decodeIndexedTestArray(t, messagePack)
}

func decodeLengthPrefixedIndexedTestArray(t *testing.T, data []byte) []codec.Raw {
	t.Helper()
	compressed, _, err := StripLengthPrefix(data)
	if err != nil {
		t.Fatalf("strip indexed test length prefix: %v", err)
	}
	return decodeCompressedIndexedTestArray(t, compressed)
}

func decodeIndexedTestArray(t *testing.T, data []byte) []codec.Raw {
	t.Helper()
	if len(data) == 0 {
		data = []byte{0xc0}
	}
	var slots []codec.Raw
	consumed, err := ct.DecodeMsgpackWithConsumed(data, &slots)
	if err != nil {
		t.Fatalf("decode indexed test array: %v; wire=% x", err, data)
	}
	if consumed != len(data) {
		t.Fatalf("indexed test array consumed %d of %d bytes", consumed, len(data))
	}
	return slots
}

func assertIndexedMetadata(t *testing.T, metadata *IndexedObjectMetadata, count int, future ...[]byte) {
	t.Helper()
	if metadata == nil || metadata.FieldCount == nil || *metadata.FieldCount != count {
		t.Fatalf("indexed metadata = %#v, want fieldCount %d", metadata, count)
	}
	if len(metadata.FutureSlots) != len(future) {
		t.Fatalf("futureSlots length = %d, want %d", len(metadata.FutureSlots), len(future))
	}
	for index := range future {
		if !bytes.Equal(metadata.FutureSlots[index], future[index]) {
			t.Fatalf("futureSlots[%d] = % x, want % x", index, metadata.FutureSlots[index], future[index])
		}
	}
}

func rawMessagePackEqual(got codec.Raw, want []byte) bool {
	if len(got) == 0 {
		got = codec.Raw{0xc0}
	}
	return bytes.Equal(got, want)
}
