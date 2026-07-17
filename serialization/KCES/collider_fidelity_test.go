package KCES

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

func TestColliderPayloadPreservesNestedIndexedWireMetadata(t *testing.T) {
	rootFuture := codec.Raw{0xd4, 0x71, 0x01}
	refFuture := codec.Raw{0x82, 0xa1, 'x', 0x01, 0xa1, 'x', 0x02}
	statusFuture := codec.Raw{0xd6, 0x72, 0, 0, 0, 7}
	rotationFuture := codec.Raw{0xcc, 0x80}
	stateFuture := codec.Raw{0xd4, 0x73, 0x01}

	capsule := []interface{}{
		int64(1000),
		nil,
		nil,
		[]interface{}{float32(1)},
		[]interface{}{float32(0), float32(0), float32(0), float32(1), rotationFuture},
		[]interface{}{float32(1), float32(1), float32(1)},
		[]interface{}{float32(0), float32(0), float32(0)},
		int64(0),
		int64(VectorTypeY),
		false,
		math.MaxFloat64,
		math.NaN(),
		math.Inf(-1),
		statusFuture,
	}
	root := []interface{}{
		int64(1000),
		[]interface{}{
			nil,
			[]interface{}{int64(ColliderTypeSphere)},
			[]interface{}{int64(ColliderTypeCapsule), capsule, refFuture},
		},
		[]interface{}{
			nil,
			[]interface{}{int64(1000)},
			[]interface{}{int64(1000), int64(2), true, stateFuture},
		},
		rootFuture,
	}
	wire := lengthPrefixedIndexedTestValue(t, root)

	envelope, err := DecodeKCESPayload(wire, ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	pkg := envelope.ColliderPackage
	assertIndexedMetadata(t, pkg.IndexedObjectMetadata, 4, rootFuture)
	assertNullElement(t, pkg.IndexedObjectMetadata, 1, 0)
	assertNullElement(t, pkg.IndexedObjectMetadata, 2, 0)
	if len(pkg.Colliders) != 3 || len(pkg.LimbEnableList) != 3 {
		t.Fatalf("decoded list sizes = colliders %d, states %d", len(pkg.Colliders), len(pkg.LimbEnableList))
	}
	assertIndexedMetadata(t, pkg.Colliders[1].IndexedObjectMetadata, 1)
	assertIndexedMetadata(t, pkg.Colliders[2].IndexedObjectMetadata, 3, refFuture)
	assertIndexedMetadata(t, pkg.LimbEnableList[1].IndexedObjectMetadata, 1)
	assertIndexedMetadata(t, pkg.LimbEnableList[2].IndexedObjectMetadata, 4, stateFuture)

	decodedCapsule, ok := pkg.Colliders[2].Collider.(*ColliderCapsule)
	if !ok {
		t.Fatalf("decoded collider type = %T, want *ColliderCapsule", pkg.Colliders[2].Collider)
	}
	assertIndexedMetadata(t, decodedCapsule.IndexedObjectMetadata, 14, statusFuture)
	assertNilSlot(t, decodedCapsule.IndexedObjectMetadata, 1)
	assertNilSlot(t, decodedCapsule.IndexedObjectMetadata, 2)
	assertIndexedMetadata(t, decodedCapsule.LocalPosition.IndexedObjectMetadata, 1)
	assertIndexedMetadata(t, decodedCapsule.LocalRotation.IndexedObjectMetadata, 5, rotationFuture)
	if !math.IsInf(float64(decodedCapsule.StartRadius), 1) ||
		!math.IsNaN(float64(decodedCapsule.EndRadius)) ||
		!math.IsInf(float64(decodedCapsule.Height), -1) {
		t.Fatalf("ReadSingle values changed: %+v", decodedCapsule)
	}
	// encoding/json cannot represent IEEE-754 non-finite numbers. Replace only
	// those three known values before exercising the ordinary JSON edit path;
	// the indexed metadata being tested remains untouched.
	decodedCapsule.StartRadius = 1
	decodedCapsule.EndRadius = 2
	decodedCapsule.Height = 3

	jsonData, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal editing JSON: %v", err)
	}
	var edited KCESPayloadEnvelope
	if err := json.Unmarshal(jsonData, &edited); err != nil {
		t.Fatalf("unmarshal editing JSON: %v", err)
	}
	edited.ColliderPackage.Version = -17
	reencoded, err := EncodeKCESPayload(&edited)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	reencodedRoot := decodeLengthPrefixedIndexedTestArray(t, reencoded)
	if len(reencodedRoot) != 4 || !rawMessagePackEqual(reencodedRoot[3], rootFuture) {
		t.Fatalf("root future slot changed: % x", reencodedRoot)
	}
	refs := decodeIndexedTestArray(t, reencodedRoot[1])
	assertRawNil(t, refs[0], "null ColliderRef")
	if len(decodeIndexedTestArray(t, refs[1])) != 1 {
		t.Fatal("short union wrapper was widened")
	}
	fullRef := decodeIndexedTestArray(t, refs[2])
	if len(fullRef) != 3 || !rawMessagePackEqual(fullRef[2], refFuture) {
		t.Fatalf("union future slot changed: % x", fullRef)
	}
	status := decodeIndexedTestArray(t, fullRef[1])
	if len(status) != 14 || !rawMessagePackEqual(status[13], statusFuture) {
		t.Fatalf("status future slot changed: % x", status)
	}
	if len(decodeIndexedTestArray(t, status[3])) != 1 {
		t.Fatal("short nested Vector3 was widened")
	}
	rotation := decodeIndexedTestArray(t, status[4])
	if len(rotation) != 5 || !rawMessagePackEqual(rotation[4], rotationFuture) {
		t.Fatalf("Vector4 future slot changed: % x", rotation)
	}
	states := decodeIndexedTestArray(t, reencodedRoot[2])
	assertRawNil(t, states[0], "null ColliderState")
	if len(decodeIndexedTestArray(t, states[1])) != 1 {
		t.Fatal("short ColliderState was widened")
	}
	fullState := decodeIndexedTestArray(t, states[2])
	if len(fullState) != 4 || !rawMessagePackEqual(fullState[3], stateFuture) {
		t.Fatalf("ColliderState future slot changed: % x", fullState)
	}
}

