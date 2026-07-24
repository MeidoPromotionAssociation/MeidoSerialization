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
	if expanded.MaidData.PropData == nil || expanded.MaidData.ColorData == nil || expanded.MaidData.BodyData == nil {
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

func TestKCESPresetJSONAlwaysUsesExpandedTypedBlocks(t *testing.T) {
	opaque, err := NewKCESPreset()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(opaque)
	if err != nil {
		t.Fatalf("marshal binary preset envelope: %v", err)
	}
	for _, field := range []string{`"propData":{`, `"colorData":{`, `"bodyData":{`} {
		if !bytes.Contains(data, []byte(field)) {
			t.Fatalf("typed preset JSON lacks %s: %s", field, data)
		}
	}
	if bytes.Contains(data, []byte(`"propData":"`)) || bytes.Contains(data, []byte(`"colorData":"`)) || bytes.Contains(data, []byte(`"bodyData":"`)) {
		t.Fatalf("binary preset envelope exposed Base64 inner data: %s", data)
	}

	var decoded KCESPreset
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal typed preset JSON into binary envelope: %v", err)
	}
	if len(decoded.MaidData.PropData) == 0 || len(decoded.MaidData.ColorData) == 0 || len(decoded.MaidData.BodyData) == 0 {
		t.Fatalf("typed preset JSON did not rebuild all binary blocks: %+v", decoded.MaidData)
	}

	rawFallback := []byte(`{"format":"kces-virtual-directory-preset","containerVersion":1000,"thumbnail":null,"maidData":{"version":1000,"propData":"AA==","colorData":"AA==","bodyData":"AA=="}}`)
	if err := json.Unmarshal(rawFallback, &decoded); err == nil || !strings.Contains(err.Error(), "cannot unmarshal") && !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("raw preset Base64 fallback error = %v", err)
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

func TestExpandedKCESPresetJSONRequiresNonNullMaidData(t *testing.T) {
	for _, data := range []string{
		`{"format":"kces-virtual-directory-preset","containerVersion":1000,"thumbnail":null}`,
		`{"format":"kces-virtual-directory-preset","containerVersion":1000,"thumbnail":null,"maidData":null}`,
	} {
		var value ExpandedKCESPreset
		if err := json.Unmarshal([]byte(data), &value); err == nil {
			t.Fatalf("json.Unmarshal(%s) unexpectedly succeeded", data)
		}
	}
}

func TestExpandedKCESPresetJSONRequiresEveryMaidDataField(t *testing.T) {
	invalid := []string{
		`{"format":"kces-virtual-directory-preset","containerVersion":1000,"thumbnail":null,"maidData":{"propData":null,"colorData":null,"bodyData":null}}`,
		`{"format":"kces-virtual-directory-preset","containerVersion":1000,"thumbnail":null,"maidData":{"version":1000,"colorData":null,"bodyData":null}}`,
		`{"format":"kces-virtual-directory-preset","containerVersion":1000,"thumbnail":null,"maidData":{"version":1000,"propData":null,"bodyData":null}}`,
		`{"format":"kces-virtual-directory-preset","containerVersion":1000,"thumbnail":null,"maidData":{"version":1000,"propData":null,"colorData":null}}`,
	}
	for _, data := range invalid {
		var value ExpandedKCESPreset
		if err := json.Unmarshal([]byte(data), &value); err == nil {
			t.Fatalf("json.Unmarshal(%s) unexpectedly succeeded", data)
		}
	}

	valid := `{"format":"kces-virtual-directory-preset","containerVersion":1000,"thumbnail":null,"maidData":{"version":1000,"propData":null,"colorData":null,"bodyData":null}}`
	var value ExpandedKCESPreset
	if err := json.Unmarshal([]byte(valid), &value); err != nil {
		t.Fatalf("explicit nullable maidData fields were rejected: %v", err)
	}
}
