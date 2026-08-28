package COM3D2

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

// CM3D21_COL
// 碰撞器文件，用于描述模型的碰撞器
//
// 无版本差异
// CM3D21_COL
// Collider file used to describe model colliders
//
// There are no version differences

// -------------------------------------------------------
// 定义 Col (ColliderFile) 的数据结构
// Define the data structure of Col (ColliderFile)
// -------------------------------------------------------

// Col 表示一个 CM3D21_COL 文件
// Col represents a CM3D21_COL file
type Col struct {
	Signature string      `json:"Signature"` // CM3D21_COL 文件签名 / CM3D21_COL file signature
	Version   int32       `json:"Version"`   // 此版本随每次更新变化，但结构不变 / This version changes with each update without changing the structure
	Colliders []ICollider `json:"Colliders"` // 碰撞器列表 / Collider list
}

// -------------------------------------------------------
// 读取 Col
// Read Col
// -------------------------------------------------------

// ReadCol 从二进制流里读取一个 Col
// ReadCol reads a Col from a binary stream
func ReadCol(r io.Reader) (*Col, error) {
	file := &Col{}

	reader := stream.NewBinaryReader(r)

	// 1. 签名
	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read signature failed: %w", err)
	}
	// if sig != ColSignature {
	// 	return nil, fmt.Errorf("invalid col signature, want %v, got %s", ColSignature, sig)
	// }
	file.Signature = sig

	// 2. 版本
	// 2. Version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read version failed: %w", err)
	}
	file.Version = ver

	// 3. 碰撞器数量
	// 3. Collider count
	count, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read collider count failed: %w", err)
	}
	if err := validateNonNegativeCount("collider count", count); err != nil {
		return nil, err
	}

	// 4. 逐个读取 Collider
	// 4. Read each Collider
	file.Colliders = makeCountedSliceForAppend[ICollider](count)
	for i := int32(0); i < count; i++ {
		typeName, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("read collider type string failed at index %d: %w", i, err)
		}

		var collider ICollider
		switch typeName {
		case "dbc":
			collider = &DynamicBoneCollider{}
		case "dpc":
			collider = &DynamicBonePlaneCollider{}
		case "dbm":
			collider = &DynamicBoneMuneCollider{}
		case "missing":
			collider = &MissingCollider{}
		default:
			return nil, fmt.Errorf("unrecognized collider type %q at index %d", typeName, i)
		}

		if err := collider.Read(reader, ver); err != nil {
			return nil, fmt.Errorf("collider.Read failed at index %d: %w", i, err)
		}
		file.Colliders = append(file.Colliders, collider)
	}

	return file, nil
}

// -------------------------------------------------------
// 写出 Col
// Write Col
// -------------------------------------------------------

// Dump 把 Col 写出到 w 中
// Dump writes Col to w
func (c *Col) Dump(w io.Writer) error {
	if c == nil {
		return fmt.Errorf("nil collider file")
	}
	count, err := collectionCountInt32("collider count", int64(len(c.Colliders)))
	if err != nil {
		return err
	}
	for index := int64(0); index < int64(len(c.Colliders)); index++ {
		collider := c.Colliders[index]
		if err := validateColliderForDump(index, collider); err != nil {
			return err
		}
	}
	writer := stream.NewBinaryWriter(w)

	// 1. 写 Signature
	// 1. Write Signature
	if err := writer.WriteString(c.Signature); err != nil {
		return fmt.Errorf("write signature failed: %w", err)
	}
	// 2. 写 Version
	// 2. Write Version
	if err := writer.WriteInt32(c.Version); err != nil {
		return fmt.Errorf("write version failed: %w", err)
	}
	// 3. 写 Collider count
	// 3. Write Collider count
	if err := writer.WriteInt32(count); err != nil {
		return fmt.Errorf("write collider count failed: %w", err)
	}
	// 4. 遍历写出每个 collider
	// 4. Write each collider
	for i := int64(0); i < int64(len(c.Colliders)); i++ {
		collider := c.Colliders[i]
		typeName := collider.GetTypeName()
		// 先写 typeName
		// Write typeName first
		if err := writer.WriteString(typeName); err != nil {
			return fmt.Errorf("write collider type failed at index %d: %w", i, err)
		}
		// 写具体数据
		// Write the concrete data
		if err := collider.Write(writer, c.Version); err != nil {
			return fmt.Errorf("collider.Write failed at index %d: %w", i, err)
		}
	}
	return nil
}

