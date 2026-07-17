package KCES

import (
	"bytes"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func presetInnerString(value string) *string { return &value }

func minimalKCESPresetPropBase() KCESPresetPropBase {
	return KCESPresetPropBase{
		Type:         "None",
		SubType:      "None",
		EditBaseData: []byte{},
	}
}

func fullKCESPresetPropertyList() *KCESPresetPropertyList {
	textureName := "mask-a"
	linkLayer := "layer-link"
	partName := "part-a"
	hairPart := "hair-a"
	attachPart := "attach-a"
	attachPoint := "Bip01 Head"
	propertyName := "_Color"
	typeName := "Color"
	propertyValue := "1,2,3,4"

	base := minimalKCESPresetPropBase()
	base.Index = 777 // The wire value is retained; the game later derives idx from MPN.
	base.Type = "Object"
	base.SubType = "Texture"
	base.FileName = presetInnerString("menu.menu")
	base.FileNameRID = math.MaxUint64
	base.Enabled = true
	base.BeforeFileNameRID = 2
	base.Defines = 3
	base.SavedTextureDataRID = 4
	base.SavedTextureDataDefines = 5
	base.SavedTextureData = []KCESPresetNamedSavedTexture{{
		Key: "main",
		Value: KCESPresetSavedTextureData{
			UseLayer:         true,
			UseMultiplyAlpha: true,
			MultiplyAlpha:    0.75,
			Masks:            []KCESPresetTextureMask{{Name: &textureName, Mask: true}, {Name: nil}},
			Transforms: []*KCESPresetTextureTransform{{
				AreaUVDefault: Vector4{X: 1, Y: 2, Z: 3, W: 4},
				ScaleDefault:  Vector2{X: 5, Y: 6},
				Position:      Vector2{X: 7, Y: 8},
				Scale:         Vector2{X: 9, Y: 10},
				Rotation:      11,
				AreaUV:        Vector4{X: 12, Y: 13, Z: 14, W: 15},
				SourcePixels:  Vector2Int{X: math.MinInt32, Y: math.MaxInt32},
				Default: &KCESPresetTextureTransform{
					AreaUVDefault: Vector4{W: 1},
					ScaleDefault:  Vector2{X: 1, Y: 1},
					Scale:         Vector2{X: 1, Y: 1},
				},
			}},
			InfinityColor: &KCESPresetInfinityColorData{
				Independent:    true,
				ColorType:      "INF_COLOR",
				PartsColorType: "MUGEN_COLOR",
				Color: KCESPresetInfinityPartsColor{
					MainHue: 1, MainChroma: 2, MainBrightness: 3, MainContrast: 4,
					ShadowRate: 5, ShadowHue: 6, ShadowChroma: 7, ShadowBrightness: 8, ShadowContrast: 9,
					Gradation: []KCESPresetInfinityPartsColorPoint{{MainHue: 10, ShadowContrast: 11}},
				},
				PartColors: []KCESPresetPartColorDef{{
					PartName:     &partName,
					Color:        KCESPresetInfinityPartsColor{MainHue: 12},
					PatternScale: Vector2{X: 2, Y: 3},
					PatternRot:   4,
				}},
				Gradation: &KCESPresetGradationColorDef{
					NotUse:     presetInnerString("opaque-name"),
					PointCount: 2,
					Rates:      []float32{0.25, 0.75},
					Ranges:     []Vector4{{X: 1, W: 2}},
					Color:      KCESPresetInfinityPartsColor{MainBrightness: 13},
				},
				GradationMugen: true,
			},
			InfinityColorLinkLayer: &linkLayer,
			UseAlphaMaskTransform:  true,
		},
	}}
	base.ShareInfinityColorData = true
	// Deliberately not valid MessagePack. This layer must preserve the nested,
	// independently-versioned payload rather than trying to migrate it.
	base.EditBaseData = []byte{0xc1, 0xff, 0x00}
	base.SavedCutoutMaskRID = 6
	base.SavedCutoutMask = &KCESPresetCutoutMask{MaxLevel: 7, NowLevel: 8, Enabled: true}
	base.SavedPartHideRID = 9
	base.SavedPartHide = []KCESPresetPartHide{{PartName: &partName, Enabled: true}}
	base.UsePartHide = true
	base.SavedAttachPositionRID = 10
	base.SavedAttachPositions = []SavedAttachData{{
		Version:               SavedAttachRecordVersion,
		ExplicitVersion:       true,
		PartName:              &attachPart,
		Enabled:               true,
		MyRID:                 11,
		MySlotID:              "accAcc1",
		TargetRID:             12,
		TargetSlotID:          "future-kces-slot",
		TargetSlotNo:          13,
		TargetAttachPointName: &attachPoint,
		TargetVertexCount:     14,
		TargetVertexIndex:     15,
		NewAttachVertexIndices: []int32{
			math.MinInt32, math.MaxInt32,
		},
		PRS2: &SavedAttachPosRotScale{Position: Vector3{X: 1}, Scale: Vector3{Y: 2}, Rotation: Vector4{W: 1}},
		BoneAttachedHierarchy: map[string]SavedAttachPosRotScale{
			"z-bone": {Position: Vector3{Z: 1}},
			"a-bone": {Rotation: Vector4{W: 1}},
		},
		BoneHierarchyOrder: []string{"z-bone", "a-bone"},
		BoneAttachEdited:   true,
	}}
	base.NoScale = true
	base.SubPropertyIsTuftTexture = true
	base.SavedHairLengthRID = 16
	base.SavedHairLengths = []KCESPresetSavedHairLength{{PartName: &hairPart, Value: 0.5}}
	base.SubProperties = []*KCESPresetSubProperty{nil, {
		Number:                      17,
		DefaultHokuroTattooSlotID:   "none",
		EditUnitData:                []byte{0xc1},
		SavedDefaultHokuroTattooRID: math.MaxUint64,
		Base:                        minimalKCESPresetPropBase(),
	}}

	return &KCESPresetPropertyList{
		Signature: KCESPresetPropertyListSignature,
		Version:   KCESPresetPropertyListVersion,
		Properties: []KCESPresetNamedProperty{{
			Key: "hairf",
			Property: KCESPresetProperty{
				Signature:    KCESPresetPropertySignature,
				Version:      KCESPresetPropertyVersion,
				Name:         "hairf",
				DefaultValue: 18,
				Value:        19,
				TempValue:    20,
				FileNameRID:  math.MaxUint64,
				Enabled:      true,
				Max:          math.MaxInt32,
				Min:          math.MinInt32,
				MaterialProperties: []KCESPresetMaterialPropertySlot{{
					SlotID:    "accAcc72",
					SlotValue: 21,
					Properties: []KCESPresetNamedMaterialProperty{{
						Key: "material-key",
						RID: math.MaxUint64,
						Property: KCESPresetMaterialPropertyValue{
							MaterialNumber: math.MinInt32,
							PropertyName:   &propertyName,
							TypeName:       &typeName,
							Value:          &propertyValue,
						},
					}},
				}},
				Base: base,
			},
		}},
		TrailingData: []byte{0xde, 0xad, 0xbe, 0xef},
	}
}

func TestKCESPresetPropertyDataFullWireRoundTrip(t *testing.T) {
	want := fullKCESPresetPropertyList()
	wire, err := EncodeKCESPresetPropertyData(want)
	if err != nil {
		t.Fatalf("EncodeKCESPresetPropertyData: %v", err)
	}
	got, err := DecodeKCESPresetPropertyData(wire)
	if err != nil {
		t.Fatalf("DecodeKCESPresetPropertyData: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("property data changed:\ngot =%#v\nwant=%#v", got, want)
	}
	reencoded, err := EncodeKCESPresetPropertyData(got)
	if err != nil {
		t.Fatalf("re-encode property data: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatal("property data wire changed after decode/re-encode")
	}
}

func versionedKCESPresetPropertyList(propertyVersion int32) *KCESPresetPropertyList {
	base := minimalKCESPresetPropBase()
	base.ShareInfinityColorData = true
	base.SubProperties = []*KCESPresetSubProperty{{
		Number:                    1,
		DefaultHokuroTattooSlotID: "none",
		EditUnitData:              []byte{},
		SavedDefaultHokuroTattooRID: func() uint64 {
			if propertyVersion >= 2001 {
				return 99
			}
			return 0
		}(),
		Base: minimalKCESPresetPropBase(),
	}}
	slotValue := int32(-1)
	if propertyVersion >= 2002 {
		slotValue = 77
	}
	return &KCESPresetPropertyList{
		Signature: KCESPresetPropertyListSignature,
		Version:   -123,
		Properties: []KCESPresetNamedProperty{{
			Key: "wire-key-not-a-migration-request",
			Property: KCESPresetProperty{
				Signature: KCESPresetPropertySignature,
				Version:   propertyVersion,
				Name:      "2147483647",
				MaterialProperties: []KCESPresetMaterialPropertySlot{{
					SlotID:     "future-slot",
					SlotValue:  slotValue,
					Properties: []KCESPresetNamedMaterialProperty{},
				}},
				Base: base,
			},
		}},
	}
}

func TestKCESPresetPropertyVersionsArePreservedWithoutMigration(t *testing.T) {
	for _, version := range []int32{-1, 0, 2000, 2001, 2002, 2004, 2100, 2101} {
		t.Run(strconv.FormatInt(int64(version), 10), func(t *testing.T) {
			want := versionedKCESPresetPropertyList(version)
			wire, err := EncodeKCESPresetPropertyData(want)
			if err != nil {
				t.Fatalf("encode version %d: %v", version, err)
			}
			got, err := DecodeKCESPresetPropertyData(wire)
			if err != nil {
				t.Fatalf("decode version %d: %v", version, err)
			}
			if got.Version != want.Version || got.Properties[0].Property.Version != version {
				t.Fatalf("versions changed: got list/property %d/%d, want %d/%d", got.Version, got.Properties[0].Property.Version, want.Version, version)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("version %d data changed:\ngot =%#v\nwant=%#v", version, got, want)
			}
		})
	}
}

func TestKCESPresetPropertyRejectsFieldsUnrepresentableByStoredVersion(t *testing.T) {
	t.Run("material slot value before 2002", func(t *testing.T) {
		value := versionedKCESPresetPropertyList(2001)
		value.Properties[0].Property.MaterialProperties[0].SlotValue = 7
		if _, err := EncodeKCESPresetPropertyData(value); err == nil || !strings.Contains(err.Error(), "slotValue") {
			t.Fatalf("unrepresentable slotValue error = %v", err)
		}
	})

	t.Run("saved hokuro tattoo RID before 2001", func(t *testing.T) {
		value := versionedKCESPresetPropertyList(2000)
		value.Properties[0].Property.Base.SubProperties[0].SavedDefaultHokuroTattooRID = 7
		if _, err := EncodeKCESPresetPropertyData(value); err == nil || !strings.Contains(err.Error(), "savedDefaultHokuroTattooRid") {
			t.Fatalf("unrepresentable savedDefaultHokuroTattooRid error = %v", err)
		}
	})
}

func TestKCESPresetPropertyDataStructuralFailures(t *testing.T) {
	want := fullKCESPresetPropertyList()
	want.TrailingData = nil
	valid, err := EncodeKCESPresetPropertyData(want)
	if err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(valid); cut++ {
		if _, err := DecodeKCESPresetPropertyData(valid[:cut]); err == nil {
			t.Fatalf("truncated property wire at %d/%d was accepted", cut, len(valid))
		}
	}

	manualHeader := func(signature string, version, count int32) []byte {
		var out bytes.Buffer
		bw := stream.NewBinaryWriter(&out)
		if err := bw.WriteString(signature); err != nil {
			t.Fatal(err)
		}
		if err := bw.WriteInt32(version); err != nil {
			t.Fatal(err)
		}
		if err := bw.WriteInt32(count); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	for _, test := range []struct {
		name string
		wire []byte
		want string
	}{
		{"signature", manualHeader("WRONG", 1270, 0), "signature"},
		{"negative count", manualHeader(KCESPresetPropertyListSignature, 1270, -1), "negative"},
		{"count bomb", manualHeader(KCESPresetPropertyListSignature, 1270, math.MaxInt32), "cannot fit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeKCESPresetPropertyData(test.wire)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %q", err, test.want)
			}
		})
	}

	cyclic := &KCESPresetTextureTransform{}
	cyclic.Default = cyclic
	tooDeep := versionedKCESPresetPropertyList(2100)
	tooDeep.Properties[0].Property.Base.SavedTextureData = []KCESPresetNamedSavedTexture{{
		Key: "cycle", Value: KCESPresetSavedTextureData{Transforms: []*KCESPresetTextureTransform{cyclic}},
	}}
	if _, err := EncodeKCESPresetPropertyData(tooDeep); err == nil || !strings.Contains(err.Error(), "nesting") {
		t.Fatalf("cyclic transform error=%v", err)
	}
}

func TestKCESPresetColorCurrentAndLegacyRoundTripsWithoutUpgrade(t *testing.T) {
	current := &KCESPresetColorData{
		Signature:    KCESPresetColorSignature,
		Version:      1201,
		PartCount:    123, // ignored by the game's >1200 reader; preserve a positive mismatched count.
		PartNames:    []string{"hair", "999", "FUTURE_PART"},
		TrailingData: []byte{1, 2, 3},
	}
	legacy := &KCESPresetColorData{
		Signature: KCESPresetColorSignature,
		Version:   200,
		PartCount: 1,
		LegacyParts: []KCESPresetLegacyColor{{
			Use: true, MainHue: 1, MainChroma: 2, MainBrightness: 3, MainContrast: 4,
			ShadowRate: 5, ShadowHue: 6, ShadowChroma: 7, ShadowBrightness: 8, ShadowContrast: 9,
		}},
		TrailingData: []byte{4, 5},
	}
	for _, want := range []*KCESPresetColorData{current, legacy} {
		wire, err := EncodeKCESPresetColorData(want)
		if err != nil {
			t.Fatalf("encode color v%d: %v", want.Version, err)
		}
		got, err := DecodeKCESPresetColorData(wire)
		if err != nil {
			t.Fatalf("decode color v%d: %v", want.Version, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("color v%d changed:\ngot =%#v\nwant=%#v", want.Version, got, want)
		}
		reencoded, err := EncodeKCESPresetColorData(got)
		if err != nil || !bytes.Equal(reencoded, wire) {
			t.Fatalf("color v%d wire changed: equal=%v err=%v", want.Version, bytes.Equal(reencoded, wire), err)
		}
	}
}

func TestKCESPresetColorStructuralFailures(t *testing.T) {
	writeHeader := func(signature string, version, count int32) []byte {
		var out bytes.Buffer
		bw := stream.NewBinaryWriter(&out)
		if err := bw.WriteString(signature); err != nil {
			t.Fatal(err)
		}
		if err := bw.WriteInt32(version); err != nil {
			t.Fatal(err)
		}
		if err := bw.WriteInt32(count); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	for _, test := range []struct {
		name string
		wire []byte
		want string
	}{
		{"signature", writeHeader("WRONG", 1270, 0), "signature"},
		{"legacy negative count", writeHeader(KCESPresetColorSignature, 1200, -1), "negative"},
		{"current negative count", writeHeader(KCESPresetColorSignature, 1270, -1), "negative"},
		{"legacy count bomb", writeHeader(KCESPresetColorSignature, 1200, math.MaxInt32), "cannot fit"},
		{"current missing MAX", writeHeader(KCESPresetColorSignature, 1270, 0), "partNames"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeKCESPresetColorData(test.wire)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %q", err, test.want)
			}
		})
	}
	if _, err := EncodeKCESPresetColorData(&KCESPresetColorData{Signature: KCESPresetColorSignature, Version: 200, PartCount: 2, LegacyParts: make([]KCESPresetLegacyColor, 1)}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("legacy mismatch error=%v", err)
	}
	if _, err := EncodeKCESPresetColorData(&KCESPresetColorData{Signature: KCESPresetColorSignature, Version: 1270, PartNames: []string{"MAX"}}); err == nil || !strings.Contains(err.Error(), "terminator") {
		t.Fatalf("reserved terminator error=%v", err)
	}
	if _, err := EncodeKCESPresetColorData(&KCESPresetColorData{Signature: KCESPresetColorSignature, Version: 1270, PartCount: -1}); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("current negative partCount error=%v", err)
	}
}

