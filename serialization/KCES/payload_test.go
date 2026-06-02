package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestDynamicBoneStatusPayloadRoundTrip(t *testing.T) {
	status := &DynamicBoneStatus{
		Version:   1000,
		Damping:   0.6,
		Gravity:   Vector3{Y: -0.05},
		EndOffset: Vector3{X: 1, Y: 2, Z: 3},
		DampingKeyFrames: []DynamicBoneAnimationFrame{
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

func TestJSONStringPayloadRoundTrip(t *testing.T) {
	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ".db2conf",
		LengthPrefixed: true,
		Kind:           PayloadKindJSONString,
		JSON:           json.RawMessage(`{"clothType":1,"rootRotation":0.5}`),
	}
	encoded, err := EncodeKCESPayload(env)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	decoded, err := DecodeKCESPayload(encoded, ".db2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	if decoded.Kind != PayloadKindJSONString || decoded.Text != `{"clothType":1,"rootRotation":0.5}` {
		t.Fatalf("unexpected decoded JSON string payload: %+v", decoded)
	}
	if !bytes.Equal(decoded.JSON, []byte(`{"clothType":1,"rootRotation":0.5}`)) {
		t.Fatalf("unexpected compact json: %s", decoded.JSON)
	}
}

func TestRawMsgpackPayloadRoundTrip(t *testing.T) {
	msgpackData, err := ct.EncodeMsgpack([]interface{}{int64(1000), []interface{}{"union-like", uint64(42)}})
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}
	compressed, err := ct.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatalf("CompressLz4BlockArray: %v", err)
	}
	input := AddLengthPrefix(compressed)

	env, err := DecodeKCESPayload(input, ".unknown")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	if env.Kind != PayloadKindRawMsgpack {
		t.Fatalf("kind got %q", env.Kind)
	}
	if env.MsgpackBase64 != base64.StdEncoding.EncodeToString(msgpackData) {
		t.Fatalf("raw msgpack was not preserved")
	}
	if len(env.MsgpackJSONPreview) == 0 {
		t.Fatalf("expected JSON preview")
	}

	out, err := EncodeKCESPayload(env)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	outPayload, prefixed, err := StripLengthPrefix(out)
	if err != nil {
		t.Fatal(err)
	}
	if !prefixed {
		t.Fatalf("expected length prefix")
	}
	decodedMsgpack, err := ct.DecompressLz4BlockArray(outPayload)
	if err != nil {
		t.Fatalf("DecompressLz4BlockArray: %v", err)
	}
	if !bytes.Equal(decodedMsgpack, msgpackData) {
		t.Fatalf("msgpack changed after raw round-trip")
	}
}