// validateColliderForDump 拒绝 nil、缺少基类数据或不是游戏支持类型的碰撞体，避免写出部分记录
// validateColliderForDump rejects nil colliders, missing base data, and types unsupported by the game so partial records are never written
func validateColliderForDump(index int64, collider ICollider) error {
	path := fmt.Sprintf("Colliders[%d]", index)
	switch value := collider.(type) {
	case *DynamicBoneCollider:
		if value == nil {
			return fmt.Errorf("%s is nil", path)
		}
		if value.Base == nil {
			return fmt.Errorf("%s.Base is nil", path)
		}
	case *DynamicBonePlaneCollider:
		if value == nil {
			return fmt.Errorf("%s is nil", path)
		}
		if value.Base == nil {
			return fmt.Errorf("%s.Base is nil", path)
		}
	case *DynamicBoneMuneCollider:
		if value == nil {
			return fmt.Errorf("%s is nil", path)
		}
		if value.Base == nil {
			return fmt.Errorf("%s.Base is nil", path)
		}
	case *MissingCollider:
		if value == nil {
			return fmt.Errorf("%s is nil", path)
		}
	case nil:
		return fmt.Errorf("%s is nil", path)
	default:
		return fmt.Errorf("%s has unsupported type %T", path, collider)
	}
	return nil
}

// ICollider 是所有Collider的接口，不同具体类型各自实现
// 注意在每个 struct 中保存 TypeName 是故意的，否则前端类型推断困难，实际不写入二进制
// ICollider is the interface implemented by every collider type
// TypeName is deliberately retained in each struct because frontend type inference would otherwise be difficult, but it is not written in the concrete binary payload
type ICollider interface {
	// GetTypeName 返回游戏外层记录使用的短类型标记
	// GetTypeName returns the short type marker used by the game's outer record
	GetTypeName() string
	// Read 从当前位置读取该具体类型的载荷
	// Read reads this concrete type's payload from the current position
	Read(reader *stream.BinaryReader, version int32) error
	// Write 将该具体类型的载荷写到当前位置
	// Write writes this concrete type's payload at the current position
	Write(writer *stream.BinaryWriter, version int32) error
}

// -------------------------------------------------------
// Collider 类型
// Collider types
// -------------------------------------------------------

// DynamicBoneColliderBase 基类
// DynamicBoneColliderBase is the base class
type DynamicBoneColliderBase struct {
	TypeName      string     `json:"TypeName"  default:"base"` // 碰撞器类型，仅标记，不序列化 "base" / Collider type marker only, with "base" not serialized
	ParentName    string     `json:"ParentName"`               // 父级 Transform （骨骼）名称 / Parent Transform or bone name
	SelfName      string     `json:"SelfName"`                 // 当前 Transform 名称 / Current Transform name
	LocalPosition [3]float32 `json:"LocalPosition"`            // 局部坐标系中的位置 (x,y,z) / Position in local coordinates (x,y,z)
	LocalRotation [4]float32 `json:"LocalRotation"`            // 局部坐标系中的旋转 (四元数) / Rotation in local coordinates as a quaternion
	LocalScale    [3]float32 `json:"LocalScale"`               // 局部坐标系中的缩放 (x,y,z) / Scale in local coordinates (x,y,z)

	Direction int32      `json:"Direction"` // 碰撞体方向，指定哪一个轴是胶囊碰撞器的高(0=x, 1=y, 2=z) / Collider direction selecting the capsule height axis (0=x, 1=y, 2=z)
	Center    [3]float32 `json:"Center"`    // 碰撞体中心偏移 / Collider center offset
	Bound     int32      `json:"Bound"`     // 碰撞约束边界类型 (0=Outside, 1=Inside) / Collision constraint bound type (0=Outside, 1=Inside)
}

