package aba

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

func TestEncodeTexture2DData_MatchesRealKCESTypeTrees(t *testing.T) {
	pixels := []byte{
		0xff, 0x00, 0x00, 0xff,
		0x00, 0xff, 0x00, 0x80,
		0x00, 0x00, 0xff, 0x40,
		0xff, 0xff, 0xff, 0x00,
	}
	tests := []struct {
		name                 string
		sample               string
		unityVersion         string
		wantMipmapLimitGroup bool
	}{
		{name: "Unity_2020_2", sample: "cm3d2_megane002.aba", unityVersion: "2020.2.4f1"},
		{name: "Unity_2021_3", sample: "cm3d2_megane002.aba", unityVersion: "2021.3.37f1"},
		{name: "Unity_2022_3", sample: "parts_bcc4_gp003_2.aba", unityVersion: "2022.3.62f2", wantMipmapLimitGroup: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tt := sampleTypeTree(t, tc.sample, ClassIDTexture2D)
			data, err := encodeTexture2DData(tc.unityVersion, "layout_test.tex", 2, 2, pixels)
			if err != nil {
				t.Fatalf("encodeTexture2DData: %v", err)
			}
			root := decodeObjectWithTypeTree(t, &tt, data)

			assertTypeTreeString(t, root, "m_Name", "layout_test.tex")
			assertTypeTreeInt(t, root, "m_ForcedFallbackFormat", int64(TextureFormatRGBA32))
			assertTypeTreeInt(t, root, "m_Width", 2)
			assertTypeTreeInt(t, root, "m_Height", 2)
			assertTypeTreeInt(t, root, "m_CompleteImageSize", int64(len(pixels)))
			assertTypeTreeInt(t, root, "m_TextureFormat", int64(TextureFormatRGBA32))
			assertTypeTreeInt(t, root, "m_MipCount", 1)
			assertTypeTreeBool(t, root, "m_IsReadable", true)
			assertTypeTreeBool(t, root, "m_IsPreProcessed", false)
			assertTypeTreeBool(t, root, "m_StreamingMipmaps", false)

			if tc.wantMipmapLimitGroup {
				assertTypeTreeBool(t, root, "m_IgnoreMipmapLimit", false)
				assertTypeTreeString(t, root, "m_MipmapLimitGroupName", "")
				if root.Field("m_IgnoreMasterTextureLimit") != nil {
					t.Fatal("2022.3 type tree unexpectedly contains m_IgnoreMasterTextureLimit")
				}
			} else {
				assertTypeTreeBool(t, root, "m_IgnoreMasterTextureLimit", false)
				if root.Field("m_MipmapLimitGroupName") != nil {
					t.Fatal("pre-2022 type tree unexpectedly contains m_MipmapLimitGroupName")
				}
			}

			imageData, ok := root.Field("image data").Bytes()
			if !ok || !bytes.Equal(imageData, pixels) {
				t.Fatalf("image data got %x ok=%v, want %x", imageData, ok, pixels)
			}
			stream := root.Field("m_StreamData")
			if stream == nil {
				t.Fatal("m_StreamData missing")
			}
			assertTypeTreeInt(t, stream, "offset", 0)
			assertTypeTreeInt(t, stream, "size", 0)
			assertTypeTreeString(t, stream, "path", "")
		})
	}
}

