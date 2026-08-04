package aba

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
)

// unity2022MeshTypeTree 解析内嵌 schema 并返回带官方 TypeHash 的 Unity 2022.3 Mesh TypeTree
// unity2022MeshTypeTree parses the embedded schema and returns the Unity 2022.3 Mesh TypeTree with the official TypeHash
func unity2022MeshTypeTree() (TypeTreeType, error) {
	blob, err := base64.StdEncoding.DecodeString(unity2022MeshSchemaBase64)
	if err != nil {
		return TypeTreeType{}, fmt.Errorf("decode embedded Mesh TypeTree schema: %w", err)
	}
	object, err := ReadNativeUnityObject(blob)
	if err != nil {
		return TypeTreeType{}, fmt.Errorf("parse embedded Mesh TypeTree schema: %w", err)
	}
	if object.ClassID != ClassIDMesh {
		return TypeTreeType{}, fmt.Errorf("embedded Mesh TypeTree schema has ClassID %d", object.ClassID)
	}
	return object.TypeTree, nil
}

// NewNativeMeshObject 从通用几何数据构建携带官方 Unity 2022.3 TypeTree 的独立 Mesh 对象
// 顶点布局与官方网格一致，位置、法线、切线和颜色在流零，纹理坐标在下一个流，蒙皮通道在最后一个流
// NewNativeMeshObject builds a standalone Mesh object carrying the official Unity 2022.3 TypeTree from generic geometry
// The vertex layout matches official meshes, with position, normal, tangent, and color in stream zero, texture coordinates in the next stream, and skin channels in the last stream
func NewNativeMeshObject(name string, geometry *MeshGeometry) (*NativeUnityObject, error) {
	if err := validateMeshGeometryForBuild(name, geometry); err != nil {
		return nil, err
	}
	tree, err := unity2022MeshTypeTree()
	if err != nil {
		return nil, err
	}
	root, next, err := newTypeTreeValueSkeleton(&tree, 0)
	if err != nil {
		return nil, fmt.Errorf("build Mesh value skeleton: %w", err)
	}
	if next != int64(len(tree.Nodes)) {
		return nil, fmt.Errorf("Mesh value skeleton consumed %d of %d TypeTree nodes", next, len(tree.Nodes))
	}

	if err := setMeshValue(root, name, "m_Name"); err != nil {
		return nil, err
	}
	indexFormat, indexBuffer, subMeshRanges, err := buildMeshIndexBuffer(geometry)
	if err != nil {
		return nil, err
	}
	if err := fillMeshSubMeshes(&tree, root, geometry, subMeshRanges); err != nil {
		return nil, err
	}
	if err := fillMeshBindPoses(&tree, root, geometry); err != nil {
		return nil, err
	}
	if err := fillMeshBonesAABB(&tree, root, geometry); err != nil {
		return nil, err
	}
	var skin *meshFixedSkin
	if geometry.SkinCounts != nil {
		skin, err = buildMeshFixedSkin(geometry)
		if err != nil {
			return nil, err
		}
		if skin.Variable {
			if err := fillMeshVariableBoneWeights(root, geometry); err != nil {
				return nil, err
			}
		}
	}
	if err := setMeshValue(root, true, "m_IsReadable"); err != nil {
		return nil, err
	}
	if err := setMeshValue(root, indexFormat, "m_IndexFormat"); err != nil {
		return nil, err
	}
	if err := setMeshBytesValue(root, indexBuffer, "m_IndexBuffer"); err != nil {
		return nil, err
	}
	if err := fillMeshVertexData(&tree, root, geometry, skin); err != nil {
		return nil, err
	}
	if err := setMeshAABB(root.Field("m_LocalAABB"), meshPositionsAABB(geometry.Positions)); err != nil {
		return nil, err
	}
	if err := setMeshValue(root, int64(unity2022DefaultMeshCookingOptions), "m_CookingOptions"); err != nil {
		return nil, err
	}
	if err := setMeshValue(root, float32(1), "m_MeshMetrics[0]"); err != nil {
		return nil, err
	}
	if err := setMeshValue(root, float32(1), "m_MeshMetrics[1]"); err != nil {
		return nil, err
	}

	object := &NativeUnityObject{ClassID: ClassIDMesh, TypeTree: tree}
	data, err := object.EncodeValue(root)
	if err != nil {
		return nil, fmt.Errorf("encode Mesh object: %w", err)
	}
	object.Data = data
	return object, nil
}