// GetTypeName 返回仅供 JSON 基类表示使用的 base 标记，游戏 C# 基类的 TypeName 实际返回空字符串
// GetTypeName returns the base marker used only for the JSON base representation; the game C# base class actually returns an empty TypeName
func (base *DynamicBoneColliderBase) GetTypeName() string {
	return "base"
}

// Read 按 DynamicBoneColliderBase.Deserialize 的字段顺序读取公共碰撞体数据
// Read reads common collider data in the field order of DynamicBoneColliderBase.Deserialize
func (base *DynamicBoneColliderBase) Read(reader *stream.BinaryReader, version int32) error {
	base.TypeName = base.GetTypeName()

	var err error

	// 1. 父级名称
	// 1. ParentName
	base.ParentName, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read parentName failed: %w", err)
	}

	// 2. 当前名称
	// 2. SelfName
	base.SelfName, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read selfName failed: %w", err)
	}

	// 3. 局部位置
	// 3. LocalPosition
	for i := 0; i < 3; i++ {
		base.LocalPosition[i], err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read localPosition[%d] failed: %w", i, err)
		}
	}

	// 4. 局部旋转
	// 4. LocalRotation
	for i := 0; i < 4; i++ {
		base.LocalRotation[i], err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read localRotation[%d] failed: %w", i, err)
		}
	}

	// 5. 局部缩放
	// 5. LocalScale
	for i := 0; i < 3; i++ {
		base.LocalScale[i], err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read localScale[%d] failed: %w", i, err)
		}
	}

	// 6. 方向
	// 6. Direction
	base.Direction, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read direction failed: %w", err)
	}

	// 7. 中心 (x,y,z)
	// 7. Center (x,y,z)
	for i := 0; i < 3; i++ {
		base.Center[i], err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read center[%d] failed: %w", i, err)
		}
	}

	// 8. 边界
	// 8. Bound
	base.Bound, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read bound failed: %w", err)
	}

	return nil
}

// Write 按 DynamicBoneColliderBase.Serialize 的字段顺序写入公共碰撞体数据
// Write writes common collider data in the field order of DynamicBoneColliderBase.Serialize
func (base *DynamicBoneColliderBase) Write(writer *stream.BinaryWriter, version int32) error {
	// 1. 父级名称
	// 1. ParentName
	if err := writer.WriteString(base.ParentName); err != nil {
		return fmt.Errorf("write parentName failed: %w", err)
	}
	// 2. 当前名称
	// 2. SelfName
	if err := writer.WriteString(base.SelfName); err != nil {
		return fmt.Errorf("write selfName failed: %w", err)
	}

	// 3. 局部位置
	// 3. LocalPosition
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(base.LocalPosition[i]); err != nil {
			return fmt.Errorf("write localPosition[%d] failed: %w", i, err)
		}
	}

	// 4. 局部旋转
	// 4. LocalRotation
	for i := 0; i < 4; i++ {
		if err := writer.WriteFloat32(base.LocalRotation[i]); err != nil {
			return fmt.Errorf("write localRotation[%d] failed: %w", i, err)
		}
	}

	// 5. 局部缩放
	// 5. LocalScale
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(base.LocalScale[i]); err != nil {
			return fmt.Errorf("write localScale[%d] failed: %w", i, err)
		}
	}

	// 6. 方向
	// 6. Direction
	if err := writer.WriteInt32(base.Direction); err != nil {
		return fmt.Errorf("write direction failed: %w", err)
	}

	// 7. 中心
	// 7. Center
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(base.Center[i]); err != nil {
			return fmt.Errorf("write center[%d] failed: %w", i, err)
		}
	}

	// 8. 边界
	// 8. Bound
	if err := writer.WriteInt32(base.Bound); err != nil {
		return fmt.Errorf("write bound failed: %w", err)
	}

	return nil
}

