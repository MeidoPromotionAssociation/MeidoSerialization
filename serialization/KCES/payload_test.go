package KCES

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

func TestDecodeKCESPayloadRequiresGameLengthPrefix(t *testing.T) {
	path := kcesfixtures.TextAssetPath(t, "partsmeta.aba", "default_hairf.dbconf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESPayload(data, path); err != nil {
		t.Fatalf("real prefixed payload was rejected: %v", err)
	}
	if len(data) <= 4 {
		t.Fatalf("real payload is unexpectedly short: %d", len(data))
	}
	if _, err := DecodeKCESPayload(data[4:], path); err == nil || !strings.Contains(err.Error(), "length prefix") {
		t.Fatalf("unprefixed game payload error = %v, want required length-prefix rejection", err)
	}

	badLength := append([]byte(nil), data...)
	badLength[0]++
	if _, err := DecodeKCESPayload(badLength, path); err == nil || !strings.Contains(err.Error(), "declared") {
		t.Fatalf("mismatched game payload length error = %v, want declared/actual rejection", err)
	}
}

func TestRecognizedKCESPayloadRoundTripsTypedNilRoot(t *testing.T) {
	for _, extension := range []string{".dbconf", ".db2conf", ".dbcol", ".limbcol", ".ikcol", ".dsbconf"} {
		t.Run(extension, func(t *testing.T) {
			compressed, err := msgpack.CompressLz4BlockArray([]byte{0xc0})
			if err != nil {
				t.Fatalf("compress nil payload root: %v", err)
			}
			value, err := DecodeKCESPayload(AddLengthPrefix(compressed), extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload rejected typed nil root: %v", err)
			}
			if !payloadRootIsNil(value) {
				t.Fatalf("typed nil root became populated: %+v", value)
			}
			reencoded, err := EncodeKCESPayload(value, extension)
			if err != nil {
				t.Fatalf("EncodeKCESPayload rejected typed nil root: %v", err)
			}
			roundTrip, err := DecodeKCESPayload(reencoded, extension)
			if err != nil || !payloadRootIsNil(roundTrip) {
				t.Fatalf("typed nil root round trip: root=%+v error=%v", roundTrip, err)
			}

			compressed, err = msgpack.CompressLz4BlockArray([]byte{0xc0, 0xc0})
			if err != nil {
				t.Fatalf("compress nil payload root with trailing value: %v", err)
			}
			if _, err := DecodeKCESPayload(AddLengthPrefix(compressed), extension); err == nil ||
				!strings.Contains(strings.ToLower(err.Error()), "trailing") {
				t.Fatalf("DecodeKCESPayload trailing-data error = %v", err)
			}
		})
	}
}

func TestDynamicBoneStatusPayloadRoundTrip(t *testing.T) {
	status := &DynamicBoneStatus{
		Version:   1000,
		Damping:   0.6,
		Gravity:   Vector3{Y: -0.05},
		EndOffset: Vector3{X: 1, Y: 2, Z: 3},
		DampingKeyFrames: []*DynamicBoneAnimationFrame{
			{Time: 0, Value: 0.25, InTangent: 0, OutTangent: 1},
		},
		FreezeAxis: 2,
	}

	encoded, err := EncodeDynamicBoneStatusFile(status)
	if err != nil {
		t.Fatalf("EncodeDynamicBoneStatusFile: %v", err)
	}
	if payloadLen := int(encoded[0]) | int(encoded[1])<<8 | int(encoded[2])<<16 | int(encoded[3])<<24; payloadLen != len(encoded)-4 {
		t.Fatalf("length prefix got %d, want %d", payloadLen, len(encoded)-4)
	}

	decoded, err := DecodeDynamicBoneStatusFile(encoded)
	if err != nil {
		t.Fatalf("DecodeDynamicBoneStatusFile: %v", err)
	}
	if decoded.Version != 1000 || decoded.FreezeAxis != 2 || decoded.Gravity.Y != -0.05 {
		t.Fatalf("unexpected decoded status: %+v", decoded)
	}
	if len(decoded.DampingKeyFrames) != 1 || decoded.DampingKeyFrames[0].Value != 0.25 {
		t.Fatalf("unexpected keyframes: %+v", decoded.DampingKeyFrames)
	}
}

