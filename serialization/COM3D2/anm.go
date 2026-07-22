package COM3D2

import (
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// CM3D2_ANIM
// 动画文件，用于描述模型的动画数据
//
// 版本 1001
// 添加了两个布尔值 BustKeyLeft 和 BustKeyRight，用于控制左胸和右胸的动画开关
// 只在 public static PhotoMotionData AddMyPose(string fullpath) in class PhotoMotionData 中判断过版本号
// 因此读取时保留了尝试读取 BustKeyLeft 和 BustKeyRight 的逻辑，即使版本号不匹配也不会报错
// 但在写入时，会根据版本号判断是否写入 BustKeyLeft 和 BustKeyRight
// CM3D2_ANIM
// This animation file describes model animation data
//
// Version 1001
// Two Boolean values named BustKeyLeft and BustKeyRight were added to control left and right bust animation
// The version is checked only in public static PhotoMotionData AddMyPose(string fullpath) in class PhotoMotionData
// The reader therefore retains the attempt to read BustKeyLeft and BustKeyRight and does not report an error when the version does not match
// The writer checks the version before writing BustKeyLeft and BustKeyRight

// PropertyIndex 表示属性索引，用于标识属性的类型
// 最高位为 6，含义如下
// PropertyIndex identifies the property type
// The highest index is 6, with the meanings listed below
const (
	LocalRotationX = 0
	LocalRotationY = 1
	LocalRotationZ = 2
	LocalRotationW = 3
	localPositionX = 4
	localPositionY = 5
	localPositionZ = 6
)

// Anm 整体描述一个 .anm 文件的结构
// Anm describes the overall structure of an .anm file
type Anm struct {
	Signature    string          `json:"Signature"`              // CM3D2_ANIM 文件签名 / CM3D2_ANIM file signature
	Version      int32           `json:"Version"`                // 版本 1001 / Version 1001
	BoneCurves   []BoneCurveData `json:"BoneCurves"`             // 所有骨骼的动画曲线数据 / Animation curve data for all bones
	BustKeyLeft  bool            `json:"BustKeyLeft,omitempty"`  // 左胸部动画开关 / Left bust-animation switch
	BustKeyRight bool            `json:"BustKeyRight,omitempty"` // 右胸部动画开关 / Right bust-animation switch
}

// PropertyCurve 存储单一属性（例如 localRotation.x）的一整条 AnimationCurve
// PropertyCurve stores a complete AnimationCurve for one property such as localRotation.x
type PropertyCurve struct {
	PropertyIndex int        `json:"PropertyIndex"` // 属性索引，b=100 对应 index=0，b=101 对应 index=1，最高位为 6，含义见上方枚举 / Property index where b=100 maps to index=0 and b=101 maps to index=1, with 6 as the highest value as listed above
	Keyframes     []Keyframe `json:"Keyframes"`     // 该属性的所有关键帧数据 / All keyframe data for this property
}

// BoneCurveData 存储某个骨骼(或节点)对应的一组曲线信息
// BoneCurveData stores a group of curves for a bone or node
type BoneCurveData struct {
	BonePath       string          `json:"BonePath"`       // 骨骼路径（如"Bip01/Bip01 Spine/Bip01 Spine0a/Bip01 Spine1"） / Bone path such as "Bip01/Bip01 Spine/Bip01 Spine0a/Bip01 Spine1"
	PropertyCurves []PropertyCurve `json:"PropertyCurves"` // 该骨骼的所有属性动画曲线 / All property animation curves for this bone
}

// ReadAnm 读取并解析一个 .anm 文件，返回 Anm 结构
// ReadAnm reads and parses an .anm file and returns an Anm structure
func ReadAnm(r io.Reader) (*Anm, error) {
	clip := &Anm{}
	reader := stream.NewBinaryReader(r)

	// 1. 读取签名字符串 "CM3D2_ANIM"
	// 1. Read the "CM3D2_ANIM" signature string
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read anm signature failed: %w", err)
	}
	// if sig != AnmSignature {
	// 	return nil, fmt.Errorf("invalid .anm signature: got %q, want %v", sig, AnmSignature)
	// }
	clip.Signature = sig

	// 2. 读取版本号 int32
	// 2. Read the int32 version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read anm version failed: %w", err)
	}
	clip.Version = ver

	// 在 clip.BoneCurves 中定位到当前骨骼对应的下标
	// Track the index of the current bone in clip.BoneCurves
	var currentBoneIndex = -1

	// 3. 循环读取，直到 b == 0
	// 3. Read repeatedly until b == 0
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read chunk byte failed: %w", err)
		}
		if b == 0 {
			// 表示骨骼曲线数据结束，跳出循环
			// This marks the end of bone curve data, so leave the loop
			break
		}

		switch {
		// 用一个字节标识不同数据块：1 表示下一行是骨骼路径字符串，>=100 表示后一段是关键帧曲线数据
		// One byte identifies each data block: 1 means a bone path string follows, while >=100 means keyframe curve data follows
		case b == 1:
			// 读入新的骨骼路径
			// Read a new bone path
			bonePath, err := reader.ReadString()
			if err != nil {
				return nil, fmt.Errorf("read bone path failed: %w", err)
			}

			// 新增一个 BoneCurveData
			// Add a BoneCurveData
			clip.BoneCurves = append(clip.BoneCurves, BoneCurveData{
				BonePath:       bonePath,
				PropertyCurves: []PropertyCurve{},
			})
			currentBoneIndex = len(clip.BoneCurves) - 1

		case b >= 100:
			if currentBoneIndex < 0 {
				return nil, fmt.Errorf("anm file invalid: got property curve data without bone path first")
			}
			// 读取关键帧数量
			// Read the keyframe count
			keyframeCount, err := reader.ReadInt32()
			if err != nil {
				return nil, fmt.Errorf("read keyframeCount failed: %w", err)
			}
			if err := validateNonNegativeCount("animation keyframeCount", keyframeCount); err != nil {
				return nil, err
			}
			kfs := makeCountedSliceForAppend[Keyframe](keyframeCount)
			for i := 0; i < int(keyframeCount); i++ {
				t, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read keyframe time failed: %w", err)
				}
				v, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read keyframe value failed: %w", err)
				}
				inT, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read keyframe inTangent failed: %w", err)
				}
				outT, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read keyframe outTangent failed: %w", err)
				}

				kfs = append(kfs, Keyframe{
					Time:       t,
					Value:      v,
					InTangent:  inT,
					OutTangent: outT,
				})
			}
			// 把这一组关键帧附加到当前骨骼
			// Attach this keyframe group to the current bone
			// 例如 100 对应 0，101 对应 1，依此类推
			// For example, 100 maps to 0, 101 maps to 1, and so on
			propertyIndex := int(b - 100)
			curve := PropertyCurve{
				PropertyIndex: propertyIndex,
				Keyframes:     kfs,
			}
			clip.BoneCurves[currentBoneIndex].PropertyCurves =
				append(clip.BoneCurves[currentBoneIndex].PropertyCurves, curve)
		default:
			return nil, fmt.Errorf("unknown chunk byte: %d", b)
		}
	}

	// 4. 读取两个 byte，用来判断是否启用胸部动画
	// 也有部分文件可能没有这两字节，如果动画不是全身骨骼角色就没有这个
	// 4. Read two bytes that indicate whether bust animation is enabled
	// Some files do not contain these bytes when the animation does not cover a full character skeleton
	bustKeyL, err := reader.ReadByte()
	if err != nil {
		return clip, nil
	}
	clip.BustKeyLeft = bustKeyL != 0
	bustKeyR, err := reader.ReadByte()
	if err != nil {
		return clip, nil
	}
	clip.BustKeyRight = bustKeyR != 0

	return clip, nil
}

