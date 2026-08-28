package aba

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio"
)

func TestInlineMeshStreamData(t *testing.T) {
	tt := streamInlineTypeTree(ClassIDMesh, []streamInlineNode{
		{level: 0, typeName: "Mesh", name: "Base"},
		{level: 1, typeName: "VertexData", name: "m_VertexData"},
		{level: 2, metaFlags: 0x4000, typeName: "TypelessData", name: "m_DataSize"},
		{level: 1, typeName: "StreamingInfo", name: "m_StreamData"},
		{level: 2, typeName: "UInt64", name: "offset"},
		{level: 2, typeName: "unsigned int", name: "size"},
		{level: 2, typeName: "string", name: "path"},
	})
	var source bytes.Buffer
	w := binaryio.NewEndianWriter(&source, binary.LittleEndian)
	if err := w.WriteInt32(0); err != nil {
		t.Fatal(err)
	}
	if err := w.Align(4); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteUInt64(11); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteUInt32(4); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteAlignedString("mesh.resS"); err != nil {
		t.Fatal(err)
	}
	info, af := streamInlineAssetsFile(ClassIDMesh, tt, source.Bytes())

	encoded, changed, err := af.InlineMeshStreamData(&info, func(name string, offset int64, size int64) ([]byte, error) {
		if name != "mesh.resS" || offset != 11 || size != 4 {
			t.Fatalf("resolver request = %q[%d:%d]", name, offset, offset+size)
		}
		return []byte("MESH"), nil
	})
	if err != nil {
		t.Fatalf("InlineMeshStreamData: %v", err)
	}
	if !changed {
		t.Fatal("InlineMeshStreamData reported no change")
	}
	reparsed := *af
	reparsed.Data = encoded
	info.ByteSize = uint32(len(encoded))
	root, err := reparsed.ReadAssetValue(&info)
	if err != nil {
		t.Fatalf("ReadAssetValue: %v", err)
	}
	vertexData, ok := root.FieldPath("m_VertexData", "m_DataSize").Bytes()
	if !ok || !bytes.Equal(vertexData, []byte("MESH")) {
		t.Fatalf("vertex data = %q, ok=%v", vertexData, ok)
	}
	streamInfo, err := readStreamingInfo(root.Field("m_StreamData"))
	if err != nil || streamInfo != (StreamingInfo{}) {
		t.Fatalf("m_StreamData = %+v, %v", streamInfo, err)
	}
}

