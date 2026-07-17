package KCES

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestExpandedKCESPresetDecodesAllKnownInnerBlocks(t *testing.T) {
	opaque, err := NewKCESPreset()
	if err != nil {
		t.Fatalf("NewKCESPreset: %v", err)
	}
	opaque.ContainerVersion = 731
	opaque.MaidData.Version = -17
	opaque.Meta.Version = 845

	expanded, err := ExpandKCESPreset(opaque)
	if err != nil {
		t.Fatalf("ExpandKCESPreset: %v", err)
	}
	if expanded.MaidData == nil || expanded.MaidData.PropData == nil || expanded.MaidData.ColorData == nil || expanded.MaidData.BodyData == nil {
		t.Fatalf("known inner blocks were not expanded: %+v", expanded.MaidData)
	}
	if expanded.MaidData.PropData.Signature != KCESPresetPropertyListSignature ||
		expanded.MaidData.ColorData.Signature != KCESPresetColorSignature ||
		expanded.MaidData.BodyData.Signature != KCESPresetBodySignature {
		t.Fatalf("expanded signatures = prop %q color %q body %q",
			expanded.MaidData.PropData.Signature,
			expanded.MaidData.ColorData.Signature,
			expanded.MaidData.BodyData.Signature)
	}
	expanded.MaidData.BodyData.Version = -23

	wire, err := EncodeExpandedKCESPreset(expanded)
	if err != nil {
		t.Fatalf("EncodeExpandedKCESPreset: %v", err)
	}
	decoded, err := DecodeExpandedKCESPreset(wire)
	if err != nil {
		t.Fatalf("DecodeExpandedKCESPreset: %v", err)
	}
	if decoded.ContainerVersion != 731 || decoded.MaidData.Version != -17 || decoded.Meta == nil || decoded.Meta.Version != 845 || decoded.MaidData.BodyData.Version != -23 {
		t.Fatalf("expanded versions changed: container=%d maid=%d meta=%+v body=%d", decoded.ContainerVersion, decoded.MaidData.Version, decoded.Meta, decoded.MaidData.BodyData.Version)
	}

	jsonData, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal expanded preset: %v", err)
	}
	if bytes.Contains(jsonData, []byte(`"propData":"`)) || bytes.Contains(jsonData, []byte(`"colorData":"`)) || bytes.Contains(jsonData, []byte(`"bodyData":"`)) {
		t.Fatalf("expanded JSON exposed a known inner block as base64: %s", jsonData)
	}
	for _, field := range []string{`"propData":{`, `"colorData":{`, `"bodyData":{`} {
		if !bytes.Contains(jsonData, []byte(field)) {
			t.Fatalf("expanded JSON lacks typed field %s: %s", field, jsonData)
		}
	}
}

func TestExpandKCESPresetRejectsMalformedPresentInnerBlock(t *testing.T) {
	value, err := NewKCESPreset()
	if err != nil {
		t.Fatal(err)
	}
	value.MaidData.PropData = []byte{0xff, 0x00}
	if _, err := ExpandKCESPreset(value); err == nil || !strings.Contains(err.Error(), "propData") {
		t.Fatalf("ExpandKCESPreset malformed propData error = %v", err)
	}
}

func TestExpandedKCESPresetPreservesNilInnerBlock(t *testing.T) {
	value, err := NewKCESPreset()
	if err != nil {
		t.Fatal(err)
	}
	value.MaidData.ColorData = nil
	expanded, err := ExpandKCESPreset(value)
	if err != nil {
		t.Fatal(err)
	}
	if expanded.MaidData.ColorData != nil {
		t.Fatalf("expanded colorData = %+v, want nil", expanded.MaidData.ColorData)
	}
	collapsed, err := CollapseExpandedKCESPreset(expanded)
	if err != nil {
		t.Fatal(err)
	}
	if collapsed.MaidData.ColorData != nil {
		t.Fatalf("collapsed colorData = %x, want nil", collapsed.MaidData.ColorData)
	}
}
