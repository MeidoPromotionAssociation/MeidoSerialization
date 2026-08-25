package KCES

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
)

func TestPartDecoderReportsCorruptLz4InsteadOfTreatingItAsRawMsgpack(t *testing.T) {
	encoded, err := EncodeMenuAssets(&MenuAssets{Assets: []*Menu{{Version: menuFixVersion}}})
	if err != nil {
		t.Fatalf("EncodeMenuAssets: %v", err)
	}
	if len(encoded) < 2 {
		t.Fatalf("encoded payload is unexpectedly short: %d", len(encoded))
	}
	_, err = DecodeMenuAssets(encoded[:len(encoded)-1])
	if err == nil {
		t.Fatal("truncated LZ4 payload unexpectedly decoded")
	}
	if !strings.Contains(err.Error(), "decompress MenuAssets") {
		t.Fatalf("error = %q, want decompression context", err)
	}
	if bytes.Contains([]byte(err.Error()), []byte("only encoded map or array")) {
		t.Fatalf("corrupt compression was incorrectly retried as raw MessagePack: %v", err)
	}
}

func TestPartEncodersEncodeTypedNilRootsWithoutPanic(t *testing.T) {
	tests := []struct {
		name   string
		encode func() ([]byte, error)
	}{
		{name: "Model", encode: func() ([]byte, error) { return EncodeModel(nil) }},
		{name: "ModelAssets", encode: func() ([]byte, error) { return EncodeModelAssets(nil) }},
		{name: "MaterialAssets", encode: func() ([]byte, error) { return EncodeMaterialAssets(nil) }},
		{name: "MenuAssets", encode: func() ([]byte, error) { return EncodeMenuAssets(nil) }},
		{name: "PriorityMaterialAssets", encode: func() ([]byte, error) { return EncodePriorityMaterialAssets(nil) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.encode()
			if err != nil {
				t.Fatalf("typed nil input was rejected: %v", err)
			}
			if len(data) == 0 {
				t.Fatal("typed nil input returned no MessagePack payload")
			}
		})
	}

	// Keep the existing raw-array helper API while making its nil behavior safe.
	if got := EncodePriorityMaterial(nil); got != nil {
		t.Fatalf("EncodePriorityMaterial(nil) = %#v, want nil", got)
	}
}

func TestEncodeMenuAssetsPreservesNestedVersionsAndOpaqueStrings(t *testing.T) {
	input := &MenuAssets{
		Assets: []*Menu{{
			Version:       -5,
			CategoryText:  protocolTestString("not_an_mpn"),
			ColorSetText:  protocolTestString("Hairf"),
			ToeLockSlotId: protocolTestString("not_a_slot"),
			PreMulTexDatas: map[uint64]*PreMulTexDatas{
				42: {Version: -4, SlotID: protocolTestString("future_slot"), PreTexCompoTypeStr: protocolTestString("future-material")},
			},
			ColvariInfo: &Colvari{
				Version:      -3,
				ColvariDatas: []*ColvariData{{Version: -2, MPN: protocolTestString("future|mpn")}},
			},
		}},
	}
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeMenuAssets(input)
	if err != nil {
		t.Fatalf("EncodeMenuAssets: %v", err)
	}
	decoded, err := DecodeMenuAssets(encoded)
	if err != nil {
		t.Fatalf("DecodeMenuAssets: %v", err)
	}
	if len(decoded.Assets) != 1 {
		t.Fatalf("decoded asset count = %d, want 1", len(decoded.Assets))
	}
	menu := decoded.Assets[0]
	if menu.Version != -5 {
		t.Fatalf("menu version = %d, want -5", menu.Version)
	}
	if got := menu.PreMulTexDatas[42]; got.Version != -4 || !protocolTestStringEqual(got.SlotID, "future_slot") ||
		!protocolTestStringEqual(got.PreTexCompoTypeStr, "future-material") {
		t.Fatalf("PreMulTexDatas changed: %+v", got)
	}
	if menu.ColvariInfo == nil || menu.ColvariInfo.Version != -3 {
		t.Fatalf("Colvari version changed: %+v", menu.ColvariInfo)
	}
	if len(menu.ColvariInfo.ColvariDatas) != 1 || menu.ColvariInfo.ColvariDatas[0].Version != -2 ||
		!protocolTestStringEqual(menu.ColvariInfo.ColvariDatas[0].MPN, "future|mpn") {
		t.Fatalf("ColvariData changed: %+v", menu.ColvariInfo.ColvariDatas)
	}
	if !protocolTestStringEqual(menu.CategoryText, "not_an_mpn") || !protocolTestStringEqual(menu.ColorSetText, "Hairf") ||
		!protocolTestStringEqual(menu.ToeLockSlotId, "not_a_slot") {
		t.Fatalf("opaque menu strings changed: %+v", menu)
	}
	after, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("EncodeMenuAssets mutated input: got %s, want %s", after, before)
	}
}