func TestSerializedFileWriterEmitsStandardEmptyMetadataTailTables(t *testing.T) {
	writer := NewSerializedFileWriter("2021.3.3f1")
	writer.AddTextAsset("tail.menuassets", []byte("payload"))
	var out bytes.Buffer
	if err := writer.Write(&out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data := out.Bytes()
	const version22HeaderSize = 48
	if len(data) < version22HeaderSize {
		t.Fatalf("SerializedFile too short: %d", len(data))
	}
	if !bytes.Equal(data[0:8], make([]byte, 8)) || !bytes.Equal(data[12:16], make([]byte, 4)) {
		t.Fatalf("v22 legacy header fields are not zero: % x / % x", data[0:8], data[12:16])
	}
	metadataSize := int(binary.BigEndian.Uint32(data[20:24]))
	if metadataSize < 13 || version22HeaderSize+metadataSize > len(data) {
		t.Fatalf("invalid metadata size %d for %d-byte file", metadataSize, len(data))
	}
	tail := data[version22HeaderSize+metadataSize-13 : version22HeaderSize+metadataSize]
	if !bytes.Equal(tail, make([]byte, 13)) {
		t.Fatalf("metadata tail = % x; want ScriptTypes=0, ExternalFiles=0, RefTypes=0, empty UserInformation", tail)
	}
}

func TestSerializedFileWriterOmitsTypeDependenciesWhenTypeTreeDisabled(t *testing.T) {
	writer := NewSerializedFileWriter("2021.3.3f1")
	writer.AddTextAsset("layout.menuassets", []byte("payload"))
	var out bytes.Buffer
	if err := writer.Write(&out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data := out.Bytes()
	metadataSize := int(binary.BigEndian.Uint32(data[20:24]))
	r := binaryio.NewEndianReader(data[48:48+metadataSize], binary.LittleEndian)
	if _, err := r.ReadNullString(); err != nil {
		t.Fatalf("read Unity version: %v", err)
	}
	if _, err := r.ReadUInt32(); err != nil {
		t.Fatalf("read target platform: %v", err)
	}
	enabled, err := r.ReadByte()
	if err != nil || enabled != 0 {
		t.Fatalf("TypeTreeEnabled = %d, %v; want 0", enabled, err)
	}
	typeCount, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read type count: %v", err)
	}
	for i := 0; i < int(typeCount); i++ {
		classID, err := r.ReadInt32()
		if err != nil {
			t.Fatalf("read type[%d] class ID: %v", i, err)
		}
		if _, err := r.ReadByte(); err != nil {
			t.Fatalf("read type[%d] stripped flag: %v", i, err)
		}
		if _, err := r.ReadInt16(); err != nil {
			t.Fatalf("read type[%d] script index: %v", i, err)
		}
		if classID == ClassIDMonoBehaviour {
			var scriptHash [16]byte
			if err := r.ReadFull(scriptHash[:]); err != nil {
				t.Fatalf("read type[%d] script hash: %v", i, err)
			}
		}
		var typeHash [16]byte
		if err := r.ReadFull(typeHash[:]); err != nil {
			t.Fatalf("read type[%d] hash: %v", i, err)
		}
	}
	assetCount, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read asset count immediately after types: %v", err)
	}
	if assetCount != 2 {
		t.Fatalf("asset count after types = %d, want 2; a stray dependency count may have been written", assetCount)
	}
}

func TestSerializedFileWriterAlignsEveryObjectToEightBytes(t *testing.T) {
	writer := NewSerializedFileWriter("2021.3.3f1")
	writer.AddRawObject(ClassIDMonoBehaviour, "a", []byte{1})
	writer.AddRawObject(ClassIDMonoBehaviour, "b", []byte{2, 3, 4, 5, 6})

	var out bytes.Buffer
	if err := writer.Write(&out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	af, err := ReadAssetsFile(out.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	for i, info := range af.Metadata.AssetInfos {
		if info.ByteOffset%8 != 0 {
			t.Fatalf("asset info[%d] byte offset = %d, want 8-byte alignment", i, info.ByteOffset)
		}
	}
	if len(out.Bytes())%8 != 0 {
		t.Fatalf("serialized file size = %d, want final 8-byte alignment", len(out.Bytes()))
	}
}

func TestEncodeAssetBundleObject_MatchesRealKCESTypeTree(t *testing.T) {
	tt := sampleTypeTree(t, "cm3d2_megane002.aba", ClassIDAssetBundle)
	w := NewSerializedFileWriter("2021.3.37f1")
	pathID := w.AddTextAssetWithLoadName("inside.menuassets", "assets/test/inside.menuassets.bytes", []byte("payload"))
	data, err := w.encodeAssetBundleObject()
	if err != nil {
		t.Fatalf("encodeAssetBundleObject: %v", err)
	}
	root := decodeObjectWithTypeTree(t, &tt, data)

	assertTypeTreeString(t, root, "m_Name", "CAB-generated")
	assertTypeTreeInt(t, root, "m_RuntimeCompatibility", 0)
	assertTypeTreeInt(t, root, "m_ExplicitDataLayout", 0)
	assertTypeTreeInt(t, root, "m_PathFlags", 0)
	mainAsset := root.Field("m_MainAsset")
	if mainAsset == nil {
		t.Fatal("m_MainAsset missing")
	}
	assertTypeTreeInt(t, mainAsset, "preloadIndex", 0)
	assertTypeTreeInt(t, mainAsset, "preloadSize", 0)
	asset := mainAsset.Field("asset")
	if asset == nil {
		t.Fatal("m_MainAsset.asset missing")
	}
	assertTypeTreeInt(t, asset, "m_FileID", 0)
	assertTypeTreeInt(t, asset, "m_PathID", 0)

	containerEntries := root.Field("m_Container")
	if containerEntries == nil {
		t.Fatal("m_Container missing")
	}
	if got := findTypeTreeIntRecursive(containerEntries, "m_PathID"); got != pathID {
		t.Fatalf("m_Container PathID got %d, want %d", got, pathID)
	}
	if root.Field("m_SceneHashes") == nil {
		t.Fatal("m_SceneHashes missing")
	}
}

func TestSerializedFileWriter_RejectsInvalidTextureAndUnknownUnityLine(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		width      int
		height     int
		imageData  []byte
		wantEncode bool
	}{
		{name: "zero_width", version: "2021.3.37f1", height: 1, imageData: []byte{0, 0, 0, 0}},
		{name: "negative_height", version: "2021.3.37f1", width: 1, height: -1, imageData: []byte{0, 0, 0, 0}},
		{name: "wrong_rgba_size", version: "2021.3.37f1", width: 1, height: 1, imageData: []byte{0, 0, 0}},
		{name: "unknown_unity_line", version: "2023.1.0f1", width: 1, height: 1, imageData: []byte{0, 0, 0, 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := encodeTexture2DData(tc.version, "invalid.tex", tc.width, tc.height, tc.imageData); err == nil {
				t.Fatal("encodeTexture2DData unexpectedly succeeded")
			}
		})
	}

	w := NewSerializedFileWriter("2023.1.0f1")
	w.AddTextAsset("test.menuassets", []byte("payload"))
	if err := w.Write(&bytes.Buffer{}); err == nil {
		t.Fatal("Write unexpectedly accepted unsupported Unity line")
	}
}

func TestSerializedFileWriter_RepeatedWriteIsDeterministic(t *testing.T) {
	w := NewSerializedFileWriter("2021.3.37f1")
	w.AddTextAsset("test.menuassets", []byte("payload"))
	var first, second bytes.Buffer
	if err := w.Write(&first); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if err := w.Write(&second); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("repeated Write produced different SerializedFile bytes")
	}
}

