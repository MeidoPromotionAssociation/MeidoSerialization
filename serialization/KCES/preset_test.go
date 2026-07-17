package KCES

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

func validKCESPresetCoreForTest(t *testing.T) *KCESPresetCore {
	t.Helper()
	core, err := NewKCESPresetCore()
	if err != nil {
		t.Fatalf("NewKCESPresetCore: %v", err)
	}
	return core
}

func TestKCESPresetGameLayoutRoundTrip(t *testing.T) {
	core := validKCESPresetCoreForTest(t)
	core.Version = 778
	containerFieldCount := 4
	thumbnailFieldCount := 3
	input := &KCESPreset{
		ContainerVersion:     777,
		ContainerFieldCount:  &containerFieldCount,
		ContainerFutureSlots: [][]byte{{0xcc, 0x09}},
		ContainerDirectories: map[string]ct.VirtualDirectoryMetadata{
			"future": {Version: -4},
			"empty":  {Version: 777},
		},
		ContainerVirtualFiles: map[string]ct.VirtualFileMetadata{
			"thumbnail": {FieldCount: &thumbnailFieldCount, FutureSlots: [][]byte{{0xd4, 0x01, 0x7f}}},
		},
		Thumbnail: []byte{0x89, 'P', 'N', 'G', 1, 2, 3},
		MaidData:  core,
		Meta:      &KCESPresetMeta{Version: 779, Data: map[string]string{"presetName": "audit"}},
		ExtraFiles: map[string][]byte{
			"future/data": {7, 8, 9},
		},
	}

	encoded, err := EncodeKCESPreset(input)
	if err != nil {
		t.Fatalf("EncodeKCESPreset: %v", err)
	}
	if !IsKCESPresetData(encoded) {
		t.Fatal("encoded preset does not have the VirtualDirectory signature")
	}
	if len(encoded) < ct.HeaderSize || encoded[7] != ct.SerializeTypeMsgPack {
		t.Fatalf("encoded serialize type = %#x, want %#x", encoded[7], ct.SerializeTypeMsgPack)
	}

	table, err := ct.ReadContentTable(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("independent VirtualDirectory parse: %v", err)
	}
	wantFiles := []string{"future/data", "maiddata", "meta", "thumbnail"}
	if got := table.GetFileNames(); !reflect.DeepEqual(got, wantFiles) {
		t.Fatalf("virtual files = %v, want %v", got, wantFiles)
	}

	decoded, err := DecodeKCESPreset(encoded)
	if err != nil {
		t.Fatalf("DecodeKCESPreset: %v", err)
	}
	if decoded.Format != KCESPresetFormat || decoded.ContainerVersion != 777 {
		t.Fatalf("decoded header = %+v", decoded)
	}
	if decoded.MaidData.Version != 778 || decoded.Meta == nil || decoded.Meta.Version != 779 {
		t.Fatalf("nested versions were not preserved: core=%+v meta=%+v", decoded.MaidData, decoded.Meta)
	}
	if !reflect.DeepEqual(decoded.ContainerFieldCount, input.ContainerFieldCount) || !reflect.DeepEqual(decoded.ContainerFutureSlots, input.ContainerFutureSlots) || !reflect.DeepEqual(decoded.ContainerDirectories, input.ContainerDirectories) || !reflect.DeepEqual(decoded.ContainerVirtualFiles, input.ContainerVirtualFiles) {
		t.Fatalf("container wire metadata changed:\n got  %+v\n want %+v", decoded, input)
	}
	if !bytes.Equal(decoded.Thumbnail, input.Thumbnail) ||
		!bytes.Equal(decoded.MaidData.PropData, input.MaidData.PropData) ||
		!bytes.Equal(decoded.MaidData.ColorData, input.MaidData.ColorData) ||
		!bytes.Equal(decoded.MaidData.BodyData, input.MaidData.BodyData) ||
		!reflect.DeepEqual(decoded.Meta.Data, input.Meta.Data) ||
		!reflect.DeepEqual(decoded.ExtraFiles, input.ExtraFiles) {
		t.Fatalf("preset round-trip mismatch:\n got  %+v\n want %+v", decoded, input)
	}

	// Encoding preserves versions and does not mutate the caller.
	if input.MaidData.Version != 778 || input.Meta.Version != 779 || input.ContainerVersion != 777 {
		t.Fatalf("EncodeKCESPreset mutated input versions: %+v", input)
	}
}

