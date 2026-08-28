package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

func TestModel(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.model")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()

			br := bufio.NewReader(f)
			model, err := ReadModel(br)
			if err != nil {
				t.Fatalf("failed to read model: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			bw := bufio.NewWriter(&buf)
			err = model.Dump(bw)
			if err != nil {
				t.Fatalf("failed to dump model: %v", err)
			}
			bw.Flush()

			// Re-read from a dumped buffer
			br2 := bufio.NewReader(&buf)
			model2, err := ReadModel(br2)
			if err != nil {
				t.Fatalf("failed to re-read dumped model: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(model, model2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}

func TestReadModelMetadata(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.model")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()

			model, err := ReadModel(bufio.NewReader(f))
			if err != nil {
				t.Fatalf("failed to read model: %v", err)
			}

			_, err = f.Seek(0, 0)
			if err != nil {
				t.Fatalf("failed to seek: %v", err)
			}
			metadata, err := ReadModelMetadata(bufio.NewReader(f))
			if err != nil {
				t.Fatalf("failed to read model metadata: %v", err)
			}

			if model.Signature != metadata.Signature {
				t.Errorf("Signature mismatch: %s != %s", model.Signature, metadata.Signature)
			}
			if model.Version != metadata.Version {
				t.Errorf("Version mismatch: %d != %d", model.Version, metadata.Version)
			}
			if model.Name != metadata.Name {
				t.Errorf("Name mismatch: %s != %s", model.Name, metadata.Name)
			}
			if model.RootBoneName != metadata.RootBoneName {
				t.Errorf("RootBoneName mismatch: %s != %s", model.RootBoneName, metadata.RootBoneName)
			}
			if len(model.Materials) != len(metadata.Materials) {
				t.Errorf("Materials count mismatch: %d != %d", len(model.Materials), len(metadata.Materials))
			}
		})
	}
}

func TestSkinThicknessPreservesWireGroupOrder(t *testing.T) {
	groups := map[string]*ThickGroup{
		"a": {GroupName: "group-a"},
		"b": {GroupName: "group-b"},
	}
	value := &SkinThickness{
		Signature:  SkinThicknessSignature,
		Version:    SkinThicknessVersion,
		Use:        true,
		Groups:     groups,
		GroupOrder: []string{"b", "a"},
	}

	var wire bytes.Buffer
	if err := writeSkinThickness(stream.NewBinaryWriter(&wire), value); err != nil {
		t.Fatalf("writeSkinThickness: %v", err)
	}
	decoded, err := ReadSkinThickness(stream.NewBinaryReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("ReadSkinThickness: %v", err)
	}
	if !reflect.DeepEqual(decoded.GroupOrder, []string{"b", "a"}) {
		t.Fatalf("group order = %#v", decoded.GroupOrder)
	}

	var reencoded bytes.Buffer
	if err := writeSkinThickness(stream.NewBinaryWriter(&reencoded), decoded); err != nil {
		t.Fatalf("rewrite SkinThickness: %v", err)
	}
	if !bytes.Equal(reencoded.Bytes(), wire.Bytes()) {
		t.Fatalf("SkinThickness wire order changed")
	}

	value.GroupOrder = nil
	wire.Reset()
	if err := writeSkinThickness(stream.NewBinaryWriter(&wire), value); err != nil {
		t.Fatalf("write sorted SkinThickness: %v", err)
	}
	decoded, err = ReadSkinThickness(stream.NewBinaryReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("read sorted SkinThickness: %v", err)
	}
	if !reflect.DeepEqual(decoded.GroupOrder, []string{"a", "b"}) {
		t.Fatalf("fallback group order = %#v, want sorted keys", decoded.GroupOrder)
	}

	value.Groups["c"] = &ThickGroup{GroupName: "group-c"}
	value.GroupOrder = []string{"b", "removed"}
	wire.Reset()
	if err := writeSkinThickness(stream.NewBinaryWriter(&wire), value); err != nil {
		t.Fatalf("write edited SkinThickness: %v", err)
	}
	decoded, err = ReadSkinThickness(stream.NewBinaryReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("read edited SkinThickness: %v", err)
	}
	if !reflect.DeepEqual(decoded.GroupOrder, []string{"b", "a", "c"}) {
		t.Fatalf("edited group order = %#v, want preserved live keys followed by sorted additions", decoded.GroupOrder)
	}
}

func TestModelDumpRejectsContradictoryLayoutsBeforeWriting(t *testing.T) {
	stringPointer := func(value string) *string { return &value }
	base := func() *Model {
		return &Model{Signature: ModelSignature, Version: 2102}
	}
	withOneVertex := func(value *Model) {
		value.VertCount = 1
		value.Vertices = []Vertex{{}}
		value.BoneWeights = []BoneWeight{{}}
	}

	tests := []struct {
		name   string
		mutate func(*Model)
	}{
		{name: "nil bone", mutate: func(value *Model) { value.Bones = []*Bone{nil} }},
		{name: "bone weight count mismatch", mutate: func(value *Model) { withOneVertex(value); value.BoneWeights = nil }},
		{name: "inconsistent vertex channels", mutate: func(value *Model) {
			value.VertCount = 2
			value.Vertices = []Vertex{{UV2: &Vector2{}}, {}}
			value.BoneWeights = make([]BoneWeight, 2)
		}},
		{name: "old version extended vertex channel", mutate: func(value *Model) {
			value.Version = 2001
			withOneVertex(value)
			value.Vertices[0].UV2 = &Vector2{}
		}},
		{name: "negative submesh index", mutate: func(value *Model) {
			value.SubMeshCount = 1
			value.SubMeshes = [][]int32{{-1}}
		}},
		{name: "nil material", mutate: func(value *Model) { value.Materials = []*Material{nil} }},
		{name: "typed nil material property", mutate: func(value *Model) {
			value.Materials = []*Material{{Properties: []Property{(*FProperty)(nil)}}}
		}},
		{name: "nil morph", mutate: func(value *Model) { value.MorphData = []*MorphData{nil} }},
		{name: "morph parallel length mismatch", mutate: func(value *Model) {
			value.MorphData = []*MorphData{{Indices: []int32{0}}}
		}},
		{name: "morph index outside uint16", mutate: func(value *Model) {
			value.MorphData = []*MorphData{{Indices: []int32{1 << 16}, Vertex: []Vector3{{}}, Normals: []Vector3{{}}}}
		}},
		{name: "old version morph tangents", mutate: func(value *Model) {
			value.Version = 2001
			value.MorphData = []*MorphData{{Tangents: make([]Quaternion, 0)}}
		}},
		{name: "old version skin thickness", mutate: func(value *Model) {
			value.Version = 2001
			value.SkinThickness = &SkinThickness{}
		}},
		{name: "nil skin thickness point", mutate: func(value *Model) {
			value.SkinThickness = &SkinThickness{Groups: map[string]*ThickGroup{"g": {Points: []*ThickPoint{nil}}}}
		}},
		{name: "unsupported shadow field", mutate: func(value *Model) { value.ShadowCastingMode = stringPointer("On") }},
		{name: "missing required shadow field", mutate: func(value *Model) { value.Version = 2104 }},
		{name: "old version bone scale", mutate: func(value *Model) {
			value.Version = 2000
			value.Bones = []*Bone{{Scale: &Vector3{}}}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := base()
			test.mutate(value)
			var wire bytes.Buffer
			if err := value.Dump(&wire); err == nil {
				t.Fatal("Dump accepted a contradictory model layout")
			}
			if wire.Len() != 0 {
				t.Fatalf("rejected model wrote %d bytes", wire.Len())
			}
		})
	}
}

func TestModelDumpRecomputesDerivedCounts(t *testing.T) {
	value := &Model{
		Signature:    ModelSignature,
		Version:      2102,
		VertCount:    -1,
		SubMeshCount: -2,
		BoneCount:    -3,
		Vertices:     []Vertex{{}},
		BoneWeights:  []BoneWeight{{}},
		SubMeshes:    [][]int32{{0}},
		BoneNames:    []string{"bone"},
		BindPoses:    []Matrix4x4{{}},
	}
	var wire bytes.Buffer
	if err := value.Dump(&wire); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if value.VertCount != 1 || value.SubMeshCount != 1 || value.BoneCount != 1 {
		t.Fatalf("derived counts were not updated: vert=%d submesh=%d bone=%d", value.VertCount, value.SubMeshCount, value.BoneCount)
	}
	decoded, err := ReadModel(bufio.NewReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("ReadModel: %v", err)
	}
	if decoded.VertCount != 1 || decoded.SubMeshCount != 1 || decoded.BoneCount != 1 {
		t.Fatalf("stored derived counts are wrong: %#v", decoded)
	}
}

func TestModelDumpMinimalCurrentLayoutCanBeRead(t *testing.T) {
	value := &Model{Signature: ModelSignature, Version: 2102}
	var wire bytes.Buffer
	if err := value.Dump(&wire); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if _, err := ReadModel(bufio.NewReader(bytes.NewReader(wire.Bytes()))); err != nil {
		t.Fatalf("ReadModel: %v", err)
	}
}
