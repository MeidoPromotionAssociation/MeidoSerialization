package KCES

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

// ConvertMeshToGLB 将独立 Mesh 主文件导出为包含全部 SubMesh 的 glTF Binary 文件
// ConvertMeshToGLB exports a standalone Mesh primary file to a glTF Binary file containing every SubMesh
func (s *NativeUnityMediaService) ConvertMeshToGLB(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	return s.ConvertMeshToGLTF(ctx, inputPath, outputPath, "glb", maxOutputBytes)
}

// ConvertMeshToGLTF 将独立 Mesh 主文件导出为包含全部 SubMesh 的 glTF 或 GLB 文件
// ConvertMeshToGLTF exports a standalone Mesh primary file to a glTF or GLB file containing every SubMesh
func (s *NativeUnityMediaService) ConvertMeshToGLTF(ctx context.Context, inputPath string, outputPath string, format string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	object, err := aba.ReadMMesh(data)
	if err != nil {
		return fmt.Errorf("read native Mesh %q: %w", inputPath, err)
	}
	geometry, err := object.DecodeMeshGeometry()
	if err != nil {
		return fmt.Errorf("decode native Mesh %q: %w", inputPath, err)
	}
	format = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(outputPath)), ".")
	}
	if format != "gltf" && format != "glb" {
		return fmt.Errorf("native Mesh output format %q is unsupported; use gltf or glb", format)
	}
	output, err := encodeMeshGLTF(geometry, format == "glb")
	if err != nil {
		return fmt.Errorf("encode native Mesh %q as %s: %w", inputPath, format, err)
	}
	return writeNativeUnityGLTFOutput(ctx, outputPath, output, maxOutputBytes)
}

// encodeMeshGLB 将 Unity 左手坐标几何转换到 glTF 右手坐标并编码为 GLB
// encodeMeshGLB converts Unity left-handed geometry to glTF right-handed coordinates and encodes it as GLB
func encodeMeshGLB(geometry *aba.MeshGeometry) ([]byte, error) {
	return encodeMeshGLTF(geometry, true)
}

