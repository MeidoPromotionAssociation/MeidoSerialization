package aba

import (
	"fmt"
	"math"
)

const unity2022DefaultMeshCookingOptions int32 = 30

// unity2022MeshRootNodeVersion 是官方 Unity 2022.3 Mesh TypeTree 根节点的 Version
// 旧版 KCES 样本的该字段为 10，迁移必须一并提升到 11，否则迁移树与官方树不一致，推导出的 TypeHash 也对不上官方哈希
// unity2022MeshRootNodeVersion is the Version carried by the root node of the official Unity 2022.3 Mesh TypeTree
// Legacy KCES samples carry 10 in that field, so the migration must raise it to 11; otherwise the migrated tree diverges from the official tree and its derived TypeHash cannot match the official hash
const unity2022MeshRootNodeVersion uint16 = 11

// unity2022AnimationClipRootNodeVersion 是官方 Unity 2022.3 AnimationClip TypeTree 根节点的 Version，旧版 KCES 样本为 6
// unity2022AnimationClipRootNodeVersion is the Version carried by the root node of the official Unity 2022.3 AnimationClip TypeTree, while legacy KCES samples carry 6
const unity2022AnimationClipRootNodeVersion uint16 = 7

// unity2022AnimationClipNestedNodeVersion 是官方 Unity 2022.3 AnimationClip 中 FloatCurve、PPtrCurve 与 StreamedClip 节点提升后的 Version，旧版 KCES 样本为 1
// unity2022AnimationClipNestedNodeVersion is the raised Version carried by the FloatCurve, PPtrCurve, and StreamedClip nodes of the official Unity 2022.3 AnimationClip, while legacy KCES samples carry 1
const unity2022AnimationClipNestedNodeVersion uint16 = 2

// EncodeAssetValueForUnity2022 将已解码对象迁移到 Unity 2022.3 内置类型布局并按对象字节序重新编码
// EncodeAssetValueForUnity2022 migrates a decoded object to the Unity 2022.3 built-in type layout and re-encodes it using the object's byte order
func (af *AssetsFile) EncodeAssetValueForUnity2022(info *AssetInfo, root *TypeTreeValue) ([]byte, bool, error) {
	data, _, changed, err := af.EncodeAssetValueAndTypeTreeForUnity2022(info, root)
	return data, changed, err
}

// EncodeAssetValueAndTypeTreeForUnity2022 将对象迁移到 Unity 2022.3，并同时返回与编码正文严格匹配的 TypeTree
// EncodeAssetValueAndTypeTreeForUnity2022 migrates an object to Unity 2022.3 and also returns the TypeTree that exactly matches the encoded payload
func (af *AssetsFile) EncodeAssetValueAndTypeTreeForUnity2022(info *AssetInfo, root *TypeTreeValue) ([]byte, TypeTreeType, bool, error) {
	if af == nil {
		return nil, TypeTreeType{}, false, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, TypeTreeType{}, false, fmt.Errorf("nil asset info")
	}
	if root == nil {
		return nil, TypeTreeType{}, false, fmt.Errorf("nil asset value")
	}
	sourceTree, err := af.typeTreeForAsset(info)
	if err != nil {
		return nil, TypeTreeType{}, false, err
	}
	targetTree := cloneTypeTreeType(sourceTree)
	changed, err := normalizeTypeTreeValueForUnity2022(info.TypeId, &targetTree, root)
	if err != nil {
		return nil, TypeTreeType{}, false, err
	}
	if !changed {
		data, err := af.EncodeAssetValue(info, root)
		return data, targetTree, false, err
	}
	// 迁移树对齐官方 2022.3 布局后，TypeHash 必须由节点表重新推导
	// 置零会让 Unity 放弃原生布局快路径，而推导值在迁移产出与官方树一致时恰好等于官方哈希
	// After the migrated tree matches the official 2022.3 layout the TypeHash must be re-derived from the node table
	// A zeroed hash makes Unity skip the native-layout fast path, while the derived value equals the official hash exactly when the migration reproduces the official tree
	targetTree.TypeHash = targetTree.ComputeTypeHash()

	targetFile := *af
	targetFile.Metadata = af.Metadata
	targetFile.Metadata.TypeTreeEnabled = true
	targetFile.Metadata.TypeTreeTypes = []TypeTreeType{targetTree}
	targetInfo := *info
	targetInfo.TypeIdOrIndex = 0
	data, err := targetFile.EncodeAssetValue(&targetInfo, root)
	if err != nil {
		return nil, TypeTreeType{}, false, fmt.Errorf("encode class %d with normalized Unity 2022.3 TypeTree: %w", info.TypeId, err)
	}
	return data, targetTree, true, nil
}

