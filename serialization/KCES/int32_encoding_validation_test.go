package KCES

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

const (
	testMinInt32 = int64(-1 << 31)
	testMaxInt32 = int64(1<<31 - 1)
)

func TestEncodeMenuAssetsRejectsInt32OverflowRecursively(t *testing.T) {
	tooHigh, tooLow := outOfInt32Values(t)

	tests := []struct {
		name   string
		value  int
		path   string
		mutate func(*MenuAssets, int)
	}{
		{name: "version above", value: tooHigh, path: "assetArray[0].version", mutate: func(v *MenuAssets, n int) { v.Assets[0].Version = n }},
		{name: "priority below", value: tooLow, path: "assetArray[0].priority", mutate: func(v *MenuAssets, n int) { v.Assets[0].Priority = n }},
		{name: "command type", value: tooHigh, path: "assetArray[0].commandList[0].type", mutate: func(v *MenuAssets, n int) { v.Assets[0].Commands[0].Type = n }},
		{name: "parts version tuple", value: tooLow, path: "assetArray[0].partsVer.item2", mutate: func(v *MenuAssets, n int) { v.Assets[0].PartsVer.Item2 = n }},
		{name: "target body type", value: tooHigh, path: "assetArray[0].targetBodyType", mutate: func(v *MenuAssets, n int) { v.Assets[0].TargetBodyType = n }},
		{name: "hara yure", value: tooLow, path: "assetArray[0].isHarayureAvailable", mutate: func(v *MenuAssets, n int) { v.Assets[0].IsHarayureAvailable = n }},
		{name: "skirt physics", value: tooHigh, path: "assetArray[0].skirt_phys", mutate: func(v *MenuAssets, n int) { v.Assets[0].SkirtPhys = n }},
		{name: "premul version", value: tooLow, path: "assetArray[0].preMulTexDatas[7].version", mutate: func(v *MenuAssets, n int) { editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.Version = n }) }},
		{name: "premul material", value: tooHigh, path: "assetArray[0].preMulTexDatas[7].f_nMatNo", mutate: func(v *MenuAssets, n int) { editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.MatNo = n }) }},
		{name: "premul layer", value: tooLow, path: "assetArray[0].preMulTexDatas[7].f_nLayerNo", mutate: func(v *MenuAssets, n int) { editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.LayerNo = n }) }},
		{name: "premul group layer", value: tooHigh, path: "assetArray[0].preMulTexDatas[7].f_nLayNoInGroup", mutate: func(v *MenuAssets, n int) { editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.LayNoInGroup = n }) }},
		{name: "premul body texture size", value: tooLow, path: "assetArray[0].preMulTexDatas[7].f_nTargetBodyTexSize", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.TargetBodyTexSize = n })
		}},
		{name: "mask link number", value: tooHigh, path: "assetArray[0].preMulTexDatas[7].maskParam.linkMaskNo", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.MaskParam.LinkMaskNo = n })
		}},
		{name: "texture source pixel", value: tooLow, path: "assetArray[0].preMulTexDatas[7].preTransTexData[0].srcTexPixcel.x", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.PreTransTexData[0].SrcTexPixcel.X = n })
		}},
		{name: "nested default texture source pixel", value: tooHigh, path: "assetArray[0].preMulTexDatas[7].preTransTexData[0].defTrans.srcTexPixcel.y", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.PreTransTexData[0].DefTrans.SrcTexPixcel.Y = n })
		}},
		{name: "infinity color type", value: tooLow, path: "assetArray[0].preMulTexDatas[7].infColParam.infColType", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.InfColParam.InfColType = n })
		}},
		{name: "infinity color id", value: tooHigh, path: "assetArray[0].preMulTexDatas[7].infColParam.infColorId", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.InfColParam.InfColorID = n })
		}},
		{name: "parts color", value: tooLow, path: "assetArray[0].preMulTexDatas[7].infColParam.pc.m_nShadowContrast", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.InfColParam.PC.ShadowContrast = n })
		}},
		{name: "part color definition", value: tooHigh, path: "assetArray[0].preMulTexDatas[7].infColParam.partCols[0].multi_col.m_nMainHue", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.InfColParam.PartCols[0].MultiCol.MainHue = n })
		}},
		{name: "gradient count", value: tooLow, path: "assetArray[0].preMulTexDatas[7].infColParam.gradeCols.gradaNum", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.InfColParam.GradeCols.GradaNum = n })
		}},
		{name: "precomputed color type", value: tooHigh, path: "assetArray[0].preMulTexDatas[7].preInfColData.infColType", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.PreInfColData.InfColType = n })
		}},
		{name: "precomputed parts color type", value: tooLow, path: "assetArray[0].preMulTexDatas[7].preInfColData.partsColorType", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.PreInfColData.PartsColorType = n })
		}},
		{name: "precomputed gradient count", value: tooHigh, path: "assetArray[0].preMulTexDatas[7].preInfColData.gradaColDef.gradaNum", mutate: func(v *MenuAssets, n int) {
			editInt32TestPreMul(v, func(p *PreMulTexDatas) { p.PreInfColData.GradaColDef.GradaNum = n })
		}},
		{name: "colvari version", value: tooLow, path: "assetArray[0].colvariInfo.version", mutate: func(v *MenuAssets, n int) { v.Assets[0].ColvariInfo.Version = n }},
		{name: "colvari icon color", value: tooHigh, path: "assetArray[0].colvariInfo.iconColor.m_nMainBrightness", mutate: func(v *MenuAssets, n int) { v.Assets[0].ColvariInfo.IconColor.MainBrightness = n }},
		{name: "colvari data version", value: tooLow, path: "assetArray[0].colvariInfo.colvariDatas[0].version", mutate: func(v *MenuAssets, n int) { v.Assets[0].ColvariInfo.ColvariDatas[0].Version = n }},
		{name: "colvari color type", value: tooHigh, path: "assetArray[0].colvariInfo.colvariDatas[0].colorType", mutate: func(v *MenuAssets, n int) { v.Assets[0].ColvariInfo.ColvariDatas[0].ColorType = n }},
		{name: "colvari nested parts color", value: tooLow, path: "assetArray[0].colvariInfo.colvariDatas[0].partColDefs[0].multi_col.m_nShadowHue", mutate: func(v *MenuAssets, n int) {
			v.Assets[0].ColvariInfo.ColvariDatas[0].PartColDefs[0].MultiCol.ShadowHue = n
		}},
		{name: "colvari gradient count", value: tooHigh, path: "assetArray[0].colvariInfo.colvariDatas[0].gradaColDef.gradaNum", mutate: func(v *MenuAssets, n int) { v.Assets[0].ColvariInfo.ColvariDatas[0].GradaColDef.GradaNum = n }},
		{name: "colvari sub color type", value: tooLow, path: "assetArray[0].colvariInfo.colvariDatas[0].colorTypeSub", mutate: func(v *MenuAssets, n int) { v.Assets[0].ColvariInfo.ColvariDatas[0].ColorTypeSub = n }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := newInt32TestMenuAssets()
			test.mutate(input, test.value)
			before := mustMarshalInt32TestJSON(t, input)

			_, err := EncodeMenuAssets(input)
			if err == nil || !strings.Contains(err.Error(), test.path) || !strings.Contains(err.Error(), "Int32") {
				t.Fatalf("EncodeMenuAssets error = %v, want Int32 rejection at %s", err, test.path)
			}
			if after := mustMarshalInt32TestJSON(t, input); !bytes.Equal(after, before) {
				t.Fatalf("EncodeMenuAssets mutated caller on error\nbefore: %s\nafter:  %s", before, after)
			}
		})
	}
}

