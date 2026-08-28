package COM3D2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

const (
	formerPresetCountLimit     = int32(65535)
	formerThumbnailLengthLimit = int32(16 << 20)
)

func testWrite(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("write test wire: %v", err)
	}
}

func buildMinimalPresetWire(t *testing.T, signature string, presetType, thumbLength int32, listSignature string, listCount int32) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := stream.NewBinaryWriter(&buf)
	testWrite(t, w.WriteString(signature))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteInt32(presetType))
	testWrite(t, w.WriteInt32(thumbLength))
	if thumbLength > 0 && thumbLength <= 16 {
		testWrite(t, w.WriteBytes(make([]byte, thumbLength)))
	}
	testWrite(t, w.WriteString(listSignature))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteInt32(listCount))
	return buf.Bytes()
}

func writeMinimalPresetProperty(t *testing.T, w *stream.BinaryWriter, signature string, version int32, name string, isCrcParts bool) {
	t.Helper()
	testWrite(t, w.WriteString(signature))
	testWrite(t, w.WriteInt32(version))
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteString(name))
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteInt32(0))
	if version >= 101 {
		testWrite(t, w.WriteInt32(0))
	}
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteString(""))
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteBool(false))
	testWrite(t, w.WriteInt32(100))
	testWrite(t, w.WriteInt32(0))
	if version >= 200 {
		testWrite(t, w.WriteInt32(0)) // sub props
		testWrite(t, w.WriteInt32(0)) // skin positions
		testWrite(t, w.WriteInt32(0)) // attach positions
		testWrite(t, w.WriteInt32(0)) // material props
		if version >= 213 {
			testWrite(t, w.WriteInt32(0)) // bone lengths
		}
	}
	if presetPropertyHasIsCrcParts(version) {
		testWrite(t, w.WriteBool(isCrcParts))
	}
}

func buildPresetWithNestedSignatures(t *testing.T, propertySignature, multiColorSignature, bodySignature string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := stream.NewBinaryWriter(&buf)
	testWrite(t, w.WriteString(PresetSignature))
	testWrite(t, w.WriteInt32(200))
	testWrite(t, w.WriteInt32(PresetTypeAll))
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteString(PresetPropertyListSignature))
	testWrite(t, w.WriteInt32(4))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("body"))
	writeMinimalPresetProperty(t, w, propertySignature, 100, "body", false)
	testWrite(t, w.WriteString(multiColorSignature))
	testWrite(t, w.WriteInt32(1201))
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteString("MAX"))
	testWrite(t, w.WriteString(bodySignature))
	testWrite(t, w.WriteInt32(200))
	return buf.Bytes()
}