// cloneTypeTreeType 深拷贝会被布局迁移修改的 TypeTree 切片字段
// cloneTypeTreeType deep-copies the TypeTree slices modified during layout migration
func cloneTypeTreeType(source *TypeTreeType) TypeTreeType {
	if source == nil {
		return TypeTreeType{}
	}
	result := *source
	result.Nodes = append([]TypeTreeNode(nil), source.Nodes...)
	result.StringBuffer = append([]byte(nil), source.StringBuffer...)
	result.TypeDependencies = append([]int32(nil), source.TypeDependencies...)
	return result
}

// normalizeTypeTreeValueForUnity2022 按实际字段形状应用 KCES 旧版本到 Unity 2022.3 的内置对象迁移
// normalizeTypeTreeValueForUnity2022 applies KCES legacy-to-Unity-2022.3 built-in object migrations according to the actual field shape
func normalizeTypeTreeValueForUnity2022(classID int32, tree *TypeTreeType, root *TypeTreeValue) (bool, error) {
	switch classID {
	case ClassIDTexture2D:
		return normalizeTexture2DForUnity2022(tree, root)
	case ClassIDMesh:
		return normalizeMeshForUnity2022(tree, root)
	case ClassIDSprite:
		return normalizeSpriteForUnity2022(tree, root)
	case ClassIDAnimationClip:
		return normalizeAnimationClipForUnity2022(tree, root)
	default:
		return false, nil
	}
}

// normalizeTexture2DForUnity2022 将旧纹理限制字段替换为 Unity 2022.3 的字段和空组名
// normalizeTexture2DForUnity2022 replaces the legacy texture-limit field with the Unity 2022.3 field and an empty group name
func normalizeTexture2DForUnity2022(tree *TypeTreeType, root *TypeTreeValue) (bool, error) {
	legacyIndex := childValueIndex(root, "m_IgnoreMasterTextureLimit")
	limitIndex := childValueIndex(root, "m_IgnoreMipmapLimit")
	groupIndex := childValueIndex(root, "m_MipmapLimitGroupName")
	if legacyIndex < 0 && limitIndex >= 0 && groupIndex >= 0 {
		return false, nil
	}
	if legacyIndex < 0 || limitIndex >= 0 || groupIndex >= 0 {
		return false, fmt.Errorf("unsupported Texture2D mipmap-limit schema")
	}

	legacyValue := root.Children[legacyIndex]
	legacyNodeIndex := legacyValue.NodeIndex
	if !validTypeTreeNodeIndex(tree, legacyNodeIndex) {
		return false, fmt.Errorf("Texture2D legacy mipmap-limit node index %d is invalid", legacyNodeIndex)
	}
	legacyNode := &tree.Nodes[legacyNodeIndex]
	if tree.GetTypeTreeString(legacyNode, true) != "bool" {
		return false, fmt.Errorf("Texture2D legacy mipmap-limit field has type %q", tree.GetTypeTreeString(legacyNode, true))
	}
	if err := setTypeTreeNodeName(tree, legacyNode, "m_IgnoreMipmapLimit"); err != nil {
		return false, err
	}
	legacyNode.MetaFlags |= 0x4000
	legacyValue.Name = "m_IgnoreMipmapLimit"

	pathNodeIndex, ok := findTypeTreeNode(tree, "string", "path")
	if !ok {
		return false, fmt.Errorf("Texture2D TypeTree has no ordinary string template")
	}
	stringNodes, err := cloneTypeTreeSubtreeAtLevel(tree, pathNodeIndex, legacyNode.Level)
	if err != nil {
		return false, err
	}
	if err := setTypeTreeNodeName(tree, &stringNodes[0], "m_MipmapLimitGroupName"); err != nil {
		return false, err
	}
	insertTypeTreeNodes(tree, legacyNodeIndex+1, stringNodes)
	insertChildValue(root, legacyIndex+1, &TypeTreeValue{
		TypeName: "string",
		Name:     "m_MipmapLimitGroupName",
		Value:    "",
	})
	return true, nil
}

