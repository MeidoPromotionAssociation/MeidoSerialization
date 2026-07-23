package aba

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

func TestReadTypeTreeValueRejectsArrayCountsBeforeAllocation(t *testing.T) {
	t.Run("remaining byte budget", func(t *testing.T) {
		tt := robustnessArrayTypeTree("int", true)
		data := make([]byte, 8)
		binary.LittleEndian.PutUint32(data[:4], 2)
		binary.LittleEndian.PutUint32(data[4:], 123)
		assertTypeTreeReadErrorWithoutPanic(t, &tt, data, "requires at least 8 bytes")
	})

	t.Run("large wire count uses remaining byte budget", func(t *testing.T) {
		tt := robustnessArrayTypeTree("int", true)
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, (1<<20)+1)
		assertTypeTreeReadErrorWithoutPanic(t, &tt, data, "requires at least 4194308 bytes")
	})
}

func TestReadTypeTreeValueRejectsMalformedArrayWithoutDataNode(t *testing.T) {
	tt := robustnessArrayTypeTree("", false)
	assertTypeTreeReadErrorWithoutPanic(t, &tt, make([]byte, 4), "missing data node")
}

func TestReadTypeTreeValueRejectsZeroWidthArrayElements(t *testing.T) {
	tt := robustnessArrayTypeTree("", true)
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 2)
	assertTypeTreeReadErrorWithoutPanic(t, &tt, data, "consumes no bytes")
}

func TestReadTypeTreeValueRejectsAlignmentPastObjectEnd(t *testing.T) {
	t.Run("aligned primitive", func(t *testing.T) {
		tt := robustnessScalarTypeTree("UInt8", 0x4000)
		assertTypeTreeReadErrorWithoutPanic(t, &tt, []byte{1}, "align")
	})

	t.Run("aligned string payload", func(t *testing.T) {
		tt := robustnessScalarTypeTree("string", 0)
		data := []byte{1, 0, 0, 0, 'x'}
		assertTypeTreeReadErrorWithoutPanic(t, &tt, data, "unexpected EOF")
	})

	t.Run("aligned nested array", func(t *testing.T) {
		tt := robustnessArrayTypeTree("UInt8", true)
		tt.Nodes[2].MetaFlags = 0x4000
		data := []byte{1, 0, 0, 0, 0x7f}
		assertTypeTreeReadErrorWithoutPanic(t, &tt, data, "align array")
	})
}

func TestTypeTreeValueUInt64DoesNotWrapToNegative(t *testing.T) {
	tt := robustnessScalarTypeTree("UInt64", 0)
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, math.MaxUint64)
	r := binaryio.NewEndianReader(data, binary.LittleEndian)
	value, _, err := readTypeTreeValue(&tt, r, 0)
	if err != nil {
		t.Fatalf("readTypeTreeValue: %v", err)
	}
	if got, ok := value.UInt64(); !ok || got != math.MaxUint64 {
		t.Fatalf("UInt64() = %d, %v; want %d, true", got, ok, uint64(math.MaxUint64))
	}
	if got, ok := value.Int64(); ok {
		t.Fatalf("Int64() = %d, true; overflowing uint64 must not wrap", got)
	}
}

func TestReadAssetValueRejectsNilInputs(t *testing.T) {
	var nilFile *AssetsFile
	if _, err := nilFile.ReadAssetValue(&AssetInfo{}); err == nil || !strings.Contains(err.Error(), "nil assets file") {
		t.Fatalf("nil AssetsFile error = %v", err)
	}
	if _, err := (&AssetsFile{}).ReadAssetValue(nil); err == nil || !strings.Contains(err.Error(), "nil asset info") {
		t.Fatalf("nil AssetInfo error = %v", err)
	}
	if _, _, err := readTypeTreeValue(nil, binaryio.NewEndianReader(nil, binary.LittleEndian), 0); err == nil || !strings.Contains(err.Error(), "nil type tree") {
		t.Fatalf("nil TypeTree error = %v", err)
	}
	if _, _, err := readTypeTreeValue(&TypeTreeType{}, nil, 0); err == nil || !strings.Contains(err.Error(), "nil type tree reader") {
		t.Fatalf("nil reader error = %v", err)
	}
}