// DynamicBoneCollider 对应 "dbc"
// DynamicBoneCollider corresponds to "dbc"
type DynamicBoneCollider struct {
	TypeName string                   `json:"TypeName" default:"dbc"` // 碰撞器类型，仅标记，不序列化 "dbc" / Collider type marker only, with "dbc" not serialized in the concrete payload
	Base     *DynamicBoneColliderBase `json:"Base"`                   // 基类 / Base class

	Radius float32 `json:"Radius"` // 碰撞器半径 / Collider radius
	Height float32 `json:"Height"` // 碰撞器高度 / Collider height
}

// GetTypeName 返回 DynamicBoneCollider 的游戏类型标记 dbc
// GetTypeName returns the game type marker dbc for DynamicBoneCollider
func (dbc *DynamicBoneCollider) GetTypeName() string {
	return "dbc"
}

// Read 读取 DynamicBoneCollider 数据
// Read reads DynamicBoneCollider data
func (dbc *DynamicBoneCollider) Read(reader *stream.BinaryReader, version int32) error {
	dbc.TypeName = dbc.GetTypeName()

	// 先读基类字段
	// Read the base fields first
	baseData := DynamicBoneColliderBase{}
	err := baseData.Read(reader, version)
	if err != nil {
		return fmt.Errorf("read base collider failed: %w", err)
	}
	dbc.Base = &baseData

	// 读 2 个 Float32
	// Read 2 Float32 values
	radius, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read m_Radius failed: %w", err)
	}
	height, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read m_Height failed: %w", err)
	}
	dbc.Radius = radius
	dbc.Height = height

	return nil
}

// Write 写入 DynamicBoneCollider 数据
// Write writes DynamicBoneCollider data
func (dbc *DynamicBoneCollider) Write(writer *stream.BinaryWriter, version int32) error {
	// 先写基类字段
	// Write the base fields first
	err := dbc.Base.Write(writer, version)
	if err != nil {
		return err
	}

	// 写 2 个 Float32
	// Write 2 Float32 values
	if err := writer.WriteFloat32(dbc.Radius); err != nil {
		return fmt.Errorf("write m_Radius failed: %w", err)
	}

	if err := writer.WriteFloat32(dbc.Height); err != nil {
		return fmt.Errorf("write m_Height failed: %w", err)
	}
	return nil
}

// DynamicBonePlaneCollider 对应 "dpc"
// 在 C# 中并无其它独立字段，只继承基类
// DynamicBonePlaneCollider corresponds to "dpc"
// It has no independent fields in C# and only inherits the base class
type DynamicBonePlaneCollider struct {
	TypeName string                   `json:"TypeName" default:"dpc"` // 碰撞器类型，仅标记，不序列化 "dpc" / Collider type marker only, with "dpc" not serialized in the concrete payload
	Base     *DynamicBoneColliderBase `json:"Base"`                   // 基类 / Base class
}

// GetTypeName 返回 DynamicBonePlaneCollider 的游戏类型标记 dpc
// GetTypeName returns the game type marker dpc for DynamicBonePlaneCollider
func (dpc *DynamicBonePlaneCollider) GetTypeName() string {
	return "dpc"
}

// Read 读取 DynamicBonePlaneCollider 数据
// Read reads DynamicBonePlaneCollider data
func (dpc *DynamicBonePlaneCollider) Read(reader *stream.BinaryReader, version int32) error {
	dpc.TypeName = dpc.GetTypeName()

	// 只有基类字段
	// There are only base fields
	baseData := DynamicBoneColliderBase{}
	err := baseData.Read(reader, version)
	if err != nil {
		return fmt.Errorf("read base collider for dpc failed: %w", err)
	}
	dpc.Base = &baseData

	return nil
}

