package COM3D2

import (
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

// CM3D21_PHY
// 物理信息文件
//
// 无版本差异
// CM3D21_PHY
// Physics information file
//
// There are no version differences

// -------------------------------------------------------
// 定义 Phy (Phy) 的数据结构
// Define the data structure of Phy (Phy)
// -------------------------------------------------------

// Phy 保存一个 CM3D21_PHY 文件的全部字段
// Phy stores all fields of a CM3D21_PHY file
type Phy struct {
	// 头部信息
	// Header information
	Signature string `json:"Signature"` // 签名，通常为 "CM3D21_PHY" / 1. Signature, normally "CM3D21_PHY"
	Version   int32  `json:"Version"`   // 版本，例如 24102，此版本随每次更新变化但结构不变 / 2. Version such as 24102, which changes with each update without changing the structure
	RootName  string `json:"RootName"`  // RootBone 名称 / 3. RootBone name

	// 4. Damping 阻尼相关参数
	// 4. Damping parameters
	EnablePartialDamping int32          `json:"EnablePartialDamping"` // PartialMode 枚举，模式 / PartialMode enum selecting the mode
	PartialDamping       []BoneValue    `json:"PartialDamping"`       // 按骨骼设置的阻尼值 / Per-bone damping values
	Damping              float32        `json:"Damping"`              // 静态或曲线模式下的阻尼值 / Damping value in static-or-curve mode
	DampingDistrib       AnimationCurve `json:"DampingDistrib"`       // 曲线 / Curve

	// 5. Elasticity 弹性相关参数
	// 5. Elasticity parameters
	EnablePartialElasticity int32          `json:"EnablePartialElasticity"` // 弹性 PartialMode 枚举 / Elasticity PartialMode enum
	PartialElasticity       []BoneValue    `json:"PartialElasticity"`       // 按骨骼设置的弹性值 / Per-bone elasticity values
	Elasticity              float32        `json:"Elasticity"`              // 静态或曲线模式下的弹性值 / Elasticity value in static-or-curve mode
	ElasticityDistrib       AnimationCurve `json:"ElasticityDistrib"`       // 弹性曲线 / Elasticity curve

	// 6. Stiffness 刚度相关参数
	// 6. Stiffness parameters
	EnablePartialStiffness int32          `json:"EnablePartialStiffness"` // 刚度 PartialMode 枚举 / Stiffness PartialMode enum
	PartialStiffness       []BoneValue    `json:"PartialStiffness"`       // 按骨骼设置的刚度值 / Per-bone stiffness values
	Stiffness              float32        `json:"Stiffness"`              // 静态或曲线模式下的刚度值 / Stiffness value in static-or-curve mode
	StiffnessDistrib       AnimationCurve `json:"StiffnessDistrib"`       // 刚度曲线 / Stiffness curve

	// 7. Inert 惯性相关参数
	// 7. Inert parameters
	EnablePartialInert int32          `json:"EnablePartialInert"` // 惯性 PartialMode 枚举 / Inert PartialMode enum
	PartialInert       []BoneValue    `json:"PartialInert"`       // 按骨骼设置的惯性值 / Per-bone inert values
	Inert              float32        `json:"Inert"`              // 静态或曲线模式下的惯性值 / Inert value in static-or-curve mode
	InertDistrib       AnimationCurve `json:"InertDistrib"`       // 惯性曲线 / Inert curve

	// 8. 碰撞半径相关参数
	// 8. Collision radius parameters
	EnablePartialRadius int32          `json:"EnablePartialRadius"` // 半径 PartialMode 枚举 / Radius PartialMode enum
	PartialRadius       []BoneValue    `json:"PartialRadius"`       // 按骨骼设置的半径值 / Per-bone radius values
	Radius              float32        `json:"Radius"`              // 静态或曲线模式下的碰撞半径 / Collision radius in static-or-curve mode
	RadiusDistrib       AnimationCurve `json:"RadiusDistrib"`       // 半径曲线 / Radius curve

	// 9. 骨骼末端参数
	// 9. Bone-end parameters
	EndLength float32    `json:"EndLength"` // 骨骼末端长度 / Bone end length
	EndOffset [3]float32 `json:"EndOffset"` // 骨骼末端偏移 x、y、z / Bone end offset x, y, z

	// 10. 外力参数
	// 10. External-force parameters
	Gravity [3]float32 `json:"Gravity"` // 外力重力向量 x、y、z / External gravity vector x, y, z
	Force   [3]float32 `json:"Force"`   // 外力向量 x、y、z / External force vector x, y, z

	// 10. 碰撞器相关参数
	// 10. Collider parameters
	ColliderFileName string `json:"ColliderFileName"` // 碰撞器文件名 / Collider filename
	CollidersCount   int32  `json:"CollidersCount"`   // 碰撞器数量，游戏只写入数量而不写后续碰撞器数据 / Collider count written by the game without following collider data

	// 11. 排除骨骼
	// 11. Excluded bones
	ExclusionsCount int32 `json:"ExclusionsCount"` // 排除的骨骼数量，游戏只写入数量而不写后续列表 / Excluded-bone count written by the game without a following list

	// 12. 冻结轴向
	// 12. Freeze axis
	FreezeAxis int32 `json:"FreezeAxis"` // 冻结轴向 FreezeAxis 枚举值 / Freeze axis FreezeAxis enum value
}

// PartialMode 枚举
// PartialMode enum
const (
	PartialModeStaticOrCurve int32 = 0 // C# 里的 StaticOrCurve，静态或曲线模式  / StaticOrCurve in C#, the static-or-curve mode
	PartialModePartial       int32 = 1 // C#里的 Partial，按骨骼设置模式 / Partial in C#, the per-bone mode
	PartialModeFromBoneName  int32 = 2 // C#里的 FromBoneName，旧自动按骨骼名设置模式 / FromBoneName in C#, the legacy automatic per-bone-name mode
)

// FreezeAxis 枚举
// FreezeAxis enum
const (
	FreezeAxisNone int32 = 0
	FreezeAxisX    int32 = 1
	FreezeAxisY    int32 = 2
	FreezeAxisZ    int32 = 3
)

// BoneValue 存储一个骨骼名称与对应 float 值
// BoneValue stores a bone name and its corresponding float value
type BoneValue struct {
	BoneName string  `json:"BoneName"` // 骨骼名称 / Bone name
	Value    float32 `json:"Value"`    // 该骨骼的参数值 / Parameter value for the bone
}

// ReadPhy 读取 "CM3D21_PHY" 格式
// ReadPhy reads the "CM3D21_PHY" format
func ReadPhy(r io.Reader) (*Phy, error) {
	p := &Phy{}

	reader := stream.NewBinaryReader(r)

	// 1. 签名
	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read signature failed: %w", err)
	}
	// if sig != PhySignature {
	// 	return nil, fmt.Errorf("invalid phy signature, want %v, got %q", PhySignature, sig)
	// }
	p.Signature = sig

	// 2. 版本
	// 2. Version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read version failed: %w", err)
	}
	p.Version = ver

	// 3. 根骨骼名称
	// 3. RootName
	rootName, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read rootName failed: %w", err)
	}
	p.RootName = rootName

	// 4. 阻尼
	// 4. Damping
	p.EnablePartialDamping, p.PartialDamping, err = readPartial(reader)
	if err != nil {
		return nil, fmt.Errorf("read partial damping failed: %w", err)
	}
	p.Damping, err = reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read Damping failed: %w", err)
	}
	p.DampingDistrib, err = ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read DampingDistrib failed: %w", err)
	}

	// 5. 弹性
	// 5. Elasticity
	p.EnablePartialElasticity, p.PartialElasticity, err = readPartial(reader)
	if err != nil {
		return nil, fmt.Errorf("read partial elasticity failed: %w", err)
	}
	p.Elasticity, err = reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read Elasticity failed: %w", err)
	}
	p.ElasticityDistrib, err = ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read ElasticityDistrib failed: %w", err)
	}

	// 6. 刚度
	// 6. Stiffness
	p.EnablePartialStiffness, p.PartialStiffness, err = readPartial(reader)
	if err != nil {
		return nil, fmt.Errorf("read partial stiffness failed: %w", err)
	}
	p.Stiffness, err = reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read Stiffness failed: %w", err)
	}
	p.StiffnessDistrib, err = ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read StiffnessDistrib failed: %w", err)
	}

	// 7. 惯性
	// 7. Inert
	p.EnablePartialInert, p.PartialInert, err = readPartial(reader)
	if err != nil {
		return nil, fmt.Errorf("read partial inert failed: %w", err)
	}
	p.Inert, err = reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read Inert failed: %w", err)
	}
	p.InertDistrib, err = ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read InertDistrib failed: %w", err)
	}

	// 8. 半径
	// 8. Radius
	p.EnablePartialRadius, p.PartialRadius, err = readPartial(reader)
	if err != nil {
		return nil, fmt.Errorf("read partial radius failed: %w", err)
	}
	p.Radius, err = reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read Radius failed: %w", err)
	}
	p.RadiusDistrib, err = ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read RadiusDistrib failed: %w", err)
	}

	// 9. 末端长度与末端偏移 (x,y,z)
	// 9. EndLength, EndOffset (x,y,z)
	p.EndLength, err = reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read EndLength failed: %w", err)
	}
	// 末端偏移
	// EndOffset
	for i := 0; i < 3; i++ {
		p.EndOffset[i], err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("read EndOffset[%d] failed: %w", i, err)
		}
	}

	// 10. 重力 (x,y,z)，外力 (x,y,z)
	// 10. Gravity (x,y,z), Force (x,y,z)
	for i := 0; i < 3; i++ {
		p.Gravity[i], err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("read Gravity[%d] failed: %w", i, err)
		}
	}
	// 外力
	// Force
	for i := 0; i < 3; i++ {
		p.Force[i], err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("read Force[%d] failed: %w", i, err)
		}
	}

	// 11. 碰撞器文件名
	// 11. ColliderFileName
	cfn, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read ColliderFileName failed: %w", err)
	}
	p.ColliderFileName = cfn

	// 12. 碰撞器数量
	// 12. CollidersCount
	colCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read CollidersCount failed: %w", err)
	}
	if err := validateNonNegativeCount("CollidersCount", colCount); err != nil {
		return nil, err
	}
	p.CollidersCount = colCount

	// 虽然 C# 记录了 CollidersCount，但并没有写任何碰撞器内容
	// 此处保留 CollidersCount 只是为了初始化列表，碰撞器数据位于独立的 CM3D21_COL 文件
	// Although C# records CollidersCount, it does not write any collider payload
	// CollidersCount is retained here just for initializing the list, while collider data lives in a separate CM3D21_COL file

	// 13. 排除骨骼数量
	// 13. ExclusionsCount
	excCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read ExclusionsCount failed: %w", err)
	}
	if err := validateNonNegativeCount("ExclusionsCount", excCount); err != nil {
		return nil, err
	}
	p.ExclusionsCount = excCount

	// 同样，C# 只写了数量，没有写任何排除骨骼内容
	// 此处保留 ExclusionsCount 只是为了初始化列表，但无法从文件恢复排除骨骼列表，猜测此功能已弃用
	// Likewise, C# writes only the count and no excluded-bone payload
	// ExclusionsCount is retained here just for initializing the list, but no excluded-bone list can be recovered from the file, It is speculated that this feature has been deprecated

	// 13. 冻结轴向
	// 13. FreezeAxis
	fa, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read freezeAxis failed: %w", err)
	}
	p.FreezeAxis = fa

	return p, nil
}

