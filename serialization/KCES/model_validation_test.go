package KCES

import "testing"

func TestModelNameSelectsExactlyOneTransform(t *testing.T) {
	tests := []struct {
		name      string
		modelName *string
		transData []*TransData
	}{
		{name: "missing selector", transData: []*TransData{{Name: modelValidationString("root"), ParentNo: -1}}},
		{name: "unknown selector", modelName: modelValidationString("missing"), transData: []*TransData{{Name: modelValidationString("root"), ParentNo: -1}}},
		{name: "ambiguous selector", modelName: modelValidationString("root"), transData: []*TransData{{Name: modelValidationString("root"), ParentNo: -1}, {Name: modelValidationString("root"), ParentNo: -1}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := &Model{ModelName: test.modelName, TransData: test.transData}
			if _, err := EncodeModel(model); err == nil {
				t.Fatal("EncodeModel accepted an invalid modelName selector")
			}
		})
	}

	if _, err := EncodeModel(&Model{ModelName: modelValidationString("root"), TransData: []*TransData{{Name: modelValidationString("root"), ParentNo: -1}}}); err != nil {
		t.Fatalf("EncodeModel rejected a unique modelName selector: %v", err)
	}
	if _, err := EncodeModel(&Model{}); err != nil {
		t.Fatalf("EncodeModel rejected the zero-value synthetic model: %v", err)
	}
}