func TestMenuAssetsPreTexCompoTypeIsOpaqueWireString(t *testing.T) {
	values := []string{"", " ", "Alpha", "infinitycolorgrada", "Alpha, Screen", "0", "2147483648", "not-a-material", "Alpha,"}
	for _, value := range values {
		input := &MenuAssets{Assets: []*Menu{{PreMulTexDatas: map[uint64]*PreMulTexDatas{1: {PreTexCompoTypeStr: protocolTestString(value)}}}}}
		encoded, err := EncodeMenuAssets(input)
		if err != nil {
			t.Fatalf("EncodeMenuAssets(%q): %v", value, err)
		}
		decoded, err := DecodeMenuAssets(encoded)
		if err != nil {
			t.Fatalf("DecodeMenuAssets(%q): %v", value, err)
		}
		if got := decoded.Assets[0].PreMulTexDatas[1].PreTexCompoTypeStr; !protocolTestStringEqual(got, value) {
			t.Fatalf("preTexCompoTypeStr round trip = %v, want %q", got, value)
		}
	}
}

func TestMenuAssetsMPNStringsAreOpaqueWireStrings(t *testing.T) {
	values := []string{"", " ", "body", "BODY", "-1", "Hara, KubiScl", "not_an_mpn", "2147483648"}
	for _, value := range values {
		assets := &MenuAssets{Assets: []*Menu{{CategoryText: protocolTestString(value), ColorSetText: protocolTestString(value)}}}
		wire, err := EncodeMenuAssets(assets)
		if err != nil {
			t.Fatalf("EncodeMenuAssets(%q): %v", value, err)
		}
		decoded, err := DecodeMenuAssets(wire)
		if err != nil {
			t.Fatalf("DecodeMenuAssets(%q): %v", value, err)
		}
		if !protocolTestStringEqual(decoded.Assets[0].CategoryText, value) || !protocolTestStringEqual(decoded.Assets[0].ColorSetText, value) {
			t.Fatalf("MPN strings changed: %+v, want %q", decoded.Assets[0], value)
		}
	}
}

func TestMenuAssetsToeLockSlotIsOpaqueWireString(t *testing.T) {
	values := []string{"", "shoes", "SHOES", " accAcc72 ", "-1", "2147483647", "shoes, accAcc1", "not_a_slot", "accAcc0", "2147483648", "shoes,1"}
	for _, value := range values {
		assets := &MenuAssets{Assets: []*Menu{{ToeLockSlotId: protocolTestString(value)}}}
		wire, err := EncodeMenuAssets(assets)
		if err != nil {
			t.Fatalf("EncodeMenuAssets(%q): %v", value, err)
		}
		decoded, err := DecodeMenuAssets(wire)
		if err != nil {
			t.Fatalf("DecodeMenuAssets(%q): %v", value, err)
		}
		if !protocolTestStringEqual(decoded.Assets[0].ToeLockSlotId, value) {
			t.Fatalf("toeLockSlotId round trip = %v, want %q", decoded.Assets[0].ToeLockSlotId, value)
		}
	}
}

func TestPreMulTexDatasRejectsWrongIndexedArrayWidth(t *testing.T) {
	raw := []interface{}{
		int64(1000), // version
		"body",      // slotId
		"",          // saveTag
		int64(0),    // f_nMatNo
		"_MainTex",  // f_strPropName
		int64(0),    // f_nLayerNo
		"source",    // f_strFileName
		"",          // f_eBlendMode
		nil,         // maskParam
		nil,         // infColParam
		false,       // f_bTexGroup
		int64(4),    // f_nLayNoInGroup explicitly present
		float64(0),  // f_fAlpha explicitly present
		int64(0),    // f_nTargetBodyTexSize
		"",          // posDefHokuroTatooSlotId
		nil,         // preMaskData
		nil,         // preTransTexData
		nil,         // preInfColData; Key(18) is intentionally absent
	}
	for _, width := range []int{11, 17} {
		wire, err := msgpack.EncodeMsgpack(raw[:width])
		if err != nil {
			t.Fatalf("EncodeMsgpack: %v", err)
		}
		var decoded PreMulTexDatas
		if err := msgpack.DecodeMsgpack(wire, &decoded); err == nil {
			t.Fatalf("DecodeMsgpack accepted %d-slot PreMulTexDatas", width)
		}
	}
	raw = append(raw, "Alpha", int64(1))
	wire, err := msgpack.EncodeMsgpack(raw)
	if err != nil {
		t.Fatal(err)
	}
	var decoded PreMulTexDatas
	if err := msgpack.DecodeMsgpack(wire, &decoded); err == nil {
		t.Fatal("DecodeMsgpack accepted high PreMulTexDatas key")
	}
}

