package aba

import (
	"bytes"
	"strconv"
	"testing"
)

// animationClipWalkValues 先序遍历值树
// animationClipWalkValues walks a value tree in preorder
func animationClipWalkValues(value *TypeTreeValue, visit func(*TypeTreeValue)) {
	if value == nil {
		return
	}
	visit(value)
	for _, child := range value.Children {
		animationClipWalkValues(child, visit)
	}
}

// animationClipFindByName 返回首个字段名匹配的值节点
// animationClipFindByName returns the first value node whose field name matches
func animationClipFindByName(t *testing.T, root *TypeTreeValue, name string) *TypeTreeValue {
	t.Helper()
	var found *TypeTreeValue
	animationClipWalkValues(root, func(value *TypeTreeValue) {
		if found == nil && value.Name == name {
			found = value
		}
	})
	if found == nil {
		t.Fatalf("AnimationClip value tree has no field %q", name)
	}
	return found
}

// animationClipChildInt 读取值节点的直接整数字段
// animationClipChildInt reads a direct integer field of a value node
func animationClipChildInt(t *testing.T, parent *TypeTreeValue, name string) int64 {
	t.Helper()
	for _, child := range parent.Children {
		if child.Name != name {
			continue
		}
		value, ok := child.Int64()
		if !ok {
			t.Fatalf("%s.%s holds %T instead of an integer", parent.Name, name, child.Value)
		}
		return value
	}
	t.Fatalf("value %s has no field %s", parent.Name, name)
	return 0
}

// animationClipSetChildInt 设置值节点的直接整数字段
// animationClipSetChildInt sets a direct integer field of a value node
func animationClipSetChildInt(t *testing.T, parent *TypeTreeValue, name string, value int64) {
	t.Helper()
	for _, child := range parent.Children {
		if child.Name == name {
			child.Value = value
			return
		}
	}
	t.Fatalf("value %s has no field %s", parent.Name, name)
}

// animationClipChildInts 收集值树中指定类型节点的直接整数字段
// animationClipChildInts collects a direct integer field from every node of a type in a value tree
func animationClipChildInts(t *testing.T, root *TypeTreeValue, typeName string, field string) []int64 {
	t.Helper()
	var out []int64
	patchValuesByType(root, typeName, func(value *TypeTreeValue) {
		for _, child := range value.Children {
			if child.Name != field {
				continue
			}
			count, ok := child.Int64()
			if !ok {
				t.Fatalf("%s.%s holds %T instead of an integer", typeName, field, child.Value)
			}
			out = append(out, count)
		}
	})
	return out
}

// animationClipSetStreamedClipCurveCount 把值树中每个 StreamedClip 的 curveCount 设为 count 并返回改动次数
// animationClipSetStreamedClipCurveCount sets every StreamedClip curveCount in a value tree to count and returns the number of edits
func animationClipSetStreamedClipCurveCount(t *testing.T, root *TypeTreeValue, count int64) int {
	t.Helper()
	patched := 0
	patchValuesByType(root, "StreamedClip", func(value *TypeTreeValue) {
		for _, child := range value.Children {
			if child.Name == "curveCount" {
				child.Value = count
				patched++
			}
		}
	})
	return patched
}

// animationClipSkeleton 用旧版 AnimationClip 树生成零值骨架
// animationClipSkeleton builds a zeroed value skeleton from a legacy AnimationClip tree
func animationClipSkeleton(t *testing.T, tree *TypeTreeType) *TypeTreeValue {
	t.Helper()
	root, next, err := newTypeTreeValueSkeleton(tree, 0)
	if err != nil {
		t.Fatalf("build AnimationClip value skeleton: %v", err)
	}
	if next != int64(len(tree.Nodes)) {
		t.Fatalf("AnimationClip skeleton consumed %d of %d TypeTree nodes", next, len(tree.Nodes))
	}
	return root
}

