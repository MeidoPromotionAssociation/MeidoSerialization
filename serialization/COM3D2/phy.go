package COM3D2

import (
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// CM3D21_PHY
// 物理信息文件
//
// 无版本差异

// -------------------------------------------------------
// 定义 Phy (Phy) 的数据结构
// -------------------------------------------------------

type Phy struct {
	// 头部信息
	Signature string `json:"Signature"` // 1. 签名, 通常为 "CM3D21_PHY"
	Version   int32  `json:"Version"`   // 2. 版本 (例如 24102) 这个版本每次更新都会更改，但无结构更改
	RootName  string `json:"RootName"`  // 3. RootBone 名称

	// 4. Damping 阻尼相关参数
	EnablePartialDamping int32          `json:"EnablePartialDamping"` // PartialMode 枚举，模式
	PartialDamping       []BoneValue    `json:"PartialDamping"`       // 按骨骼设置的阻尼值
	Damping              float32        `json:"Damping"`              // 静态或曲线模式下的阻尼值
	DampingDistrib       AnimationCurve `json:"DampingDistrib"`       // 曲线

	// 5. Elasticity 弹性相关参数
	EnablePartialElasticity int32          `json:"EnablePartialElasticity"`
	PartialElasticity       []BoneValue    `json:"PartialElasticity"`
	Elasticity              float32        `json:"Elasticity"`
	ElasticityDistrib       AnimationCurve `json:"ElasticityDistrib"`

	// 6. Stiffness 刚度相关参数
	EnablePartialStiffness int32          `json:"EnablePartialStiffness"`
	PartialStiffness       []BoneValue    `json:"PartialStiffness"`
	Stiffness              float32        `json:"Stiffness"`
	StiffnessDistrib       AnimationCurve `json:"StiffnessDistrib"`

	// 7. Inert 惯性相关参数
	EnablePartialInert int32          `json:"EnablePartialInert"`
	PartialInert       []BoneValue    `json:"PartialInert"`
	Inert              float32        `json:"Inert"`
	InertDistrib       AnimationCurve `json:"InertDistrib"`

	// 8. 碰撞半径相关参数
	EnablePartialRadius int32          `json:"EnablePartialRadius"`
	PartialRadius       []BoneValue    `json:"PartialRadius"`
	Radius              float32        `json:"Radius"`
	RadiusDistrib       AnimationCurve `json:"RadiusDistrib"`

	// 9. 骨骼末端参数
	EndLength float32    `json:"EndLength"`
	EndOffset [3]float32 `json:"EndOffset"`

	// 10. 外力参数
	Gravity [3]float32 `json:"Gravity"`
	Force   [3]float32 `json:"Force"`

	// 10. 碰撞器相关参数
	ColliderFileName string `json:"ColliderFileName"` // 碰撞器文件名
	CollidersCount   int32  `json:"CollidersCount"`   // 碰撞器数量

	// 11.  排除骨骼
	ExclusionsCount int32 `json:"ExclusionsCount"` // 排除的骨骼数量

	// 12. 冻结轴向
	FreezeAxis int32 `json:"FreezeAxis"` // FreezeAxis 枚举
}

// PartialMode 枚举
const (
	PartialModeStaticOrCurve int32 = 0 // C#里的 StaticOrCurve，静态或曲线模式
	PartialModePartial       int32 = 1 // C#里的 Partial，按骨骼设置模式
	PartialModeFromBoneName  int32 = 2 // C#里的 FromBoneName，旧自动按骨骼名设置模式
)

// FreezeAxis 枚举
const (
	FreezeAxisNone int32 = 0
	FreezeAxisX    int32 = 1
	FreezeAxisY    int32 = 2
	FreezeAxisZ    int32 = 3
)

// BoneValue 存储一个骨骼名称与对应 float 值
type BoneValue struct {
	BoneName string  `json:"BoneName"`
	Value    float32 `json:"Value"`
}

// ReadPhy 读取 "CM3D21_PHY" 格式
func ReadPhy(r io.Reader) (*Phy, error) {
	p := &Phy{}

	reader := stream.NewBinaryReader(r)

	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read signature failed: %w", err)
	}
	//if sig != PhySignature {
	//	return nil, fmt.Errorf("invalid phy signature, want %v, got %q", PhySignature, sig)
	//}
	p.Signature = sig

	// 2. Version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read version failed: %w", err)
	}
	p.Version = ver

	// 3. RootName
	rootName, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read rootName failed: %w", err)
	}
	p.RootName = rootName

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

	// 9. EndLength, EndOffset (x,y,z)
	p.EndLength, err = reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read EndLength failed: %w", err)
	}
	// EndOffset
	for i := 0; i < 3; i++ {
		p.EndOffset[i], err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("read EndOffset[%d] failed: %w", i, err)
		}
	}

	// 10.  Gravity (x,y,z), Force (x,y,z)
	for i := 0; i < 3; i++ {
		p.Gravity[i], err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("read Gravity[%d] failed: %w", i, err)
		}
	}
	// Force
	for i := 0; i < 3; i++ {
		p.Force[i], err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("read Force[%d] failed: %w", i, err)
		}
	}

	// 11. ColliderFileName
	cfn, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read ColliderFileName failed: %w", err)
	}
	p.ColliderFileName = cfn

	// 12. CollidersCount
	colCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read CollidersCount failed: %w", err)
	}
	p.CollidersCount = colCount

	// 虽然 C# 记录了 CollidersCount，但并没有写任何内容
	// 目前记录 CollidersCount 只是为了初始化列表

	// 13. ExclusionsCount
	excCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read ExclusionsCount failed: %w", err)
	}
	p.ExclusionsCount = excCount

	// 同样，C# 只写了数量，没有写任何内容
	// 猜测此功能已弃用
	// 目前记录 ExclusionsCount 只是为了初始化列表

	// 13. FreezeAxis
	fa, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read freezeAxis failed: %w", err)
	}
	p.FreezeAxis = fa

	return p, nil
}

