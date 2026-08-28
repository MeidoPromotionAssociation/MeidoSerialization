package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

// ConvertGLTFToModel 将 glTF 或 GLB 转换为 .model 与 .mmesh 文件并写入输出目录
// 文件名优先取自导出时保存的 kcesModel extras，否则从输入文件名小写派生
// ConvertGLTFToModel converts glTF or GLB into .model and .mmesh files written to the output directory
// File names prefer the kcesModel extras saved during export and otherwise derive from the lowercased input file name
func (s *ModelService) ConvertGLTFToModel(ctx context.Context, inputPath string, outputDir string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	document, err := gltf.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open glTF %q: %w", inputPath, err)
	}
	model, geometry, extras, err := decodeGLTFModelDocument(document)
	if err != nil {
		return fmt.Errorf("convert glTF %q to Model: %w", inputPath, err)
	}

	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath)))
	fileName := stem + ".model"
	meshFileName := stem + ".mmesh"
	if extras != nil && extras.FileName != "" {
		fileName = extras.FileName
	}
	if extras != nil && extras.MeshFileName != "" {
		meshFileName = extras.MeshFileName
	}
	model.FileName = &fileName
	model.MeshFileName = &meshFileName

	meshObject, err := aba.NewNativeMeshObject(meshFileName, geometry)
	if err != nil {
		return fmt.Errorf("build native Mesh for %q: %w", inputPath, err)
	}
	meshBytes, err := aba.WriteMMesh(meshObject)
	if err != nil {
		return fmt.Errorf("encode native Mesh for %q: %w", inputPath, err)
	}
	modelBytes, err := serializationKCES.EncodeModel(model)
	if err != nil {
		return fmt.Errorf("encode Model for %q: %w", inputPath, err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory %q: %w", outputDir, err)
	}
	if err := writeNativeUnityGLTFOutput(ctx, filepath.Join(outputDir, meshFileName), meshBytes, maxOutputBytes); err != nil {
		return err
	}
	return writeNativeUnityGLTFOutput(ctx, filepath.Join(outputDir, fileName), modelBytes, maxOutputBytes)
}

