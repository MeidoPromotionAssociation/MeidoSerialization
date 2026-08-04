package KCES

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

func officialModelSample(t *testing.T, bundle string, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "aba", bundle+".aba_unpacked", "TextAsset", name+".model")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	return path
}

func decodeModelAndMesh(t *testing.T, modelPath string) (*serializationKCES.Model, *aba.MeshGeometry) {
	t.Helper()
	modelData, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model, err := serializationKCES.DecodeModel(modelData)
	if err != nil {
		t.Fatal(err)
	}
	meshPath, err := locateModelMeshFile(modelPath, model)
	if err != nil {
		t.Fatal(err)
	}
	meshData, err := os.ReadFile(meshPath)
	if err != nil {
		t.Fatal(err)
	}
	object, err := aba.ReadMMesh(meshData)
	if err != nil {
		t.Fatal(err)
	}
	geometry, err := object.DecodeMeshGeometry()
	if err != nil {
		t.Fatal(err)
	}
	return model, geometry
}

func approxEqual(a, b, tolerance float32) bool {
	return math.Abs(float64(a)-float64(b)) <= float64(tolerance)
}

func TestModelGLTFRoundTripOfficialSamples(t *testing.T) {
	for _, sample := range []struct {
		bundle string
		name   string
	}{
		{bundle: "parts_dlc395_gp003", name: "crc_dress044_shoe"},
		{bundle: "parts_dlc395_gp003", name: "crc_dress044_wear"},
		{bundle: "parts_dlc580_gp003", name: "crc2_dress311_acchead"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			modelPath := officialModelSample(t, sample.bundle, sample.name)
			original, originalGeometry := decodeModelAndMesh(t, modelPath)

			service := &ModelService{}
			glbPath := filepath.Join(t.TempDir(), sample.name+".glb")
			if err := service.ConvertModelToGLTF(context.Background(), modelPath, glbPath, "glb", TestConversionMaxOutput); err != nil {
				t.Fatal(err)
			}

			document, err := gltf.Open(glbPath)
			if err != nil {
				t.Fatal(err)
			}
			if len(document.Nodes) != len(original.TransData) {
				t.Fatalf("glTF nodes %d != transData %d", len(document.Nodes), len(original.TransData))
			}
			if len(document.Skins) != 1 || len(document.Skins[0].Joints) != len(original.BoneNames) {
				t.Fatalf("glTF skin joints do not match boneNames %d", len(original.BoneNames))
			}
			if len(document.Meshes) != 1 || len(document.Meshes[0].Primitives) != len(originalGeometry.Primitives) {
				t.Fatalf("glTF primitives do not match SubMesh count %d", len(originalGeometry.Primitives))
			}
			if len(document.Meshes[0].Primitives[0].Targets) != len(original.Morphs) {
				t.Fatalf("glTF morph targets %d != morphs %d", len(document.Meshes[0].Primitives[0].Targets), len(original.Morphs))
			}

			outputDir := t.TempDir()
			if err := service.ConvertGLTFToModel(context.Background(), glbPath, outputDir, TestConversionMaxOutput); err != nil {
				t.Fatal(err)
			}
			rebuilt, rebuiltGeometry := decodeModelAndMesh(t, filepath.Join(outputDir, sample.name+".model"))

			assertModelsEquivalent(t, original, rebuilt)
			assertGeometryEquivalent(t, originalGeometry, rebuiltGeometry)
			assertMorphsEquivalent(t, original, rebuilt, len(originalGeometry.Positions))
		})
	}
}

