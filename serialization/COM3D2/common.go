package COM3D2

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

const (
	MenuSignature   = "CM3D2_MENU"
	MenuVersion     = 1000
	MateSignature   = "CM3D2_MATERIAL"
	MateVersion     = 2001
	PMatSignature   = "CM3D2_PMATERIAL"
	PMatVersion     = 1000
	ColSignature    = "CM3D21_COL"
	ColVersion      = 24301
	PhySignature    = "CM3D21_PHY"
	PhyVersion      = 24301
	PskSignature    = "CM3D21_PSK"
	PskVersion      = 24301
	TexSignature    = "CM3D2_TEX"
	TexVersion      = 1010
	AnmSignature    = "CM3D2_ANIM"
	AnmVersion      = 1001
	ModelSignature  = "CM3D2_MESH"
	ModelVersion    = 2001
	PresetSignature = "CM3D2_PRESET"
	PresetVersion   = 24301
	SaveSignature   = "COM3D2_SAVE"
	SaveVersion     = 24301
)

const (
	endByte = 0x00
	EndTag  = "end"
)

const (
	SkinThicknessSignature = "SkinThickness"
	SkinThicknessVersion   = 100
)

const (
	PresetPropertyListSignature = "CM3D2_MPROP_LIST"
	PresetPropertyListVersion   = 24301
	PresetPropertySignature     = "CM3D2_MPROP"
	PresetPropertyVersion       = 24301
	MultiColorSignature         = "CM3D2_MULTI_COL"
	MultiColorVersion           = 24301
	BodyPropertySignature       = "CM3D2_MAID_BODY"
	BodyPropertyVersion         = 24301
)

var (
	NeiSignature = []byte{0x77, 0x73, 0x76, 0xFF}
)

// Vector2 表示二维向量或UV坐标
// Vector2 represents a two-dimensional vector or UV coordinate
type Vector2 struct {
	X float32 `json:"X"` // X 轴分量 / X-axis component
	Y float32 `json:"Y"` // Y 轴分量 / Y-axis component
}

// Vector3 表示三维向量
// Vector3 represents a three-dimensional vector
type Vector3 struct {
	X float32 `json:"X"` // X 轴分量 / X-axis component
	Y float32 `json:"Y"` // Y 轴分量 / Y-axis component
	Z float32 `json:"Z"` // Z 轴分量 / Z-axis component
}

// Quaternion 表示四元数
// Quaternion represents a quaternion
type Quaternion struct {
	X float32 `json:"X"` // 四元数的 X 分量 / X component of the quaternion
	Y float32 `json:"Y"` // 四元数的 Y 分量 / Y component of the quaternion
	Z float32 `json:"Z"` // 四元数的 Z 分量 / Z component of the quaternion
	W float32 `json:"W"` // 四元数的 W 分量 / W component of the quaternion
}

// PositionRotationScale 表示组合位置、旋转、缩放信息
// PositionRotationScale represents combined position, rotation, and scale information
type PositionRotationScale struct {
	Position Vector3    `json:"Position"` // 位置 / Position
	Rotation Quaternion `json:"Rotation"` // 旋转 / Rotation
	Scale    Vector3    `json:"Scale"`    // 缩放 / Scale
}

// Vector4 表示四维向量
// Vector4 represents a four-dimensional vector
type Vector4 struct {
	X float32 `json:"X"` // X 轴分量 / X-axis component
	Y float32 `json:"Y"` // Y 轴分量 / Y-axis component
	Z float32 `json:"Z"` // Z 轴分量 / Z-axis component
	W float32 `json:"W"` // W 轴分量 / W-axis component
}

// Color 表示颜色（ARGB 顺序，与 Unity 序列化一致）
// Color represents a color in ARGB order, matching Unity serialization
type Color struct {
	A float32 `json:"A"` // Alpha 分量 / Alpha component
	R float32 `json:"R"` // 红色分量 / Red component
	G float32 `json:"G"` // 绿色分量 / Green component
	B float32 `json:"B"` // 蓝色分量 / Blue component
}

// Rect 表示矩形区域
// Rect represents a rectangular region
type Rect struct {
	XMin float32 `json:"XMin"` // X 轴最小边界 / Minimum X bound
	XMax float32 `json:"XMax"` // X 轴最大边界 / Maximum X bound
	YMin float32 `json:"YMin"` // Y 轴最小边界 / Minimum Y bound
	YMax float32 `json:"YMax"` // Y 轴最大边界 / Maximum Y bound
}

// KeyValuePairInt 表示 int32 键值对
// KeyValuePairInt represents an int32 key-value pair
type KeyValuePairInt struct {
	Key   int32 `json:"Key"`   // 键 / Key
	Value int32 `json:"Value"` // 值 / Value
}

// Matrix4x4 表示4x4矩阵
// Matrix4x4 represents a 4x4 matrix
type Matrix4x4 [16]float32

// AnimationCurve 用于存储 Keyframe 数组
// AnimationCurve stores a Keyframe array
type AnimationCurve struct {
	Keyframes []Keyframe `json:"Keyframes"` // 按时间保存的关键帧 / Keyframes stored in time order
}

// Keyframe 与 UnityEngine.Keyframe 对应
// Keyframe corresponds to UnityEngine.Keyframe
type Keyframe struct {
	Time       float32 `json:"Time"`       // 关键帧时间 / Keyframe time
	Value      float32 `json:"Value"`      // 关键帧值 / Keyframe value
	InTangent  float32 `json:"InTangent"`  // 入切线 / Incoming tangent
	OutTangent float32 `json:"OutTangent"` // 出切线 / Outgoing tangent
}