// decodeGLTFModelDocument 从 glTF 文档还原 Model 骨架数据和左手坐标网格几何
// decodeGLTFModelDocument restores Model skeleton data and left-handed mesh geometry from a glTF document
func decodeGLTFModelDocument(document *gltf.Document) (*serializationKCES.Model, *aba.MeshGeometry, *kcesModelExtras, error) {
	extras, err := parseKCESModelExtras(document)
	if err != nil {
		return nil, nil, nil, err
	}
	meshNodeIndex := -1
	for nodeIndex, node := range document.Nodes {
		if node != nil && node.Mesh != nil {
			if meshNodeIndex >= 0 {
				return nil, nil, nil, fmt.Errorf("document has more than one mesh node; exactly one is required")
			}
			meshNodeIndex = nodeIndex
		}
	}
	if meshNodeIndex < 0 {
		return nil, nil, nil, fmt.Errorf("document has no mesh node")
	}
	meshNode := document.Nodes[meshNodeIndex]
	if *meshNode.Mesh < 0 || *meshNode.Mesh >= len(document.Meshes) {
		return nil, nil, nil, fmt.Errorf("mesh node references mesh %d out of range", *meshNode.Mesh)
	}

	order, parents, err := collectGLTFSceneNodes(document)
	if err != nil {
		return nil, nil, nil, err
	}
	transIndexByNode := make(map[int]int, len(order))
	nodeNames := make(map[string]bool, len(order))
	for transIndex, nodeIndex := range order {
		name := document.Nodes[nodeIndex].Name
		if name == "" {
			return nil, nil, nil, fmt.Errorf("node %d has no name; every skeleton node needs a unique name", nodeIndex)
		}
		if nodeNames[name] {
			return nil, nil, nil, fmt.Errorf("node name %q is duplicated; skeleton node names must be unique", name)
		}
		nodeNames[name] = true
		transIndexByNode[nodeIndex] = transIndex
	}
	sclBones := make(map[string]bool)
	if extras != nil {
		for _, boneName := range extras.SCLBones {
			sclBones[boneName] = true
		}
	}
	transData := make([]*serializationKCES.TransData, len(order))
	for transIndex, nodeIndex := range order {
		node := document.Nodes[nodeIndex]
		if node.Matrix != ([16]float64{}) && node.Matrix != gltf.DefaultMatrix {
			return nil, nil, nil, fmt.Errorf("node %q uses a matrix transform; decompose it to translation, rotation, and scale before converting", node.Name)
		}
		translation := node.TranslationOrDefault()
		rotation := node.RotationOrDefault()
		scale := node.ScaleOrDefault()
		parentNo := int32(-1)
		if parentNode, hasParent := parents[nodeIndex]; hasParent {
			parentNo = int32(transIndexByNode[parentNode])
		}
		name := node.Name
		transData[transIndex] = &serializationKCES.TransData{
			Name:     &name,
			ParentNo: parentNo,
			IsSCL:    sclBones[name],
			Pos:      serializationKCES.Vector3{X: float32(-translation[0]), Y: float32(translation[1]), Z: float32(translation[2])},
			Rot:      serializationKCES.Vector4{X: float32(rotation[0]), Y: float32(-rotation[1]), Z: float32(-rotation[2]), W: float32(rotation[3])},
			Scale:    serializationKCES.Vector3{X: float32(scale[0]), Y: float32(scale[1]), Z: float32(scale[2])},
		}
	}

	geometry, primitiveMaterials, primitivePools, err := decodeGLTFMeshGeometry(document, document.Meshes[*meshNode.Mesh])
	if err != nil {
		return nil, nil, nil, err
	}
	geometry.Name = meshNode.Name

	model := serializationKCES.NewModel()
	modelName := meshNode.Name
	model.ModelName = &modelName
	model.TransData = transData
	if extras != nil {
		model.Version = extras.Version
		model.IndexedArrayWidth = extras.IndexedArrayWidth
		model.ShadowModeFlags = extras.ShadowModeFlags
		model.SkinThick = extras.SkinThick
		if extras.ModelName != "" {
			if extras.ModelName != modelName {
				return nil, nil, nil, fmt.Errorf("extras modelName %q does not match the mesh node name %q", extras.ModelName, modelName)
			}
		}
	}

	if meshNode.Skin != nil {
		if *meshNode.Skin < 0 || *meshNode.Skin >= len(document.Skins) {
			return nil, nil, nil, fmt.Errorf("mesh node references skin %d out of range", *meshNode.Skin)
		}
		skin := document.Skins[*meshNode.Skin]
		if len(skin.Joints) == 0 {
			return nil, nil, nil, fmt.Errorf("skin has no joints")
		}
		boneNames := make([]*string, len(skin.Joints))
		for jointIndex, jointNode := range skin.Joints {
			if jointNode < 0 || jointNode >= len(document.Nodes) {
				return nil, nil, nil, fmt.Errorf("skin joint %d references node %d out of range", jointIndex, jointNode)
			}
			if _, inScene := transIndexByNode[jointNode]; !inScene {
				return nil, nil, nil, fmt.Errorf("skin joint %q is not part of the scene hierarchy", document.Nodes[jointNode].Name)
			}
			name := document.Nodes[jointNode].Name
			boneNames[jointIndex] = &name
		}
		model.BoneNames = boneNames
		geometry.BindPoses, err = readGLTFInverseBindMatrices(document, skin, len(skin.Joints))
		if err != nil {
			return nil, nil, nil, err
		}
		if geometry.SkinCounts == nil {
			return nil, nil, nil, fmt.Errorf("mesh node has a skin but the primitives carry no JOINTS and WEIGHTS attributes")
		}
	} else {
		if geometry.SkinCounts != nil {
			return nil, nil, nil, fmt.Errorf("primitives carry skin attributes but the mesh node has no skin")
		}
		// 无蒙皮场景合成单骨绑定，让网格刚性跟随 modelName 节点
		// An unskinned scene synthesizes a single-bone binding so the mesh follows the modelName node rigidly
		name := modelName
		model.BoneNames = []*string{&name}
		geometry.BindPoses = [][16]float32{{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}}
		vertexCount := len(geometry.Positions)
		geometry.SkinCounts = make([]uint8, vertexCount)
		geometry.SkinIndices = make([]uint32, vertexCount)
		geometry.SkinWeights = make([]float32, vertexCount)
		for vertexIndex := range geometry.SkinCounts {
			geometry.SkinCounts[vertexIndex] = 1
			geometry.SkinWeights[vertexIndex] = 1
		}
	}

	model.Morphs, err = decodeGLTFMorphTargets(document, document.Meshes[*meshNode.Mesh], geometry, primitivePools)
	if err != nil {
		return nil, nil, nil, err
	}

	if extras != nil && len(extras.MaterialFileNames) != 0 {
		model.MaterialFileName = make([]*string, len(extras.MaterialFileNames))
		for materialIndex := range extras.MaterialFileNames {
			model.MaterialFileName[materialIndex] = &extras.MaterialFileNames[materialIndex]
		}
	} else {
		model.MaterialFileName = make([]*string, len(primitiveMaterials))
		for materialIndex, materialName := range primitiveMaterials {
			if materialName == "" {
				return nil, nil, nil, fmt.Errorf("primitive %d has no named material; name every glTF material after its target entry in the .materialassets container", materialIndex)
			}
			name := materialName
			model.MaterialFileName[materialIndex] = &name
		}
	}
	return model, geometry, extras, nil
}