func TestEncodeMaterialAssetsRejectsInt32OverflowRecursively(t *testing.T) {
	tooHigh, tooLow := outOfInt32Values(t)
	tests := []struct {
		name   string
		value  int
		path   string
		mutate func(*MaterialAssets, int)
	}{
		{name: "version", value: tooHigh, path: "assetArray[0].version", mutate: func(v *MaterialAssets, n int) { v.Assets[0].Version = n }},
		{name: "texture type", value: tooLow, path: "assetArray[0].textureProps[0].type", mutate: func(v *MaterialAssets, n int) { v.Assets[0].TextureProps[0].Type = n }},
		{name: "color type", value: tooHigh, path: "assetArray[0].colorProps[0].type", mutate: func(v *MaterialAssets, n int) { v.Assets[0].ColorProps[0].Type = n }},
		{name: "vector type", value: tooLow, path: "assetArray[0].vectorProps[0].type", mutate: func(v *MaterialAssets, n int) { v.Assets[0].VectorProps[0].Type = n }},
		{name: "float type", value: tooHigh, path: "assetArray[0].floatProps[0].type", mutate: func(v *MaterialAssets, n int) { v.Assets[0].FloatProps[0].Type = n }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := newInt32TestMaterialAssets()
			test.mutate(input, test.value)
			before := mustMarshalInt32TestJSON(t, input)
			_, err := EncodeMaterialAssets(input)
			if err == nil || !strings.Contains(err.Error(), test.path) || !strings.Contains(err.Error(), "Int32") {
				t.Fatalf("EncodeMaterialAssets error = %v, want Int32 rejection at %s", err, test.path)
			}
			if after := mustMarshalInt32TestJSON(t, input); !bytes.Equal(after, before) {
				t.Fatalf("EncodeMaterialAssets mutated caller on error\nbefore: %s\nafter:  %s", before, after)
			}
		})
	}
}