// validateMeshGeometryForBuild 验证几何数据的属性数量、图元拓扑和蒙皮一致性
// validateMeshGeometryForBuild validates attribute counts, primitive topology, and skin consistency of geometry data
func validateMeshGeometryForBuild(name string, geometry *MeshGeometry) error {
	if name == "" {
		return fmt.Errorf("Mesh name is required")
	}
	if geometry == nil || len(geometry.Positions) == 0 {
		return fmt.Errorf("Mesh geometry has no positions")
	}
	vertexCount := int64(len(geometry.Positions))
	if vertexCount > math.MaxUint32 {
		return fmt.Errorf("Mesh vertex count %d exceeds UInt32 range", vertexCount)
	}
	for _, attribute := range []struct {
		name  string
		count int64
	}{
		{"normal", int64(len(geometry.Normals))},
		{"tangent", int64(len(geometry.Tangents))},
		{"color", int64(len(geometry.Colors))},
	} {
		if attribute.count != 0 && attribute.count != vertexCount {
			return fmt.Errorf("Mesh %s count %d does not match vertex count %d", attribute.name, attribute.count, vertexCount)
		}
	}
	for setIndex := int64(0); setIndex < MeshTexCoordSetCount; setIndex++ {
		if count := int64(len(geometry.TexCoords[setIndex])); count != 0 && count != vertexCount {
			return fmt.Errorf("Mesh UV%d count %d does not match vertex count %d", setIndex, count, vertexCount)
		}
	}
	if geometry.SkinCounts == nil {
		if len(geometry.SkinIndices) != 0 || len(geometry.SkinWeights) != 0 || len(geometry.BindPoses) != 0 {
			return fmt.Errorf("Mesh skin arrays and bind poses require per-vertex skin counts")
		}
	} else {
		if int64(len(geometry.SkinCounts)) != vertexCount {
			return fmt.Errorf("Mesh skin count entries %d do not match vertex count %d", len(geometry.SkinCounts), vertexCount)
		}
		if len(geometry.BindPoses) == 0 {
			return fmt.Errorf("Mesh skin channels and bind poses must be provided together")
		}
		boneCount := int64(len(geometry.BindPoses))
		var total int64
		for vertexIndex, count := range geometry.SkinCounts {
			if count == 0 {
				return fmt.Errorf("Mesh vertex %d has no bone influences", vertexIndex)
			}
			total += int64(count)
		}
		if int64(len(geometry.SkinIndices)) != total || int64(len(geometry.SkinWeights)) != total {
			return fmt.Errorf("Mesh flattened skin arrays have %d indices and %d weights for %d influences", len(geometry.SkinIndices), len(geometry.SkinWeights), total)
		}
		maxCount := uint8(0)
		for _, count := range geometry.SkinCounts {
			if count > maxCount {
				maxCount = count
			}
		}
		for entryIndex, boneIndex := range geometry.SkinIndices {
			if int64(boneIndex) >= boneCount {
				return fmt.Errorf("Mesh skin entry %d references bone %d outside %d bind poses", entryIndex, boneIndex, boneCount)
			}
			if maxCount > 4 && boneIndex > math.MaxUint16 {
				return fmt.Errorf("Mesh skin entry %d bone index %d exceeds the UInt16 range required by variable bone weights", entryIndex, boneIndex)
			}
		}
		for entryIndex, weight := range geometry.SkinWeights {
			if !(weight > 0) || weight > 1 {
				return fmt.Errorf("Mesh skin entry %d weight %f is outside the zero-to-one range", entryIndex, weight)
			}
		}
	}
	if len(geometry.MorphTargets) != 0 {
		return fmt.Errorf("embedded Mesh blend shapes cannot be written; store morphs in the Model instead")
	}
	if len(geometry.Primitives) == 0 {
		return fmt.Errorf("Mesh geometry has no primitives")
	}
	for primitiveIndex, primitive := range geometry.Primitives {
		if primitive.Mode != MeshPrimitiveModeTriangles {
			return fmt.Errorf("Mesh primitive %d mode %d is unsupported; only triangles can be written", primitiveIndex, primitive.Mode)
		}
		if len(primitive.Indices) == 0 || len(primitive.Indices)%3 != 0 {
			return fmt.Errorf("Mesh primitive %d triangle index count %d is not a positive multiple of three", primitiveIndex, len(primitive.Indices))
		}
		for _, index := range primitive.Indices {
			if int64(index) >= vertexCount {
				return fmt.Errorf("Mesh primitive %d index %d targets a vertex outside %d vertices", primitiveIndex, index, vertexCount)
			}
		}
	}
	return nil
}

// meshSubMeshRange 描述一个 SubMesh 在索引缓冲中的范围与顶点范围 / meshSubMeshRange describes one SubMesh's index-buffer span and vertex range
type meshSubMeshRange struct {
	FirstByte   int64 // 索引缓冲字节偏移 / Index-buffer byte offset
	IndexCount  int64 // 索引数量 / Index count
	FirstVertex int64 // 引用的最小顶点索引 / Lowest referenced vertex index
	VertexCount int64 // 引用的顶点范围长度 / Referenced vertex range length
}

