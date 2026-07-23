package KCES

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

func TestMaterialAssetsPreservesNilStringsAndClassArrayElements(t *testing.T) {
	material := []interface{}{
		int64(1000), uint64(1), nil, nil,
		[]interface{}{
			nil,
			[]interface{}{int64(1), nil, float32(0), float32(0), float32(1), float32(1)},
		},
		[]interface{}{nil},
		[]interface{}{},
		[]interface{}{},
	}
	wire := compressIndexedTestValue(t, []interface{}{
		nil,
		[]interface{}{nil, material},
	})
	assets, err := DecodeMaterialAssets(wire)
	if err != nil {
		t.Fatalf("DecodeMaterialAssets: %v", err)
	}
	assertNilSlot(t, assets.IndexedObjectMetadata, 0)
	assertNullElement(t, assets.IndexedObjectMetadata, 1, 0)
	if assets.FileName != "" || len(assets.Assets) != 2 {
		t.Fatalf("decoded root values = fileName %q, assets %d", assets.FileName, len(assets.Assets))
	}
	decodedMaterial := &assets.Assets[1]
	assertNilSlot(t, decodedMaterial.IndexedObjectMetadata, 2)
	assertNilSlot(t, decodedMaterial.IndexedObjectMetadata, 3)
	assertNullElement(t, decodedMaterial.IndexedObjectMetadata, 4, 0)
	assertNullElement(t, decodedMaterial.IndexedObjectMetadata, 5, 0)
	assertNilSlot(t, decodedMaterial.TextureProps[1].IndexedObjectMetadata, 1)

	// Metadata must survive the actual JSON editing boundary, not merely an
	// in-memory decode/encode call.
	jsonData, err := json.Marshal(assets)
	if err != nil {
		t.Fatalf("marshal MaterialAssets JSON: %v", err)
	}
	var edited MaterialAssets
	if err := json.Unmarshal(jsonData, &edited); err != nil {
		t.Fatalf("unmarshal MaterialAssets JSON: %v", err)
	}
	edited.Assets[1].ID = 2 // edit an unrelated known field
	reencoded, err := EncodeMaterialAssets(&edited)
	if err != nil {
		t.Fatalf("EncodeMaterialAssets: %v", err)
	}
	assertMaterialNullWire(t, reencoded)

	// A null element is represented as a zero Go value plus an explicit flag.
	// Populating it without clearing the flag must not be silently discarded.
	edited.Assets[0].Version = 1000
	if _, err := EncodeMaterialAssets(&edited); err == nil || !strings.Contains(err.Error(), "would discard a populated value") {
		t.Fatalf("populated null class element error = %v", err)
	}
}

func TestModelPreservesNilStringElementsAndNullMapValues(t *testing.T) {
	model := []interface{}{
		int64(1001), uint64(1), nil, nil, nil,
		[]interface{}{nil},
		[]interface{}{nil, "root"},
		[]interface{}{nil},
		[]interface{}{nil},
		[]interface{}{
			true,
			map[string]interface{}{"null-group": nil},
		},
		nil,
	}
	decoded, err := DecodeModel(compressIndexedTestValue(t, model))
	if err != nil {
		t.Fatalf("DecodeModel: %v", err)
	}
	assertNilSlot(t, decoded.IndexedObjectMetadata, 2)
	assertNilSlot(t, decoded.IndexedObjectMetadata, 3)
	assertNilSlot(t, decoded.IndexedObjectMetadata, 4)
	assertNilSlot(t, decoded.IndexedObjectMetadata, 10)
	assertNullElement(t, decoded.IndexedObjectMetadata, 5, 0)
	assertNullElement(t, decoded.IndexedObjectMetadata, 6, 0)
	assertNullElement(t, decoded.IndexedObjectMetadata, 7, 0)
	assertNullElement(t, decoded.IndexedObjectMetadata, 8, 0)
	if decoded.SkinThick == nil {
		t.Fatal("skinThick unexpectedly nil")
	}
	assertNullMapKey(t, decoded.SkinThick.IndexedObjectMetadata, 1, "null-group")

	reencoded, err := EncodeModel(decoded)
	if err != nil {
		t.Fatalf("EncodeModel: %v", err)
	}
	root := decodeCompressedIndexedTestArray(t, reencoded)
	for _, slot := range []int32{2, 3, 4, 10} {
		assertRawNil(t, root[slot], "Model scalar slot")
	}
	for _, slot := range []int32{5, 6, 7, 8} {
		elements := decodeIndexedTestArray(t, root[slot])
		assertRawNil(t, elements[0], "Model array element")
	}
	skin := decodeIndexedTestArray(t, root[9])
	var groups map[string]codec.Raw
	if err := decodeOneTestValue(skin[1], &groups); err != nil {
		t.Fatalf("decode re-encoded skin groups: %v", err)
	}
	assertRawNil(t, groups["null-group"], "SkinThickness map value")
}