// Dump 写出 "CM3D21_PHY" 格式
// Dump writes the "CM3D21_PHY" format
func (p *Phy) Dump(w io.Writer) error {
	if err := validatePhyForDump(p); err != nil {
		return err
	}
	writer := stream.NewBinaryWriter(w)

	// 1. 签名
	// 1. Signature
	if err := writer.WriteString(p.Signature); err != nil {
		return fmt.Errorf("write signature failed: %w", err)
	}
	// 2. 版本
	// 2. Version
	if err := writer.WriteInt32(p.Version); err != nil {
		return fmt.Errorf("write version failed: %w", err)
	}
	// 3. 根骨骼名称
	// 3. RootName
	if err := writer.WriteString(p.RootName); err != nil {
		return fmt.Errorf("write rootName failed: %w", err)
	}

	// 4. 阻尼
	// 4. Damping
	if err := writePartial(writer, p.EnablePartialDamping, p.PartialDamping); err != nil {
		return fmt.Errorf("write partial damping failed: %w", err)
	}
	if err := writer.WriteFloat32(p.Damping); err != nil {
		return fmt.Errorf("write Damping failed: %w", err)
	}
	if err := WriteAnimationCurve(writer, p.DampingDistrib); err != nil {
		return fmt.Errorf("write DampingDistrib failed: %w", err)
	}

	// 5. 弹性
	// 5. Elasticity
	if err := writePartial(writer, p.EnablePartialElasticity, p.PartialElasticity); err != nil {
		return fmt.Errorf("write partial elasticity failed: %w", err)
	}
	if err := writer.WriteFloat32(p.Elasticity); err != nil {
		return fmt.Errorf("write Elasticity failed: %w", err)
	}
	if err := WriteAnimationCurve(writer, p.ElasticityDistrib); err != nil {
		return fmt.Errorf("write ElasticityDistrib failed: %w", err)
	}

	// 6. 刚度
	// 6. Stiffness
	if err := writePartial(writer, p.EnablePartialStiffness, p.PartialStiffness); err != nil {
		return fmt.Errorf("write partial stiffness failed: %w", err)
	}
	if err := writer.WriteFloat32(p.Stiffness); err != nil {
		return fmt.Errorf("write Stiffness failed: %w", err)
	}
	if err := WriteAnimationCurve(writer, p.StiffnessDistrib); err != nil {
		return fmt.Errorf("write StiffnessDistrib failed: %w", err)
	}

	// 7. 惯性
	// 7. Inert
	if err := writePartial(writer, p.EnablePartialInert, p.PartialInert); err != nil {
		return fmt.Errorf("write partial inert failed: %w", err)
	}
	if err := writer.WriteFloat32(p.Inert); err != nil {
		return fmt.Errorf("write Inert failed: %w", err)
	}
	if err := WriteAnimationCurve(writer, p.InertDistrib); err != nil {
		return fmt.Errorf("write InertDistrib failed: %w", err)
	}

	// 8. 半径
	// 8. Radius
	if err := writePartial(writer, p.EnablePartialRadius, p.PartialRadius); err != nil {
		return fmt.Errorf("write partial radius failed: %w", err)
	}
	if err := writer.WriteFloat32(p.Radius); err != nil {
		return fmt.Errorf("write Radius failed: %w", err)
	}
	if err := WriteAnimationCurve(writer, p.RadiusDistrib); err != nil {
		return fmt.Errorf("write RadiusDistrib failed: %w", err)
	}

	// 9. 末端长度
	// 9. EndLength
	if err := writer.WriteFloat32(p.EndLength); err != nil {
		return fmt.Errorf("write EndLength failed: %w", err)
	}
	// 10. 末端偏移 (x, y, z)
	// 10. EndOffset (x, y, z)
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(p.EndOffset[i]); err != nil {
			return fmt.Errorf("write EndOffset[%d] failed: %w", i, err)
		}
	}
	// 11. 重力 (x, y, z)
	// 11. Gravity (x, y, z)
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(p.Gravity[i]); err != nil {
			return fmt.Errorf("write Gravity[%d] failed: %w", i, err)
		}
	}
	// 12. 外力 (x, y, z)
	// 12. Force (x, y, z)
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(p.Force[i]); err != nil {
			return fmt.Errorf("write Force[%d] failed: %w", i, err)
		}
	}

	// 13. 碰撞器文件名
	// 13. ColliderFileName
	if err := writer.WriteString(p.ColliderFileName); err != nil {
		return fmt.Errorf("write ColliderFileName failed: %w", err)
	}

	// 14. 碰撞器数量
	// 14. CollidersCount
	if err := writer.WriteInt32(p.CollidersCount); err != nil {
		return fmt.Errorf("write CollidersCount failed: %w", err)
	}

	// 虽然 C# 记录了 CollidersCount，但并没有写任何内容，所以这里直接略过
	// 碰撞器有自己的 col 格式，因此 phy 内不写出 col 内容，猜测以前的版本中 phy 和 col 是合并的
	// CollidersCount 仍保留在线格式中
	// Although C# records CollidersCount, it writes no payload here, so it is skipped
	// Colliders use their own col format, so phy does not contain col payload, It is speculated that in previous versions, phy and col were merged
	// CollidersCount remains present on the wire

	// 15. 排除骨骼数量
	// 15. ExclusionsCount
	if err := writer.WriteInt32(p.ExclusionsCount); err != nil {
		return fmt.Errorf("write ExclusionsCount failed: %w", err)
	}
	// 同样，C# 只写了数量，没有写任何内容
	// ExclusionsCount 仍保留在线格式中
	// Likewise, C# writes only the count and no payload
	// ExclusionsCount remains present on the wire

	// 16. 冻结轴向
	// 16. FreezeAxis
	if err := writer.WriteInt32(p.FreezeAxis); err != nil {
		return fmt.Errorf("write freezeAxis failed: %w", err)
	}

	return nil
}