func TestEncodePriorityMaterialAssetsRejectsInt32Overflow(t *testing.T) {
	tooHigh, tooLow := outOfInt32Values(t)
	for _, test := range []struct {
		name  string
		value int
	}{
		{name: "above", value: tooHigh},
		{name: "below", value: tooLow},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := &PriorityMaterialAssets{Assets: []PriorityMaterial{{Version: test.value}}}
			before := mustMarshalInt32TestJSON(t, input)
			_, err := EncodePriorityMaterialAssets(input)
			if err == nil || !strings.Contains(err.Error(), "assetArray[0].version") || !strings.Contains(err.Error(), "Int32") {
				t.Fatalf("EncodePriorityMaterialAssets error = %v, want version Int32 rejection", err)
			}
			if after := mustMarshalInt32TestJSON(t, input); !bytes.Equal(after, before) {
				t.Fatalf("EncodePriorityMaterialAssets mutated caller on error\nbefore: %s\nafter:  %s", before, after)
			}
		})
	}
}

func TestPartsEncodersAcceptInt32BoundariesWithoutMutation(t *testing.T) {
	min := int(testMinInt32)
	max := int(testMaxInt32)

	menu := newInt32TestMenuAssets()
	menu.Assets[0].Priority = min
	menu.Assets[0].Commands[0].Type = max
	menu.Assets[0].PartsVer.Item2 = min
	editInt32TestPreMul(menu, func(p *PreMulTexDatas) {
		p.MatNo = max
		p.PreTransTexData[0].DefTrans.SrcTexPixcel.X = min
		p.InfColParam.PC.MainHue = max
	})
	menu.Assets[0].ColvariInfo.ColvariDatas[0].ColorTypeSub = min
	assertInt32TestEncoderDoesNotMutate(t, "menu", menu, func() error {
		_, err := EncodeMenuAssets(menu)
		return err
	})

	material := newInt32TestMaterialAssets()
	material.Assets[0].Version = min
	material.Assets[0].TextureProps[0].Type = max
	material.Assets[0].ColorProps[0].Type = min
	material.Assets[0].VectorProps[0].Type = max
	material.Assets[0].FloatProps[0].Type = min
	assertInt32TestEncoderDoesNotMutate(t, "material", material, func() error {
		_, err := EncodeMaterialAssets(material)
		return err
	})

	priority := &PriorityMaterialAssets{Assets: []PriorityMaterial{{Version: max}}}
	assertInt32TestEncoderDoesNotMutate(t, "priority material", priority, func() error {
		_, err := EncodePriorityMaterialAssets(priority)
		return err
	})
}

func newInt32TestMenuAssets() *MenuAssets {
	preMul := *NewPreMulTexDatas()
	preMul.MaskParam = &MaskParam{}
	preMul.PreTransTexData = []TransTexData{*NewTransTexData()}
	preMul.PreTransTexData[0].DefTrans = NewTransTexData()
	preMul.InfColParam = NewInfColorParam()
	preMul.InfColParam.PartCols = []PartColDef{*NewPartColDef()}
	preMul.InfColParam.GradeCols = &GradaColDef{}
	preMul.PreInfColData = NewInfColData()
	preMul.PreInfColData.PartColDefs = []PartColDef{*NewPartColDef()}
	preMul.PreInfColData.GradaColDef = &GradaColDef{}

	return &MenuAssets{Assets: []Menu{{
		Commands:       []Command{{Args: []string{}}},
		PartsVer:       &TupleStringInt{},
		PreMulTexDatas: map[uint64]PreMulTexDatas{7: preMul},
		ColvariInfo: &Colvari{ColvariDatas: []ColvariData{{
			PartColDefs: []PartColDef{*NewPartColDef()},
			GradaColDef: &GradaColDef{},
		}}},
	}}}
}

func editInt32TestPreMul(assets *MenuAssets, edit func(*PreMulTexDatas)) {
	value := assets.Assets[0].PreMulTexDatas[7]
	edit(&value)
	assets.Assets[0].PreMulTexDatas[7] = value
}

func newInt32TestMaterialAssets() *MaterialAssets {
	return &MaterialAssets{Assets: []Material{{
		Version:      materialFixVersion,
		TextureProps: []TextureProp{{}},
		ColorProps:   []ColorProp{{}},
		VectorProps:  []VectorProp{{}},
		FloatProps:   []FloatProp{{}},
	}}}
}

func outOfInt32Values(t *testing.T) (int, int) {
	t.Helper()
	if strconv.IntSize < 64 {
		t.Skip("host int cannot represent values outside Int32")
	}
	return int(testMaxInt32 + 1), int(testMinInt32 - 1)
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