// parseKCESModelExtras 解析文档 extras 中的 kcesModel 字段
// parseKCESModelExtras parses the kcesModel field in document extras
func parseKCESModelExtras(document *gltf.Document) (*kcesModelExtras, error) {
	extrasMap, ok := document.Extras.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	raw, ok := extrasMap[KCESModelExtrasKey]
	if !ok {
		return nil, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode kcesModel extras: %w", err)
	}
	extras := &kcesModelExtras{}
	if err := json.Unmarshal(encoded, extras); err != nil {
		return nil, fmt.Errorf("parse kcesModel extras: %w", err)
	}
	return extras, nil
}

// collectGLTFSceneNodes 按先序收集场景节点并返回节点的父子关系
// collectGLTFSceneNodes collects scene nodes in preorder and returns the parent relationships
func collectGLTFSceneNodes(document *gltf.Document) ([]int, map[int]int, error) {
	if len(document.Scenes) == 0 {
		return nil, nil, fmt.Errorf("document has no scene")
	}
	sceneIndex := 0
	if document.Scene != nil {
		sceneIndex = *document.Scene
	}
	if sceneIndex < 0 || sceneIndex >= len(document.Scenes) {
		return nil, nil, fmt.Errorf("default scene %d out of range", sceneIndex)
	}
	var order []int
	parents := make(map[int]int)
	visited := make(map[int]bool)
	var walk func(nodeIndex int) error
	walk = func(nodeIndex int) error {
		if nodeIndex < 0 || nodeIndex >= len(document.Nodes) || document.Nodes[nodeIndex] == nil {
			return fmt.Errorf("node index %d out of range", nodeIndex)
		}
		if visited[nodeIndex] {
			return fmt.Errorf("node %q appears in the hierarchy more than once", document.Nodes[nodeIndex].Name)
		}
		visited[nodeIndex] = true
		order = append(order, nodeIndex)
		for _, childIndex := range document.Nodes[nodeIndex].Children {
			parents[childIndex] = nodeIndex
			if err := walk(childIndex); err != nil {
				return err
			}
		}
		return nil
	}
	for _, rootIndex := range document.Scenes[sceneIndex].Nodes {
		if err := walk(rootIndex); err != nil {
			return nil, nil, err
		}
	}
	if len(order) == 0 {
		return nil, nil, fmt.Errorf("scene has no nodes")
	}
	return order, parents, nil
}

