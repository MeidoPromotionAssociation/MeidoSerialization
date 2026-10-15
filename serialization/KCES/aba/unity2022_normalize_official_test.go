package aba

import (
	"os"
	"path/filepath"
	"testing"
)

// normalizeTestNode 是忽略字符串偏移的逻辑节点视图
// 布局迁移会向 StringBuffer 追加字符串，迁移树与内置树因此拥有不同的缓冲区与偏移，只能按解析后的逻辑内容比较
// normalizeTestNode is a logical node view that ignores string offsets
// Layout migration appends strings to the StringBuffer, so a migrated tree and a built-in tree hold different buffers and offsets and can only be compared by resolved logical content
type normalizeTestNode struct {
	typeName  string
	fieldName string
	level     byte
	byteSize  int32
	typeFlags byte
	metaFlags uint32
	version   uint16
}

// logicalTypeTreeNodes 把节点表投影为忽略偏移的逻辑节点序列
// logicalTypeTreeNodes projects a node table into a logical node sequence that ignores offsets
func logicalTypeTreeNodes(tree *TypeTreeType) []normalizeTestNode {
	nodes := make([]normalizeTestNode, 0, len(tree.Nodes))
	for nodeIndex := range tree.Nodes {
		node := &tree.Nodes[nodeIndex]
		nodes = append(nodes, normalizeTestNode{
			typeName:  tree.GetTypeTreeString(node, true),
			fieldName: tree.GetTypeTreeString(node, false),
			level:     node.Level,
			byteSize:  node.ByteSize,
			typeFlags: node.TypeFlags,
			metaFlags: node.MetaFlags,
			version:   node.Version,
		})
	}
	return nodes
}

// findTypeTreeNodeByFieldName 返回首个字段名匹配的节点索引，未找到时返回负一
// findTypeTreeNodeByFieldName returns the first node index whose field name matches, or negative one when absent
func findTypeTreeNodeByFieldName(tree *TypeTreeType, fieldName string) int64 {
	for nodeIndex := range tree.Nodes {
		if tree.GetTypeTreeString(&tree.Nodes[nodeIndex], false) == fieldName {
			return int64(nodeIndex)
		}
	}
	return -1
}

// assertTypeTreesLogicallyEqual 断言两棵树拥有逐字段一致的逻辑节点序列，并在不一致时报告首个差异
// assertTypeTreesLogicallyEqual asserts that two trees hold field-by-field identical logical node sequences and reports the first difference
func assertTypeTreesLogicallyEqual(t *testing.T, label string, got *TypeTreeType, want *TypeTreeType) {
	t.Helper()
	gotNodes := logicalTypeTreeNodes(got)
	wantNodes := logicalTypeTreeNodes(want)
	if len(gotNodes) != len(wantNodes) {
		t.Fatalf("%s node count = %d, want %d", label, len(gotNodes), len(wantNodes))
	}
	for nodeIndex := range wantNodes {
		if gotNodes[nodeIndex] != wantNodes[nodeIndex] {
			t.Fatalf("%s node[%d] = %+v, want %+v", label, nodeIndex, gotNodes[nodeIndex], wantNodes[nodeIndex])
		}
	}
}

// legacyTexture2DTreeForMigrationTest 把官方 Texture2D 树退回迁移前的旧版形状，并返回旧版限制字段的节点索引
// 退回操作是迁移插入动作的严格逆运算：清除 AlignBytes、把字段名改回 m_IgnoreMasterTextureLimit、删掉新增的空组名字段子树
// legacyTexture2DTreeForMigrationTest reverts the official Texture2D tree into its pre-migration legacy shape and returns the node index of the legacy limit field
// The revert is the exact inverse of the migration's insert: it clears AlignBytes, renames the field back to m_IgnoreMasterTextureLimit, and removes the added empty group-name subtree
func legacyTexture2DTreeForMigrationTest(t *testing.T, official *TypeTreeType) (TypeTreeType, int64) {
	t.Helper()
	legacy := cloneTypeTreeType(official)

	limitIndex := findTypeTreeNodeByFieldName(&legacy, "m_IgnoreMipmapLimit")
	if limitIndex < 0 {
		t.Fatal("official Texture2D tree has no m_IgnoreMipmapLimit")
	}
	groupIndex := findTypeTreeNodeByFieldName(&legacy, "m_MipmapLimitGroupName")
	if groupIndex < 0 {
		t.Fatal("official Texture2D tree has no m_MipmapLimitGroupName")
	}
	if groupIndex <= limitIndex {
		t.Fatalf("m_MipmapLimitGroupName node %d does not follow m_IgnoreMipmapLimit node %d", groupIndex, limitIndex)
	}
	legacy.Nodes[limitIndex].MetaFlags &^= typeHashAlignFlag
	if err := setTypeTreeNodeName(&legacy, &legacy.Nodes[limitIndex], "m_IgnoreMasterTextureLimit"); err != nil {
		t.Fatal(err)
	}
	legacy.Nodes = append(legacy.Nodes[:groupIndex], legacy.Nodes[skipSubtree(&legacy, groupIndex):]...)
	return legacy, limitIndex
}