// normalizeMeshForUnity2022 为旧 Mesh 布局补入 Unity 2022.3 默认碰撞网格烹饪选项
// normalizeMeshForUnity2022 inserts the Unity 2022.3 default collision-mesh cooking options into a legacy Mesh layout
func normalizeMeshForUnity2022(tree *TypeTreeType, root *TypeTreeValue) (bool, error) {
	if childValueIndex(root, "m_CookingOptions") >= 0 {
		return false, nil
	}
	usageIndex := childValueIndex(root, "m_MeshUsageFlags")
	if usageIndex < 0 || childValueIndex(root, "m_BakedConvexCollisionMesh") < 0 {
		return false, fmt.Errorf("unsupported Mesh schema without m_CookingOptions")
	}
	usageValue := root.Children[usageIndex]
	usageNodeIndex := usageValue.NodeIndex
	if !validTypeTreeNodeIndex(tree, usageNodeIndex) {
		return false, fmt.Errorf("Mesh usage node index %d is invalid", usageNodeIndex)
	}
	cookingNode := tree.Nodes[usageNodeIndex]
	cookingNode.MetaFlags = 0
	if err := setTypeTreeNodeName(tree, &cookingNode, "m_CookingOptions"); err != nil {
		return false, err
	}
	insertTypeTreeNodes(tree, skipSubtree(tree, usageNodeIndex), []TypeTreeNode{cookingNode})
	// 根节点是节点表的首项，官方 2022.3 把它提升到 11，迁移必须同步，否则迁移树无法与官方树全等
	// The root node is the first entry of the node table, and official 2022.3 raises it to 11; the migration must follow or the migrated tree cannot match the official tree
	tree.Nodes[0].Version = unity2022MeshRootNodeVersion
	insertChildValue(root, usageIndex+1, &TypeTreeValue{
		TypeName: "int",
		Name:     "m_CookingOptions",
		Value:    int64(unity2022DefaultMeshCookingOptions),
	})
	return true, nil
}

// normalizeSpriteForUnity2022 为旧版 SpriteBone 元素补入 Unity 2022.3 的 GUID 和白色色调
// normalizeSpriteForUnity2022 adds the Unity 2022.3 GUID and white tint to legacy SpriteBone elements
func normalizeSpriteForUnity2022(tree *TypeTreeType, root *TypeTreeValue) (bool, error) {
	boneNodeIndex, ok := findTypeTreeNode(tree, "SpriteBone", "data")
	if !ok {
		return false, fmt.Errorf("unsupported Sprite schema without SpriteBone")
	}
	nameNodeIndex, nameEnd, ok := findDirectTypeTreeChild(tree, boneNodeIndex, "string", "name")
	if !ok {
		return false, fmt.Errorf("SpriteBone has no name field")
	}
	_, parentEnd, hasParent := findDirectTypeTreeChild(tree, boneNodeIndex, "int", "parentId")
	if !hasParent {
		return false, fmt.Errorf("SpriteBone has no parentId field")
	}
	_, _, hasGUID := findDirectTypeTreeChild(tree, boneNodeIndex, "string", "guid")
	_, _, hasColor := findDirectTypeTreeChild(tree, boneNodeIndex, "ColorRGBA", "color")
	if hasGUID && hasColor {
		return false, nil
	}
	if hasGUID || hasColor {
		return false, fmt.Errorf("unsupported partial Unity 2022.3 SpriteBone schema")
	}

	guidNodes, err := cloneTypeTreeSubtreeAtLevel(tree, nameNodeIndex, tree.Nodes[nameNodeIndex].Level)
	if err != nil {
		return false, err
	}
	if err := setTypeTreeNodeName(tree, &guidNodes[0], "guid"); err != nil {
		return false, err
	}
	colorNodes, err := makeColorRGBASubtree(tree, tree.Nodes[nameNodeIndex].Level)
	if err != nil {
		return false, err
	}
	insertTypeTreeNodes(tree, parentEnd, colorNodes)
	insertTypeTreeNodes(tree, nameEnd, guidNodes)
	patchSpriteBoneValues(root)
	return true, nil
}