func TestInlineAudioClipStreamData(t *testing.T) {
	tt := streamInlineTypeTree(ClassIDAudioClip, []streamInlineNode{
		{level: 0, typeName: "AudioClip", name: "Base"},
		{level: 1, typeName: "string", name: "m_Name"},
		{level: 1, typeName: "StreamedResource", name: "m_Resource"},
		{level: 2, typeName: "string", name: "m_Source"},
		{level: 2, typeName: "UInt64", name: "m_Offset"},
		{level: 2, typeName: "UInt64", name: "m_Size"},
		{level: 1, typeName: "int", name: "m_CompressionFormat"},
	})
	var source bytes.Buffer
	w := binaryio.NewEndianWriter(&source, binary.LittleEndian)
	if err := w.WriteAlignedString("voice"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteAlignedString("voice.resource"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteUInt64(19); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteUInt64(5); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteInt32(1); err != nil {
		t.Fatal(err)
	}
	info, af := streamInlineAssetsFile(ClassIDAudioClip, tt, source.Bytes())

	encoded, changed, err := af.InlineAudioClipStreamData(&info, func(name string, offset int64, size int64) ([]byte, error) {
		if name != "voice.resource" || offset != 19 || size != 5 {
			t.Fatalf("resolver request = %q[%d:%d]", name, offset, offset+size)
		}
		return []byte("AUDIO"), nil
	})
	if err != nil {
		t.Fatalf("InlineAudioClipStreamData: %v", err)
	}
	if !changed {
		t.Fatal("InlineAudioClipStreamData reported no change")
	}
	reparsed := *af
	reparsed.Data = encoded
	info.ByteSize = uint32(len(encoded))
	root, consumed, objectSize, err := reparsed.readAssetValuePrefix(&info)
	if err != nil {
		t.Fatalf("readAssetValuePrefix: %v", err)
	}
	resourceInfo, err := readStreamingInfo(root.Field("m_Resource"))
	if err != nil {
		t.Fatal(err)
	}
	if resourceInfo.Offset != 0 || resourceInfo.Path != "" || resourceInfo.Size != 5 {
		t.Fatalf("m_Resource = %+v", resourceInfo)
	}
	if !bytes.Equal(encoded[consumed:objectSize], []byte("AUDIO")) {
		t.Fatalf("inline audio = %q", encoded[consumed:objectSize])
	}

	stable, changedAgain, err := reparsed.InlineAudioClipStreamData(&info, nil)
	if err != nil {
		t.Fatalf("second InlineAudioClipStreamData: %v", err)
	}
	if changedAgain || !bytes.Equal(stable, encoded) {
		t.Fatalf("second inline changed=%v bytesEqual=%v", changedAgain, bytes.Equal(stable, encoded))
	}
}

func TestInlineCubemapStreamData(t *testing.T) {
	tt := streamInlineTypeTree(ClassIDCubemap, []streamInlineNode{
		{level: 0, typeName: "Cubemap", name: "Base"},
		{level: 1, typeName: "int", name: "m_CompleteImageSize"},
		{level: 1, metaFlags: 0x4000, typeName: "TypelessData", name: "image data"},
		{level: 1, typeName: "StreamingInfo", name: "m_StreamData"},
		{level: 2, typeName: "UInt64", name: "offset"},
		{level: 2, typeName: "unsigned int", name: "size"},
		{level: 2, typeName: "string", name: "path"},
	})
	var source bytes.Buffer
	w := binaryio.NewEndianWriter(&source, binary.LittleEndian)
	if err := w.WriteInt32(4); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteInt32(0); err != nil {
		t.Fatal(err)
	}
	if err := w.Align(4); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteUInt64(31); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteUInt32(4); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteAlignedString("cube.resS"); err != nil {
		t.Fatal(err)
	}
	info, af := streamInlineAssetsFile(ClassIDCubemap, tt, source.Bytes())

	encoded, changed, err := af.InlineCubemapStreamData(&info, func(name string, offset int64, size int64) ([]byte, error) {
		if name != "cube.resS" || offset != 31 || size != 4 {
			t.Fatalf("resolver request = %q[%d:%d]", name, offset, offset+size)
		}
		return []byte("CUBE"), nil
	})
	if err != nil {
		t.Fatalf("InlineCubemapStreamData: %v", err)
	}
	if !changed {
		t.Fatal("InlineCubemapStreamData reported no change")
	}
	reparsed := *af
	reparsed.Data = encoded
	info.ByteSize = uint32(len(encoded))
	root, err := reparsed.ReadAssetValue(&info)
	if err != nil {
		t.Fatalf("ReadAssetValue: %v", err)
	}
	imageData, ok := root.Field("image data").Bytes()
	if !ok || !bytes.Equal(imageData, []byte("CUBE")) {
		t.Fatalf("image data = %q, ok=%v", imageData, ok)
	}
	completeSize, ok := root.Field("m_CompleteImageSize").Int64()
	if !ok || completeSize != 4 {
		t.Fatalf("m_CompleteImageSize = %d, ok=%v", completeSize, ok)
	}
	streamInfo, err := readStreamingInfo(root.Field("m_StreamData"))
	if err != nil || streamInfo != (StreamingInfo{}) {
		t.Fatalf("m_StreamData = %+v, %v", streamInfo, err)
	}
}

type streamInlineNode struct {
	level     byte
	metaFlags uint32
	typeName  string
	name      string
}

func streamInlineTypeTree(classID int32, nodes []streamInlineNode) TypeTreeType {
	var stringBuffer []byte
	stringOffset := func(value string) uint32 {
		offset := uint32(len(stringBuffer))
		stringBuffer = append(stringBuffer, value...)
		stringBuffer = append(stringBuffer, 0)
		return offset
	}
	typeTree := TypeTreeType{TypeId: classID}
	for _, source := range nodes {
		typeTree.Nodes = append(typeTree.Nodes, TypeTreeNode{
			Level:      source.level,
			TypeStrOff: stringOffset(source.typeName),
			NameStrOff: stringOffset(source.name),
			MetaFlags:  source.metaFlags,
		})
	}
	typeTree.StringBuffer = stringBuffer
	return typeTree
}

func streamInlineAssetsFile(classID int32, typeTree TypeTreeType, data []byte) (AssetInfo, *AssetsFile) {
	info := AssetInfo{PathId: 7, ByteSize: uint32(len(data)), TypeIdOrIndex: 0, TypeId: classID}
	af := &AssetsFile{
		Header: AssetsFileHeader{Version: 22},
		Metadata: AssetsMetadata{
			TypeTreeEnabled: true,
			TypeTreeTypes:   []TypeTreeType{typeTree},
		},
		Data: data,
	}
	return info, af
}
