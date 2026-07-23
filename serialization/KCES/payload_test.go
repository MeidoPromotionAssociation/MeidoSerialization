package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestDecodeKCESPayloadRequiresGameLengthPrefix(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "kces_payload", "default_hairf.dbconf")
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

func TestRecognizedKCESPayloadNilRootRoundTrip(t *testing.T) {
	tail := []byte{0xde, 0xad, 0xc1}
	compressed, err := ct.CompressLz4BlockArray(append([]byte{0xc0}, tail...))
	if err != nil {
		t.Fatalf("compress nil payload root: %v", err)
	}
	wire := AddLengthPrefix(compressed)
	tests := []struct {
		extension string
		kind      string
	}{
		{extension: ".dbconf", kind: PayloadKindDynamicBoneStatus},
		{extension: ".db2conf", kind: PayloadKindJSONString},
		{extension: ".dbcol", kind: PayloadKindColliderPackage},
		{extension: ".limbcol", kind: PayloadKindLimbCollider},
		{extension: ".ikcol", kind: PayloadKindIKCollider},
		{extension: ".dsbconf", kind: PayloadKindClothParams},
	}
	for _, test := range tests {
		t.Run(test.extension, func(t *testing.T) {
			env, err := DecodeKCESPayload(wire, test.extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload: %v", err)
			}
			if env.Kind != test.kind || !env.MsgpackRootNil {
				t.Fatalf("decoded nil root kind/rootNil = %q/%v, want %q/true", env.Kind, env.MsgpackRootNil, test.kind)
			}
			if !bytes.Equal(env.MsgpackTrailingData, tail) {
				t.Fatalf("decoded trailing = % x, want % x", env.MsgpackTrailingData, tail)
			}
			reencoded, err := EncodeKCESPayload(env)
			if err != nil {
				t.Fatalf("EncodeKCESPayload: %v", err)
			}
			payload, _, err := StripLengthPrefix(reencoded)
			if err != nil {
				t.Fatalf("StripLengthPrefix: %v", err)
			}
			decompressed, err := ct.DecompressLz4BlockArray(payload)
			if err != nil {
				t.Fatalf("DecompressLz4BlockArray: %v", err)
			}
			want := append([]byte{0xc0}, tail...)
			if !bytes.Equal(decompressed, want) {
				t.Fatalf("re-encoded nil root = % x, want % x", decompressed, want)
			}
		})
	}
}

func TestKCESPayloadNilRootRejectsDiscardedTypedData(t *testing.T) {
	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ".dbconf",
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
		Kind:           PayloadKindDynamicBoneStatus,
		MsgpackRootNil: true,
		DynamicBone:    &DynamicBoneStatus{},
	}
	if _, err := EncodeKCESPayload(env); err == nil {
		t.Fatal("msgpackRootNil silently discarded dynamicBoneStatus")
	}
	env.DynamicBone = nil
	env.Kind = PayloadKindRawMsgpack
	env.MsgpackBase64 = "wA=="
	if _, err := EncodeKCESPayload(env); err == nil {
		t.Fatal("raw-msgpack accepted ambiguous msgpackRootNil")
	}
}

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
	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ".db2conf",
		LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
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

func TestJSONStringPayloadPreservesOriginalTextUntilEdited(t *testing.T) {
	original := "{\r\n  \"clothType\" : 1,\r\n  \"future\" : [ 1, 2 ]\r\n}\r\n"
	msgpack, err := ct.EncodeMsgpack(original)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := ct.CompressLz4BlockArray(msgpack)
	if err != nil {
		t.Fatal(err)
	}
	wire := AddLengthPrefix(compressed)

	envelope, err := DecodeKCESPayload(wire, ".db2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload() error = %v", err)
	}
	if envelope.Text != original || !bytes.Equal(envelope.JSON, []byte(`{"clothType":1,"future":[1,2]}`)) {
		t.Fatalf("decoded text/json = %q / %s", envelope.Text, envelope.JSON)
	}

	unchanged, err := EncodeKCESPayload(envelope)
	if err != nil {
		t.Fatalf("EncodeKCESPayload(unchanged) error = %v", err)
	}
	if !bytes.Equal(unchanged, wire) {
		t.Fatalf("unchanged JSON string was rebuilt:\n got  %x\n want %x", unchanged, wire)
	}

	envelope.JSON = json.RawMessage(` { "clothType" : 2, "future" : null } `)
	edited, err := EncodeKCESPayload(envelope)
	if err != nil {
		t.Fatalf("EncodeKCESPayload(edited) error = %v", err)
	}
	redecoded, err := DecodeKCESPayload(edited, ".db2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload(edited) error = %v", err)
	}
	if redecoded.Text != `{"clothType":2,"future":null}` {
		t.Fatalf("edited JSON string = %q", redecoded.Text)
	}
}

