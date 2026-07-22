package KCES

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestColliderJSONBindsTypeToConcreteObject(t *testing.T) {
	encoded, err := json.Marshal(ColliderRef{Type: ColliderTypeCapsule, Collider: &ColliderCapsule{}})
	if err != nil {
		t.Fatal(err)
	}
	var valid ColliderRef
	if err := json.Unmarshal(encoded, &valid); err != nil {
		t.Fatalf("valid capsule JSON rejected: %v", err)
	}
	if _, ok := valid.Collider.(*ColliderCapsule); !ok {
		t.Fatalf("valid collider type = %T", valid.Collider)
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	object["type"] = json.RawMessage("0")
	mismatched, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	var mismatch ColliderRef
	if err := json.Unmarshal(mismatched, &mismatch); err == nil {
		t.Fatal("capsule object with plane discriminator was accepted")
	}
}

func TestColliderJSONRejectsUnknownConcreteFieldsAndTrailingValues(t *testing.T) {
	encoded, err := json.Marshal(ColliderRef{Type: ColliderTypeSphere, Collider: &ColliderSphere{}})
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	var collider map[string]json.RawMessage
	if err := json.Unmarshal(object["collider"], &collider); err != nil {
		t.Fatal(err)
	}
	collider["futureField"] = json.RawMessage(`true`)
	object["collider"], err = json.Marshal(collider)
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	var value ColliderRef
	if err := json.Unmarshal(unknown, &value); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown collider field error = %v", err)
	}
	if err := value.UnmarshalJSON(append(encoded, []byte("\n{}")...)); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing collider JSON error = %v", err)
	}
}
