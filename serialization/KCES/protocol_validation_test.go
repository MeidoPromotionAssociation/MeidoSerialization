package KCES

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestPartDecoderReportsCorruptLz4InsteadOfTreatingItAsRawMsgpack(t *testing.T) {
	encoded, err := EncodeMenuAssets(&MenuAssets{Assets: []Menu{{Version: menuFixVersion}}})
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

func TestPartEncodersHandleNilWithoutPanic(t *testing.T) {
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
			if err == nil {
				t.Fatalf("expected an error for nil input, got data length %d", len(data))
			}
			if data != nil {
				t.Fatalf("nil input returned non-nil data: %x", data)
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
		Assets: []Menu{{
			Version:       -5,
			CategoryText:  "not_an_mpn",
			ColorSetText:  "Hairf",
			ToeLockSlotId: "not_a_slot",
			PreMulTexDatas: map[uint64]PreMulTexDatas{
				42: {Version: -4, SlotID: "future_slot", PreTexCompoTypeStr: "future-material"},
			},
			ColvariInfo: &Colvari{
				Version:      -3,
				ColvariDatas: []ColvariData{{Version: -2, MPN: "future|mpn"}},
			},
		}},
	}
	before := *input
	before.Assets = append([]Menu(nil), input.Assets...)

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
	if got := menu.PreMulTexDatas[42]; got.Version != -4 || got.SlotID != "future_slot" || got.PreTexCompoTypeStr != "future-material" {
		t.Fatalf("PreMulTexDatas changed: %+v", got)
	}
	if menu.ColvariInfo == nil || menu.ColvariInfo.Version != -3 {
		t.Fatalf("Colvari version changed: %+v", menu.ColvariInfo)
	}
	if len(menu.ColvariInfo.ColvariDatas) != 1 || menu.ColvariInfo.ColvariDatas[0].Version != -2 || menu.ColvariInfo.ColvariDatas[0].MPN != "future|mpn" {
		t.Fatalf("ColvariData changed: %+v", menu.ColvariInfo.ColvariDatas)
	}
	if menu.CategoryText != "not_an_mpn" || menu.ColorSetText != "Hairf" || menu.ToeLockSlotId != "not_a_slot" {
		t.Fatalf("opaque menu strings changed: %+v", menu)
	}
	if !reflect.DeepEqual(*input, before) {
		t.Fatalf("EncodeMenuAssets mutated input: got %#v, want %#v", input, before)
	}
}

func TestMenuAssetsPreTexCompoTypeIsOpaqueWireString(t *testing.T) {
	values := []string{"", " ", "Alpha", "infinitycolorgrada", "Alpha, Screen", "0", "2147483648", "not-a-material", "Alpha,"}
	for _, value := range values {
		input := &MenuAssets{Assets: []Menu{{PreMulTexDatas: map[uint64]PreMulTexDatas{1: {PreTexCompoTypeStr: value}}}}}
		encoded, err := EncodeMenuAssets(input)
		if err != nil {
			t.Fatalf("EncodeMenuAssets(%q): %v", value, err)
		}
		decoded, err := DecodeMenuAssets(encoded)
		if err != nil {
			t.Fatalf("DecodeMenuAssets(%q): %v", value, err)
		}
		if got := decoded.Assets[0].PreMulTexDatas[1].PreTexCompoTypeStr; got != value {
			t.Fatalf("preTexCompoTypeStr round trip = %q, want %q", got, value)
		}
	}
}

func TestMenuAssetsMPNStringsAreOpaqueWireStrings(t *testing.T) {
	values := []string{"", " ", "body", "BODY", "-1", "Hara, KubiScl", "not_an_mpn", "2147483648"}
	for _, value := range values {
		assets := &MenuAssets{Assets: []Menu{{CategoryText: value, ColorSetText: value}}}
		wire, err := EncodeMenuAssets(assets)
		if err != nil {
			t.Fatalf("EncodeMenuAssets(%q): %v", value, err)
		}
		decoded, err := DecodeMenuAssets(wire)
		if err != nil {
			t.Fatalf("DecodeMenuAssets(%q): %v", value, err)
		}
		if decoded.Assets[0].CategoryText != value || decoded.Assets[0].ColorSetText != value {
			t.Fatalf("MPN strings changed: %+v, want %q", decoded.Assets[0], value)
		}
	}
}

func TestMenuAssetsToeLockSlotIsOpaqueWireString(t *testing.T) {
	values := []string{"", "shoes", "SHOES", " accAcc72 ", "-1", "2147483647", "shoes, accAcc1", "not_a_slot", "accAcc0", "2147483648", "shoes,1"}
	for _, value := range values {
		assets := &MenuAssets{Assets: []Menu{{ToeLockSlotId: value}}}
		wire, err := EncodeMenuAssets(assets)
		if err != nil {
			t.Fatalf("EncodeMenuAssets(%q): %v", value, err)
		}
		decoded, err := DecodeMenuAssets(wire)
		if err != nil {
			t.Fatalf("DecodeMenuAssets(%q): %v", value, err)
		}
		if decoded.Assets[0].ToeLockSlotId != value {
			t.Fatalf("toeLockSlotId round trip = %q, want %q", decoded.Assets[0].ToeLockSlotId, value)
		}
	}
}

func TestPreMulTexDatasShortArrayKeepsOnlyPresentFields(t *testing.T) {
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
	wire, err := ct.EncodeMsgpack(raw)
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}
	var decoded PreMulTexDatas
	if err := ct.DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatalf("DecodeMsgpack(short PreMulTexDatas): %v", err)
	}
	if decoded.Version != 1000 || decoded.LayNoInGroup != 4 || decoded.Alpha != 0 {
		t.Fatalf("present legacy fields were not retained: %+v", decoded)
	}
	if decoded.PreTexCompoTypeStr != "" {
		t.Fatalf("missing Key(18) gained a game initializer: %q", decoded.PreTexCompoTypeStr)
	}

	// A still-shorter payload leaves every omitted field at the wire-model zero.
	wire, err = ct.EncodeMsgpack(raw[:11])
	if err != nil {
		t.Fatal(err)
	}
	if err := ct.DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.LayNoInGroup != 0 || decoded.Alpha != 0 || decoded.PreTexCompoTypeStr != "" {
		t.Fatalf("short payload gained constructor defaults: %+v", decoded)
	}
}