// validateNonNegativeCount 拒绝游戏格式中不合法的负 Int32 集合计数
// validateNonNegativeCount rejects negative Int32 collection counts that are invalid in the game formats
func validateNonNegativeCount(name string, count int32) error {
	if count < 0 {
		return fmt.Errorf("%s is negative: %d", name, count)
	}
	return nil
}

// collectionCountInt32 将 Go 集合长度转换为游戏线格式使用的 Int32 计数
// collectionCountInt32 converts a Go collection length to the Int32 count used by the game wire formats
func collectionCountInt32(name string, length int64) (int32, error) {
	if length < 0 || length > 1<<31-1 {
		return 0, fmt.Errorf("%s %d exceeds Int32", name, length)
	}
	return int32(length), nil
}

// makeCountedSliceForAppend 为线格式中的 Int32 计数创建较小的初始缓冲区
// 它不会强加格式限制，有效的大集合仍可在读取时增长，而带有损坏超大计数的截断流无法在读取首个元素前强制分配数 GB 内存
// makeCountedSliceForAppend creates a small initial buffer for an on-wire
// Int32 count; it deliberately does not impose a format limit: valid large
// collections can still grow as they are read, while a truncated stream with
// a corrupt huge count cannot force a multi-gigabyte allocation before the
// first element is available
func makeCountedSliceForAppend[T any](count int32) []T {
	if count <= 0 {
		return make([]T, 0)
	}
	const maxInitialCapacity = 1024
	capacity := count
	if capacity > maxInitialCapacity {
		capacity = maxInitialCapacity
	}
	return make([]T, 0, capacity)
}

// makeCountedMap 根据线格式计数创建容量受限的初始映射，避免损坏文件触发巨额预分配
// makeCountedMap creates a map with a capped initial capacity from a wire count, preventing corrupt files from triggering excessive preallocation
func makeCountedMap[K comparable, V any](count int32) map[K]V {
	const maxInitialCapacity = 1024
	capacity := count
	if capacity < 0 {
		capacity = 0
	} else if capacity > maxInitialCapacity {
		capacity = maxInitialCapacity
	}
	return make(map[K]V, capacity)
}

// 因为循环依赖问题，所以写在这里了
// These helpers are placed here because of an import cycle

// ReadAnimationCurve 读取 AnimationCurve：先读 int(个数)，若为 0 则返回空
// ReadAnimationCurve reads the keyframe count first and returns an empty curve when it is 0
func ReadAnimationCurve(reader *stream.BinaryReader) (AnimationCurve, error) {
	// 读取 Keyframe 数量
	// Read the Keyframe count
	n, err := reader.ReadInt32()
	if err != nil {
		return AnimationCurve{}, fmt.Errorf("read curve keyCount failed: %w", err)
	}
	if err := validateNonNegativeCount("curve keyCount", n); err != nil {
		return AnimationCurve{}, err
	}
	if n == 0 {
		return AnimationCurve{}, nil
	}
	keyframes := makeCountedSliceForAppend[Keyframe](n)
	for i := int32(0); i < n; i++ {
		// 读取关键帧时间
		// Read the keyframe time
		t, err := reader.ReadFloat32()
		if err != nil {
			return AnimationCurve{}, fmt.Errorf("read keyframe time failed: %w", err)
		}
		// 读取关键帧值
		// Read the keyframe value
		v, err := reader.ReadFloat32()
		if err != nil {
			return AnimationCurve{}, fmt.Errorf("read keyframe value failed: %w", err)
		}
		// 读取关键字入切线
		// Read the incoming keyframe tangent
		inT, err := reader.ReadFloat32()
		if err != nil {
			return AnimationCurve{}, fmt.Errorf("read keyframe inTangent failed: %w", err)
		}
		// 读取关键字出切线
		// Read the outgoing keyframe tangent
		outT, err := reader.ReadFloat32()
		if err != nil {
			return AnimationCurve{}, fmt.Errorf("read keyframe outTangent failed: %w", err)
		}
		keyframes = append(keyframes, Keyframe{Time: t, Value: v, InTangent: inT, OutTangent: outT})
	}
	return AnimationCurve{Keyframes: keyframes}, nil
}

// WriteAnimationCurve 写出 AnimationCurve：先写 int(个数)，然后依次写 time,value,inTangent,outTangent
// WriteAnimationCurve writes the count first, followed by time, value, inTangent, and outTangent for each keyframe
func WriteAnimationCurve(writer *stream.BinaryWriter, ac AnimationCurve) error {
	count, err := collectionCountInt32("curve keyCount", int64(len(ac.Keyframes)))
	if err != nil {
		return err
	}
	// 写入 Keyframe 数量
	// Write the Keyframe count
	err = writer.WriteInt32(count)
	if err != nil {
		return fmt.Errorf("write curve keyCount failed: %w", err)
	}

	for _, k := range ac.Keyframes {
		// 写入关键帧时间
		// Write the keyframe time
		err = writer.WriteFloat32(k.Time)
		if err != nil {
			return fmt.Errorf("write keyframe time failed: %w", err)
		}

		// 写入关键帧值
		// Write the keyframe value
		err = writer.WriteFloat32(k.Value)
		if err != nil {
			return fmt.Errorf("write keyframe value failed: %w", err)
		}

		// 写入关键字入切线
		// Write the incoming keyframe tangent
		err = writer.WriteFloat32(k.InTangent)
		if err != nil {
			return fmt.Errorf("write keyframe inTangent failed: %w", err)
		}

		// 写入关键字出切线
		// Write the outgoing keyframe tangent
		err = writer.WriteFloat32(k.OutTangent)
		if err != nil {
			return fmt.Errorf("write keyframe outTangent failed: %w", err)
		}
	}
	return nil
}