// officialTypeTreeFromAbaSample 从 testdata/aba 的官方样本读取指定 ClassID 的 TypeTree
// 该目录被 .gitignore 排除，样本缺失时跳过测试，与 openAbaSample 的既有约定一致
// officialTypeTreeFromAbaSample reads the TypeTree of a ClassID from an official sample under testdata/aba
// The directory is gitignored, so a missing sample skips the test, matching the existing openAbaSample convention
func officialTypeTreeFromAbaSample(t *testing.T, sample string, classID int32) TypeTreeType {
	t.Helper()
	filePath := filepath.Join("..", "..", "..", "testdata", "aba", sample)
	f, err := os.Open(filePath)
	if err != nil {
		t.Skipf("sample %s not available: %v", sample, err)
	}
	defer f.Close()
	bundle, err := ReadAba(f)
	if err != nil {
		t.Fatalf("ReadAba(%s): %v", sample, err)
	}
	for directoryIndex, directory := range bundle.BlockInfo.DirectoryInfos {
		if !directory.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(int64(directoryIndex))
		if err != nil {
			t.Fatalf("GetFileData(%s): %v", sample, err)
		}
		assetsFile, err := ReadAssetsFile(data)
		if err != nil {
			t.Fatalf("ReadAssetsFile(%s): %v", sample, err)
		}
		for typeIndex := range assetsFile.Metadata.TypeTreeTypes {
			if assetsFile.Metadata.TypeTreeTypes[typeIndex].TypeId == classID {
				return cloneTypeTreeType(&assetsFile.Metadata.TypeTreeTypes[typeIndex])
			}
		}
	}
	t.Skipf("sample %s has no tree for class %d", sample, classID)
	return TypeTreeType{}
}

// legacySpriteTreeForMigrationTest 把官方 Sprite 树退回迁移前的旧版形状
// 退回操作是迁移插入动作的严格逆运算：删掉 SpriteBone 下新增的 guid 与 color 子树
// legacySpriteTreeForMigrationTest reverts the official Sprite tree into its pre-migration legacy shape
// The revert is the exact inverse of the migration's insert: it removes the guid and color subtrees added under SpriteBone
func legacySpriteTreeForMigrationTest(t *testing.T, official *TypeTreeType) TypeTreeType {
	t.Helper()
	legacy := cloneTypeTreeType(official)

	boneIndex, ok := findTypeTreeNode(&legacy, "SpriteBone", "data")
	if !ok {
		t.Fatal("official Sprite tree has no SpriteBone data node")
	}
	guidIndex, guidEnd, hasGUID := findDirectTypeTreeChild(&legacy, boneIndex, "string", "guid")
	colorIndex, colorEnd, hasColor := findDirectTypeTreeChild(&legacy, boneIndex, "ColorRGBA", "color")
	if !hasGUID || !hasColor {
		t.Fatalf("official Sprite tree lacks guid (%t) or color (%t)", hasGUID, hasColor)
	}

	// 先删靠后的子树，使靠前子树的索引保持有效
	// Remove the later subtree first so the earlier indexes stay valid
	firstIndex, firstEnd, secondIndex, secondEnd := guidIndex, guidEnd, colorIndex, colorEnd
	if colorIndex > guidIndex {
		firstIndex, firstEnd, secondIndex, secondEnd = colorIndex, colorEnd, guidIndex, guidEnd
	}
	legacy.Nodes = append(legacy.Nodes[:firstIndex], legacy.Nodes[firstEnd:]...)
	legacy.Nodes = append(legacy.Nodes[:secondIndex], legacy.Nodes[secondEnd:]...)
	return legacy
}

