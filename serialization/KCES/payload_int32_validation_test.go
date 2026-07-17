package KCES

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

type payloadInt32Case struct {
	name        string
	wantPath    string
	newEnvelope func() *KCESPayloadEnvelope
	setValue    func(*KCESPayloadEnvelope, int)
}

func TestEncodeKCESPayloadRejectsCLRInt32Overflow(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot represent values outside the CLR Int32 range")
	}

	cases := payloadInt32Cases()
	outOfRange := []struct {
		name  string
		value int
	}{
		{name: "above_max", value: int(int64(math.MaxInt32) + 1)},
		{name: "below_min", value: int(int64(math.MinInt32) - 1)},
	}

	for _, test := range cases {
		test := test
		for _, invalid := range outOfRange {
			invalid := invalid
			t.Run(test.name+"/"+invalid.name, func(t *testing.T) {
				envelope := test.newEnvelope()
				test.setValue(envelope, invalid.value)

				_, err := EncodeKCESPayload(envelope)
				if err == nil {
					t.Fatalf("EncodeKCESPayload accepted %s=%d", test.wantPath, invalid.value)
				}
				if !strings.Contains(err.Error(), test.wantPath) || !strings.Contains(err.Error(), "Int32") {
					t.Fatalf("EncodeKCESPayload error = %v, want field path %q and Int32 range context", err, test.wantPath)
				}
			})
		}
	}
}

func TestEncodeKCESPayloadAcceptsCLRInt32Boundaries(t *testing.T) {
	boundaries := []struct {
		name  string
		value int
	}{
		{name: "min", value: int(int64(math.MinInt32))},
		{name: "max", value: int(int64(math.MaxInt32))},
	}

	for _, boundary := range boundaries {
		boundary := boundary
		t.Run(boundary.name+"/dynamic_bone", func(t *testing.T) {
			envelope := newDynamicBoneInt32Envelope()
			envelope.DynamicBone.Version = boundary.value
			envelope.DynamicBone.FreezeAxis = boundary.value
			if _, err := EncodeKCESPayload(envelope); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/cloth", func(t *testing.T) {
			envelope := newClothInt32Envelope()
			params := envelope.ClothParams
			params.BendDistanceMaxCount = boundary.value
			params.NearDistanceMaxCount = boundary.value
			params.AdjustMode = ClothAdjustMode(boundary.value)
			params.PenetrationMode = ClothPenetrationMode(boundary.value)
			params.PenetrationAxis = ClothPenetrationAxis(boundary.value)
			params.TeleportMode = ClothTeleportMode(boundary.value)
			if _, err := EncodeKCESPayload(envelope); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/generic_collider", func(t *testing.T) {
			envelope := newGenericColliderInt32Envelope()
			pkg := envelope.ColliderPackage
			capsule := pkg.Colliders[0].Collider.(*ColliderCapsule)
			sphere := pkg.Colliders[1].Collider.(*ColliderSphere)
			pkg.Version = boundary.value
			capsule.Version = boundary.value
			capsule.Bound = boundary.value
			capsule.Direction = boundary.value
			sphere.Version = boundary.value
			sphere.Bound = boundary.value
			pkg.LimbEnableList[0].Version = boundary.value
			pkg.LimbEnableList[0].LimbType = boundary.value
			if _, err := EncodeKCESPayload(envelope); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/limb_collider", func(t *testing.T) {
			envelope := newLimbColliderInt32Envelope()
			pkg := envelope.LimbCollider
			collider := pkg.Items[0].Collider.(*ColliderMaidProp)
			pkg.Version = boundary.value
			pkg.Items[0].Version = boundary.value
			pkg.Items[0].Target = boundary.value
			collider.Version = boundary.value
			collider.Bound = boundary.value
			collider.Direction = boundary.value
			if _, err := EncodeKCESPayload(envelope); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/maid_prop_mpn", func(t *testing.T) {
			envelope := newLimbColliderInt32Envelope()
			collider := envelope.LimbCollider.Items[0].Collider.(*ColliderMaidProp)
			collider.Version = colliderMaidPropFixVersion
			collider.CenterMpnList = []int{boundary.value}
			collider.StartRadiusMpnList = []int{boundary.value}
			collider.EndRadiusMpnList = []int{boundary.value}
			if _, err := EncodeKCESPayload(envelope); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 MPN boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/ik_collider", func(t *testing.T) {
			envelope := newIKColliderInt32Envelope()
			pkg := envelope.IKCollider
			collider := pkg.Groups[0].Colliders[0].Collider.(*ColliderPlane)
			pkg.Version = boundary.value
			pkg.Groups[0].Version = boundary.value
			pkg.Groups[0].Target = boundary.value
			collider.Version = boundary.value
			collider.Bound = boundary.value
			collider.Direction = boundary.value
			if _, err := EncodeKCESPayload(envelope); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})
	}
}