func assertModelsEquivalent(t *testing.T, original, rebuilt *serializationKCES.Model) {
	t.Helper()
	if rebuilt.Version != original.Version || rebuilt.ShadowModeFlags != original.ShadowModeFlags {
		t.Fatalf("version/shadow = %d/%d, want %d/%d", rebuilt.Version, rebuilt.ShadowModeFlags, original.Version, original.ShadowModeFlags)
	}
	if *rebuilt.FileName != *original.FileName || *rebuilt.MeshFileName != *original.MeshFileName || *rebuilt.ModelName != *original.ModelName {
		t.Fatalf("names %q/%q/%q do not match original", *rebuilt.FileName, *rebuilt.MeshFileName, *rebuilt.ModelName)
	}
	if rebuilt.ID != original.ID {
		t.Fatalf("ID %d != %d", rebuilt.ID, original.ID)
	}
	if len(rebuilt.TransData) != len(original.TransData) {
		t.Fatalf("transData count %d != %d", len(rebuilt.TransData), len(original.TransData))
	}
	for transIndex := range original.TransData {
		want, got := original.TransData[transIndex], rebuilt.TransData[transIndex]
		if *got.Name != *want.Name || got.ParentNo != want.ParentNo || got.IsSCL != want.IsSCL {
			t.Fatalf("transData[%d] %q parent %d scl %t, want %q parent %d scl %t", transIndex, *got.Name, got.ParentNo, got.IsSCL, *want.Name, want.ParentNo, want.IsSCL)
		}
		for _, pair := range [][2]float32{
			{got.Pos.X, want.Pos.X}, {got.Pos.Y, want.Pos.Y}, {got.Pos.Z, want.Pos.Z},
			{got.Rot.X, want.Rot.X}, {got.Rot.Y, want.Rot.Y}, {got.Rot.Z, want.Rot.Z}, {got.Rot.W, want.Rot.W},
			{got.Scale.X, want.Scale.X}, {got.Scale.Y, want.Scale.Y}, {got.Scale.Z, want.Scale.Z},
		} {
			if !approxEqual(pair[0], pair[1], 1e-6) {
				t.Fatalf("transData[%d] %q transform differs: %f != %f", transIndex, *want.Name, pair[0], pair[1])
			}
		}
	}
	if len(rebuilt.BoneNames) != len(original.BoneNames) {
		t.Fatalf("boneNames count %d != %d", len(rebuilt.BoneNames), len(original.BoneNames))
	}
	for boneIndex := range original.BoneNames {
		if *rebuilt.BoneNames[boneIndex] != *original.BoneNames[boneIndex] {
			t.Fatalf("boneNames[%d] %q != %q", boneIndex, *rebuilt.BoneNames[boneIndex], *original.BoneNames[boneIndex])
		}
	}
	if len(rebuilt.MaterialFileName) != len(original.MaterialFileName) {
		t.Fatalf("materialFileName count %d != %d", len(rebuilt.MaterialFileName), len(original.MaterialFileName))
	}
	for materialIndex := range original.MaterialFileName {
		if *rebuilt.MaterialFileName[materialIndex] != *original.MaterialFileName[materialIndex] {
			t.Fatalf("materialFileName[%d] %q != %q", materialIndex, *rebuilt.MaterialFileName[materialIndex], *original.MaterialFileName[materialIndex])
		}
	}
	if (rebuilt.SkinThick == nil) != (original.SkinThick == nil) {
		t.Fatalf("skinThick presence %t != %t", rebuilt.SkinThick != nil, original.SkinThick != nil)
	}
}