// buildMeshIndexBuffer 根据顶点数选择索引宽度并编码所有 SubMesh 的索引
// buildMeshIndexBuffer selects the index width from the vertex count and encodes every SubMesh's indices
func buildMeshIndexBuffer(geometry *MeshGeometry) (int64, []byte, []meshSubMeshRange, error) {
	var indexFormat int64
	indexSize := int64(2)
	if len(geometry.Positions) > math.MaxUint16 {
		indexFormat = 1
		indexSize = 4
	}
	var totalIndices int64
	for _, primitive := range geometry.Primitives {
		totalIndices += int64(len(primitive.Indices))
	}
	buffer := make([]byte, 0, totalIndices*indexSize)
	ranges := make([]meshSubMeshRange, 0, len(geometry.Primitives))
	for _, primitive := range geometry.Primitives {
		entry := meshSubMeshRange{FirstByte: int64(len(buffer)), IndexCount: int64(len(primitive.Indices))}
		minIndex, maxIndex := int64(math.MaxInt64), int64(-1)
		for _, index := range primitive.Indices {
			if int64(index) < minIndex {
				minIndex = int64(index)
			}
			if int64(index) > maxIndex {
				maxIndex = int64(index)
			}
			if indexSize == 2 {
				buffer = binary.LittleEndian.AppendUint16(buffer, uint16(index))
			} else {
				buffer = binary.LittleEndian.AppendUint32(buffer, index)
			}
		}
		entry.FirstVertex = minIndex
		entry.VertexCount = maxIndex - minIndex + 1
		ranges = append(ranges, entry)
	}
	return indexFormat, buffer, ranges, nil
}

// meshAABBBounds 表示一个用最小值和最大值描述的包围盒 / meshAABBBounds represents a bounding box described by its minimum and maximum corners
type meshAABBBounds struct {
	Min [3]float32 // 最小角 / Minimum corner
	Max [3]float32 // 最大角 / Maximum corner
}

// expand 将一个点并入包围盒
// expand merges one point into the bounding box
func (b *meshAABBBounds) expand(point [3]float32, first bool) {
	for axis := int64(0); axis < 3; axis++ {
		if first || point[axis] < b.Min[axis] {
			b.Min[axis] = point[axis]
		}
		if first || point[axis] > b.Max[axis] {
			b.Max[axis] = point[axis]
		}
	}
}

// meshPositionsAABB 计算一组顶点位置的包围盒
// meshPositionsAABB computes the bounding box of a set of vertex positions
func meshPositionsAABB(positions [][3]float32) meshAABBBounds {
	var bounds meshAABBBounds
	for positionIndex, position := range positions {
		bounds.expand(position, positionIndex == 0)
	}
	return bounds
}

// fillMeshSubMeshes 写入 m_SubMeshes 数组，包括索引范围、顶点范围和局部包围盒
// fillMeshSubMeshes fills the m_SubMeshes array with index spans, vertex ranges, and local bounding boxes
func fillMeshSubMeshes(tree *TypeTreeType, root *TypeTreeValue, geometry *MeshGeometry, ranges []meshSubMeshRange) error {
	subMeshes, err := meshValueField(root, "m_SubMeshes")
	if err != nil {
		return err
	}
	for rangeIndex, entry := range ranges {
		element, err := newMeshArrayElement(tree, subMeshes, rangeIndex)
		if err != nil {
			return err
		}
		primitive := geometry.Primitives[rangeIndex]
		var bounds meshAABBBounds
		for indexIndex, index := range primitive.Indices {
			bounds.expand(geometry.Positions[index], indexIndex == 0)
		}
		for _, field := range []struct {
			name  string
			value int64
		}{
			{"firstByte", entry.FirstByte},
			{"indexCount", entry.IndexCount},
			{"topology", 0},
			{"baseVertex", 0},
			{"firstVertex", entry.FirstVertex},
			{"vertexCount", entry.VertexCount},
		} {
			if err := setMeshValue(element, field.value, field.name); err != nil {
				return err
			}
		}
		if err := setMeshAABB(element.Field("localAABB"), bounds); err != nil {
			return err
		}
		subMeshes.Children = append(subMeshes.Children, element)
	}
	return nil
}