// encodeMeshGLTF 将 Unity 左手坐标几何转换到 glTF 右手坐标并按指定容器编码
// encodeMeshGLTF converts Unity left-handed geometry to glTF right-handed coordinates and encodes it in the requested container
func encodeMeshGLTF(geometry *aba.MeshGeometry, binaryOutput bool) ([]byte, error) {
	if geometry == nil || len(geometry.Positions) == 0 {
		return nil, fmt.Errorf("Mesh geometry has no positions")
	}
	document := gltf.NewDocument()
	document.Asset.Generator = "MeidoSerialization"

	positions := append([][3]float32(nil), geometry.Positions...)
	for positionIndex := range positions {
		positions[positionIndex][0] = -positions[positionIndex][0]
	}
	attributes := gltf.PrimitiveAttributes{
		gltf.POSITION: modeler.WritePosition(document, positions),
	}
	if len(geometry.Normals) != 0 {
		if len(geometry.Normals) != len(positions) {
			return nil, fmt.Errorf("normal count %d does not match position count %d", len(geometry.Normals), len(positions))
		}
		normals := append([][3]float32(nil), geometry.Normals...)
		for normalIndex := range normals {
			normals[normalIndex][0] = -normals[normalIndex][0]
		}
		attributes[gltf.NORMAL] = modeler.WriteNormal(document, normals)
	}
	if len(geometry.Tangents) != 0 {
		if len(geometry.Tangents) != len(positions) {
			return nil, fmt.Errorf("tangent count %d does not match position count %d", len(geometry.Tangents), len(positions))
		}
		tangents := append([][4]float32(nil), geometry.Tangents...)
		for tangentIndex := range tangents {
			tangents[tangentIndex][0] = -tangents[tangentIndex][0]
			tangents[tangentIndex][3] = -tangents[tangentIndex][3]
		}
		attributes[gltf.TANGENT] = modeler.WriteTangent(document, tangents)
	}
	if len(geometry.TexCoord0) != 0 {
		if len(geometry.TexCoord0) != len(positions) {
			return nil, fmt.Errorf("texture-coordinate count %d does not match position count %d", len(geometry.TexCoord0), len(positions))
		}
		texCoord0 := append([][2]float32(nil), geometry.TexCoord0...)
		for coordinateIndex := range texCoord0 {
			texCoord0[coordinateIndex][1] = 1 - texCoord0[coordinateIndex][1]
		}
		attributes[gltf.TEXCOORD_0] = modeler.WriteTextureCoord(document, texCoord0)
	}
	if len(geometry.Colors) != 0 {
		if len(geometry.Colors) != len(positions) {
			return nil, fmt.Errorf("color count %d does not match position count %d", len(geometry.Colors), len(positions))
		}
		attributes[gltf.COLOR_0] = modeler.WriteColor(document, geometry.Colors)
	}

	primitives := make([]*gltf.Primitive, 0, len(geometry.Primitives))
	for primitiveIndex, source := range geometry.Primitives {
		if len(source.Indices) == 0 {
			continue
		}
		indices := append([]uint32(nil), source.Indices...)
		if source.Mode == aba.MeshPrimitiveModeTriangles {
			if len(indices)%3 != 0 {
				return nil, fmt.Errorf("primitive %d triangle index count %d is not divisible by three", primitiveIndex, len(indices))
			}
			for index := int64(0); index < int64(len(indices)); index += 3 {
				indices[index+1], indices[index+2] = indices[index+2], indices[index+1]
			}
		}
		indexAccessor := modeler.WriteIndices(document, indices)
		mode, err := gltfPrimitiveMode(source.Mode)
		if err != nil {
			return nil, fmt.Errorf("primitive %d: %w", primitiveIndex, err)
		}
		primitives = append(primitives, &gltf.Primitive{
			Attributes: attributes,
			Indices:    gltf.Index(indexAccessor),
			Mode:       mode,
		})
	}
	if len(primitives) == 0 {
		return nil, fmt.Errorf("Mesh geometry has no indexed primitives")
	}
	document.Meshes = []*gltf.Mesh{{Name: geometry.Name, Primitives: primitives}}
	document.Nodes = []*gltf.Node{{Name: geometry.Name, Mesh: gltf.Index(0)}}
	document.Scenes[0].Nodes = []int{0}

	var output bytes.Buffer
	encoder := gltf.NewEncoder(&output)
	encoder.AsBinary = binaryOutput
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// gltfPrimitiveMode 将本库使用的 glTF 线格式拓扑值映射到 qmuntal/gltf 的内部枚举
// gltfPrimitiveMode maps this library's glTF wire topology values to qmuntal/gltf's internal enumeration
func gltfPrimitiveMode(mode aba.MeshPrimitiveMode) (gltf.PrimitiveMode, error) {
	switch mode {
	case aba.MeshPrimitiveModePoints:
		return gltf.PrimitivePoints, nil
	case aba.MeshPrimitiveModeLines:
		return gltf.PrimitiveLines, nil
	case aba.MeshPrimitiveModeLineStrip:
		return gltf.PrimitiveLineStrip, nil
	case aba.MeshPrimitiveModeTriangles:
		return gltf.PrimitiveTriangles, nil
	default:
		return 0, fmt.Errorf("unsupported primitive mode %d", mode)
	}
}

// ConvertAnimationClipToGLTF 将带显式 Transform 路径的独立 AnimationClip 导出为 glTF 或 GLB 动画
// ConvertAnimationClipToGLTF exports a standalone AnimationClip with explicit Transform paths as a glTF or GLB animation
func (s *NativeUnityMediaService) ConvertAnimationClipToGLTF(ctx context.Context, inputPath string, outputPath string, format string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	object, err := aba.ReadAnimationClip(data)
	if err != nil {
		return fmt.Errorf("read native AnimationClip %q: %w", inputPath, err)
	}
	curves, err := object.DecodeAnimationClipCurves()
	if err != nil {
		return fmt.Errorf("decode native AnimationClip %q: %w", inputPath, err)
	}
	format = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(outputPath)), ".")
	}
	if format != "gltf" && format != "glb" {
		return fmt.Errorf("native AnimationClip output format %q is unsupported; use gltf or glb", format)
	}
	output, err := encodeAnimationGLTF(curves, format == "glb")
	if err != nil {
		return fmt.Errorf("encode native AnimationClip %q as %s: %w", inputPath, format, err)
	}
	return writeNativeUnityGLTFOutput(ctx, outputPath, output, maxOutputBytes)
}

// writeNativeUnityGLTFOutput 在上下文有效且大小不超限时写入 glTF 转换结果
// writeNativeUnityGLTFOutput writes glTF conversion output while the context is active and the size remains within the limit
func writeNativeUnityGLTFOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive glTF conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: glTF conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write glTF conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}