func assertGeometryEquivalent(t *testing.T, original, rebuilt *aba.MeshGeometry) {
	t.Helper()
	if len(rebuilt.Positions) != len(original.Positions) {
		t.Fatalf("vertex count %d != %d", len(rebuilt.Positions), len(original.Positions))
	}
	for vertexIndex := range original.Positions {
		if rebuilt.Positions[vertexIndex] != original.Positions[vertexIndex] {
			t.Fatalf("position %d %v != %v", vertexIndex, rebuilt.Positions[vertexIndex], original.Positions[vertexIndex])
		}
	}
	for vertexIndex := range original.Normals {
		if rebuilt.Normals[vertexIndex] != original.Normals[vertexIndex] {
			t.Fatalf("normal %d differs", vertexIndex)
		}
	}
	for vertexIndex := range original.Tangents {
		if rebuilt.Tangents[vertexIndex] != original.Tangents[vertexIndex] {
			t.Fatalf("tangent %d differs", vertexIndex)
		}
	}
	for setIndex := range original.TexCoords {
		if len(rebuilt.TexCoords[setIndex]) != len(original.TexCoords[setIndex]) {
			t.Fatalf("uv%d count %d != %d", setIndex, len(rebuilt.TexCoords[setIndex]), len(original.TexCoords[setIndex]))
		}
		for vertexIndex := range original.TexCoords[setIndex] {
			if !approxEqual(rebuilt.TexCoords[setIndex][vertexIndex][0], original.TexCoords[setIndex][vertexIndex][0], 1e-6) ||
				!approxEqual(rebuilt.TexCoords[setIndex][vertexIndex][1], original.TexCoords[setIndex][vertexIndex][1], 1e-6) {
				t.Fatalf("uv%d[%d] %v != %v", setIndex, vertexIndex, rebuilt.TexCoords[setIndex][vertexIndex], original.TexCoords[setIndex][vertexIndex])
			}
		}
	}
	if len(rebuilt.Primitives) != len(original.Primitives) {
		t.Fatalf("primitive count %d != %d", len(rebuilt.Primitives), len(original.Primitives))
	}
	for primitiveIndex := range original.Primitives {
		wantIndices := original.Primitives[primitiveIndex].Indices
		gotIndices := rebuilt.Primitives[primitiveIndex].Indices
		if len(gotIndices) != len(wantIndices) {
			t.Fatalf("primitive %d index count %d != %d", primitiveIndex, len(gotIndices), len(wantIndices))
		}
		for indexIndex := range wantIndices {
			if gotIndices[indexIndex] != wantIndices[indexIndex] {
				t.Fatalf("primitive %d index %d = %d, want %d", primitiveIndex, indexIndex, gotIndices[indexIndex], wantIndices[indexIndex])
			}
		}
	}
	if len(rebuilt.BindPoses) != len(original.BindPoses) {
		t.Fatalf("bind pose count %d != %d", len(rebuilt.BindPoses), len(original.BindPoses))
	}
	for matrixIndex := range original.BindPoses {
		for elementIndex := int64(0); elementIndex < 16; elementIndex++ {
			if !approxEqual(rebuilt.BindPoses[matrixIndex][elementIndex], original.BindPoses[matrixIndex][elementIndex], 1e-6) {
				t.Fatalf("bind pose %d element %d differs", matrixIndex, elementIndex)
			}
		}
	}
	if len(rebuilt.SkinCounts) != len(original.SkinCounts) {
		t.Fatalf("skin count entries %d != %d", len(rebuilt.SkinCounts), len(original.SkinCounts))
	}
	originalCursor, rebuiltCursor := int64(0), int64(0)
	for vertexIndex := range original.SkinCounts {
		wantWeights := make(map[uint32]float32)
		for entryIndex := int64(0); entryIndex < int64(original.SkinCounts[vertexIndex]); entryIndex++ {
			wantWeights[original.SkinIndices[originalCursor+entryIndex]] += original.SkinWeights[originalCursor+entryIndex]
		}
		gotWeights := make(map[uint32]float32)
		for entryIndex := int64(0); entryIndex < int64(rebuilt.SkinCounts[vertexIndex]); entryIndex++ {
			gotWeights[rebuilt.SkinIndices[rebuiltCursor+entryIndex]] += rebuilt.SkinWeights[rebuiltCursor+entryIndex]
		}
		for bone, want := range wantWeights {
			if !approxEqual(gotWeights[bone], want, 3e-4) {
				t.Fatalf("vertex %d bone %d weight %f != %f", vertexIndex, bone, gotWeights[bone], want)
			}
		}
		originalCursor += int64(original.SkinCounts[vertexIndex])
		rebuiltCursor += int64(rebuilt.SkinCounts[vertexIndex])
	}
}