func TestReadPresetRejectsMalformedEnvelope(t *testing.T) {
	base := buildMinimalPresetWire(t, PresetSignature, PresetTypeAll, 0, PresetPropertyListSignature, 0)
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"signature", buildMinimalPresetWire(t, "NOT_A_PRESET", PresetTypeAll, 0, PresetPropertyListSignature, 0), "signature"},
		{"negative thumbnail", buildMinimalPresetWire(t, PresetSignature, PresetTypeAll, -1, PresetPropertyListSignature, 0), "ThumbLength"},
		{"property list signature", buildMinimalPresetWire(t, PresetSignature, PresetTypeAll, 0, "NOT_A_LIST", 0), "signature"},
		{"negative property count", buildMinimalPresetWire(t, PresetSignature, PresetTypeAll, 0, PresetPropertyListSignature, -1), "propertyCount"},
		{"trailing byte", append(append([]byte(nil), base...), 0x7f), "trailing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadPreset(bytes.NewReader(tt.data))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ReadPreset error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestReadPresetMetadataValidatesEnvelope(t *testing.T) {
	tests := []struct {
		name        string
		signature   string
		presetType  int32
		thumbLength int32
		want        string
	}{
		{"signature", "NOT_A_PRESET", PresetTypeAll, 0, "signature"},
		{"negative thumbnail", PresetSignature, PresetTypeAll, -1, "ThumbLength"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildMinimalPresetWire(t, tt.signature, tt.presetType, tt.thumbLength, PresetPropertyListSignature, 0)
			_, err := ReadPresetMetadata(stream.NewBinaryReader(bytes.NewReader(data)))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ReadPresetMetadata error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestReadPresetPreservesUnknownPresetType(t *testing.T) {
	for _, presetType := range []int32{-1, 3, 99} {
		t.Run(fmt.Sprint(presetType), func(t *testing.T) {
			data := buildMinimalPresetWire(t, PresetSignature, presetType, 0, PresetPropertyListSignature, 0)
			preset, err := ReadPreset(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("ReadPreset rejected unknown enum value: %v", err)
			}
			if preset.PresetType != presetType {
				t.Fatalf("PresetType = %d, want %d", preset.PresetType, presetType)
			}
			var encoded bytes.Buffer
			if err := preset.Dump(&encoded); err != nil {
				t.Fatalf("Dump rejected unknown enum value: %v", err)
			}
			roundTrip, err := ReadPreset(bytes.NewReader(encoded.Bytes()))
			if err != nil || roundTrip.PresetType != presetType {
				t.Fatalf("unknown PresetType did not round-trip: value=%v error=%v", roundTrip, err)
			}
		})
	}
}

func TestReadPresetMetadataHasNoArbitraryThumbnailLimit(t *testing.T) {
	length := formerThumbnailLengthLimit + 1
	var wire bytes.Buffer
	w := stream.NewBinaryWriter(&wire)
	testWrite(t, w.WriteString(PresetSignature))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteInt32(PresetTypeAll))
	testWrite(t, w.WriteInt32(length))
	testWrite(t, w.WriteBytes(make([]byte, int(length))))

	metadata, err := ReadPresetMetadata(stream.NewBinaryReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("ReadPresetMetadata rejected a valid Int32-sized thumbnail: %v", err)
	}
	if len(metadata.ThumbData) != int(length) {
		t.Fatalf("thumbnail length = %d, want %d", len(metadata.ThumbData), length)
	}
}

func TestReadPresetCountsHaveNoLowCapacityFormatLimit(t *testing.T) {
	data := buildMinimalPresetWire(t, PresetSignature, PresetTypeAll, 0, PresetPropertyListSignature, formerPresetCountLimit+1)
	_, err := ReadPreset(bytes.NewReader(data))
	if err == nil {
		t.Fatal("truncated property list unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "maximum") {
		t.Fatalf("normal preset reader applied the low-capacity-format limit: %v", err)
	}
}

func TestReadPresetByteBlockHasNoArbitrarySizeLimit(t *testing.T) {
	var wire bytes.Buffer
	w := stream.NewBinaryWriter(&wire)
	testWrite(t, w.WriteInt32((64<<20)+1))
	_, err := readPresetByteBlock(stream.NewBinaryReader(bytes.NewReader(wire.Bytes())), "blob")
	if err == nil {
		t.Fatal("truncated byte block unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "maximum") {
		t.Fatalf("byte block reader applied an arbitrary size limit: %v", err)
	}
}

func TestReadPresetRejectsNestedSignatureMismatch(t *testing.T) {
	tests := []struct {
		name     string
		propSig  string
		colorSig string
		bodySig  string
		want     string
	}{
		{"property", "NOT_A_PROP", MultiColorSignature, BodyPropertySignature, "PresetProperty signature"},
		{"multi color", PresetPropertySignature, "NOT_A_COLOR", BodyPropertySignature, "MultiColor signature"},
		{"body", PresetPropertySignature, MultiColorSignature, "NOT_A_BODY", "Body signature"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildPresetWithNestedSignatures(t, tt.propSig, tt.colorSig, tt.bodySig)
			_, err := ReadPreset(bytes.NewReader(data))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ReadPreset error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func writePresetPropertyPrefix(t *testing.T, w *stream.BinaryWriter, version int32) {
	t.Helper()
	testWrite(t, w.WriteString(PresetPropertySignature))
	testWrite(t, w.WriteInt32(version))
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteString("body"))
	for i := 0; i < 3; i++ {
		testWrite(t, w.WriteInt32(0))
	}
	if version >= 101 {
		testWrite(t, w.WriteInt32(0))
	}
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteString(""))
	testWrite(t, w.WriteInt32(0))
	testWrite(t, w.WriteBool(false))
	testWrite(t, w.WriteInt32(100))
	testWrite(t, w.WriteInt32(0))
}

func buildPropertyWithInvalidCount(t *testing.T, field string, count int32) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := stream.NewBinaryWriter(&buf)
	writePresetPropertyPrefix(t, w, 213)
	switch field {
	case "subProp":
		testWrite(t, w.WriteInt32(count))
	case "skinPos":
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(count))
	case "attachPos":
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(count))
	case "attachPos inner":
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(1))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(count))
	case "matProp":
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(count))
	case "boneLen":
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(count))
	case "boneLen inner":
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(1))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(0))
		testWrite(t, w.WriteInt32(count))
	default:
		t.Fatalf("unknown field %q", field)
	}
	return buf.Bytes()
}