func TestColliderEncodingValidationRejectsInvalidObjectsWithoutPanic(t *testing.T) {
	var typedNilSphere *ColliderSphere
	tests := []struct {
		name    string
		env     *KCESPayloadEnvelope
		wantErr string
	}{
		{
			name: "typed nil collider",
			env: &KCESPayloadEnvelope{Format: PayloadFormatKCESMessagePack, Extension: ".dbcol", LengthPrefixed: true, StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindColliderPackage, ColliderPackage: &ColliderPackage{
				Colliders: []ColliderRef{{Type: ColliderTypeSphere, Collider: typedNilSphere}},
			}},
			wantErr: "is nil (*ColliderSphere)",
		},
		{
			name: "union tag mismatch",
			env: &KCESPayloadEnvelope{Format: PayloadFormatKCESMessagePack, Extension: ".dbcol", LengthPrefixed: true, StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindColliderPackage, ColliderPackage: &ColliderPackage{
				Colliders: []ColliderRef{{Type: ColliderTypePlane, Collider: &ColliderCapsule{}}},
			}},
			wantErr: "concrete type requires 1",
		},
		{
			name: "limb collider must be MaidProp",
			env: &KCESPayloadEnvelope{Format: PayloadFormatKCESMessagePack, Extension: ".limbcol", LengthPrefixed: true, StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindLimbCollider, LimbCollider: &LimbColliderPackage{
				Items: []LimbColliderItem{{Collider: &ColliderCapsule{}}},
			}},
			wantErr: "must be *ColliderMaidProp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EncodeKCESPayload(tc.env)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestLimbColliderDecodeUsesDeclaredMaidPropTypeWithoutInferringByWidth(t *testing.T) {
	// Key(2) is statically NativeMaidPropColliderStatus. A 13-slot array is a
	// short instance of that declared type; the serializer must not infer a
	// Capsule merely because the present prefix happens to match its fields.
	raw := []interface{}{
		int64(limbColliderPackageFixVersion),
		[]interface{}{
			[]interface{}{int64(limbColliderItemFixVersion), int64(0), colliderCapsuleIndexedTestValue(colliderStatusFixVersion)},
		},
	}
	envelope, err := DecodeKCESPayload(lengthPrefixedIndexedTestValue(t, raw), ".limbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	status, ok := envelope.LimbCollider.Items[0].Collider.(*ColliderMaidProp)
	if !ok {
		t.Fatalf("decoded concrete type = %T, want *ColliderMaidProp", envelope.LimbCollider.Items[0].Collider)
	}
	assertIndexedMetadata(t, status.IndexedObjectMetadata, 13)
	if status.CenterMpnList != nil || status.CenterMpnNameList != nil {
		t.Fatalf("omitted MaidProp fields gained values: %+v", status)
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
		env := &KCESPayloadEnvelope{
			Format: PayloadFormatKCESMessagePack, Extension: ".dbcol", LengthPrefixed: true,
			StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindColliderPackage,
			ColliderPackage: &ColliderPackage{
				Colliders:      []ColliderRef{{Type: ColliderTypePlane, Collider: &ColliderPlane{}}},
				LimbEnableList: []ColliderState{{LimbType: 1, IsEnable: true}},
			},
		}
		encoded, err := EncodeKCESPayload(env)
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		decoded, err := DecodeKCESPayload(encoded, ".dbcol")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		plane, ok := decoded.ColliderPackage.Colliders[0].Collider.(*ColliderPlane)
		if !ok {
			t.Fatalf("decoded collider type = %T", decoded.ColliderPackage.Colliders[0].Collider)
		}
		if decoded.ColliderPackage.Version != 0 || plane.Version != 0 ||
			decoded.ColliderPackage.LimbEnableList[0].Version != 0 {
			t.Fatalf("versions changed: package=%d collider=%d state=%d",
				decoded.ColliderPackage.Version, plane.Version, decoded.ColliderPackage.LimbEnableList[0].Version)
		}
		if env.ColliderPackage.Version != 0 || env.ColliderPackage.Colliders[0].Collider.(*ColliderPlane).Version != 0 ||
			env.ColliderPackage.LimbEnableList[0].Version != 0 {
			t.Fatalf("EncodeKCESPayload mutated generic collider input: %+v", env.ColliderPackage)
		}
	})

	t.Run("limb package", func(t *testing.T) {
		env := &KCESPayloadEnvelope{
			Format: PayloadFormatKCESMessagePack, Extension: ".limbcol", LengthPrefixed: true,
			StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindLimbCollider,
			LimbCollider: &LimbColliderPackage{
				Items: []LimbColliderItem{{Collider: &ColliderMaidProp{}}},
			},
		}
		encoded, err := EncodeKCESPayload(env)
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		decoded, err := DecodeKCESPayload(encoded, ".limbcol")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		maidProp, ok := decoded.LimbCollider.Items[0].Collider.(*ColliderMaidProp)
		if !ok {
			t.Fatalf("decoded collider type = %T", decoded.LimbCollider.Items[0].Collider)
		}
		if decoded.LimbCollider.Version != 0 || decoded.LimbCollider.Items[0].Version != 0 || maidProp.Version != 0 {
			t.Fatalf("versions changed: package=%d item=%d collider=%d",
				decoded.LimbCollider.Version, decoded.LimbCollider.Items[0].Version, maidProp.Version)
		}
		if env.LimbCollider.Version != 0 || env.LimbCollider.Items[0].Version != 0 ||
			env.LimbCollider.Items[0].Collider.(*ColliderMaidProp).Version != 0 {
			t.Fatalf("EncodeKCESPayload mutated limb collider input: %+v", env.LimbCollider)
		}
	})

	t.Run("IK package", func(t *testing.T) {
		env := &KCESPayloadEnvelope{
			Format: PayloadFormatKCESMessagePack, Extension: ".ikcol", LengthPrefixed: true,
			StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindIKCollider,
			IKCollider: &IKColliderPackage{
				Groups: []IKColliderGroup{{
					Colliders: []ColliderRef{{Type: ColliderTypeSphere, Collider: &ColliderSphere{}}},
				}},
			},
		}
		encoded, err := EncodeKCESPayload(env)
		if err != nil {
			t.Fatalf("EncodeKCESPayload: %v", err)
		}
		decoded, err := DecodeKCESPayload(encoded, ".ikcol")
		if err != nil {
			t.Fatalf("DecodeKCESPayload: %v", err)
		}
		sphere, ok := decoded.IKCollider.Groups[0].Colliders[0].Collider.(*ColliderSphere)
		if !ok {
			t.Fatalf("decoded collider type = %T", decoded.IKCollider.Groups[0].Colliders[0].Collider)
		}
		if decoded.IKCollider.Version != 0 || decoded.IKCollider.Groups[0].Version != 0 || sphere.Version != 0 {
			t.Fatalf("versions changed: package=%d group=%d collider=%d",
				decoded.IKCollider.Version, decoded.IKCollider.Groups[0].Version, sphere.Version)
		}
		if env.IKCollider.Version != 0 || env.IKCollider.Groups[0].Version != 0 ||
			env.IKCollider.Groups[0].Colliders[0].Collider.(*ColliderSphere).Version != 0 {
			t.Fatalf("EncodeKCESPayload mutated IK collider input: %+v", env.IKCollider)
		}
	})
}

func TestMaidPropEncodingPreservesMPNFields(t *testing.T) {
	maidProp := &ColliderMaidProp{
		ColliderObject:         ColliderObject{Version: 1002},
		CenterMpnList:          []int{7, -1},
		StartRadiusMpnList:     []int{25},
		EndRadiusMpnList:       []int{49},
		CenterMpnNameList:      []string{"stale-center"},
		StartRadiusMpnNameList: []string{"stale-start"},
		EndRadiusMpnNameList:   []string{"stale-end"},
	}
	env := &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".limbcol", LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindLimbCollider,
		LimbCollider: &LimbColliderPackage{
			Items: []LimbColliderItem{{Collider: maidProp}},
		},
	}

	encoded, err := EncodeKCESPayload(env)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	decoded, err := DecodeKCESPayload(encoded, ".limbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	got, ok := decoded.LimbCollider.Items[0].Collider.(*ColliderMaidProp)
	if !ok {
		t.Fatalf("decoded collider type = %T", decoded.LimbCollider.Items[0].Collider)
	}
	if got.Version != 1002 {
		t.Fatalf("decoded version = %d, want 1002", got.Version)
	}
	if want := []string{"stale-center"}; !reflect.DeepEqual(got.CenterMpnNameList, want) {
		t.Fatalf("center names = %#v, want %#v", got.CenterMpnNameList, want)
	}
	if want := []string{"stale-start"}; !reflect.DeepEqual(got.StartRadiusMpnNameList, want) {
		t.Fatalf("start names = %#v, want %#v", got.StartRadiusMpnNameList, want)
	}
	if want := []string{"stale-end"}; !reflect.DeepEqual(got.EndRadiusMpnNameList, want) {
		t.Fatalf("end names = %#v, want %#v", got.EndRadiusMpnNameList, want)
	}
	if !reflect.DeepEqual(got.CenterMpnList, maidProp.CenterMpnList) ||
		!reflect.DeepEqual(got.StartRadiusMpnList, maidProp.StartRadiusMpnList) ||
		!reflect.DeepEqual(got.EndRadiusMpnList, maidProp.EndRadiusMpnList) {
		t.Fatalf("numeric MPN lists changed: %+v", got)
	}

	if maidProp.Version != 1002 ||
		!reflect.DeepEqual(maidProp.CenterMpnList, []int{7, -1}) ||
		!reflect.DeepEqual(maidProp.CenterMpnNameList, []string{"stale-center"}) ||
		!reflect.DeepEqual(maidProp.StartRadiusMpnNameList, []string{"stale-start"}) ||
		!reflect.DeepEqual(maidProp.EndRadiusMpnNameList, []string{"stale-end"}) {
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
		CenterMpnList:      []int{9},
		StartRadiusMpnList: []int{},
		EndRadiusMpnList:   []int{},
	}
	legacyWire, err := ct.EncodeIndexedMsgpack(legacy)
	if err != nil {
		t.Fatalf("encode legacy MaidProp: %v", err)
	}
	var legacyResult ColliderMaidProp
	err = ct.DecodeMsgpack(legacyWire, &legacyResult)
	if err != nil {
		t.Fatalf("decode legacy MaidProp: %v", err)
	}
	if legacyResult.Version != 1001 || !reflect.DeepEqual(legacyResult.CenterMpnList, []int{9}) ||
		legacyResult.CenterMpnNameList != nil {
		t.Fatalf("legacy fields were migrated: %+v", legacyResult)
	}

	current := &ColliderMaidProp{
		ColliderObject: ColliderObject{
			Version:       colliderMaidPropFixVersion,
			LocalRotation: Vector4{W: 1},
			LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
		},
		CenterMpnList:          []int{7},
		StartRadiusMpnList:     []int{25},
		EndRadiusMpnList:       []int{26},
		CenterMpnNameList:      []string{"MuneUpDown"},
		StartRadiusMpnNameList: []string{"stale"},
		EndRadiusMpnNameList:   []string{"stale"},
	}
	currentWire, err := ct.EncodeIndexedMsgpack(current)
	if err != nil {
		t.Fatalf("encode current MaidProp: %v", err)
	}
	var currentResult ColliderMaidProp
	err = ct.DecodeMsgpack(currentWire, &currentResult)
	if err != nil {
		t.Fatalf("decode current MaidProp: %v", err)
	}
	if !reflect.DeepEqual(currentResult.CenterMpnList, []int{7}) ||
		!reflect.DeepEqual(currentResult.StartRadiusMpnList, []int{25}) ||
		!reflect.DeepEqual(currentResult.EndRadiusMpnList, []int{26}) ||
		!reflect.DeepEqual(currentResult.CenterMpnNameList, []string{"MuneUpDown"}) ||
		!reflect.DeepEqual(currentResult.StartRadiusMpnNameList, []string{"stale"}) ||
		!reflect.DeepEqual(currentResult.EndRadiusMpnNameList, []string{"stale"}) {
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
		CenterMpnList:          []int{},
		StartRadiusMpnList:     []int{},
		EndRadiusMpnList:       []int{},
		CenterMpnNameList:      []string{"MuneL"},
		StartRadiusMpnNameList: []string{},
		EndRadiusMpnNameList:   []string{},
	}

	raw := colliderMaidPropIndexedTestValue(base.Version)
	raw[16] = nil
	raw[22] = []interface{}{"not_an_mpn"}
	wire, err := ct.EncodeMsgpack(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got ColliderMaidProp
	err = ct.DecodeMsgpack(wire, &got)
	if err != nil {
		t.Fatalf("decode nil/opaque MaidProp fields: %v", err)
	}
	if got.CenterMpnList != nil || !reflect.DeepEqual(got.CenterMpnNameList, []string{"not_an_mpn"}) {
		t.Fatalf("nil/opaque fields changed: %+v", got)
	}

	malformed := colliderMaidPropIndexedTestValue(base.Version)
	malformed[22] = []interface{}{int64(7)}
	malformedWire, err := ct.EncodeMsgpack(malformed)
	if err != nil {
		t.Fatal(err)
	}
	if err := ct.DecodeMsgpack(malformedWire, &got); err == nil {
		t.Fatal("non-string List<string> element was accepted")
	}
}

func TestMaidPropEncodingRejectsMPNOutsideInt32(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot represent an out-of-Int32 test value")
	}
	tooLarge, err := strconv.ParseInt("2147483648", 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	env := &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".limbcol", LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindLimbCollider,
		LimbCollider: &LimbColliderPackage{Items: []LimbColliderItem{{
			Collider: &ColliderMaidProp{CenterMpnList: []int{int(tooLarge)}},
		}}},
	}
	if _, err := EncodeKCESPayload(env); err == nil || !strings.Contains(err.Error(), "Int32 range") {
		t.Fatalf("EncodeKCESPayload error = %v, want Int32 range rejection", err)
	}
}