// normalizeAnimationClipForUnity2022 把旧版 AnimationClip 迁移到 Unity 2022.3 布局
// 迁移内容：FloatCurve 与 PPtrCurve 补 flags；StreamedClip 把 curveCount 由 unsigned int 收窄为 UInt16 并补 discreteCurveCount；
// GenericBinding 补 isIntCurve 与 isSerializeReferenceCurve；相关节点 Version 提升到官方 2022.3 取值
// normalizeAnimationClipForUnity2022 migrates a legacy AnimationClip to the Unity 2022.3 layout
// It adds flags to FloatCurve and PPtrCurve, narrows StreamedClip curveCount from unsigned int to UInt16 while appending discreteCurveCount,
// adds isIntCurve and isSerializeReferenceCurve to GenericBinding, and raises the affected node Versions to their official 2022.3 values
func normalizeAnimationClipForUnity2022(tree *TypeTreeType, root *TypeTreeValue) (bool, error) {
	if tree == nil || len(tree.Nodes) == 0 {
		return false, fmt.Errorf("AnimationClip TypeTree has no nodes")
	}
	changed := false

	for _, typeName := range []string{"FloatCurve", "PPtrCurve"} {
		parentIndex, ok := findTypeTreeNodeByType(tree, typeName)
		if !ok {
			return false, fmt.Errorf("AnimationClip TypeTree has no %s", typeName)
		}
		if tree.Nodes[parentIndex].Version < unity2022AnimationClipNestedNodeVersion {
			tree.Nodes[parentIndex].Version = unity2022AnimationClipNestedNodeVersion
			changed = true
		}
		_, _, hasFlags := findDirectTypeTreeChild(tree, parentIndex, "int", "flags")
		if hasFlags {
			continue
		}
		_, scriptEnd, hasScript := findDirectTypeTreeChild(tree, parentIndex, "", "script")
		if !hasScript {
			return false, fmt.Errorf("AnimationClip %s has no script field", typeName)
		}
		flagsNode, err := makePrimitiveTypeTreeNode(tree, tree.Nodes[parentIndex].Level+1, "int", "flags", 4, 1)
		if err != nil {
			return false, err
		}
		insertTypeTreeNodes(tree, scriptEnd, []TypeTreeNode{flagsNode})
		patchValuesByType(root, typeName, func(value *TypeTreeValue) {
			insertChildValue(value, int64(len(value.Children)), &TypeTreeValue{TypeName: "int", Name: "flags", Value: int64(0)})
		})
		changed = true
	}

	streamedChanged, err := normalizeStreamedClipForUnity2022(tree, root)
	if err != nil {
		return false, err
	}
	if streamedChanged {
		changed = true
	}

	bindingIndex, ok := findTypeTreeNodeByType(tree, "GenericBinding")
	if !ok {
		return false, fmt.Errorf("AnimationClip TypeTree has no GenericBinding")
	}
	if _, _, hasIntCurve := findDirectTypeTreeChild(tree, bindingIndex, "UInt8", "isIntCurve"); !hasIntCurve {
		pptrIndex, pptrEnd, hasPPtr := findDirectTypeTreeChild(tree, bindingIndex, "UInt8", "isPPtrCurve")
		if !hasPPtr {
			return false, fmt.Errorf("AnimationClip GenericBinding has no isPPtrCurve field")
		}
		// 官方 2022.3 把 AlignBytes 从 isPPtrCurve 移到新增的末位字节字段 isSerializeReferenceCurve，isIntCurve 自身不带该标志
		// Official 2022.3 moves AlignBytes from isPPtrCurve onto the added trailing byte field isSerializeReferenceCurve, leaving isIntCurve without that flag
		intCurveNode := tree.Nodes[pptrIndex]
		intCurveNode.MetaFlags &^= typeHashAlignFlag
		if err := setTypeTreeNodeName(tree, &intCurveNode, "isIntCurve"); err != nil {
			return false, err
		}
		serializeReferenceNode := tree.Nodes[pptrIndex]
		serializeReferenceNode.MetaFlags |= typeHashAlignFlag
		if err := setTypeTreeNodeName(tree, &serializeReferenceNode, "isSerializeReferenceCurve"); err != nil {
			return false, err
		}
		tree.Nodes[pptrIndex].MetaFlags &^= typeHashAlignFlag
		insertTypeTreeNodes(tree, pptrEnd, []TypeTreeNode{intCurveNode, serializeReferenceNode})
		if tree.Nodes[bindingIndex].ByteSize >= 0 {
			if tree.Nodes[bindingIndex].ByteSize > math.MaxInt32-2 {
				return false, fmt.Errorf("AnimationClip GenericBinding byte size exceeds Int32 range")
			}
			tree.Nodes[bindingIndex].ByteSize += 2
		}
		patchValuesByType(root, "GenericBinding", func(value *TypeTreeValue) {
			pptrValueIndex := childValueIndex(value, "isPPtrCurve")
			insertChildValue(value, pptrValueIndex+1, &TypeTreeValue{TypeName: "UInt8", Name: "isIntCurve", Value: int64(0)})
			insertChildValue(value, pptrValueIndex+2, &TypeTreeValue{TypeName: "UInt8", Name: "isSerializeReferenceCurve", Value: int64(0)})
		})
		changed = true
	}

	if tree.Nodes[0].Version < unity2022AnimationClipRootNodeVersion {
		tree.Nodes[0].Version = unity2022AnimationClipRootNodeVersion
		changed = true
	}
	return changed, nil
}