func assertMorphsEquivalent(t *testing.T, original, rebuilt *serializationKCES.Model, vertexCount int) {
	t.Helper()
	if len(rebuilt.Morphs) != len(original.Morphs) {
		t.Fatalf("morph count %d != %d", len(rebuilt.Morphs), len(original.Morphs))
	}
	dense := func(blend *serializationKCES.BlendData) [][3]float32 {
		result := make([][3]float32, vertexCount)
		for entryIndex, vertexIndex := range blend.VIndex {
			result[vertexIndex] = [3]float32{blend.Vert[entryIndex].X, blend.Vert[entryIndex].Y, blend.Vert[entryIndex].Z}
		}
		return result
	}
	for morphIndex := range original.Morphs {
		want, got := original.Morphs[morphIndex], rebuilt.Morphs[morphIndex]
		if *got.Name != *want.Name {
			t.Fatalf("morph %d name %q != %q", morphIndex, *got.Name, *want.Name)
		}
		wantDeltas, gotDeltas := dense(want), dense(got)
		for vertexIndex := range wantDeltas {
			for axis := int64(0); axis < 3; axis++ {
				if !approxEqual(gotDeltas[vertexIndex][axis], wantDeltas[vertexIndex][axis], 1e-6) {
					t.Fatalf("morph %q vertex %d axis %d delta %f != %f", *want.Name, vertexIndex, axis, gotDeltas[vertexIndex][axis], wantDeltas[vertexIndex][axis])
				}
			}
		}
	}
}

func TestConvertGLTFToModelSynthesizesSingleBoneSkin(t *testing.T) {
	document := gltf.NewDocument()
	positions := modeler.WritePosition(document, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}})
	indices := modeler.WriteIndices(document, []uint32{0, 1, 2})
	document.Materials = append(document.Materials, &gltf.Material{Name: "static_mat"})
	document.Meshes = []*gltf.Mesh{{
		Name: "static",
		Primitives: []*gltf.Primitive{{
			Attributes: gltf.PrimitiveAttributes{gltf.POSITION: positions},
			Indices:    gltf.Index(indices),
			Material:   gltf.Index(0),
		}},
	}}
	document.Nodes = []*gltf.Node{{Name: "static_root", Mesh: gltf.Index(0)}}
	document.Scenes[0].Nodes = []int{0}
	inputPath := filepath.Join(t.TempDir(), "Static_Prop.gltf")
	if err := gltf.Save(document, inputPath); err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()
	if err := (&ModelService{}).ConvertGLTFToModel(context.Background(), inputPath, outputDir, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	model, geometry := decodeModelAndMesh(t, filepath.Join(outputDir, "static_prop.model"))
	if *model.FileName != "static_prop.model" || *model.MeshFileName != "static_prop.mmesh" {
		t.Fatalf("derived names %q/%q", *model.FileName, *model.MeshFileName)
	}
	if len(model.BoneNames) != 1 || *model.BoneNames[0] != "static_root" || *model.ModelName != "static_root" {
		t.Fatalf("synthesized skeleton is wrong: %+v", model.BoneNames)
	}
	if len(geometry.SkinCounts) != 3 {
		t.Fatalf("skin counts %d", len(geometry.SkinCounts))
	}
	for vertexIndex, count := range geometry.SkinCounts {
		if count != 1 || geometry.SkinWeights[vertexIndex] != 1 || geometry.SkinIndices[vertexIndex] != 0 {
			t.Fatalf("vertex %d skin %d/%f/%d", vertexIndex, count, geometry.SkinWeights[vertexIndex], geometry.SkinIndices[vertexIndex])
		}
	}
	if len(model.MaterialFileName) != 1 || *model.MaterialFileName[0] != "static_mat" {
		t.Fatalf("materials %+v", model.MaterialFileName)
	}
}

func TestConvertGLTFToModelRejectsUnnamedMaterial(t *testing.T) {
	document := gltf.NewDocument()
	positions := modeler.WritePosition(document, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}})
	indices := modeler.WriteIndices(document, []uint32{0, 1, 2})
	document.Meshes = []*gltf.Mesh{{
		Name: "static",
		Primitives: []*gltf.Primitive{{
			Attributes: gltf.PrimitiveAttributes{gltf.POSITION: positions},
			Indices:    gltf.Index(indices),
		}},
	}}
	document.Nodes = []*gltf.Node{{Name: "static_root", Mesh: gltf.Index(0)}}
	document.Scenes[0].Nodes = []int{0}
	inputPath := filepath.Join(t.TempDir(), "unnamed.gltf")
	if err := gltf.Save(document, inputPath); err != nil {
		t.Fatal(err)
	}
	if err := (&ModelService{}).ConvertGLTFToModel(context.Background(), inputPath, t.TempDir(), TestConversionMaxOutput); err == nil {
		t.Fatal("expected an error for a primitive without a named material")
	}
}
