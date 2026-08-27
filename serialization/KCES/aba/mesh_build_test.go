package aba

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestUnity2022MeshTypeTreeParsesEmbeddedSchema(t *testing.T) {
	tree, err := unity2022MeshTypeTree()
	if err != nil {
		t.Fatal(err)
	}
	if tree.TypeId != ClassIDMesh || len(tree.Nodes) != 239 {
		t.Fatalf("unexpected embedded Mesh tree: classID=%d nodes=%d", tree.TypeId, len(tree.Nodes))
	}
	if tree.TypeHash == ([16]byte{}) {
		t.Fatal("embedded Mesh tree has a zero TypeHash")
	}
	root := tree.GetTypeTreeString(&tree.Nodes[0], true)
	if root != "Mesh" {
		t.Fatalf("embedded tree root type is %q", root)
	}
}

func TestNewNativeMeshObjectRoundTripsFixtureGeometry(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "testdata", "KCES", "parts_dlc395_gp003.aba_unpacked", "Mesh", "crc_dress044_shoe.mmesh")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Skipf("fixture not available: %v", err)
	}
	object, err := ReadMMesh(data)
	if err != nil {
		t.Fatal(err)
	}
	original, err := object.DecodeMeshGeometry()
	if err != nil {
		t.Fatal(err)
	}

	rebuilt, err := NewNativeMeshObject(original.Name, original)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := WriteMMesh(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	reread, err := ReadMMesh(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if reread.TypeTree.TypeHash == ([16]byte{}) {
		t.Fatal("rebuilt Mesh lost the official TypeHash")
	}
	decoded, err := reread.DecodeMeshGeometry()
	if err != nil {
		t.Fatal(err)
	}

	if decoded.Name != original.Name {
		t.Fatalf("name %q != %q", decoded.Name, original.Name)
	}
	if len(decoded.Positions) != len(original.Positions) {
		t.Fatalf("position count %d != %d", len(decoded.Positions), len(original.Positions))
	}
	for i := range original.Positions {
		if decoded.Positions[i] != original.Positions[i] {
			t.Fatalf("position %d = %v, want %v", i, decoded.Positions[i], original.Positions[i])
		}
	}
	for i := range original.Normals {
		if decoded.Normals[i] != original.Normals[i] {
			t.Fatalf("normal %d = %v, want %v", i, decoded.Normals[i], original.Normals[i])
		}
	}
	for i := range original.Tangents {
		if decoded.Tangents[i] != original.Tangents[i] {
			t.Fatalf("tangent %d = %v, want %v", i, decoded.Tangents[i], original.Tangents[i])
		}
	}
	for set := range original.TexCoords {
		if len(decoded.TexCoords[set]) != len(original.TexCoords[set]) {
			t.Fatalf("uv%d count %d != %d", set, len(decoded.TexCoords[set]), len(original.TexCoords[set]))
		}
		for i := range original.TexCoords[set] {
			if decoded.TexCoords[set][i] != original.TexCoords[set][i] {
				t.Fatalf("uv%d[%d] = %v, want %v", set, i, decoded.TexCoords[set][i], original.TexCoords[set][i])
			}
		}
	}
	for i := range original.SkinCounts {
		if decoded.SkinCounts[i] != original.SkinCounts[i] {
			t.Fatalf("skin count %d = %d, want %d", i, decoded.SkinCounts[i], original.SkinCounts[i])
		}
	}
	if len(decoded.SkinIndices) != len(original.SkinIndices) || len(decoded.SkinWeights) != len(original.SkinWeights) {
		t.Fatalf("flattened skin sizes %d/%d != %d/%d", len(decoded.SkinIndices), len(decoded.SkinWeights), len(original.SkinIndices), len(original.SkinWeights))
	}
	for i := range original.SkinIndices {
		if decoded.SkinIndices[i] != original.SkinIndices[i] || decoded.SkinWeights[i] != original.SkinWeights[i] {
			t.Fatalf("skin entry %d = %d/%f, want %d/%f", i, decoded.SkinIndices[i], decoded.SkinWeights[i], original.SkinIndices[i], original.SkinWeights[i])
		}
	}
	if len(decoded.BindPoses) != len(original.BindPoses) {
		t.Fatalf("bind pose count %d != %d", len(decoded.BindPoses), len(original.BindPoses))
	}
	for i := range original.BindPoses {
		if decoded.BindPoses[i] != original.BindPoses[i] {
			t.Fatalf("bind pose %d differs", i)
		}
	}
	if len(decoded.Primitives) != len(original.Primitives) {
		t.Fatalf("primitive count %d != %d", len(decoded.Primitives), len(original.Primitives))
	}
	for p := range original.Primitives {
		if len(decoded.Primitives[p].Indices) != len(original.Primitives[p].Indices) {
			t.Fatalf("primitive %d index count differs", p)
		}
		for i := range original.Primitives[p].Indices {
			if decoded.Primitives[p].Indices[i] != original.Primitives[p].Indices[i] {
				t.Fatalf("primitive %d index %d differs", p, i)
			}
		}
	}
}

func TestNewNativeMeshObjectRoundTripsVariableBoneWeights(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "testdata", "KCES", "parts_dlc395_gp003.aba_unpacked", "Mesh", "crc_dress044_wear.mmesh")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Skipf("fixture not available: %v", err)
	}
	object, err := ReadMMesh(data)
	if err != nil {
		t.Fatal(err)
	}
	original, err := object.DecodeMeshGeometry()
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := NewNativeMeshObject(original.Name, original)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := WriteMMesh(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	reread, err := ReadMMesh(encoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := reread.DecodeMeshGeometry()
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.SkinCounts) != len(original.SkinCounts) || len(decoded.SkinWeights) != len(original.SkinWeights) {
		t.Fatalf("skin sizes %d/%d != %d/%d", len(decoded.SkinCounts), len(decoded.SkinWeights), len(original.SkinCounts), len(original.SkinWeights))
	}
	for i := range original.SkinCounts {
		if decoded.SkinCounts[i] != original.SkinCounts[i] {
			t.Fatalf("skin count %d = %d, want %d", i, decoded.SkinCounts[i], original.SkinCounts[i])
		}
	}
	for i := range original.SkinIndices {
		if decoded.SkinIndices[i] != original.SkinIndices[i] {
			t.Fatalf("skin bone %d = %d, want %d", i, decoded.SkinIndices[i], original.SkinIndices[i])
		}
		if math.Abs(float64(decoded.SkinWeights[i]-original.SkinWeights[i])) > 3.0/65535 {
			t.Fatalf("skin weight %d = %f, want %f", i, decoded.SkinWeights[i], original.SkinWeights[i])
		}
	}
	for set := range original.TexCoords {
		if len(decoded.TexCoords[set]) != len(original.TexCoords[set]) {
			t.Fatalf("uv%d count %d != %d", set, len(decoded.TexCoords[set]), len(original.TexCoords[set]))
		}
	}
}