func TestReadPresetRejectsInvalidNestedCounts(t *testing.T) {
	fields := []string{"subProp", "skinPos", "attachPos", "attachPos inner", "matProp", "boneLen", "boneLen inner"}
	for _, field := range fields {
		for _, count := range []int32{-1, formerPresetCountLimit + 1} {
			name := fmt.Sprintf("%s/%d", field, count)
			t.Run(name, func(t *testing.T) {
				_, err := readPresetProperty(stream.NewBinaryReader(bytes.NewReader(buildPropertyWithInvalidCount(t, field, count))))
				if err == nil {
					t.Fatal("readPresetProperty unexpectedly accepted truncated data")
				}
				if count < 0 && !strings.Contains(err.Error(), field) {
					t.Fatalf("readPresetProperty error = %v, want error containing %q", err, field)
				}
				if count > 0 && (strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "maximum")) {
					t.Fatalf("readPresetProperty applied an arbitrary count limit: %v", err)
				}
			})
		}
	}
}

func TestReadMultiColorRejectsInvalidCount(t *testing.T) {
	for _, count := range []int32{-1, formerPresetCountLimit + 1} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var buf bytes.Buffer
			w := stream.NewBinaryWriter(&buf)
			testWrite(t, w.WriteString(MultiColorSignature))
			testWrite(t, w.WriteInt32(1200))
			testWrite(t, w.WriteInt32(count))
			_, err := readMultiColor(stream.NewBinaryReader(bytes.NewReader(buf.Bytes())))
			if err == nil {
				t.Fatal("readMultiColor unexpectedly accepted truncated data")
			}
			if count < 0 && !strings.Contains(err.Error(), "count") {
				t.Fatalf("readMultiColor error = %v, want count error", err)
			}
			if count > 0 && (strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "maximum")) {
				t.Fatalf("readMultiColor applied an arbitrary count limit: %v", err)
			}
		})
	}
}

func TestReadMultiColorPreservesOnlyWireEntries(t *testing.T) {
	var current bytes.Buffer
	w := stream.NewBinaryWriter(&current)
	testWrite(t, w.WriteString(MultiColorSignature))
	testWrite(t, w.WriteInt32(1201))
	testWrite(t, w.WriteInt32(37)) // ignored by the current game reader
	testWrite(t, w.WriteString("MAX"))
	mc, err := readMultiColor(stream.NewBinaryReader(bytes.NewReader(current.Bytes())))
	if err != nil {
		t.Fatalf("read current MultiColor: %v", err)
	}
	if mc.PartCount != 37 || len(mc.PartNames) != 0 || len(mc.PartsColors) != 0 {
		t.Fatalf("current empty wire gained runtime defaults: %#v", mc)
	}
	var currentRoundTrip bytes.Buffer
	if err := dumpMultiColor(stream.NewBinaryWriter(&currentRoundTrip), mc); err != nil {
		t.Fatalf("write current MultiColor: %v", err)
	}
	if !bytes.Equal(currentRoundTrip.Bytes(), current.Bytes()) {
		t.Fatal("current empty MultiColor wire changed")
	}

	var legacy bytes.Buffer
	w = stream.NewBinaryWriter(&legacy)
	testWrite(t, w.WriteString(MultiColorSignature))
	testWrite(t, w.WriteInt32(100))
	testWrite(t, w.WriteInt32(7))
	for i := 0; i < 7; i++ {
		testWrite(t, w.WriteBool(false))
		for j := 0; j < 9; j++ {
			testWrite(t, w.WriteInt32(0))
		}
	}
	mc, err = readMultiColor(stream.NewBinaryReader(bytes.NewReader(legacy.Bytes())))
	if err != nil {
		t.Fatalf("read legacy MultiColor: %v", err)
	}
	if mc.PartCount != 7 || len(mc.PartsColors) != 7 || len(mc.PartNames) != 0 {
		t.Fatalf("legacy wire was expanded: %#v", mc)
	}
	var legacyRoundTrip bytes.Buffer
	if err := dumpMultiColor(stream.NewBinaryWriter(&legacyRoundTrip), mc); err != nil {
		t.Fatalf("write legacy MultiColor: %v", err)
	}
	if !bytes.Equal(legacyRoundTrip.Bytes(), legacy.Bytes()) {
		t.Fatal("legacy MultiColor wire changed")
	}
}

func TestDumpMultiColorDoesNotPadOrNormalizeCurrentEntries(t *testing.T) {
	mc := &MultiColor{
		Signature:   MultiColorSignature,
		Version:     1201,
		PartCount:   99,
		PartNames:   []string{"FUTURE_COLOR", "EYE_L", "EYE_L"},
		PartsColors: []PartsColor{{MainHue: 999}, {MainHue: 7}, {MainHue: 8}},
	}
	var wire bytes.Buffer
	if err := dumpMultiColor(stream.NewBinaryWriter(&wire), mc); err != nil {
		t.Fatalf("dumpMultiColor: %v", err)
	}
	decoded, err := readMultiColor(stream.NewBinaryReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("readMultiColor: %v", err)
	}
	if decoded.PartCount != 99 || strings.Join(decoded.PartNames, ",") != "FUTURE_COLOR,EYE_L,EYE_L" ||
		len(decoded.PartsColors) != 3 || decoded.PartsColors[0].MainHue != 999 || decoded.PartsColors[2].MainHue != 8 {
		t.Fatalf("current entries were normalized: %#v", decoded)
	}

	mc.PartNames = mc.PartNames[:1]
	if err := dumpMultiColor(stream.NewBinaryWriter(&bytes.Buffer{}), mc); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched names/colors error = %v", err)
	}
}