func TestDynamicBoneStatusEncodingDoesNotMutateInput(t *testing.T) {
	status := &DynamicBoneStatus{}
	encoded, err := EncodeDynamicBoneStatusFile(status)
	if err != nil {
		t.Fatalf("EncodeDynamicBoneStatusFile: %v", err)
	}
	if status.Version != 0 || status.DampingKeyFrames != nil || status.ElasticityKeyFrames != nil ||
		status.StiffnessKeyFrames != nil || status.InertKeyFrames != nil || status.RadiusKeyFrames != nil {
		t.Fatalf("encoding mutated input: %+v", status)
	}
	decoded, err := DecodeDynamicBoneStatusFile(encoded)
	if err != nil {
		t.Fatalf("DecodeDynamicBoneStatusFile: %v", err)
	}
	if !reflect.DeepEqual(decoded, status) {
		t.Fatalf("zero values were changed by serialization:\n got  %#v\n want %#v", decoded, status)
	}
}

func TestJSONStringPayloadRoundTrip(t *testing.T) {
	clothType := int32(1)
	rootRotation := float32(0.5)
	document := &MagicaClothSerializeData{ClothType: &clothType, RootRotation: &rootRotation}
	encoded, err := EncodeKCESPayload(document, ".db2conf")
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	decoded, err := DecodeKCESPayload(encoded, ".db2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	if !reflect.DeepEqual(decoded, document) {
		t.Fatalf("unexpected decoded ClothSerializeData:\n got  %#v\n want %#v", decoded, document)
	}
}

func TestJSONStringPayloadKeepsOnlyModeledMembers(t *testing.T) {
	original := "{\r\n  \"clothType\" : 1,\r\n  \"rootRotation\" : 0.5\r\n}\r\n"
	msgpackData, err := msgpack.EncodeMsgpack(original)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatal(err)
	}
	wire := AddLengthPrefix(compressed)

	decoded, err := DecodeKCESPayload(wire, ".db2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload() error = %v", err)
	}
	document, ok := decoded.(*MagicaClothSerializeData)
	if !ok || document == nil || document.ClothType == nil || *document.ClothType != 1 ||
		document.RootRotation == nil || *document.RootRotation != 0.5 {
		t.Fatalf("decoded ClothSerializeData = %#v", decoded)
	}

	normalized, err := EncodeKCESPayload(document, ".db2conf")
	if err != nil {
		t.Fatalf("EncodeKCESPayload(normalized) error = %v", err)
	}
	if bytes.Equal(normalized, wire) {
		t.Fatal("ClothSerializeData unexpectedly retained formatting-only source bytes")
	}
	renormalized, err := DecodeKCESPayload(normalized, ".db2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload(normalized) error = %v", err)
	}
	if !reflect.DeepEqual(renormalized, document) {
		t.Fatalf("normalized ClothSerializeData changed: got %#v, want %#v", renormalized, document)
	}

	editedType := int32(2)
	document.ClothType = &editedType
	edited, err := EncodeKCESPayload(document, ".db2conf")
	if err != nil {
		t.Fatalf("EncodeKCESPayload(edited) error = %v", err)
	}
	redecoded, err := DecodeKCESPayload(edited, ".db2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload(edited) error = %v", err)
	}
	editedDocument, ok := redecoded.(*MagicaClothSerializeData)
	if !ok || editedDocument == nil || editedDocument.ClothType == nil || *editedDocument.ClothType != 2 {
		t.Fatalf("edited ClothSerializeData = %#v", redecoded)
	}
}

func TestJSONStringPayloadRejectsUnknownInnerMembers(t *testing.T) {
	msgpackData, err := msgpack.EncodeMsgpack(`{"clothType":1,"newMagicaField":3}`)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESPayload(AddLengthPrefix(compressed), ".db2conf"); err == nil ||
		!strings.Contains(err.Error(), "newMagicaField") {
		t.Fatalf("DecodeKCESPayload() error = %v, want unknown inner member rejection", err)
	}
}

