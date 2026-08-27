package aba

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func readMeshFixture(t *testing.T, path string) *MeshGeometry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture not available: %v", err)
	}
	object, err := ReadMMesh(data)
	if err != nil {
		t.Fatal(err)
	}
	geometry, err := object.DecodeMeshGeometry()
	if err != nil {
		t.Fatal(err)
	}
	return geometry
}

func TestDecodeMeshGeometryDecodesSkinnedFixture(t *testing.T) {
	geometry := readMeshFixture(t, filepath.Join("..", "..", "..", "testdata", "KCES", "parts_dlc395_gp003.aba_unpacked", "Mesh", "crc_dress044_shoe.mmesh"))
	vertexCount := len(geometry.Positions)
	if vertexCount == 0 {
		t.Fatal("fixture has no positions")
	}
	if len(geometry.Normals) != vertexCount || len(geometry.Tangents) != vertexCount || len(geometry.TexCoords[0]) != vertexCount {
		t.Fatalf("attribute counts mismatch: normals=%d tangents=%d uv0=%d vertices=%d", len(geometry.Normals), len(geometry.Tangents), len(geometry.TexCoords[0]), vertexCount)
	}
	if len(geometry.SkinCounts) != vertexCount {
		t.Fatalf("skin count entries %d do not match %d vertices", len(geometry.SkinCounts), vertexCount)
	}
	if len(geometry.BindPoses) == 0 {
		t.Fatal("fixture has no bind poses")
	}
	assertRaggedSkinValid(t, geometry)
	for matrixIndex, matrix := range geometry.BindPoses {
		if matrix[12] != 0 || matrix[13] != 0 || matrix[14] != 0 || matrix[15] != 1 {
			t.Fatalf("bind pose %d bottom row is %v instead of affine 0,0,0,1", matrixIndex, matrix[12:16])
		}
	}
	if len(geometry.Primitives) == 0 {
		t.Fatal("fixture has no primitives")
	}
}

func TestDecodeMeshGeometryDecodesVariableBoneWeightFixture(t *testing.T) {
	geometry := readMeshFixture(t, filepath.Join("..", "..", "..", "testdata", "KCES", "parts_dlc395_gp003.aba_unpacked", "Mesh", "crc_dress044_wear.mmesh"))
	maxCount := uint8(0)
	for _, count := range geometry.SkinCounts {
		if count > maxCount {
			maxCount = count
		}
	}
	if maxCount <= 4 {
		t.Fatalf("fixture max influence count is %d, expected more than four", maxCount)
	}
	if len(geometry.TexCoords[1]) == 0 || len(geometry.TexCoords[2]) == 0 {
		t.Fatalf("fixture UV sets missing: uv1=%d uv2=%d", len(geometry.TexCoords[1]), len(geometry.TexCoords[2]))
	}
	assertRaggedSkinValid(t, geometry)
}

func TestDecodeMeshGeometryDecodesReducedSkinChannelFixture(t *testing.T) {
	geometry := readMeshFixture(t, filepath.Join("..", "..", "..", "testdata", "KCES", "parts_dlc580_gp003.aba_unpacked", "Mesh", "crc2_dress311_acchead.mmesh"))
	maxCount := uint8(0)
	for _, count := range geometry.SkinCounts {
		if count > maxCount {
			maxCount = count
		}
	}
	if maxCount == 0 || maxCount > 2 {
		t.Fatalf("fixture max influence count is %d, expected one or two from the reduced skin channels", maxCount)
	}
	assertRaggedSkinValid(t, geometry)
}

func assertRaggedSkinValid(t *testing.T, geometry *MeshGeometry) {
	t.Helper()
	boneCount := uint32(len(geometry.BindPoses))
	cursor := int64(0)
	for vertexIndex, count := range geometry.SkinCounts {
		if count == 0 {
			t.Fatalf("vertex %d has no bone influences", vertexIndex)
		}
		var sum float32
		for entryIndex := int64(0); entryIndex < int64(count); entryIndex++ {
			weight := geometry.SkinWeights[cursor+entryIndex]
			bone := geometry.SkinIndices[cursor+entryIndex]
			if bone >= boneCount {
				t.Fatalf("vertex %d references bone %d outside %d bind poses", vertexIndex, bone, boneCount)
			}
			if !(weight > 0) || weight > 1 {
				t.Fatalf("vertex %d has invalid weight %f", vertexIndex, weight)
			}
			sum += weight
		}
		if math.Abs(float64(sum)-1) > 1e-3 {
			t.Fatalf("vertex %d bone weights sum to %f", vertexIndex, sum)
		}
		cursor += int64(count)
	}
	if cursor != int64(len(geometry.SkinIndices)) || cursor != int64(len(geometry.SkinWeights)) {
		t.Fatalf("flattened skin arrays hold %d indices and %d weights for %d influences", len(geometry.SkinIndices), len(geometry.SkinWeights), cursor)
	}
}

func TestDecodeMeshGeometryDecodesLegacyOriginFixture(t *testing.T) {
	geometry := readMeshFixture(t, filepath.Join("..", "..", "..", "testdata", "KCES", "cm3d2_megane002.aba_unpacked", "Mesh", "cm3d2_megane002.mmesh"))
	if len(geometry.Positions) == 0 || len(geometry.Primitives) == 0 {
		t.Fatalf("fixture decoded without geometry: vertices=%d primitives=%d", len(geometry.Positions), len(geometry.Primitives))
	}
	if (len(geometry.SkinCounts) == 0) != (len(geometry.BindPoses) == 0) {
		t.Fatalf("skin channels and bind poses disagree: counts=%d bindPoses=%d", len(geometry.SkinCounts), len(geometry.BindPoses))
	}
}