// readGLTFInverseBindMatrices 读取蒙皮的逆绑定矩阵并镜像回 Unity 左手坐标
// readGLTFInverseBindMatrices reads the skin's inverse bind matrices and mirrors them back to Unity left-handed coordinates
func readGLTFInverseBindMatrices(document *gltf.Document, skin *gltf.Skin, jointCount int) ([][16]float32, error) {
	bindPoses := make([][16]float32, jointCount)
	if skin.InverseBindMatrices == nil {
		for matrixIndex := range bindPoses {
			bindPoses[matrixIndex] = [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
		}
		return bindPoses, nil
	}
	if *skin.InverseBindMatrices < 0 || *skin.InverseBindMatrices >= len(document.Accessors) {
		return nil, fmt.Errorf("skin inverseBindMatrices accessor %d out of range", *skin.InverseBindMatrices)
	}
	matrices, err := modeler.ReadInverseBindMatrices(document, document.Accessors[*skin.InverseBindMatrices], nil)
	if err != nil {
		return nil, fmt.Errorf("read inverse bind matrices: %w", err)
	}
	if len(matrices) < jointCount {
		return nil, fmt.Errorf("inverse bind matrix count %d is smaller than joint count %d", len(matrices), jointCount)
	}
	for matrixIndex := 0; matrixIndex < jointCount; matrixIndex++ {
		bindPoses[matrixIndex] = mirrorUnityMatrixX(unityMatrixFromGLTF(matrices[matrixIndex]))
	}
	return bindPoses, nil
}

// gltfPrimitivePool 记录一组共享访问器的图元在合并顶点池中的基址 / gltfPrimitivePool records the base offset of primitives sharing accessors within the merged vertex pool
type gltfPrimitivePool struct {
	Base        int64 // 顶点池基址 / Vertex pool base offset
	VertexCount int64 // 顶点数量 / Vertex count
	First       bool  // 是否为该组的第一个图元 / Whether this is the group's first primitive
}

// decodeGLTFMeshGeometry 读取全部图元属性并合并为共享顶点池的左手坐标几何
// decodeGLTFMeshGeometry reads every primitive's attributes and merges them into left-handed geometry with a shared vertex pool
func decodeGLTFMeshGeometry(document *gltf.Document, mesh *gltf.Mesh) (*aba.MeshGeometry, []string, []*gltfPrimitivePool, error) {
	if mesh == nil || len(mesh.Primitives) == 0 {
		return nil, nil, nil, fmt.Errorf("mesh has no primitives")
	}
	geometry := &aba.MeshGeometry{Name: mesh.Name}
	materials := make([]string, 0, len(mesh.Primitives))
	pools := make(map[string]*gltfPrimitivePool)
	primitivePools := make([]*gltfPrimitivePool, len(mesh.Primitives))

	reference := mesh.Primitives[0].Attributes
	for primitiveIndex, primitive := range mesh.Primitives {
		if primitive == nil {
			return nil, nil, nil, fmt.Errorf("primitive %d is null", primitiveIndex)
		}
		if primitive.Mode != gltf.PrimitiveTriangles {
			return nil, nil, nil, fmt.Errorf("primitive %d mode is not triangles", primitiveIndex)
		}
		for attributeName := range reference {
			if _, ok := primitive.Attributes[attributeName]; !ok {
				return nil, nil, nil, fmt.Errorf("primitive %d is missing attribute %s carried by primitive 0; all primitives must share the same attribute set", primitiveIndex, attributeName)
			}
		}
		for attributeName := range primitive.Attributes {
			if _, ok := reference[attributeName]; !ok {
				return nil, nil, nil, fmt.Errorf("primitive 0 is missing attribute %s carried by primitive %d; all primitives must share the same attribute set", attributeName, primitiveIndex)
			}
		}

		key := gltfAttributeSignature(primitive.Attributes)
		pool, exists := pools[key]
		if !exists {
			base := int64(len(geometry.Positions))
			vertexCount, err := appendGLTFPrimitiveVertices(document, primitive, geometry)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("primitive %d: %w", primitiveIndex, err)
			}
			pool = &gltfPrimitivePool{Base: base, VertexCount: vertexCount, First: true}
			pools[key] = pool
		} else {
			pool = &gltfPrimitivePool{Base: pool.Base, VertexCount: pool.VertexCount}
		}
		primitivePools[primitiveIndex] = pool

		indices, err := readGLTFPrimitiveIndices(document, primitive, pool.VertexCount)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("primitive %d: %w", primitiveIndex, err)
		}
		for index := int64(0); index < int64(len(indices)); index += 3 {
			indices[index+1], indices[index+2] = indices[index+2], indices[index+1]
		}
		for indexIndex := range indices {
			indices[indexIndex] += uint32(pool.Base)
		}
		geometry.Primitives = append(geometry.Primitives, aba.MeshPrimitive{Mode: aba.MeshPrimitiveModeTriangles, Indices: indices})

		materialName := ""
		if primitive.Material != nil {
			if *primitive.Material < 0 || *primitive.Material >= len(document.Materials) {
				return nil, nil, nil, fmt.Errorf("primitive %d references material %d out of range", primitiveIndex, *primitive.Material)
			}
			materialName = document.Materials[*primitive.Material].Name
		}
		materials = append(materials, materialName)
	}
	return geometry, materials, primitivePools, nil
}

