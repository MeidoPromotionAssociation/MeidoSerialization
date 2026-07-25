package aba

import (
	"bytes"
	"testing"
)

func TestEncodeLegacyAssetValueForUnity2022MatchesRealTypeTrees(t *testing.T) {
	tests := []struct {
		name     string
		classID  int32
		source   string
		target   string
		validate func(*testing.T, *TypeTreeValue)
	}{
		{
			name:    "Texture2D",
			classID: ClassIDTexture2D,
			source:  "parts_bcc2_gp003.aba",
			target:  "parts_personal_om015_gp003.aba",
			validate: func(t *testing.T, root *TypeTreeValue) {
				if root.Field("m_IgnoreMasterTextureLimit") != nil {
					t.Fatal("legacy mipmap-limit field remains")
				}
				if root.Field("m_IgnoreMipmapLimit") == nil || root.Field("m_MipmapLimitGroupName") == nil {
					t.Fatal("Unity 2022.3 mipmap-limit fields are missing")
				}
			},
		},
		{
			name:    "Mesh",
			classID: ClassIDMesh,
			source:  "parts_bcc2_gp003.aba",
			target:  "parts_personal_om015_gp003.aba",
			validate: func(t *testing.T, root *TypeTreeValue) {
				value, ok := root.Field("m_CookingOptions").Int64()
				if !ok || value != int64(unity2022DefaultMeshCookingOptions) {
					t.Fatalf("m_CookingOptions = %d, %t", value, ok)
				}
			},
		},
		{
			name:    "Sprite",
			classID: ClassIDSprite,
			source:  "parts_bcc2_gp003.aba",
			target:  "parts_personal_om015_gp003.aba",
			validate: func(t *testing.T, root *TypeTreeValue) {
				validateSpriteBoneDefaults(t, root)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			af, info := sampleAssetByClass(t, tt.source, tt.classID)
			root, err := af.ReadAssetValue(info)
			if err != nil {
				t.Fatal(err)
			}
			encoded, changed, err := af.EncodeAssetValueForUnity2022(info, root)
			if err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatal("legacy schema was not changed")
			}
			targetTree := sampleTypeTree(t, tt.target, tt.classID)
			decoded := decodeObjectWithTypeTree(t, &targetTree, encoded)
			tt.validate(t, decoded)
		})
	}
}

func TestEncodeCurrentAssetValueForUnity2022DoesNotChangeSchema(t *testing.T) {
	for _, classID := range []int32{ClassIDTexture2D, ClassIDMesh, ClassIDSprite} {
		t.Run(classIDName(classID), func(t *testing.T) {
			af, info := sampleAssetByClass(t, "parts_personal_om015_gp003.aba", classID)
			root, err := af.ReadAssetValue(info)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := af.GetAssetData(info)
			if err != nil {
				t.Fatal(err)
			}
			encoded, changed, err := af.EncodeAssetValueForUnity2022(info, root)
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				t.Fatal("Unity 2022.3 schema was reported as changed")
			}
			if !bytes.Equal(encoded, raw) {
				t.Fatal("Unity 2022.3 object bytes changed after decode and encode")
			}
		})
	}
}

func TestEncodeLegacyAnimationClipValueForUnity2022(t *testing.T) {
	af, info := sampleAssetByClass(t, "motion.aba", ClassIDAnimationClip)
	root, err := af.ReadAssetValue(info)
	if err != nil {
		t.Fatal(err)
	}
	encoded, changed, err := af.EncodeAssetValueForUnity2022(info, root)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("legacy AnimationClip schema was not changed")
	}

	sourceTree, err := af.typeTreeForAsset(info)
	if err != nil {
		t.Fatal(err)
	}
	targetTree := cloneTypeTreeType(sourceTree)
	targetRoot, err := af.ReadAssetValue(info)
	if err != nil {
		t.Fatal(err)
	}
	if normalized, err := normalizeAnimationClipForUnity2022(&targetTree, targetRoot); err != nil || !normalized {
		t.Fatalf("normalize target tree = %t, %v", normalized, err)
	}
	assertAnimationClipUnity2022Tree(t, &targetTree)
	decoded := decodeObjectWithTypeTree(t, &targetTree, encoded)
	assertAnimationClipUnity2022Values(t, decoded)
}

