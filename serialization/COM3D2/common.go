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
type Vector2 struct {
	X float32 `json:"X"`
	Y float32 `json:"Y"`
}

// Vector3 表示三维向量
type Vector3 struct {
	X float32 `json:"X"`
	Y float32 `json:"Y"`
	Z float32 `json:"Z"`
}

// Quaternion 表示四元数
type Quaternion struct {
	X float32 `json:"X"`
	Y float32 `json:"Y"`
	Z float32 `json:"Z"`
	W float32 `json:"W"`
}

// PositionRotationScale 表示组合位置、旋转、缩放信息
type PositionRotationScale struct {
	Position Vector3    `json:"Position"` // 位置
	Rotation Quaternion `json:"Rotation"` // 旋转
	Scale    Vector3    `json:"Scale"`    // 缩放
}

// Vector4 表示四维向量
type Vector4 struct {
	X float32 `json:"X"`
	Y float32 `json:"Y"`
	Z float32 `json:"Z"`
	W float32 `json:"W"`
}

// Color 表示颜色（ARGB 顺序，与 Unity 序列化一致）
type Color struct {
	A float32 `json:"A"`
	R float32 `json:"R"`
	G float32 `json:"G"`
	B float32 `json:"B"`
}

// Rect 表示矩形区域
type Rect struct {
	XMin float32 `json:"XMin"`
	XMax float32 `json:"XMax"`
	YMin float32 `json:"YMin"`
	YMax float32 `json:"YMax"`
}

// KeyValuePairInt 表示 int32 键值对
type KeyValuePairInt struct {
	Key   int32 `json:"Key"`
	Value int32 `json:"Value"`
}

// Matrix4x4 表示4x4矩阵
type Matrix4x4 [16]float32

// AnimationCurve 用于存储 Keyframe 数组
type AnimationCurve struct {
	Keyframes []Keyframe `json:"Keyframes"`
}

// Keyframe 与 UnityEngine.Keyframe 对应
type Keyframe struct {
	Time       float32 `json:"Time"`
	Value      float32 `json:"Value"`
	InTangent  float32 `json:"InTangent"`
	OutTangent float32 `json:"OutTangent"`
}

func validateNonNegativeCount(name string, count int32) error {
	if count < 0 {
		return fmt.Errorf("%s is negative: %d", name, count)
	}
	return nil
}

func collectionCountInt32(name string, length int) (int32, error) {
	if int64(length) > int64(1<<31-1) {
		return 0, fmt.Errorf("%s %d exceeds Int32", name, length)
	}
	return int32(length), nil
}

// makeCountedSliceForAppend creates a small initial buffer for an on-wire
// Int32 count. It deliberately does not impose a format limit: valid large
// collections can still grow as they are read, while a truncated stream with
// a corrupt huge count cannot force a multi-gigabyte allocation before the
// first element is available.
func makeCountedSliceForAppend[T any](count int32) []T {
	if count <= 0 {
		return make([]T, 0)
	}
	const maxInitialCapacity = 1024
	capacity := int(count)
	if capacity > maxInitialCapacity {
		capacity = maxInitialCapacity
	}
	return make([]T, 0, capacity)
}

func makeCountedMap[K comparable, V any](count int32) map[K]V {
	const maxInitialCapacity = 1024
	capacity := int(count)
	if capacity < 0 {
		capacity = 0
	} else if capacity > maxInitialCapacity {
		capacity = maxInitialCapacity
	}
	return make(map[K]V, capacity)
}

// 因为循环依赖问题，所以写在这里了

// ReadAnimationCurve 读取 AnimationCurve：先读 int(个数)，若为 0 则返回空
func ReadAnimationCurve(reader *stream.BinaryReader) (AnimationCurve, error) {
	n, err := reader.ReadInt32() // 读取 Keyframe 数量
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
	for i := 0; i < int(n); i++ {
		t, err := reader.ReadFloat32() // 读取关键帧时间
		if err != nil {
			return AnimationCurve{}, fmt.Errorf("read keyframe time failed: %w", err)
		}
		v, err := reader.ReadFloat32() // 读取关键帧值
		if err != nil {
			return AnimationCurve{}, fmt.Errorf("read keyframe value failed: %w", err)
		}
		inT, err := reader.ReadFloat32() // 读取关键字入切线
		if err != nil {
			return AnimationCurve{}, fmt.Errorf("read keyframe inTangent failed: %w", err)
		}
		outT, err := reader.ReadFloat32() // 读取关键字出切线
		if err != nil {
			return AnimationCurve{}, fmt.Errorf("read keyframe outTangent failed: %w", err)
		}
		keyframes = append(keyframes, Keyframe{Time: t, Value: v, InTangent: inT, OutTangent: outT})
	}
	return AnimationCurve{Keyframes: keyframes}, nil
}

// WriteAnimationCurve 写出 AnimationCurve：先写 int(个数)，然后依次写 time,value,inTangent,outTangent
func WriteAnimationCurve(writer *stream.BinaryWriter, ac AnimationCurve) error {
	count, err := collectionCountInt32("curve keyCount", len(ac.Keyframes))
	if err != nil {
		return err
	}
	err = writer.WriteInt32(count) // 写入 Keyframe 数量
	if err != nil {
		return fmt.Errorf("write curve keyCount failed: %w", err)
	}

	for _, k := range ac.Keyframes {
		err = writer.WriteFloat32(k.Time) // 写入关键帧时间
		if err != nil {
			return fmt.Errorf("write keyframe time failed: %w", err)
		}

		err = writer.WriteFloat32(k.Value) // 写入关键帧值
		if err != nil {
			return fmt.Errorf("write keyframe value failed: %w", err)
		}

		err = writer.WriteFloat32(k.InTangent) // 写入关键字入切线
		if err != nil {
			return fmt.Errorf("write keyframe inTangent failed: %w", err)
		}

		err = writer.WriteFloat32(k.OutTangent) // 写入关键字出切线
		if err != nil {
			return fmt.Errorf("write keyframe outTangent failed: %w", err)
		}
	}
	return nil
}
