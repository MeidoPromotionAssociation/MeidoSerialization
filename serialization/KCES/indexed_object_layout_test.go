package KCES

import (
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

func TestGameIndexedObjectWidths(t *testing.T) {
	// Widths come from the highest [Key(n)] in KCES 1.34.4 plus one. Versioned
	// AMessagePackSerializationVersionControlIntKey classes include inherited
	// Key(0). Sparse Menu/Cloth keys still occupy nil array slots.
	tests := []struct {
		name  string
		value interface{}
		width int
	}{
		{name: "Material", value: &Material{}, width: 8},
		{name: "TextureProp", value: &TextureProp{}, width: 6},
		{name: "ColorProp", value: &ColorProp{}, width: 5},
		{name: "VectorProp", value: &VectorProp{}, width: 5},
		{name: "FloatProp", value: &FloatProp{}, width: 2},
		{name: "MaterialAssets", value: &MaterialAssets{}, width: 2},
		{name: "PriorityMaterial", value: &PriorityMaterial{}, width: 5},
		{name: "PriorityMaterialAssets", value: &PriorityMaterialAssets{}, width: 2},
		{name: "Model", value: &Model{}, width: 11},
		{name: "TransData", value: &TransData{}, width: 6},
		{name: "ModelAssets", value: &ModelAssets{}, width: 2},
		{name: "Menu", value: &Menu{}, width: 31},
		{name: "Command", value: &Command{}, width: 2},
		{name: "MenuAssets", value: &MenuAssets{}, width: 2},
		{name: "Vector2", value: &Vector2{}, width: 2},
		{name: "Vector2Int", value: &Vector2Int{}, width: 2},
		{name: "Vector3", value: &Vector3{}, width: 3},
		{name: "Vector4", value: &Vector4{}, width: 4},
		{name: "PartsColor", value: &PartsColor{}, width: 10},
		{name: "PreMulTexDatas", value: &PreMulTexDatas{}, width: 19},
		{name: "TransTexData", value: &TransTexData{}, width: 6},
		{name: "InfColorParam", value: &InfColorParam{}, width: 11},
		{name: "MaskData", value: &MaskData{}, width: 2},
		{name: "MaskParam", value: &MaskParam{}, width: 6},
		{name: "PartColDef", value: &PartColDef{}, width: 4},
		{name: "GradaColDef", value: &GradaColDef{}, width: 5},
		{name: "InfColData", value: &InfColData{}, width: 7},
		{name: "Colvari", value: &Colvari{}, width: 5},
		{name: "ColvariData", value: &ColvariData{}, width: 14},
		{name: "BlendData", value: &BlendData{}, width: 5},
		{name: "SkinThickness", value: &SkinThickness{}, width: 2},
		{name: "ThicknessGroup", value: &ThicknessGroup{}, width: 5},
		{name: "ThicknessPoint", value: &ThicknessPoint{}, width: 3},
		{name: "ThicknessDefPerAngle", value: &ThicknessDefPerAngle{}, width: 3},
		{name: "TupleStringInt", value: &TupleStringInt{}, width: 2},
		{name: "BezierParam", value: &BezierParam{}, width: 5},
		{name: "ClothParams", value: &ClothParams{}, width: 83},
		{name: "DynamicBoneStatus", value: &DynamicBoneStatus{}, width: 16},
		{name: "DynamicBoneAnimationFrame", value: &DynamicBoneAnimationFrame{}, width: 4},
		{name: "KCESPresetCore", value: &KCESPresetCore{}, width: 4},
		{name: "KCESPresetMeta", value: &KCESPresetMeta{}, width: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selfer, ok := test.value.(codec.Selfer)
			if !ok {
				t.Fatalf("%T does not implement codec.Selfer", test.value)
			}
			wire, err := msgpack.EncodeIndexedMsgpack(selfer)
			if err != nil {
				t.Fatalf("EncodeIndexedMsgpack: %v", err)
			}
			slots := decodeIndexedTestArray(t, wire)
			if len(slots) != test.width {
				t.Fatalf("wire width = %d, want %d", len(slots), test.width)
			}
		})
	}
}