func TestPreMulTexDatasAcceptsHistoricalWidth18WithConstructorDefault(t *testing.T) {
	// Width 18 predates preTexCompoTypeStr at Key 18 and remains in base-game parts.menuassets
	// MessagePack-CSharp constructs the object before assigning available keys, so the missing field keeps "Alpha"
	raw := []interface{}{
		int64(1000), // version
		"body",      // slotId
		"",          // saveTag
		int64(0),    // f_nMatNo
		"_MainTex",  // f_strPropName
		int64(0),    // f_nLayerNo
		"source",    // f_strFileName
		"",          // f_eBlendMode
		nil,         // maskParam
		nil,         // infColParam
		false,       // f_bTexGroup
		int64(4),    // f_nLayNoInGroup explicitly present
		float64(0),  // f_fAlpha explicitly present
		int64(0),    // f_nTargetBodyTexSize
		"",          // posDefHokuroTatooSlotId
		nil,         // preMaskData
		nil,         // preTransTexData
		nil,         // preInfColData; Key(18) is intentionally absent
	}
	wire, err := msgpack.EncodeMsgpack(raw)
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}
	var decoded PreMulTexDatas
	if err := msgpack.DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatalf("DecodeMsgpack rejected historical 18-slot PreMulTexDatas: %v", err)
	}
	if decoded.PreTexCompoTypeStr == nil || *decoded.PreTexCompoTypeStr != "Alpha" {
		t.Fatalf("preTexCompoTypeStr = %v, want constructor default %q", decoded.PreTexCompoTypeStr, "Alpha")
	}
	if decoded.LayNoInGroup != 4 {
		t.Fatalf("f_nLayNoInGroup = %d, want wire value 4 overriding constructor default -1", decoded.LayNoInGroup)
	}
	if decoded.Alpha != 0 {
		t.Fatalf("f_fAlpha = %v, want wire value 0 overriding constructor default 1", decoded.Alpha)
	}
	if decoded.Version != 1000 {
		t.Fatalf("version = %d, want wire value 1000 overriding constructor default 1001", decoded.Version)
	}

	currentWire, err := msgpack.EncodeMsgpack(append(append([]interface{}(nil), raw...), nil))
	if err != nil {
		t.Fatalf("encode current layout with explicit nil: %v", err)
	}
	var current PreMulTexDatas
	if err := msgpack.DecodeMsgpack(currentWire, &current); err != nil {
		t.Fatalf("decode current layout with explicit nil: %v", err)
	}
	if current.PreTexCompoTypeStr != nil {
		t.Fatalf("explicit nil preTexCompoTypeStr became constructor default %v", current.PreTexCompoTypeStr)
	}
}

func TestMaskParamAcceptsHistoricalWidthFive(t *testing.T) {
	raw := []interface{}{
		nil,
		"mask.png",
		[]interface{}{},
		"linked-mask",
		int64(3),
	}
	wire, err := msgpack.EncodeMsgpack(raw)
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}
	var decoded MaskParam
	if err := msgpack.DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatalf("DecodeMsgpack rejected historical five-slot MaskParam: %v", err)
	}
	if decoded.MaskTexName == nil || *decoded.MaskTexName != "mask.png" ||
		decoded.LinkMaskName == nil || *decoded.LinkMaskName != "linked-mask" || decoded.LinkMaskNo != 3 {
		t.Fatalf("decoded historical MaskParam = %#v", decoded)
	}
	if decoded.ShareRtTargetPart != nil {
		t.Fatalf("shareRtTargetPart = %v, want nil for the historical layout", decoded.ShareRtTargetPart)
	}

	reencoded, err := msgpack.EncodeIndexedMsgpack(&decoded)
	if err != nil {
		t.Fatalf("EncodeIndexedMsgpack: %v", err)
	}
	var slots []interface{}
	if err := msgpack.DecodeMsgpack(reencoded, &slots); err != nil {
		t.Fatalf("decode re-encoded MaskParam slots: %v", err)
	}
	if len(slots) != 6 || slots[5] != nil {
		t.Fatalf("re-encoded MaskParam slots = %#v, want six slots ending in nil", slots)
	}
}