// normalizeStreamedClipForUnity2022 把 StreamedClip 迁移到 Unity 2022.3 布局
// 2022.3 把 curveCount 由 unsigned int 收窄为 UInt16，并在其后新增 UInt16 discreteCurveCount
// normalizeStreamedClipForUnity2022 migrates StreamedClip to the Unity 2022.3 layout
// Version 2022.3 narrows curveCount from unsigned int to UInt16 and appends a UInt16 discreteCurveCount after it
func normalizeStreamedClipForUnity2022(tree *TypeTreeType, root *TypeTreeValue) (bool, error) {
	streamedIndex, ok := findTypeTreeNodeByType(tree, "StreamedClip")
	if !ok {
		return false, fmt.Errorf("AnimationClip TypeTree has no StreamedClip")
	}
	changed := false

	curveCountIndex, curveCountEnd, isLegacyCurveCount := findDirectTypeTreeChild(tree, streamedIndex, "unsigned int", "curveCount")
	if !isLegacyCurveCount {
		var found bool
		curveCountIndex, curveCountEnd, found = findDirectTypeTreeChild(tree, streamedIndex, "UInt16", "curveCount")
		if !found {
			return false, fmt.Errorf("AnimationClip StreamedClip has no curveCount field")
		}
	} else {
		// 收窄类型与字节宽度时必须同步值的类型名，否则写入阶段会因类型不匹配而拒绝
		// Narrowing the type and byte width requires updating the value type name too, or the writer rejects the mismatched value
		if err := setTypeTreeNodeType(tree, &tree.Nodes[curveCountIndex], "UInt16"); err != nil {
			return false, err
		}
		tree.Nodes[curveCountIndex].ByteSize = 2
		if err := narrowStreamedClipCurveCountValue(root); err != nil {
			return false, err
		}
		changed = true
	}

	if _, _, hasDiscrete := findDirectTypeTreeChild(tree, streamedIndex, "UInt16", "discreteCurveCount"); !hasDiscrete {
		discreteNode, err := makePrimitiveTypeTreeNode(tree, tree.Nodes[curveCountIndex].Level, "UInt16", "discreteCurveCount", 2, 0)
		if err != nil {
			return false, err
		}
		insertTypeTreeNodes(tree, curveCountEnd, []TypeTreeNode{discreteNode})
		patchValuesByType(root, "StreamedClip", func(value *TypeTreeValue) {
			curveValueIndex := childValueIndex(value, "curveCount")
			insertChildValue(value, curveValueIndex+1, &TypeTreeValue{TypeName: "UInt16", Name: "discreteCurveCount", Value: int64(0)})
		})
		changed = true
	}

	if tree.Nodes[streamedIndex].Version < unity2022AnimationClipNestedNodeVersion {
		tree.Nodes[streamedIndex].Version = unity2022AnimationClipNestedNodeVersion
		changed = true
	}
	return changed, nil
}