// fillMeshBindPoses 写入 m_BindPose 矩阵数组
// fillMeshBindPoses fills the m_BindPose matrix array
func fillMeshBindPoses(tree *TypeTreeType, root *TypeTreeValue, geometry *MeshGeometry) error {
	if len(geometry.BindPoses) == 0 {
		return nil
	}
	bindPose, err := meshValueField(root, "m_BindPose")
	if err != nil {
		return err
	}
	for matrixIndex, matrix := range geometry.BindPoses {
		element, err := newMeshArrayElement(tree, bindPose, matrixIndex)
		if err != nil {
			return err
		}
		if len(element.Children) != 16 {
			return fmt.Errorf("Mesh bind pose element skeleton has %d children instead of sixteen", len(element.Children))
		}
		for elementIndex := int64(0); elementIndex < 16; elementIndex++ {
			element.Children[elementIndex].Value = matrix[elementIndex]
		}
		bindPose.Children = append(bindPose.Children, element)
	}
	return nil
}

// fillMeshBonesAABB 在绑定姿势空间中为每根骨骼计算受影响顶点的包围盒并写入 m_BonesAABB
// fillMeshBonesAABB computes each bone's bounding box over influenced vertices in bind-pose space and fills m_BonesAABB
func fillMeshBonesAABB(tree *TypeTreeType, root *TypeTreeValue, geometry *MeshGeometry) error {
	boneCount := int64(len(geometry.BindPoses))
	if boneCount == 0 {
		return nil
	}
	bounds := make([]meshAABBBounds, boneCount)
	seen := make([]bool, boneCount)
	cursor := int64(0)
	for vertexIndex, count := range geometry.SkinCounts {
		for entryIndex := int64(0); entryIndex < int64(count); entryIndex++ {
			if geometry.SkinWeights[cursor+entryIndex] <= 0 {
				continue
			}
			boneIndex := int64(geometry.SkinIndices[cursor+entryIndex])
			local := meshTransformPoint(geometry.BindPoses[boneIndex], geometry.Positions[vertexIndex])
			bounds[boneIndex].expand(local, !seen[boneIndex])
			seen[boneIndex] = true
		}
		cursor += int64(count)
	}
	bonesAABB, err := meshValueField(root, "m_BonesAABB")
	if err != nil {
		return err
	}
	for boneIndex := int64(0); boneIndex < boneCount; boneIndex++ {
		element, err := newMeshArrayElement(tree, bonesAABB, int(boneIndex))
		if err != nil {
			return err
		}
		if err := setMeshVector3(element.Field("m_Min"), bounds[boneIndex].Min); err != nil {
			return err
		}
		if err := setMeshVector3(element.Field("m_Max"), bounds[boneIndex].Max); err != nil {
			return err
		}
		bonesAABB.Children = append(bonesAABB.Children, element)
	}
	return nil
}

// meshTransformPoint 用 e00 到 e33 行主序矩阵变换一个点
// meshTransformPoint transforms one point with an e00 through e33 row-major matrix
func meshTransformPoint(matrix [16]float32, point [3]float32) [3]float32 {
	var result [3]float32
	for row := int64(0); row < 3; row++ {
		result[row] = matrix[row*4]*point[0] + matrix[row*4+1]*point[1] + matrix[row*4+2]*point[2] + matrix[row*4+3]
	}
	return result
}

// fillMeshVariableBoneWeights 按官方布局写入 m_VariableBoneCountWeights 的偏移表和打包权重对
// 每顶点的打包权重被调整为总和恰为 65535，余量记入该顶点的最大权重
// fillMeshVariableBoneWeights fills the offset table and packed weight pairs of m_VariableBoneCountWeights in the official layout
// Packed weights per vertex are adjusted to sum to exactly 65535, with the remainder assigned to the vertex's largest weight
func fillMeshVariableBoneWeights(root *TypeTreeValue, geometry *MeshGeometry) error {
	data, err := meshValueField(root, "m_VariableBoneCountWeights", "m_Data")
	if err != nil {
		return err
	}
	vertexCount := int64(len(geometry.SkinCounts))
	tableSize := vertexCount + 1
	entries := make([]*TypeTreeValue, 0, tableSize+int64(len(geometry.SkinIndices)))
	appendEntry := func(value int64) {
		entries = append(entries, &TypeTreeValue{
			TypeName: "unsigned int",
			Name:     fmt.Sprintf("data[%d]", len(entries)),
			Value:    value,
		})
	}
	offset := tableSize
	appendEntry(offset)
	for _, count := range geometry.SkinCounts {
		offset += int64(count)
		appendEntry(offset)
	}
	cursor := int64(0)
	for vertexIndex, count := range geometry.SkinCounts {
		influences := int64(count)
		var sum float64
		largest, largestWeight := cursor, float32(-1)
		for entryIndex := cursor; entryIndex < cursor+influences; entryIndex++ {
			sum += float64(geometry.SkinWeights[entryIndex])
			if geometry.SkinWeights[entryIndex] > largestWeight {
				largestWeight = geometry.SkinWeights[entryIndex]
				largest = entryIndex
			}
		}
		if sum <= 0 {
			return fmt.Errorf("Mesh vertex %d bone weights sum to %f", vertexIndex, sum)
		}
		remainder := int64(65535)
		packed := make([]int64, influences)
		for entryIndex := cursor; entryIndex < cursor+influences; entryIndex++ {
			if entryIndex == largest {
				continue
			}
			scaled := int64(math.Round(float64(geometry.SkinWeights[entryIndex]) / sum * 65535))
			if scaled > remainder {
				scaled = remainder
			}
			packed[entryIndex-cursor] = scaled
			remainder -= scaled
		}
		packed[largest-cursor] = remainder
		for entryIndex := cursor; entryIndex < cursor+influences; entryIndex++ {
			appendEntry(packed[entryIndex-cursor]<<16 | int64(geometry.SkinIndices[entryIndex]))
		}
		cursor += influences
	}
	data.Children = entries
	return nil
}