// TestUnity2022MigrationRestoresOfficialSpriteLayout 校验 Sprite 布局迁移精确还原官方 2022.3 树
// 官方基准取自 testdata/aba 样本，其 167 节点 Sprite 树在 8 份样本中哈希一致
// TestUnity2022MigrationRestoresOfficialSpriteLayout verifies that the Sprite layout migration exactly restores the official 2022.3 tree
// The baseline comes from testdata/aba samples, whose 167-node Sprite tree hashes identically across eight samples
func TestUnity2022MigrationRestoresOfficialSpriteLayout(t *testing.T) {
	official := officialTypeTreeFromAbaSample(t, "parts_personal_om015_gp003.aba", ClassIDSprite)
	legacy := legacySpriteTreeForMigrationTest(t, &official)

	changed, err := normalizeSpriteForUnity2022(&legacy, &TypeTreeValue{TypeName: "Sprite", Name: "Base"})
	if err != nil {
		t.Fatalf("normalizeSpriteForUnity2022: %v", err)
	}
	if !changed {
		t.Fatal("legacy Sprite schema was not migrated")
	}

	assertTypeTreesLogicallyEqual(t, "migrated Sprite", &legacy, &official)
	if derived, want := legacy.ComputeTypeHash(), official.ComputeTypeHash(); derived != want {
		t.Fatalf("migrated Sprite TypeHash = %x, want official %x", derived, want)
	}
}

// findTypeTreeInAba 在已打开的 aba 中查找指定 ClassID 的 TypeTree 深拷贝
// findTypeTreeInAba finds a deep copy of the TypeTree of a ClassID inside an already-opened aba
func findTypeTreeInAba(t *testing.T, label string, bundle *Aba, classID int32) TypeTreeType {
	t.Helper()
	for directoryIndex, directory := range bundle.BlockInfo.DirectoryInfos {
		if !directory.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(int64(directoryIndex))
		if err != nil {
			t.Fatalf("GetFileData(%s): %v", label, err)
		}
		assetsFile, err := ReadAssetsFile(data)
		if err != nil {
			t.Fatalf("ReadAssetsFile(%s): %v", label, err)
		}
		for typeIndex := range assetsFile.Metadata.TypeTreeTypes {
			if assetsFile.Metadata.TypeTreeTypes[typeIndex].TypeId == classID {
				return cloneTypeTreeType(&assetsFile.Metadata.TypeTreeTypes[typeIndex])
			}
		}
	}
	t.Skipf("%s has no tree for class %d", label, classID)
	return TypeTreeType{}
}

// findAssetByClassInAba 在已打开的 aba 中取出指定 ClassID 的首个资产及其 AssetsFile
// findAssetByClassInAba returns the first asset of a ClassID and its AssetsFile from an already-opened aba
func findAssetByClassInAba(t *testing.T, label string, bundle *Aba, classID int32) (*AssetsFile, *AssetInfo) {
	t.Helper()
	for directoryIndex, directory := range bundle.BlockInfo.DirectoryInfos {
		if !directory.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(int64(directoryIndex))
		if err != nil {
			t.Fatalf("GetFileData(%s): %v", label, err)
		}
		assetsFile, err := ReadAssetsFile(data)
		if err != nil {
			t.Fatalf("ReadAssetsFile(%s): %v", label, err)
		}
		for assetIndex := range assetsFile.Metadata.AssetInfos {
			if assetsFile.Metadata.AssetInfos[assetIndex].TypeId == classID {
				return assetsFile, &assetsFile.Metadata.AssetInfos[assetIndex]
			}
		}
	}
	t.Skipf("%s has no asset of class %d", label, classID)
	return nil, nil
}

// legacyAssetByClassFromAbaSample 从 testdata/aba 的旧版样本取出指定 ClassID 的首个资产及其 AssetsFile
// legacyAssetByClassFromAbaSample returns the first asset of a ClassID and its AssetsFile from a legacy sample under testdata/aba
func legacyAssetByClassFromAbaSample(t *testing.T, sample string, classID int32) (*AssetsFile, *AssetInfo) {
	t.Helper()
	filePath := filepath.Join("..", "..", "..", "testdata", "aba", sample)
	f, err := os.Open(filePath)
	if err != nil {
		t.Skipf("sample %s not available: %v", sample, err)
	}
	defer f.Close()
	bundle, err := ReadAba(f)
	if err != nil {
		t.Fatalf("ReadAba(%s): %v", sample, err)
	}
	return findAssetByClassInAba(t, sample, bundle, classID)
}

