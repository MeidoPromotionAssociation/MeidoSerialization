package aba

import (
	"encoding/binary"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

func TestReadTypeTreeValue_AlignsNestedArrayNode(t *testing.T) {
	var stringBuffer []byte
	stringOffset := func(value string) uint32 {
		offset := uint32(len(stringBuffer))
		stringBuffer = append(stringBuffer, value...)
		stringBuffer = append(stringBuffer, 0)
		return offset
	}
	node := func(level byte, typeFlags byte, metaFlags uint32, typeName, name string) TypeTreeNode {
		return TypeTreeNode{
			Level:      level,
			TypeFlags:  typeFlags,
			TypeStrOff: stringOffset(typeName),
			NameStrOff: stringOffset(name),
			MetaFlags:  metaFlags,
		}
	}

	tt := &TypeTreeType{
		Nodes: []TypeTreeNode{
			node(0, 0, 0, "TestObject", "Base"),
			node(1, 0, 0, "vector", "values"),
			node(2, 1, 0x4000, "Array", "Array"),
			node(3, 0, 0, "int", "size"),
			node(3, 0, 0, "UInt8", "data"),
			node(1, 0, 0, "unsigned int", "marker"),
		},
		StringBuffer: stringBuffer,
	}

	// The one-byte vector is followed by three alignment bytes before marker.
	data := []byte{
		1, 0, 0, 0,
		0x7f, 0, 0, 0,
		0x44, 0x33, 0x22, 0x11,
	}
	r := binaryio.NewEndianReader(data, binary.LittleEndian)
	root, next, err := readTypeTreeValue(tt, r, 0)
	if err != nil {
		t.Fatalf("readTypeTreeValue: %v", err)
	}
	if next != int64(len(tt.Nodes)) {
		t.Fatalf("next node = %d, want %d", next, len(tt.Nodes))
	}
	values, ok := root.Field("values").Bytes()
	if !ok || len(values) != 1 || values[0] != 0x7f {
		t.Fatalf("values = % x, ok=%v", values, ok)
	}
	marker, ok := root.Field("marker").Int64()
	if !ok || marker != 0x11223344 {
		t.Fatalf("marker = %#x, ok=%v", marker, ok)
	}
}

func TestReadAssetValue_ArrayAlignmentRealSamples(t *testing.T) {
	tests := []struct {
		abaName string
		classID int32
	}{
		{abaName: "cm3d2_eyes.aba", classID: ClassIDSprite},
		{abaName: "crc_nt008_accha001.aba", classID: ClassIDMesh},
	}

	for _, test := range tests {
		t.Run(test.abaName, func(t *testing.T) {
			abaFile, file := openAbaSample(t, test.abaName)
			defer file.Close()

			decoded := 0
			for dirIndex, dir := range abaFile.BlockInfo.DirectoryInfos {
				if !dir.IsSerialized() {
					continue
				}
				data, err := abaFile.GetFileData(int64(dirIndex))
				if err != nil {
					t.Fatalf("GetFileData(%q): %v", dir.Name, err)
				}
				af, err := ReadAssetsFile(data)
				if err != nil {
					t.Fatalf("ReadAssetsFile(%q): %v", dir.Name, err)
				}
				for infoIndex := range af.Metadata.AssetInfos {
					info := &af.Metadata.AssetInfos[infoIndex]
					if info.TypeId != test.classID {
						continue
					}
					if _, err := af.ReadAssetValue(info); err != nil {
						t.Errorf("ReadAssetValue(%q, PathID=%d, ClassID=%d): %v", dir.Name, info.PathId, info.TypeId, err)
						continue
					}
					decoded++
				}
			}
			if decoded == 0 {
				t.Fatalf("no ClassID %d assets decoded", test.classID)
			}
		})
	}
}
