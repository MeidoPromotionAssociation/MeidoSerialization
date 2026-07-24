package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var payloadSamplesWithUnsupportedSparseMaidPropSlots = map[string]struct{}{
	"default_accmimi_col.dbcol": {},
	"default_yure_col.dbcol":    {},
}

func TestDecodeKCESPayload_FromTestdataSamples(t *testing.T) {
	pathsByExt := groupPayloadSamplesByExt(t)
	for ext, paths := range pathsByExt {
		ext := ext
		paths := paths
		t.Run(ext, func(t *testing.T) {
			for _, path := range paths {
				path := path
				t.Run(filepath.Base(path), func(t *testing.T) {
					if _, unsupported := payloadSamplesWithUnsupportedSparseMaidPropSlots[filepath.Base(path)]; unsupported {
						assertPayloadSampleRejectsSparseMaidPropSlots(t, path)
						return
					}
					assertPayloadSampleRoundTripDeepEqual(t, path)
				})
			}
		})
	}
}

func assertPayloadSampleRejectsSparseMaidPropSlots(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read payload sample %s: %v", path, err)
	}
	_, err = DecodeKCESPayload(data, filepath.Base(path))
	if err == nil || !strings.Contains(err.Error(), "sparse slot 13 must be nil") {
		t.Fatalf("DecodeKCESPayload() error = %v, want non-nil undeclared MaidProp slot rejection", err)
	}
}

func groupPayloadSamplesByExt(t *testing.T) map[string][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(payloadSampleDir(), "*"))
	if err != nil {
		t.Fatalf("glob payload samples: %v", err)
	}
	if len(paths) == 0 {
		t.Skip("no payload samples found in testdata/kces_payload")
	}

	pathsByExt := map[string][]string{}
	for _, path := range paths {
		name := filepath.Base(path)
		ext := NormalizeKCESPayloadExtension(name)
		if ext == "" {
			t.Fatalf("unexpected payload sample %q", name)
		}
		if kind := payloadKindForExtension(ext); kind == "" {
			t.Fatalf("no payload kind for suffix %q sample %q", ext, name)
		}
		pathsByExt[ext] = append(pathsByExt[ext], path)
	}
	return pathsByExt
}

func assertPayloadSampleRoundTripDeepEqual(t *testing.T, path string) {
	t.Helper()
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read payload sample %s: %v", path, err)
	}
	env, err := DecodeKCESPayload(data, name)
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	assertPayloadEnvelopeStrict(t, env, name)

	encoded, err := EncodeKCESPayload(env)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	decoded, err := DecodeKCESPayload(encoded, name)
	if err != nil {
		t.Fatalf("re-decode %s: %v", name, err)
	}
	if !reflect.DeepEqual(decoded, env) {
		t.Fatalf("%s changed after decode/encode/decode: got %#v, want %#v", name, decoded, env)
	}
}

func assertPayloadEnvelopeStrict(t *testing.T, env *KCESPayloadEnvelope, name string) {
	t.Helper()
	ext := NormalizeKCESPayloadExtension(name)
	if env.Format != PayloadFormatKCESMessagePack {
		t.Fatalf("format got %q, want %q", env.Format, PayloadFormatKCESMessagePack)
	}
	if env.Extension != ext {
		t.Fatalf("extension got %q, want %q", env.Extension, ext)
	}
	wantKind := payloadKindForExtension(ext)
	if env.Kind != wantKind {
		t.Fatalf("kind got %q, want %q", env.Kind, wantKind)
	}
	switch wantKind {
	case PayloadKindDynamicBoneStatus:
		if env.DynamicBone == nil || env.DynamicBone.Version == 0 {
			t.Fatalf("missing dynamicBoneStatus: %+v", env)
		}
	case PayloadKindJSONString:
		if len(env.JSON) == 0 {
			t.Fatalf("missing JSON string payload: %+v", env)
		}
		if !json.Valid(env.JSON) {
			t.Fatalf("JSON string field is invalid: %s", env.JSON)
		}
	case PayloadKindColliderPackage:
		if env.ColliderPackage == nil || env.ColliderPackage.Version == 0 || len(env.ColliderPackage.Colliders) == 0 {
			t.Fatalf("missing colliderPackage: %+v", env)
		}
		assertColliderPackageSampleFields(t, name, env.ColliderPackage)
	case PayloadKindLimbCollider:
		if env.LimbCollider == nil || env.LimbCollider.Version == 0 || len(env.LimbCollider.Items) == 0 {
			t.Fatalf("missing limbColliderPackage: %+v", env)
		}
		assertLimbColliderSampleFields(t, name, env.LimbCollider)
	case PayloadKindIKCollider:
		if env.IKCollider == nil || env.IKCollider.Version == 0 || len(env.IKCollider.Groups) == 0 {
			t.Fatalf("missing ikColliderPackage: %+v", env)
		}
	case PayloadKindClothParams:
		if env.ClothParams == nil {
			t.Fatalf("missing clothParams: %+v", env)
		}
	default:
		t.Fatalf("unsupported payload kind %q", wantKind)
	}
}

