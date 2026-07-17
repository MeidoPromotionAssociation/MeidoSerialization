package KCES

import (
	"encoding/json"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestDynamicBoneStatusJSONKeepsOmittedFieldsZero(t *testing.T) {
	var omitted DynamicBoneStatus
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.Version != 0 || omitted.Damping != 0 || omitted.Elasticity != 0 || omitted.Stiffness != 0 || omitted.Gravity != (Vector3{}) {
		t.Fatalf("omitted JSON fields gained C# defaults: %+v", omitted)
	}
	created := NewDynamicBoneStatus()
	if created.Version != 1000 || created.Damping != 0.6 || created.Elasticity != 0.1 || created.Stiffness != 0.1 || created.Gravity.Y != -0.05 {
		t.Fatalf("explicit constructor defaults = %+v", created)
	}

	var explicit DynamicBoneStatus
	if err := json.Unmarshal([]byte(`{
		"version":0,
		"damping":0,
		"elasticity":0,
		"stiffness":0,
		"gravity":{"x":0,"y":0,"z":0}
	}`), &explicit); err != nil {
		t.Fatal(err)
	}
	if explicit.Version != 0 || explicit.Damping != 0 || explicit.Elasticity != 0 || explicit.Stiffness != 0 || explicit.Gravity != (Vector3{}) {
		t.Fatalf("explicit JSON zero did not override defaults: %+v", explicit)
	}
}

func TestPreMulTexDatasJSONKeepsOmittedFieldsZero(t *testing.T) {
	var omitted PreMulTexDatas
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.Version != 0 || omitted.LayNoInGroup != 0 || omitted.Alpha != 0 || omitted.PreTexCompoTypeStr != "" {
		t.Fatalf("omitted JSON fields gained C# defaults: %+v", omitted)
	}
	created := NewPreMulTexDatas()
	if created.Version != 1001 || created.LayNoInGroup != -1 || created.Alpha != 1 || created.PreTexCompoTypeStr != "Alpha" {
		t.Fatalf("explicit constructor defaults = %+v", created)
	}

	var explicit PreMulTexDatas
	if err := json.Unmarshal([]byte(`{
		"version":0,
		"f_nLayNoInGroup":0,
		"f_fAlpha":0,
		"preTexCompoTypeStr":"0"
	}`), &explicit); err != nil {
		t.Fatal(err)
	}
	if explicit.Version != 0 || explicit.LayNoInGroup != 0 || explicit.Alpha != 0 || explicit.PreTexCompoTypeStr != "0" {
		t.Fatalf("explicit JSON values did not override defaults: %+v", explicit)
	}
}

func TestNestedMenuTypesKeepOmittedFieldsZero(t *testing.T) {
	t.Run("transTexData", func(t *testing.T) {
		var fromJSON TransTexData
		if err := json.Unmarshal([]byte(`{}`), &fromJSON); err != nil {
			t.Fatal(err)
		}
		if fromJSON != (TransTexData{}) {
			t.Fatalf("JSON TransTexData gained defaults: %+v", fromJSON)
		}

		var fromMsgpack TransTexData
		decodeShortArray(t, &fromMsgpack)
		if fromMsgpack.IndexedObjectMetadata == nil || fromMsgpack.FieldCount == nil || *fromMsgpack.FieldCount != 0 {
			t.Fatalf("short MessagePack TransTexData did not retain its zero-slot width: %+v", fromMsgpack.IndexedObjectMetadata)
		}
		fromMsgpack.IndexedObjectMetadata = nil
		if fromMsgpack != (TransTexData{}) {
			t.Fatalf("MessagePack TransTexData gained defaults: %+v", fromMsgpack)
		}
		assertTransTexDefaults(t, NewTransTexData())

		var explicit TransTexData
		if err := json.Unmarshal([]byte(`{"scale":{"x":0,"y":0},"areaUV":{"x":0,"y":0,"z":0,"w":0}}`), &explicit); err != nil {
			t.Fatal(err)
		}
		if explicit.Scale != (Vector2{}) || explicit.AreaUV != (Vector4{}) {
			t.Fatalf("explicit zero did not override TransTexData defaults: %+v", explicit)
		}
	})

	t.Run("infColorParam", func(t *testing.T) {
		var fromJSON InfColorParam
		if err := json.Unmarshal([]byte(`{}`), &fromJSON); err != nil {
			t.Fatal(err)
		}
		if fromJSON.InfColorID != 0 {
			t.Fatalf("JSON infColorId = %d, want zero", fromJSON.InfColorID)
		}
		var fromMsgpack InfColorParam
		decodeShortArray(t, &fromMsgpack)
		if fromMsgpack.InfColorID != 0 {
			t.Fatalf("short MessagePack infColorId = %d, want zero", fromMsgpack.InfColorID)
		}
		if NewInfColorParam().InfColorID != -1 {
			t.Fatal("explicit InfColorParam constructor lost its current-game default")
		}
		var explicit InfColorParam
		if err := json.Unmarshal([]byte(`{"infColorId":0}`), &explicit); err != nil {
			t.Fatal(err)
		}
		if explicit.InfColorID != 0 {
			t.Fatalf("explicit infColorId zero was not preserved: %+v", explicit)
		}
	})

	t.Run("partColDef", func(t *testing.T) {
		var fromJSON PartColDef
		if err := json.Unmarshal([]byte(`{}`), &fromJSON); err != nil {
			t.Fatal(err)
		}
		if fromJSON.PatternScale != (Vector2{}) {
			t.Fatalf("JSON patternScale = %+v, want zero", fromJSON.PatternScale)
		}
		var fromMsgpack PartColDef
		decodeShortArray(t, &fromMsgpack)
		if fromMsgpack.PatternScale != (Vector2{}) {
			t.Fatalf("short MessagePack patternScale = %+v, want zero", fromMsgpack.PatternScale)
		}
		if NewPartColDef().PatternScale != (Vector2{X: 1, Y: 1}) {
			t.Fatal("explicit PartColDef constructor lost its current-game default")
		}
		var explicit PartColDef
		if err := json.Unmarshal([]byte(`{"patternScale":{"x":0,"y":0}}`), &explicit); err != nil {
			t.Fatal(err)
		}
		if explicit.PatternScale != (Vector2{}) {
			t.Fatalf("explicit patternScale zero was not preserved: %+v", explicit)
		}
	})

	t.Run("infColData", func(t *testing.T) {
		var fromJSON InfColData
		if err := json.Unmarshal([]byte(`{}`), &fromJSON); err != nil {
			t.Fatal(err)
		}
		if fromJSON.PartsColorType != 0 {
			t.Fatalf("JSON partsColorType = %d, want zero", fromJSON.PartsColorType)
		}
		var fromMsgpack InfColData
		decodeShortArray(t, &fromMsgpack)
		if fromMsgpack.PartsColorType != 0 {
			t.Fatalf("short MessagePack partsColorType = %d, want zero", fromMsgpack.PartsColorType)
		}
		if NewInfColData().PartsColorType != -1 {
			t.Fatal("explicit InfColData constructor lost its current-game default")
		}
		var explicit InfColData
		if err := json.Unmarshal([]byte(`{"partsColorType":0}`), &explicit); err != nil {
			t.Fatal(err)
		}
		if explicit.PartsColorType != 0 {
			t.Fatalf("explicit partsColorType zero was not preserved: %+v", explicit)
		}
	})
}

func TestColliderJSONKeepsOmittedFieldsZero(t *testing.T) {
	tests := []struct {
		name     string
		typeID   int
		validate func(*testing.T, ColliderStatusUnion)
	}{
		{
			name:   "plane",
			typeID: ColliderTypePlane,
			validate: func(t *testing.T, status ColliderStatusUnion) {
				value := status.(*ColliderPlane)
				if *value != (ColliderPlane{}) {
					t.Fatalf("plane gained defaults: %+v", value)
				}
			},
		},
		{
			name:   "capsule",
			typeID: ColliderTypeCapsule,
			validate: func(t *testing.T, status ColliderStatusUnion) {
				value := status.(*ColliderCapsule)
				if *value != (ColliderCapsule{}) {
					t.Fatalf("capsule gained defaults: %+v", value)
				}
			},
		},
		{
			name:   "sphere",
			typeID: ColliderTypeSphere,
			validate: func(t *testing.T, status ColliderStatusUnion) {
				value := status.(*ColliderSphere)
				if *value != (ColliderSphere{}) {
					t.Fatalf("sphere gained defaults: %+v", value)
				}
			},
		},
		{
			name:   "maid_prop",
			typeID: ColliderTypeMaidProp,
			validate: func(t *testing.T, status ColliderStatusUnion) {
				value := status.(*ColliderMaidProp)
				if value.Version != 0 || value.LocalRotation != (Vector4{}) || value.LocalScale != (Vector3{}) ||
					value.Direction != 0 || value.StartRadius != 0 || value.EndRadius != 0 || value.MaxStartRadius != 0 || value.MaxEndRadius != 0 {
					t.Fatalf("maid-prop gained scalar defaults: %+v", value)
				}
				if value.CenterMpnList != nil || value.StartRadiusMpnList != nil || value.EndRadiusMpnList != nil ||
					value.CenterMpnNameList != nil || value.StartRadiusMpnNameList != nil || value.EndRadiusMpnNameList != nil {
					t.Fatalf("maid-prop gained list defaults: %+v", value)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(map[string]interface{}{
				"type":     test.typeID,
				"collider": map[string]interface{}{},
			})
			if err != nil {
				t.Fatal(err)
			}
			var ref ColliderRef
			if err := json.Unmarshal(data, &ref); err != nil {
				t.Fatal(err)
			}
			test.validate(t, ref.Collider)
		})
	}
	created := NewColliderMaidProp()
	if created.Version != 1002 || created.LocalRotation.W != 1 || created.LocalScale != (Vector3{X: 1, Y: 1, Z: 1}) || created.StartRadius != 0.5 {
		t.Fatalf("explicit collider constructor defaults = %+v", created)
	}
}

func TestColliderJSONExplicitZeroOverridesCSharpDefaults(t *testing.T) {
	var ref ColliderRef
	if err := json.Unmarshal([]byte(`{
		"type":3,
		"collider":{
			"version":0,
			"localRotation":{"x":0,"y":0,"z":0,"w":0},
			"localScale":{"x":0,"y":0,"z":0},
			"direction":0,
			"startRadius":0,
			"endRadius":0,
			"maxStartRadius":0,
			"maxEndRadius":0
		}
	}`), &ref); err != nil {
		t.Fatal(err)
	}
	value := ref.Collider.(*ColliderMaidProp)
	if value.Version != 0 || value.LocalRotation != (Vector4{}) || value.LocalScale != (Vector3{}) ||
		value.Direction != VectorTypeX || value.StartRadius != 0 || value.EndRadius != 0 || value.MaxStartRadius != 0 || value.MaxEndRadius != 0 {
		t.Fatalf("explicit collider zero did not override defaults: %+v", value)
	}
}

func TestJSONZeroValuesSurvivePublicPayloadEncodeDecode(t *testing.T) {
	var dynamicEnvelope KCESPayloadEnvelope
	if err := json.Unmarshal([]byte(`{
		"format":"kces-msgpack-lz4",
		"extension":".dbconf",
		"kind":"dynamic-bone-status",
		"dynamicBoneStatus":{}
	}`), &dynamicEnvelope); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeKCESPayload(&dynamicEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeKCESPayload(encoded, ".dbconf")
	if err != nil {
		t.Fatal(err)
	}
	if decoded.DynamicBone == nil || decoded.DynamicBone.Version != 0 || decoded.DynamicBone.Damping != 0 || decoded.DynamicBone.Gravity != (Vector3{}) {
		t.Fatalf("dynamic-bone zero values changed in public round trip: %+v", decoded.DynamicBone)
	}

	var colliderEnvelope KCESPayloadEnvelope
	if err := json.Unmarshal([]byte(`{
		"format":"kces-msgpack-lz4",
		"extension":".dbcol",
		"kind":"collider-package",
		"colliderPackage":{
			"colliders":[{"type":1,"collider":{}}]
		}
	}`), &colliderEnvelope); err != nil {
		t.Fatal(err)
	}
	encoded, err = EncodeKCESPayload(&colliderEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = DecodeKCESPayload(encoded, ".dbcol")
	if err != nil {
		t.Fatal(err)
	}
	capsule := decoded.ColliderPackage.Colliders[0].Collider.(*ColliderCapsule)
	if capsule.Version != 0 || capsule.LocalRotation != (Vector4{}) || capsule.LocalScale != (Vector3{}) || capsule.StartRadius != 0 || capsule.EndRadius != 0 {
		t.Fatalf("collider zero values changed in public round trip: %+v", capsule)
	}
}

func assertTransTexDefaults(t *testing.T, value *TransTexData) {
	t.Helper()
	if value.Scale != (Vector2{X: 1, Y: 1}) || value.AreaUV != (Vector4{Z: 1, W: 1}) {
		t.Fatalf("TransTexData defaults = %+v, want Vector2.one and (0,0,1,1)", value)
	}
}

func decodeShortArray(t *testing.T, out interface{}) {
	t.Helper()
	data, err := ct.EncodeMsgpack([]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if err := ct.DecodeMsgpack(data, out); err != nil {
		t.Fatal(err)
	}
}