func payloadInt32Cases() []payloadInt32Case {
	return []payloadInt32Case{
		{name: "dynamic_bone/version", wantPath: "dynamicBoneStatus.version", newEnvelope: newDynamicBoneInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.DynamicBone.Version = value }},
		{name: "dynamic_bone/freeze_axis", wantPath: "dynamicBoneStatus.freezeAxis", newEnvelope: newDynamicBoneInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.DynamicBone.FreezeAxis = value }},

		{name: "cloth/bend_distance_max_count", wantPath: "clothParams.bendDistanceMaxCount", newEnvelope: newClothInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ClothParams.BendDistanceMaxCount = value }},
		{name: "cloth/near_distance_max_count", wantPath: "clothParams.nearDistanceMaxCount", newEnvelope: newClothInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ClothParams.NearDistanceMaxCount = value }},
		{name: "cloth/adjust_mode", wantPath: "clothParams.adjustMode", newEnvelope: newClothInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ClothParams.AdjustMode = ClothAdjustMode(value) }},
		{name: "cloth/penetration_mode", wantPath: "clothParams.penetrationMode", newEnvelope: newClothInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ClothParams.PenetrationMode = ClothPenetrationMode(value) }},
		{name: "cloth/penetration_axis", wantPath: "clothParams.penetrationAxis", newEnvelope: newClothInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ClothParams.PenetrationAxis = ClothPenetrationAxis(value) }},
		{name: "cloth/teleport_mode", wantPath: "clothParams.teleportMode", newEnvelope: newClothInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ClothParams.TeleportMode = ClothTeleportMode(value) }},

		{name: "generic/package_version", wantPath: "colliderPackage.version", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ColliderPackage.Version = value }},
		{name: "generic/ref_type", wantPath: "colliderPackage.colliders[0].type", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ColliderPackage.Colliders[0].Type = value }},
		{name: "generic/status_version", wantPath: "colliderPackage.colliders[0].collider.version", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.ColliderPackage.Colliders[0].Collider.(*ColliderCapsule).Version = value
			}},
		{name: "generic/status_bound", wantPath: "colliderPackage.colliders[0].collider.bound", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.ColliderPackage.Colliders[0].Collider.(*ColliderCapsule).Bound = value
			}},
		{name: "generic/status_direction", wantPath: "colliderPackage.colliders[0].collider.direction", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.ColliderPackage.Colliders[0].Collider.(*ColliderCapsule).Direction = value
			}},
		{name: "generic/sphere_status_version", wantPath: "colliderPackage.colliders[1].collider.version", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.ColliderPackage.Colliders[1].Collider.(*ColliderSphere).Version = value
			}},
		{name: "generic/sphere_status_bound", wantPath: "colliderPackage.colliders[1].collider.bound", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.ColliderPackage.Colliders[1].Collider.(*ColliderSphere).Bound = value
			}},
		{name: "generic/state_version", wantPath: "colliderPackage.limbEnableList[0].version", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ColliderPackage.LimbEnableList[0].Version = value }},
		{name: "generic/state_limb_type", wantPath: "colliderPackage.limbEnableList[0].limbType", newEnvelope: newGenericColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.ColliderPackage.LimbEnableList[0].LimbType = value }},

		{name: "limb/package_version", wantPath: "limbColliderPackage.version", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.LimbCollider.Version = value }},
		{name: "limb/item_version", wantPath: "limbColliderPackage.items[0].version", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.LimbCollider.Items[0].Version = value }},
		{name: "limb/item_target", wantPath: "limbColliderPackage.items[0].target", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.LimbCollider.Items[0].Target = value }},
		{name: "limb/status_version", wantPath: "limbColliderPackage.items[0].collider.version", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.LimbCollider.Items[0].Collider.(*ColliderMaidProp).Version = value
			}},
		{name: "limb/status_bound", wantPath: "limbColliderPackage.items[0].collider.bound", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.LimbCollider.Items[0].Collider.(*ColliderMaidProp).Bound = value
			}},
		{name: "limb/status_direction", wantPath: "limbColliderPackage.items[0].collider.direction", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.LimbCollider.Items[0].Collider.(*ColliderMaidProp).Direction = value
			}},
		{name: "limb/center_mpn", wantPath: "limbColliderPackage.items[0].collider.centerMpnList[0]", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.LimbCollider.Items[0].Collider.(*ColliderMaidProp).CenterMpnList = []int{value}
			}},
		{name: "limb/start_radius_mpn", wantPath: "limbColliderPackage.items[0].collider.startRadiusMpnList[0]", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.LimbCollider.Items[0].Collider.(*ColliderMaidProp).StartRadiusMpnList = []int{value}
			}},
		{name: "limb/end_radius_mpn", wantPath: "limbColliderPackage.items[0].collider.endRadiusMpnList[0]", newEnvelope: newLimbColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.LimbCollider.Items[0].Collider.(*ColliderMaidProp).EndRadiusMpnList = []int{value}
			}},

		{name: "ik/package_version", wantPath: "ikColliderPackage.version", newEnvelope: newIKColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.IKCollider.Version = value }},
		{name: "ik/group_version", wantPath: "ikColliderPackage.groups[0].version", newEnvelope: newIKColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.IKCollider.Groups[0].Version = value }},
		{name: "ik/group_target", wantPath: "ikColliderPackage.groups[0].target", newEnvelope: newIKColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.IKCollider.Groups[0].Target = value }},
		{name: "ik/ref_type", wantPath: "ikColliderPackage.groups[0].colliders[0].type", newEnvelope: newIKColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) { e.IKCollider.Groups[0].Colliders[0].Type = value }},
		{name: "ik/status_version", wantPath: "ikColliderPackage.groups[0].colliders[0].collider.version", newEnvelope: newIKColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.IKCollider.Groups[0].Colliders[0].Collider.(*ColliderPlane).Version = value
			}},
		{name: "ik/status_bound", wantPath: "ikColliderPackage.groups[0].colliders[0].collider.bound", newEnvelope: newIKColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.IKCollider.Groups[0].Colliders[0].Collider.(*ColliderPlane).Bound = value
			}},
		{name: "ik/status_direction", wantPath: "ikColliderPackage.groups[0].colliders[0].collider.direction", newEnvelope: newIKColliderInt32Envelope,
			setValue: func(e *KCESPayloadEnvelope, value int) {
				e.IKCollider.Groups[0].Colliders[0].Collider.(*ColliderPlane).Direction = value
			}},
	}
}

func newDynamicBoneInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Extension:   ".dbconf",
		Kind:        PayloadKindDynamicBoneStatus,
		DynamicBone: NewDynamicBoneStatus(),
	}
}

func newClothInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Extension:   ".dsbconf",
		Kind:        PayloadKindClothParams,
		ClothParams: NewClothParams(),
	}
}

func newGenericColliderInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Extension: ".dbcol",
		Kind:      PayloadKindColliderPackage,
		ColliderPackage: &ColliderPackage{
			Version: 1000,
			Colliders: []ColliderRef{
				{
					Type:     ColliderTypeCapsule,
					Collider: NewColliderCapsule(),
				},
				{
					Type:     ColliderTypeSphere,
					Collider: NewColliderSphere(),
				},
			},
			LimbEnableList: []ColliderState{{Version: 1000, LimbType: 0, IsEnable: true}},
		},
	}
}

func newLimbColliderInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Extension: ".limbcol",
		Kind:      PayloadKindLimbCollider,
		LimbCollider: &LimbColliderPackage{
			Version: 1000,
			Items: []LimbColliderItem{{
				Version:  1000,
				Target:   0,
				Collider: NewColliderMaidProp(),
			}},
		},
	}
}

func newIKColliderInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Extension: ".ikcol",
		Kind:      PayloadKindIKCollider,
		IKCollider: &IKColliderPackage{
			Version: 1000,
			Groups: []IKColliderGroup{{
				Version: 1000,
				Target:  0,
				Colliders: []ColliderRef{{
					Type:     ColliderTypePlane,
					Collider: NewColliderPlane(),
				}},
			}},
		},
	}
}
