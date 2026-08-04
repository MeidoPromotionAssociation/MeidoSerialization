package KCES

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

// KCESModelExtrasKey 是 glTF 文档 extras 中承载 KCES 专有模型字段的键名
// KCESModelExtrasKey is the key in glTF document extras that carries KCES-specific model fields
const KCESModelExtrasKey = "kcesModel"

// kcesModelExtras 保存 glTF 无法自然表达且反向转换需要还原的 Model 字段 / kcesModelExtras stores Model fields that glTF cannot express naturally and that the reverse conversion restores
type kcesModelExtras struct {
	Version           int32                            `json:"version"`                     // Model 版本号 / Model version value
	IndexedArrayWidth int32                            `json:"indexedArrayWidth,omitempty"` // 线格式数组宽度 / Wire array width
	FileName          string                           `json:"fileName,omitempty"`          // 模型文件名 / Model file name
	MeshFileName      string                           `json:"meshFileName,omitempty"`      // 网格文件名 / Mesh file name
	ModelName         string                           `json:"modelName,omitempty"`         // 网格挂载节点名 / Mesh attachment node name
	MaterialFileNames []string                         `json:"materialFileName,omitempty"`  // 材质文件名列表 / Material file names
	ShadowModeFlags   int32                            `json:"shadowModeFlags"`             // 阴影模式标志 / Shadow-mode flags
	SCLBones          []string                         `json:"sclBones,omitempty"`          // 标记为缩放骨骼的骨骼名 / Names of bones flagged as scale bones
	SkinThick         *serializationKCES.SkinThickness `json:"skinThick,omitempty"`         // 皮肤厚度数据 / Skin-thickness data
}

// IsKCESGLTFFile 判断路径是否为 glTF 或 GLB 文件
// IsKCESGLTFFile reports whether a path is a glTF or GLB file
func IsKCESGLTFFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".gltf") || strings.HasSuffix(lower, ".glb")
}

// ConvertModelToGLTF 将 .model 与其引用的 .mmesh 导出为带骨架、蒙皮、变形目标和材质名的完整 glTF 或 GLB
// KCES 专有字段保存在文档 extras 的 kcesModel 键下，供反向转换无损还原
// ConvertModelToGLTF exports a .model and its referenced .mmesh to complete glTF or GLB with the skeleton, skin, morph targets, and material names
// KCES-specific fields are stored under the kcesModel key in document extras so the reverse conversion can restore them losslessly
func (s *ModelService) ConvertModelToGLTF(ctx context.Context, inputPath string, outputPath string, format string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	model, err := serializationKCES.DecodeModel(data)
	if err != nil {
		return fmt.Errorf("decode Model %q: %w", inputPath, err)
	}
	if model == nil {
		return fmt.Errorf("Model %q is null", inputPath)
	}
	meshPath, err := locateModelMeshFile(inputPath, model)
	if err != nil {
		return err
	}
	meshData, err := os.ReadFile(meshPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", meshPath, err)
	}
	object, err := aba.ReadMMesh(meshData)
	if err != nil {
		return fmt.Errorf("read native Mesh %q: %w", meshPath, err)
	}
	geometry, err := object.DecodeMeshGeometry()
	if err != nil {
		return fmt.Errorf("decode native Mesh %q: %w", meshPath, err)
	}
	document, err := encodeModelGLTFDocument(model, geometry)
	if err != nil {
		return fmt.Errorf("convert Model %q to glTF: %w", inputPath, err)
	}
	format = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(outputPath)), ".")
	}
	if format != "gltf" && format != "glb" {
		return fmt.Errorf("Model glTF output format %q is unsupported; use gltf or glb", format)
	}
	var output bytes.Buffer
	encoder := gltf.NewEncoder(&output)
	encoder.AsBinary = format == "glb"
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("encode Model %q as %s: %w", inputPath, format, err)
	}
	return writeNativeUnityGLTFOutput(ctx, outputPath, output.Bytes(), maxOutputBytes)
}