func TestColliderEncodingValidationRejectsInvalidObjectsWithoutPanic(t *testing.T) {
	var typedNilSphere *ColliderSphere
	tests := []struct {
		name    string
		pkg     *ColliderPackage
		wantErr string
	}{
		{
			name: "typed nil collider",
			pkg: &ColliderPackage{
				Colliders: []*ColliderRef{{Type: ColliderTypeSphere, Collider: typedNilSphere}},
			},
			wantErr: "is nil (*ColliderSphere)",
		},
		{
			name: "union tag mismatch",
			pkg: &ColliderPackage{
				Colliders: []*ColliderRef{{Type: ColliderTypePlane, Collider: &ColliderCapsule{}}},
			},
			wantErr: "concrete type requires 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EncodeKCESPayload(tc.pkg, ".dbcol")
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestLimbColliderDecodeRequiresDeclaredMaidPropWidth(t *testing.T) {
	short := []interface{}{
		int64(limbColliderPackageFixVersion),
		[]interface{}{
			[]interface{}{int64(limbColliderItemFixVersion), int64(0), colliderCapsuleIndexedTestValue(colliderStatusFixVersion)},
		},
	}
	if _, err := DecodeKCESPayload(lengthPrefixedIndexedTestValue(t, short), ".limbcol"); err == nil {
		t.Fatal("DecodeKCESPayload accepted a short NativeMaidPropColliderStatus")
	}

	full := []interface{}{
		int64(limbColliderPackageFixVersion),
		[]interface{}{
			[]interface{}{int64(limbColliderItemFixVersion), int64(0), colliderMaidPropIndexedTestValue(colliderStatusFixVersion)},
		},
	}
	root, err := DecodeKCESPayload(lengthPrefixedIndexedTestValue(t, full), ".limbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload full MaidProp: %v", err)
	}
	if root.(*LimbColliderPackage).Items[0].Collider == nil {
		t.Fatal("full NativeMaidPropColliderStatus decoded as nil")
	}

	var item LimbColliderItem
	if err := json.Unmarshal([]byte(`{"version":1000,"target":0,"collider":null}`), &item); err != nil {
		t.Fatalf("null concrete reference should remain serializable: %v", err)
	}
	if item.Collider != nil {
		t.Fatalf("null limb collider became %T", item.Collider)
	}
}

func TestColliderEncodingPreservesZeroVersions(t *testing.T) {
	t.Run("generic package", func(t *testing.T) {
		pkg := &ColliderPackage{
			Colliders:      []*ColliderRef{{Type: ColliderTypePlane, Collider: &ColliderPlane{}}},
			LimbEnableList: []*ColliderState{{LimbType: 1, IsEnable: true}},
		}
		encoded, err := EncodeKCESPayload(pkg, ".dbcol")
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		root, err := DecodeKCESPayload(encoded, ".dbcol")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		decoded := root.(*ColliderPackage)
		plane, ok := decoded.Colliders[0].Collider.(*ColliderPlane)
		if !ok {
			t.Fatalf("decoded collider type = %T", decoded.Colliders[0].Collider)
		}
		if decoded.Version != 0 || plane.Version != 0 || decoded.LimbEnableList[0].Version != 0 {
			t.Fatalf("versions changed: package=%d collider=%d state=%d",
				decoded.Version, plane.Version, decoded.LimbEnableList[0].Version)
		}
		if pkg.Version != 0 || pkg.Colliders[0].Collider.(*ColliderPlane).Version != 0 ||
			pkg.LimbEnableList[0].Version != 0 {
			t.Fatalf("EncodeKCESPayload mutated generic collider input: %+v", pkg)
		}
	})

	t.Run("limb package", func(t *testing.T) {
		pkg := &LimbColliderPackage{
			Items: []*LimbColliderItem{{Collider: &ColliderMaidProp{}}},
		}
		encoded, err := EncodeKCESPayload(pkg, ".limbcol")
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		root, err := DecodeKCESPayload(encoded, ".limbcol")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		decoded := root.(*LimbColliderPackage)
		maidProp := decoded.Items[0].Collider
		if decoded.Version != 0 || decoded.Items[0].Version != 0 || maidProp.Version != 0 {
			t.Fatalf("versions changed: package=%d item=%d collider=%d",
				decoded.Version, decoded.Items[0].Version, maidProp.Version)
		}
		if pkg.Version != 0 || pkg.Items[0].Version != 0 || pkg.Items[0].Collider.Version != 0 {
			t.Fatalf("EncodeKCESPayload mutated limb collider input: %+v", pkg)
		}
	})

	t.Run("IK package", func(t *testing.T) {
		pkg := &IKColliderPackage{
			Groups: []*IKColliderGroup{{
				Colliders: []*ColliderRef{{Type: ColliderTypeSphere, Collider: &ColliderSphere{}}},
			}},
		}
		encoded, err := EncodeKCESPayload(pkg, ".ikcol")
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		root, err := DecodeKCESPayload(encoded, ".ikcol")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		decoded := root.(*IKColliderPackage)
		sphere, ok := decoded.Groups[0].Colliders[0].Collider.(*ColliderSphere)
		if !ok {
			t.Fatalf("decoded collider type = %T", decoded.Groups[0].Colliders[0].Collider)
		}
		if decoded.Version != 0 || decoded.Groups[0].Version != 0 || sphere.Version != 0 {
			t.Fatalf("versions changed: package=%d group=%d collider=%d",
				decoded.Version, decoded.Groups[0].Version, sphere.Version)
		}
		if pkg.Version != 0 || pkg.Groups[0].Version != 0 ||
			pkg.Groups[0].Colliders[0].Collider.(*ColliderSphere).Version != 0 {
			t.Fatalf("EncodeKCESPayload mutated IK collider input: %+v", pkg)
		}
	})
}

func TestMaidPropEncodingPreservesMPNFields(t *testing.T) {
	maidProp := &ColliderMaidProp{
		ColliderObject:         ColliderObject{Version: 1002},
		CenterMpnList:          []int32{7, -1},
		StartRadiusMpnList:     []int32{25},
		EndRadiusMpnList:       []int32{49},
		CenterMpnNameList:      []*string{protocolTestString("stale-center")},
		StartRadiusMpnNameList: []*string{protocolTestString("stale-start")},
		EndRadiusMpnNameList:   []*string{protocolTestString("stale-end")},
	}
	pkg := &LimbColliderPackage{
		Items: []*LimbColliderItem{{Collider: maidProp}},
	}

	encoded, err := EncodeKCESPayload(pkg, ".limbcol")
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	root, err := DecodeKCESPayload(encoded, ".limbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	got := root.(*LimbColliderPackage).Items[0].Collider
	if got.Version != 1002 {
		t.Fatalf("decoded version = %d, want 1002", got.Version)
	}
	if want := []*string{protocolTestString("stale-center")}; !reflect.DeepEqual(got.CenterMpnNameList, want) {
		t.Fatalf("center names = %#v, want %#v", got.CenterMpnNameList, want)
	}
	if want := []*string{protocolTestString("stale-start")}; !reflect.DeepEqual(got.StartRadiusMpnNameList, want) {
		t.Fatalf("start names = %#v, want %#v", got.StartRadiusMpnNameList, want)
	}
	if want := []*string{protocolTestString("stale-end")}; !reflect.DeepEqual(got.EndRadiusMpnNameList, want) {
		t.Fatalf("end names = %#v, want %#v", got.EndRadiusMpnNameList, want)
	}
	if !reflect.DeepEqual(got.CenterMpnList, maidProp.CenterMpnList) ||
		!reflect.DeepEqual(got.StartRadiusMpnList, maidProp.StartRadiusMpnList) ||
		!reflect.DeepEqual(got.EndRadiusMpnList, maidProp.EndRadiusMpnList) {
		t.Fatalf("numeric MPN lists changed: %+v", got)
	}

	if maidProp.Version != 1002 ||
		!reflect.DeepEqual(maidProp.CenterMpnList, []int32{7, -1}) ||
		!reflect.DeepEqual(maidProp.CenterMpnNameList, []*string{protocolTestString("stale-center")}) ||
		!reflect.DeepEqual(maidProp.StartRadiusMpnNameList, []*string{protocolTestString("stale-start")}) ||
		!reflect.DeepEqual(maidProp.EndRadiusMpnNameList, []*string{protocolTestString("stale-end")}) {
		t.Fatalf("encoding mutated caller-owned MaidProp data: %+v", maidProp)
	}
}

func TestMaidPropDecodeDoesNotMigrateVersionOrMPNRepresentations(t *testing.T) {
	legacy := &ColliderMaidProp{
		ColliderObject: ColliderObject{
			Version:       1001,
			LocalRotation: Vector4{W: 1},
			LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
		},
		CenterMpnList:      []int32{9},
		StartRadiusMpnList: []int32{},
		EndRadiusMpnList:   []int32{},
	}
	legacyWire, err := msgpack.EncodeIndexedMsgpack(legacy)
	if err != nil {
		t.Fatalf("encode legacy MaidProp: %v", err)
	}
	var legacyResult ColliderMaidProp
	err = msgpack.DecodeMsgpack(legacyWire, &legacyResult)
	if err != nil {
		t.Fatalf("decode legacy MaidProp: %v", err)
	}
	if legacyResult.Version != 1001 || !reflect.DeepEqual(legacyResult.CenterMpnList, []int32{9}) ||
		legacyResult.CenterMpnNameList != nil {
		t.Fatalf("legacy fields were migrated: %+v", legacyResult)
	}

	current := &ColliderMaidProp{
		ColliderObject: ColliderObject{
			Version:       colliderMaidPropFixVersion,
			LocalRotation: Vector4{W: 1},
			LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
		},
		CenterMpnList:          []int32{7},
		StartRadiusMpnList:     []int32{25},
		EndRadiusMpnList:       []int32{26},
		CenterMpnNameList:      []*string{protocolTestString("MuneUpDown")},
		StartRadiusMpnNameList: []*string{protocolTestString("stale")},
		EndRadiusMpnNameList:   []*string{protocolTestString("stale")},
	}
	currentWire, err := msgpack.EncodeIndexedMsgpack(current)
	if err != nil {
		t.Fatalf("encode current MaidProp: %v", err)
	}
	var currentResult ColliderMaidProp
	err = msgpack.DecodeMsgpack(currentWire, &currentResult)
	if err != nil {
		t.Fatalf("decode current MaidProp: %v", err)
	}
	if !reflect.DeepEqual(currentResult.CenterMpnList, []int32{7}) ||
		!reflect.DeepEqual(currentResult.StartRadiusMpnList, []int32{25}) ||
		!reflect.DeepEqual(currentResult.EndRadiusMpnList, []int32{26}) ||
		!reflect.DeepEqual(currentResult.CenterMpnNameList, []*string{protocolTestString("MuneUpDown")}) ||
		!reflect.DeepEqual(currentResult.StartRadiusMpnNameList, []*string{protocolTestString("stale")}) ||
		!reflect.DeepEqual(currentResult.EndRadiusMpnNameList, []*string{protocolTestString("stale")}) {
		t.Fatalf("current numeric/name fields changed: %+v", currentResult)
	}
}

func TestMaidPropDecodeAcceptsNilListsAndOpaqueNames(t *testing.T) {
	base := &ColliderMaidProp{
		ColliderObject: ColliderObject{
			Version:       colliderMaidPropFixVersion,
			LocalRotation: Vector4{W: 1},
			LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
		},
		CenterMpnList:          []int32{},
		StartRadiusMpnList:     []int32{},
		EndRadiusMpnList:       []int32{},
		CenterMpnNameList:      []*string{protocolTestString("MuneL")},
		StartRadiusMpnNameList: []*string{},
		EndRadiusMpnNameList:   []*string{},
	}

	raw := colliderMaidPropIndexedTestValue(base.Version)
	raw[16] = nil
	raw[22] = []interface{}{"not_an_mpn"}
	wire, err := msgpack.EncodeMsgpack(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got ColliderMaidProp
	err = msgpack.DecodeMsgpack(wire, &got)
	if err != nil {
		t.Fatalf("decode nil/opaque MaidProp fields: %v", err)
	}
	if got.CenterMpnList != nil || !reflect.DeepEqual(got.CenterMpnNameList, []*string{protocolTestString("not_an_mpn")}) {
		t.Fatalf("nil/opaque fields changed: %+v", got)
	}

	malformed := colliderMaidPropIndexedTestValue(base.Version)
	malformed[22] = []interface{}{int64(7)}
	malformedWire, err := msgpack.EncodeMsgpack(malformed)
	if err != nil {
		t.Fatal(err)
	}
	if err := msgpack.DecodeMsgpack(malformedWire, &got); err == nil {
		t.Fatal("non-string List<string> element was accepted")
	}
}

func protocolTestString(value string) *string { return &value }

func protocolTestStringEqual(value *string, want string) bool {
	return value != nil && *value == want
}
