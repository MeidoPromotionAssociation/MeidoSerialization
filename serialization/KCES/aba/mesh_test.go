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
	geometry := readMeshFixture(t, filepath.Join("..", "..", "..", "testdata", "aba", "parts_dlc395_gp003.aba_unpacked", "Mesh", "crc_dress044_shoe.mmesh"))
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
	geometry := readMeshFixture(t, filepath.Join("..", "..", "..", "testdata", "aba", "parts_dlc395_gp003.aba_unpacked", "Mesh", "crc_dress044_wear.mmesh"))
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
	geometry := readMeshFixture(t, filepath.Join("..", "..", "..", "testdata", "aba", "cm3d2_megane002.aba_unpacked", "Mesh", "cm3d2_megane002.mmesh"))
	if len(geometry.Positions) == 0 || len(geometry.Primitives) == 0 {
		t.Fatalf("fixture decoded without geometry: vertices=%d primitives=%d", len(geometry.Positions), len(geometry.Primitives))
	}
	if (len(geometry.SkinCounts) == 0) != (len(geometry.BindPoses) == 0) {
		t.Fatalf("skin channels and bind poses disagree: counts=%d bindPoses=%d", len(geometry.SkinCounts), len(geometry.BindPoses))
	}
}