// narrowStreamedClipCurveCountValue 把每个 StreamedClip 值中的 curveCount 收窄为 UInt16 并校验范围
// 官方 2022.3 的 curveCount 是 UInt16，旧版为 unsigned int；超出 UInt16 范围时拒绝迁移而不是静默截断
// narrowStreamedClipCurveCountValue narrows the curveCount of every StreamedClip value to UInt16 and validates its range
// Official 2022.3 stores curveCount as UInt16 while legacy data stores it as unsigned int, and an out-of-range value refuses migration instead of being silently truncated
func narrowStreamedClipCurveCountValue(root *TypeTreeValue) error {
	var firstErr error
	patchValuesByType(root, "StreamedClip", func(value *TypeTreeValue) {
		if firstErr != nil {
			return
		}
		curveValueIndex := childValueIndex(value, "curveCount")
		if curveValueIndex < 0 {
			firstErr = fmt.Errorf("StreamedClip value has no curveCount field")
			return
		}
		curveValue := value.Children[curveValueIndex]
		count, ok := curveValue.Int64()
		if !ok || count < 0 || count > math.MaxUint16 {
			firstErr = fmt.Errorf("StreamedClip curveCount %v is outside UInt16 range", curveValue.Value)
			return
		}
		curveValue.TypeName = "UInt16"
		curveValue.Value = count
	})
	return firstErr
}

// childValueIndex 返回直接子字段索引，未找到时返回负一
// childValueIndex returns the direct child-field index or negative one when absent
func childValueIndex(parent *TypeTreeValue, name string) int64 {
	if parent == nil {
		return -1
	}
	for childIndex, child := range parent.Children {
		if child != nil && child.Name == name {
			return int64(childIndex)
		}
	}
	return -1
}

// insertChildValue 在确定位置插入一个 TypeTree 值
// insertChildValue inserts a TypeTree value at a known position
func insertChildValue(parent *TypeTreeValue, index int64, value *TypeTreeValue) {
	if parent == nil || index < 0 || index > int64(len(parent.Children)) {
		return
	}
	parent.Children = append(parent.Children, nil)
	copy(parent.Children[index+1:], parent.Children[index:])
	parent.Children[index] = value
}

// validTypeTreeNodeIndex 判断节点索引是否位于 TypeTree 范围内
// validTypeTreeNodeIndex reports whether a node index is inside a TypeTree
func validTypeTreeNodeIndex(tree *TypeTreeType, index int64) bool {
	return tree != nil && index >= 0 && index < int64(len(tree.Nodes))
}

// findTypeTreeNode 查找首个类型名和字段名都匹配的节点
// findTypeTreeNode finds the first node matching both type and field names
func findTypeTreeNode(tree *TypeTreeType, typeName string, name string) (int64, bool) {
	if tree == nil {
		return 0, false
	}
	for nodeIndex := range tree.Nodes {
		node := &tree.Nodes[nodeIndex]
		if tree.GetTypeTreeString(node, true) == typeName && tree.GetTypeTreeString(node, false) == name {
			return int64(nodeIndex), true
		}
	}
	return 0, false
}