func TestPresetPropertyIsCrcPartsVersionGates(t *testing.T) {
	tests := []struct {
		version int32
		value   bool
	}{
		{2000, false},
		{2001, true},
		{2005, true},
		{20000, false},
		{30000, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.version), func(t *testing.T) {
			var original bytes.Buffer
			writeMinimalPresetProperty(t, stream.NewBinaryWriter(&original), PresetPropertySignature, tt.version, "body", tt.value)
			prop, err := readPresetProperty(stream.NewBinaryReader(bytes.NewReader(original.Bytes())))
			if err != nil {
				t.Fatalf("readPresetProperty: %v", err)
			}
			if prop.IsCrcParts != tt.value {
				t.Fatalf("IsCrcParts = %v, want %v", prop.IsCrcParts, tt.value)
			}
			var encoded bytes.Buffer
			if err := writePresetProperty(stream.NewBinaryWriter(&encoded), prop); err != nil {
				t.Fatalf("writePresetProperty: %v", err)
			}
			if !bytes.Equal(encoded.Bytes(), original.Bytes()) {
				t.Fatal("MaidProp wire changed after round-trip")
			}
		})
	}
}

func TestPresetPropertyRejectsTempValueBeforeVersion101(t *testing.T) {
	prop := &PresetProperty{
		Signature: PresetPropertySignature,
		Version:   100,
		Name:      "body",
		TempValue: 1,
	}
	var encoded bytes.Buffer
	err := writePresetProperty(stream.NewBinaryWriter(&encoded), prop)
	if err == nil || !strings.Contains(err.Error(), "TempValue") {
		t.Fatalf("writePresetProperty error = %v", err)
	}
	if encoded.Len() != 0 {
		t.Fatalf("writePresetProperty wrote %d bytes before rejecting TempValue", encoded.Len())
	}
}

func TestPresetPropertyRejectsBoneLengthsBeforeVersion213(t *testing.T) {
	prop := &PresetProperty{
		Signature: PresetPropertySignature,
		Version:   212,
		Name:      "body",
		BoneLengths: map[int32]BoneLengthEntry{
			1: {Lengths: map[string]float32{"Bip01": 1}},
		},
	}
	var encoded bytes.Buffer
	err := writePresetProperty(stream.NewBinaryWriter(&encoded), prop)
	if err == nil || !strings.Contains(err.Error(), "BoneLengths") {
		t.Fatalf("writePresetProperty error = %v, want BoneLengths version error", err)
	}
	if encoded.Len() != 0 {
		t.Fatalf("writePresetProperty wrote %d bytes before rejecting BoneLengths", encoded.Len())
	}
}

func TestPresetPropertyListVersion4PreservesEmptyStoredKey(t *testing.T) {
	var original bytes.Buffer
	w := stream.NewBinaryWriter(&original)
	testWrite(t, w.WriteString(PresetPropertyListSignature))
	testWrite(t, w.WriteInt32(4))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString(""))
	writeMinimalPresetProperty(t, w, PresetPropertySignature, 100, "body", false)

	ppl, err := readPresetPropertyList(stream.NewBinaryReader(bytes.NewReader(original.Bytes())))
	if err != nil {
		t.Fatalf("readPresetPropertyList: %v", err)
	}
	if _, ok := ppl.PresetProperties[""]; !ok || len(ppl.PropertyOrder) != 1 || ppl.PropertyOrder[0] != "" {
		t.Fatalf("empty stored key was not preserved: %#v", ppl)
	}
	var encoded bytes.Buffer
	if err := dumpPresetPropertyList(stream.NewBinaryWriter(&encoded), ppl); err != nil {
		t.Fatalf("dumpPresetPropertyList: %v", err)
	}
	if !bytes.Equal(encoded.Bytes(), original.Bytes()) {
		t.Fatal("version 4 empty property key changed after round-trip")
	}
}

func TestPresetPropertyListBeforeVersion4RejectsUnstoredKey(t *testing.T) {
	ppl := &PresetPropertyList{
		Signature: PresetPropertyListSignature,
		Version:   3,
		PresetProperties: map[string]PresetProperty{
			"alias": {Signature: PresetPropertySignature, Version: 100, Name: "body"},
		},
	}
	var encoded bytes.Buffer
	err := dumpPresetPropertyList(stream.NewBinaryWriter(&encoded), ppl)
	if err == nil || !strings.Contains(err.Error(), "property key") {
		t.Fatalf("dumpPresetPropertyList error = %v", err)
	}
	if encoded.Len() != 0 {
		t.Fatalf("dumpPresetPropertyList wrote %d bytes before rejecting the unstored key", encoded.Len())
	}
}

