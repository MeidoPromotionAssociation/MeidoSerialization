package aba

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestNativeUnityObjectRoundTripAndValueEditing(t *testing.T) {
	tree := nativeObjectTestTypeTree(ClassIDMaterial)
	object := &NativeUnityObject{ClassID: ClassIDMaterial, TypeTree: tree}
	value := &TypeTreeValue{
		TypeName: "Material",
		Name:     "Base",
		Children: []*TypeTreeValue{
			{TypeName: "string", Name: "m_Name", Value: "sample"},
			{TypeName: "int", Name: "m_Custom", Value: int64(17)},
		},
	}
	data, err := object.EncodeValue(value)
	if err != nil {
		t.Fatal(err)
	}
	object.Data = data

	encoded, err := EncodeNativeUnityObject(object)
	if err != nil {
		t.Fatal(err)
	}
	wantSize, err := NativeUnityObjectSize(object)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(encoded)) != wantSize {
		t.Fatalf("encoded size = %d, want %d", len(encoded), wantSize)
	}

	header, err := ReadNativeUnityObjectHeader(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	if header.ClassID != ClassIDMaterial || header.DataOffset+int64(header.DataSize) != int64(len(encoded)) {
		t.Fatalf("unexpected header: %+v", header)
	}
	restored, err := ReadNativeUnityObject(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ClassID != object.ClassID || restored.BigEndian != object.BigEndian || !reflect.DeepEqual(restored.TypeTree, object.TypeTree) || !bytes.Equal(restored.Data, object.Data) {
		t.Fatalf("round trip changed object: got %+v, want %+v", restored, object)
	}
	decoded, err := restored.DecodeValue()
	if err != nil {
		t.Fatal(err)
	}
	name, ok := decoded.Field("m_Name").String()
	if !ok || name != "sample" {
		t.Fatalf("decoded m_Name = %q, %v", name, ok)
	}
	decoded.Field("m_Custom").Value = int64(23)
	modified, err := restored.EncodeValue(decoded)
	if err != nil {
		t.Fatal(err)
	}
	restored.Data = modified
	decodedAgain, err := restored.DecodeValue()
	if err != nil {
		t.Fatal(err)
	}
	if got := decodedAgain.Field("m_Custom").Value; got != int64(23) {
		t.Fatalf("edited m_Custom = %#v, want 23", got)
	}
}

func TestNativeUnityObjectRejectsInvalidEnvelope(t *testing.T) {
	object := &NativeUnityObject{ClassID: ClassIDMaterial, TypeTree: nativeObjectTestTypeTree(ClassIDMaterial), Data: []byte{0, 0, 0, 0, 0, 0, 0, 0}}
	encoded, err := EncodeNativeUnityObject(object)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "truncated", data: encoded[:len(encoded)-1], want: "declares"},
		{name: "bad magic", data: append([]byte("BADMAGIC"), encoded[8:]...), want: "magic"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadNativeUnityObject(test.data)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestSerializedFileWriterEmbedsNativeUnityObjectTypeTree(t *testing.T) {
	tree := nativeObjectTestTypeTree(ClassIDMaterial)
	object := &NativeUnityObject{ClassID: ClassIDMaterial, TypeTree: tree}
	value := &TypeTreeValue{
		TypeName: "Material",
		Name:     "Base",
		Children: []*TypeTreeValue{
			{TypeName: "string", Name: "m_Name", Value: "embedded"},
			{TypeName: "int", Name: "m_Custom", Value: int64(41)},
		},
	}
	var err error
	object.Data, err = object.EncodeValue(value)
	if err != nil {
		t.Fatal(err)
	}

	writer := NewSerializedFileWriter("2022.3.35f1")
	pathID := writer.AddNativeUnityObjectWithLoadNameAndPathID(object, "embedded", "embedded.material", 123)
	if pathID != 123 {
		t.Fatalf("PathID = %d, want 123", pathID)
	}
	var serialized bytes.Buffer
	if err := writer.Write(&serialized); err != nil {
		t.Fatal(err)
	}
	af, err := ReadAssetsFile(serialized.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !af.Metadata.TypeTreeEnabled {
		t.Fatal("SerializedFile TypeTree is disabled")
	}
	info := af.GetAssetInfoByPathID(pathID)
	if info == nil || !af.AssetHasTypeTree(info) {
		t.Fatalf("material object has no TypeTree: %+v", info)
	}
	decoded, err := af.ReadAssetValue(info)
	if err != nil {
		t.Fatal(err)
	}
	name, ok := decoded.Field("m_Name").String()
	if !ok || name != "embedded" || decoded.Field("m_Custom").Value != int64(41) {
		t.Fatalf("decoded embedded object = %+v", decoded)
	}
}

func TestNativeUnityAudioClipPreservesInlinePayload(t *testing.T) {
	var stringsBuffer []byte
	stringOffset := func(value string) uint32 {
		offset := uint32(len(stringsBuffer))
		stringsBuffer = append(stringsBuffer, value...)
		stringsBuffer = append(stringsBuffer, 0)
		return offset
	}
	tree := TypeTreeType{
		TypeId:          ClassIDAudioClip,
		ScriptTypeIndex: -1,
		Nodes: []TypeTreeNode{
			{Level: 0, TypeStrOff: stringOffset("AudioClip"), NameStrOff: stringOffset("Base"), ByteSize: -1},
			{Level: 1, TypeStrOff: stringOffset("string"), NameStrOff: stringOffset("m_Name"), ByteSize: -1, MetaFlags: 0x4000},
			{Level: 1, TypeStrOff: stringOffset("StreamedResource"), NameStrOff: stringOffset("m_Resource"), ByteSize: -1},
			{Level: 2, TypeStrOff: stringOffset("string"), NameStrOff: stringOffset("m_Source"), ByteSize: -1, MetaFlags: 0x4000},
			{Level: 2, TypeStrOff: stringOffset("UInt64"), NameStrOff: stringOffset("m_Offset"), ByteSize: 8},
			{Level: 2, TypeStrOff: stringOffset("UInt64"), NameStrOff: stringOffset("m_Size"), ByteSize: 8},
		},
		StringBuffer: stringsBuffer,
	}
	root := &TypeTreeValue{
		TypeName: "AudioClip",
		Name:     "Base",
		Children: []*TypeTreeValue{
			{TypeName: "string", Name: "m_Name", Value: "voice"},
			{TypeName: "StreamedResource", Name: "m_Resource", Children: []*TypeTreeValue{
				{TypeName: "string", Name: "m_Source", Value: ""},
				{TypeName: "UInt64", Name: "m_Offset", Value: uint64(0)},
				{TypeName: "UInt64", Name: "m_Size", Value: uint64(4)},
			}},
		},
	}
	object := &NativeUnityObject{ClassID: ClassIDAudioClip, TypeTree: tree}
	var err error
	object.Data, err = object.EncodeValueWithTrailingData(root, []byte("OggS"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, trailing, err := object.DecodeValueAndTrailingData()
	if err != nil {
		t.Fatal(err)
	}
	if name, ok := decoded.Field("m_Name").String(); !ok || name != "voice" || !bytes.Equal(trailing, []byte("OggS")) {
		t.Fatalf("decoded AudioClip = %+v trailing=%q", decoded, trailing)
	}
	audioData, err := object.AudioData()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(audioData, []byte("OggS")) {
		t.Fatalf("AudioData = %q", audioData)
	}
	decoded.Field("m_Name").Value = "edited"
	edited, err := object.EncodeValue(decoded)
	if err != nil {
		t.Fatal(err)
	}
	object.Data = edited
	if audioData, err = object.AudioData(); err != nil || !bytes.Equal(audioData, []byte("OggS")) {
		t.Fatalf("edited AudioData = %q, %v", audioData, err)
	}
}

func nativeObjectTestTypeTree(classID int32) TypeTreeType {
	var stringsBuffer []byte
	stringOffset := func(value string) uint32 {
		offset := uint32(len(stringsBuffer))
		stringsBuffer = append(stringsBuffer, value...)
		stringsBuffer = append(stringsBuffer, 0)
		return offset
	}
	return TypeTreeType{
		TypeId:          classID,
		ScriptTypeIndex: -1,
		TypeHash:        [16]byte{1, 2, 3},
		Nodes: []TypeTreeNode{
			{Version: 1, Level: 0, TypeStrOff: stringOffset("Material"), NameStrOff: stringOffset("Base"), ByteSize: -1},
			{Version: 1, Level: 1, TypeStrOff: stringOffset("string"), NameStrOff: stringOffset("m_Name"), ByteSize: -1, MetaFlags: 0x4000},
			{Version: 1, Level: 1, TypeStrOff: stringOffset("int"), NameStrOff: stringOffset("m_Custom"), ByteSize: 4},
		},
		StringBuffer:     stringsBuffer,
		TypeDependencies: []int32{classID},
	}
}