// findTypeTreeNodeByType 查找首个类型名匹配的节点
// findTypeTreeNodeByType finds the first node matching a type name
func findTypeTreeNodeByType(tree *TypeTreeType, typeName string) (int64, bool) {
	if tree == nil {
		return 0, false
	}
	for nodeIndex := range tree.Nodes {
		if tree.GetTypeTreeString(&tree.Nodes[nodeIndex], true) == typeName {
			return int64(nodeIndex), true
		}
	}
	return 0, false
}

// findDirectTypeTreeChild 在父节点子树中查找直接子字段并返回其子树范围
// findDirectTypeTreeChild finds a direct field inside a parent subtree and returns its subtree range
func findDirectTypeTreeChild(tree *TypeTreeType, parentIndex int64, typeName string, name string) (int64, int64, bool) {
	if !validTypeTreeNodeIndex(tree, parentIndex) {
		return 0, 0, false
	}
	parentLevel := tree.Nodes[parentIndex].Level
	parentEnd := skipSubtree(tree, parentIndex)
	for nodeIndex := parentIndex + 1; nodeIndex < parentEnd; {
		node := &tree.Nodes[nodeIndex]
		if node.Level == parentLevel+1 && (typeName == "" || tree.GetTypeTreeString(node, true) == typeName) && tree.GetTypeTreeString(node, false) == name {
			return nodeIndex, skipSubtree(tree, nodeIndex), true
		}
		next := skipSubtree(tree, nodeIndex)
		if next <= nodeIndex {
			return 0, 0, false
		}
		nodeIndex = next
	}
	return 0, 0, false
}

// cloneTypeTreeSubtreeAtLevel 复制一个节点子树并整体调整到目标层级
// cloneTypeTreeSubtreeAtLevel copies a node subtree and shifts it to a target level
func cloneTypeTreeSubtreeAtLevel(tree *TypeTreeType, start int64, targetLevel byte) ([]TypeTreeNode, error) {
	if !validTypeTreeNodeIndex(tree, start) {
		return nil, fmt.Errorf("TypeTree subtree start %d is invalid", start)
	}
	end := skipSubtree(tree, start)
	result := append([]TypeTreeNode(nil), tree.Nodes[start:end]...)
	delta := int64(targetLevel) - int64(result[0].Level)
	for nodeIndex := range result {
		level := int64(result[nodeIndex].Level) + delta
		if level < 0 || level > math.MaxUint8 {
			return nil, fmt.Errorf("TypeTree level %d is outside UInt8 range", level)
		}
		result[nodeIndex].Level = byte(level)
	}
	return result, nil
}

// insertTypeTreeNodes 在节点数组的确定位置插入一组节点
// insertTypeTreeNodes inserts nodes at a known position in the node array
func insertTypeTreeNodes(tree *TypeTreeType, index int64, nodes []TypeTreeNode) {
	if tree == nil || len(nodes) == 0 || index < 0 || index > int64(len(tree.Nodes)) {
		return
	}
	updated := make([]TypeTreeNode, 0, len(tree.Nodes)+len(nodes))
	updated = append(updated, tree.Nodes[:index]...)
	updated = append(updated, nodes...)
	updated = append(updated, tree.Nodes[index:]...)
	tree.Nodes = updated
}

// setTypeTreeNodeName 把字段名追加到本地字符串缓冲区并更新节点偏移
// setTypeTreeNodeName appends a field name to the local string buffer and updates the node offset
func setTypeTreeNodeName(tree *TypeTreeType, node *TypeTreeNode, name string) error {
	offset, err := appendTypeTreeString(tree, name)
	if err != nil {
		return err
	}
	node.NameStrOff = offset
	return nil
}

// setTypeTreeNodeType 把类型名追加到本地字符串缓冲区并更新节点偏移
// setTypeTreeNodeType appends a type name to the local string buffer and updates the node offset
func setTypeTreeNodeType(tree *TypeTreeType, node *TypeTreeNode, typeName string) error {
	offset, err := appendTypeTreeString(tree, typeName)
	if err != nil {
		return err
	}
	node.TypeStrOff = offset
	return nil
}