func TestJSONStringPayloadRejectsLiteralNullDocument(t *testing.T) {
	msgpackData, err := msgpack.EncodeMsgpack(`null`)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESPayload(AddLengthPrefix(compressed), ".dsl2conf"); err == nil ||
		!strings.Contains(err.Error(), "literal null") {
		t.Fatalf("DecodeKCESPayload() error = %v, want literal null rejection", err)
	}
}

func TestRecognizedMessagePackPayloadRejectsTrailingBytes(t *testing.T) {
	root, err := msgpack.EncodeMsgpack(`{"clothType":1}`)
	if err != nil {
		t.Fatal(err)
	}
	tail := []byte{0xde, 0xad, 0xbe, 0xef, 0xc1}
	decompressed := append(append([]byte(nil), root...), tail...)
	compressed, err := msgpack.CompressLz4BlockArray(decompressed)
	if err != nil {
		t.Fatal(err)
	}
	wire := AddLengthPrefix(compressed)

	if _, err := DecodeKCESPayload(wire, ".db2conf"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "trailing") {
		t.Fatalf("DecodeKCESPayload trailing-data error = %v", err)
	}
}

func TestJSONStringPayloadRejectsInvalidInnerJSON(t *testing.T) {
	msgpackData, err := msgpack.EncodeMsgpack(`{not-json}`)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESPayload(AddLengthPrefix(compressed), ".db2conf"); err == nil ||
		!strings.Contains(err.Error(), "ClothSerializeData") {
		t.Fatalf("DecodeKCESPayload error = %v, want invalid inner JSON rejection", err)
	}
}

func TestKCESPayloadRejectsUnknownExtension(t *testing.T) {
	msgpackData, err := msgpack.EncodeMsgpack([]interface{}{int64(1000), []interface{}{"union-like", uint64(42)}})
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatalf("CompressLz4BlockArray: %v", err)
	}
	input := AddLengthPrefix(compressed)

	if _, err := DecodeKCESPayload(input, ".unknown"); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("DecodeKCESPayload unknown-extension error = %v", err)
	}
}