// locateModelMeshFile 在模型同目录和 aba 解包目录布局中查找 meshFileName 指向的 .mmesh
// locateModelMeshFile finds the .mmesh referenced by meshFileName in the model directory and the unpacked aba directory layout
func locateModelMeshFile(modelPath string, model *serializationKCES.Model) (string, error) {
	if model.MeshFileName == nil || *model.MeshFileName == "" {
		return "", fmt.Errorf("Model %q has no meshFileName", modelPath)
	}
	directory := filepath.Dir(modelPath)
	candidates := []string{
		filepath.Join(directory, *model.MeshFileName),
		filepath.Join(directory, "..", "Mesh", *model.MeshFileName),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Mesh file %q referenced by %q was not found in %q or the sibling Mesh directory", *model.MeshFileName, modelPath, directory)
}

// encodeModelGLTFDocument 从 Model 骨架与 Mesh 几何构建 -X 镜像后的右手坐标 glTF 文档
// encodeModelGLTFDocument builds a right-handed glTF document mirrored along -X from the Model skeleton and Mesh geometry
func encodeModelGLTFDocument(model *serializationKCES.Model, geometry *aba.MeshGeometry) (*gltf.Document, error) {
	if len(model.TransData) == 0 {
		return nil, fmt.Errorf("Model has no transData skeleton")
	}
	if model.ModelName == nil || *model.ModelName == "" {
		return nil, fmt.Errorf("Model has no modelName")
	}
	document := gltf.NewDocument()
	document.Asset.Generator = "MeidoSerialization"

	nodeByName := make(map[string]int, len(model.TransData))
	for transIndex, transData := range model.TransData {
		if transData == nil || transData.Name == nil || *transData.Name == "" {
			return nil, fmt.Errorf("transData[%d] has no name", transIndex)
		}
		if _, exists := nodeByName[*transData.Name]; exists {
			return nil, fmt.Errorf("transData bone name %q is duplicated", *transData.Name)
		}
		node := &gltf.Node{
			Name:        *transData.Name,
			Translation: [3]float64{float64(-transData.Pos.X), float64(transData.Pos.Y), float64(transData.Pos.Z)},
			Rotation:    [4]float64{float64(transData.Rot.X), float64(-transData.Rot.Y), float64(-transData.Rot.Z), float64(transData.Rot.W)},
			Scale:       [3]float64{float64(transData.Scale.X), float64(transData.Scale.Y), float64(transData.Scale.Z)},
		}
		if node.Rotation == ([4]float64{}) {
			node.Rotation = gltf.DefaultRotation
		}
		if node.Scale == ([3]float64{}) {
			node.Scale = gltf.DefaultScale
		}
		nodeByName[*transData.Name] = len(document.Nodes)
		document.Nodes = append(document.Nodes, node)
	}
	document.Scenes[0].Nodes = nil
	for transIndex, transData := range model.TransData {
		parentNo := int64(transData.ParentNo)
		if parentNo >= 0 {
			if parentNo >= int64(len(model.TransData)) || parentNo == int64(transIndex) {
				return nil, fmt.Errorf("transData[%d] has invalid parent index %d", transIndex, parentNo)
			}
			parent := document.Nodes[parentNo]
			parent.Children = append(parent.Children, transIndex)
		} else {
			document.Scenes[0].Nodes = append(document.Scenes[0].Nodes, transIndex)
		}
	}
	if err := validateGLTFNodeTree(document, len(model.TransData)); err != nil {
		return nil, err
	}
	meshNodeIndex, ok := nodeByName[*model.ModelName]
	if !ok {
		return nil, fmt.Errorf("modelName %q does not match any transData bone", *model.ModelName)
	}

	skinned := geometry.SkinCounts != nil
	attributes, err := buildModelGLTFAttributes(document, geometry, skinned)
	if err != nil {
		return nil, err
	}
	targets, targetNames, err := buildModelGLTFMorphTargets(document, model, geometry)
	if err != nil {
		return nil, err
	}

	materialCount := int64(len(model.MaterialFileName))
	for materialIndex, materialName := range model.MaterialFileName {
		if materialName == nil || *materialName == "" {
			return nil, fmt.Errorf("materialFileName[%d] is empty", materialIndex)
		}
		document.Materials = append(document.Materials, &gltf.Material{
			Name:                 *materialName,
			PBRMetallicRoughness: &gltf.PBRMetallicRoughness{},
		})
	}

	primitives := make([]*gltf.Primitive, 0, len(geometry.Primitives))
	for primitiveIndex, source := range geometry.Primitives {
		if source.Mode != aba.MeshPrimitiveModeTriangles {
			return nil, fmt.Errorf("SubMesh %d topology is not triangles", primitiveIndex)
		}
		indices := append([]uint32(nil), source.Indices...)
		if len(indices)%3 != 0 {
			return nil, fmt.Errorf("SubMesh %d triangle index count %d is not divisible by three", primitiveIndex, len(indices))
		}
		for index := int64(0); index < int64(len(indices)); index += 3 {
			indices[index+1], indices[index+2] = indices[index+2], indices[index+1]
		}
		primitive := &gltf.Primitive{
			Attributes: attributes,
			Indices:    gltf.Index(modeler.WriteIndices(document, indices)),
			Mode:       gltf.PrimitiveTriangles,
			Targets:    targets,
		}
		if int64(primitiveIndex) < materialCount {
			primitive.Material = gltf.Index(primitiveIndex)
		}
		primitives = append(primitives, primitive)
	}

	mesh := &gltf.Mesh{Name: geometry.Name, Primitives: primitives}
	if len(targetNames) != 0 {
		mesh.Weights = make([]float64, len(targetNames))
		mesh.Extras = map[string]interface{}{"targetNames": targetNames}
	}
	document.Meshes = append(document.Meshes, mesh)
	document.Nodes[meshNodeIndex].Mesh = gltf.Index(0)

	if skinned {
		if len(model.BoneNames) == 0 {
			return nil, fmt.Errorf("Mesh is skinned but the Model has no boneNames")
		}
		if len(model.BoneNames) != len(geometry.BindPoses) {
			return nil, fmt.Errorf("boneNames count %d does not match bind pose count %d", len(model.BoneNames), len(geometry.BindPoses))
		}
		joints := make([]int, len(model.BoneNames))
		for boneIndex, boneName := range model.BoneNames {
			if boneName == nil || *boneName == "" {
				return nil, fmt.Errorf("boneNames[%d] is empty", boneIndex)
			}
			nodeIndex, ok := nodeByName[*boneName]
			if !ok {
				return nil, fmt.Errorf("boneNames[%d] %q does not match any transData bone", boneIndex, *boneName)
			}
			joints[boneIndex] = nodeIndex
		}
		inverseBindMatrices := make([][4][4]float32, len(geometry.BindPoses))
		for matrixIndex, matrix := range geometry.BindPoses {
			inverseBindMatrices[matrixIndex] = gltfMatrixFromUnity(mirrorUnityMatrixX(matrix))
		}
		document.Skins = append(document.Skins, &gltf.Skin{
			Name:                geometry.Name,
			Joints:              joints,
			InverseBindMatrices: gltf.Index(modeler.WriteAccessor(document, gltf.TargetNone, inverseBindMatrices)),
		})
		document.Nodes[meshNodeIndex].Skin = gltf.Index(0)
	}

	extras, err := buildKCESModelExtras(model)
	if err != nil {
		return nil, err
	}
	document.Extras = map[string]interface{}{KCESModelExtrasKey: extras}
	return document, nil
}

// validateGLTFNodeTree 验证节点从场景根出发恰好每个可达一次，防止环和多父节点
// validateGLTFNodeTree verifies every node is reachable exactly once from the scene roots, preventing cycles and multiple parents
func validateGLTFNodeTree(document *gltf.Document, nodeCount int) error {
	visited := make([]bool, nodeCount)
	var walk func(nodeIndex int) error
	walk = func(nodeIndex int) error {
		if nodeIndex < 0 || nodeIndex >= nodeCount {
			return fmt.Errorf("node index %d out of range", nodeIndex)
		}
		if visited[nodeIndex] {
			return fmt.Errorf("node %q is reachable through more than one path", document.Nodes[nodeIndex].Name)
		}
		visited[nodeIndex] = true
		for _, childIndex := range document.Nodes[nodeIndex].Children {
			if err := walk(childIndex); err != nil {
				return err
			}
		}
		return nil
	}
	for _, rootIndex := range document.Scenes[0].Nodes {
		if err := walk(rootIndex); err != nil {
			return err
		}
	}
	for nodeIndex := int64(0); nodeIndex < int64(nodeCount); nodeIndex++ {
		if !visited[nodeIndex] {
			return fmt.Errorf("node %q is not reachable from any root", document.Nodes[nodeIndex].Name)
		}
	}
	return nil
}

// buildModelGLTFAttributes 将镜像后的全部顶点属性写入文档并返回共享属性表
// buildModelGLTFAttributes writes every mirrored vertex attribute into the document and returns the shared attribute map
func buildModelGLTFAttributes(document *gltf.Document, geometry *aba.MeshGeometry, skinned bool) (gltf.PrimitiveAttributes, error) {
	vertexCount := int64(len(geometry.Positions))
	if vertexCount == 0 {
		return nil, fmt.Errorf("Mesh geometry has no positions")
	}
	positions := make([][3]float32, vertexCount)
	for vertexIndex, position := range geometry.Positions {
		positions[vertexIndex] = [3]float32{-position[0], position[1], position[2]}
	}
	attributes := gltf.PrimitiveAttributes{
		gltf.POSITION: modeler.WritePosition(document, positions),
	}
	if len(geometry.Normals) != 0 {
		normals := make([][3]float32, vertexCount)
		for vertexIndex, normal := range geometry.Normals {
			normals[vertexIndex] = [3]float32{-normal[0], normal[1], normal[2]}
		}
		attributes[gltf.NORMAL] = modeler.WriteNormal(document, normals)
	}
	if len(geometry.Tangents) != 0 {
		tangents := make([][4]float32, vertexCount)
		for vertexIndex, tangent := range geometry.Tangents {
			tangents[vertexIndex] = [4]float32{-tangent[0], tangent[1], tangent[2], -tangent[3]}
		}
		attributes[gltf.TANGENT] = modeler.WriteTangent(document, tangents)
	}
	for setIndex := int64(0); setIndex < aba.MeshTexCoordSetCount; setIndex++ {
		if len(geometry.TexCoords[setIndex]) == 0 {
			continue
		}
		coords := make([][2]float32, vertexCount)
		for vertexIndex, coordinate := range geometry.TexCoords[setIndex] {
			coords[vertexIndex] = [2]float32{coordinate[0], 1 - coordinate[1]}
		}
		attributes[fmt.Sprintf("TEXCOORD_%d", setIndex)] = modeler.WriteTextureCoord(document, coords)
	}
	if len(geometry.Colors) != 0 {
		attributes[gltf.COLOR_0] = modeler.WriteColor(document, append([][4]float32(nil), geometry.Colors...))
	}
	if skinned {
		maxCount := uint8(0)
		for _, count := range geometry.SkinCounts {
			if count > maxCount {
				maxCount = count
			}
		}
		setCount := (int64(maxCount) + 3) / 4
		joints := make([][][4]uint16, setCount)
		weights := make([][][4]float32, setCount)
		for setIndex := int64(0); setIndex < setCount; setIndex++ {
			joints[setIndex] = make([][4]uint16, vertexCount)
			weights[setIndex] = make([][4]float32, vertexCount)
		}
		cursor := int64(0)
		for vertexIndex, count := range geometry.SkinCounts {
			for entryIndex := int64(0); entryIndex < int64(count); entryIndex++ {
				boneIndex := geometry.SkinIndices[cursor+entryIndex]
				if boneIndex > math.MaxUint16 {
					return nil, fmt.Errorf("vertex %d bone index %d exceeds the glTF UInt16 joint range", vertexIndex, boneIndex)
				}
				setIndex, componentIndex := entryIndex/4, entryIndex%4
				joints[setIndex][vertexIndex][componentIndex] = uint16(boneIndex)
				weights[setIndex][vertexIndex][componentIndex] = geometry.SkinWeights[cursor+entryIndex]
			}
			cursor += int64(count)
		}
		for setIndex := int64(0); setIndex < setCount; setIndex++ {
			attributes[fmt.Sprintf("JOINTS_%d", setIndex)] = modeler.WriteJoints(document, joints[setIndex])
			attributes[fmt.Sprintf("WEIGHTS_%d", setIndex)] = modeler.WriteWeights(document, weights[setIndex])
		}
	}
	return attributes, nil
}

// buildModelGLTFMorphTargets 将 Model 的稀疏 morph 差分展开为稠密 glTF 变形目标
// 切线差分只保留前三个分量，因为 glTF 变形目标的 TANGENT 是三维向量
// buildModelGLTFMorphTargets expands the Model's sparse morph deltas into dense glTF morph targets
// Tangent deltas keep only their first three components because glTF morph-target TANGENT is a three-component vector
func buildModelGLTFMorphTargets(document *gltf.Document, model *serializationKCES.Model, geometry *aba.MeshGeometry) ([]gltf.PrimitiveAttributes, []string, error) {
	if len(model.Morphs) == 0 {
		return nil, nil, nil
	}
	vertexCount := int64(len(geometry.Positions))
	targets := make([]gltf.PrimitiveAttributes, 0, len(model.Morphs))
	names := make([]string, 0, len(model.Morphs))
	for morphIndex, morph := range model.Morphs {
		if morph == nil {
			return nil, nil, fmt.Errorf("morphs[%d] is null", morphIndex)
		}
		name := ""
		if morph.Name != nil {
			name = *morph.Name
		}
		if len(morph.Vert) != len(morph.VIndex) {
			return nil, nil, fmt.Errorf("morph %q has %d vertex deltas for %d indices", name, len(morph.Vert), len(morph.VIndex))
		}
		hasNormals := len(morph.Norm) != 0
		hasTangents := len(morph.Tan) != 0
		if hasNormals && len(morph.Norm) != len(morph.VIndex) {
			return nil, nil, fmt.Errorf("morph %q has %d normal deltas for %d indices", name, len(morph.Norm), len(morph.VIndex))
		}
		if hasTangents && len(morph.Tan) != len(morph.VIndex) {
			return nil, nil, fmt.Errorf("morph %q has %d tangent deltas for %d indices", name, len(morph.Tan), len(morph.VIndex))
		}
		deltaPositions := make([][3]float32, vertexCount)
		var deltaNormals, deltaTangents [][3]float32
		if hasNormals {
			deltaNormals = make([][3]float32, vertexCount)
		}
		if hasTangents {
			deltaTangents = make([][3]float32, vertexCount)
		}
		for entryIndex, vertexIndex := range morph.VIndex {
			if int64(vertexIndex) < 0 || int64(vertexIndex) >= vertexCount {
				return nil, nil, fmt.Errorf("morph %q targets vertex %d outside %d vertices", name, vertexIndex, vertexCount)
			}
			deltaPositions[vertexIndex] = [3]float32{-morph.Vert[entryIndex].X, morph.Vert[entryIndex].Y, morph.Vert[entryIndex].Z}
			if hasNormals {
				deltaNormals[vertexIndex] = [3]float32{-morph.Norm[entryIndex].X, morph.Norm[entryIndex].Y, morph.Norm[entryIndex].Z}
			}
			if hasTangents {
				deltaTangents[vertexIndex] = [3]float32{-morph.Tan[entryIndex].X, morph.Tan[entryIndex].Y, morph.Tan[entryIndex].Z}
			}
		}
		target := gltf.PrimitiveAttributes{
			gltf.POSITION: writeMorphPositionAccessor(document, deltaPositions),
		}
		if hasNormals {
			target[gltf.NORMAL] = modeler.WriteAccessor(document, gltf.TargetArrayBuffer, deltaNormals)
		}
		if hasTangents {
			target[gltf.TANGENT] = modeler.WriteAccessor(document, gltf.TargetArrayBuffer, deltaTangents)
		}
		targets = append(targets, target)
		names = append(names, name)
	}
	return targets, names, nil
}

// writeMorphPositionAccessor 写入变形目标位置差分访问器并按规范补充 min 和 max 边界
// writeMorphPositionAccessor writes a morph-target position delta accessor with the min and max bounds the specification requires
func writeMorphPositionAccessor(document *gltf.Document, deltas [][3]float32) int {
	accessorIndex := modeler.WriteAccessor(document, gltf.TargetArrayBuffer, deltas)
	minBounds := []float64{0, 0, 0}
	maxBounds := []float64{0, 0, 0}
	for deltaIndex, delta := range deltas {
		for axis := int64(0); axis < 3; axis++ {
			value := float64(delta[axis])
			if deltaIndex == 0 || value < minBounds[axis] {
				minBounds[axis] = value
			}
			if deltaIndex == 0 || value > maxBounds[axis] {
				maxBounds[axis] = value
			}
		}
	}
	document.Accessors[accessorIndex].Min = minBounds
	document.Accessors[accessorIndex].Max = maxBounds
	return accessorIndex
}

// buildKCESModelExtras 收集需要通过 extras 保真的 Model 字段
// buildKCESModelExtras collects the Model fields preserved through extras
func buildKCESModelExtras(model *serializationKCES.Model) (*kcesModelExtras, error) {
	extras := &kcesModelExtras{
		Version:           model.Version,
		IndexedArrayWidth: model.IndexedArrayWidth,
		ShadowModeFlags:   model.ShadowModeFlags,
		SkinThick:         model.SkinThick,
	}
	if model.FileName != nil {
		extras.FileName = *model.FileName
	}
	if model.MeshFileName != nil {
		extras.MeshFileName = *model.MeshFileName
	}
	if model.ModelName != nil {
		extras.ModelName = *model.ModelName
	}
	for materialIndex, materialName := range model.MaterialFileName {
		if materialName == nil {
			return nil, fmt.Errorf("materialFileName[%d] is null", materialIndex)
		}
		extras.MaterialFileNames = append(extras.MaterialFileNames, *materialName)
	}
	for _, transData := range model.TransData {
		if transData != nil && transData.IsSCL && transData.Name != nil {
			extras.SCLBones = append(extras.SCLBones, *transData.Name)
		}
	}
	return extras, nil
}

// mirrorUnityMatrixX 用 diag(-1,1,1,1) 相似变换将行主序矩阵在 X 轴镜像
// mirrorUnityMatrixX mirrors a row-major matrix along the X axis with a diag(-1,1,1,1) similarity transform
func mirrorUnityMatrixX(matrix [16]float32) [16]float32 {
	signs := [4]float32{-1, 1, 1, 1}
	var result [16]float32
	for row := int64(0); row < 4; row++ {
		for column := int64(0); column < 4; column++ {
			result[row*4+column] = signs[row] * matrix[row*4+column] * signs[column]
		}
	}
	return result
}

// gltfMatrixFromUnity 将 e00 到 e33 行主序矩阵转换为 qmuntal 按行列下标存储的 glTF 矩阵
// gltfMatrixFromUnity converts an e00 through e33 row-major matrix to the row-column indexed glTF matrix used by qmuntal
func gltfMatrixFromUnity(matrix [16]float32) [4][4]float32 {
	var result [4][4]float32
	for row := int64(0); row < 4; row++ {
		for column := int64(0); column < 4; column++ {
			result[row][column] = matrix[row*4+column]
		}
	}
	return result
}

// unityMatrixFromGLTF 将 qmuntal 按行列下标存储的 glTF 矩阵转换回 e00 到 e33 行主序表示
// unityMatrixFromGLTF converts a row-column indexed qmuntal glTF matrix back to the e00 through e33 row-major representation
func unityMatrixFromGLTF(matrix [4][4]float32) [16]float32 {
	var result [16]float32
	for row := int64(0); row < 4; row++ {
		for column := int64(0); column < 4; column++ {
			result[row*4+column] = matrix[row][column]
		}
	}
	return result
}
