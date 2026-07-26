package aba

import (
	"fmt"
	"strings"
)

// AnimationCurveProperty 表示可映射到 glTF 节点 TRS 的 Unity 曲线属性 / AnimationCurveProperty represents a Unity curve property that maps to glTF node TRS
type AnimationCurveProperty uint8

const (
	// AnimationCurveTranslation 表示局部位置曲线 / AnimationCurveTranslation represents a local-position curve
	AnimationCurveTranslation AnimationCurveProperty = iota
	// AnimationCurveRotation 表示局部四元数旋转曲线 / AnimationCurveRotation represents a local quaternion-rotation curve
	AnimationCurveRotation
	// AnimationCurveScale 表示局部缩放曲线 / AnimationCurveScale represents a local-scale curve
	AnimationCurveScale
	// AnimationCurveEuler 表示需要转换为四元数的局部欧拉角曲线 / AnimationCurveEuler represents a local Euler-angle curve that requires quaternion conversion
	AnimationCurveEuler
)

// AnimationTRSKeyframe 表示一个固定宽度的 Unity TRS 关键帧 / AnimationTRSKeyframe represents one fixed-width Unity TRS keyframe
type AnimationTRSKeyframe struct {
	Time  float32    // 关键帧时间，单位为秒 / Keyframe time in seconds
	Value [4]float32 // XYZ 或 XYZW 值 / XYZ or XYZW value
}

// AnimationTRSCurve 表示绑定到一个 Transform 路径的显式 TRS 曲线 / AnimationTRSCurve represents an explicit TRS curve bound to one Transform path
type AnimationTRSCurve struct {
	Path      string                 // 相对于动画根节点的 Transform 路径 / Transform path relative to the animation root
	Property  AnimationCurveProperty // TRS 属性 / TRS property
	Dimension uint8                  // 值分量数量 / Value component count
	Keyframes []AnimationTRSKeyframe // 有序关键帧 / Ordered keyframes
}

// AnimationClipCurves 表示从 Unity AnimationClip 中提取的显式节点曲线 / AnimationClipCurves represents explicit node curves extracted from a Unity AnimationClip
type AnimationClipCurves struct {
	Name   string              // Unity m_Name / Unity m_Name
	Curves []AnimationTRSCurve // 可导出的 TRS 曲线 / Exportable TRS curves
}

// DecodeAnimationClipCurves 使用内嵌 TypeTree 提取带明文 Transform 路径的显式 TRS 曲线
// DecodeAnimationClipCurves extracts explicit TRS curves with clear-text Transform paths using the embedded TypeTree
func (object *NativeUnityObject) DecodeAnimationClipCurves() (*AnimationClipCurves, error) {
	if object == nil || object.ClassID != ClassIDAnimationClip {
		return nil, fmt.Errorf("native Unity object is not an AnimationClip")
	}
	root, err := object.DecodeValue()
	if err != nil {
		return nil, fmt.Errorf("decode AnimationClip TypeTree: %w", err)
	}
	result := &AnimationClipCurves{}
	result.Name, _ = root.Field("m_Name").String()
	rotationPaths := make(map[string]struct{})
	rotationCurves, err := decodeAnimationCurveVector(root.Field("m_RotationCurves"), AnimationCurveRotation, 4)
	if err != nil {
		return nil, fmt.Errorf("decode AnimationClip rotation curves: %w", err)
	}
	for _, curve := range rotationCurves {
		rotationPaths[curve.Path] = struct{}{}
	}
	result.Curves = append(result.Curves, rotationCurves...)
	positionCurves, err := decodeAnimationCurveVector(root.Field("m_PositionCurves"), AnimationCurveTranslation, 3)
	if err != nil {
		return nil, fmt.Errorf("decode AnimationClip position curves: %w", err)
	}
	result.Curves = append(result.Curves, positionCurves...)
	scaleCurves, err := decodeAnimationCurveVector(root.Field("m_ScaleCurves"), AnimationCurveScale, 3)
	if err != nil {
		return nil, fmt.Errorf("decode AnimationClip scale curves: %w", err)
	}
	result.Curves = append(result.Curves, scaleCurves...)
	eulerCurves, err := decodeAnimationCurveVector(root.Field("m_EulerCurves"), AnimationCurveEuler, 3)
	if err != nil {
		return nil, fmt.Errorf("decode AnimationClip Euler curves: %w", err)
	}
	for _, curve := range eulerCurves {
		if _, hasQuaternionCurve := rotationPaths[curve.Path]; !hasQuaternionCurve {
			result.Curves = append(result.Curves, curve)
		}
	}
	if len(result.Curves) == 0 {
		return nil, fmt.Errorf("AnimationClip has no explicit TRS curves with Transform paths; use TypeTree JSON for optimized or hashed curves")
	}
	return result, nil
}

// decodeAnimationCurveVector 解码一个 Unity AnimationClip 曲线向量
// decodeAnimationCurveVector decodes one Unity AnimationClip curve vector
func decodeAnimationCurveVector(vector *TypeTreeValue, property AnimationCurveProperty, dimension uint8) ([]AnimationTRSCurve, error) {
	if vector == nil || len(vector.Children) == 0 {
		return nil, nil
	}
	curves := make([]AnimationTRSCurve, 0, len(vector.Children))
	for curveIndex, entry := range vector.Children {
		path, ok := entry.Field("path").String()
		if !ok || strings.IndexByte(path, 0) >= 0 {
			return nil, fmt.Errorf("curve %d has an invalid path", curveIndex)
		}
		curveValue := entry.Field("curve")
		if curveValue == nil {
			return nil, fmt.Errorf("curve %d has no curve field", curveIndex)
		}
		keys := curveValue.Field("m_Curve")
		if keys == nil {
			return nil, fmt.Errorf("curve %d has no m_Curve field", curveIndex)
		}
		curve := AnimationTRSCurve{Path: path, Property: property, Dimension: dimension}
		curve.Keyframes = make([]AnimationTRSKeyframe, 0, len(keys.Children))
		var previousTime float32
		for keyIndex, key := range keys.Children {
			timeValue := key.Field("time")
			time, ok := animationFloat32(timeValue)
			if !ok || (keyIndex != 0 && time <= previousTime) {
				return nil, fmt.Errorf("curve %d keyframe %d has non-increasing time", curveIndex, keyIndex)
			}
			value := key.Field("value")
			if value == nil {
				return nil, fmt.Errorf("curve %d keyframe %d has no value", curveIndex, keyIndex)
			}
			keyframe := AnimationTRSKeyframe{Time: time}
			componentNames := [...]string{"x", "y", "z", "w"}
			for componentIndex := uint8(0); componentIndex < dimension; componentIndex++ {
				component, ok := animationFloat32(value.Field(componentNames[componentIndex]))
				if !ok {
					return nil, fmt.Errorf("curve %d keyframe %d has no %s component", curveIndex, keyIndex, componentNames[componentIndex])
				}
				keyframe.Value[componentIndex] = component
			}
			curve.Keyframes = append(curve.Keyframes, keyframe)
			previousTime = time
		}
		if len(curve.Keyframes) != 0 {
			curves = append(curves, curve)
		}
	}
	return curves, nil
}

// animationFloat32 从 TypeTree 标量读取可精确表示的 float32
// animationFloat32 reads a float32-representable scalar from a TypeTree value
func animationFloat32(value *TypeTreeValue) (float32, bool) {
	if value == nil {
		return 0, false
	}
	return value.Float32()
}