func TestNewNativeMeshObjectRejectsInvalidGeometry(t *testing.T) {
	valid := &MeshGeometry{
		Positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		Primitives: []MeshPrimitive{{
			Mode:    MeshPrimitiveModeTriangles,
			Indices: []uint32{0, 1, 2},
		}},
	}
	if _, err := NewNativeMeshObject("tri.mmesh", valid); err != nil {
		t.Fatalf("minimal geometry rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(g *MeshGeometry)
	}{
		{"empty name", func(g *MeshGeometry) { g.Name = "" }},
		{"no positions", func(g *MeshGeometry) { g.Positions = nil }},
		{"normal count", func(g *MeshGeometry) { g.Normals = [][3]float32{{0, 0, 1}} }},
		{"line primitive", func(g *MeshGeometry) { g.Primitives[0].Mode = MeshPrimitiveModeLines }},
		{"index out of range", func(g *MeshGeometry) { g.Primitives[0].Indices = []uint32{0, 1, 9} }},
		{"weights without counts", func(g *MeshGeometry) { g.SkinWeights = []float32{1, 1, 1} }},
		{"morph targets", func(g *MeshGeometry) { g.MorphTargets = []MeshMorphTarget{{Name: "x"}} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			geometry := &MeshGeometry{
				Positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
				Primitives: []MeshPrimitive{{
					Mode:    MeshPrimitiveModeTriangles,
					Indices: []uint32{0, 1, 2},
				}},
			}
			name := "tri.mmesh"
			tc.mutate(geometry)
			if geometry.Name != "" {
				name = geometry.Name
			}
			if tc.name == "empty name" {
				name = ""
			}
			if _, err := NewNativeMeshObject(name, geometry); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestNewNativeMeshObjectWritesCompactSingleBoneSkin(t *testing.T) {
	// 每顶点单骨骼的蒙皮要写回官方的紧凑布局：没有权重通道，索引一维，尾部不补齐
	// A skin with one bone per vertex must be written back in the official compact layout: no weight channel, a one-dimensional index, and no trailing padding
	fixture := filepath.Join("..", "..", "..", "testdata", "KCES", "parts_dlc562_gp003.aba_unpacked", "Mesh", "crc2_dress283_shoe_heel.mmesh")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Skipf("fixture not available: %v", err)
	}
	object, err := ReadMMesh(data)
	if err != nil {
		t.Fatal(err)
	}
	originalRoot, err := object.DecodeValue()
	if err != nil {
		t.Fatal(err)
	}
	originalBytes, ok := originalRoot.FieldPath("m_VertexData", "m_DataSize").Bytes()
	if !ok {
		t.Fatal("fixture has no inline vertex data")
	}
	original, err := object.DecodeMeshGeometry()
	if err != nil {
		t.Fatal(err)
	}

	rebuilt, err := NewNativeMeshObject(original.Name, original)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := WriteMMesh(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	reread, err := ReadMMesh(encoded)
	if err != nil {
		t.Fatal(err)
	}
	rebuiltRoot, err := reread.DecodeValue()
	if err != nil {
		t.Fatal(err)
	}
	channels := rebuiltRoot.FieldPath("m_VertexData", "m_Channels")
	if channels == nil || len(channels.Children) != 14 {
		t.Fatalf("rebuilt Mesh has %v channel descriptors", channels)
	}
	if dimension, _ := meshUnsignedField(channels.Children[meshChannelBlendWeight], "dimension"); dimension != 0 {
		t.Fatalf("rebuilt Mesh writes %d blend weights per vertex instead of omitting the channel", dimension)
	}
	dimension, _ := meshUnsignedField(channels.Children[meshChannelBlendIndices], "dimension")
	format, _ := meshUnsignedField(channels.Children[meshChannelBlendIndices], "format")
	if dimension != 1 || format != 10 {
		t.Fatalf("rebuilt blend indices are %d components of format %d, want one UInt32", dimension, format)
	}
	rebuiltBytes, ok := rebuiltRoot.FieldPath("m_VertexData", "m_DataSize").Bytes()
	if !ok {
		t.Fatal("rebuilt Mesh has no inline vertex data")
	}
	if len(rebuiltBytes) != len(originalBytes) {
		t.Fatalf("rebuilt vertex data is %d bytes, want the official %d", len(rebuiltBytes), len(originalBytes))
	}

	decoded, err := reread.DecodeMeshGeometry()
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.SkinIndices) != len(original.SkinIndices) {
		t.Fatalf("rebuilt skin holds %d influences, want %d", len(decoded.SkinIndices), len(original.SkinIndices))
	}
	for entry := range original.SkinIndices {
		if decoded.SkinIndices[entry] != original.SkinIndices[entry] || decoded.SkinWeights[entry] != original.SkinWeights[entry] {
			t.Fatalf("skin entry %d = %d/%f, want %d/%f", entry, decoded.SkinIndices[entry], decoded.SkinWeights[entry], original.SkinIndices[entry], original.SkinWeights[entry])
		}
	}
}