func TestPresetPropertyListExtensionsRoundTrip(t *testing.T) {
	partsColorOther := &MultiColor{
		Signature: MultiColorSignature,
		Version:   1201,
		PartCount: 37,
		PartNames: []string{"EYE_L"},
		PartsColors: []PartsColor{{
			IsUse: true, MainHue: 10, MainChroma: 20, MainBrightness: 30, MainContrast: 40,
			ShadowRate: 50, ShadowHue: 60, ShadowChroma: 70, ShadowBrightness: 80, ShadowContrast: 90,
		}},
	}
	partsColorOtherWire, err := EncodeMultiColorBlock(partsColorOther)
	if err != nil {
		t.Fatalf("EncodeMultiColorBlock: %v", err)
	}
	crcOpaque, err := serializationKCES.NewKCESPreset()
	if err != nil {
		t.Fatalf("NewKCESPreset: %v", err)
	}
	crcOpaque.ContainerVersion = 812
	crcOpaque.MaidData.Version = 813
	crcPreset, err := serializationKCES.ExpandKCESPreset(crcOpaque)
	if err != nil {
		t.Fatalf("ExpandKCESPreset: %v", err)
	}
	crcPresetWire, err := serializationKCES.EncodeExpandedKCESPreset(crcPreset)
	if err != nil {
		t.Fatalf("EncodeExpandedKCESPreset: %v", err)
	}

	var original bytes.Buffer
	w := stream.NewBinaryWriter(&original)
	testWrite(t, w.WriteString(PresetPropertyListSignature))
	testWrite(t, w.WriteInt32(2002))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("body"))
	writeMinimalPresetProperty(t, w, PresetPropertySignature, 2005, "body", true)
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("other"))
	writeMinimalPresetProperty(t, w, PresetPropertySignature, 30000, "other", false)
	testWrite(t, w.WriteInt32(int32(len(partsColorOtherWire))))
	testWrite(t, w.WriteBytes(partsColorOtherWire))
	testWrite(t, w.WriteInt32(int32(len(crcPresetWire))))
	testWrite(t, w.WriteBytes(crcPresetWire))

	ppl, err := readPresetPropertyList(stream.NewBinaryReader(bytes.NewReader(original.Bytes())))
	if err != nil {
		t.Fatalf("readPresetPropertyList: %v", err)
	}
	if len(ppl.MaidPropOther) != 1 || ppl.MaidPropOther[0].Key != "other" || ppl.MaidPropOther[0].Property.Name != "other" {
		t.Fatalf("MaidPropOther = %#v", ppl.MaidPropOther)
	}
	if ppl.PartsColorOther == nil || ppl.PartsColorOther.Version != 1201 || len(ppl.PartsColorOther.PartNames) != 1 || ppl.PartsColorOther.PartNames[0] != "EYE_L" || ppl.PartsColorOther.PartsColors[0].ShadowContrast != 90 {
		t.Fatalf("PartsColorOther = %+v", ppl.PartsColorOther)
	}
	if ppl.CRCPreset == nil || ppl.CRCPreset.ContainerVersion != 812 || ppl.CRCPreset.MaidData.Version != 813 || ppl.CRCPreset.MaidData.PropData == nil {
		t.Fatalf("CRCPreset = %+v", ppl.CRCPreset)
	}
	jsonData, err := json.Marshal(ppl)
	if err != nil {
		t.Fatalf("marshal typed preset extensions: %v", err)
	}
	if bytes.Contains(jsonData, []byte("PartsColorOtherBin")) || bytes.Contains(jsonData, []byte("CRCPresetBin")) || bytes.Contains(jsonData, []byte(`"propData":"`)) {
		t.Fatalf("preset extension JSON still exposes known binary blocks: %s", jsonData)
	}
	for _, field := range []string{`"PartsColorOther":{`, `"CRCPreset":{`, `"propData":{`} {
		if !bytes.Contains(jsonData, []byte(field)) {
			t.Fatalf("typed preset extension JSON lacks %s: %s", field, jsonData)
		}
	}

	var encoded bytes.Buffer
	if err := dumpPresetPropertyList(stream.NewBinaryWriter(&encoded), ppl); err != nil {
		t.Fatalf("dumpPresetPropertyList: %v", err)
	}
	redecoded, err := readPresetPropertyList(stream.NewBinaryReader(bytes.NewReader(encoded.Bytes())))
	if err != nil {
		t.Fatalf("re-read typed MPROP_LIST extensions: %v", err)
	}
	if redecoded.PartsColorOther == nil || redecoded.PartsColorOther.Version != 1201 || len(redecoded.PartsColorOther.PartNames) != 1 || redecoded.PartsColorOther.PartNames[0] != "EYE_L" || redecoded.PartsColorOther.PartsColors[0].ShadowContrast != 90 {
		t.Fatalf("redecoded PartsColorOther = %+v", redecoded.PartsColorOther)
	}
	if redecoded.CRCPreset == nil || redecoded.CRCPreset.ContainerVersion != 812 || redecoded.CRCPreset.MaidData.Version != 813 || redecoded.CRCPreset.MaidData.PropData == nil || redecoded.CRCPreset.MaidData.ColorData == nil || redecoded.CRCPreset.MaidData.BodyData == nil {
		t.Fatalf("redecoded CRCPreset = %+v", redecoded.CRCPreset)
	}
}

