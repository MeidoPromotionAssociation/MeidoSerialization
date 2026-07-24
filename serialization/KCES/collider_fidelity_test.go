package KCES

import (
	"strings"
	"testing"
)

func TestColliderPayloadRejectsUnknownUnionTag(t *testing.T) {
	data := lengthPrefixedIndexedTestValue(t, []interface{}{
		int64(1000),
		[]interface{}{
			[]interface{}{int64(255), []interface{}{}},
		},
		nil,
	})
	if _, err := DecodeKCESPayload(data, ".dbcol"); err == nil || !strings.Contains(err.Error(), "unsupported collider type") {
		t.Fatalf("DecodeKCESPayload() error = %v, want unsupported collider type", err)
	}
}

func TestColliderPayloadRejectsSparseRawFallback(t *testing.T) {
	data := lengthPrefixedIndexedTestValue(t, []interface{}{
		int64(1000),
		[]interface{}{
			[]interface{}{int64(1000), int64(0), []interface{}{}},
		},
	})
	if _, err := DecodeKCESPayload(data, ".limbcol"); err == nil {
		t.Fatal("malformed collider payload unexpectedly decoded")
	}
}
