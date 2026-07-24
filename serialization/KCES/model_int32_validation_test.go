package KCES

import (
	"bytes"
	"encoding/json"
	"testing"
)

const (
	modelTestMinInt32 int32 = -1 << 31
	modelTestMaxInt32 int32 = 1<<31 - 1
)

func TestModelEncodersAcceptInt32BoundariesWithoutMutation(t *testing.T) {
	model := validModelForInt32Test()
	model.Version = modelTestMinInt32
	model.ShadowModeFlags = modelTestMaxInt32
	model.Morphs[0].VIndex[0] = modelTestMaxInt32
	group := model.SkinThick.Groups["group"]
	group.StepAngleDegree = modelTestMinInt32
	group.Points[0].DistanceParAngle[0].AngleDegree = modelTestMaxInt32
	group.Points[0].DistanceParAngle[0].VertexIndex = modelTestMaxInt32
	model.SkinThick.Groups["group"] = group

	before := mustMarshalModelInt32Test(t, &model)
	if _, err := EncodeModel(&model); err != nil {
		t.Fatalf("EncodeModel rejected exact Int32 boundaries: %v", err)
	}
	if after := mustMarshalModelInt32Test(t, &model); !bytes.Equal(after, before) {
		t.Fatalf("EncodeModel mutated caller\nbefore: %s\nafter:  %s", before, after)
	}

	assets := &ModelAssets{Assets: []*Model{&model}}
	assetsBefore := mustMarshalModelInt32Test(t, assets)
	if _, err := EncodeModelAssets(assets); err != nil {
		t.Fatalf("EncodeModelAssets rejected exact Int32 boundaries: %v", err)
	}
	if after := mustMarshalModelInt32Test(t, assets); !bytes.Equal(after, assetsBefore) {
		t.Fatalf("EncodeModelAssets mutated caller\nbefore: %s\nafter:  %s", assetsBefore, after)
	}
}

func validModelForInt32Test() Model {
	root := modelInt32String("root")
	morph := modelInt32String("morph")
	groupName := modelInt32String("group")
	return Model{
		Version:          modelFixVersion,
		ModelName:        root,
		TransData:        []*TransData{{Name: root, ParentNo: -1}},
		BoneNames:        []*string{root},
		MaterialFileName: []*string{},
		Morphs: []*BlendData{{
			Name:   morph,
			VIndex: []int32{0},
			Vert:   []Vector3{{}},
			Norm:   []Vector3{{}},
			Tan:    []Vector4{{}},
		}},
		SkinThick: &SkinThickness{
			Use: true,
			Groups: map[string]*ThicknessGroup{
				"group": {
					GroupName:       groupName,
					StartBoneName:   root,
					EndBoneName:     root,
					StepAngleDegree: 1,
					Points: []*ThicknessPoint{{
						TargetBoneName: root,
						DistanceParAngle: []*ThicknessDefPerAngle{{
							AngleDegree: 0,
							VertexIndex: 0,
						}},
					}},
				},
			},
		},
	}
}

func modelInt32String(value string) *string { return &value }

func mustMarshalModelInt32Test(t *testing.T, value interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal model test value: %v", err)
	}
	return data
}