func TestMaidPropLayoutUsesWireWidthNotStoredVersion(t *testing.T) {
	gap13 := codec.Raw{0x82, 0xa1, 'g', 0x01, 0xa1, 'g', 0x02}
	gap14 := codec.Raw{0xd4, 0x21, 0x7f}
	gap15 := codec.Raw{0xcc, 0x01}
	maidFuture := codec.Raw{0xd6, 0x22, 0, 0, 0, 9}
	itemFuture := codec.Raw{0xcc, 0x80}
	packageFuture := codec.Raw{0xd4, 0x23, 0x01}

	maid := make([]interface{}, 25)
	maid[0] = int64(1001) // stored version does not select the formatter width
	maid[1] = "parent"
	maid[2] = "name"
	maid[3] = []interface{}{float32(0), float32(0), float32(0)}
	maid[4] = []interface{}{float32(0), float32(0), float32(0), float32(1)}
	maid[5] = []interface{}{float32(1), float32(1), float32(1)}
	maid[6] = []interface{}{float32(0), float32(0), float32(0)}
	maid[7] = int64(0)
	maid[8] = int64(VectorTypeX)
	maid[9] = false
	maid[10] = float32(0.1)
	maid[11] = float32(0.2)
	maid[12] = float32(0.3)
	maid[13] = gap13
	maid[14] = gap14
	maid[15] = gap15
	maid[16] = []interface{}{int64(7)}
	maid[17] = []interface{}{float32(1), float32(2), float32(3)}
	maid[18] = []interface{}{int64(40), int64(41)}
	maid[19] = float32(1)
	maid[20] = []interface{}{int64(42), int64(43)}
	maid[21] = float32(1)
	maid[22] = []interface{}{nil, "center-name"}
	maid[23] = []interface{}{"start-name"}
	maid[24] = []interface{}{"end-name"}
	maid = append(maid, maidFuture)

	root := []interface{}{
		int64(1000),
		[]interface{}{
			nil,
			[]interface{}{int64(1000), int64(3), maid, itemFuture},
		},
		packageFuture,
	}
	envelope, err := DecodeKCESPayload(lengthPrefixedIndexedTestValue(t, root), ".limbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	pkg := envelope.LimbCollider
	assertIndexedMetadata(t, pkg.IndexedObjectMetadata, 3, packageFuture)
	assertNullElement(t, pkg.IndexedObjectMetadata, 1, 0)
	item := &pkg.Items[1]
	assertIndexedMetadata(t, item.IndexedObjectMetadata, 4, itemFuture)
	decoded, ok := item.Collider.(*ColliderMaidProp)
	if !ok {
		t.Fatalf("decoded collider = %T, want *ColliderMaidProp", item.Collider)
	}
	if decoded.Version != 1001 || !reflect.DeepEqual(decoded.CenterMpnNameList, []string{"", "center-name"}) ||
		!reflect.DeepEqual(decoded.StartRadiusMpnNameList, []string{"start-name"}) ||
		!reflect.DeepEqual(decoded.EndRadiusMpnNameList, []string{"end-name"}) {
		t.Fatalf("stored version incorrectly selected fields: %+v", decoded)
	}
	assertIndexedMetadata(t, decoded.IndexedObjectMetadata, 26, maidFuture)
	assertNullElement(t, decoded.IndexedObjectMetadata, 22, 0)
	if !bytes.Equal(decoded.Reserved13, gap13) || !bytes.Equal(decoded.Reserved14, gap14) || !bytes.Equal(decoded.Reserved15, gap15) {
		t.Fatalf("sparse slots changed: % x / % x / % x", decoded.Reserved13, decoded.Reserved14, decoded.Reserved15)
	}

	reencoded, err := EncodeKCESPayload(envelope)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	reencodedRoot := decodeLengthPrefixedIndexedTestArray(t, reencoded)
	items := decodeIndexedTestArray(t, reencodedRoot[1])
	assertRawNil(t, items[0], "null LimbColliderItem")
	itemSlots := decodeIndexedTestArray(t, items[1])
	maidSlots := decodeIndexedTestArray(t, itemSlots[2])
	if len(maidSlots) != 26 || !rawMessagePackEqual(maidSlots[13], gap13) ||
		!rawMessagePackEqual(maidSlots[14], gap14) || !rawMessagePackEqual(maidSlots[15], gap15) ||
		!rawMessagePackEqual(maidSlots[25], maidFuture) {
		t.Fatalf("MaidProp sparse/future slots changed: % x", maidSlots)
	}
	names := decodeIndexedTestArray(t, maidSlots[22])
	assertRawNil(t, names[0], "null MaidProp name")

	// A 22-slot object remains 22 slots even when its stored Version is 1002.
	shortMaid := append([]interface{}(nil), maid[:22]...)
	shortMaid[0] = int64(1002)
	shortRoot := []interface{}{int64(1000), []interface{}{[]interface{}{int64(1000), int64(0), shortMaid}}}
	shortEnvelope, err := DecodeKCESPayload(lengthPrefixedIndexedTestValue(t, shortRoot), ".limbcol")
	if err != nil {
		t.Fatalf("decode 22-slot MaidProp: %v", err)
	}
	shortStatus := shortEnvelope.LimbCollider.Items[0].Collider.(*ColliderMaidProp)
	assertIndexedMetadata(t, shortStatus.IndexedObjectMetadata, 22)
	if shortStatus.CenterMpnNameList != nil || shortStatus.StartRadiusMpnNameList != nil || shortStatus.EndRadiusMpnNameList != nil {
		t.Fatalf("omitted name slots gained values: %+v", shortStatus)
	}
	shortReencoded, err := EncodeKCESPayload(shortEnvelope)
	if err != nil {
		t.Fatalf("encode 22-slot MaidProp: %v", err)
	}
	shortRootSlots := decodeLengthPrefixedIndexedTestArray(t, shortReencoded)
	shortItems := decodeIndexedTestArray(t, shortRootSlots[1])
	shortItem := decodeIndexedTestArray(t, shortItems[0])
	if got := len(decodeIndexedTestArray(t, shortItem[2])); got != 22 {
		t.Fatalf("22-slot MaidProp was widened to %d", got)
	}
}

func TestColliderUnknownUnionTagPreservesRawConcreteValue(t *testing.T) {
	unknownConcrete := codec.Raw{0x82, 0xa1, 'x', 0x01, 0xa1, 'x', 0x02}
	root := []interface{}{
		int64(1000),
		[]interface{}{[]interface{}{int64(99), unknownConcrete}},
		[]interface{}{},
	}
	wire := lengthPrefixedIndexedTestValue(t, root)
	envelope, err := DecodeKCESPayload(wire, ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	ref := &envelope.ColliderPackage.Colliders[0]
	if ref.Type != 99 || ref.Collider != nil || !bytes.Equal(ref.ColliderRaw, unknownConcrete) {
		t.Fatalf("unknown union = %+v, raw % x", ref, ref.ColliderRaw)
	}
	editingJSON, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var edited KCESPayloadEnvelope
	if err := json.Unmarshal(editingJSON, &edited); err != nil {
		t.Fatalf("unmarshal editing JSON: %v", err)
	}
	reencoded, err := EncodeKCESPayload(&edited)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	rootSlots := decodeLengthPrefixedIndexedTestArray(t, reencoded)
	refs := decodeIndexedTestArray(t, rootSlots[1])
	refSlots := decodeIndexedTestArray(t, refs[0])
	if len(refSlots) != 2 || !rawMessagePackEqual(refSlots[1], unknownConcrete) {
		t.Fatalf("unknown union raw changed: % x", refSlots)
	}
}

func TestColliderCurrentIndexedWidthsAreIndependentOfVersion(t *testing.T) {
	tests := []struct {
		name  string
		value codec.Selfer
		width int
	}{
		{name: "package", value: &ColliderPackage{}, width: 3},
		{name: "union", value: &ColliderRef{}, width: 2},
		{name: "plane", value: &ColliderPlane{ColliderObject: ColliderObject{Version: -9}}, width: 10},
		{name: "capsule", value: &ColliderCapsule{ColliderObject: ColliderObject{Version: -9}}, width: 13},
		{name: "sphere", value: &ColliderSphere{ColliderObject: ColliderObject{Version: -9}}, width: 9},
		{name: "maidProp old version", value: &ColliderMaidProp{ColliderObject: ColliderObject{Version: 1001}}, width: 25},
		{name: "maidProp future version", value: &ColliderMaidProp{ColliderObject: ColliderObject{Version: 9999}}, width: 25},
		{name: "state", value: &ColliderState{}, width: 3},
		{name: "limb package", value: &LimbColliderPackage{}, width: 2},
		{name: "limb item", value: &LimbColliderItem{}, width: 3},
		{name: "IK package", value: &IKColliderPackage{}, width: 2},
		{name: "IK group", value: &IKColliderGroup{}, width: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire, err := ct.EncodeIndexedMsgpack(test.value)
			if err != nil {
				t.Fatalf("EncodeIndexedMsgpack: %v", err)
			}
			if got := len(decodeIndexedTestArray(t, wire)); got != test.width {
				t.Fatalf("width = %d, want %d", got, test.width)
			}
		})
	}
}

func TestColliderShortWidthRejectsPopulatedOmittedField(t *testing.T) {
	count := 8
	value := &ColliderPlane{
		IndexedObjectMetadata: &IndexedObjectMetadata{FieldCount: &count},
		Direction:             VectorTypeY,
	}
	if _, err := ct.EncodeIndexedMsgpack(value); err == nil || !strings.Contains(err.Error(), "would discard direction") {
		t.Fatalf("short ColliderPlane populated-field error = %v", err)
	}
}

func colliderCapsuleIndexedTestValue(version int) []interface{} {
	return []interface{}{
		int64(version), "parent", "name",
		[]interface{}{float32(0), float32(0), float32(0)},
		[]interface{}{float32(0), float32(0), float32(0), float32(1)},
		[]interface{}{float32(1), float32(1), float32(1)},
		[]interface{}{float32(0), float32(0), float32(0)},
		int64(0), int64(VectorTypeY), false,
		float32(0.5), float32(0.5), float32(0),
	}
}

func colliderMaidPropIndexedTestValue(version int) []interface{} {
	value := append([]interface{}(nil), colliderCapsuleIndexedTestValue(version)...)
	value = append(value, nil, nil, nil)
	value = append(value,
		[]interface{}{},
		[]interface{}{float32(0), float32(0), float32(0)},
		[]interface{}{}, float32(1),
		[]interface{}{}, float32(1),
		[]interface{}{}, []interface{}{}, []interface{}{},
	)
	return value
}