func TestDecodeMeshGeometryDecodesSingleBoneSkinFixture(t *testing.T) {
	// 该网格每顶点只绑定一根骨骼，因此官方完全省掉了权重通道，最后一个顶点流的末尾也没有补齐到十六字节
	// Every vertex of this mesh is bound to a single bone, so the official file omits the weight channel entirely and does not pad the last vertex stream up to sixteen bytes
	geometry := readMeshFixture(t, filepath.Join("..", "..", "..", "testdata", "KCES", "parts_dlc562_gp003.aba_unpacked", "Mesh", "crc2_dress283_shoe_heel.mmesh"))
	if len(geometry.SkinCounts) != len(geometry.Positions) {
		t.Fatalf("fixture decoded %d skin counts for %d vertices", len(geometry.SkinCounts), len(geometry.Positions))
	}
	bones := make(map[uint32]struct{})
	for vertexIndex, count := range geometry.SkinCounts {
		if count != 1 {
			t.Fatalf("vertex %d has %d influences instead of a single rigid bind", vertexIndex, count)
		}
		if geometry.SkinWeights[vertexIndex] != 1 {
			t.Fatalf("vertex %d has weight %f instead of one", vertexIndex, geometry.SkinWeights[vertexIndex])
		}
		bones[geometry.SkinIndices[vertexIndex]] = struct{}{}
	}
	// 鞋跟按左右脚各绑定一根骨骼，因此索引不是常量，缺少权重通道也不等于缺少蒙皮
	// The heel binds one bone per foot, so the indices are not constant and a missing weight channel does not mean the mesh is unskinned
	if len(bones) < 2 {
		t.Fatalf("fixture uses %d distinct bones, expected one per foot", len(bones))
	}
	assertRaggedSkinValid(t, geometry)
}

func TestMeshVertexStreamsDoNotRequireTrailingAlignment(t *testing.T) {
	// 官方网格只在流与流之间补齐到十六字节，m_DataSize 正好停在最后一个流的数据末尾
	// Official meshes pad to sixteen bytes only between streams, and m_DataSize stops exactly at the end of the last stream's data
	channels := make([]meshVertexChannel, 14)
	channels[meshChannelPosition] = meshVertexChannel{Stream: 0, Format: 0, Dimension: 3}
	channels[meshChannelTexCoord0] = meshVertexChannel{Stream: 1, Format: 0, Dimension: 2}
	channels[meshChannelBlendIndices] = meshVertexChannel{Stream: 2, Format: 10, Dimension: 1}
	// 两个顶点：流零占二十四字节并补齐到三十二，流一占十六字节，流二占八字节，末尾停在五十六
	// Two vertices: stream zero takes twenty-four bytes padded to thirty-two, stream one takes sixteen, stream two takes eight, and the data ends at fifty-six
	offsets, strides, err := meshVertexStreams(2, channels, 56)
	if err != nil {
		t.Fatalf("unpadded tail rejected: %v", err)
	}
	if want := []uint64{0, 32, 48}; !equalUint64Slices(offsets, want) {
		t.Fatalf("stream offsets %v, want %v", offsets, want)
	}
	if want := []uint64{12, 8, 4}; !equalUint64Slices(strides, want) {
		t.Fatalf("stream strides %v, want %v", strides, want)
	}
	if _, _, err := meshVertexStreams(2, channels, 55); err == nil {
		t.Fatal("truncated vertex data accepted")
	}
}

func equalUint64Slices(got []uint64, want []uint64) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func TestDecodeMeshGeometryRejectsMultipleBlendIndicesWithoutWeights(t *testing.T) {
	// 只有单维索引才能推出权重必然是一，多维索引缺少权重时每个影响占多少无从核实
	// Only a one-dimensional index channel implies that the weight is necessarily one, and multi-dimensional indices without weights leave each influence's share unverifiable
	geometry := &MeshGeometry{
		Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		SkinCounts:  []uint8{2, 2, 2},
		SkinIndices: []uint32{0, 1, 0, 1, 0, 1},
		SkinWeights: []float32{0.75, 0.25, 0.5, 0.5, 0.25, 0.75},
		BindPoses:   [][16]float32{{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}, {1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}},
		Primitives: []MeshPrimitive{{
			Mode:    MeshPrimitiveModeTriangles,
			Indices: []uint32{0, 1, 2},
		}},
	}
	object, err := NewNativeMeshObject("tri.mmesh", geometry)
	if err != nil {
		t.Fatal(err)
	}
	root, err := object.DecodeValue()
	if err != nil {
		t.Fatal(err)
	}
	channels := root.FieldPath("m_VertexData", "m_Channels")
	if channels == nil || len(channels.Children) != 14 {
		t.Fatalf("rebuilt Mesh has %v channel descriptors", channels)
	}
	if err := setMeshValue(channels.Children[meshChannelBlendWeight], int64(0), "dimension"); err != nil {
		t.Fatal(err)
	}
	data, err := object.EncodeValue(root)
	if err != nil {
		t.Fatal(err)
	}
	object.Data = data
	if _, err := object.DecodeMeshGeometry(); err == nil {
		t.Fatal("four blend indices without blend weights were accepted")
	}
}