func TestPresetPropertyListRejectsMalformedExtensionPayloads(t *testing.T) {
	validColor, err := EncodeMultiColorBlock(&MultiColor{
		Signature: MultiColorSignature,
		Version:   1201,
		PartCount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		colorData   []byte
		crcData     []byte
		wantInError string
	}{
		{name: "parts color", colorData: []byte{1, 2, 3}, wantInError: "PartsColorOtherBin"},
		{name: "CRC preset", colorData: validColor, crcData: []byte{0xff, 0x00}, wantInError: "CRCPresetBin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var wire bytes.Buffer
			w := stream.NewBinaryWriter(&wire)
			testWrite(t, w.WriteString(PresetPropertyListSignature))
			testWrite(t, w.WriteInt32(2002))
			testWrite(t, w.WriteInt32(0))
			testWrite(t, w.WriteInt32(0))
			testWrite(t, w.WriteInt32(int32(len(test.colorData))))
			testWrite(t, w.WriteBytes(test.colorData))
			testWrite(t, w.WriteInt32(int32(len(test.crcData))))
			testWrite(t, w.WriteBytes(test.crcData))
			_, err := readPresetPropertyList(stream.NewBinaryReader(bytes.NewReader(wire.Bytes())))
			if err == nil || !strings.Contains(err.Error(), test.wantInError) {
				t.Fatalf("readPresetPropertyList error = %v, want %q", err, test.wantInError)
			}
		})
	}
}

func TestPresetPropertyListRejectsInvalidExtensionLengths(t *testing.T) {
	for _, field := range []string{"MaidPropOther", "PartsColorOtherBin", "CRCPresetBin"} {
		for _, length := range []int32{-1, formerPresetCountLimit + 1} {
			t.Run(fmt.Sprintf("%s/%d", field, length), func(t *testing.T) {
				var buf bytes.Buffer
				w := stream.NewBinaryWriter(&buf)
				testWrite(t, w.WriteString(PresetPropertyListSignature))
				testWrite(t, w.WriteInt32(2002))
				testWrite(t, w.WriteInt32(0))
				switch field {
				case "MaidPropOther":
					testWrite(t, w.WriteInt32(length))
				case "PartsColorOtherBin":
					testWrite(t, w.WriteInt32(0))
					testWrite(t, w.WriteInt32(length))
				case "CRCPresetBin":
					testWrite(t, w.WriteInt32(0))
					testWrite(t, w.WriteInt32(0))
					testWrite(t, w.WriteInt32(length))
				}
				_, err := readPresetPropertyList(stream.NewBinaryReader(bytes.NewReader(buf.Bytes())))
				if err == nil || !strings.Contains(err.Error(), field) {
					t.Fatalf("readPresetPropertyList error = %v, want error containing %q", err, field)
				}
				if length > 0 && (strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "maximum")) {
					t.Fatalf("extension reader applied an arbitrary limit: %v", err)
				}
			})
		}
	}
}

func writeTestPositionRotationScale(t *testing.T, w *stream.BinaryWriter, start float32) {
	t.Helper()
	for i := 0; i < 10; i++ {
		testWrite(t, w.WriteFloat32(start+float32(i)))
	}
}