// meshBuildChannel 描述写出时一个启用的顶点通道来源 / meshBuildChannel describes one enabled vertex-channel source during writing
type meshBuildChannel struct {
	Slot          int64                                                     // 语义通道索引 / Semantic channel index
	Stream        uint8                                                     // 顶点流索引 / Vertex stream index
	Format        uint8                                                     // Unity VertexAttributeFormat / Unity VertexAttributeFormat
	Dimension     uint8                                                     // 分量数量 / Component count
	ComponentSize int64                                                     // 单个分量的字节数 / Byte size of one component
	Put           func(vertexIndex int64, componentIndex int64, out []byte) // 写入一个分量的字节 / Writes the bytes of one component
}

// meshFixedSkin 保存从变长蒙皮换算出的每顶点前四影响 / meshFixedSkin stores the per-vertex top-four influences converted from ragged skin data
type meshFixedSkin struct {
	Weights16 [][4]uint16  // 重归一化的 UNorm16 权重 / Renormalized UNorm16 weights
	Weights   [][4]float32 // 原始浮点权重 / Raw floating-point weights
	Indices   [][4]uint32  // 骨骼索引 / Bone indices
	Variable  bool         // 是否需要变长权重表 / Whether the variable weight table is required
}

// buildMeshFixedSkin 将变长蒙皮排序后取前四影响，超过四影响时重归一化为总和恰为 65535 的 UNorm16
// buildMeshFixedSkin sorts ragged skin data and keeps the top four influences, renormalizing them to UNorm16 values summing to exactly 65535 when more than four influences exist
func buildMeshFixedSkin(geometry *MeshGeometry) (*meshFixedSkin, error) {
	vertexCount := int64(len(geometry.Positions))
	fixed := &meshFixedSkin{
		Weights16: make([][4]uint16, vertexCount),
		Weights:   make([][4]float32, vertexCount),
		Indices:   make([][4]uint32, vertexCount),
	}
	cursor := int64(0)
	for vertexIndex := int64(0); vertexIndex < vertexCount; vertexIndex++ {
		count := int64(geometry.SkinCounts[vertexIndex])
		if count > 4 {
			fixed.Variable = true
		}
		type influence struct {
			weight float32
			bone   uint32
		}
		influences := make([]influence, count)
		for entryIndex := int64(0); entryIndex < count; entryIndex++ {
			influences[entryIndex] = influence{weight: geometry.SkinWeights[cursor+entryIndex], bone: geometry.SkinIndices[cursor+entryIndex]}
		}
		cursor += count
		for outer := int64(1); outer < count; outer++ {
			for inner := outer; inner > 0 && influences[inner].weight > influences[inner-1].weight; inner-- {
				influences[inner], influences[inner-1] = influences[inner-1], influences[inner]
			}
		}
		top := count
		if top > 4 {
			top = 4
		}
		var topSum float32
		for entryIndex := int64(0); entryIndex < top; entryIndex++ {
			topSum += influences[entryIndex].weight
		}
		if topSum <= 0 {
			return nil, fmt.Errorf("Mesh vertex %d bone weights sum to %f", vertexIndex, topSum)
		}
		remainder := int64(65535)
		for entryIndex := top - 1; entryIndex >= 0; entryIndex-- {
			fixed.Weights[vertexIndex][entryIndex] = influences[entryIndex].weight
			fixed.Indices[vertexIndex][entryIndex] = influences[entryIndex].bone
			if entryIndex == 0 {
				fixed.Weights16[vertexIndex][entryIndex] = uint16(remainder)
			} else {
				scaled := int64(math.Round(float64(influences[entryIndex].weight) / float64(topSum) * 65535))
				if scaled > remainder {
					scaled = remainder
				}
				fixed.Weights16[vertexIndex][entryIndex] = uint16(scaled)
				remainder -= scaled
			}
		}
	}
	return fixed, nil
}