// Dump 将 Anm 结构写到 w 中，生成符合 CM3D2_ANIM 格式的二进制数据
// Dump writes an Anm structure to w as CM3D2_ANIM binary data
func (a Anm) Dump(w io.Writer) error {
	if err := validateAnmForDump(&a); err != nil {
		return err
	}
	writer := stream.NewBinaryWriter(w)

	// 1. 写签名
	// 1. Write the signature
	if err := writer.WriteString(a.Signature); err != nil {
		return fmt.Errorf("write anm signature failed: %w", err)
	}
	// 2. 写版本号
	// 2. Write the version
	if err := writer.WriteInt32(a.Version); err != nil {
		return fmt.Errorf("write anm version failed: %w", err)
	}

	// 3. 写骨骼曲线数据
	// 先写 b=1 再写 bonePath
	// 然后对每条属性写 b=(100+index)，再写 keyframeCount，接着写 N 个 keyframe
	// 3. Write the bone curve data
	// Write b=1 followed by bonePath
	// Then write b=(100+index), keyframeCount, and N keyframes for each property
	for _, boneData := range a.BoneCurves {
		// 标记 byte=1，后跟骨骼路径
		// Marker byte=1 is followed by the bone path
		if err := writer.WriteByte(1); err != nil {
			return fmt.Errorf("write boneData mark failed: %w", err)
		}
		if err := writer.WriteString(boneData.BonePath); err != nil {
			return fmt.Errorf("write bone path failed: %w", err)
		}

		// 写所有 PropertyCurve（旋转和位置数据）
		// Write all PropertyCurve values for rotation and position data
		for _, pc := range boneData.PropertyCurves {
			// 属性标记 = 100 + PropertyIndex
			// PropertyIndex 含义参考顶部枚举
			// The property marker equals 100 + PropertyIndex
			// See the enumeration at the top for PropertyIndex meanings
			b := byte(100 + pc.PropertyIndex)
			if err := writer.WriteByte(b); err != nil {
				return fmt.Errorf("write property mark failed: %w", err)
			}
			// keyframe 数量
			// Keyframe count
			kfCount, err := collectionCountInt32("animation keyframe count", len(pc.Keyframes))
			if err != nil {
				return err
			}
			if err := writer.WriteInt32(kfCount); err != nil {
				return fmt.Errorf("write keyframeCount failed: %w", err)
			}
			// 写每个关键帧
			// Write each keyframe
			for _, kf := range pc.Keyframes {
				if err := writer.WriteFloat32(kf.Time); err != nil {
					return fmt.Errorf("write keyframe time failed: %w", err)
				}
				if err := writer.WriteFloat32(kf.Value); err != nil {
					return fmt.Errorf("write keyframe value failed: %w", err)
				}
				if err := writer.WriteFloat32(kf.InTangent); err != nil {
					return fmt.Errorf("write keyframe inTangent failed: %w", err)
				}
				if err := writer.WriteFloat32(kf.OutTangent); err != nil {
					return fmt.Errorf("write keyframe outTangent failed: %w", err)
				}
			}
		}
	}

	// 4. 写一个 0 标记骨骼曲线段结束
	// 4. Write a 0 marker to terminate the bone curve section
	if err := writer.WriteByte(0); err != nil {
		return fmt.Errorf("write end-of-bonedata mark failed: %w", err)
	}

	if a.Version >= 1001 {
		// 5. 写两个 byte，表示胸部动画标志
		// 5. Write two bytes containing the bust-animation flags
		var bustL, bustR byte
		if a.BustKeyLeft {
			bustL = 1
		} else {
			bustL = 0
		}
		if a.BustKeyRight {
			bustR = 1
		} else {
			bustR = 0
		}
		if err := writer.WriteByte(bustL); err != nil {
			return fmt.Errorf("write bustKeyL failed: %w", err)
		}
		if err := writer.WriteByte(bustR); err != nil {
			return fmt.Errorf("write bustKeyR failed: %w", err)
		}
	}

	return nil
}

// validateAnmForDump 检查胸部标志的版本门槛、属性标记的字节范围以及关键帧计数的 Int32 范围
// validateAnmForDump checks the bust-flag version gate, the byte range of property markers, and the Int32 range of keyframe counts
func validateAnmForDump(animation *Anm) error {
	if animation.Version < 1001 && (animation.BustKeyLeft || animation.BustKeyRight) {
		return fmt.Errorf("ANM version %d cannot encode bust-key flags", animation.Version)
	}
	for boneIndex := range animation.BoneCurves {
		bone := &animation.BoneCurves[boneIndex]
		for propertyIndex := range bone.PropertyCurves {
			property := &bone.PropertyCurves[propertyIndex]
			if property.PropertyIndex < 0 || property.PropertyIndex > 255-100 {
				return fmt.Errorf("BoneCurves[%d].PropertyCurves[%d].PropertyIndex=%d cannot be represented by an ANM chunk byte", boneIndex, propertyIndex, property.PropertyIndex)
			}
			if _, err := collectionCountInt32(fmt.Sprintf("BoneCurves[%d].PropertyCurves[%d] keyframe count", boneIndex, propertyIndex), len(property.Keyframes)); err != nil {
				return err
			}
		}
	}
	return nil
}
