package KCES

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

const (
	modelTestMinInt32 = int64(-1 << 31)
	modelTestMaxInt32 = int64(1<<31 - 1)
)

type modelInt32FieldTest struct {
	name   string
	path   string
	mutate func(*Model, int)
}

func TestModelInt32WireFieldsAcceptExactBounds(t *testing.T) {
	fields := modelInt32FieldTests()
	for _, field := range fields {
		for _, bound := range []struct {
			name  string
			value int
		}{
			{name: "min", value: int(modelTestMinInt32)},
			{name: "max", value: int(modelTestMaxInt32)},
		} {
			t.Run(field.name+"/"+bound.name, func(t *testing.T) {
				model := validModelForInt32Test()
				field.mutate(&model, bound.value)
				if err := validateGameInt32Fields(&model); err != nil {
					t.Fatalf("%s=%d should fit the game's Int32 wire field: %v", field.path, bound.value, err)
				}
			})
		}
	}
}

func TestModelEncodersRejectEveryInt32WireFieldOverflowWithoutMutation(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot represent values outside Int32")
	}

	encoders := []struct {
		name     string
		newInput func(Model) (interface{}, func() ([]byte, error))
	}{
		{name: "model", newInput: func(model Model) (interface{}, func() ([]byte, error)) {
			input := &model
			return input, func() ([]byte, error) { return EncodeModel(input) }
		}},
		{name: "model assets", newInput: func(model Model) (interface{}, func() ([]byte, error)) {
			input := &ModelAssets{Assets: []Model{model}}
			return input, func() ([]byte, error) { return EncodeModelAssets(input) }
		}},
	}

	for _, field := range modelInt32FieldTests() {
		for _, overflow := range []struct {
			name  string
			value int
		}{
			{name: "below", value: int(modelTestMinInt32 - 1)},
			{name: "above", value: int(modelTestMaxInt32 + 1)},
		} {
			for _, encoder := range encoders {
				t.Run(field.name+"/"+overflow.name+"/"+encoder.name, func(t *testing.T) {
					model := validModelForInt32Test()
					field.mutate(&model, overflow.value)
					input, encode := encoder.newInput(model)
					before := mustMarshalModelInt32Test(t, input)

					_, err := encode()
					if err == nil || !strings.Contains(err.Error(), field.path) || !strings.Contains(err.Error(), "Int32") {
						t.Fatalf("encoder error = %v, want Int32 rejection at %s", err, field.path)
					}
					if after := mustMarshalModelInt32Test(t, input); !bytes.Equal(after, before) {
						t.Fatalf("encoder mutated caller on error\nbefore: %s\nafter:  %s", before, after)
					}
				})
			}
		}
	}
}

func TestModelEncodersAcceptInt32BoundariesWithoutMutation(t *testing.T) {
	model := validModelForInt32Test()
	model.Version = int(modelTestMinInt32)
	model.ShadowModeFlags = int(modelTestMaxInt32)
	model.Morphs[0].VIndex[0] = int(modelTestMaxInt32)
	group := model.SkinThick.Groups["group"]
	group.StepAngleDegree = int(modelTestMinInt32)
	group.Points[0].DistanceParAngle[0].AngleDegree = int(modelTestMaxInt32)
	group.Points[0].DistanceParAngle[0].VertexIndex = int(modelTestMaxInt32)
	model.SkinThick.Groups["group"] = group

	before := mustMarshalModelInt32Test(t, &model)
	if _, err := EncodeModel(&model); err != nil {
		t.Fatalf("EncodeModel rejected exact Int32 boundaries: %v", err)
	}
	if after := mustMarshalModelInt32Test(t, &model); !bytes.Equal(after, before) {
		t.Fatalf("EncodeModel mutated caller\nbefore: %s\nafter:  %s", before, after)
	}

	assets := &ModelAssets{Assets: []Model{model}}
	assetsBefore := mustMarshalModelInt32Test(t, assets)
	if _, err := EncodeModelAssets(assets); err != nil {
		t.Fatalf("EncodeModelAssets rejected exact Int32 boundaries: %v", err)
	}
	if after := mustMarshalModelInt32Test(t, assets); !bytes.Equal(after, assetsBefore) {
		t.Fatalf("EncodeModelAssets mutated caller\nbefore: %s\nafter:  %s", assetsBefore, after)
	}
}

func modelInt32FieldTests() []modelInt32FieldTest {
	return []modelInt32FieldTest{
		{name: "version", path: "version", mutate: func(v *Model, n int) { v.Version = n }},
		{name: "shadow mode flags", path: "shadowModeFlags", mutate: func(v *Model, n int) { v.ShadowModeFlags = n }},
		{name: "transform parent", path: "transData[0].paretnNo", mutate: func(v *Model, n int) { v.TransData[0].ParentNo = n }},
		{name: "morph vertex index", path: "morphs[0].v_index[0]", mutate: func(v *Model, n int) { v.Morphs[0].VIndex[0] = n }},
		{name: "thickness step angle", path: "skinThick.groups[\"group\"].stepAngleDgree", mutate: func(v *Model, n int) {
			group := v.SkinThick.Groups["group"]
			group.StepAngleDegree = n
			v.SkinThick.Groups["group"] = group
		}},
		{name: "thickness angle", path: "skinThick.groups[\"group\"].points[0].distanceParAngle[0].angleDgree", mutate: func(v *Model, n int) {
			group := v.SkinThick.Groups["group"]
			group.Points[0].DistanceParAngle[0].AngleDegree = n
			v.SkinThick.Groups["group"] = group
		}},
		{name: "thickness vertex index", path: "skinThick.groups[\"group\"].points[0].distanceParAngle[0].vidx", mutate: func(v *Model, n int) {
			group := v.SkinThick.Groups["group"]
			group.Points[0].DistanceParAngle[0].VertexIndex = n
			v.SkinThick.Groups["group"] = group
		}},
	}
}

func validModelForInt32Test() Model {
	return Model{
		Version:          modelFixVersion,
		TransData:        []TransData{{Name: "root", ParentNo: -1}},
		BoneNames:        []string{"root"},
		MaterialFileName: []string{},
		Morphs: []BlendData{{
			Name:   "morph",
			VIndex: []int{0},
			Vert:   []Vector3{{}},
			Norm:   []Vector3{{}},
			Tan:    []Vector4{{}},
		}},
		SkinThick: &SkinThickness{
			Use: true,
			Groups: map[string]ThicknessGroup{
				"group": {
					GroupName:       "group",
					StartBoneName:   "root",
					EndBoneName:     "root",
					StepAngleDegree: 1,
					Points: []ThicknessPoint{{
						TargetBoneName: "root",
						DistanceParAngle: []ThicknessDefPerAngle{{
							AngleDegree: 0,
							VertexIndex: 0,
						}},
					}},
				},
			},
		},
	}
}

func mustMarshalModelInt32Test(t *testing.T, value interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal model test value: %v", err)
	}
	return data
}
