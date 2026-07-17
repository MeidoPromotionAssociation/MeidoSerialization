package utilities

import (
	"reflect"
	"testing"
)

func TestMergeOrderedMapKeysPreservesLiveOrderAndAppendsAdditions(t *testing.T) {
	values := map[string]int{"a": 1, "b": 2, "c": 3}
	got, err := MergeOrderedMapKeys(values, []string{"b", "removed"}, "test order")
	if err != nil {
		t.Fatalf("MergeOrderedMapKeys: %v", err)
	}
	if want := []string{"b", "a", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %#v, want %#v", got, want)
	}
}

func TestMergeOrderedMapKeysRejectsDuplicateLiveKey(t *testing.T) {
	if _, err := MergeOrderedMapKeys(map[int]struct{}{2: {}}, []int{2, 2}, "test order"); err == nil {
		t.Fatal("duplicate live key was accepted")
	}
}