// Write 写入 DynamicBonePlaneCollider 数据
// Write writes DynamicBonePlaneCollider data
func (dpc *DynamicBonePlaneCollider) Write(writer *stream.BinaryWriter, version int32) error {
	// 只有基类字段
	// There are only base fields
	if err := dpc.Base.Write(writer, version); err != nil {
		return fmt.Errorf("write base collider for dpc failed: %w", err)
	}

	return nil
}

// DynamicBoneMuneCollider 对应 "dbm"
// DynamicBoneMuneCollider corresponds to "dbm"
type DynamicBoneMuneCollider struct {
	TypeName string                   `json:"TypeName" default:"dbm"` // 碰撞器类型，仅标记，不序列化 "dbm" / Collider type marker only, with "dbm" not serialized in the concrete payload
	Base     *DynamicBoneColliderBase `json:"Base"`                   // 基类 / Base class

	Radius          float32    `json:"Radius"`          // 碰撞器半径 / Collider radius
	Height          float32    `json:"Height"`          // 碰撞器高度 / Collider height
	ScaleRateMulMax float32    `json:"ScaleRateMulMax"` // 最大缩放倍率 / Maximum scale multiplier
	CenterRateMax   [3]float32 `json:"CenterRateMax"`   // 最大中心偏移(x,y,z) / Maximum center offset (x,y,z)
}

// GetTypeName 返回 DynamicBoneMuneCollider 的游戏类型标记 dbm
// GetTypeName returns the game type marker dbm for DynamicBoneMuneCollider
func (c *DynamicBoneMuneCollider) GetTypeName() string {
	return "dbm"
}

// Read 读取 DynamicBoneMuneCollider 数据
// Read reads DynamicBoneMuneCollider data
func (c *DynamicBoneMuneCollider) Read(reader *stream.BinaryReader, version int32) error {
	c.TypeName = c.GetTypeName()

	// 先读基类字段
	// Read the base fields first
	baseData := DynamicBoneColliderBase{}
	err := baseData.Read(reader, version)
	if err != nil {
		return fmt.Errorf("read base collider for dbm failed: %w", err)
	}
	c.Base = &baseData

	radius, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read m_Radius failed: %w", err)
	}
	c.Radius = radius

	height, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read m_Height failed: %w", err)
	}
	c.Height = height

	scaleRateMulMax, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read m_fScaleRateMulMax failed: %w", err)
	}
	c.ScaleRateMulMax = scaleRateMulMax

	var centerRateMax [3]float32
	for i := 0; i < 3; i++ {
		centerRateMax[i], err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read m_CenterRateMax[%d] failed: %w", i, err)
		}
	}
	c.CenterRateMax = centerRateMax

	return nil
}

// Write 写入 DynamicBoneMuneCollider 数据
// Write writes DynamicBoneMuneCollider data
func (c *DynamicBoneMuneCollider) Write(writer *stream.BinaryWriter, version int32) error {
	// 写基类字段
	// Write the base fields
	if err := c.Base.Write(writer, version); err != nil {
		return fmt.Errorf("write base collider failed: %w", err)
	}

	// 写 2 个 Float32
	// Write 2 Float32 values
	if err := writer.WriteFloat32(c.Radius); err != nil {
		return fmt.Errorf("write m_Radius failed: %w", err)
	}

	if err := writer.WriteFloat32(c.Height); err != nil {
		return fmt.Errorf("write m_Height failed: %w", err)
	}

	// 写 1 个 Float32
	// Write 1 Float32 value
	if err := writer.WriteFloat32(c.ScaleRateMulMax); err != nil {
		return fmt.Errorf("write m_fScaleRateMulMax failed: %w", err)
	}

	// 写 3 个 Float32
	// Write 3 Float32 values
	for i := 0; i < 3; i++ {
		if err := writer.WriteFloat32(c.CenterRateMax[i]); err != nil {
			return fmt.Errorf("write m_CenterRateMax[%d] failed: %w", i, err)
		}
	}

	return nil
}