// animationClipMigrationSamples 打开旧版与官方样本，并返回旧版 AssetsFile、资产信息和官方 TypeTree
// animationClipMigrationSamples opens the legacy and official samples and returns the legacy AssetsFile, asset info, and official TypeTree
func animationClipMigrationSamples(t *testing.T) (*AssetsFile, *AssetInfo, TypeTreeType) {
	t.Helper()
	legacyBundle, legacyFile := openAbaSample(t, "motion.aba")
	t.Cleanup(func() { legacyFile.Close() })
	assetsFile, info := findAssetByClassInAba(t, "motion.aba", legacyBundle, ClassIDAnimationClip)

	officialBundle, officialFile := openAbaSample(t, "motion_kces2.aba")
	t.Cleanup(func() { officialFile.Close() })
	official := findTypeTreeInAba(t, "motion_kces2.aba", officialBundle, ClassIDAnimationClip)
	return assetsFile, info, official
}

// TestAnimationClipMigrationPreservesStreamedClipCurveCount 校验旧版 curveCount 收窄后取值不变
// 样本里的 AnimationClip 全部没有曲线数据（curveCount 恒为零），无法证明收窄写出的低十六位真的保留了原值
// 因此这里用真实旧版树生成骨架并填入非零计数，再走完整的编码 → 迁移 → 官方树解码往返
// TestAnimationClipMigrationPreservesStreamedClipCurveCount verifies that narrowing the legacy curveCount preserves its value
// Every AnimationClip in the samples carries no curve data (curveCount is always zero), which cannot prove that the narrowed low sixteen bits really keep the original value
// This test therefore builds a skeleton from the real legacy tree, fills in a non-zero count, and runs the full encode -> migrate -> official-tree decode round trip
func TestAnimationClipMigrationPreservesStreamedClipCurveCount(t *testing.T) {
	assetsFile, info, official := animationClipMigrationSamples(t)
	legacyTree, err := assetsFile.AssetTypeTree(info)
	if err != nil {
		t.Fatal(err)
	}

	for _, count := range []int64{0, 1, 162, 4096, 65535} {
		t.Run("curveCount="+strconv.FormatInt(count, 10), func(t *testing.T) {
			root := animationClipSkeleton(t, &legacyTree)
			if patched := animationClipSetStreamedClipCurveCount(t, root, count); patched == 0 {
				t.Fatal("AnimationClip skeleton has no StreamedClip curveCount field")
			}

			// 旧版树编码发生在迁移之前，此时 root 仍是旧版形状
			// The legacy-tree encoding happens before migration, while root still holds the legacy shape
			legacyEncoded, err := assetsFile.EncodeAssetValue(info, root)
			if err != nil {
				t.Fatalf("encode with legacy tree: %v", err)
			}

			migrated, migratedTree, changed, err := assetsFile.EncodeAssetValueAndTypeTreeForUnity2022(info, root)
			if err != nil {
				t.Fatalf("migrate curveCount=%d: %v", count, err)
			}
			if !changed {
				t.Fatal("legacy AnimationClip schema was not migrated")
			}

			// 收窄成 UInt16 后紧跟一个零值 UInt16，与旧版四字节小端布局逐字节等价
			// Narrowing into UInt16 followed by a zero UInt16 is byte-for-byte equivalent to the legacy four-byte little-endian layout
			if !bytes.Equal(legacyEncoded, migrated) {
				t.Fatalf("curveCount=%d: migrated payload differs from legacy payload\nlegacy=%x\nmigrated=%x",
					count, legacyEncoded, migrated)
			}

			decoded := decodeObjectWithTypeTree(t, &official, migrated)
			curveCounts := animationClipChildInts(t, decoded, "StreamedClip", "curveCount")
			discreteCounts := animationClipChildInts(t, decoded, "StreamedClip", "discreteCurveCount")
			if len(curveCounts) == 0 {
				t.Fatal("decoded AnimationClip has no StreamedClip curveCount")
			}
			if len(discreteCounts) != len(curveCounts) {
				t.Fatalf("discreteCurveCount count %d != curveCount count %d", len(discreteCounts), len(curveCounts))
			}
			for i, got := range curveCounts {
				if got != count {
					t.Fatalf("curveCount[%d] = %d, want %d", i, got, count)
				}
			}
			for i, got := range discreteCounts {
				if got != 0 {
					t.Fatalf("discreteCurveCount[%d] = %d, want 0", i, got)
				}
			}
			if len(migratedTree.Nodes) != len(official.Nodes) {
				t.Fatalf("migrated tree nodes %d != official %d", len(migratedTree.Nodes), len(official.Nodes))
			}
			if migratedTree.TypeHash != official.TypeHash {
				t.Fatalf("migrated TypeHash = %x, want official %x", migratedTree.TypeHash, official.TypeHash)
			}
		})
	}
}

