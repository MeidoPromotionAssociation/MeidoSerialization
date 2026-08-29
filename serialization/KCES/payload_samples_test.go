package KCES

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
)

func TestDecodeKCESPayload_FromTestdataSamples(t *testing.T) {
	pathsByExt := groupPayloadSamplesByExt(t)
	for ext, paths := range pathsByExt {
		ext := ext
		paths := paths
		t.Run(ext, func(t *testing.T) {
			for _, path := range paths {
				path := path
				t.Run(filepath.Base(path), func(t *testing.T) {
					assertPayloadSampleRoundTripDeepEqual(t, path)
				})
			}
		})
	}
}

func groupPayloadSamplesByExt(t *testing.T) map[string][]string {
	t.Helper()
	paths := kcesfixtures.PayloadSamplePaths(t)

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
	value, err := DecodeKCESPayload(data, name)
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	assertPayloadRootStrict(t, value, name)

	encoded, err := EncodeKCESPayload(value, name)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	decoded, err := DecodeKCESPayload(encoded, name)
	if err != nil {
		t.Fatalf("re-decode %s: %v", name, err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("%s changed after decode/encode/decode: got %#v, want %#v", name, decoded, value)
	}
}

func assertPayloadRootStrict(t *testing.T, value any, name string) {
	t.Helper()
	ext := NormalizeKCESPayloadExtension(name)
	switch payloadKindForExtension(ext) {
	case PayloadKindDynamicBoneStatus:
		status, ok := value.(*DynamicBoneStatus)
		if !ok || status == nil || status.Version == 0 {
			t.Fatalf("missing DynamicBoneStatus root: %#v", value)
		}
	case PayloadKindJSONString:
		document, ok := value.(*MagicaClothSerializeData)
		if !ok || document == nil {
			t.Fatalf("missing MagicaCloth ClothSerializeData root: %#v", value)
		}
		if document.ClothType == nil {
			t.Fatalf("MagicaCloth ClothSerializeData is missing clothType: %#v", document)
		}
	case PayloadKindColliderPackage:
		pkg, ok := value.(*ColliderPackage)
		if !ok || pkg == nil || pkg.Version == 0 || len(pkg.Colliders) == 0 {
			t.Fatalf("missing ColliderPackage root: %#v", value)
		}
		assertColliderPackageSampleFields(t, name, pkg)
	case PayloadKindLimbCollider:
		pkg, ok := value.(*LimbColliderPackage)
		if !ok || pkg == nil || pkg.Version == 0 || len(pkg.Items) == 0 {
			t.Fatalf("missing LimbColliderPackage root: %#v", value)
		}
		assertLimbColliderSampleFields(t, name, pkg)
	case PayloadKindIKCollider:
		pkg, ok := value.(*IKColliderPackage)
		if !ok || pkg == nil || pkg.Version == 0 || len(pkg.Groups) == 0 {
			t.Fatalf("missing IKColliderPackage root: %#v", value)
		}
	case PayloadKindClothParams:
		params, ok := value.(*ClothParams)
		if !ok || params == nil {
			t.Fatalf("missing ClothParams root: %#v", value)
		}
	default:
		t.Fatalf("unsupported payload extension %q", ext)
	}
}

func assertColliderPackageSampleFields(t *testing.T, name string, pkg *ColliderPackage) {
	t.Helper()
	if name != "default_acckami_col.dbcol" {
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
		// Key(13) 至 Key(15) 在当前 C# 类型中没有成员，游戏解码时跳过，本库按线格式类型逐值保留
		// Key(13) through Key(15) have no member in the current C# type and the game skips them while decoding, so this library preserves each value according to its wire type
		if maidProp.Reserved13 == nil || *maidProp.Reserved13 != 7 {
			t.Fatalf("%s reserved13 got %v, want the stored 7", name, maidProp.Reserved13)
		}
		if maidProp.Reserved14 == nil {
			t.Fatalf("%s reserved14 is absent, want a stored Single", name)
		}
		if maidProp.Reserved15 == nil || *maidProp.Reserved15 != (Vector3{}) {
			t.Fatalf("%s reserved15 got %v, want the stored zero Vector3", name, maidProp.Reserved15)
		}
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
