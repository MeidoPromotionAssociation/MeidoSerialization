package aba

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

func TestGetTexture2DData_Sample(t *testing.T) {
	abaFile, f := openAbaSample(t, "parts_personal002.aba")
	defer f.Close()

	for i, dir := range abaFile.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		fileData, err := abaFile.GetFileData(int64(i))
		if err != nil {
			t.Fatalf("GetFileData: %v", err)
		}
		af, err := ReadAssetsFile(fileData)
		if err != nil {
			t.Fatalf("ReadAssetsFile: %v", err)
		}
		for _, info := range af.Metadata.AssetInfos {
			if info.TypeId != ClassIDTexture2D {
				continue
			}
			tex, err := af.GetTexture2DDataRange(&info, abaFile.GetFileDataRangeByName)
			if err != nil {
				if root, rootErr := af.ReadAssetValue(&info); rootErr == nil {
					for _, child := range root.Children {
						if child == nil {
							continue
						}
						t.Logf("field %s %s children=%d value=%T", child.TypeName, child.Name, len(child.Children), child.Value)
					}
				}
				t.Fatalf("GetTexture2DData pathId=%d: %v", info.PathId, err)
			}
			if tex.Name == "" || tex.Width <= 0 || tex.Height <= 0 || len(tex.ImageData) == 0 {
				t.Fatalf("bad texture data: name=%q size=%dx%d data=%d", tex.Name, tex.Width, tex.Height, len(tex.ImageData))
			}
			root, err := af.ReadAssetValue(&info)
			if err != nil {
				t.Fatalf("ReadAssetValue: %v", err)
			}
			imageData := root.Field("image data")
			if imageData == nil {
				imageData = root.Field("m_ImageData")
			}
			if imageData != nil {
				raw, ok := imageData.Bytes()
				if !ok {
					t.Fatalf("image data field did not expose bytes")
				}
				if len(imageData.Children) != 0 {
					t.Fatalf("byte array should not allocate per-byte children: bytes=%d children=%d", len(raw), len(imageData.Children))
				}
			}
			t.Logf("%s: %dx%d format=%d data=%d", tex.Name, tex.Width, tex.Height, tex.TextureFormat, len(tex.ImageData))
			return
		}
	}
	t.Fatal("no Texture2D found in sample")
}

func TestReadStreamingInfoRejectsUnrepresentableRanges(t *testing.T) {
	field := func(name string, value interface{}) *TypeTreeValue {
		return &TypeTreeValue{Name: name, Value: value}
	}
	stream := &TypeTreeValue{Children: []*TypeTreeValue{
		field("offset", uint64(math.MaxInt64)+1),
		field("size", int64(1)),
		field("path", "x.resS"),
	}}
	if _, err := readStreamingInfo(stream); err == nil || !strings.Contains(err.Error(), "offset") {
		t.Fatalf("oversized offset error = %v", err)
	}
	stream.Children[0].Value = uint64(1)
	stream.Children[1].Value = int64(-1)
	if _, err := readStreamingInfo(stream); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("negative size error = %v", err)
	}
}

func TestInlineTexture2DStreamData(t *testing.T) {
	var stringBuffer []byte
	stringOffset := func(value string) uint32 {
		offset := uint32(len(stringBuffer))
		stringBuffer = append(stringBuffer, value...)
		stringBuffer = append(stringBuffer, 0)
		return offset
	}
	node := func(level byte, metaFlags uint32, typeName, name string) TypeTreeNode {
		return TypeTreeNode{
			Level:      level,
			TypeStrOff: stringOffset(typeName),
			NameStrOff: stringOffset(name),
			MetaFlags:  metaFlags,
		}
	}
	tt := TypeTreeType{
		TypeId: ClassIDTexture2D,
		Nodes: []TypeTreeNode{
			node(0, 0, "Texture2D", "Base"),
			node(1, 0, "string", "m_Name"),
			node(1, 0, "int", "m_CompleteImageSize"),
			node(1, 0x4000, "TypelessData", "image data"),
			node(1, 0, "StreamingInfo", "m_StreamData"),
			node(2, 0, "UInt64", "offset"),
			node(2, 0, "unsigned int", "size"),
			node(2, 0, "string", "path"),
		},
		StringBuffer: stringBuffer,
	}

	var source bytes.Buffer
	w := binaryio.NewEndianWriter(&source, binary.LittleEndian)
	if err := w.WriteAlignedString("streamed"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteInt32(4); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteInt32(0); err != nil {
		t.Fatal(err)
	}
	if err := w.Align(4); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteUInt64(3); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteUInt32(4); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteAlignedString("x.resS"); err != nil {
		t.Fatal(err)
	}

	info := AssetInfo{
		PathId:        7,
		ByteSize:      uint32(source.Len()),
		TypeIdOrIndex: 0,
		TypeId:        ClassIDTexture2D,
	}
	af := &AssetsFile{
		Header: AssetsFileHeader{Version: 22},
		Metadata: AssetsMetadata{
			TypeTreeEnabled: true,
			TypeTreeTypes:   []TypeTreeType{tt},
		},
		Data: source.Bytes(),
	}

	encoded, changed, err := af.InlineTexture2DStreamData(&info, func(name string, offset int64, size int64) ([]byte, error) {
		if name != "x.resS" || offset != 3 || size != 4 {
			t.Fatalf("resolver request = %q[%d:%d]", name, offset, offset+size)
		}
		return []byte("ABCD"), nil
	})
	if err != nil {
		t.Fatalf("InlineTexture2DStreamData: %v", err)
	}
	if !changed {
		t.Fatal("InlineTexture2DStreamData reported no change")
	}

	reparsed := *af
	reparsed.Data = encoded
	info.ByteSize = uint32(len(encoded))
	root, err := reparsed.ReadAssetValue(&info)
	if err != nil {
		t.Fatalf("ReadAssetValue after inline: %v", err)
	}
	imageData, ok := root.Field("image data").Bytes()
	if !ok || !bytes.Equal(imageData, []byte("ABCD")) {
		t.Fatalf("inline image data = %q, ok=%v", imageData, ok)
	}
	completeSize, ok := root.Field("m_CompleteImageSize").Int64()
	if !ok || completeSize != 4 {
		t.Fatalf("m_CompleteImageSize = %d, ok=%v", completeSize, ok)
	}
	streamInfo, err := readStreamingInfo(root.Field("m_StreamData"))
	if err != nil {
		t.Fatal(err)
	}
	if streamInfo.Offset != 0 || streamInfo.Size != 0 || streamInfo.Path != "" {
		t.Fatalf("m_StreamData was not cleared: %+v", streamInfo)
	}
}