func TestModelEncodersPreserveRuntimeUnsafeButRepresentableStructures(t *testing.T) {
	validMorph := func() *BlendData {
		return &BlendData{
			Name:   modelValidationString("morph"),
			VIndex: []int32{0},
			Vert:   []Vector3{{}},
			Norm:   []Vector3{{}},
			Tan:    []Vector4{{}},
		}
	}
	withSkinPoint := func(point *ThicknessPoint) *SkinThickness {
		return &SkinThickness{
			Use: true,
			Groups: map[string]*ThicknessGroup{
				"group": {
					GroupName: modelValidationString("group"),
					Points:    []*ThicknessPoint{point},
				},
			},
		}
	}
	validSkinPoint := func() *ThicknessPoint {
		return &ThicknessPoint{
			TargetBoneName:   modelValidationString("root"),
			DistanceParAngle: []*ThicknessDefPerAngle{{AngleDegree: 0, VertexIndex: 0}},
		}
	}

	tests := []struct {
		name  string
		model func() Model
		want  string
	}{
		{
			name: "parent below root sentinel",
			model: func() Model {
				model := validModelForStructureTest()
				model.TransData[0].ParentNo = -2
				return model
			},
			want: "transData[0].paretnNo",
		},
		{
			name: "parent out of range",
			model: func() Model {
				model := validModelForStructureTest()
				model.TransData[0].ParentNo = int32(len(model.TransData))
				return model
			},
			want: "transData[0].paretnNo",
		},
		{
			name: "self parent",
			model: func() Model {
				model := validModelForStructureTest()
				model.TransData[0].ParentNo = 0
				return model
			},
			want: "cannot reference itself",
		},
		{
			name: "parent cycle",
			model: func() Model {
				model := validModelForStructureTest()
				model.TransData = []*TransData{{Name: modelValidationString("a"), ParentNo: 1}, {Name: modelValidationString("b"), ParentNo: 0}}
				model.ModelName = modelValidationString("a")
				return model
			},
			want: "cycle",
		},
		{
			name: "nil morph vertex indexes",
			model: func() Model {
				model := validModelForStructureTest()
				morph := validMorph()
				morph.VIndex = nil
				model.Morphs = []*BlendData{morph}
				return model
			},
			want: "morphs[0].v_index",
		},
		{
			name: "nil morph vertices",
			model: func() Model {
				model := validModelForStructureTest()
				morph := validMorph()
				morph.Vert = nil
				model.Morphs = []*BlendData{morph}
				return model
			},
			want: "morphs[0].vert",
		},
		{
			name: "nil morph normals",
			model: func() Model {
				model := validModelForStructureTest()
				morph := validMorph()
				morph.Norm = nil
				model.Morphs = []*BlendData{morph}
				return model
			},
			want: "morphs[0].norm",
		},
		{
			name: "nil morph tangents",
			model: func() Model {
				model := validModelForStructureTest()
				morph := validMorph()
				morph.Tan = nil
				model.Morphs = []*BlendData{morph}
				return model
			},
			want: "morphs[0].tan",
		},
		{
			name: "morph vertex length mismatch",
			model: func() Model {
				model := validModelForStructureTest()
				morph := validMorph()
				morph.Vert = []Vector3{}
				model.Morphs = []*BlendData{morph}
				return model
			},
			want: "morphs[0].vert length",
		},
		{
			name: "morph normal length mismatch",
			model: func() Model {
				model := validModelForStructureTest()
				morph := validMorph()
				morph.Norm = []Vector3{}
				model.Morphs = []*BlendData{morph}
				return model
			},
			want: "morphs[0].norm length",
		},
		{
			name: "morph tangent length mismatch",
			model: func() Model {
				model := validModelForStructureTest()
				morph := validMorph()
				morph.Tan = []Vector4{}
				model.Morphs = []*BlendData{morph}
				return model
			},
			want: "morphs[0].tan length",
		},
		{
			name: "negative morph vertex index",
			model: func() Model {
				model := validModelForStructureTest()
				morph := validMorph()
				morph.VIndex[0] = -1
				model.Morphs = []*BlendData{morph}
				return model
			},
			want: "morphs[0].v_index[0]",
		},
		{
			name: "nil skin groups",
			model: func() Model {
				model := validModelForStructureTest()
				model.SkinThick = &SkinThickness{Use: true}
				return model
			},
			want: "skinThick.groups",
		},
		{
			name: "empty skin groups",
			model: func() Model {
				model := validModelForStructureTest()
				model.SkinThick = &SkinThickness{Use: true, Groups: map[string]*ThicknessGroup{}}
				return model
			},
			want: "skinThick.groups",
		},
		{
			name: "nil skin points",
			model: func() Model {
				model := validModelForStructureTest()
				model.SkinThick = &SkinThickness{Use: true, Groups: map[string]*ThicknessGroup{"group": {GroupName: modelValidationString("group")}}}
				return model
			},
			want: "skinThick.groups[\"group\"].points",
		},
		{
			name: "empty skin points",
			model: func() Model {
				model := validModelForStructureTest()
				model.SkinThick = &SkinThickness{Use: true, Groups: map[string]*ThicknessGroup{"group": {GroupName: modelValidationString("group"), Points: []*ThicknessPoint{}}}}
				return model
			},
			want: "skinThick.groups[\"group\"].points",
		},
		{
			name: "nil skin angle definitions",
			model: func() Model {
				model := validModelForStructureTest()
				point := validSkinPoint()
				point.DistanceParAngle = nil
				model.SkinThick = withSkinPoint(point)
				return model
			},
			want: "distanceParAngle",
		},
		{
			name: "empty skin angle definitions",
			model: func() Model {
				model := validModelForStructureTest()
				point := validSkinPoint()
				point.DistanceParAngle = []*ThicknessDefPerAngle{}
				model.SkinThick = withSkinPoint(point)
				return model
			},
			want: "distanceParAngle",
		},
		{
			name: "negative skin vertex index",
			model: func() Model {
				model := validModelForStructureTest()
				point := validSkinPoint()
				point.DistanceParAngle[0].VertexIndex = -1
				model.SkinThick = withSkinPoint(point)
				return model
			},
			want: "distanceParAngle[0].vidx",
		},
	}

	encoders := []struct {
		name   string
		encode func(Model) ([]byte, error)
	}{
		{name: "model", encode: func(model Model) ([]byte, error) { return EncodeModel(&model) }},
		{name: "model assets", encode: func(model Model) ([]byte, error) {
			return EncodeModelAssets(&ModelAssets{Assets: []*Model{&model}})
		}},
	}
	for _, test := range tests {
		for _, encoder := range encoders {
			t.Run(test.name+"/"+encoder.name, func(t *testing.T) {
				if _, err := encoder.encode(test.model()); err != nil {
					t.Fatalf("encoder rejected a wire-representable model: %v", err)
				}
			})
		}
	}
}

func TestEncodeModelDoesNotGuessExternalMeshVertexUpperBound(t *testing.T) {
	model := validModelForStructureTest()
	model.Morphs = []*BlendData{{
		Name:   modelValidationString("external-mesh-bound"),
		VIndex: []int32{1 << 30},
		Vert:   []Vector3{{}},
		Norm:   []Vector3{{}},
		Tan:    []Vector4{{}},
	}}
	if _, err := EncodeModel(&model); err != nil {
		t.Fatalf("EncodeModel rejected an index whose upper bound requires the external mesh: %v", err)
	}
}

func validModelForStructureTest() Model {
	return Model{
		ModelName:        modelValidationString("root"),
		TransData:        []*TransData{{Name: modelValidationString("root"), ParentNo: -1}},
		BoneNames:        []*string{},
		MaterialFileName: []*string{},
		Morphs:           []*BlendData{},
		SkinThick:        &SkinThickness{},
	}
}

func modelValidationString(value string) *string { return &value }
