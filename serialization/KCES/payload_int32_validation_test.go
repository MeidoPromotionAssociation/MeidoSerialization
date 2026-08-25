package KCES

import "testing"

func TestEncodeKCESPayloadAcceptsCLRInt32Boundaries(t *testing.T) {
	boundaries := []struct {
		name  string
		value int32
	}{
		{name: "min", value: -1 << 31},
		{name: "max", value: 1<<31 - 1},
	}

	for _, boundary := range boundaries {
		boundary := boundary
		t.Run(boundary.name+"/dynamic_bone", func(t *testing.T) {
			status := NewDynamicBoneStatus()
			status.Version = boundary.value
			status.FreezeAxis = boundary.value
			if _, err := EncodeKCESPayload(status, ".dbconf"); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/cloth", func(t *testing.T) {
			params := NewClothParams()
			params.BendDistanceMaxCount = boundary.value
			params.NearDistanceMaxCount = boundary.value
			params.AdjustMode = ClothAdjustMode(boundary.value)
			params.PenetrationMode = ClothPenetrationMode(boundary.value)
			params.PenetrationAxis = ClothPenetrationAxis(boundary.value)
			params.TeleportMode = ClothTeleportMode(boundary.value)
			if _, err := EncodeKCESPayload(params, ".dsbconf"); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/generic_collider", func(t *testing.T) {
			pkg := newGenericColliderInt32Package()
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
			if _, err := EncodeKCESPayload(pkg, ".dbcol"); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/limb_collider", func(t *testing.T) {
			pkg := newLimbColliderInt32Package()
			collider := pkg.Items[0].Collider
			pkg.Version = boundary.value
			pkg.Items[0].Version = boundary.value
			pkg.Items[0].Target = boundary.value
			collider.Version = boundary.value
			collider.Bound = boundary.value
			collider.Direction = boundary.value
			collider.CenterMpnList = []int32{boundary.value}
			collider.StartRadiusMpnList = []int32{boundary.value}
			collider.EndRadiusMpnList = []int32{boundary.value}
			if _, err := EncodeKCESPayload(pkg, ".limbcol"); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})

		t.Run(boundary.name+"/ik_collider", func(t *testing.T) {
			pkg := newIKColliderInt32Package()
			collider := pkg.Groups[0].Colliders[0].Collider.(*ColliderPlane)
			pkg.Version = boundary.value
			pkg.Groups[0].Version = boundary.value
			pkg.Groups[0].Target = boundary.value
			collider.Version = boundary.value
			collider.Bound = boundary.value
			collider.Direction = boundary.value
			if _, err := EncodeKCESPayload(pkg, ".ikcol"); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
			}
		})
	}
}

func TestEncodeKCESPayloadRejectsMismatchedRootType(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		value     any
	}{
		{name: "dynamic bone root on collider extension", extension: ".dbcol", value: NewDynamicBoneStatus()},
		{name: "collider root on dynamic bone extension", extension: ".dbconf", value: newGenericColliderInt32Package()},
		{name: "cloth params root on MagicaCloth2 extension", extension: ".db2conf", value: NewClothParams()},
		{name: "MagicaCloth2 root on cloth params extension", extension: ".dsbconf", value: &MagicaClothSerializeData{}},
		{name: "IK collider root on limb collider extension", extension: ".limbcol", value: newIKColliderInt32Package()},
		{name: "unsupported root type", extension: ".dbconf", value: "not a payload"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if _, err := EncodeKCESPayload(test.value, test.extension); err == nil {
				t.Fatal("EncodeKCESPayload accepted a root type the extension does not declare")
			}
		})
	}
}

func newGenericColliderInt32Package() *ColliderPackage {
	return &ColliderPackage{
		Version: 1000,
		Colliders: []*ColliderRef{
			{Type: ColliderTypeCapsule, Collider: NewColliderCapsule()},
			{Type: ColliderTypeSphere, Collider: NewColliderSphere()},
		},
		LimbEnableList: []*ColliderState{{Version: 1000, LimbType: 0, IsEnable: true}},
	}
}

func newLimbColliderInt32Package() *LimbColliderPackage {
	return &LimbColliderPackage{
		Version: 1000,
		Items:   []*LimbColliderItem{{Version: 1000, Target: 0, Collider: NewColliderMaidProp()}},
	}
}

func newIKColliderInt32Package() *IKColliderPackage {
	return &IKColliderPackage{
		Version: 1000,
		Groups: []*IKColliderGroup{{
			Version: 1000,
			Target:  0,
			Colliders: []*ColliderRef{{
				Type: ColliderTypePlane, Collider: NewColliderPlane(),
			}},
		}},
	}
}