func TestKCESPresetBodyVersionAndTrailingDataArePreserved(t *testing.T) {
	for _, version := range []int32{math.MinInt32, 0, 1270, math.MaxInt32} {
		want := &KCESPresetBodyData{
			Signature:    KCESPresetBodySignature,
			Version:      version,
			TrailingData: []byte{0xaa, 0xbb},
		}
		wire, err := EncodeKCESPresetBodyData(want)
		if err != nil {
			t.Fatalf("encode body v%d: %v", version, err)
		}
		got, err := DecodeKCESPresetBodyData(wire)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("body v%d changed: got=%#v err=%v", version, got, err)
		}
		reencoded, err := EncodeKCESPresetBodyData(got)
		if err != nil || !bytes.Equal(reencoded, wire) {
			t.Fatalf("body v%d wire changed: equal=%v err=%v", version, bytes.Equal(reencoded, wire), err)
		}
	}
}

func TestKCESPresetInnerEncodersDoNotInjectMissingSignatures(t *testing.T) {
	if _, err := EncodeKCESPresetPropertyData(&KCESPresetPropertyList{}); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("property-list empty signature error = %v", err)
	}
	if _, err := EncodeKCESPresetColorData(&KCESPresetColorData{Version: KCESPresetColorVersion}); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("color empty signature error = %v", err)
	}
	if _, err := EncodeKCESPresetBodyData(&KCESPresetBodyData{}); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("body empty signature error = %v", err)
	}

	list := NewKCESPresetPropertyList()
	list.Properties = []KCESPresetNamedProperty{{Property: KCESPresetProperty{}}}
	if _, err := EncodeKCESPresetPropertyData(list); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("nested property empty signature error = %v", err)
	}
}

func TestKCESPresetInnerConstructorsUseExplicitCurrentVersions(t *testing.T) {
	properties := NewKCESPresetPropertyList()
	colors := NewKCESPresetColorData()
	body := NewKCESPresetBodyData()
	if properties.Version != KCESPresetPropertyListVersion || colors.Version != KCESPresetColorVersion || body.Version != KCESPresetBodyVersion {
		t.Fatalf("constructor versions: properties=%d colors=%d body=%d", properties.Version, colors.Version, body.Version)
	}
	if _, err := EncodeKCESPresetPropertyData(properties); err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeKCESPresetColorData(colors); err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeKCESPresetBodyData(body); err != nil {
		t.Fatal(err)
	}
}