// gltfAttributeSignature 用排序后的属性访问器索引构造顶点池共享键
// gltfAttributeSignature builds a vertex-pool sharing key from sorted attribute accessor indices
func gltfAttributeSignature(attributes gltf.PrimitiveAttributes) string {
	keys := make([]string, 0, len(attributes))
	for attributeName, accessorIndex := range attributes {
		keys = append(keys, fmt.Sprintf("%s=%d", attributeName, accessorIndex))
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// readGLTFPrimitiveIndices 读取图元索引，未提供索引时生成顺序索引
// readGLTFPrimitiveIndices reads primitive indices and generates sequential indices when none are provided
func readGLTFPrimitiveIndices(document *gltf.Document, primitive *gltf.Primitive, vertexCount int64) ([]uint32, error) {
	if primitive.Indices == nil {
		indices := make([]uint32, vertexCount)
		for indexIndex := range indices {
			indices[indexIndex] = uint32(indexIndex)
		}
		if len(indices)%3 != 0 {
			return nil, fmt.Errorf("non-indexed primitive vertex count %d is not divisible by three", vertexCount)
		}
		return indices, nil
	}
	if *primitive.Indices < 0 || *primitive.Indices >= len(document.Accessors) {
		return nil, fmt.Errorf("index accessor %d out of range", *primitive.Indices)
	}
	indices, err := modeler.ReadIndices(document, document.Accessors[*primitive.Indices], nil)
	if err != nil {
		return nil, fmt.Errorf("read indices: %w", err)
	}
	if len(indices)%3 != 0 {
		return nil, fmt.Errorf("index count %d is not divisible by three", len(indices))
	}
	for indexIndex, index := range indices {
		if int64(index) >= vertexCount {
			return nil, fmt.Errorf("index %d targets vertex %d outside %d vertices", indexIndex, index, vertexCount)
		}
	}
	return indices, nil
}

// gltfPrimitiveAccessor 按属性名取图元访问器
// gltfPrimitiveAccessor gets a primitive accessor by attribute name
func gltfPrimitiveAccessor(document *gltf.Document, primitive *gltf.Primitive, name string) (*gltf.Accessor, bool, error) {
	accessorIndex, ok := primitive.Attributes[name]
	if !ok {
		return nil, false, nil
	}
	if accessorIndex < 0 || accessorIndex >= len(document.Accessors) {
		return nil, false, fmt.Errorf("attribute %s references accessor %d out of range", name, accessorIndex)
	}
	return document.Accessors[accessorIndex], true, nil
}

// appendGLTFPrimitiveVertices 读取一个图元的顶点属性并镜像追加到共享几何中，返回该图元的顶点数
// appendGLTFPrimitiveVertices reads one primitive's vertex attributes, mirrors them into the shared geometry, and returns the primitive's vertex count
func appendGLTFPrimitiveVertices(document *gltf.Document, primitive *gltf.Primitive, geometry *aba.MeshGeometry) (int64, error) {
	positionAccessor, hasPositions, err := gltfPrimitiveAccessor(document, primitive, gltf.POSITION)
	if err != nil {
		return 0, err
	}
	if !hasPositions {
		return 0, fmt.Errorf("primitive has no POSITION attribute")
	}
	positions, err := modeler.ReadPosition(document, positionAccessor, nil)
	if err != nil {
		return 0, fmt.Errorf("read POSITION: %w", err)
	}
	vertexCount := int64(len(positions))
	if vertexCount == 0 {
		return 0, fmt.Errorf("primitive POSITION accessor is empty")
	}
	for vertexIndex := range positions {
		geometry.Positions = append(geometry.Positions, [3]float32{-positions[vertexIndex][0], positions[vertexIndex][1], positions[vertexIndex][2]})
	}

	if accessor, ok, err := gltfPrimitiveAccessor(document, primitive, gltf.NORMAL); err != nil {
		return 0, err
	} else if ok {
		normals, err := modeler.ReadNormal(document, accessor, nil)
		if err != nil {
			return 0, fmt.Errorf("read NORMAL: %w", err)
		}
		if int64(len(normals)) != vertexCount {
			return 0, fmt.Errorf("NORMAL count %d does not match POSITION count %d", len(normals), vertexCount)
		}
		for vertexIndex := range normals {
			geometry.Normals = append(geometry.Normals, [3]float32{-normals[vertexIndex][0], normals[vertexIndex][1], normals[vertexIndex][2]})
		}
	}

	if accessor, ok, err := gltfPrimitiveAccessor(document, primitive, gltf.TANGENT); err != nil {
		return 0, err
	} else if ok {
		tangents, err := modeler.ReadTangent(document, accessor, nil)
		if err != nil {
			return 0, fmt.Errorf("read TANGENT: %w", err)
		}
		if int64(len(tangents)) != vertexCount {
			return 0, fmt.Errorf("TANGENT count %d does not match POSITION count %d", len(tangents), vertexCount)
		}
		for vertexIndex := range tangents {
			geometry.Tangents = append(geometry.Tangents, [4]float32{-tangents[vertexIndex][0], tangents[vertexIndex][1], tangents[vertexIndex][2], -tangents[vertexIndex][3]})
		}
	}

	for setIndex := int64(0); setIndex < aba.MeshTexCoordSetCount; setIndex++ {
		accessor, ok, err := gltfPrimitiveAccessor(document, primitive, fmt.Sprintf("TEXCOORD_%d", setIndex))
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		coords, err := modeler.ReadTextureCoord(document, accessor, nil)
		if err != nil {
			return 0, fmt.Errorf("read TEXCOORD_%d: %w", setIndex, err)
		}
		if int64(len(coords)) != vertexCount {
			return 0, fmt.Errorf("TEXCOORD_%d count %d does not match POSITION count %d", setIndex, len(coords), vertexCount)
		}
		for vertexIndex := range coords {
			geometry.TexCoords[setIndex] = append(geometry.TexCoords[setIndex], [2]float32{coords[vertexIndex][0], 1 - coords[vertexIndex][1]})
		}
	}

	if accessor, ok, err := gltfPrimitiveAccessor(document, primitive, gltf.COLOR_0); err != nil {
		return 0, err
	} else if ok {
		colors, err := readGLTFColors(document, accessor)
		if err != nil {
			return 0, err
		}
		if int64(len(colors)) != vertexCount {
			return 0, fmt.Errorf("COLOR_0 count %d does not match POSITION count %d", len(colors), vertexCount)
		}
		geometry.Colors = append(geometry.Colors, colors...)
	}

	return vertexCount, appendGLTFPrimitiveSkin(document, primitive, geometry, vertexCount)
}

// readGLTFColors 读取顶点颜色并把整数分量归一化为浮点
// readGLTFColors reads vertex colors and normalizes integer components to floats
func readGLTFColors(document *gltf.Document, accessor *gltf.Accessor) ([][4]float32, error) {
	raw, err := modeler.ReadAccessor(document, accessor, nil)
	if err != nil {
		return nil, fmt.Errorf("read COLOR_0: %w", err)
	}
	switch values := raw.(type) {
	case [][4]float32:
		return values, nil
	case [][3]float32:
		colors := make([][4]float32, len(values))
		for valueIndex := range values {
			colors[valueIndex] = [4]float32{values[valueIndex][0], values[valueIndex][1], values[valueIndex][2], 1}
		}
		return colors, nil
	case [][4]uint8:
		colors := make([][4]float32, len(values))
		for valueIndex := range values {
			for componentIndex := int64(0); componentIndex < 4; componentIndex++ {
				colors[valueIndex][componentIndex] = float32(values[valueIndex][componentIndex]) / 255
			}
		}
		return colors, nil
	case [][3]uint8:
		colors := make([][4]float32, len(values))
		for valueIndex := range values {
			colors[valueIndex] = [4]float32{float32(values[valueIndex][0]) / 255, float32(values[valueIndex][1]) / 255, float32(values[valueIndex][2]) / 255, 1}
		}
		return colors, nil
	case [][4]uint16:
		colors := make([][4]float32, len(values))
		for valueIndex := range values {
			for componentIndex := int64(0); componentIndex < 4; componentIndex++ {
				colors[valueIndex][componentIndex] = float32(values[valueIndex][componentIndex]) / 65535
			}
		}
		return colors, nil
	case [][3]uint16:
		colors := make([][4]float32, len(values))
		for valueIndex := range values {
			colors[valueIndex] = [4]float32{float32(values[valueIndex][0]) / 65535, float32(values[valueIndex][1]) / 65535, float32(values[valueIndex][2]) / 65535, 1}
		}
		return colors, nil
	default:
		return nil, fmt.Errorf("COLOR_0 accessor type %T is unsupported", raw)
	}
}

// appendGLTFPrimitiveSkin 读取图元的所有 JOINTS 和 WEIGHTS 集并追加为归一化降序的变长蒙皮
// appendGLTFPrimitiveSkin reads every JOINTS and WEIGHTS set of a primitive and appends normalized descending ragged skin data
func appendGLTFPrimitiveSkin(document *gltf.Document, primitive *gltf.Primitive, geometry *aba.MeshGeometry, vertexCount int64) error {
	type skinSet struct {
		joints  [][4]uint16
		weights [][4]float32
	}
	var sets []skinSet
	for setIndex := int64(0); ; setIndex++ {
		jointAccessor, hasJoints, err := gltfPrimitiveAccessor(document, primitive, fmt.Sprintf("JOINTS_%d", setIndex))
		if err != nil {
			return err
		}
		weightAccessor, hasWeights, err := gltfPrimitiveAccessor(document, primitive, fmt.Sprintf("WEIGHTS_%d", setIndex))
		if err != nil {
			return err
		}
		if !hasJoints && !hasWeights {
			break
		}
		if hasJoints != hasWeights {
			return fmt.Errorf("JOINTS_%d and WEIGHTS_%d must be provided together", setIndex, setIndex)
		}
		joints, err := modeler.ReadJoints(document, jointAccessor, nil)
		if err != nil {
			return fmt.Errorf("read JOINTS_%d: %w", setIndex, err)
		}
		weights, err := modeler.ReadWeights(document, weightAccessor, nil)
		if err != nil {
			return fmt.Errorf("read WEIGHTS_%d: %w", setIndex, err)
		}
		if int64(len(joints)) != vertexCount || int64(len(weights)) != vertexCount {
			return fmt.Errorf("JOINTS_%d and WEIGHTS_%d counts %d and %d do not match POSITION count %d", setIndex, setIndex, len(joints), len(weights), vertexCount)
		}
		sets = append(sets, skinSet{joints: joints, weights: weights})
	}
	if len(sets) == 0 {
		if geometry.SkinCounts != nil {
			return fmt.Errorf("primitive has no skin attributes but earlier primitives do")
		}
		return nil
	}
	if geometry.SkinCounts == nil && int64(len(geometry.Positions)) != vertexCount {
		return fmt.Errorf("primitive has skin attributes but earlier primitives do not")
	}

	type influence struct {
		bone   uint32
		weight float32
	}
	for vertexIndex := int64(0); vertexIndex < vertexCount; vertexIndex++ {
		var influences []influence
		var sum float64
		for _, set := range sets {
			for componentIndex := int64(0); componentIndex < 4; componentIndex++ {
				weight := set.weights[vertexIndex][componentIndex]
				if weight != weight || weight < 0 {
					return fmt.Errorf("vertex %d has invalid bone weight %f", vertexIndex, weight)
				}
				if weight == 0 {
					continue
				}
				influences = append(influences, influence{bone: uint32(set.joints[vertexIndex][componentIndex]), weight: weight})
				sum += float64(weight)
			}
		}
		if len(influences) == 0 || sum <= 0 {
			return fmt.Errorf("vertex %d has no bone influences; every vertex of a skinned mesh needs at least one weight", vertexIndex)
		}
		if len(influences) > math.MaxUint8 {
			return fmt.Errorf("vertex %d has %d bone influences", vertexIndex, len(influences))
		}
		sort.SliceStable(influences, func(left, right int) bool {
			return influences[left].weight > influences[right].weight
		})
		geometry.SkinCounts = append(geometry.SkinCounts, uint8(len(influences)))
		for _, entry := range influences {
			geometry.SkinIndices = append(geometry.SkinIndices, entry.bone)
			geometry.SkinWeights = append(geometry.SkinWeights, float32(float64(entry.weight)/sum))
		}
	}
	return nil
}

// decodeGLTFMorphTargets 将稠密变形目标稀疏化为 Model 的 BlendData 差分
// decodeGLTFMorphTargets sparsifies dense morph targets into the Model's BlendData deltas
func decodeGLTFMorphTargets(document *gltf.Document, mesh *gltf.Mesh, geometry *aba.MeshGeometry, primitivePools []*gltfPrimitivePool) ([]*serializationKCES.BlendData, error) {
	targetCount := len(mesh.Primitives[0].Targets)
	for primitiveIndex, primitive := range mesh.Primitives {
		if len(primitive.Targets) != targetCount {
			return nil, fmt.Errorf("primitive %d has %d morph targets while primitive 0 has %d", primitiveIndex, len(primitive.Targets), targetCount)
		}
	}
	if targetCount == 0 {
		return nil, nil
	}
	targetNames := gltfMorphTargetNames(mesh, targetCount)
	vertexCount := int64(len(geometry.Positions))

	morphs := make([]*serializationKCES.BlendData, targetCount)
	for targetIndex := 0; targetIndex < targetCount; targetIndex++ {
		deltaPositions := make([][3]float32, vertexCount)
		var deltaNormals, deltaTangents [][3]float32
		for primitiveIndex, primitive := range mesh.Primitives {
			pool := primitivePools[primitiveIndex]
			if !pool.First {
				continue
			}
			target := primitive.Targets[targetIndex]
			positions, err := readGLTFMorphAttribute(document, target, gltf.POSITION, pool.VertexCount)
			if err != nil {
				return nil, fmt.Errorf("morph target %d primitive %d: %w", targetIndex, primitiveIndex, err)
			}
			if positions == nil {
				return nil, fmt.Errorf("morph target %d primitive %d has no POSITION deltas", targetIndex, primitiveIndex)
			}
			copy(deltaPositions[pool.Base:], positions)
			normals, err := readGLTFMorphAttribute(document, target, gltf.NORMAL, pool.VertexCount)
			if err != nil {
				return nil, fmt.Errorf("morph target %d primitive %d: %w", targetIndex, primitiveIndex, err)
			}
			if normals != nil {
				if deltaNormals == nil {
					deltaNormals = make([][3]float32, vertexCount)
				}
				copy(deltaNormals[pool.Base:], normals)
			}
			tangents, err := readGLTFMorphAttribute(document, target, gltf.TANGENT, pool.VertexCount)
			if err != nil {
				return nil, fmt.Errorf("morph target %d primitive %d: %w", targetIndex, primitiveIndex, err)
			}
			if tangents != nil {
				if deltaTangents == nil {
					deltaTangents = make([][3]float32, vertexCount)
				}
				copy(deltaTangents[pool.Base:], tangents)
			}
		}

		name := targetNames[targetIndex]
		blend := &serializationKCES.BlendData{Name: &name}
		for vertexIndex := int64(0); vertexIndex < vertexCount; vertexIndex++ {
			position := deltaPositions[vertexIndex]
			var normal, tangent [3]float32
			if deltaNormals != nil {
				normal = deltaNormals[vertexIndex]
			}
			if deltaTangents != nil {
				tangent = deltaTangents[vertexIndex]
			}
			if position == ([3]float32{}) && normal == ([3]float32{}) && tangent == ([3]float32{}) {
				continue
			}
			blend.VIndex = append(blend.VIndex, int32(vertexIndex))
			blend.Vert = append(blend.Vert, serializationKCES.Vector3{X: -position[0], Y: position[1], Z: position[2]})
			if deltaNormals != nil {
				blend.Norm = append(blend.Norm, serializationKCES.Vector3{X: -normal[0], Y: normal[1], Z: normal[2]})
			}
			if deltaTangents != nil {
				blend.Tan = append(blend.Tan, serializationKCES.Vector4{X: -tangent[0], Y: tangent[1], Z: tangent[2], W: 0})
			}
		}
		morphs[targetIndex] = blend
	}
	return morphs, nil
}

// readGLTFMorphAttribute 读取一个变形目标属性的三维差分数组
// readGLTFMorphAttribute reads one morph-target attribute's three-component delta array
func readGLTFMorphAttribute(document *gltf.Document, target gltf.PrimitiveAttributes, name string, vertexCount int64) ([][3]float32, error) {
	accessorIndex, ok := target[name]
	if !ok {
		return nil, nil
	}
	if accessorIndex < 0 || accessorIndex >= len(document.Accessors) {
		return nil, fmt.Errorf("morph attribute %s references accessor %d out of range", name, accessorIndex)
	}
	raw, err := modeler.ReadAccessor(document, document.Accessors[accessorIndex], nil)
	if err != nil {
		return nil, fmt.Errorf("read morph attribute %s: %w", name, err)
	}
	values, ok := raw.([][3]float32)
	if !ok {
		return nil, fmt.Errorf("morph attribute %s type %T is unsupported", name, raw)
	}
	if int64(len(values)) != vertexCount {
		return nil, fmt.Errorf("morph attribute %s count %d does not match vertex count %d", name, len(values), vertexCount)
	}
	return values, nil
}

// gltfMorphTargetNames 从网格 extras 的 targetNames 惯例提取变形目标名称
// gltfMorphTargetNames extracts morph-target names from the targetNames convention in mesh extras
func gltfMorphTargetNames(mesh *gltf.Mesh, targetCount int) []string {
	names := make([]string, targetCount)
	for targetIndex := range names {
		names[targetIndex] = fmt.Sprintf("morph%d", targetIndex)
	}
	extrasMap, ok := mesh.Extras.(map[string]interface{})
	if !ok {
		return names
	}
	rawNames, ok := extrasMap["targetNames"].([]interface{})
	if !ok {
		return names
	}
	for nameIndex, rawName := range rawNames {
		if nameIndex >= targetCount {
			break
		}
		if name, ok := rawName.(string); ok && name != "" {
			names[nameIndex] = name
		}
	}
	return names
}
