package KCES

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
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
	presetName := "audit"
	input := &KCESPreset{
		ContainerVersion: 777,
		ContainerDirectories: map[string]ct.VirtualDirectoryMetadata{
			"future": {Version: -4},
			"empty":  {Version: 777},
		},
		Thumbnail: []byte{0x89, 'P', 'N', 'G', 1, 2, 3},
		MaidData:  core,
		Meta:      &KCESPresetMeta{Version: 779, Data: map[string]*string{"presetName": &presetName}},
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
	if decoded.ContainerVersion != 777 {
		t.Fatalf("decoded header = %+v", decoded)
	}
	if decoded.MaidData.Version != 778 || decoded.Meta == nil || decoded.Meta.Version != 779 {
		t.Fatalf("nested versions were not preserved: core=%+v meta=%+v", decoded.MaidData, decoded.Meta)
	}
	if !reflect.DeepEqual(decoded.ContainerDirectories, input.ContainerDirectories) {
		t.Fatalf("container directory fields changed:\n got  %+v\n want %+v", decoded, input)
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

func TestKCESPresetExtendedContainerFramingRoundTrip(t *testing.T) {
	value, err := NewKCESPreset()
	if err != nil {
		t.Fatal(err)
	}
	value.ContainerFraming = ct.VirtualDirectoryFramingExtended
	encoded, err := EncodeKCESPreset(value)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) < ct.HeaderSize || encoded[7] != ct.SerializeTypeMsgPackExtended {
		t.Fatalf("encoded serialize type = %#x, want %#x", encoded[7], ct.SerializeTypeMsgPackExtended)
	}
	decoded, err := DecodeExpandedKCESPreset(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ContainerFraming != ct.VirtualDirectoryFramingExtended {
		t.Fatalf("decoded container framing = %d, want extended", decoded.ContainerFraming)
	}
	reencoded, err := EncodeExpandedKCESPreset(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(reencoded) < ct.HeaderSize || reencoded[7] != ct.SerializeTypeMsgPackExtended {
		t.Fatalf("re-encoded serialize type = %#x, want %#x", reencoded[7], ct.SerializeTypeMsgPackExtended)
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

	nullCore, err := msgpack.CompressLz4BlockArray([]byte{0xc0})
	if err != nil {
		t.Fatal(err)
	}
	table = &ct.ContentTable{
		Version: 1000,
		Raw:     make([]byte, ct.HeaderSize),
		Files:   make(map[string]ct.VirtualFile),
	}
	if err := table.AddFile(kcesPresetThumbnailFile, nil); err != nil {
		t.Fatal(err)
	}
	if err := table.AddFile(kcesPresetMaidDataFile, nullCore); err != nil {
		t.Fatal(err)
	}
	var nullMaidData bytes.Buffer
	if err := ct.WriteContentTable(&nullMaidData, table); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESPreset(nullMaidData.Bytes()); err == nil || !strings.Contains(err.Error(), "root must not be null") {
		t.Fatalf("null maiddata root error = %v", err)
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

func TestKCESPresetRejectsMalformedKnownBlocksAndMessagePackTrailingData(t *testing.T) {
	malformed := &KCESPreset{
		Thumbnail: []byte{1, 2, 3},
		MaidData: &KCESPresetCore{
			Version:  1000,
			PropData: []byte{0xff, 0x00, 0x01},
		},
	}
	if _, err := EncodeKCESPreset(malformed); err == nil || !strings.Contains(err.Error(), "propData") {
		t.Fatalf("malformed known block error = %v", err)
	}

	core := validKCESPresetCoreForTest(t)
	coreMessagePack, err := msgpack.EncodeIndexedMsgpack(core)
	if err != nil {
		t.Fatal(err)
	}
	coreMessagePack = append(coreMessagePack, 0xde, 0xad)
	coreWithTrailing, err := msgpack.CompressLz4BlockArray(coreMessagePack)
	if err != nil {
		t.Fatal(err)
	}
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize), Files: make(map[string]ct.VirtualFile)}
	if err := table.AddFile(kcesPresetThumbnailFile, []byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := table.AddFile(kcesPresetMaidDataFile, coreWithTrailing); err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	if err := ct.WriteContentTable(&wire, table); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESPreset(wire.Bytes()); err == nil || !strings.Contains(strings.ToLower(err.Error()), "trailing") {
		t.Fatalf("maiddata trailing-data error = %v", err)
	}
}

func TestKCESPresetRejectsFutureSlotsAndUsesTypedNullMapValues(t *testing.T) {
	writePreset := func(maidData, metaData []byte) []byte {
		t.Helper()
		table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize), Files: make(map[string]ct.VirtualFile)}
		if err := table.AddFile(kcesPresetThumbnailFile, []byte{1, 2, 3}); err != nil {
			t.Fatal(err)
		}
		if err := table.AddFile(kcesPresetMaidDataFile, maidData); err != nil {
			t.Fatal(err)
		}
		if metaData != nil {
			if err := table.AddFile(kcesPresetMetaFile, metaData); err != nil {
				t.Fatal(err)
			}
		}
		var wire bytes.Buffer
		if err := ct.WriteContentTable(&wire, table); err != nil {
			t.Fatal(err)
		}
		return wire.Bytes()
	}

	core := validKCESPresetCoreForTest(t)
	validCore, err := encodeCompressedMsgpack(core, "test core")
	if err != nil {
		t.Fatal(err)
	}
	coreWithFutureSlot := compressIndexedTestValue(t, []interface{}{
		int64(core.Version), core.PropData, core.ColorData, core.BodyData, int64(1),
	})
	if _, err := DecodeKCESPreset(writePreset(coreWithFutureSlot, nil)); err == nil {
		t.Fatal("preset maiddata accepted a high MessagePack key")
	}

	metaWithFutureSlot := compressIndexedTestValue(t, []interface{}{
		int64(1000), map[string]interface{}{"value": "ok"}, int64(1),
	})
	if _, err := DecodeKCESPreset(writePreset(validCore, metaWithFutureSlot)); err == nil {
		t.Fatal("preset meta accepted a high MessagePack key")
	}

	nullMeta, err := encodeCompressedMsgpack(nil, "test null meta")
	if err != nil {
		t.Fatal(err)
	}
	nullMetaPreset, err := DecodeKCESPreset(writePreset(validCore, nullMeta))
	if err != nil {
		t.Fatalf("DecodeKCESPreset null meta: %v", err)
	}
	if nullMetaPreset.Meta != nil {
		t.Fatalf("decoded null meta = %+v, want nil", nullMetaPreset.Meta)
	}
	canonical, err := EncodeKCESPreset(nullMetaPreset)
	if err != nil {
		t.Fatalf("EncodeKCESPreset canonical null meta: %v", err)
	}
	canonicalTable, err := ct.ReadContentTable(bytes.NewReader(canonical))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := canonicalTable.Files[kcesPresetMetaFile]; ok {
		t.Fatal("canonical preset retained a null meta virtual file")
	}

	metaWithNull := compressIndexedTestValue(t, []interface{}{
		int64(1000), map[string]interface{}{"null": nil, "value": "ok"},
	})
	preset, err := DecodeKCESPreset(writePreset(validCore, metaWithNull))
	if err != nil {
		t.Fatalf("DecodeKCESPreset typed null map: %v", err)
	}
	if preset.Meta == nil || preset.Meta.Data["null"] != nil || preset.Meta.Data["value"] == nil || *preset.Meta.Data["value"] != "ok" {
		t.Fatalf("typed meta nullability changed: %+v", preset.Meta)
	}
	reencoded, err := EncodeKCESPreset(preset)
	if err != nil {
		t.Fatalf("EncodeKCESPreset typed null map: %v", err)
	}
	redecoded, err := DecodeKCESPreset(reencoded)
	if err != nil || redecoded.Meta.Data["null"] != nil {
		t.Fatalf("typed meta nullability round trip: meta=%+v err=%v", redecoded.Meta, err)
	}
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
