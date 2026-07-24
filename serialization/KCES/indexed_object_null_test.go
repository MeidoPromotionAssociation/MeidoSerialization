package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestIndexedObjectsRejectNilForValueTypedFields(t *testing.T) {
	var value Vector3
	if err := ct.DecodeMsgpack([]byte{0x93, 0xc0, 0x01, 0x02}, &value); err == nil || !strings.Contains(err.Error(), "nil is not valid") {
		t.Fatalf("DecodeMsgpack() error = %v, want value-typed nil rejection", err)
	}
}

func TestIndexedObjectsAcceptNullableStringFields(t *testing.T) {
	// MaterialAssets has nullable names in the wire model; a nil name is a typed null, not side metadata
	wire := compressIndexedTestValue(t, []interface{}{
		nil,
		[]interface{}{},
	})
	decoded, err := DecodeMaterialAssets(wire)
	if err != nil {
		t.Fatalf("DecodeMaterialAssets() error = %v", err)
	}
	if decoded.FileName != nil || decoded.Assets == nil {
		t.Fatalf("typed nullable values changed: %#v", decoded)
	}
}