func TestJSONStringPayloadCanExplicitlyStoreJSONNull(t *testing.T) {
	envelope := &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".dsl2conf", LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindJSONString,
		JSON: json.RawMessage(`null`),
	}
	wire, err := EncodeKCESPayload(envelope)
	if err != nil {
		t.Fatalf("EncodeKCESPayload() error = %v", err)
	}
	decoded, err := DecodeKCESPayload(wire, ".dsl2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload() error = %v", err)
	}
	if decoded.Text != "null" || !bytes.Equal(decoded.JSON, []byte("null")) {
		t.Fatalf("decoded null payload = text %q, json %s", decoded.Text, decoded.JSON)
	}
}

func TestRecognizedMessagePackPayloadPreservesTrailingBytes(t *testing.T) {
	root, err := ct.EncodeMsgpack(`{"clothType":1}`)
	if err != nil {
		t.Fatal(err)
	}
	tail := []byte{0xde, 0xad, 0xbe, 0xef, 0xc1}
	decompressed := append(append([]byte(nil), root...), tail...)
	compressed, err := ct.CompressLz4BlockArray(decompressed)
	if err != nil {
		t.Fatal(err)
	}
	wire := AddLengthPrefix(compressed)

	envelope, err := DecodeKCESPayload(wire, ".db2conf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	if !bytes.Equal(envelope.MsgpackTrailingData, tail) {
		t.Fatalf("msgpackTrailingData = % x, want % x", envelope.MsgpackTrailingData, tail)
	}

	reencoded, err := EncodeKCESPayload(envelope)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	reencodedPayload, prefixed, err := StripLengthPrefix(reencoded)
	if err != nil || !prefixed {
		t.Fatalf("StripLengthPrefix: prefixed=%v err=%v", prefixed, err)
	}
	reencodedDecompressed, err := ct.DecompressLz4BlockArray(reencodedPayload)
	if err != nil {
		t.Fatalf("DecompressLz4BlockArray: %v", err)
	}
	if !bytes.Equal(reencodedDecompressed, decompressed) {
		t.Fatalf("recognized payload stream changed:\n got  % x\n want % x", reencodedDecompressed, decompressed)
	}
}