// validatePhyForDump 验证计数、Partial 模式和所有 AnimationCurve 的关键帧数量不会在写出时被静默丢弃或溢出
// validatePhyForDump verifies that counts, Partial modes, and all AnimationCurve keyframe counts will not be silently discarded or overflow during writing
func validatePhyForDump(phy *Phy) error {
	if phy == nil {
		return fmt.Errorf("nil phy")
	}
	if err := validateNonNegativeCount("CollidersCount", phy.CollidersCount); err != nil {
		return err
	}
	if err := validateNonNegativeCount("ExclusionsCount", phy.ExclusionsCount); err != nil {
		return err
	}
	partials := []struct {
		name   string      // 校验项名称 / Validation item name
		mode   int32       // Partial 模式值 / Partial mode value
		values []BoneValue // 对应的骨骼值 / Bone values for the item
	}{
		{name: "PartialDamping", mode: phy.EnablePartialDamping, values: phy.PartialDamping},
		{name: "PartialElasticity", mode: phy.EnablePartialElasticity, values: phy.PartialElasticity},
		{name: "PartialStiffness", mode: phy.EnablePartialStiffness, values: phy.PartialStiffness},
		{name: "PartialInert", mode: phy.EnablePartialInert, values: phy.PartialInert},
		{name: "PartialRadius", mode: phy.EnablePartialRadius, values: phy.PartialRadius},
	}
	for _, partial := range partials {
		if partial.mode != PartialModePartial && len(partial.values) != 0 {
			return fmt.Errorf("%s mode=%d would discard %d bone values", partial.name, partial.mode, len(partial.values))
		}
		if partial.mode == PartialModePartial {
			if _, err := collectionCountInt32(partial.name+" count", int64(len(partial.values))); err != nil {
				return err
			}
		}
	}
	curves := []struct {
		name  string         // 曲线名称 / Curve name
		curve AnimationCurve // 曲线数据 / Curve data
	}{
		{name: "DampingDistrib", curve: phy.DampingDistrib},
		{name: "ElasticityDistrib", curve: phy.ElasticityDistrib},
		{name: "StiffnessDistrib", curve: phy.StiffnessDistrib},
		{name: "InertDistrib", curve: phy.InertDistrib},
		{name: "RadiusDistrib", curve: phy.RadiusDistrib},
	}
	for _, curve := range curves {
		if _, err := collectionCountInt32(curve.name+" keyCount", int64(len(curve.curve.Keyframes))); err != nil {
			return err
		}
	}
	return nil
}