func TestKCESPresetRejectsMissingOrCorruptOuterGameFiles(t *testing.T) {
	if _, err := EncodeKCESPreset(&KCESPreset{}); err == nil || !strings.Contains(err.Error(), "maidData") {
		t.Fatalf("missing maidData error = %v", err)
	}

	table := &ct.ContentTable{
		Version: 1000,
		Raw:     make([]byte, ct.HeaderSize),
		Files:   make(map[string]ct.VirtualFile),
	}
	table.AddFile("thumbnail", nil)
	table.AddFile("maiddata", []byte{0x92, 0x01}) // truncated ordinary MessagePack array
	var malformed bytes.Buffer
	if err := ct.WriteContentTable(&malformed, table); err != nil {
		t.Fatalf("build malformed preset: %v", err)
	}
	if _, err := DecodeKCESPreset(malformed.Bytes()); err == nil || !strings.Contains(err.Error(), "maiddata") {
		t.Fatalf("corrupt maiddata error = %v", err)
	}

	table = &ct.ContentTable{
		Version: 1000,
		Raw:     make([]byte, ct.HeaderSize),
		Files:   make(map[string]ct.VirtualFile),
	}
	validCore, err := encodeCompressedMsgpack(validKCESPresetCoreForTest(t), "test core")
	if err != nil {
		t.Fatal(err)
	}
	table.AddFile("maiddata", validCore)
	var missingThumbnail bytes.Buffer
	if err := ct.WriteContentTable(&missingThumbnail, table); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESPreset(missingThumbnail.Bytes()); err == nil || !strings.Contains(err.Error(), "thumbnail") {
		t.Fatalf("missing thumbnail error = %v", err)
	}
}

func TestKCESPresetPreservesOpaqueInnerBlocksAndMessagePackTrailingData(t *testing.T) {
	tail := []byte{0xde, 0xad, 0xbe, 0xef}
	input := &KCESPreset{
		Thumbnail: []byte{1, 2, 3},
		MaidData: &KCESPresetCore{
			Version:      -7,
			PropData:     []byte{0xff, 0x00, 0x01},
			ColorData:    nil,
			BodyData:     []byte{},
			TrailingData: append([]byte(nil), tail...),
		},
		Meta: &KCESPresetMeta{
			Version:      0,
			Data:         map[string]string{"future": "value"},
			TrailingData: []byte{0xc1, 0x99},
		},
	}

	encoded, err := EncodeKCESPreset(input)
	if err != nil {
		t.Fatalf("EncodeKCESPreset opaque inner blocks: %v", err)
	}
	decoded, err := DecodeKCESPreset(encoded)
	if err != nil {
		t.Fatalf("DecodeKCESPreset opaque inner blocks: %v", err)
	}
	if decoded.MaidData.Version != -7 || decoded.Meta == nil || decoded.Meta.Version != 0 {
		t.Fatalf("explicit versions changed: maid=%d meta=%+v", decoded.MaidData.Version, decoded.Meta)
	}
	if !bytes.Equal(decoded.MaidData.PropData, input.MaidData.PropData) ||
		decoded.MaidData.ColorData != nil ||
		decoded.MaidData.BodyData == nil ||
		!bytes.Equal(decoded.MaidData.TrailingData, input.MaidData.TrailingData) ||
		!bytes.Equal(decoded.Meta.TrailingData, input.Meta.TrailingData) {
		t.Fatalf("opaque preset payload changed:\n got  %+v\n want %+v", decoded, input)
	}

	reencoded, err := EncodeKCESPreset(decoded)
	if err != nil {
		t.Fatalf("re-encode opaque preset: %v", err)
	}
	decodedAgain, err := DecodeKCESPreset(reencoded)
	if err != nil {
		t.Fatalf("decode re-encoded opaque preset: %v", err)
	}
	if !bytes.Equal(decodedAgain.MaidData.TrailingData, tail) || !bytes.Equal(decodedAgain.Meta.TrailingData, input.Meta.TrailingData) {
		t.Fatalf("MessagePack trailing bytes changed after second round trip: %+v", decodedAgain)
	}
}