// encodeAnimationGLTF 将 Unity 显式 TRS 曲线转换到 glTF 节点层次并编码
// encodeAnimationGLTF converts explicit Unity TRS curves to a glTF node hierarchy and encodes the document
func encodeAnimationGLTF(clip *aba.AnimationClipCurves, binaryOutput bool) ([]byte, error) {
	if clip == nil || len(clip.Curves) == 0 {
		return nil, fmt.Errorf("AnimationClip has no exportable curves")
	}
	document := gltf.NewDocument()
	document.Asset.Generator = "MeidoSerialization"
	nodeByPath := make(map[string]int)
	animation := &gltf.Animation{Name: clip.Name}
	for curveIndex, curve := range clip.Curves {
		if len(curve.Keyframes) == 0 {
			continue
		}
		nodeIndex, err := ensureGLTFAnimationNode(document, nodeByPath, curve.Path, clip.Name)
		if err != nil {
			return nil, fmt.Errorf("curve %d path %q: %w", curveIndex, curve.Path, err)
		}
		times := make([]float32, len(curve.Keyframes))
		for keyIndex, keyframe := range curve.Keyframes {
			times[keyIndex] = keyframe.Time
		}
		inputAccessor := modeler.WriteAccessor(document, gltf.TargetNone, times)
		document.Accessors[inputAccessor].Min = []float64{float64(times[0])}
		document.Accessors[inputAccessor].Max = []float64{float64(times[len(times)-1])}

		var outputAccessor int
		var targetPath gltf.TRSProperty
		switch curve.Property {
		case aba.AnimationCurveTranslation:
			values := make([][3]float32, len(curve.Keyframes))
			for keyIndex, keyframe := range curve.Keyframes {
				values[keyIndex] = [3]float32{-keyframe.Value[0], keyframe.Value[1], keyframe.Value[2]}
			}
			outputAccessor = modeler.WriteAccessor(document, gltf.TargetNone, values)
			targetPath = gltf.TRSTranslation
		case aba.AnimationCurveScale:
			values := make([][3]float32, len(curve.Keyframes))
			for keyIndex, keyframe := range curve.Keyframes {
				values[keyIndex] = [3]float32{keyframe.Value[0], keyframe.Value[1], keyframe.Value[2]}
			}
			outputAccessor = modeler.WriteAccessor(document, gltf.TargetNone, values)
			targetPath = gltf.TRSScale
		case aba.AnimationCurveRotation, aba.AnimationCurveEuler:
			values := make([][4]float32, len(curve.Keyframes))
			for keyIndex, keyframe := range curve.Keyframes {
				quaternion := keyframe.Value
				if curve.Property == aba.AnimationCurveEuler {
					quaternion = unityEulerQuaternion(keyframe.Value[0], keyframe.Value[1], keyframe.Value[2])
				}
				quaternion = [4]float32{quaternion[0], -quaternion[1], -quaternion[2], quaternion[3]}
				quaternion, err = normalizeGLTFQuaternion(quaternion)
				if err != nil {
					return nil, fmt.Errorf("curve %d keyframe %d: %w", curveIndex, keyIndex, err)
				}
				if keyIndex != 0 && quaternionDot(values[keyIndex-1], quaternion) < 0 {
					for componentIndex := int64(0); componentIndex < 4; componentIndex++ {
						quaternion[componentIndex] = -quaternion[componentIndex]
					}
				}
				values[keyIndex] = quaternion
			}
			outputAccessor = modeler.WriteAccessor(document, gltf.TargetNone, values)
			targetPath = gltf.TRSRotation
		default:
			return nil, fmt.Errorf("curve %d has unsupported property %d", curveIndex, curve.Property)
		}
		samplerIndex := len(animation.Samplers)
		animation.Samplers = append(animation.Samplers, &gltf.AnimationSampler{
			Input:         inputAccessor,
			Interpolation: gltf.InterpolationLinear,
			Output:        outputAccessor,
		})
		animation.Channels = append(animation.Channels, &gltf.AnimationChannel{
			Sampler: samplerIndex,
			Target: gltf.AnimationChannelTarget{
				Node: gltf.Index(nodeIndex),
				Path: targetPath,
			},
		})
	}
	if len(animation.Channels) == 0 {
		return nil, fmt.Errorf("AnimationClip produced no glTF channels")
	}
	document.Animations = []*gltf.Animation{animation}
	var output bytes.Buffer
	encoder := gltf.NewEncoder(&output)
	encoder.AsBinary = binaryOutput
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// ensureGLTFAnimationNode 为 Transform 路径建立缺失的 glTF 节点并返回叶节点索引
// ensureGLTFAnimationNode creates missing glTF nodes for a Transform path and returns the leaf node index
func ensureGLTFAnimationNode(document *gltf.Document, nodeByPath map[string]int, transformPath string, fallbackName string) (int, error) {
	if document == nil {
		return 0, fmt.Errorf("nil glTF document")
	}
	transformPath = strings.Trim(strings.ReplaceAll(transformPath, "\\", "/"), "/")
	if transformPath == "" {
		if existing, ok := nodeByPath[""]; ok {
			return existing, nil
		}
		name := fallbackName
		if name == "" {
			name = "AnimationRoot"
		}
		nodeIndex := len(document.Nodes)
		document.Nodes = append(document.Nodes, &gltf.Node{Name: name})
		document.Scenes[0].Nodes = append(document.Scenes[0].Nodes, nodeIndex)
		nodeByPath[""] = nodeIndex
		return nodeIndex, nil
	}
	parts := strings.Split(transformPath, "/")
	currentPath := ""
	parentIndex := -1
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.IndexByte(part, 0) >= 0 {
			return 0, fmt.Errorf("invalid Transform path component %q", part)
		}
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath += "/" + part
		}
		if existing, ok := nodeByPath[currentPath]; ok {
			parentIndex = existing
			continue
		}
		nodeIndex := len(document.Nodes)
		document.Nodes = append(document.Nodes, &gltf.Node{Name: part})
		nodeByPath[currentPath] = nodeIndex
		if parentIndex >= 0 {
			document.Nodes[parentIndex].Children = append(document.Nodes[parentIndex].Children, nodeIndex)
		} else {
			document.Scenes[0].Nodes = append(document.Scenes[0].Nodes, nodeIndex)
		}
		parentIndex = nodeIndex
	}
	return parentIndex, nil
}