// readPartial 读取：
// int(PartialMode) -> 如果 != PartialModePartial, 结束；
// int(boneCount) -> 循环读取 boneName + floatValue
// readPartial reads:
// int(PartialMode) -> finish when != PartialModePartial
// int(boneCount) -> repeatedly read boneName + floatValue
func readPartial(reader *stream.BinaryReader) (int32, []BoneValue, error) {
	// 读取 PartialMode，对应 PartialMode 枚举
	// Read PartialMode, corresponding to the PartialMode enum
	mode, err := reader.ReadInt32()
	if err != nil {
		return 0, nil, fmt.Errorf("read partialMode failed: %w", err)
	}
	// 如果不是 PartialModePartial 部位模式，直接返回
	// Return immediately when this is not PartialModePartial mode
	if mode != PartialModePartial {
		return mode, nil, nil
	}

	// 读取骨骼数量
	// Read the bone count
	count, err := reader.ReadInt32()
	if err != nil {
		return mode, nil, fmt.Errorf("read partial count failed: %w", err)
	}
	if err := validateNonNegativeCount("partial count", count); err != nil {
		return mode, nil, err
	}

	vals := makeCountedSliceForAppend[BoneValue](count)
	// 循环读取骨骼名称和对应 float 值
	// Repeatedly read each bone name and corresponding float value
	for i := int32(0); i < count; i++ {
		// 读取骨骼名称
		// Read the bone name
		bn, err := reader.ReadString()
		if err != nil {
			return mode, nil, fmt.Errorf("read boneName failed: %w", err)
		}
		// 读取对应 float 值
		// Read the corresponding float value
		fv, err := reader.ReadFloat32()
		if err != nil {
			return mode, nil, fmt.Errorf("read boneValue failed: %w", err)
		}
		// 存储到切片中
		// Store in the slice
		vals = append(vals, BoneValue{BoneName: bn, Value: fv})
	}
	return mode, vals, nil
}

// writePartial 写出：
// int(PartialMode) -> 如果 == PartialModePartial 再写 (count + boneName + floatValue * count)
// writePartial writes:
// int(PartialMode) -> when == PartialModePartial, also write (count + boneName + floatValue * count)
func writePartial(writer *stream.BinaryWriter, mode int32, values []BoneValue) error {
	if err := writer.WriteInt32(mode); err != nil {
		return fmt.Errorf("write partialMode failed: %w", err)
	}
	if mode != PartialModePartial {
		return nil
	}

	count, err := collectionCountInt32("partial count", int64(len(values)))
	if err != nil {
		return err
	}
	if err := writer.WriteInt32(count); err != nil {
		return fmt.Errorf("write partial count failed: %w", err)
	}
	for _, bv := range values {
		if err := writer.WriteString(bv.BoneName); err != nil {
			return fmt.Errorf("write boneName failed: %w", err)
		}
		if err := writer.WriteFloat32(bv.Value); err != nil {
			return fmt.Errorf("write boneValue failed: %w", err)
		}
	}
	return nil
}