// appendTypeTreeString 追加 NUL 结尾字符串并返回 UInt32 偏移
// appendTypeTreeString appends a NUL-terminated string and returns its UInt32 offset
func appendTypeTreeString(tree *TypeTreeType, value string) (uint32, error) {
	if tree == nil {
		return 0, fmt.Errorf("nil TypeTree")
	}
	if uint64(len(tree.StringBuffer)) > math.MaxUint32 {
		return 0, fmt.Errorf("TypeTree string buffer length %d exceeds UInt32 range", len(tree.StringBuffer))
	}
	offset := uint32(len(tree.StringBuffer))
	tree.StringBuffer = append(tree.StringBuffer, value...)
	tree.StringBuffer = append(tree.StringBuffer, 0)
	return offset, nil
}

// makePrimitiveTypeTreeNode 创建一个使用本地类型名和字段名的标量节点
// makePrimitiveTypeTreeNode creates a scalar node using local type and field names
func makePrimitiveTypeTreeNode(tree *TypeTreeType, level byte, typeName string, name string, byteSize int32, metaFlags uint32) (TypeTreeNode, error) {
	node := TypeTreeNode{Version: 1, Level: level, ByteSize: byteSize, MetaFlags: metaFlags}
	if err := setTypeTreeNodeType(tree, &node, typeName); err != nil {
		return TypeTreeNode{}, err
	}
	if err := setTypeTreeNodeName(tree, &node, name); err != nil {
		return TypeTreeNode{}, err
	}
	return node, nil
}

// makeColorRGBASubtree 创建 Unity 2022.3 新增的 ColorRGBA 字段子树
// 官方 2022.3 样本中 ColorRGBA 节点的 Version 为 2 而子节点 rgba 为 1，与 makePrimitiveTypeTreeNode 的默认值不同，因此这里显式覆盖
// makeColorRGBASubtree creates a ColorRGBA field subtree added by Unity 2022.3
// Official 2022.3 samples carry Version 2 on the ColorRGBA node while its rgba child stays at 1, which differs from the makePrimitiveTypeTreeNode default and is therefore overridden explicitly
func makeColorRGBASubtree(tree *TypeTreeType, level byte) ([]TypeTreeNode, error) {
	color, err := makePrimitiveTypeTreeNode(tree, level, "ColorRGBA", "color", 4, 0)
	if err != nil {
		return nil, err
	}
	color.Version = 2
	rgba, err := makePrimitiveTypeTreeNode(tree, level+1, "unsigned int", "rgba", 4, 1)
	if err != nil {
		return nil, err
	}
	return []TypeTreeNode{color, rgba}, nil
}

// patchSpriteBoneValues 为全部嵌套 SpriteBone 值追加默认字段
// patchSpriteBoneValues appends default fields to every nested SpriteBone value
func patchSpriteBoneValues(value *TypeTreeValue) {
	if value == nil {
		return
	}
	if value.TypeName == "SpriteBone" && childValueIndex(value, "guid") < 0 && childValueIndex(value, "color") < 0 {
		nameIndex := childValueIndex(value, "name")
		parentIndex := childValueIndex(value, "parentId")
		insertChildValue(value, parentIndex+1, &TypeTreeValue{
			TypeName: "ColorRGBA",
			Name:     "color",
			Children: []*TypeTreeValue{{TypeName: "unsigned int", Name: "rgba", Value: uint32(math.MaxUint32)}},
		})
		insertChildValue(value, nameIndex+1, &TypeTreeValue{TypeName: "string", Name: "guid", Value: ""})
	}
	for _, child := range value.Children {
		patchSpriteBoneValues(child)
	}
}

// patchValuesByType 对值树中全部指定类型节点应用修改
// patchValuesByType applies a mutation to every node of a specified type in a value tree
func patchValuesByType(value *TypeTreeValue, typeName string, patch func(*TypeTreeValue)) {
	if value == nil {
		return
	}
	if value.TypeName == typeName {
		patch(value)
	}
	for _, child := range value.Children {
		patchValuesByType(child, typeName, patch)
	}
}