func TestClothParamsPayloadRoundTrip(t *testing.T) {
	params := &ClothParams{
		Radius:                           &BezierParam{StartValue: 0.02, EndValue: 0.04, UseEndValue: true},
		Mass:                             &BezierParam{StartValue: 1, EndValue: 1},
		UseGravity:                       true,
		Gravity:                          &BezierParam{StartValue: -9.8, EndValue: -9.8},
		UseDrag:                          true,
		Drag:                             &BezierParam{StartValue: 0.02, EndValue: 0.02, UseEndValue: true},
		UseMaxVelocity:                   true,
		MaxVelocity:                      &BezierParam{StartValue: 3, EndValue: 3},
		WorldMoveInfluence:               &BezierParam{StartValue: 0.5, EndValue: 0.5},
		WorldRotationInfluence:           &BezierParam{StartValue: 0.5, EndValue: 0.5},
		MassInfluence:                    0.3,
		WindInfluence:                    1,
		WindRandomScale:                  0.7,
		DisableDistance:                  20,
		DisableFadeDistance:              5,
		TeleportDistance:                 0.2,
		TeleportRotation:                 45,
		UseClampDistanceRatio:            true,
		ClampDistanceMinRatio:            0.7,
		ClampDistanceMaxRatio:            1.1,
		ClampDistanceVelocityInfluence:   0.2,
		ClampPositionLength:              &BezierParam{StartValue: 0.03, EndValue: 0.2, UseEndValue: true},
		ClampPositionRatioX:              1,
		ClampPositionRatioY:              1,
		ClampPositionRatioZ:              1,
		ClampPositionVelocityInfluence:   0.2,
		ClampRotationAngle:               &BezierParam{StartValue: 30, EndValue: 30, UseEndValue: true},
		ClampRotationVelocityInfluence:   0.2,
		RestoreDistanceVelocityInfluence: 1,
		StructDistanceStiffness:          &BezierParam{StartValue: 1, EndValue: 1},
		BendDistanceMaxCount:             2,
		BendDistanceStiffness:            &BezierParam{StartValue: 0.5, EndValue: 0.5},
		NearDistanceMaxCount:             3,
		NearDistanceMaxDepth:             1,
		NearDistanceLength:               &BezierParam{StartValue: 0.1, EndValue: 0.1, UseEndValue: true},
		NearDistanceStiffness:            &BezierParam{StartValue: 0.3, EndValue: 0.3},
		RestoreRotation:                  &BezierParam{StartValue: 0.3, EndValue: 0.1, UseEndValue: true},
		SpringPower:                      0.017,
		SpringRadius:                     0.1,
		SpringScaleX:                     1,
		SpringScaleY:                     1,
		SpringScaleZ:                     1,
		SpringIntensity:                  1,
		SpringDirectionAtten:             &BezierParam{StartValue: 1, EndValue: 0, UseEndValue: true, CurveValue: 0.234, UseCurveValue: true},
		SpringDistanceAtten:              &BezierParam{StartValue: 1, EndValue: 0, UseEndValue: true, CurveValue: 0.395, UseCurveValue: true},
		AdjustRotationPower:              5,
		TriangleBend:                     &BezierParam{StartValue: 0.5, EndValue: 0.5, UseEndValue: true},
		MaxVolumeLength:                  0.1,
		VolumeStretchStiffness:           &BezierParam{StartValue: 0.5, EndValue: 0.5, UseEndValue: true},
		VolumeShearStiffness:             &BezierParam{StartValue: 0.5, EndValue: 0.5, UseEndValue: true},
		Friction:                         0.2,
		UsePenetration:                   true,
		PenetrationMode:                  ClothPenetrationModeColliderPenetration,
		PenetrationAxis:                  ClothPenetrationAxisInverseZ,
		PenetrationMaxDepth:              1,
		PenetrationConnectDistance:       &BezierParam{StartValue: 0.2, EndValue: 0.3, UseEndValue: true},
		PenetrationDistance:              &BezierParam{StartValue: 0.1, EndValue: 0.2, UseEndValue: true},
		PenetrationRadius:                &BezierParam{StartValue: 0.3, EndValue: 1, UseEndValue: true},
		UseLineAvarageRotation:           true,
		GravityDirection:                 Vector3{Y: 1},
		MaxMoveSpeed:                     10,
		MaxRotationSpeed:                 360,
		ResetStabilizationTime:           0.1,
		ClampRotationVelocityLimit:       1,
	}

	encoded, err := EncodeClothParamsFile(params, ".dsbconf")
	if err != nil {
		t.Fatalf("EncodeClothParamsFile: %v", err)
	}
	decoded, err := DecodeKCESPayload(encoded, ".dsbconf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	decodedParams, ok := decoded.(*ClothParams)
	if !ok || decodedParams == nil {
		t.Fatalf("unexpected decoded cloth params: %#v", decoded)
	}
	if decodedParams.PenetrationMode != ClothPenetrationModeColliderPenetration {
		t.Fatalf("penetration mode got %d", decodedParams.PenetrationMode)
	}
}

func TestColliderPackagePayloadRoundTrip(t *testing.T) {
	height0 := float32(0.333)
	pkg := &ColliderPackage{
		Version: 1000,
		Colliders: []*ColliderRef{
			{
				Type: 1,
				Collider: &ColliderCapsule{
					ColliderObject: ColliderObject{
						Version:       1000,
						ParentName:    payloadTestString("Bip01 Head"),
						SelfName:      payloadTestString("Collider"),
						LocalPosition: Vector3{X: 1},
						LocalRotation: Vector4{W: 1},
						LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
						Center:        Vector3{Y: 0.5},
						Bound:         ColliderBoundOutside,
					},
					Direction:   VectorTypeY,
					StartRadius: 0.1,
					EndRadius:   0.1,
					Height:      0.2,
				},
			},
		},
		LimbEnableList: []*ColliderState{{Version: 1000, LimbType: 0, IsEnable: true}},
	}
	encoded, err := EncodeKCESPayload(pkg, ".dbcol")
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	decoded, err := DecodeKCESPayload(encoded, ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	decodedPackage, ok := decoded.(*ColliderPackage)
	if !ok || decodedPackage == nil {
		t.Fatalf("unexpected decoded collider package: %#v", decoded)
	}
	collider, ok := decodedPackage.Colliders[0].Collider.(*ColliderCapsule)
	if !ok {
		t.Fatalf("unexpected collider type: %T", decodedPackage.Colliders[0].Collider)
	}
	if collider.ParentName == nil || *collider.ParentName != "Bip01 Head" {
		t.Fatalf("unexpected collider: %+v", decodedPackage.Colliders[0])
	}

	jsonData, err := json.Marshal(decodedPackage)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(jsonData), `"limbEnableList"`) || strings.Contains(string(jsonData), `"states"`) {
		t.Fatalf("collider package JSON should use limbEnableList, got %s", string(jsonData))
	}
	var fromJSON ColliderPackage
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(fromJSON.LimbEnableList) != 1 {
		t.Fatalf("limb enable list was not preserved after JSON round-trip: %+v", fromJSON)
	}
	if fromCollider, ok := fromJSON.Colliders[0].Collider.(*ColliderCapsule); ok {
		fromCollider.Height = height0
	} else {
		t.Fatalf("unexpected collider type: %T", fromJSON.Colliders[0].Collider)
	}

	legacyJSON := []byte(`{"version":1000,"colliders":[],"states":[{"version":1000,"index":7,"enabled":true}]}`)
	var legacy ColliderPackage
	if err := json.Unmarshal(legacyJSON, &legacy); err == nil {
		t.Fatalf("legacy states JSON should be rejected: %+v", legacy)
	}
	if _, err := EncodeKCESPayload(&fromJSON, ".dbcol"); err != nil {
		t.Fatalf("EncodeKCESPayload from JSON root: %v", err)
	}
}

func TestColliderPayloadPreservesCLRNonFiniteSingleConversions(t *testing.T) {
	collider := colliderCapsuleIndexedTestValue(0)
	// MessagePackReader.ReadSingle converts all numeric markers through CLR
	// Single semantics: a finite double can overflow to infinity, while NaN and
	// explicit infinities remain valid floating results.
	collider[10] = math.MaxFloat64
	collider[11] = math.NaN()
	collider[12] = math.Inf(-1)
	raw := []interface{}{
		int64(0),
		[]interface{}{[]interface{}{int64(ColliderTypeCapsule), collider}},
		nil,
	}
	msgpackData, err := msgpack.EncodeMsgpack(raw)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatal(err)
	}

	root, err := DecodeKCESPayload(AddLengthPrefix(compressed), ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	pkg, ok := root.(*ColliderPackage)
	if !ok || pkg == nil {
		t.Fatalf("decoded payload root = %#v", root)
	}
	decoded, ok := pkg.Colliders[0].Collider.(*ColliderCapsule)
	if !ok {
		t.Fatalf("decoded collider type = %T", pkg.Colliders[0].Collider)
	}
	if !math.IsInf(float64(decoded.StartRadius), 1) || !math.IsNaN(float64(decoded.EndRadius)) || !math.IsInf(float64(decoded.Height), -1) {
		t.Fatalf("non-finite conversions changed: start=%v end=%v height=%v", decoded.StartRadius, decoded.EndRadius, decoded.Height)
	}

	reencoded, err := EncodeKCESPayload(pkg, ".dbcol")
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	again, err := DecodeKCESPayload(reencoded, ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload(re-encoded): %v", err)
	}
	againCollider := again.(*ColliderPackage).Colliders[0].Collider.(*ColliderCapsule)
	if !math.IsInf(float64(againCollider.StartRadius), 1) || !math.IsNaN(float64(againCollider.EndRadius)) || !math.IsInf(float64(againCollider.Height), -1) {
		t.Fatalf("non-finite values changed after re-encode: %+v", againCollider)
	}
}

func TestGroupedColliderPayloadRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name      string
		extension string
		root      any
	}{
		{
			name:      "limbcol",
			extension: ".limbcol",
			root: &LimbColliderPackage{
				Version: 1000,
				Items: []*LimbColliderItem{{
					Version: 1000,
					Target:  0,
					Collider: &ColliderMaidProp{
						ColliderObject: ColliderObject{
							Version:       1002,
							ParentName:    payloadTestString("Bip01 L UpperArm"),
							SelfName:      payloadTestString("Arm"),
							LocalRotation: Vector4{W: 1},
							LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
							Bound:         ColliderBoundOutside,
						},
						Direction:   VectorTypeX,
						StartRadius: 0.1,
						EndRadius:   0.05,
						Height:      0.2,
					},
				}},
			},
		},
		{
			name:      "ikcol",
			extension: ".ikcol",
			root: &IKColliderPackage{
				Version: 1000,
				Groups: []*IKColliderGroup{{
					Version: 1000,
					Target:  1,
					Colliders: []*ColliderRef{{
						Type: 2,
						Collider: &ColliderSphere{
							ColliderObject: ColliderObject{
								Version:       1000,
								ParentName:    payloadTestString("Bip01 R Hand"),
								SelfName:      payloadTestString("ColliderObject"),
								LocalRotation: Vector4{W: 1},
								LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
								Bound:         ColliderBoundInside,
							},
							Radius: 0.02,
						},
					}},
				}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeKCESPayload(tc.root, tc.extension)
			if err != nil {
				t.Fatalf("EncodeKCESPayload: %v", err)
			}
			decoded, err := DecodeKCESPayload(encoded, tc.extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload: %v", err)
			}
			if !reflect.DeepEqual(decoded, tc.root) {
				t.Fatalf("round trip changed the payload root:\n got  %#v\n want %#v", decoded, tc.root)
			}
		})
	}
}