// TestUnity2022MigrationPayloadDecodesWithOfficialTree 校验真实旧版对象迁移后的正文能被官方 2022.3 树完整解码
// 这覆盖树等价之外的另一半：迁移不只产出官方形状的树，产出的字节也必须严格符合该树描述的布局
// TestUnity2022MigrationPayloadDecodesWithOfficialTree verifies that a migrated real legacy payload decodes completely under the official 2022.3 tree
// This covers the half beyond tree equivalence: the migration must not only emit an official-shaped tree but also emit bytes that match the layout that tree describes
func TestUnity2022MigrationPayloadDecodesWithOfficialTree(t *testing.T) {
	const legacySample = "parts_personal004.aba"
	const officialSample = "parts_personal_om015_gp003.aba"

	tests := []struct {
		name     string
		classID  int32
		validate func(*testing.T, *TypeTreeValue)
	}{
		{
			name:    "Texture2D",
			classID: ClassIDTexture2D,
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
			validate: func(t *testing.T, root *TypeTreeValue) {
				options, ok := root.Field("m_CookingOptions").Int64()
				if !ok || options != int64(unity2022DefaultMeshCookingOptions) {
					t.Fatalf("m_CookingOptions = %d, %t", options, ok)
				}
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assetsFile, info := legacyAssetByClassFromAbaSample(t, legacySample, testCase.classID)
			root, err := assetsFile.ReadAssetValue(info)
			if err != nil {
				t.Fatal(err)
			}
			encoded, migratedTree, changed, err := assetsFile.EncodeAssetValueAndTypeTreeForUnity2022(info, root)
			if err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatalf("%s legacy schema in %s was not migrated", testCase.name, legacySample)
			}

			// 迁移必须把旧版树完全对齐到官方 2022.3 树，包括 Mesh 根节点 Version 10 到 11 的提升
			// The migration must align the legacy tree completely with the official 2022.3 tree, including raising the Mesh root node Version from 10 to 11
			official := officialTypeTreeFromAbaSample(t, officialSample, testCase.classID)
			assertTypeTreesLogicallyEqual(t, "migrated "+testCase.name, &migratedTree, &official)
			if derived, want := migratedTree.TypeHash, official.TypeHash; derived != want {
				t.Fatalf("migrated %s TypeHash = %x, want official %x", testCase.name, derived, want)
			}
			// decodeObjectWithTypeTree 同时断言节点被完整消费且正文无剩余字节
			// decodeObjectWithTypeTree also asserts that every node is consumed and no payload bytes are left over
			testCase.validate(t, decodeObjectWithTypeTree(t, &official, encoded))
		})
	}
}

// TestUnity2022MigrationRestoresOfficialAnimationClipLayout 校验 AnimationClip 布局迁移精确还原官方 2022.3 树
// 官方基准取自 testdata/KCES/motion_kces2.aba，旧版源取自 testdata/KCES/motion.aba；两者相差 5 个节点与若干 Version
// TestUnity2022MigrationRestoresOfficialAnimationClipLayout verifies that the AnimationClip layout migration exactly restores the official 2022.3 tree
// The official baseline comes from testdata/KCES/motion_kces2.aba and the legacy source from testdata/KCES/motion.aba, which differ by five nodes and several Versions
func TestUnity2022MigrationRestoresOfficialAnimationClipLayout(t *testing.T) {
	legacyBundle, legacyFile := openAbaSample(t, "motion.aba")
	defer legacyFile.Close()
	assetsFile, info := findAssetByClassInAba(t, "motion.aba", legacyBundle, ClassIDAnimationClip)

	root, err := assetsFile.ReadAssetValue(info)
	if err != nil {
		t.Fatal(err)
	}
	encoded, migratedTree, changed, err := assetsFile.EncodeAssetValueAndTypeTreeForUnity2022(info, root)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("legacy AnimationClip schema was not migrated")
	}

	officialBundle, officialFile := openAbaSample(t, "motion_kces2.aba")
	defer officialFile.Close()
	official := findTypeTreeInAba(t, "motion_kces2.aba", officialBundle, ClassIDAnimationClip)

	assertTypeTreesLogicallyEqual(t, "migrated AnimationClip", &migratedTree, &official)
	if derived, want := migratedTree.TypeHash, official.TypeHash; derived != want {
		t.Fatalf("migrated AnimationClip TypeHash = %x, want official %x", derived, want)
	}
	// 迁移正文必须能被官方树完整解码，覆盖 curveCount 收窄与新增字段的字节布局
	// The migrated payload must decode completely under the official tree, covering the narrowed curveCount and the added fields
	decodeObjectWithTypeTree(t, &official, encoded)
}