// buildMeshChannels 按官方流布局枚举启用的顶点通道
// 四影响以内的蒙皮使用 float32 权重和 UInt32 索引，超过四影响时匹配官方的 UNorm16 权重和 UInt16 索引
// buildMeshChannels enumerates enabled vertex channels using the official stream layout
// Skins within four influences use float32 weights and UInt32 indices, while more than four influences match the official UNorm16 weights and UInt16 indices
func buildMeshChannels(geometry *MeshGeometry, skin *meshFixedSkin) []meshBuildChannel {
	channels := make([]meshBuildChannel, 0, 14)
	putFloat3 := func(values [][3]float32) func(int64, int64, []byte) {
		return func(vertexIndex int64, componentIndex int64, out []byte) {
			binary.LittleEndian.PutUint32(out, math.Float32bits(values[vertexIndex][componentIndex]))
		}
	}
	putFloat4 := func(values [][4]float32) func(int64, int64, []byte) {
		return func(vertexIndex int64, componentIndex int64, out []byte) {
			binary.LittleEndian.PutUint32(out, math.Float32bits(values[vertexIndex][componentIndex]))
		}
	}
	channels = append(channels, meshBuildChannel{Slot: meshChannelPosition, Format: 0, Dimension: 3, ComponentSize: 4, Put: putFloat3(geometry.Positions)})
	if len(geometry.Normals) != 0 {
		channels = append(channels, meshBuildChannel{Slot: meshChannelNormal, Format: 0, Dimension: 3, ComponentSize: 4, Put: putFloat3(geometry.Normals)})
	}
	if len(geometry.Tangents) != 0 {
		channels = append(channels, meshBuildChannel{Slot: meshChannelTangent, Format: 0, Dimension: 4, ComponentSize: 4, Put: putFloat4(geometry.Tangents)})
	}
	if len(geometry.Colors) != 0 {
		channels = append(channels, meshBuildChannel{Slot: meshChannelColor, Format: 0, Dimension: 4, ComponentSize: 4, Put: putFloat4(geometry.Colors)})
	}
	uvStream := false
	for setIndex := int64(0); setIndex < MeshTexCoordSetCount; setIndex++ {
		if len(geometry.TexCoords[setIndex]) == 0 {
			continue
		}
		uvStream = true
		values := geometry.TexCoords[setIndex]
		channels = append(channels, meshBuildChannel{Slot: meshChannelTexCoord0 + setIndex, Stream: 1, Format: 0, Dimension: 2, ComponentSize: 4, Put: func(vertexIndex int64, componentIndex int64, out []byte) {
			binary.LittleEndian.PutUint32(out, math.Float32bits(values[vertexIndex][componentIndex]))
		}})
	}
	if skin != nil {
		skinStream := uint8(1)
		if uvStream {
			skinStream = 2
		}
		if skin.Variable {
			channels = append(channels, meshBuildChannel{Slot: meshChannelBlendWeight, Stream: skinStream, Format: 4, Dimension: 4, ComponentSize: 2, Put: func(vertexIndex int64, componentIndex int64, out []byte) {
				binary.LittleEndian.PutUint16(out, skin.Weights16[vertexIndex][componentIndex])
			}})
			channels = append(channels, meshBuildChannel{Slot: meshChannelBlendIndices, Stream: skinStream, Format: 8, Dimension: 4, ComponentSize: 2, Put: func(vertexIndex int64, componentIndex int64, out []byte) {
				binary.LittleEndian.PutUint16(out, uint16(skin.Indices[vertexIndex][componentIndex]))
			}})
		} else {
			channels = append(channels, meshBuildChannel{Slot: meshChannelBlendWeight, Stream: skinStream, Format: 0, Dimension: 4, ComponentSize: 4, Put: putFloat4(skin.Weights)})
			channels = append(channels, meshBuildChannel{Slot: meshChannelBlendIndices, Stream: skinStream, Format: 10, Dimension: 4, ComponentSize: 4, Put: func(vertexIndex int64, componentIndex int64, out []byte) {
				binary.LittleEndian.PutUint32(out, skin.Indices[vertexIndex][componentIndex])
			}})
		}
	}
	return channels
}