func TestPresetPropertyV2005SlotNamesAndNilSubPropsRoundTrip(t *testing.T) {
	var original bytes.Buffer
	w := stream.NewBinaryWriter(&original)
	writePresetPropertyPrefix(t, w, 2005)

	testWrite(t, w.WriteInt32(2))
	testWrite(t, w.WriteBool(false))
	testWrite(t, w.WriteBool(true))
	testWrite(t, w.WriteBool(true))
	testWrite(t, w.WriteString("sub.menu"))
	testWrite(t, w.WriteInt32(123))
	testWrite(t, w.WriteFloat32(0.75))

	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("head"))
	testWrite(t, w.WriteInt32(101))
	testWrite(t, w.WriteBool(true))
	writeTestPositionRotationScale(t, w, 1)

	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("head"))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("attach"))
	testWrite(t, w.WriteInt32(202))
	testWrite(t, w.WriteBool(true))
	testWrite(t, w.WriteInt32(3))
	testWrite(t, w.WriteInt32(2))
	writeTestPositionRotationScale(t, w, 11)

	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("head"))
	testWrite(t, w.WriteInt32(303))
	testWrite(t, w.WriteInt32(4))
	testWrite(t, w.WriteString("_Color"))
	testWrite(t, w.WriteString("Color"))
	testWrite(t, w.WriteString("1,2,3,4"))

	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("head"))
	testWrite(t, w.WriteInt32(404))
	testWrite(t, w.WriteInt32(1))
	testWrite(t, w.WriteString("hair_length"))
	testWrite(t, w.WriteFloat32(0.5))
	testWrite(t, w.WriteBool(true))

	prop, err := readPresetProperty(stream.NewBinaryReader(bytes.NewReader(original.Bytes())))
	if err != nil {
		t.Fatalf("readPresetProperty: %v", err)
	}
	if len(prop.SubProps) != 2 || prop.SubProps[0] != nil || prop.SubProps[1] == nil {
		t.Fatalf("SubProps = %#v", prop.SubProps)
	}
	if got := prop.SkinPositions[1].SlotName; got != "head" {
		t.Fatalf("SkinPositions SlotName = %q", got)
	}
	if got := prop.AttachPositionSlotNames[1]; got != "head" {
		t.Fatalf("AttachPositions SlotName = %q", got)
	}
	if got := prop.MaterialProps[1].SlotName; got != "head" {
		t.Fatalf("MaterialProps SlotName = %q", got)
	}
	if got := prop.BoneLengths[1].SlotName; got != "head" {
		t.Fatalf("BoneLengths SlotName = %q", got)
	}
	if !prop.IsCrcParts {
		t.Fatal("IsCrcParts = false, want true")
	}

	var encoded bytes.Buffer
	if err := writePresetProperty(stream.NewBinaryWriter(&encoded), prop); err != nil {
		t.Fatalf("writePresetProperty: %v", err)
	}
	if !bytes.Equal(encoded.Bytes(), original.Bytes()) {
		t.Fatal("v2005 MaidProp wire changed after round-trip")
	}
}

func TestPresetPropertyListPreservesMainPropertyOrder(t *testing.T) {
	var original bytes.Buffer
	w := stream.NewBinaryWriter(&original)
	testWrite(t, w.WriteString(PresetPropertyListSignature))
	testWrite(t, w.WriteInt32(4))
	testWrite(t, w.WriteInt32(2))
	for _, name := range []string{"z-first", "a-second"} {
		testWrite(t, w.WriteString(name))
		writeMinimalPresetProperty(t, w, PresetPropertySignature, 100, name, false)
	}

	ppl, err := readPresetPropertyList(stream.NewBinaryReader(bytes.NewReader(original.Bytes())))
	if err != nil {
		t.Fatalf("readPresetPropertyList: %v", err)
	}
	if got := strings.Join(ppl.PropertyOrder, ","); got != "z-first,a-second" {
		t.Fatalf("PropertyOrder = %q", got)
	}
	var encoded bytes.Buffer
	if err := dumpPresetPropertyList(stream.NewBinaryWriter(&encoded), ppl); err != nil {
		t.Fatalf("dumpPresetPropertyList: %v", err)
	}
	if !bytes.Equal(encoded.Bytes(), original.Bytes()) {
		t.Fatal("main property order changed after round-trip")
	}
}

func TestPresetPropertyPreservesNestedMapOrders(t *testing.T) {
	property := &PresetProperty{
		Signature:         PresetPropertySignature,
		Version:           30000,
		SkinPositions:     map[int32]BoneAttachPosEntry{1: {SlotName: "one"}, 2: {SlotName: "two"}},
		SkinPositionOrder: []int32{2, 1},
		AttachPositions: map[int32]map[string]VtxAttachPosEntry{
			3: {"a": {}, "z": {}},
			4: {},
		},
		AttachPositionOrder:      []int32{4, 3},
		AttachPositionNameOrders: map[int32][]string{4: {}, 3: {"z", "a"}},
		AttachPositionSlotNames:  map[int32]string{3: "three", 4: "four"},
		MaterialProps:            map[int32]MatPropSaveEntry{5: {SlotName: "five"}, 6: {SlotName: "six"}},
		MaterialPropOrder:        []int32{6, 5},
		BoneLengths: map[int32]BoneLengthEntry{
			7: {SlotName: "seven", Lengths: map[string]float32{"a": 1, "z": 2}, LengthOrder: []string{"z", "a"}},
			8: {SlotName: "eight", Lengths: map[string]float32{}},
		},
		BoneLengthOrder: []int32{8, 7},
	}
	if err := validatePresetPropertyForDump(property); err != nil {
		t.Fatalf("validatePresetPropertyForDump: %v", err)
	}

	var original bytes.Buffer
	if err := writePresetProperty(stream.NewBinaryWriter(&original), property); err != nil {
		t.Fatalf("writePresetProperty: %v", err)
	}
	decoded, err := readPresetProperty(stream.NewBinaryReader(bytes.NewReader(original.Bytes())))
	if err != nil {
		t.Fatalf("readPresetProperty: %v", err)
	}
	if fmt.Sprint(decoded.SkinPositionOrder) != "[2 1]" ||
		fmt.Sprint(decoded.AttachPositionOrder) != "[4 3]" ||
		strings.Join(decoded.AttachPositionNameOrders[3], ",") != "z,a" ||
		fmt.Sprint(decoded.MaterialPropOrder) != "[6 5]" ||
		fmt.Sprint(decoded.BoneLengthOrder) != "[8 7]" ||
		strings.Join(decoded.BoneLengths[7].LengthOrder, ",") != "z,a" {
		t.Fatalf("nested order metadata changed: %#v", decoded)
	}

	var reencoded bytes.Buffer
	if err := writePresetProperty(stream.NewBinaryWriter(&reencoded), decoded); err != nil {
		t.Fatalf("re-dump PresetProperty: %v", err)
	}
	if !bytes.Equal(reencoded.Bytes(), original.Bytes()) {
		t.Fatal("nested PresetProperty map order changed after round-trip")
	}
}