// TestUnity2022MigrationRestoresOfficialTexture2DLayout 校验 Texture2D 布局迁移精确还原官方 2022.3 树
// 迁移产出与官方树逻辑节点全等且推导哈希等于官方哈希，因此把迁移后的 TypeHash 由置零改为推导值即可让 Unity 命中原生布局快路径
// TestUnity2022MigrationRestoresOfficialTexture2DLayout verifies that the Texture2D layout migration exactly restores the official 2022.3 tree
// The migrated output is logically node-for-node identical to the official tree and derives the official hash, so replacing the zeroed TypeHash with a derived one lets Unity take the native-layout fast path
func TestUnity2022MigrationRestoresOfficialTexture2DLayout(t *testing.T) {
	official := unity2022Texture2DTypeTree()
	legacy, limitIndex := legacyTexture2DTreeForMigrationTest(t, &official)

	root := &TypeTreeValue{
		TypeName: "Texture2D",
		Name:     "Base",
		Children: []*TypeTreeValue{{
			TypeName:  "bool",
			Name:      "m_IgnoreMasterTextureLimit",
			NodeIndex: limitIndex,
			Value:     false,
		}},
	}
	changed, err := normalizeTexture2DForUnity2022(&legacy, root)
	if err != nil {
		t.Fatalf("normalizeTexture2DForUnity2022: %v", err)
	}
	if !changed {
		t.Fatal("legacy Texture2D schema was not migrated")
	}

	assertTypeTreesLogicallyEqual(t, "migrated Texture2D", &legacy, &official)
	if derived, want := legacy.ComputeTypeHash(), official.ComputeTypeHash(); derived != want {
		t.Fatalf("migrated Texture2D TypeHash = %x, want official %x", derived, want)
	}
}

// legacyMeshTreeForMigrationTest 把官方 Mesh 树退回迁移前的旧版形状，并返回用法标志与已烘焙碰撞网格字段的节点索引
// legacyMeshTreeForMigrationTest reverts the official Mesh tree into its pre-migration legacy shape and returns the node indexes of the usage-flags and baked-collision-mesh fields
func legacyMeshTreeForMigrationTest(t *testing.T, official *TypeTreeType) (TypeTreeType, int64, int64) {
	t.Helper()
	legacy := cloneTypeTreeType(official)

	cookingIndex := findTypeTreeNodeByFieldName(&legacy, "m_CookingOptions")
	if cookingIndex < 0 {
		t.Fatal("official Mesh tree has no m_CookingOptions")
	}
	legacy.Nodes = append(legacy.Nodes[:cookingIndex], legacy.Nodes[cookingIndex+1:]...)

	usageIndex := findTypeTreeNodeByFieldName(&legacy, "m_MeshUsageFlags")
	bakedIndex := findTypeTreeNodeByFieldName(&legacy, "m_BakedConvexCollisionMesh")
	if usageIndex < 0 || bakedIndex < 0 {
		t.Fatalf("official Mesh tree lacks m_MeshUsageFlags (%d) or m_BakedConvexCollisionMesh (%d)", usageIndex, bakedIndex)
	}
	return legacy, usageIndex, bakedIndex
}

// TestUnity2022MigrationRestoresOfficialMeshLayout 校验 Mesh 布局迁移精确还原官方 2022.3 树
// TestUnity2022MigrationRestoresOfficialMeshLayout verifies that the Mesh layout migration exactly restores the official 2022.3 tree
func TestUnity2022MigrationRestoresOfficialMeshLayout(t *testing.T) {
	official, err := unity2022MeshTypeTree()
	if err != nil {
		t.Fatal(err)
	}
	legacy, usageIndex, bakedIndex := legacyMeshTreeForMigrationTest(t, &official)

	root := &TypeTreeValue{
		TypeName: "Mesh",
		Name:     "Base",
		Children: []*TypeTreeValue{
			{TypeName: "unsigned int", Name: "m_MeshUsageFlags", NodeIndex: usageIndex, Value: int64(0)},
			{TypeName: "TypelessData", Name: "m_BakedConvexCollisionMesh", NodeIndex: bakedIndex},
		},
	}
	changed, err := normalizeMeshForUnity2022(&legacy, root)
	if err != nil {
		t.Fatalf("normalizeMeshForUnity2022: %v", err)
	}
	if !changed {
		t.Fatal("legacy Mesh schema was not migrated")
	}

	assertTypeTreesLogicallyEqual(t, "migrated Mesh", &legacy, &official)
	if derived, want := legacy.ComputeTypeHash(), official.ComputeTypeHash(); derived != want {
		t.Fatalf("migrated Mesh TypeHash = %x, want official %x", derived, want)
	}
}