// unityEulerQuaternion 按 Unity 的 ZXY 欧拉角应用顺序生成四元数
// unityEulerQuaternion creates a quaternion using Unity's ZXY Euler-angle application order
func unityEulerQuaternion(xDegrees float32, yDegrees float32, zDegrees float32) [4]float32 {
	const degreesToHalfRadians = math.Pi / 360
	x := float64(xDegrees) * degreesToHalfRadians
	y := float64(yDegrees) * degreesToHalfRadians
	z := float64(zDegrees) * degreesToHalfRadians
	qx := [4]float64{math.Sin(x), 0, 0, math.Cos(x)}
	qy := [4]float64{0, math.Sin(y), 0, math.Cos(y)}
	qz := [4]float64{0, 0, math.Sin(z), math.Cos(z)}
	result := multiplyQuaternion(multiplyQuaternion(qy, qx), qz)
	return [4]float32{float32(result[0]), float32(result[1]), float32(result[2]), float32(result[3])}
}

// multiplyQuaternion 将两个 XYZW 四元数相乘
// multiplyQuaternion multiplies two XYZW quaternions
func multiplyQuaternion(left [4]float64, right [4]float64) [4]float64 {
	return [4]float64{
		left[3]*right[0] + left[0]*right[3] + left[1]*right[2] - left[2]*right[1],
		left[3]*right[1] - left[0]*right[2] + left[1]*right[3] + left[2]*right[0],
		left[3]*right[2] + left[0]*right[1] - left[1]*right[0] + left[2]*right[3],
		left[3]*right[3] - left[0]*right[0] - left[1]*right[1] - left[2]*right[2],
	}
}

// normalizeGLTFQuaternion 归一化四元数并拒绝零长度或非有限值
// normalizeGLTFQuaternion normalizes a quaternion and rejects zero-length or non-finite values
func normalizeGLTFQuaternion(value [4]float32) ([4]float32, error) {
	lengthSquared := float64(value[0])*float64(value[0]) + float64(value[1])*float64(value[1]) + float64(value[2])*float64(value[2]) + float64(value[3])*float64(value[3])
	if lengthSquared <= 0 || math.IsNaN(lengthSquared) || math.IsInf(lengthSquared, 0) {
		return [4]float32{}, fmt.Errorf("rotation quaternion has invalid length")
	}
	inverseLength := float32(1 / math.Sqrt(lengthSquared))
	for componentIndex := int64(0); componentIndex < 4; componentIndex++ {
		value[componentIndex] *= inverseLength
	}
	return value, nil
}

// quaternionDot 返回两个 XYZW 四元数的点积
// quaternionDot returns the dot product of two XYZW quaternions
func quaternionDot(left [4]float32, right [4]float32) float32 {
	return left[0]*right[0] + left[1]*right[1] + left[2]*right[2] + left[3]*right[3]
}