func TestJSONStringPayloadRejectsInvalidInnerJSON(t *testing.T) {
	env := &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".db2conf", LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindJSONString,
		Text: `{not-json}`,
	}
	if _, err := EncodeKCESPayload(env); err == nil || !strings.Contains(err.Error(), "Magica JSON") {
		t.Fatalf("EncodeKCESPayload error = %v, want invalid inner JSON rejection", err)
	}

	msgpack, err := ct.EncodeMsgpack(`{not-json}`)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := ct.CompressLz4BlockArray(msgpack)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCESPayload(AddLengthPrefix(compressed), ".db2conf"); err == nil || !strings.Contains(err.Error(), "Magica JSON") {
		t.Fatalf("DecodeKCESPayload error = %v, want invalid inner JSON rejection", err)
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

func TestRawMsgpackPayloadDoesNotDuplicateTrailingAnnotation(t *testing.T) {
	completeStream := []byte{0x01, 0xde, 0xad, 0xbe, 0xef}
	compressed, err := ct.CompressLz4BlockArray(completeStream)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := DecodeKCESPayload(compressed, ".unknown")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	if envelope.MsgpackTrailingData != nil {
		t.Fatalf("raw payload unexpectedly split trailing bytes: % x", envelope.MsgpackTrailingData)
	}
	if got, err := base64.StdEncoding.DecodeString(envelope.MsgpackBase64); err != nil || !bytes.Equal(got, completeStream) {
		t.Fatalf("raw msgpackBase64 = % x, err=%v", got, err)
	}

	envelope.MsgpackTrailingData = []byte{1}
	if _, err := EncodeKCESPayload(envelope); err == nil || !strings.Contains(err.Error(), "already stores the complete") {
		t.Fatalf("ambiguous raw trailing annotation error = %v", err)
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
	height0 := float32(0.333)
	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ".dbcol",
		LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
		Kind:           PayloadKindColliderPackage,
		ColliderPackage: &ColliderPackage{
			Version: 1000,
			Colliders: []ColliderRef{
				{
					Type: 1,
					Collider: &ColliderCapsule{
						ColliderObject: ColliderObject{
							Version:       1000,
							ParentName:    "Bip01 Head",
							SelfName:      "Collider",
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
			LimbEnableList: []ColliderState{{Version: 1000, LimbType: 0, IsEnable: true}},
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
	collider, ok := decoded.ColliderPackage.Colliders[0].Collider.(*ColliderCapsule)
	if !ok {
		t.Fatalf("unexpected collider type: %T", decoded.ColliderPackage.Colliders[0].Collider)
	}
	if collider.ParentName != "Bip01 Head" {
		t.Fatalf("unexpected collider: %+v", decoded.ColliderPackage.Colliders[0])
	}

	jsonData, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(jsonData), `"limbEnableList"`) || strings.Contains(string(jsonData), `"states"`) {
		t.Fatalf("collider package JSON should use limbEnableList, got %s", string(jsonData))
	}
	var fromJSON KCESPayloadEnvelope
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(fromJSON.ColliderPackage.LimbEnableList) != 1 {
		t.Fatalf("limb enable list was not preserved after JSON round-trip: %+v", fromJSON.ColliderPackage)
	}
	if fromCollider, ok := fromJSON.ColliderPackage.Colliders[0].Collider.(*ColliderCapsule); ok {
		fromCollider.Height = height0
	} else {
		t.Fatalf("unexpected collider type: %T", fromJSON.ColliderPackage.Colliders[0].Collider)
	}

	legacyJSON := []byte(`{"version":1000,"colliders":[],"states":[{"version":1000,"index":7,"enabled":true}]}`)
	var legacy ColliderPackage
	if err := json.Unmarshal(legacyJSON, &legacy); err == nil {
		t.Fatalf("legacy states JSON should be rejected: %+v", legacy)
	}
	if _, err := EncodeKCESPayload(&fromJSON); err != nil {
		t.Fatalf("EncodeKCESPayload from JSON envelope: %v", err)
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
	msgpack, err := ct.EncodeMsgpack(raw)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := ct.CompressLz4BlockArray(msgpack)
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := DecodeKCESPayload(AddLengthPrefix(compressed), ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	decoded, ok := envelope.ColliderPackage.Colliders[0].Collider.(*ColliderCapsule)
	if !ok {
		t.Fatalf("decoded collider type = %T", envelope.ColliderPackage.Colliders[0].Collider)
	}
	if !math.IsInf(float64(decoded.StartRadius), 1) || !math.IsNaN(float64(decoded.EndRadius)) || !math.IsInf(float64(decoded.Height), -1) {
		t.Fatalf("non-finite conversions changed: start=%v end=%v height=%v", decoded.StartRadius, decoded.EndRadius, decoded.Height)
	}

	reencoded, err := EncodeKCESPayload(envelope)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	again, err := DecodeKCESPayload(reencoded, ".dbcol")
	if err != nil {
		t.Fatalf("DecodeKCESPayload(re-encoded): %v", err)
	}
	againCollider := again.ColliderPackage.Colliders[0].Collider.(*ColliderCapsule)
	if !math.IsInf(float64(againCollider.StartRadius), 1) || !math.IsNaN(float64(againCollider.EndRadius)) || !math.IsInf(float64(againCollider.Height), -1) {
		t.Fatalf("non-finite values changed after re-encode: %+v", againCollider)
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
				StorageVariant: PayloadStorageInt32LZ4MessagePack,
				Kind:           PayloadKindLimbCollider,
				LimbCollider: &LimbColliderPackage{
					Version: 1000,
					Items: []LimbColliderItem{{
						Version: 1000,
						Target:  0,
						Collider: &ColliderMaidProp{
							ColliderObject: ColliderObject{
								Version:       1002,
								ParentName:    "Bip01 L UpperArm",
								SelfName:      "Arm",
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
		},
		{
			name: "ikcol",
			env: &KCESPayloadEnvelope{
				Format:         PayloadFormatKCESMessagePack,
				Extension:      ".ikcol",
				LengthPrefixed: true,
				StorageVariant: PayloadStorageInt32LZ4MessagePack,
				Kind:           PayloadKindIKCollider,
				IKCollider: &IKColliderPackage{
					Version: 1000,
					Groups: []IKColliderGroup{{
						Version: 1000,
						Target:  1,
						Colliders: []ColliderRef{{
							Type: 2,
							Collider: &ColliderSphere{
								ColliderObject: ColliderObject{
									Version:       1000,
									ParentName:    "Bip01 R Hand",
									SelfName:      "ColliderObject",
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

func TestColliderMaidPropVersionEncodingLayout(t *testing.T) {
	// A newly constructed object uses the current declared Key(0)..Key(24)
	// shape regardless of the opaque stored version value.
	value := &ColliderMaidProp{
		ColliderObject:         ColliderObject{Version: 1001},
		CenterMpnList:          []int32{7},
		StartRadiusMpnList:     []int32{40, 41},
		EndRadiusMpnList:       []int32{42, 43},
		CenterMpnNameList:      []string{"center"},
		StartRadiusMpnNameList: []string{"start"},
		EndRadiusMpnNameList:   []string{"end"},
	}
	envelope := &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".limbcol", LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindLimbCollider,
		LimbCollider: &LimbColliderPackage{Items: []LimbColliderItem{{
			Collider: value,
		}}},
	}
	wire, err := EncodeKCESPayload(envelope)
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
	got := decoded.LimbCollider.Items[0].Collider.(*ColliderMaidProp)
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
	tests := map[string]string{
		"crc2_Underwear.undressdat":  ".undressdat",
		"crc2_Underwear.undresspdat": ".undresspdat",
		"dance_enabled_list.NSON":    ".nson",
		"Uwagi.hitcheck":             "",
		"default_hairf.db2conf":      "",
	}
	for input, want := range tests {
		if got := NormalizeKCESJSONTextExtension(input); got != want {
			t.Fatalf("NormalizeKCESJSONTextExtension(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestKCESPayloadDescriptorsCoverExtensions(t *testing.T) {
	expected := map[string]struct {
		kind          string
		exportKind    string
		exportStorage string
	}{
		KCESDBConfExtension:     {kind: PayloadKindDynamicBoneStatus, exportKind: PayloadKindExportCMDynamicBoneJSON, exportStorage: PayloadStorageExportCMUnityJSON},
		KCESDBColExtension:      {kind: PayloadKindColliderPackage, exportKind: PayloadKindExportCMColliderJSON, exportStorage: PayloadStorageExportCMUnityJSON},
		KCESDB2ConfExtension:    {kind: PayloadKindJSONString},
		KCESDSBConfExtension:    {kind: PayloadKindClothParams},
		KCESDSB2ConfExtension:   {kind: PayloadKindJSONString},
		KCESDSLConfExtension:    {kind: PayloadKindClothParams},
		KCESDSL2ConfExtension:   {kind: PayloadKindJSONString},
		KCESDSLColExtension:     {kind: PayloadKindColliderPackage, exportKind: PayloadKindExportCMColliderJSON, exportStorage: PayloadStorageExportCMDotNetStringJSON},
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
		if descriptor.ExportCMKind != want.exportKind || descriptor.ExportCMStorageVariant != want.exportStorage {
			t.Errorf("descriptor %s ExportCM = %q/%q, want %q/%q", extension, descriptor.ExportCMKind, descriptor.ExportCMStorageVariant, want.exportKind, want.exportStorage)
		}
		if got := NormalizeKCESPayloadExtension("folder/sample" + extension); got != extension {
			t.Errorf("NormalizeKCESPayloadExtension(%s) = %q", extension, got)
		}
	}
}

func TestEncodeKCESPayloadRejectsDescriptorContractViolations(t *testing.T) {
	for _, descriptor := range kcesPayloadDescriptors {
		descriptor := descriptor
		t.Run(strings.TrimPrefix(descriptor.Extension, "."), func(t *testing.T) {
			if _, err := EncodeKCESPayload(newDescriptorNativeEnvelope(descriptor)); err != nil {
				t.Fatalf("valid native tuple: %v", err)
			}

			for _, mismatch := range []struct {
				name   string
				mutate func(*KCESPayloadEnvelope)
			}{
				{name: "format", mutate: func(env *KCESPayloadEnvelope) { env.Format = PayloadFormatKCESExportCM }},
				{name: "storage", mutate: func(env *KCESPayloadEnvelope) { env.StorageVariant = "wrong-storage" }},
				{name: "kind", mutate: func(env *KCESPayloadEnvelope) { env.Kind = PayloadKindRawMsgpack }},
				{name: "length prefix", mutate: func(env *KCESPayloadEnvelope) { env.LengthPrefixed = !descriptor.LengthPrefixed }},
			} {
				t.Run("tuple "+mismatch.name, func(t *testing.T) {
					envelope := newDescriptorNativeEnvelope(descriptor)
					mismatch.mutate(envelope)
					assertPayloadEncodeRejected(t, envelope)
				})
			}

			t.Run("missing active root", func(t *testing.T) {
				envelope := newDescriptorNativeEnvelope(descriptor)
				clearDescriptorActiveRoot(envelope, descriptor.Kind)
				assertPayloadEncodeRejected(t, envelope)
			})

			t.Run("inactive typed root", func(t *testing.T) {
				envelope := newDescriptorNativeEnvelope(descriptor)
				setDescriptorInactiveTypedRoot(envelope, descriptor.Kind)
				assertPayloadEncodeRejected(t, envelope)
			})

			if descriptor.Kind != PayloadKindJSONString {
				for _, field := range []struct {
					name   string
					mutate func(*KCESPayloadEnvelope)
				}{
					{name: "text", mutate: func(env *KCESPayloadEnvelope) { env.Text = "{}" }},
					{name: "json", mutate: func(env *KCESPayloadEnvelope) { env.JSON = json.RawMessage(`{}`) }},
				} {
					t.Run("inactive "+field.name, func(t *testing.T) {
						envelope := newDescriptorNativeEnvelope(descriptor)
						field.mutate(envelope)
						assertPayloadEncodeRejected(t, envelope)
					})
				}
			}

			for _, field := range []struct {
				name   string
				mutate func(*KCESPayloadEnvelope)
			}{
				{name: "msgpackBase64", mutate: func(env *KCESPayloadEnvelope) { env.MsgpackBase64 = "wA==" }},
				{name: "msgpackJsonPreview", mutate: func(env *KCESPayloadEnvelope) { env.MsgpackJSONPreview = json.RawMessage(`null`) }},
			} {
				t.Run("inactive "+field.name, func(t *testing.T) {
					envelope := newDescriptorNativeEnvelope(descriptor)
					field.mutate(envelope)
					assertPayloadEncodeRejected(t, envelope)
				})
			}

			t.Run("nil root with typed root", func(t *testing.T) {
				envelope := newDescriptorNativeEnvelope(descriptor)
				envelope.MsgpackRootNil = true
				assertPayloadEncodeRejected(t, envelope)
			})

			if descriptor.ExportCMKind != "" {
				exportEnvelope := newDescriptorExportCMEnvelope(descriptor)
				if _, err := EncodeKCESPayload(exportEnvelope); err != nil {
					t.Fatalf("valid ExportCM tuple: %v", err)
				}
				for _, field := range []struct {
					name   string
					mutate func(*KCESPayloadEnvelope)
				}{
					{name: "typed root", mutate: func(env *KCESPayloadEnvelope) { env.DynamicBone = NewDynamicBoneStatus() }},
					{name: "trailing MessagePack", mutate: func(env *KCESPayloadEnvelope) { env.MsgpackTrailingData = []byte{0xc0} }},
				} {
					t.Run("ExportCM native "+field.name, func(t *testing.T) {
						envelope := newDescriptorExportCMEnvelope(descriptor)
						field.mutate(envelope)
						assertPayloadEncodeRejected(t, envelope)
					})
				}
			}
		})
	}
}

func newDescriptorNativeEnvelope(descriptor kcesPayloadDescriptor) *KCESPayloadEnvelope {
	envelope := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      descriptor.Extension,
		LengthPrefixed: descriptor.LengthPrefixed,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
		Kind:           descriptor.Kind,
	}
	switch descriptor.Kind {
	case PayloadKindDynamicBoneStatus:
		envelope.DynamicBone = NewDynamicBoneStatus()
	case PayloadKindColliderPackage:
		envelope.ColliderPackage = &ColliderPackage{Version: 1000}
	case PayloadKindLimbCollider:
		envelope.LimbCollider = &LimbColliderPackage{Version: 1000}
	case PayloadKindIKCollider:
		envelope.IKCollider = &IKColliderPackage{Version: 1000}
	case PayloadKindClothParams:
		envelope.ClothParams = NewClothParams()
	case PayloadKindJSONString:
		envelope.Text = "{}"
	default:
		panic("unsupported native payload kind " + descriptor.Kind)
	}
	return envelope
}

func newDescriptorExportCMEnvelope(descriptor kcesPayloadDescriptor) *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESExportCM,
		Extension:      descriptor.Extension,
		StorageVariant: descriptor.ExportCMStorageVariant,
		Kind:           descriptor.ExportCMKind,
		JSON:           json.RawMessage(`{"ok":true}`),
	}
}

func clearDescriptorActiveRoot(envelope *KCESPayloadEnvelope, kind string) {
	switch kind {
	case PayloadKindDynamicBoneStatus:
		envelope.DynamicBone = nil
	case PayloadKindColliderPackage:
		envelope.ColliderPackage = nil
	case PayloadKindLimbCollider:
		envelope.LimbCollider = nil
	case PayloadKindIKCollider:
		envelope.IKCollider = nil
	case PayloadKindClothParams:
		envelope.ClothParams = nil
	case PayloadKindJSONString:
		envelope.Text = ""
		envelope.JSON = nil
	}
}

func setDescriptorInactiveTypedRoot(envelope *KCESPayloadEnvelope, activeKind string) {
	if activeKind == PayloadKindDynamicBoneStatus {
		envelope.ColliderPackage = &ColliderPackage{}
		return
	}
	envelope.DynamicBone = NewDynamicBoneStatus()
}

func assertPayloadEncodeRejected(t *testing.T, envelope *KCESPayloadEnvelope) {
	t.Helper()
	if _, err := EncodeKCESPayload(envelope); err == nil {
		t.Fatalf("EncodeKCESPayload unexpectedly accepted %+v", envelope)
	}
}

func TestKCESJSONTextDescriptorsCoverExtensions(t *testing.T) {
	expected := []string{
		KCESUndressDataExtension,
		KCESUndressPartsDataExtension,
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
	envelope := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      KCESDBConfExtension,
		LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
		Kind:           PayloadKindClothParams,
		ClothParams:    NewClothParams(),
	}
	if _, err := EncodeKCESPayload(envelope); err == nil || !strings.Contains(err.Error(), "requires kind") {
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
