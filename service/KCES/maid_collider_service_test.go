package KCES

import (
	"os"
	"path/filepath"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestMaidColliderServiceHandlesExtensionlessTextAsset(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "maid_collider")
	jsonPath := input + ".json"
	back := filepath.Join(dir, "maid_collider.bytes")
	encoded, err := serializationKCES.EncodeMaidCollider(&serializationKCES.MaidColliderFile{
		Colliders: []serializationKCES.MaidCapsuleCollider{{
			BonePath:  "Bip01/Bip01 Pelvis",
			Direction: 1,
			Height:    2,
			Radius:    0.5,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, encoded, 0644); err != nil {
		t.Fatal(err)
	}
	if !IsKCESMaidColliderFile(input) || !IsKCESMaidColliderFile(back) {
		t.Fatal("maid collider base-name detection failed")
	}
	service := &MaidColliderService{}
	if err := service.ConvertMaidColliderToJSON(TestConversionContext, input, jsonPath, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertMaidColliderToJSON: %v", err)
	}
	if !IsKCESMaidColliderJSONFile(jsonPath) {
		t.Fatal("maid collider JSON marker was not detected")
	}
	if err := service.ConvertJSONToMaidCollider(TestConversionContext, jsonPath, back, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJSONToMaidCollider: %v", err)
	}
	if _, err := serializationKCES.DecodeMaidCollider(mustReadFile(t, back)); err != nil {
		t.Fatalf("decode rebuilt maid collider: %v", err)
	}
}