// fillMeshVertexData 写入 m_VertexData 的顶点数、十四个通道描述和交错顶点字节
// fillMeshVertexData fills m_VertexData with the vertex count, fourteen channel descriptions, and interleaved vertex bytes
func fillMeshVertexData(tree *TypeTreeType, root *TypeTreeValue, geometry *MeshGeometry, skin *meshFixedSkin) error {
	vertexData, err := meshValueField(root, "m_VertexData")
	if err != nil {
		return err
	}
	vertexCount := int64(len(geometry.Positions))
	if err := setMeshValue(vertexData, vertexCount, "m_VertexCount"); err != nil {
		return err
	}
	sources := buildMeshChannels(geometry, skin)

	streamCount := int64(0)
	for _, source := range sources {
		if int64(source.Stream)+1 > streamCount {
			streamCount = int64(source.Stream) + 1
		}
	}
	offsets := make([]uint8, len(sources))
	strides := make([]int64, streamCount)
	for sourceIndex, source := range sources {
		if strides[source.Stream] > math.MaxUint8 {
			return fmt.Errorf("Mesh vertex stream %d stride exceeds the UInt8 channel offset range", source.Stream)
		}
		offsets[sourceIndex] = uint8(strides[source.Stream])
		strides[source.Stream] += int64(source.Dimension) * source.ComponentSize
	}
	streamOffsets := make([]int64, streamCount)
	var totalSize int64
	for streamIndex := int64(0); streamIndex < streamCount; streamIndex++ {
		streamOffsets[streamIndex] = totalSize
		totalSize += vertexCount * strides[streamIndex]
		totalSize = (totalSize + 15) &^ int64(15)
	}

	buffer := make([]byte, totalSize)
	for sourceIndex, source := range sources {
		base := streamOffsets[source.Stream] + int64(offsets[sourceIndex])
		stride := strides[source.Stream]
		for vertexIndex := int64(0); vertexIndex < vertexCount; vertexIndex++ {
			at := base + vertexIndex*stride
			for componentIndex := int64(0); componentIndex < int64(source.Dimension); componentIndex++ {
				source.Put(vertexIndex, componentIndex, buffer[at+componentIndex*source.ComponentSize:])
			}
		}
	}

	channelsValue, err := meshValueField(vertexData, "m_Channels")
	if err != nil {
		return err
	}
	sourceBySlot := make(map[int64]int64, len(sources))
	for sourceIndex := range sources {
		sourceBySlot[sources[sourceIndex].Slot] = int64(sourceIndex)
	}
	for slot := int64(0); slot < 14; slot++ {
		element, err := newMeshArrayElement(tree, channelsValue, int(slot))
		if err != nil {
			return err
		}
		if sourceIndex, active := sourceBySlot[slot]; active {
			source := sources[sourceIndex]
			for _, field := range []struct {
				name  string
				value int64
			}{
				{"stream", int64(source.Stream)},
				{"offset", int64(offsets[sourceIndex])},
				{"format", int64(source.Format)},
				{"dimension", int64(source.Dimension)},
			} {
				if err := setMeshValue(element, field.value, field.name); err != nil {
					return err
				}
			}
		}
		channelsValue.Children = append(channelsValue.Children, element)
	}
	dataSize, err := meshValueField(vertexData, "m_DataSize")
	if err != nil {
		return err
	}
	dataSize.Value = buffer
	return nil
}

// newTypeTreeValueSkeleton 按 TypeTree 先序构建标量为零、数组为空的默认值树，并返回下一个未消费节点索引
// newTypeTreeValueSkeleton builds a default value tree with zero scalars and empty arrays in TypeTree preorder and returns the next unconsumed node index
func newTypeTreeValueSkeleton(tt *TypeTreeType, idx int64) (*TypeTreeValue, int64, error) {
	if tt == nil || idx < 0 || idx >= int64(len(tt.Nodes)) {
		return nil, idx, fmt.Errorf("type tree node index %d out of range", idx)
	}
	node := &tt.Nodes[idx]
	value := &TypeTreeValue{
		TypeName:  tt.GetTypeTreeString(node, true),
		Name:      tt.GetTypeTreeString(node, false),
		NodeIndex: idx,
	}
	switch value.TypeName {
	case "string":
		value.Value = ""
		return value, skipSubtree(tt, idx), nil
	case "TypelessData":
		value.Value = []byte{}
		return value, skipSubtree(tt, idx), nil
	}
	next := idx + 1
	if next < int64(len(tt.Nodes)) && tt.Nodes[next].Level > node.Level {
		if isArrayNode(node, value.TypeName) {
			value.Children = []*TypeTreeValue{}
			return value, skipSubtree(tt, idx), nil
		}
		for next < int64(len(tt.Nodes)) && tt.Nodes[next].Level > node.Level {
			child, following, err := newTypeTreeValueSkeleton(tt, next)
			if err != nil {
				return nil, following, err
			}
			value.Children = append(value.Children, child)
			next = following
		}
		return value, next, nil
	}
	defaultValue, err := defaultTypeTreeScalar(value.TypeName)
	if err != nil {
		return nil, next, fmt.Errorf("node %d field %s: %w", idx, value.Name, err)
	}
	value.Value = defaultValue
	return value, next, nil
}

