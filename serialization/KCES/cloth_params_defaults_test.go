package KCES

import (
	"encoding/json"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestDecodeClothParamsShortArrayKeepsOnlyPresentFields(t *testing.T) {
	radius := []interface{}{float64(0.9), float64(0.8), true, float64(0.7), true}
	msgpackData, err := ct.EncodeMsgpack([]interface{}{radius})
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}
	compressed, err := ct.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatalf("CompressLz4BlockArray: %v", err)
	}

	env, err := DecodeKCESPayload(AddLengthPrefix(compressed), ".dsbconf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	p := env.ClothParams
	if p == nil {
		t.Fatal("DecodeKCESPayload returned nil ClothParams")
	}
	if p.Radius.StartValue != 0.9 || p.Radius.EndValue != 0.8 || !p.Radius.UseCurveValue {
		t.Fatalf("present radius key was not decoded: %+v", p.Radius)
	}
	if p.Mass != (BezierParam{}) || p.Gravity != (BezierParam{}) || p.WindInfluence != 0 || p.DisableDistance != 0 {
		t.Fatalf("short payload gained game constructor defaults: %+v", p)
	}
}

func TestClothParamsJSONOmittedFieldsRemainZero(t *testing.T) {
	var p ClothParams
	if err := json.Unmarshal([]byte(`{"useGravity":false,"massInfluence":0,"gravityDirection":{"x":2,"y":0,"z":0}}`), &p); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	// Explicit zero/false values must win over defaults.
	if p.UseGravity || p.MassInfluence != 0 || p.GravityDirection != (Vector3{X: 2}) {
		t.Fatalf("explicit JSON values were overwritten: %+v", p)
	}
	if p.Mass != (BezierParam{}) || p.Gravity != (BezierParam{}) || p.WindInfluence != 0 || p.DisableDistance != 0 {
		t.Fatalf("omitted JSON fields gained game constructor defaults: %+v", p)
	}
	created := NewClothParams()
	assertClothGameDefaults(t, created)
}

func assertClothGameDefaults(t *testing.T, p *ClothParams) {
	t.Helper()
	checks := []struct {
		name string
		got  float32
		want float32
	}{
		{"mass.startValue", p.Mass.StartValue, 1},
		{"gravity.startValue", p.Gravity.StartValue, -9.8},
		{"drag.startValue", p.Drag.StartValue, 0.02},
		{"maxVelocity.startValue", p.MaxVelocity.StartValue, 3},
		{"windInfluence", p.WindInfluence, 1},
		{"disableDistance", p.DisableDistance, 20},
		{"teleportRotation", p.TeleportRotation, 45},
		{"clampDistanceMaxRatio", p.ClampDistanceMaxRatio, 1.1},
		{"springDirectionAtten.curveValue", p.SpringDirectionAtten.CurveValue, 0.234},
		{"adjustRotationPower", p.AdjustRotationPower, 5},
		{"penetrationRadius.endValue", p.PenetrationRadius.EndValue, 1},
		{"maxMoveSpeed", p.MaxMoveSpeed, 10},
		{"maxRotationSpeed", p.MaxRotationSpeed, 360},
		{"clampRotationVelocityLimit", p.ClampRotationVelocityLimit, 1},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v, want game initializer %v", check.name, check.got, check.want)
		}
	}
	if !p.UseDrag || !p.UseMaxVelocity || !p.UseClampDistanceRatio || !p.UseLineAvarageRotation {
		t.Errorf("boolean game initializers were not retained: drag=%v maxVelocity=%v clampDistance=%v lineAverage=%v",
			p.UseDrag, p.UseMaxVelocity, p.UseClampDistanceRatio, p.UseLineAvarageRotation)
	}
	if p.PenetrationAxis != ClothPenetrationAxisInverseZ {
		t.Errorf("penetrationAxis = %v, want InverseZ", p.PenetrationAxis)
	}
}