func TestClothParamsPayloadRoundTrip(t *testing.T) {
	params := &ClothParams{
		Radius:                           BezierParam{StartValue: 0.02, EndValue: 0.04, UseEndValue: true},
		Mass:                             BezierParam{StartValue: 1, EndValue: 1},
		UseGravity:                       true,
		Gravity:                          BezierParam{StartValue: -9.8, EndValue: -9.8},
		UseDrag:                          true,
		Drag:                             BezierParam{StartValue: 0.02, EndValue: 0.02, UseEndValue: true},
		UseMaxVelocity:                   true,
		MaxVelocity:                      BezierParam{StartValue: 3, EndValue: 3},
		WorldMoveInfluence:               BezierParam{StartValue: 0.5, EndValue: 0.5},
		WorldRotationInfluence:           BezierParam{StartValue: 0.5, EndValue: 0.5},
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
		ClampPositionLength:              BezierParam{StartValue: 0.03, EndValue: 0.2, UseEndValue: true},
		ClampPositionRatioX:              1,
		ClampPositionRatioY:              1,
		ClampPositionRatioZ:              1,
		ClampPositionVelocityInfluence:   0.2,
		ClampRotationAngle:               BezierParam{StartValue: 30, EndValue: 30, UseEndValue: true},
		ClampRotationVelocityInfluence:   0.2,
		RestoreDistanceVelocityInfluence: 1,
		StructDistanceStiffness:          BezierParam{StartValue: 1, EndValue: 1},
		BendDistanceMaxCount:             2,
		BendDistanceStiffness:            BezierParam{StartValue: 0.5, EndValue: 0.5},
		NearDistanceMaxCount:             3,
		NearDistanceMaxDepth:             1,
		NearDistanceLength:               BezierParam{StartValue: 0.1, EndValue: 0.1, UseEndValue: true},
		NearDistanceStiffness:            BezierParam{StartValue: 0.3, EndValue: 0.3},
		RestoreRotation:                  BezierParam{StartValue: 0.3, EndValue: 0.1, UseEndValue: true},
		SpringPower:                      0.017,
		SpringRadius:                     0.1,
		SpringScaleX:                     1,
		SpringScaleY:                     1,
		SpringScaleZ:                     1,
		SpringIntensity:                  1,
		SpringDirectionAtten:             BezierParam{StartValue: 1, EndValue: 0, UseEndValue: true, CurveValue: 0.234, UseCurveValue: true},
		SpringDistanceAtten:              BezierParam{StartValue: 1, EndValue: 0, UseEndValue: true, CurveValue: 0.395, UseCurveValue: true},
		AdjustRotationPower:              5,
		TriangleBend:                     BezierParam{StartValue: 0.5, EndValue: 0.5, UseEndValue: true},
		MaxVolumeLength:                  0.1,
		VolumeStretchStiffness:           BezierParam{StartValue: 0.5, EndValue: 0.5, UseEndValue: true},
		VolumeShearStiffness:             BezierParam{StartValue: 0.5, EndValue: 0.5, UseEndValue: true},
		Friction:                         0.2,
		UsePenetration:                   true,
		PenetrationMode:                  ClothPenetrationModeColliderPenetration,
		PenetrationAxis:                  ClothPenetrationAxisInverseZ,
		PenetrationMaxDepth:              1,
		PenetrationConnectDistance:       BezierParam{StartValue: 0.2, EndValue: 0.3, UseEndValue: true},
		PenetrationDistance:              BezierParam{StartValue: 0.1, EndValue: 0.2, UseEndValue: true},
		PenetrationRadius:                BezierParam{StartValue: 0.3, EndValue: 1, UseEndValue: true},
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
	if decoded.Kind != PayloadKindClothParams || decoded.ClothParams == nil {
		t.Fatalf("unexpected decoded cloth params: %+v", decoded)
	}
	if decoded.ClothParams.PenetrationMode != ClothPenetrationModeColliderPenetration {
		t.Fatalf("penetration mode got %d", decoded.ClothParams.PenetrationMode)
	}
}

func TestColliderPackagePayloadRoundTrip(t *testing.T) {
	tail0 := int64(0)
	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ".dbcol",
		LengthPrefixed: true,
		Kind:           PayloadKindColliderPackage,
		ColliderPackage: &ColliderPackage{
			Version: 1000,
			Colliders: []ColliderRef{
				{
					Type: 1,
					Collider: ColliderObject{
						Version:       1000,
						ParentName:    "Bip01 Head",
						SelfName:      "Collider",
						LocalPosition: Vector3{X: 1},
						LocalRotation: Vector4{W: 1},
						LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
						Center:        Vector3{Y: 0.5},
						Tail:          []interface{}{int64(0), int64(0), false, 0.1, 0.1, 0.2},
					},
				},
			},
			States: []ColliderState{{Version: 1000, Index: 0, Enabled: true}},
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
	if decoded.Kind != PayloadKindColliderPackage || decoded.ColliderPackage == nil {
		t.Fatalf("unexpected decoded collider package: %+v", decoded)
	}
	if decoded.ColliderPackage.Colliders[0].Collider.ParentName != "Bip01 Head" {
		t.Fatalf("unexpected collider: %+v", decoded.ColliderPackage.Colliders[0])
	}

	jsonData, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fromJSON KCESPayloadEnvelope
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fromJSON.ColliderPackage.Colliders[0].Collider.Tail = append(fromJSON.ColliderPackage.Colliders[0].Collider.Tail, tail0)
	if _, err := EncodeKCESPayload(&fromJSON); err != nil {
		t.Fatalf("EncodeKCESPayload from JSON envelope: %v", err)
	}
}

func TestGroupedColliderPayloadRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  *KCESPayloadEnvelope
	}{
		{
			name: "limbcol",
			env: &KCESPayloadEnvelope{
				Format:         PayloadFormatKCESMessagePack,
				Extension:      ".limbcol",
				LengthPrefixed: true,
				Kind:           PayloadKindLimbCollider,
				LimbCollider: &LimbColliderPackage{
					Version: 1000,
					Items: []LimbColliderItem{{
						Version: 1000,
						Target:  0,
						Collider: ColliderObject{
							Version:       1001,
							ParentName:    "Bip01 L UpperArm",
							SelfName:      "Arm",
							LocalRotation: Vector4{W: 1},
							LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
							Tail:          []interface{}{int64(0), int64(0), true, 0.1, 0.05, 0.2},
						},
					}},
				},
			},
		},
		{
			name: "ikcol",
			env: &KCESPayloadEnvelope{
				Format:         PayloadFormatKCESMessagePack,
				Extension:      ".ikcol",
				LengthPrefixed: true,
				Kind:           PayloadKindIKCollider,
				IKCollider: &IKColliderPackage{
					Version: 1000,
					Groups: []IKColliderGroup{{
						Version: 1000,
						Target:  1,
						Colliders: []ColliderRef{{
							Type: 2,
							Collider: ColliderObject{
								Version:       1000,
								ParentName:    "Bip01 R Hand",
								SelfName:      "ColliderObject",
								LocalRotation: Vector4{W: 1},
								LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
								Tail:          []interface{}{int64(0), 0.02},
							},
						}},
					}},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeKCESPayload(tc.env)
			if err != nil {
				t.Fatalf("EncodeKCESPayload: %v", err)
			}
			decoded, err := DecodeKCESPayload(encoded, tc.env.Extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload: %v", err)
			}
			if decoded.Kind != tc.env.Kind {
				t.Fatalf("kind got %q, want %q", decoded.Kind, tc.env.Kind)
			}
		})
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
	tests := map[string]string{
		"crc2_Underwear.undressdat":  ".undressdat",
		"crc2_Underwear.undresspdat": ".undresspdat",
		"Uwagi.hitcheck":             "",
		"default_hairf.db2conf":      "",
	}
	for input, want := range tests {
		if got := NormalizeKCESJSONTextExtension(input); got != want {
			t.Fatalf("NormalizeKCESJSONTextExtension(%q)=%q, want %q", input, got, want)
		}
	}
}