func TestKCESPresetPreservesIndexedFutureAndNullMetadata(t *testing.T) {
	coreFuture := codec.Raw{0xd4, 0x23, 0x01}
	metaFuture := codec.Raw{0x81, 0xa1, 'x', 0xcc, 0x80}
	table := &ct.ContentTable{
		Version: 1000,
		Raw:     make([]byte, ct.HeaderSize),
		Files:   make(map[string]ct.VirtualFile),
	}
	table.AddFile(kcesPresetThumbnailFile, []byte{1, 2, 3})
	table.AddFile(kcesPresetMaidDataFile, compressIndexedTestValue(t, []interface{}{
		int64(1000), nil, []byte{}, []byte{7}, coreFuture,
	}))
	table.AddFile(kcesPresetMetaFile, compressIndexedTestValue(t, []interface{}{
		int64(1000), map[string]interface{}{"null": nil, "value": "ok"}, metaFuture,
	}))
	var wire bytes.Buffer
	if err := ct.WriteContentTable(&wire, table); err != nil {
		t.Fatalf("build preset wire: %v", err)
	}

	preset, err := DecodeKCESPreset(wire.Bytes())
	if err != nil {
		t.Fatalf("DecodeKCESPreset: %v", err)
	}
	assertIndexedMetadata(t, preset.MaidData.IndexedObjectMetadata, 5, coreFuture)
	assertIndexedMetadata(t, preset.Meta.IndexedObjectMetadata, 3, metaFuture)
	assertNullMapKey(t, preset.Meta.IndexedObjectMetadata, 1, "null")
	if preset.MaidData.PropData != nil || preset.MaidData.ColorData == nil || preset.Meta.Data["null"] != "" {
		t.Fatalf("decoded preset nil/empty values changed: maid=%+v meta=%+v", preset.MaidData, preset.Meta)
	}

	reencoded, err := EncodeKCESPreset(preset)
	if err != nil {
		t.Fatalf("EncodeKCESPreset: %v", err)
	}
	reencodedTable, err := ct.ReadContentTable(bytes.NewReader(reencoded))
	if err != nil {
		t.Fatalf("ReadContentTable: %v", err)
	}
	coreCompressed, err := reencodedTable.GetFileData(kcesPresetMaidDataFile)
	if err != nil {
		t.Fatal(err)
	}
	coreSlots := decodeCompressedIndexedTestArray(t, coreCompressed)
	if len(coreSlots) != 5 || !rawMessagePackEqual(coreSlots[4], coreFuture) {
		t.Fatalf("re-encoded core slots = % x", coreSlots)
	}
	assertRawNil(t, coreSlots[1], "preset propData")
	metaCompressed, err := reencodedTable.GetFileData(kcesPresetMetaFile)
	if err != nil {
		t.Fatal(err)
	}
	metaSlots := decodeCompressedIndexedTestArray(t, metaCompressed)
	if len(metaSlots) != 3 || !rawMessagePackEqual(metaSlots[2], metaFuture) {
		t.Fatalf("re-encoded meta slots = % x", metaSlots)
	}
	var metaMap map[string]codec.Raw
	if err := ct.DecodeMsgpack(metaSlots[1], &metaMap); err != nil {
		t.Fatalf("decode re-encoded meta map: %v", err)
	}
	assertRawNil(t, metaMap["null"], "preset metaData null value")
}

func TestKCESPresetMetaDictionaryAndReservedFiles(t *testing.T) {
	input := &KCESPreset{
		MaidData: validKCESPresetCoreForTest(t),
		Meta:     &KCESPresetMeta{},
	}
	encoded, err := EncodeKCESPreset(input)
	if err != nil {
		t.Fatalf("EncodeKCESPreset(nil metaData): %v", err)
	}
	decoded, err := DecodeKCESPreset(encoded)
	if err != nil {
		t.Fatalf("DecodeKCESPreset: %v", err)
	}
	if decoded.Meta == nil || decoded.Meta.Data != nil {
		t.Fatalf("decoded metaData = %#v, want preserved nil map", decoded.Meta)
	}
	if input.Meta.Data != nil {
		t.Fatal("EncodeKCESPreset mutated caller's nil metaData")
	}

	input.ExtraFiles = map[string][]byte{"maiddata": {1}}
	if _, err := EncodeKCESPreset(input); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved extra file error = %v", err)
	}
}