func assertColliderPackageSampleFields(t *testing.T, name string, pkg *ColliderPackage) {
	t.Helper()
	if name != "default_accmimi_col.dbcol" {
		return
	}
	if len(pkg.Colliders) != 12 || len(pkg.LimbEnableList) != 8 {
		t.Fatalf("%s counts got colliders=%d limbEnableList=%d, want 12/8", name, len(pkg.Colliders), len(pkg.LimbEnableList))
	}
	for _, ref := range pkg.Colliders {
		maidProp, ok := ref.Collider.(*ColliderMaidProp)
		if !ok {
			continue
		}
		if ref.Type != ColliderTypeMaidProp || maidProp.Version != 1001 {
			t.Fatalf("%s maidProp metadata got type=%d version=%d, want stored 3/1001", name, ref.Type, maidProp.Version)
		}
		assertIntSliceEqual(t, name+" centerMpnList", maidProp.CenterMpnList, []int32{7})
		assertIntSliceEqual(t, name+" startRadiusMpnList", maidProp.StartRadiusMpnList, []int32{7})
		assertIntSliceEqual(t, name+" endRadiusMpnList", maidProp.EndRadiusMpnList, []int32{7})
		return
	}
	t.Fatalf("%s did not contain a MaidProp collider", name)
}

func assertLimbColliderSampleFields(t *testing.T, name string, pkg *LimbColliderPackage) {
	t.Helper()
	if name != "limbconf.limbcol" {
		return
	}
	if len(pkg.Items) != 8 {
		t.Fatalf("%s item count got %d, want 8", name, len(pkg.Items))
	}
	first := pkg.Items[0].Collider
	if first == nil {
		t.Fatalf("%s first collider is nil", name)
	}
	if pkg.Items[0].Target != 0 || first.ParentName == nil || *first.ParentName != "Bip01 L UpperArm" ||
		first.SelfName == nil || *first.SelfName != "UpperArm_L LimbCollider" {
		t.Fatalf("%s first item mismatch: target=%d collider=%+v", name, pkg.Items[0].Target, first.ColliderObject)
	}
	for _, item := range pkg.Items {
		if item.Target != 4 {
			continue
		}
		maidProp := item.Collider
		if maidProp == nil {
			t.Fatalf("%s target 4 collider is nil", name)
		}
		assertIntSliceEqual(t, name+" target4 centerMpnList", maidProp.CenterMpnList, []int32{})
		assertIntSliceEqual(t, name+" target4 startRadiusMpnList", maidProp.StartRadiusMpnList, []int32{40, 41})
		assertIntSliceEqual(t, name+" target4 endRadiusMpnList", maidProp.EndRadiusMpnList, []int32{42, 43})
		return
	}
	t.Fatalf("%s did not contain target 4 limb collider", name)
}

func assertIntSliceEqual(t *testing.T, name string, got, want []int32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s got %v, want %v", name, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s got %v, want %v", name, got, want)
		}
	}
}

func readPayloadSample(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(payloadSampleDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read payload sample %s: %v", path, err)
	}
	return data
}

func payloadSampleDir() string {
	return filepath.Join("..", "..", "testdata", "kces_payload")
}