// defaultTypeTreeScalar 返回标量类型的零值，其表示与解码器产生的值类型一致
// defaultTypeTreeScalar returns a scalar type's zero value with the same representation the decoder produces
func defaultTypeTreeScalar(typeName string) (interface{}, error) {
	switch typeName {
	case "bool":
		return false, nil
	case "char", "SInt8", "UInt8", "unsigned char", "short", "SInt16", "unsigned short", "UInt16",
		"int", "SInt32", "unsigned int", "UInt32", "Type*", "long long", "SInt64":
		return int64(0), nil
	case "unsigned long long", "UInt64", "FileSize":
		return uint64(0), nil
	case "float":
		return float32(0), nil
	case "double":
		return float64(0), nil
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported scalar type %q", typeName)
	}
}

// findTypeTreeArrayDataNode 返回数组字段的元素 data 节点索引
// findTypeTreeArrayDataNode returns the element data node index of an array field
func findTypeTreeArrayDataNode(tt *TypeTreeType, idx int64) (int64, error) {
	if tt == nil || idx < 0 || idx >= int64(len(tt.Nodes)) {
		return 0, fmt.Errorf("type tree node index %d out of range", idx)
	}
	node := &tt.Nodes[idx]
	dataNodeIdx := idx + 1
	for dataNodeIdx < int64(len(tt.Nodes)) && tt.Nodes[dataNodeIdx].Level > node.Level && tt.GetTypeTreeString(&tt.Nodes[dataNodeIdx], false) != "data" {
		dataNodeIdx++
	}
	if dataNodeIdx >= int64(len(tt.Nodes)) || tt.Nodes[dataNodeIdx].Level <= node.Level {
		return 0, fmt.Errorf("array node %d has no data node", idx)
	}
	return dataNodeIdx, nil
}

// newMeshArrayElement 为数组字段构建一个元素的默认值骨架
// newMeshArrayElement builds one element's default value skeleton for an array field
func newMeshArrayElement(tt *TypeTreeType, arrayValue *TypeTreeValue, elementIndex int) (*TypeTreeValue, error) {
	if arrayValue == nil {
		return nil, fmt.Errorf("nil array value")
	}
	dataNodeIdx, err := findTypeTreeArrayDataNode(tt, arrayValue.NodeIndex)
	if err != nil {
		return nil, fmt.Errorf("array %s: %w", arrayValue.Name, err)
	}
	element, _, err := newTypeTreeValueSkeleton(tt, dataNodeIdx)
	if err != nil {
		return nil, fmt.Errorf("array %s element: %w", arrayValue.Name, err)
	}
	element.Name = fmt.Sprintf("data[%d]", elementIndex)
	return element, nil
}

// meshValueField 逐字段名导航并在缺失时报错
// meshValueField navigates by field names and reports an error when a field is missing
func meshValueField(value *TypeTreeValue, path ...string) (*TypeTreeValue, error) {
	current := value
	for _, name := range path {
		next := current.Field(name)
		if next == nil {
			return nil, fmt.Errorf("Mesh value has no field %q", name)
		}
		current = next
	}
	return current, nil
}

// setMeshValue 设置路径末端标量字段的值
// setMeshValue sets the value of the scalar field at the end of a path
func setMeshValue(value *TypeTreeValue, scalar interface{}, path ...string) error {
	field, err := meshValueField(value, path...)
	if err != nil {
		return err
	}
	field.Value = scalar
	return nil
}

// setMeshBytesValue 设置字节数组字段的连续字节内容
// setMeshBytesValue sets the contiguous byte content of a byte-array field
func setMeshBytesValue(value *TypeTreeValue, data []byte, path ...string) error {
	field, err := meshValueField(value, path...)
	if err != nil {
		return err
	}
	field.Children = nil
	field.Value = data
	return nil
}

// setMeshVector3 设置一个 Vector3f 字段的三个分量
// setMeshVector3 sets the three components of a Vector3f field
func setMeshVector3(field *TypeTreeValue, vector [3]float32) error {
	if field == nil {
		return fmt.Errorf("nil Vector3f field")
	}
	for componentIndex, componentName := range []string{"x", "y", "z"} {
		if err := setMeshValue(field, vector[componentIndex], componentName); err != nil {
			return err
		}
	}
	return nil
}

// setMeshAABB 设置一个 AABB 字段的中心和半长
// setMeshAABB sets the center and extent of an AABB field
func setMeshAABB(field *TypeTreeValue, bounds meshAABBBounds) error {
	if field == nil {
		return fmt.Errorf("nil AABB field")
	}
	var center, extent [3]float32
	for axis := int64(0); axis < 3; axis++ {
		center[axis] = (bounds.Min[axis] + bounds.Max[axis]) / 2
		extent[axis] = (bounds.Max[axis] - bounds.Min[axis]) / 2
	}
	if err := setMeshVector3(field.Field("m_Center"), center); err != nil {
		return err
	}
	return setMeshVector3(field.Field("m_Extent"), extent)
}