// TestAnimationClipMigrationPreservesBindingAndCurveValues 校验迁移补入的 GenericBinding 与曲线字段取值正确
// 样本里既没有 GenericBinding 也没有 FloatCurve/PPtrCurve 实例，新增字段的字节布局因此从未被实际写出过
// 这里为三类数组各构造一个元素并填入非零的绑定标识，再断言官方树解码后新增字段为零、原有字段原样保留
// TestAnimationClipMigrationPreservesBindingAndCurveValues verifies that the fields the migration adds to GenericBinding and the curve types hold correct values
// The samples contain neither a GenericBinding nor a FloatCurve/PPtrCurve instance, so the byte layout of the added fields has never actually been emitted
// This test builds one element for each of the three arrays, fills in non-zero binding identifiers, and asserts that the official tree decodes the added fields as zero while the original fields survive unchanged
func TestAnimationClipMigrationPreservesBindingAndCurveValues(t *testing.T) {
	assetsFile, info, official := animationClipMigrationSamples(t)
	legacyTree, err := assetsFile.AssetTypeTree(info)
	if err != nil {
		t.Fatal(err)
	}

	root := animationClipSkeleton(t, &legacyTree)

	bindings := animationClipFindByName(t, root, "genericBindings")
	binding, err := newMeshArrayElement(&legacyTree, bindings, 0)
	if err != nil {
		t.Fatalf("build GenericBinding element: %v", err)
	}
	bindings.Children = append(bindings.Children, binding)
	animationClipSetChildInt(t, binding, "path", 12345)
	animationClipSetChildInt(t, binding, "attribute", 678)
	animationClipSetChildInt(t, binding, "typeID", 74)
	animationClipSetChildInt(t, binding, "customType", 1)

	for _, field := range []string{"m_FloatCurves", "m_PPtrCurves"} {
		array := animationClipFindByName(t, root, field)
		element, err := newMeshArrayElement(&legacyTree, array, 0)
		if err != nil {
			t.Fatalf("build %s element: %v", field, err)
		}
		array.Children = append(array.Children, element)
	}

	if patched := animationClipSetStreamedClipCurveCount(t, root, 42); patched == 0 {
		t.Fatal("AnimationClip skeleton has no StreamedClip curveCount field")
	}

	legacyEncoded, err := assetsFile.EncodeAssetValue(info, root)
	if err != nil {
		t.Fatalf("encode with legacy tree: %v", err)
	}
	migrated, migratedTree, changed, err := assetsFile.EncodeAssetValueAndTypeTreeForUnity2022(info, root)
	if err != nil {
		t.Fatalf("migrate AnimationClip with binding and curve elements: %v", err)
	}
	if !changed {
		t.Fatal("legacy AnimationClip schema was not migrated")
	}

	// 新增字段会真实加宽 GenericBinding 与两类曲线，因此这里不再期望字节等价，只要求官方树完整解码
	// The added fields genuinely widen GenericBinding and both curve types, so byte equality is no longer expected here; the official tree must still decode completely
	decoded := decodeObjectWithTypeTree(t, &official, migrated)

	for _, curveType := range []string{"FloatCurve", "PPtrCurve"} {
		flags := animationClipChildInts(t, decoded, curveType, "flags")
		if len(flags) != 1 {
			t.Fatalf("%s count = %d, want 1", curveType, len(flags))
		}
		if flags[0] != 0 {
			t.Fatalf("%s flags = %d, want 0", curveType, flags[0])
		}
	}

	intCurves := animationClipChildInts(t, decoded, "GenericBinding", "isIntCurve")
	serializeReference := animationClipChildInts(t, decoded, "GenericBinding", "isSerializeReferenceCurve")
	if len(intCurves) != 1 || len(serializeReference) != 1 {
		t.Fatalf("GenericBinding added fields count = %d/%d, want 1/1", len(intCurves), len(serializeReference))
	}
	if intCurves[0] != 0 {
		t.Fatalf("isIntCurve = %d, want 0", intCurves[0])
	}
	if serializeReference[0] != 0 {
		t.Fatalf("isSerializeReferenceCurve = %d, want 0", serializeReference[0])
	}

	// 原有绑定标识必须原样保留，且 isPPtrCurve 仍为假
	// The original binding identifiers must survive unchanged and isPPtrCurve must stay false
	var decodedBinding *TypeTreeValue
	patchValuesByType(decoded, "GenericBinding", func(value *TypeTreeValue) {
		if decodedBinding == nil {
			decodedBinding = value
		}
	})
	if decodedBinding == nil {
		t.Fatal("decoded AnimationClip has no GenericBinding")
	}
	for name, want := range map[string]int64{"path": 12345, "attribute": 678, "typeID": 74, "customType": 1, "isPPtrCurve": 0} {
		if got := animationClipChildInt(t, decodedBinding, name); got != want {
			t.Fatalf("GenericBinding.%s = %d, want %d", name, got, want)
		}
	}

	// 迁移必须真的加宽正文，否则说明新增字段没有写出去
	// The migration must genuinely widen the payload, otherwise the added fields were never emitted
	if len(migrated) <= len(legacyEncoded) {
		t.Fatalf("migrated payload %d bytes is not wider than the legacy payload %d bytes", len(migrated), len(legacyEncoded))
	}

	// 数组级的 AlignBytes 会把元素内部对齐的差异吸收掉，因此字节相同并不能证明 AlignBytes 搬到了正确节点
	// 只有 TypeHash 会随 MetaFlags 变化，所以这里必须单独校验哈希才能锁住 AlignBytes 的归属
	// The array-level AlignBytes absorbs the difference between the elements' internal alignment, so identical bytes do not prove that AlignBytes moved onto the right node
	// Only the TypeHash reacts to MetaFlags, so the hash must be asserted separately to pin down where AlignBytes belongs
	if migratedTree.TypeHash != official.TypeHash {
		t.Fatalf("migrated TypeHash = %x, want official %x", migratedTree.TypeHash, official.TypeHash)
	}
}

// TestAnimationClipMigrationRejectsCurveCountBeyondUInt16 校验超出 UInt16 的 curveCount 被拒绝而不是静默截断
// TestAnimationClipMigrationRejectsCurveCountBeyondUInt16 verifies that a curveCount beyond UInt16 is refused instead of being silently truncated
func TestAnimationClipMigrationRejectsCurveCountBeyondUInt16(t *testing.T) {
	assetsFile, info, _ := animationClipMigrationSamples(t)
	legacyTree, err := assetsFile.AssetTypeTree(info)
	if err != nil {
		t.Fatal(err)
	}

	root := animationClipSkeleton(t, &legacyTree)
	if patched := animationClipSetStreamedClipCurveCount(t, root, 65536); patched == 0 {
		t.Fatal("AnimationClip skeleton has no StreamedClip curveCount field")
	}
	if _, _, _, err := assetsFile.EncodeAssetValueAndTypeTreeForUnity2022(info, root); err == nil {
		t.Fatal("curveCount 65536 was migrated instead of being rejected")
	}
}