func TestPresetOrderMetadataToleratesMapEdits(t *testing.T) {
	ppl := &PresetPropertyList{
		PresetProperties: map[string]PresetProperty{"a": {}, "b": {}, "c": {}},
		PropertyOrder:    []string{"b", "removed"},
	}
	keys, err := orderedPresetPropertyKeys(ppl)
	if err != nil {
		t.Fatalf("orderedPresetPropertyKeys: %v", err)
	}
	if got := strings.Join(keys, ","); got != "b,a,c" {
		t.Fatalf("property keys = %q, want b,a,c", got)
	}

	intKeys, err := orderedPresetIntMapKeys(map[int32]struct{}{1: {}, 2: {}, 3: {}}, []int32{2, 99}, "test ints")
	if err != nil {
		t.Fatalf("orderedPresetIntMapKeys: %v", err)
	}
	if got := fmt.Sprint(intKeys); got != "[2 1 3]" {
		t.Fatalf("int keys = %s, want [2 1 3]", got)
	}

	stringKeys, err := orderedPresetStringMapKeys(map[string]struct{}{"a": {}, "m": {}, "z": {}}, []string{"z", "removed"}, "test strings")
	if err != nil {
		t.Fatalf("orderedPresetStringMapKeys: %v", err)
	}
	if got := strings.Join(stringKeys, ","); got != "z,a,m" {
		t.Fatalf("string keys = %q, want z,a,m", got)
	}

	if _, err := orderedPresetStringMapKeys(map[string]struct{}{"live": {}}, []string{"live", "live"}, "test duplicate"); err == nil {
		t.Fatal("duplicate live order key was accepted")
	}
}

func TestPresetDumpValidatesBeforeWriting(t *testing.T) {
	valid := &Preset{
		Signature:  PresetSignature,
		Version:    1,
		PresetType: PresetTypeAll,
		PresetPropertyList: &PresetPropertyList{
			Signature:        PresetPropertyListSignature,
			Version:          1,
			PresetProperties: map[string]PresetProperty{},
		},
	}
	valid.Signature = "NOT_A_PRESET"
	var buf bytes.Buffer
	err := valid.Dump(&buf)
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("Dump error = %v, want signature error", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("Dump wrote %d bytes before validation failed", buf.Len())
	}
}

func TestPresetDumpRecomputesDerivedLengths(t *testing.T) {
	value := &Preset{
		Signature:   PresetSignature,
		Version:     1,
		PresetType:  PresetTypeAll,
		ThumbLength: -1,
		ThumbData:   []byte{1, 2, 3},
		PresetPropertyList: &PresetPropertyList{
			Signature:        PresetPropertyListSignature,
			Version:          1,
			PropertyCount:    99,
			PresetProperties: map[string]PresetProperty{},
		},
	}
	var wire bytes.Buffer
	if err := value.Dump(&wire); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if value.ThumbLength != 3 || value.PresetPropertyList.PropertyCount != 0 {
		t.Fatalf("derived fields were not updated: thumb=%d properties=%d", value.ThumbLength, value.PresetPropertyList.PropertyCount)
	}
	decoded, err := ReadPreset(bytes.NewReader(wire.Bytes()))
	if err != nil {
		t.Fatalf("ReadPreset: %v", err)
	}
	if decoded.ThumbLength != 3 || decoded.PresetPropertyList.PropertyCount != 0 {
		t.Fatalf("stored derived fields are wrong: %#v", decoded)
	}
}