// MissingCollider 对应 "missing"
// MissingCollider corresponds to "missing"
type MissingCollider struct {
	TypeName string `json:"TypeName" default:"missing"` // 碰撞器类型，仅标记，不序列化 "missing" / Collider type marker only, with "missing" not serialized
}

// GetTypeName 返回无载荷碰撞体的游戏类型标记 missing
// GetTypeName returns the game type marker missing for a payload-less collider
func (m *MissingCollider) GetTypeName() string {
	return "missing"
}

// Read 读取 MissingCollider 数据
// Read reads MissingCollider data
func (m *MissingCollider) Read(reader *stream.BinaryReader, version int32) error {
	m.TypeName = m.GetTypeName()
	// "missing" 字段什么都不做，typeName 已经在外层写了
	// The "missing" field does nothing because typeName was already written by the outer record
	return nil
}

// Write 写入 MissingCollider 数据
// Write writes MissingCollider data
func (m *MissingCollider) Write(writer *stream.BinaryWriter, version int32) error {
	// "missing" 字段什么都不做
	// The "missing" field does nothing
	return nil
}

// UnmarshalJSON 为 Col 实现自定义 UnmarshalJSON
// 因为 Col 的 ICollider 字段是一个接口切片，需要根据 typeName 字段来决定反序列化为哪个具体类型
// UnmarshalJSON implements custom JSON unmarshalling for Col
// Col contains an ICollider interface slice, so typeName determines the concrete type used for deserialization
func (c *Col) UnmarshalJSON(data []byte) error {
	// 先定义一个中间结构来接住 Colliders 的原始数据
	// 其他字段 Signature 和 Version 可以直接接收
	// First define an intermediate structure to hold the raw Colliders data
	// Other fields such as Signature and Version can be received directly
	type colAlias Col
	var temp struct {
		Colliders []json.RawMessage `json:"Colliders"` // 未分派的碰撞体 JSON / Undispatched collider JSON
		*colAlias                   // Col 的其余字段 / Remaining fields of Col
	}
	temp.colAlias = (*colAlias)(c)

	// 先把大部分字段 (Signature, Version) 解析出来
	// First parse most fields such as Signature and Version
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// 此时 c.Signature 和 c.Version 已经有值了
	// 逐个解析 Colliders
	// At this point c.Signature and c.Version already have values
	// Parse Colliders one by one
	var result []ICollider
	for _, raw := range temp.Colliders {
		// 1. 先解析出 TypeName 用来分辨子类型
		// 1. Parse TypeName first to distinguish the subtype
		var typeHolder struct {
			TypeName string `json:"TypeName"` // JSON 类型判别标记 / JSON type discriminator
		}
		if err := json.Unmarshal(raw, &typeHolder); err != nil {
			return err
		}

		// 2. 根据 TypeName 创建对应的 collider 实例
		// 2. Create the corresponding collider instance from TypeName
		var collider ICollider
		switch typeHolder.TypeName {
		case "dbc":
			collider = &DynamicBoneCollider{}
		case "dpc":
			collider = &DynamicBonePlaneCollider{}
		case "dbm":
			collider = &DynamicBoneMuneCollider{}
		case "missing":
			collider = &MissingCollider{}
		default:
			return fmt.Errorf("unrecognized collider TypeName: %q", typeHolder.TypeName)
		}

		// 3. 用创建好的实例再去解析整个 JSON
		// 3. Parse the complete JSON using the created instance
		if err := json.Unmarshal(raw, collider); err != nil {
			return err
		}
		result = append(result, collider)
	}

	// 全部解析完毕，赋值给真实字段
	// Assign all parsed values to the actual field
	c.Colliders = result
	return nil
}