func TestMenuPreservesNullCommandArgsAndMapValues(t *testing.T) {
	menu := make([]interface{}, 31)
	menu[0] = nil // malformed for Int32, but still faithfully serializable
	menu[3] = nil
	menu[12] = []interface{}{
		[]interface{}{int64(1), []interface{}{nil, "arg"}},
	}
	menu[16] = map[uint64]interface{}{uint64(7): nil}
	wire := compressIndexedTestValue(t, []interface{}{"menuassets", []interface{}{menu}})
	assets, err := DecodeMenuAssets(wire)
	if err != nil {
		t.Fatalf("DecodeMenuAssets: %v", err)
	}
	decoded := &assets.Assets[0]
	assertNilSlot(t, decoded.IndexedObjectMetadata, 0)
	assertNilSlot(t, decoded.IndexedObjectMetadata, 3)
	assertNullElement(t, decoded.Commands[0].IndexedObjectMetadata, 1, 0)
	assertNullMapUint64Key(t, decoded.IndexedObjectMetadata, 16, 7)

	reencoded, err := EncodeMenuAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMenuAssets: %v", err)
	}
	root := decodeCompressedIndexedTestArray(t, reencoded)
	menus := decodeIndexedTestArray(t, root[1])
	slots := decodeIndexedTestArray(t, menus[0])
	assertRawNil(t, slots[0], "Menu version")
	assertRawNil(t, slots[3], "Menu fileName")
	commands := decodeIndexedTestArray(t, slots[12])
	command := decodeIndexedTestArray(t, commands[0])
	args := decodeIndexedTestArray(t, command[1])
	assertRawNil(t, args[0], "Command args[0]")
	var preMul map[uint64]codec.Raw
	if err := decodeOneTestValue(slots[16], &preMul); err != nil {
		t.Fatalf("decode preMulTexDatas: %v", err)
	}
	assertRawNil(t, preMul[7], "preMulTexDatas[7]")
}

func TestTypedIndexedObjectRejectsDuplicateMapKeysItCannotRepresent(t *testing.T) {
	duplicateGroups := codec.Raw{0x82, 0xa1, 'g', 0xc0, 0xa1, 'g', 0xc0}
	skin := []interface{}{true, duplicateGroups}
	model := []interface{}{
		int64(1001), uint64(0), "", "", "",
		[]interface{}{}, []interface{}{}, []interface{}{}, []interface{}{}, skin, int64(0),
	}
	if _, err := DecodeModel(compressIndexedTestValue(t, model)); err == nil || !strings.Contains(err.Error(), "duplicate keys") {
		t.Fatalf("duplicate map-key decode error = %v", err)
	}
}

func assertMaterialNullWire(t *testing.T, data []byte) {
	t.Helper()
	root := decodeCompressedIndexedTestArray(t, data)
	assertRawNil(t, root[0], "MaterialAssets fileName")
	materials := decodeIndexedTestArray(t, root[1])
	assertRawNil(t, materials[0], "MaterialAssets assetArray[0]")
	material := decodeIndexedTestArray(t, materials[1])
	assertRawNil(t, material[2], "Material fileName")
	assertRawNil(t, material[3], "Material shaderName")
	textures := decodeIndexedTestArray(t, material[4])
	assertRawNil(t, textures[0], "Material textureProps[0]")
	texture := decodeIndexedTestArray(t, textures[1])
	assertRawNil(t, texture[1], "TextureProp fileName")
	colors := decodeIndexedTestArray(t, material[5])
	assertRawNil(t, colors[0], "Material colorProps[0]")
}

func assertNilSlot(t *testing.T, metadata *IndexedObjectMetadata, slot int32) {
	t.Helper()
	if metadata == nil || !containsInt(metadata.NilSlots, slot) {
		t.Fatalf("nilSlots = %#v, want slot %d", metadata, slot)
	}
}

func assertNullElement(t *testing.T, metadata *IndexedObjectMetadata, slot int32, element int) {
	t.Helper()
	if metadata == nil || len(metadata.NullElements[slot]) <= element || !metadata.NullElements[slot][element] {
		t.Fatalf("nullElements = %#v, want slot %d element %d", metadata, slot, element)
	}
}

func assertNullMapKey(t *testing.T, metadata *IndexedObjectMetadata, slot int32, key string) {
	t.Helper()
	if metadata == nil {
		t.Fatalf("nil metadata, want null map key %q", key)
	}
	for _, raw := range metadata.NullMapValueKeys[slot] {
		var decoded string
		if err := decodeOneTestValue(raw, &decoded); err == nil && decoded == key {
			return
		}
	}
	t.Fatalf("nullMapValueKeys = % x, want key %q", metadata.NullMapValueKeys[slot], key)
}

func assertNullMapUint64Key(t *testing.T, metadata *IndexedObjectMetadata, slot int32, key uint64) {
	t.Helper()
	if metadata == nil {
		t.Fatalf("nil metadata, want null map key %d", key)
	}
	for _, raw := range metadata.NullMapValueKeys[slot] {
		var decoded uint64
		if err := decodeOneTestValue(raw, &decoded); err == nil && decoded == key {
			return
		}
	}
	t.Fatalf("nullMapValueKeys = % x, want key %d", metadata.NullMapValueKeys[slot], key)
}

func assertRawNil(t *testing.T, raw codec.Raw, label string) {
	t.Helper()
	if len(raw) != 0 && !bytes.Equal(raw, []byte{0xc0}) {
		t.Fatalf("%s = % x, want MessagePack nil", label, raw)
	}
}

func decodeOneTestValue(data []byte, out interface{}) error {
	if len(data) == 0 {
		data = []byte{0xc0}
	}
	return ctDecodeOne(data, out)
}

func ctDecodeOne(data []byte, out interface{}) error {
	// Kept as a tiny wrapper so test assertions stay readable.
	return ct.DecodeMsgpack(data, out)
}

func containsInt(values []int32, target int32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