func TestColliderMaidPropVersionEncodingLayout(t *testing.T) {
	// A newly constructed object uses the current declared Key(0)..Key(24)
	// shape regardless of the opaque stored version value.
	value := &ColliderMaidProp{
		ColliderObject:         ColliderObject{Version: 1001},
		CenterMpnList:          []int32{7},
		StartRadiusMpnList:     []int32{40, 41},
		EndRadiusMpnList:       []int32{42, 43},
		CenterMpnNameList:      []*string{payloadTestString("center")},
		StartRadiusMpnNameList: []*string{payloadTestString("start")},
		EndRadiusMpnNameList:   []*string{payloadTestString("end")},
	}
	pkg := &LimbColliderPackage{Items: []*LimbColliderItem{{
		Collider: value,
	}}}
	wire, err := EncodeKCESPayload(pkg, ".limbcol")
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	root := decodeLengthPrefixedIndexedTestArray(t, wire)
	items := decodeIndexedTestArray(t, root[1])
	item := decodeIndexedTestArray(t, items[0])
	status := decodeIndexedTestArray(t, item[2])
	if len(status) != 25 {
		t.Fatalf("stored version 1001 selected width %d, want declared width 25", len(status))
	}
	for _, slot := range []int32{13, 14, 15} {
		assertRawNil(t, status[slot], "MaidProp sparse slot")
	}
	decoded, err := DecodeKCESPayload(wire, ".limbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	got := decoded.(*LimbColliderPackage).Items[0].Collider
	if got.Version != 1001 || !reflect.DeepEqual(got.CenterMpnNameList, value.CenterMpnNameList) ||
		!reflect.DeepEqual(got.StartRadiusMpnNameList, value.StartRadiusMpnNameList) ||
		!reflect.DeepEqual(got.EndRadiusMpnNameList, value.EndRadiusMpnNameList) {
		t.Fatalf("version-driven field migration occurred: %+v", got)
	}
}

func TestNormalizeKCESPayloadExtension(t *testing.T) {
	tests := map[string]string{
		"default_hairf.db2conf":     ".db2conf",
		"maidIKCollider.ikcol":      ".ikcol",
		"ik_collider.ikcol.bytes":   ".ikcol.bytes",
		"default_sleeve_col.dslcol": ".dslcol",
		"crc2_Underwear.undressdat": "",
		"Uwagi.hitcheck":            "",
	}
	for input, want := range tests {
		if got := NormalizeKCESPayloadExtension(input); got != want {
			t.Fatalf("NormalizeKCESPayloadExtension(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestNormalizeKCESJSONTextExtension(t *testing.T) {
	// .undressdat 与 .undresspdat 的原生文件同样是 JSON 文本，但它们的领域结构已被建模，
	// 因此不在自由形式 JSON-text 注册表里
	// The native .undressdat and .undresspdat files are JSON text as well, but their domain
	// structure is modeled, so they are not in the free-form JSON-text registry
	tests := map[string]string{
		"dance_enabled_list.NSON":    ".nson",
		"crc2_Underwear.undressdat":  "",
		"crc2_Underwear.undresspdat": "",
		"Uwagi.hitcheck":             "",
		"default_hairf.db2conf":      "",
	}
	for input, want := range tests {
		if got := NormalizeKCESJSONTextExtension(input); got != want {
			t.Fatalf("NormalizeKCESJSONTextExtension(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestNormalizeKCESUnityJSONDocumentExtension(t *testing.T) {
	tests := map[string]string{
		"crc2_Underwear.undressdat":  ".undressdat",
		"crc2_Underwear.UNDRESSPDAT": ".undresspdat",
		"dance_enabled_list.nson":    "",
		"Uwagi.hitcheck":             "",
		"default_hairf.db2conf":      "",
	}
	for input, want := range tests {
		if got := NormalizeKCESUnityJSONDocumentExtension(input); got != want {
			t.Fatalf("NormalizeKCESUnityJSONDocumentExtension(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestKCESPayloadDescriptorsCoverExtensions(t *testing.T) {
	expected := map[string]struct {
		kind string
	}{
		KCESDBConfExtension:     {kind: PayloadKindDynamicBoneStatus},
		KCESDBColExtension:      {kind: PayloadKindColliderPackage},
		KCESDB2ConfExtension:    {kind: PayloadKindJSONString},
		KCESDSBConfExtension:    {kind: PayloadKindClothParams},
		KCESDSB2ConfExtension:   {kind: PayloadKindJSONString},
		KCESDSLConfExtension:    {kind: PayloadKindClothParams},
		KCESDSL2ConfExtension:   {kind: PayloadKindJSONString},
		KCESDSLColExtension:     {kind: PayloadKindColliderPackage},
		KCESIKColExtension:      {kind: PayloadKindIKCollider},
		KCESIKColBytesExtension: {kind: PayloadKindIKCollider},
		KCESLimbColExtension:    {kind: PayloadKindLimbCollider},
	}
	if len(kcesPayloadDescriptorByExtension) != len(expected) {
		t.Fatalf("payload descriptor count = %d, want %d", len(kcesPayloadDescriptorByExtension), len(expected))
	}
	for extension, want := range expected {
		descriptor, ok := kcesPayloadDescriptorByExtension[extension]
		if !ok {
			t.Errorf("missing payload descriptor for %s", extension)
			continue
		}
		if descriptor.Extension != extension || descriptor.Kind != want.kind || !descriptor.LengthPrefixed {
			t.Errorf("descriptor %s = %+v, want kind=%q and lengthPrefixed=true", extension, descriptor, want.kind)
		}
		if got := NormalizeKCESPayloadExtension("folder/sample" + extension); got != extension {
			t.Errorf("NormalizeKCESPayloadExtension(%s) = %q", extension, got)
		}
	}
}

// payloadKindsForRootTypeChecks 列出全部原生载荷类型，用于验证扩展名只接受自己声明的根类型
// payloadKindsForRootTypeChecks lists every native payload kind, used to verify each extension only accepts the root type it declares
var payloadKindsForRootTypeChecks = []string{
	PayloadKindDynamicBoneStatus,
	PayloadKindColliderPackage,
	PayloadKindLimbCollider,
	PayloadKindIKCollider,
	PayloadKindClothParams,
	PayloadKindJSONString,
}

func TestEncodeKCESPayloadEnforcesExtensionRootTypeContract(t *testing.T) {
	for _, descriptor := range kcesPayloadDescriptors {
		descriptor := descriptor
		t.Run(strings.TrimPrefix(descriptor.Extension, "."), func(t *testing.T) {
			if _, err := EncodeKCESPayload(newPayloadRootForKind(descriptor.Kind), descriptor.Extension); err != nil {
				t.Fatalf("declared payload root was rejected: %v", err)
			}

			t.Run("nil root", func(t *testing.T) {
				wire, err := EncodeKCESPayload(nil, descriptor.Extension)
				if err != nil {
					t.Fatalf("EncodeKCESPayload rejected a nil payload root: %v", err)
				}
				decoded, err := DecodeKCESPayload(wire, descriptor.Extension)
				if err != nil || !payloadRootIsNil(decoded) {
					t.Fatalf("nil payload root round trip: root=%+v error=%v", decoded, err)
				}
			})

			for _, kind := range payloadKindsForRootTypeChecks {
				if kind == descriptor.Kind {
					continue
				}
				kind := kind
				t.Run("foreign root "+kind, func(t *testing.T) {
					if _, err := EncodeKCESPayload(newPayloadRootForKind(kind), descriptor.Extension); err == nil {
						t.Fatalf("EncodeKCESPayload accepted a %s root", kind)
					}
				})
			}

			t.Run("foreign root type", func(t *testing.T) {
				if _, err := EncodeKCESPayload(map[string]any{}, descriptor.Extension); err == nil {
					t.Fatal("EncodeKCESPayload accepted a root type no extension declares")
				}
			})
		})
	}
}

// newPayloadRootForKind 为一个原生载荷类型创建可编码的最小根对象
// newPayloadRootForKind creates a minimal encodable root object for one native payload kind
func newPayloadRootForKind(kind string) any {
	switch kind {
	case PayloadKindDynamicBoneStatus:
		return NewDynamicBoneStatus()
	case PayloadKindColliderPackage:
		return &ColliderPackage{Version: 1000}
	case PayloadKindLimbCollider:
		return &LimbColliderPackage{Version: 1000}
	case PayloadKindIKCollider:
		return &IKColliderPackage{Version: 1000}
	case PayloadKindClothParams:
		return NewClothParams()
	case PayloadKindJSONString:
		return &MagicaClothSerializeData{}
	default:
		panic("unsupported native payload kind " + kind)
	}
}

// payloadRootIsNil 判断载荷根是否为 nil 或类型化 nil 指针，两者都表示 MessagePack 根值为 nil
// payloadRootIsNil reports whether a payload root is nil or a typed nil pointer, both of which represent a nil MessagePack root value
func payloadRootIsNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Ptr && reflected.IsNil()
}

func payloadTestString(value string) *string { return &value }

func TestKCESJSONTextDescriptorsCoverExtensions(t *testing.T) {
	expected := []string{
		KCESNSONExtension,
	}
	if len(kcesJSONTextDescriptorByExtension) != len(expected) {
		t.Fatalf("JSON-text descriptor count = %d, want %d", len(kcesJSONTextDescriptorByExtension), len(expected))
	}
	for _, extension := range expected {
		descriptor, ok := kcesJSONTextDescriptorByExtension[extension]
		if !ok || descriptor.Extension != extension {
			t.Errorf("JSON-text descriptor %s = %+v, present=%v", extension, descriptor, ok)
		}
		if got := NormalizeKCESJSONTextExtension("folder/sample" + extension); got != extension {
			t.Errorf("NormalizeKCESJSONTextExtension(%s) = %q", extension, got)
		}
	}
}

func TestKCESMessagePackPayloadRejectsExtensionKindMismatch(t *testing.T) {
	_, err := EncodeKCESPayload(NewClothParams(), KCESDBConfExtension)
	if err == nil || !strings.Contains(err.Error(), "requires payload kind") ||
		!strings.Contains(err.Error(), PayloadKindDynamicBoneStatus) {
		t.Fatalf("EncodeKCESPayload mismatch error = %v, want extension/kind rejection", err)
	}
}

func TestClothParamsFileRejectsUnrelatedExtensions(t *testing.T) {
	for _, extension := range []string{KCESDBConfExtension, ".unknown"} {
		t.Run(extension, func(t *testing.T) {
			if _, err := EncodeClothParamsFile(NewClothParams(), extension); err == nil || !strings.Contains(err.Error(), "unsupported ClothParams extension") {
				t.Fatalf("EncodeClothParamsFile(%q) error = %v, want extension rejection", extension, err)
			}
			if _, err := DecodeClothParamsFile(nil, extension); err == nil || !strings.Contains(err.Error(), "unsupported ClothParams extension") {
				t.Fatalf("DecodeClothParamsFile(%q) error = %v, want extension rejection", extension, err)
			}
		})
	}
}
