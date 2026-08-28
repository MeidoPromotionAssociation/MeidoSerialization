package KCES

import (
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

func TestKCMeshRoundTripPreservesUnityFormatterLayouts(t *testing.T) {
	vertexTarget := "Vertex"
	indexTarget := "Index"
	value := &KCMesh{
		Version:               7654,
		IndexFormat:           1,
		VertexBufferCount:     2,
		VertexBufferTargetStr: &vertexTarget,
		IndexBufferTargetStr:  &indexTarget,
		BlendShapeCount:       1,
		BlendShapeNames:       []*string{&vertexTarget},
		BlendShapeFrameCount:  []int32{1},
		DeltaVertices:         [][]Vector3{{{X: 1, Y: 2, Z: 3}}},
		DeltaNormals:          [][]Vector3{{{X: 4, Y: 5, Z: 6}}},
		DeltaTangents:         [][]Vector3{{{X: 7, Y: 8, Z: 9}}},
		BlendShapeFrameWeight: []float32{100},
		BindPoses: []Matrix4x4{{
			M00: 1, M10: 2, M20: 3, M30: 4,
			M01: 5, M11: 6, M21: 7, M31: 8,
			M02: 9, M12: 10, M22: 11, M32: 12,
			M03: 13, M13: 14, M23: 15, M33: 16,
		}},
		IsReadable:         true,
		VertexCount:        1,
		SubMeshCount:       1,
		Vertices:           []Vector3{{X: 1, Y: 2, Z: 3}},
		Normals:            []Vector3{{X: 0, Y: 1, Z: 0}},
		Tangents:           []Vector4{{X: 1, W: -1}},
		UV:                 []Vector2{{X: 0.25, Y: 0.75}},
		Colors:             []Color{{R: 1, G: 0.5, B: 0.25, A: 1}},
		Colors32:           []Color32{{R: 1, G: 2, B: 3, A: 4}},
		Triangles:          []int32{0, 0, 0},
		TrianglesInSubmesh: [][]int32{{0, 0, 0}},
		BoneWeightWeight:   []float32{1},
		BoneWeightIndex:    []int32{0},
		BonePerVertex:      []byte{1},
	}

	wire, err := EncodeKCMesh(value)
	if err != nil {
		t.Fatalf("EncodeKCMesh: %v", err)
	}
	if got := rawArrayWidth(t, wire); got != 34 {
		t.Fatalf("KCMesh width = %d, want 34", got)
	}
	var slots []codec.Raw
	if err := msgpack.DecodeMsgpack(wire, &slots); err != nil {
		t.Fatalf("decode KCMesh slots: %v", err)
	}
	var matrices []codec.Raw
	if err := msgpack.DecodeMsgpack(slots[12], &matrices); err != nil {
		t.Fatalf("decode bindposes: %v", err)
	}
	if len(matrices) != 1 {
		t.Fatalf("bindposes length = %d, want 1", len(matrices))
	}
	if got := rawArrayWidth(t, matrices[0]); got != 16 {
		t.Fatalf("Matrix4x4 width = %d, want 16", got)
	}

	decoded, err := DecodeKCMesh(wire)
	if err != nil {
		t.Fatalf("DecodeKCMesh: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("KCMesh round trip mismatch\n got: %#v\nwant: %#v", decoded, value)
	}
}