func TestTypeTreeRealSampleConsumptionProfile(t *testing.T) {
	files := smallAbaTestFiles(t)
	remainders := map[int]int{}
	objects := 0
	maxChildren := 0
	maxBytes := 0

	for _, filePath := range files {
		f, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("open %s: %v", filePath, err)
		}
		abaFile, err := ReadAba(f)
		if err != nil {
			_ = f.Close()
			if isEncryptedError(err) {
				continue
			}
			t.Fatalf("ReadAba(%s): %v", filePath, err)
		}

		for dirIndex, dir := range abaFile.BlockInfo.DirectoryInfos {
			if !dir.IsSerialized() {
				continue
			}
			data, err := abaFile.GetFileData(int64(dirIndex))
			if err != nil {
				t.Fatalf("GetFileData(%s:%s): %v", filePath, dir.Name, err)
			}
			af, err := ReadAssetsFile(data)
			if err != nil {
				t.Fatalf("ReadAssetsFile(%s:%s): %v", filePath, dir.Name, err)
			}
			for infoIndex := range af.Metadata.AssetInfos {
				info := &af.Metadata.AssetInfos[infoIndex]
				tt, err := af.typeTreeForAsset(info)
				if err != nil {
					t.Fatalf("typeTreeForAsset(%s:%s PathID=%d): %v", filePath, dir.Name, info.PathId, err)
				}
				objectData, err := af.GetAssetData(info)
				if err != nil {
					t.Fatalf("GetAssetData(%s:%s PathID=%d): %v", filePath, dir.Name, info.PathId, err)
				}
				r := binaryio.NewEndianReader(objectData, byteOrderForTest(af))
				root, next, err := readTypeTreeValue(tt, r, 0)
				if err != nil {
					t.Fatalf("readTypeTreeValue(%s:%s PathID=%d ClassID=%d): %v", filePath, dir.Name, info.PathId, info.TypeId, err)
				}
				if next != int64(len(tt.Nodes)) {
					t.Fatalf("readTypeTreeValue(%s:%s PathID=%d) stopped at node %d/%d", filePath, dir.Name, info.PathId, next, len(tt.Nodes))
				}
				if r.Pos() > len(objectData) {
					t.Fatalf("readTypeTreeValue(%s:%s PathID=%d) aligned past object: pos=%d size=%d", filePath, dir.Name, info.PathId, r.Pos(), len(objectData))
				}
				remainders[len(objectData)-r.Pos()]++
				profileTypeTreeValue(root, &maxChildren, &maxBytes)
				objects++
			}
		}
		_ = f.Close()
	}

	keys := make([]int, 0, len(remainders))
	for remaining := range remainders {
		keys = append(keys, remaining)
	}
	sort.Ints(keys)
	for _, remaining := range keys {
		t.Logf("object remainder %d bytes: %d objects", remaining, remainders[remaining])
	}
	if len(remainders) != 1 || remainders[0] != objects {
		t.Fatalf("real TypeTree objects were not consumed exactly: objects=%d remainders=%v", objects, remainders)
	}
	t.Logf("profiled objects=%d maxChildren=%d maxBytes=%d", objects, maxChildren, maxBytes)
}