func sampleTypeTree(t *testing.T, sample string, classID int32) TypeTreeType {
	t.Helper()
	abaFile, f := openAbaSample(t, sample)
	defer f.Close()
	for i, dir := range abaFile.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := abaFile.GetFileData(i)
		if err != nil {
			t.Fatalf("GetFileData(%q): %v", dir.Name, err)
		}
		af, err := ReadAssetsFile(data)
		if err != nil {
			t.Fatalf("ReadAssetsFile(%q): %v", dir.Name, err)
		}
		for typeIndex := range af.Metadata.TypeTreeTypes {
			tt := af.Metadata.TypeTreeTypes[typeIndex]
			if tt.TypeId == classID && len(tt.Nodes) > 0 {
				return tt
			}
		}
	}
	t.Fatalf("class ID %d type tree not found in %s", classID, sample)
	return TypeTreeType{}
}

func decodeObjectWithTypeTree(t *testing.T, tt *TypeTreeType, data []byte) *TypeTreeValue {
	t.Helper()
	r := binaryio.NewEndianReader(data, binary.LittleEndian)
	root, next, err := readTypeTreeValue(tt, r, 0)
	if err != nil {
		t.Fatalf("decode generated object with real KCES type tree: %v", err)
	}
	if next != len(tt.Nodes) {
		t.Fatalf("type tree stopped at node %d/%d", next, len(tt.Nodes))
	}
	if r.Remaining() != 0 {
		t.Fatalf("generated object has %d unconsumed bytes", r.Remaining())
	}
	return root
}

func assertTypeTreeString(t *testing.T, parent *TypeTreeValue, name, want string) {
	t.Helper()
	field := parent.Field(name)
	got, ok := field.String()
	if !ok || got != want {
		t.Fatalf("%s got %q ok=%v, want %q", name, got, ok, want)
	}
}

func assertTypeTreeInt(t *testing.T, parent *TypeTreeValue, name string, want int64) {
	t.Helper()
	field := parent.Field(name)
	got, ok := field.Int64()
	if !ok || got != want {
		t.Fatalf("%s got %d ok=%v, want %d", name, got, ok, want)
	}
}

func assertTypeTreeBool(t *testing.T, parent *TypeTreeValue, name string, want bool) {
	t.Helper()
	field := parent.Field(name)
	if field == nil {
		t.Fatalf("%s missing", name)
	}
	got, ok := field.Value.(bool)
	if !ok || got != want {
		t.Fatalf("%s got %v ok=%v, want %v", name, got, ok, want)
	}
}

func findTypeTreeIntRecursive(value *TypeTreeValue, name string) int64 {
	if value == nil {
		return 0
	}
	if value.Name == name {
		if result, ok := value.Int64(); ok {
			return result
		}
	}
	for _, child := range value.Children {
		if result := findTypeTreeIntRecursive(child, name); result != 0 {
			return result
		}
	}
	return 0
}
