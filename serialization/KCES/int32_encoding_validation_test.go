package KCES

import (
	"bytes"
	"encoding/json"
	"testing"
)

const (
	testMinInt32 int32 = -1 << 31
	testMaxInt32 int32 = 1<<31 - 1
)

func TestPartsEncodersAcceptInt32BoundariesWithoutMutation(t *testing.T) {
	menu := newInt32TestMenuAssets()
	menu.Assets[0].Priority = testMinInt32
	menu.Assets[0].Commands[0].Type = testMaxInt32
	menu.Assets[0].PartsVer.Item2 = testMinInt32
	editInt32TestPreMul(menu, func(value *PreMulTexDatas) {
		value.MatNo = testMaxInt32
		value.PreTransTexData[0].DefTrans.SrcTexPixcel.X = testMinInt32
		value.InfColParam.PC.MainHue = testMaxInt32
	})
	menu.Assets[0].ColvariInfo.ColvariDatas[0].ColorTypeSub = testMinInt32
	assertInt32TestEncoderDoesNotMutate(t, "menu", menu, func() error {
		_, err := EncodeMenuAssets(menu)
		return err
	})

	material := newInt32TestMaterialAssets()
	material.Assets[0].Version = testMinInt32
	material.Assets[0].TextureProps[0].Type = testMaxInt32
	material.Assets[0].ColorProps[0].Type = testMinInt32
	material.Assets[0].VectorProps[0].Type = testMaxInt32
	material.Assets[0].FloatProps[0].Type = testMinInt32
	assertInt32TestEncoderDoesNotMutate(t, "material", material, func() error {
		_, err := EncodeMaterialAssets(material)
		return err
	})

	priority := &PriorityMaterialAssets{Assets: []*PriorityMaterial{{Version: testMaxInt32}}}
	assertInt32TestEncoderDoesNotMutate(t, "priority material", priority, func() error {
		_, err := EncodePriorityMaterialAssets(priority)
		return err
	})
}

func newInt32TestMenuAssets() *MenuAssets {
	preMul := NewPreMulTexDatas()
	preMul.MaskParam = &MaskParam{}
	preMul.PreTransTexData = []*TransTexData{NewTransTexData()}
	preMul.PreTransTexData[0].DefTrans = NewTransTexData()
	preMul.InfColParam = NewInfColorParam()
	preMul.InfColParam.PartCols = []*PartColDef{NewPartColDef()}
	preMul.InfColParam.GradeCols = &GradaColDef{}
	preMul.PreInfColData = NewInfColData()
	preMul.PreInfColData.PartColDefs = []*PartColDef{NewPartColDef()}
	preMul.PreInfColData.GradaColDef = &GradaColDef{}

	return &MenuAssets{Assets: []*Menu{{
		Commands:       []*Command{{Args: []*string{}}},
		PartsVer:       &TupleStringInt{},
		PreMulTexDatas: map[uint64]*PreMulTexDatas{7: preMul},
		ColvariInfo: &Colvari{ColvariDatas: []*ColvariData{{
			PartColDefs: []*PartColDef{NewPartColDef()},
			GradaColDef: &GradaColDef{},
		}}},
	}}}
}

func editInt32TestPreMul(assets *MenuAssets, edit func(*PreMulTexDatas)) {
	value := assets.Assets[0].PreMulTexDatas[7]
	edit(value)
	assets.Assets[0].PreMulTexDatas[7] = value
}

func newInt32TestMaterialAssets() *MaterialAssets {
	return &MaterialAssets{Assets: []*Material{{
		Version:      materialFixVersion,
		TextureProps: []*TextureProp{{}},
		ColorProps:   []*ColorProp{{}},
		VectorProps:  []*VectorProp{{}},
		FloatProps:   []*FloatProp{{}},
	}}}
}

func mustMarshalInt32TestJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test value: %v", err)
	}
	return data
}

func assertInt32TestEncoderDoesNotMutate(t *testing.T, name string, value interface{}, encode func() error) {
	t.Helper()
	before := mustMarshalInt32TestJSON(t, value)
	if err := encode(); err != nil {
		t.Fatalf("encode %s with Int32 boundaries: %v", name, err)
	}
	if after := mustMarshalInt32TestJSON(t, value); !bytes.Equal(after, before) {
		t.Fatalf("encode %s mutated caller\nbefore: %s\nafter:  %s", name, before, after)
	}
}