func TestLegacyTransformBytesMatchUnity2022TypeTree(t *testing.T) {
	af, info := sampleAssetByClass(t, "bg.aba", ClassIDTransform)
	raw, err := af.GetAssetData(info)
	if err != nil {
		t.Fatal(err)
	}
	targetTree := sampleTypeTree(t, "language.aba", ClassIDTransform)
	decodeObjectWithTypeTree(t, &targetTree, raw)
}

func sampleAssetByClass(t *testing.T, sample string, classID int32) (*AssetsFile, *AssetInfo) {
	t.Helper()
	bundle, file := openAbaSample(t, sample)
	defer file.Close()
	for directoryIndex, entry := range bundle.BlockInfo.DirectoryInfos {
		if !entry.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(int64(directoryIndex))
		if err != nil {
			t.Fatal(err)
		}
		af, err := ReadAssetsFile(data)
		if err != nil {
			t.Fatal(err)
		}
		for infoIndex := range af.Metadata.AssetInfos {
			info := &af.Metadata.AssetInfos[infoIndex]
			if info.TypeId == classID {
				return af, info
			}
		}
	}
	t.Fatalf("class ID %d object not found in %s", classID, sample)
	return nil, nil
}

func classIDName(classID int32) string {
	switch classID {
	case ClassIDTexture2D:
		return "Texture2D"
	case ClassIDMesh:
		return "Mesh"
	case ClassIDSprite:
		return "Sprite"
	default:
		return "unknown"
	}
}

func validateSpriteBoneDefaults(t *testing.T, value *TypeTreeValue) {
	t.Helper()
	if value == nil {
		return
	}
	if value.TypeName == "SpriteBone" {
		guid, guidOK := value.Field("guid").String()
		color, colorOK := value.Field("color").Field("rgba").UInt64()
		if !guidOK || guid != "" || !colorOK || color != uint64(^uint32(0)) {
			t.Fatalf("SpriteBone defaults = guid %q/%t color %#x/%t", guid, guidOK, color, colorOK)
		}
	}
	for _, child := range value.Children {
		validateSpriteBoneDefaults(t, child)
	}
}

func assertAnimationClipUnity2022Tree(t *testing.T, tree *TypeTreeType) {
	t.Helper()
	for _, typeName := range []string{"FloatCurve", "PPtrCurve"} {
		parentIndex, ok := findTypeTreeNodeByType(tree, typeName)
		if !ok {
			t.Fatalf("%s missing", typeName)
		}
		if _, _, ok := findDirectTypeTreeChild(tree, parentIndex, "int", "flags"); !ok {
			t.Fatalf("%s.flags missing", typeName)
		}
	}
	bindingIndex, ok := findTypeTreeNodeByType(tree, "GenericBinding")
	if !ok {
		t.Fatal("GenericBinding missing")
	}
	if tree.Nodes[bindingIndex].ByteSize != 27 {
		t.Fatalf("GenericBinding byte size = %d, want 27", tree.Nodes[bindingIndex].ByteSize)
	}
	pptrIndex, _, ok := findDirectTypeTreeChild(tree, bindingIndex, "UInt8", "isPPtrCurve")
	if !ok || tree.Nodes[pptrIndex].MetaFlags&0x4000 != 0 {
		t.Fatal("GenericBinding.isPPtrCurve is missing or still aligned")
	}
	intIndex, _, ok := findDirectTypeTreeChild(tree, bindingIndex, "UInt8", "isIntCurve")
	if !ok || tree.Nodes[intIndex].MetaFlags&0x4000 == 0 {
		t.Fatal("GenericBinding.isIntCurve is missing or not aligned")
	}
}

func assertAnimationClipUnity2022Values(t *testing.T, value *TypeTreeValue) {
	t.Helper()
	if value == nil {
		return
	}
	switch value.TypeName {
	case "FloatCurve", "PPtrCurve":
		flags, ok := value.Field("flags").Int64()
		if !ok || flags != 0 {
			t.Fatalf("%s.flags = %d, %t", value.TypeName, flags, ok)
		}
	case "GenericBinding":
		isIntCurve, ok := value.Field("isIntCurve").UInt64()
		if !ok || isIntCurve != 0 {
			t.Fatalf("GenericBinding.isIntCurve = %d, %t", isIntCurve, ok)
		}
	}
	for _, child := range value.Children {
		assertAnimationClipUnity2022Values(t, child)
	}
}