func byteOrderForTest(af *AssetsFile) binary.ByteOrder {
	if af.Header.Endianness {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func profileTypeTreeValue(value *TypeTreeValue, maxChildren, maxBytes *int) {
	if value == nil {
		return
	}
	if len(value.Children) > *maxChildren {
		*maxChildren = len(value.Children)
	}
	if data, ok := value.Value.([]byte); ok && len(data) > *maxBytes {
		*maxBytes = len(data)
	}
	for _, child := range value.Children {
		profileTypeTreeValue(child, maxChildren, maxBytes)
	}
}

func robustnessArrayTypeTree(elementType string, includeData bool) TypeTreeType {
	stringsBuffer := []byte{}
	offset := func(value string) uint32 {
		result := uint32(len(stringsBuffer))
		stringsBuffer = append(stringsBuffer, value...)
		stringsBuffer = append(stringsBuffer, 0)
		return result
	}
	node := func(level byte, flags byte, typeName, name string) TypeTreeNode {
		return TypeTreeNode{Level: level, TypeFlags: flags, TypeStrOff: offset(typeName), NameStrOff: offset(name)}
	}
	nodes := []TypeTreeNode{
		node(0, 0, "TestObject", "Base"),
		node(1, 0, "vector", "values"),
		node(2, 1, "Array", "Array"),
		node(3, 0, "int", "size"),
	}
	if includeData {
		nodes = append(nodes, node(3, 0, elementType, "data"))
	}
	return TypeTreeType{Nodes: nodes, StringBuffer: stringsBuffer}
}

func robustnessScalarTypeTree(typeName string, metaFlags uint32) TypeTreeType {
	stringsBuffer := append([]byte(typeName), 0)
	nameOffset := uint32(len(stringsBuffer))
	stringsBuffer = append(stringsBuffer, "value"...)
	stringsBuffer = append(stringsBuffer, 0)
	return TypeTreeType{
		Nodes:        []TypeTreeNode{{TypeStrOff: 0, NameStrOff: nameOffset, MetaFlags: metaFlags}},
		StringBuffer: stringsBuffer,
	}
}

func assertTypeTreeReadErrorWithoutPanic(t *testing.T, tt *TypeTreeType, data []byte, want string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("readTypeTreeValue panicked: %v", recovered)
		}
	}()
	r := binaryio.NewEndianReader(data, binary.LittleEndian)
	_, _, err := readTypeTreeValue(tt, r, 0)
	if err == nil {
		t.Fatal("readTypeTreeValue unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}

func TestAssetBundleContainerRejectsHostileCountsAndRanges(t *testing.T) {
	t.Run("nil inputs", func(t *testing.T) {
		var nilFile *AssetsFile
		if _, err := nilFile.GetAssetBundleContainerMap(); err == nil || !strings.Contains(err.Error(), "nil assets file") {
			t.Fatalf("nil map receiver error = %v", err)
		}
		if _, err := (&AssetsFile{}).GetAssetBundleContainerEntries(nil); err == nil || !strings.Contains(err.Error(), "nil asset info") {
			t.Fatalf("nil info error = %v", err)
		}
	})

	t.Run("preload count", func(t *testing.T) {
		var data bytes.Buffer
		_ = binary.Write(&data, binary.LittleEndian, int32(0)) // m_Name
		_ = binary.Write(&data, binary.LittleEndian, int32(math.MaxInt32))
		assertAssetBundleContainerErrorWithoutPanic(t, data.Bytes(), "m_PreloadTable size")
	})

	t.Run("container count", func(t *testing.T) {
		var data bytes.Buffer
		_ = binary.Write(&data, binary.LittleEndian, int32(0)) // m_Name
		_ = binary.Write(&data, binary.LittleEndian, int32(0)) // preload count
		_ = binary.Write(&data, binary.LittleEndian, int32(math.MaxInt32))
		assertAssetBundleContainerErrorWithoutPanic(t, data.Bytes(), "m_Container size")
	})

	t.Run("preload range", func(t *testing.T) {
		var data bytes.Buffer
		for _, value := range []int32{0, 0, 1, 0, 1, 0} {
			_ = binary.Write(&data, binary.LittleEndian, value)
		}
		_ = binary.Write(&data, binary.LittleEndian, int32(0))
		_ = binary.Write(&data, binary.LittleEndian, int64(0))
		assertAssetBundleContainerErrorWithoutPanic(t, data.Bytes(), "preload range")
	})
}

func assertAssetBundleContainerErrorWithoutPanic(t *testing.T, data []byte, want string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("GetAssetBundleContainerEntries panicked: %v", recovered)
		}
	}()
	af := &AssetsFile{Header: AssetsFileHeader{Version: 22}, Data: data}
	info := &AssetInfo{TypeId: ClassIDAssetBundle, ByteSize: uint32(len(data))}
	_, err := af.GetAssetBundleContainerEntries(info)
	if err == nil {
		t.Fatal("GetAssetBundleContainerEntries unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}
