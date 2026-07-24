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
			if _, err := EncodeKCESPayload(envelope); err != nil {
				t.Fatalf("EncodeKCESPayload rejected CLR Int32 boundary: %v", err)
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

func newDynamicBoneInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".dbconf",
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindDynamicBoneStatus,
		DynamicBone: NewDynamicBoneStatus(),
	}
}

func newClothInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".dsbconf",
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindClothParams,
		ClothParams: NewClothParams(),
	}
}

func newGenericColliderInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".dbcol",
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindColliderPackage,
		ColliderPackage: &ColliderPackage{
			Version: 1000,
			Colliders: []*ColliderRef{
				{Type: ColliderTypeCapsule, Collider: NewColliderCapsule()},
				{Type: ColliderTypeSphere, Collider: NewColliderSphere()},
			},
			LimbEnableList: []*ColliderState{{Version: 1000, LimbType: 0, IsEnable: true}},
		},
	}
}

func newLimbColliderInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".limbcol",
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindLimbCollider,
		LimbCollider: &LimbColliderPackage{
			Version: 1000,
			Items:   []*LimbColliderItem{{Version: 1000, Target: 0, Collider: NewColliderMaidProp()}},
		},
	}
}

func newIKColliderInt32Envelope() *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Format: PayloadFormatKCESMessagePack, Extension: ".ikcol",
		StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindIKCollider,
		IKCollider: &IKColliderPackage{
			Version: 1000,
			Groups: []*IKColliderGroup{{
				Version: 1000,
				Target:  0,
				Colliders: []*ColliderRef{{
					Type: ColliderTypePlane, Collider: NewColliderPlane(),
				}},
			}},
		},
	}
}