// Dump 写出 "CM3D21_PHY" 格式
func (p *Phy) Dump(w io.Writer) error {
	writer := stream.NewBinaryWriter(w)

	// 1. Signature
	if err := writer.WriteString(p.Signature); err != nil {
		return fmt.Errorf("write signature failed: %w", err)
	}
	// 2. Version
	if err := writer.WriteInt32(p.Version); err != nil {
		return fmt.Errorf("write version failed: %w", err)
	}
	// 3. RootName
	if err := writer.WriteString(p.RootName); err != nil {
		return fmt.Errorf("write rootName failed: %w", err)
	}

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

	// 9. EndLength
	if err := writer.WriteFloat32(p.EndLength); err != nil {
		return fmt.Errorf("write EndLength failed: %w", err)
	}
	// 10. EndOffset (x, y, z)
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(p.EndOffset[i]); err != nil {
			return fmt.Errorf("write EndOffset[%d] failed: %w", i, err)
		}
	}
	// 11. Gravity (x, y, z)
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(p.Gravity[i]); err != nil {
			return fmt.Errorf("write Gravity[%d] failed: %w", i, err)
		}
	}
	// 12. Force (x, y, z)
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(p.Force[i]); err != nil {
			return fmt.Errorf("write Force[%d] failed: %w", i, err)
		}
	}

	// 13. ColliderFileName
	if err := writer.WriteString(p.ColliderFileName); err != nil {
		return fmt.Errorf("write ColliderFileName failed: %w", err)
	}

	// 14. CollidersCount
	if err := writer.WriteInt32(p.CollidersCount); err != nil {
		return fmt.Errorf("write CollidersCount failed: %w", err)
	}

	// 虽然 C# 记录了 CollidersCount，但并没有写任何内容，所以这里直接略过
	// 猜测因为以前 phy 和 col 是合并的
	// 但是碰撞器有自己的格式 col，所以 phy 内不写出 col 的内容
	// 记录 CollidersCount 只是为了初始化列表

	// 15. ExclusionsCount
	if err := writer.WriteInt32(p.ExclusionsCount); err != nil {
		return fmt.Errorf("write ExclusionsCount failed: %w", err)
	}
	// 同样，C# 只写了数量，没有写任何内容
	// 猜测此功能已弃用
	// 记录 ExclusionsCount 只是为了初始化列表

	// 16. FreezeAxis
	if err := writer.WriteInt32(p.FreezeAxis); err != nil {
		return fmt.Errorf("write freezeAxis failed: %w", err)
	}

	return nil
}

// readPartial 读取：
//
//	int(PartialMode) -> 如果 != PartialModePartial, 结束；
//	int(boneCount) -> 循环读取 boneName + floatValue
func readPartial(reader *stream.BinaryReader) (int32, []BoneValue, error) {
	mode, err := reader.ReadInt32() // 读取 PartialMode，对应 PartialMode 枚举
	if err != nil {
		return 0, nil, fmt.Errorf("read partialMode failed: %w", err)
	}
	if mode != PartialModePartial { // 如果不是 PartialModePartial 部位模式，直接返回
		return mode, nil, nil
	}

	count, err := reader.ReadInt32() // 读取骨骼数量
	if err != nil {
		return mode, nil, fmt.Errorf("read partial count failed: %w", err)
	}

	vals := make([]BoneValue, count)
	for i := 0; i < int(count); i++ { // 循环读取骨骼名称和对应 float 值
		bn, err := reader.ReadString() // 读取骨骼名称
		if err != nil {
			return mode, nil, fmt.Errorf("read boneName failed: %w", err)
		}
		fv, err := reader.ReadFloat32() // 读取对应 float 值
		if err != nil {
			return mode, nil, fmt.Errorf("read boneValue failed: %w", err)
		}
		vals[i] = BoneValue{BoneName: bn, Value: fv} // 存储到切片中
	}
	return mode, vals, nil
}

// writePartial 写出：
//
//	int(PartialMode) -> 如果 == PartialModePartial 再写 (count + boneName + floatValue * count)
func writePartial(writer *stream.BinaryWriter, mode int32, values []BoneValue) error {
	if err := writer.WriteInt32(mode); err != nil {
		return fmt.Errorf("write partialMode failed: %w", err)
	}
	if mode != PartialModePartial {
		return nil
	}

	count := int32(len(values))
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
