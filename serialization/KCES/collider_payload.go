package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// .dbcol、.dslcol、.limbcol 与 .ikcol 系列共用的碰撞体结构、union 编码和校验逻辑
// Collider structures, union encoding, and validation shared by .dbcol, .dslcol, .limbcol, and .ikcol families

// ColliderPackage 表示通用碰撞体包
// ColliderPackage represents a generic collider package
type ColliderPackage struct {
	_struct                struct{}        `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`     // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int             `json:"version"`                  // 版本号 / Version value
	Colliders              []ColliderRef   `json:"colliders"`                // 碰撞体引用列表 / Collider reference list
	LimbEnableList         []ColliderState `json:"limbEnableList,omitempty"` // DynamicYureBone.LimbColliderInfo 列表 / DynamicYureBone.LimbColliderInfo list
}

// colliderPackageJSON 表示 ColliderPackage 的 JSON 兼容视图
// colliderPackageJSON represents the JSON compatibility view of ColliderPackage
type colliderPackageJSON struct {
	*IndexedObjectMetadata                 // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int             `json:"version"`                  // 版本号 / Version value
	Colliders              []ColliderRef   `json:"colliders"`                // 碰撞体引用列表 / Collider reference list
	LimbEnableList         []ColliderState `json:"limbEnableList,omitempty"` // limb 启用状态列表 / Limb enable-state list
	States                 json.RawMessage `json:"states,omitempty"`         // 仅用于拒绝旧版 states 字段 / Used only to reject the legacy states field
}

// MarshalJSON 将 ColliderPackage 编码为当前 JSON 字段名
// MarshalJSON encodes ColliderPackage using the current JSON field names
func (p ColliderPackage) MarshalJSON() ([]byte, error) {
	return json.Marshal(colliderPackageJSON{
		IndexedObjectMetadata: p.IndexedObjectMetadata,
		Version:               p.Version,
		Colliders:             p.Colliders,
		LimbEnableList:        p.LimbEnableList,
	})
}

// UnmarshalJSON 解码 ColliderPackage 并拒绝已移除的 states 字段
// UnmarshalJSON decodes ColliderPackage and rejects the removed states field
func (p *ColliderPackage) UnmarshalJSON(data []byte) error {
	var raw colliderPackageJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.States) > 0 {
		return fmt.Errorf(`colliderPackage.states is no longer supported; use "limbEnableList"`)
	}
	*p = ColliderPackage{
		IndexedObjectMetadata: raw.IndexedObjectMetadata,
		Version:               raw.Version,
		Colliders:             raw.Colliders,
		LimbEnableList:        raw.LimbEnableList,
	}
	return nil
}

// ColliderType 映射（与 C# 的 ANativeColliderStatus.ColliderType 一致）
// ColliderType mapping (same as ANativeColliderStatus.ColliderType)
const (
	ColliderTypePlane    = 0
	ColliderTypeCapsule  = 1
	ColliderTypeSphere   = 2
	ColliderTypeMaidProp = 3
)

// VectorType 映射（与 Scourt.Utility.MathUtility.VectorType 一致）
// VectorType mapping (same as Scourt.Utility.MathUtility.VectorType)
const (
	VectorTypeX = 0
	VectorTypeY = 1
	VectorTypeZ = 2
)

const (
	ColliderBoundOutside = 0
	ColliderBoundInside  = 1
)

// ColliderRef 表示带 union 类型枚举的碰撞体引用
// ColliderRef represents a collider reference with its union type enum
type ColliderRef struct {
	*IndexedObjectMetadata `codec:"-"`         // union 封套的线格式元数据 / Wire metadata for the union wrapper
	Type                   int                 `json:"type"`                            // 碰撞体类型枚举 / Collider type enum
	Collider               ColliderStatusUnion `json:"collider"`                        // 碰撞体对象数据 / Collider object data
	ColliderRaw            RawMessagePackSlot  `json:"colliderRaw,omitempty" codec:"-"` // Type 新于本库时逐字节保留的完整 union 载荷 / Complete union payload preserved byte-for-byte when Type is newer than this library
}

// colliderRefAlias 避免 ColliderRef.MarshalJSON 递归调用自身
// colliderRefAlias prevents ColliderRef.MarshalJSON from recursively calling itself
type colliderRefAlias ColliderRef

// colliderRefJSON 表示可区分强类型载荷与原始未来载荷的 JSON 输入
// colliderRefJSON represents JSON input that distinguishes a typed payload from a raw future payload
type colliderRefJSON struct {
	*IndexedObjectMetadata                    // union 封套的线格式元数据 / Wire metadata for the union wrapper
	Type                   int                `json:"type"`                  // union 类型标记 / Union type tag
	Collider               json.RawMessage    `json:"collider"`              // 强类型碰撞体 JSON / Typed collider JSON
	ColliderRaw            RawMessagePackSlot `json:"colliderRaw,omitempty"` // 未知类型的原始 MessagePack 载荷 / Raw MessagePack payload for an unknown type
}

// UnmarshalJSON 按 Type 解码已知碰撞体或保留未知类型原始载荷
// UnmarshalJSON decodes a known collider according to Type or preserves an unknown raw payload
func (c *ColliderRef) UnmarshalJSON(data []byte) error {
	var raw colliderRefJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.ColliderRaw) != 0 && !jsonMessageIsNullOrMissing(raw.Collider) {
		return fmt.Errorf("collider and colliderRaw cannot both be populated")
	}
	var collider ColliderStatusUnion
	if len(raw.ColliderRaw) == 0 && !jsonMessageIsNullOrMissing(raw.Collider) {
		var err error
		collider, err = decodeColliderObjectAsType(raw.Collider, raw.Type)
		if err != nil {
			return err
		}
	}
	*c = ColliderRef{
		IndexedObjectMetadata: raw.IndexedObjectMetadata,
		Type:                  raw.Type,
		Collider:              collider,
		ColliderRaw:           cloneRawMessagePackSlot(raw.ColliderRaw),
	}
	return nil
}

// MarshalJSON 编码碰撞体引用及其强类型或原始载荷
// MarshalJSON encodes a collider reference and its typed or raw payload
func (c ColliderRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(colliderRefAlias(c))
}

// ColliderStatusUnion 表示强类型碰撞体状态可实现的 union 接口
// ColliderStatusUnion represents the union interface implemented by typed collider states
type ColliderStatusUnion interface {
	toColliderType() int
}

// ColliderObject 表示游戏 ANativeColliderStatus 的共用基类字段
// ColliderObject represents shared base fields from the game's ANativeColliderStatus
type ColliderObject struct {
	Version       int     `json:"version"`       // 版本号 / Version value
	ParentName    string  `json:"parentName"`    // 父对象名称 / Parent object name
	SelfName      string  `json:"selfName"`      // 自身对象名称 / Own object name
	LocalPosition Vector3 `json:"localPosition"` // 本地位置 / Local position
	LocalRotation Vector4 `json:"localRotation"` // 本地旋转四元数 / Local rotation quaternion
	LocalScale    Vector3 `json:"localScale"`    // 本地缩放 / Local scale
	Center        Vector3 `json:"center"`        // 碰撞体中心 / Collider center
	Bound         int     `json:"bound"`         // 修正方式：0=Outside,1=Inside / Correction mode: 0=Outside,1=Inside
}

// ColliderPlane 对应游戏 NativePlaneColliderStatus
// ColliderPlane corresponds to the game's NativePlaneColliderStatus
type ColliderPlane struct {
	_struct                struct{}          `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`       // 索引对象的线格式元数据 / Indexed-object wire metadata
	ColliderObject         `codec:",inline"` // ANativeColliderStatus 基类字段 / ANativeColliderStatus base fields
	Direction              int               `json:"direction"`          // 平面法线方向 / Plane normal direction
	IsDirectionInverse     bool              `json:"isDirectionInverse"` // 法线方向反转 / Reverse normal direction
}

// toColliderType 返回 Plane union 类型标记
// toColliderType returns the Plane union type tag
func (*ColliderPlane) toColliderType() int { return ColliderTypePlane }

// ColliderCapsule 对应游戏 NativeCapsuleColliderStatus
// ColliderCapsule corresponds to the game's NativeCapsuleColliderStatus
type ColliderCapsule struct {
	_struct                struct{}          `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`       // 索引对象的线格式元数据 / Indexed-object wire metadata
	ColliderObject         `codec:",inline"` // ANativeColliderStatus 基类字段 / ANativeColliderStatus base fields
	Direction              int               `json:"direction"`          // 胶囊主轴方向 / Capsule axis direction
	IsDirectionInverse     bool              `json:"isDirectionInverse"` // 方向反转 / Direction reversed
	StartRadius            float32           `json:"startRadius"`        // 起点半径 / Start radius
	EndRadius              float32           `json:"endRadius"`          // 终点半径 / End radius
	Height                 float32           `json:"height"`             // 长度 / Height
}

// toColliderType 返回 Capsule union 类型标记
// toColliderType returns the Capsule union type tag
func (*ColliderCapsule) toColliderType() int { return ColliderTypeCapsule }

// ColliderSphere 对应游戏 NativeSphereColliderStatus
// ColliderSphere corresponds to the game's NativeSphereColliderStatus
type ColliderSphere struct {
	_struct                struct{}          `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`       // 索引对象的线格式元数据 / Indexed-object wire metadata
	ColliderObject         `codec:",inline"` // ANativeColliderStatus 基类字段 / ANativeColliderStatus base fields
	Radius                 float32           `json:"radius"` // 半径 / Radius
}

// toColliderType 返回 Sphere union 类型标记
// toColliderType returns the Sphere union type tag
func (*ColliderSphere) toColliderType() int { return ColliderTypeSphere }

// ColliderMaidProp 对应游戏 NativeMaidPropColliderStatus
// ColliderMaidProp corresponds to the game's NativeMaidPropColliderStatus
type ColliderMaidProp struct {
	_struct                struct{}           `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`        // 索引对象的线格式元数据 / Indexed-object wire metadata
	ColliderObject         `codec:",inline"`  // NativeCapsuleColliderStatus 与基类字段 / NativeCapsuleColliderStatus and base fields
	Direction              int                `json:"direction"`              // 胶囊主轴方向 / Capsule axis direction
	IsDirectionInverse     bool               `json:"isDirectionInverse"`     // 方向反转 / Direction reversed
	StartRadius            float32            `json:"startRadius"`            // 起点半径 / Start radius
	EndRadius              float32            `json:"endRadius"`              // 终点半径 / End radius
	Height                 float32            `json:"height"`                 // 长度 / Height
	Reserved13             RawMessagePackSlot `json:"reserved13,omitempty"`   // C# 无 Key(13)，通常为 MessagePack nil / C# has no Key(13), normally MessagePack nil
	Reserved14             RawMessagePackSlot `json:"reserved14,omitempty"`   // C# 无 Key(14)，通常为 MessagePack nil / C# has no Key(14), normally MessagePack nil
	Reserved15             RawMessagePackSlot `json:"reserved15,omitempty"`   // C# 无 Key(15)，通常为 MessagePack nil / C# has no Key(15), normally MessagePack nil
	CenterMpnList          []int              `json:"centerMpnList"`          // 中心MPN枚举列表，对应 C# List<MPN> / Center MPN enum list, matching C# List<MPN>
	CenterRateMax          Vector3            `json:"centerRateMax"`          // 中心最大比率 / Max center rate
	StartRadiusMpnList     []int              `json:"startRadiusMpnList"`     // 起点半径MPN枚举列表 / Start-radius MPN enum list
	MaxStartRadius         float32            `json:"maxStartRadius"`         // 起点半径最大值 / Max start radius
	EndRadiusMpnList       []int              `json:"endRadiusMpnList"`       // 终点半径MPN枚举列表 / End-radius MPN enum list
	MaxEndRadius           float32            `json:"maxEndRadius"`           // 终点半径最大值 / Max end radius
	CenterMpnNameList      []string           `json:"centerMpnNameList"`      // 中心MPN名 / Center MPN names
	StartRadiusMpnNameList []string           `json:"startRadiusMpnNameList"` // 起点半径MPN名 / Start-radius MPN names
	EndRadiusMpnNameList   []string           `json:"endRadiusMpnNameList"`   // 终点半径MPN名 / End-radius MPN names
}

// toColliderType 返回 MaidPropCol union 类型标记
// toColliderType returns the MaidPropCol union type tag
func (*ColliderMaidProp) toColliderType() int { return ColliderTypeMaidProp }

// newColliderObject 创建使用游戏基类字段初始化默认值的碰撞体对象
// newColliderObject creates a collider object with the game's base-field initializer defaults
func newColliderObject(version int) ColliderObject {
	return ColliderObject{
		Version:       version,
		LocalRotation: Vector4{W: 1},
		LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
	}
}

// NewColliderPlane 按当前 C# 构造函数与字段初始化器创建新的平面碰撞体，线格式和 JSON 解码始终从零值开始且不运行游戏迁移
// NewColliderPlane creates a new plane collider from current C# constructors and field initializers, while wire and JSON decoding always start from zero values and never run game migrations
func NewColliderPlane() *ColliderPlane {
	return &ColliderPlane{
		ColliderObject: newColliderObject(colliderStatusFixVersion),
		Direction:      VectorTypeY,
	}
}

// NewColliderCapsule 按当前 C# 构造函数与字段初始化器创建新的胶囊碰撞体
// NewColliderCapsule creates a new capsule collider from current C# constructors and field initializers
func NewColliderCapsule() *ColliderCapsule {
	return &ColliderCapsule{
		ColliderObject: newColliderObject(colliderStatusFixVersion),
		Direction:      VectorTypeY,
		StartRadius:    0.5,
		EndRadius:      0.5,
	}
}

// NewColliderSphere 按当前 C# 构造函数与字段初始化器创建新的球形碰撞体
// NewColliderSphere creates a new sphere collider from current C# constructors and field initializers
func NewColliderSphere() *ColliderSphere {
	return &ColliderSphere{
		ColliderObject: newColliderObject(colliderStatusFixVersion),
		Radius:         0.5,
	}
}

// NewColliderMaidProp 按当前 C# 构造函数与字段初始化器创建新的女仆属性碰撞体
// NewColliderMaidProp creates a new maid-property collider from current C# constructors and field initializers
func NewColliderMaidProp() *ColliderMaidProp {
	return &ColliderMaidProp{
		ColliderObject:         newColliderObject(colliderMaidPropFixVersion),
		Direction:              VectorTypeY,
		StartRadius:            0.5,
		EndRadius:              0.5,
		CenterMpnList:          []int{},
		StartRadiusMpnList:     []int{},
		MaxStartRadius:         1,
		EndRadiusMpnList:       []int{},
		MaxEndRadius:           1,
		CenterMpnNameList:      []string{},
		StartRadiusMpnNameList: []string{},
		EndRadiusMpnNameList:   []string{},
	}
}

// UnmarshalJSON 解码 ColliderPlane 而不注入构造默认值
// UnmarshalJSON decodes ColliderPlane without injecting constructor defaults
func (v *ColliderPlane) UnmarshalJSON(data []byte) error {
	type colliderPlaneJSON ColliderPlane
	var value colliderPlaneJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = ColliderPlane(value)
	return nil
}

// UnmarshalJSON 解码 ColliderCapsule 而不注入构造默认值
// UnmarshalJSON decodes ColliderCapsule without injecting constructor defaults
func (v *ColliderCapsule) UnmarshalJSON(data []byte) error {
	type colliderCapsuleJSON ColliderCapsule
	var value colliderCapsuleJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = ColliderCapsule(value)
	return nil
}

// UnmarshalJSON 解码 ColliderSphere 而不注入构造默认值
// UnmarshalJSON decodes ColliderSphere without injecting constructor defaults
func (v *ColliderSphere) UnmarshalJSON(data []byte) error {
	type colliderSphereJSON ColliderSphere
	var value colliderSphereJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = ColliderSphere(value)
	return nil
}

// UnmarshalJSON 解码 ColliderMaidProp 而不注入构造默认值或运行游戏回调
// UnmarshalJSON decodes ColliderMaidProp without injecting constructor defaults or running game callbacks
func (v *ColliderMaidProp) UnmarshalJSON(data []byte) error {
	type colliderMaidPropJSON ColliderMaidProp
	var value colliderMaidPropJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = ColliderMaidProp(value)
	return nil
}

// ColliderState 表示 DynamicYureBone.LimbColliderInfo
// ColliderState represents DynamicYureBone.LimbColliderInfo
type ColliderState struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int         `json:"version"`  // 版本号 / Version value
	LimbType               int         `json:"limbType"` // limbType 枚举值 / LimbType enum value
	IsEnable               bool        `json:"isEnable"` // 是否启用 / Enabled
}

// colliderStateJSON 表示 ColliderState 的 JSON 兼容输入
// colliderStateJSON represents JSON compatibility input for ColliderState
type colliderStateJSON struct {
	*IndexedObjectMetadata                 // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int             `json:"version"`            // 版本号 / Version value
	LimbType               *int            `json:"limbType,omitempty"` // 可选 limb 类型 / Optional limb type
	IsEnable               *bool           `json:"isEnable,omitempty"` // 可选启用状态 / Optional enable state
	Index                  json.RawMessage `json:"index,omitempty"`    // 仅用于拒绝旧版 index 字段 / Used only to reject the legacy index field
	Enabled                json.RawMessage `json:"enabled,omitempty"`  // 仅用于拒绝旧版 enabled 字段 / Used only to reject the legacy enabled field
}

// UnmarshalJSON 解码 ColliderState 并拒绝已移除的旧字段名
// UnmarshalJSON decodes ColliderState and rejects removed legacy field names
func (s *ColliderState) UnmarshalJSON(data []byte) error {
	var raw colliderStateJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Index) > 0 {
		return fmt.Errorf(`collider state "index" is no longer supported; use "limbType"`)
	}
	if len(raw.Enabled) > 0 {
		return fmt.Errorf(`collider state "enabled" is no longer supported; use "isEnable"`)
	}
	value := ColliderState{IndexedObjectMetadata: raw.IndexedObjectMetadata, Version: raw.Version}
	if raw.LimbType != nil {
		value.LimbType = *raw.LimbType
	}
	if raw.IsEnable != nil {
		value.IsEnable = *raw.IsEnable
	}
	*s = value
	return nil
}

// LimbColliderPackage 对应 LimbColliderMgr 保存的 limb collider 包
// LimbColliderPackage maps the limb-collider package saved by LimbColliderMgr
type LimbColliderPackage struct {
	_struct                struct{}           `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`        // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int                `json:"version"` // 版本号 / Version value
	Items                  []LimbColliderItem `json:"items"`   // limb 碰撞体条目列表 / limb collider item list
}

// LimbColliderItem 表示一个 limb 类型和 NativeMaidPropColliderStatus
// LimbColliderItem represents one limb type and NativeMaidPropColliderStatus
type LimbColliderItem struct {
	*IndexedObjectMetadata `codec:"-"`         // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int                 `json:"version"`                         // 版本号 / Version value
	Target                 int                 `json:"target"`                          // limbType 枚举值 / LimbType enum value
	Collider               ColliderStatusUnion `json:"collider"`                        // 碰撞体状态 / Collider status
	ColliderRaw            RawMessagePackSlot  `json:"colliderRaw,omitempty" codec:"-"` // 无法作为固定 MaidProp 类型编辑时保留的完整 Key(2) 载荷 / Complete Key(2) payload preserved when it cannot be edited as the fixed MaidProp type
}

// limbColliderItemAlias 避免 LimbColliderItem.MarshalJSON 递归调用自身
// limbColliderItemAlias prevents LimbColliderItem.MarshalJSON from recursively calling itself
type limbColliderItemAlias LimbColliderItem

// limbColliderItemJSON 表示可区分强类型载荷与原始载荷的 limb 条目 JSON
// limbColliderItemJSON represents limb-item JSON that distinguishes a typed payload from a raw payload
type limbColliderItemJSON struct {
	*IndexedObjectMetadata                    // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int                `json:"version"`               // 条目版本 / Item version
	Target                 int                `json:"target"`                // limb 类型枚举 / Limb type enum
	Collider               json.RawMessage    `json:"collider"`              // MaidProp 碰撞体 JSON / MaidProp collider JSON
	ColliderRaw            RawMessagePackSlot `json:"colliderRaw,omitempty"` // 原始 Key(2) MessagePack 载荷 / Raw Key(2) MessagePack payload
}

// UnmarshalJSON 将条目碰撞体固定解码为游戏声明的 NativeMaidPropColliderStatus
// UnmarshalJSON decodes the item collider as the NativeMaidPropColliderStatus fixed by the game declaration
func (item *LimbColliderItem) UnmarshalJSON(data []byte) error {
	var raw limbColliderItemJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.ColliderRaw) != 0 && !jsonMessageIsNullOrMissing(raw.Collider) {
		return fmt.Errorf("limb collider item collider and colliderRaw cannot both be populated")
	}
	var collider ColliderStatusUnion
	if len(raw.ColliderRaw) == 0 && !jsonMessageIsNullOrMissing(raw.Collider) {
		// C# LimbColliderMgr.LimbColliderData.Key(2) 是具体 NativeMaidPropColliderStatus，而不是 ANativeColliderStatus union
		// C# LimbColliderMgr.LimbColliderData.Key(2) is a concrete NativeMaidPropColliderStatus rather than the ANativeColliderStatus union
		var err error
		collider, err = decodeColliderObjectAsType(raw.Collider, ColliderTypeMaidProp)
		if err != nil {
			return fmt.Errorf("limb collider item collider: %w", err)
		}
	}
	*item = LimbColliderItem{
		IndexedObjectMetadata: raw.IndexedObjectMetadata,
		Version:               raw.Version,
		Target:                raw.Target,
		Collider:              collider,
		ColliderRaw:           cloneRawMessagePackSlot(raw.ColliderRaw),
	}
	return nil
}

// MarshalJSON 编码 limb 碰撞体条目及其强类型或原始载荷
// MarshalJSON encodes a limb-collider item and its typed or raw payload
func (item LimbColliderItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(limbColliderItemAlias(item))
}

// IKColliderPackage 对应 IKColliderSaveLoader 保存的 IK collider 包
// IKColliderPackage maps the IK-collider package saved by IKColliderSaveLoader
type IKColliderPackage struct {
	_struct                struct{}          `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`       // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int               `json:"version"` // 版本号 / Version value
	Groups                 []IKColliderGroup `json:"groups"`  // IK 效果器分组列表 / IK effector group list
}

// IKColliderGroup 表示一个 IK 效果器的碰撞体列表
// IKColliderGroup represents colliders for one IK effector
type IKColliderGroup struct {
	_struct                struct{}      `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`   // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int           `json:"version"`   // 版本号 / Version value
	Target                 int           `json:"target"`    // effectorType 枚举值 / Effector type enum
	Colliders              []ColliderRef `json:"colliders"` // 该效果器关联的碰撞体引用列表 / Collider references associated with effector
}

// colliderRefWire 是依次保存 unionTag 和 concreteObject 的 MessagePack-CSharp union 封套
// 具体对象只在选择格式化器之前保持原始值，全部数组框架和元数据处理仍由共用 indexed-object 编解码器负责
// colliderRefWire is the MessagePack-CSharp union wrapper storing unionTag followed by concreteObject
// The concrete object remains raw only long enough to select its formatter, while all array framing and metadata handling stays in the shared indexed-object codec
type colliderRefWire struct {
	_struct                struct{}           `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`        // union 封套的线格式元数据 / Wire metadata for the union wrapper
	Type                   int                `json:"type"`     // union 类型标记 / Union type tag
	Collider               RawMessagePackSlot `json:"collider"` // 具体对象的完整 MessagePack 值 / Complete MessagePack value of the concrete object
}

// CodecEncodeSelf 按共享 indexed-object 规则编码碰撞体 union 封套
// CodecEncodeSelf encodes a collider union wrapper using the shared indexed-object rules
func (v colliderRefWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码碰撞体 union 封套
// CodecDecodeSelf decodes a collider union wrapper using the shared indexed-object rules
func (v *colliderRefWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// limbColliderItemWire 表示 LimbColliderData 的固定三槽 MessagePack 布局
// limbColliderItemWire represents the fixed three-slot MessagePack layout of LimbColliderData
type limbColliderItemWire struct {
	_struct                struct{}           `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`        // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int                `json:"version"`  // 条目版本 / Item version
	Target                 int                `json:"target"`   // limb 类型枚举 / Limb type enum
	Collider               RawMessagePackSlot `json:"collider"` // NativeMaidPropColliderStatus 的完整 MessagePack 值 / Complete MessagePack value of NativeMaidPropColliderStatus
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 LimbColliderData 线格式
// CodecEncodeSelf encodes the LimbColliderData wire form using the shared indexed-object rules
func (v limbColliderItemWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 LimbColliderData 线格式
// CodecDecodeSelf decodes the LimbColliderData wire form using the shared indexed-object rules
func (v *limbColliderItemWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 将 ColliderRef 的强类型或原始载荷编码为 MessagePack-CSharp union
// CodecEncodeSelf encodes the typed or raw payload of ColliderRef as a MessagePack-CSharp union
func (c ColliderRef) CodecEncodeSelf(e *codec.Encoder) {
	raw, err := encodeColliderStatusSlot(c.Collider, c.ColliderRaw, "ColliderRef.collider")
	if err != nil {
		panic(err)
	}
	wire := colliderRefWire{
		IndexedObjectMetadata: c.IndexedObjectMetadata,
		Type:                  c.Type,
		Collider:              raw,
	}
	ct.EncodeIndexedObjectSelf(e, &wire)
}

// CodecDecodeSelf 解码 MessagePack-CSharp union 并保留未知或无法选择格式化器的载荷
// CodecDecodeSelf decodes a MessagePack-CSharp union and preserves payloads with unknown or unselectable formatters
func (c *ColliderRef) CodecDecodeSelf(d *codec.Decoder) {
	var root codec.Raw
	d.MustDecode(&root)
	// nil 原始值表示 null ANativeColliderStatus 列表元素
	// A nil raw value represents a null ANativeColliderStatus list element
	if len(root) == 0 {
		*c = ColliderRef{}
		return
	}
	var wire colliderRefWire
	if err := ct.DecodeMsgpack(root, &wire); err != nil {
		panic(fmt.Errorf("decode ColliderRef union wrapper: %w", err))
	}
	result := ColliderRef{
		IndexedObjectMetadata: wire.IndexedObjectMetadata,
		Type:                  wire.Type,
	}
	if indexedObjectSlotPresent(wire.IndexedObjectMetadata, 1, 2) && len(wire.Collider) != 0 {
		if indexedObjectSlotIsNil(wire.IndexedObjectMetadata, 0) {
			// nil union 标记无法选择已知格式化器，因此保留第二个槽位而不猜测 Go int 零值表示 Plane
			// A nil union tag cannot select a known formatter, so preserve the second slot instead of guessing that a zero Go int meant Plane
			result.ColliderRaw = cloneRawMessagePackSlot(wire.Collider)
		} else if status, known := newColliderStatusForType(wire.Type); known {
			if err := ct.DecodeMsgpack(wire.Collider, status); err != nil {
				panic(fmt.Errorf("decode ColliderRef type %d payload: %w", wire.Type, err))
			}
			result.Collider = status
		} else {
			result.ColliderRaw = cloneRawMessagePackSlot(wire.Collider)
		}
	}
	*c = result
}

// CodecEncodeSelf 将 LimbColliderItem 的固定 MaidProp 碰撞体或原始载荷编码为三槽布局
// CodecEncodeSelf encodes the fixed MaidProp collider or raw payload of LimbColliderItem in its three-slot layout
func (item LimbColliderItem) CodecEncodeSelf(e *codec.Encoder) {
	raw, err := encodeColliderStatusSlot(item.Collider, item.ColliderRaw, "LimbColliderItem.collider")
	if err != nil {
		panic(err)
	}
	wire := limbColliderItemWire{
		IndexedObjectMetadata: item.IndexedObjectMetadata,
		Version:               item.Version,
		Target:                item.Target,
		Collider:              raw,
	}
	ct.EncodeIndexedObjectSelf(e, &wire)
}

// CodecDecodeSelf 将三槽 LimbColliderData 解码为固定 NativeMaidPropColliderStatus
// CodecDecodeSelf decodes the three-slot LimbColliderData as its fixed NativeMaidPropColliderStatus
func (item *LimbColliderItem) CodecDecodeSelf(d *codec.Decoder) {
	var root codec.Raw
	d.MustDecode(&root)
	// nil 原始值表示 null LimbColliderData 列表元素
	// A nil raw value represents a null LimbColliderData list element
	if len(root) == 0 {
		*item = LimbColliderItem{}
		return
	}
	var wire limbColliderItemWire
	if err := ct.DecodeMsgpack(root, &wire); err != nil {
		panic(fmt.Errorf("decode LimbColliderItem: %w", err))
	}
	result := LimbColliderItem{
		IndexedObjectMetadata: wire.IndexedObjectMetadata,
		Version:               wire.Version,
		Target:                wire.Target,
	}
	if indexedObjectSlotPresent(wire.IndexedObjectMetadata, 2, 3) && len(wire.Collider) != 0 {
		status := &ColliderMaidProp{}
		if err := ct.DecodeMsgpack(wire.Collider, status); err != nil {
			panic(fmt.Errorf("decode LimbColliderItem.collider as NativeMaidPropColliderStatus: %w", err))
		}
		result.Collider = status
	}
	*item = result
}

// encodeColliderStatusSlot 将强类型碰撞体或一个原始完整值转换为 union 载荷槽位
// encodeColliderStatusSlot converts a typed collider or one raw complete value into a union payload slot
func encodeColliderStatusSlot(status ColliderStatusUnion, raw RawMessagePackSlot, name string) (RawMessagePackSlot, error) {
	if status != nil && len(raw) != 0 {
		return nil, fmt.Errorf("%s and colliderRaw cannot both be populated", name)
	}
	if len(raw) != 0 {
		if err := validateRawMessagePackSlot(raw, name+"Raw"); err != nil {
			return nil, err
		}
		return cloneRawMessagePackSlot(raw), nil
	}
	if status == nil {
		// nil 状态使用普通 MessagePack nil
		// A nil status uses normal MessagePack nil
		return nil, nil
	}
	selfer, ok := status.(codec.Selfer)
	if !ok {
		return nil, fmt.Errorf("%s type %T does not implement the indexed MessagePack codec", name, status)
	}
	encoded, err := ct.EncodeIndexedMsgpack(selfer)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", name, err)
	}
	return RawMessagePackSlot(encoded), nil
}

// validateRawMessagePackSlot 验证原始槽位恰好包含一个完整 MessagePack 值
// validateRawMessagePackSlot verifies that a raw slot contains exactly one complete MessagePack value
func validateRawMessagePackSlot(raw RawMessagePackSlot, name string) error {
	root, trailing, err := ct.SplitFirstMsgpackValue(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if len(root) != len(raw) || len(trailing) != 0 {
		return fmt.Errorf("%s contains %d trailing bytes", name, len(trailing))
	}
	return nil
}

// newColliderStatusForType 为已知 union 类型标记创建零值具体碰撞体
// newColliderStatusForType creates a zero-value concrete collider for a known union type tag
func newColliderStatusForType(typ int) (ColliderStatusUnion, bool) {
	switch typ {
	case ColliderTypePlane:
		return &ColliderPlane{}, true
	case ColliderTypeCapsule:
		return &ColliderCapsule{}, true
	case ColliderTypeSphere:
		return &ColliderSphere{}, true
	case ColliderTypeMaidProp:
		return &ColliderMaidProp{}, true
	default:
		return nil, false
	}
}

// indexedObjectSlotPresent 按保存的 FieldCount 判断槽位是否在线格式中存在
// indexedObjectSlotPresent reports whether a slot exists on the wire according to the preserved FieldCount
func indexedObjectSlotPresent(metadata *IndexedObjectMetadata, slot, known int) bool {
	count := known
	if metadata != nil && metadata.FieldCount != nil {
		count = *metadata.FieldCount
	}
	return slot >= 0 && slot < count
}

// indexedObjectSlotIsNil 判断 indexed-object 元数据是否将指定槽位记录为 nil
// indexedObjectSlotIsNil reports whether indexed-object metadata records the specified slot as nil
func indexedObjectSlotIsNil(metadata *IndexedObjectMetadata, slot int) bool {
	if metadata == nil {
		return false
	}
	for _, candidate := range metadata.NilSlots {
		if candidate == slot {
			return true
		}
	}
	return false
}

// cloneRawMessagePackSlot 复制原始 MessagePack 槽位并保留 nil 状态
// cloneRawMessagePackSlot clones a raw MessagePack slot while preserving nil state
func cloneRawMessagePackSlot(raw RawMessagePackSlot) RawMessagePackSlot {
	if raw == nil {
		return nil
	}
	return append(RawMessagePackSlot(nil), raw...)
}

// jsonMessageIsNullOrMissing 判断原始 JSON 字段是否缺失、空白或为 null
// jsonMessageIsNullOrMissing reports whether a raw JSON field is missing, blank, or null
func jsonMessageIsNullOrMissing(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

const (
	colliderPackageFixVersion     = 1000
	colliderStatusFixVersion      = 1000
	colliderMaidPropFixVersion    = 1002
	colliderStateFixVersion       = 1000
	limbColliderPackageFixVersion = 1000
	limbColliderItemFixVersion    = 1000
	ikColliderPackageFixVersion   = 1000
	ikColliderGroupFixVersion     = 1000
)

// validateColliderStatusForEncoding 验证具体碰撞体字段并返回其必需的 union 类型标记
// validateColliderStatusForEncoding validates concrete collider fields and returns the required union type tag
func validateColliderStatusForEncoding(status ColliderStatusUnion, name string) (int, error) {
	switch v := status.(type) {
	case *ColliderPlane:
		if v == nil {
			return -1, fmt.Errorf("%s is nil (*ColliderPlane)", name)
		}
		if err := validateColliderObjectInt32(&v.ColliderObject, name); err != nil {
			return -1, err
		}
		if err := requireInt32(name+".direction", v.Direction); err != nil {
			return -1, err
		}
		return ColliderTypePlane, nil
	case *ColliderCapsule:
		if v == nil {
			return -1, fmt.Errorf("%s is nil (*ColliderCapsule)", name)
		}
		if err := validateColliderObjectInt32(&v.ColliderObject, name); err != nil {
			return -1, err
		}
		if err := requireInt32(name+".direction", v.Direction); err != nil {
			return -1, err
		}
		return ColliderTypeCapsule, nil
	case *ColliderSphere:
		if v == nil {
			return -1, fmt.Errorf("%s is nil (*ColliderSphere)", name)
		}
		if err := validateColliderObjectInt32(&v.ColliderObject, name); err != nil {
			return -1, err
		}
		return ColliderTypeSphere, nil
	case *ColliderMaidProp:
		if v == nil {
			return -1, fmt.Errorf("%s is nil (*ColliderMaidProp)", name)
		}
		if err := validateColliderObjectInt32(&v.ColliderObject, name); err != nil {
			return -1, err
		}
		if err := requireInt32(name+".direction", v.Direction); err != nil {
			return -1, err
		}
		if err := validateMaidPropMPNList(v.CenterMpnList, name+".centerMpnList"); err != nil {
			return -1, err
		}
		if err := validateMaidPropMPNList(v.StartRadiusMpnList, name+".startRadiusMpnList"); err != nil {
			return -1, err
		}
		if err := validateMaidPropMPNList(v.EndRadiusMpnList, name+".endRadiusMpnList"); err != nil {
			return -1, err
		}
		return ColliderTypeMaidProp, nil
	default:
		return -1, fmt.Errorf("%s has unsupported type %T", name, status)
	}
}

// validateColliderObjectInt32 验证碰撞体基类中的 C# Int32 字段范围
// validateColliderObjectInt32 validates the ranges of C# Int32 fields in the collider base object
func validateColliderObjectInt32(value *ColliderObject, name string) error {
	if err := requireInt32(name+".version", value.Version); err != nil {
		return err
	}
	return requireInt32(name+".bound", value.Bound)
}

// validateColliderRefsForEncoding 验证碰撞体引用的 null 元素、union 标记和载荷一致性
// validateColliderRefsForEncoding validates null elements, union tags, and payload consistency in collider references
func validateColliderRefsForEncoding(refs []ColliderRef, metadata *IndexedObjectMetadata, slot int, name string) error {
	for i := range refs {
		if indexedObjectNullElementAt(metadata, slot, i) {
			continue
		}
		refName := fmt.Sprintf("%s[%d]", name, i)
		if err := requireInt32(refName+".type", refs[i].Type); err != nil {
			return err
		}
		if refs[i].Collider != nil && len(refs[i].ColliderRaw) != 0 {
			return fmt.Errorf("%s.collider and colliderRaw cannot both be populated", refName)
		}
		if len(refs[i].ColliderRaw) != 0 {
			if err := validateRawMessagePackSlot(refs[i].ColliderRaw, refName+".colliderRaw"); err != nil {
				return err
			}
			continue
		}
		// MessagePack-CSharp union 可以承载 nil，较短封套也可省略具体对象槽位，由 FieldCount 元数据区分两种情况
		// MessagePack-CSharp unions can carry nil and a short wrapper can omit the concrete slot, with FieldCount metadata distinguishing the cases
		if refs[i].Collider == nil {
			continue
		}
		actualType, err := validateColliderStatusForEncoding(refs[i].Collider, refName+".collider")
		if err != nil {
			return err
		}
		if refs[i].Type != actualType {
			return fmt.Errorf("%s.type is %d, but collider concrete type requires %d", refName, refs[i].Type, actualType)
		}
	}
	return nil
}

// validateColliderPackageForEncoding 验证通用碰撞体包的版本、引用和 limb 状态字段
// validateColliderPackageForEncoding validates version, references, and limb-state fields in a generic collider package
func validateColliderPackageForEncoding(p *ColliderPackage) error {
	if err := requireInt32("colliderPackage.version", p.Version); err != nil {
		return err
	}
	if err := validateColliderRefsForEncoding(p.Colliders, p.IndexedObjectMetadata, 1, "colliderPackage.colliders"); err != nil {
		return err
	}
	for i := range p.LimbEnableList {
		if indexedObjectNullElementAt(p.IndexedObjectMetadata, 2, i) {
			continue
		}
		name := fmt.Sprintf("colliderPackage.limbEnableList[%d]", i)
		if err := requireInt32(name+".version", p.LimbEnableList[i].Version); err != nil {
			return err
		}
		if err := requireInt32(name+".limbType", p.LimbEnableList[i].LimbType); err != nil {
			return err
		}
	}
	return nil
}

// validateLimbColliderPackageForEncoding 验证 LimbColliderMgr 包及其固定 MaidProp 碰撞体条目
// validateLimbColliderPackageForEncoding validates a LimbColliderMgr package and its fixed MaidProp collider items
func validateLimbColliderPackageForEncoding(p *LimbColliderPackage) error {
	if err := requireInt32("limbColliderPackage.version", p.Version); err != nil {
		return err
	}
	for i := range p.Items {
		if indexedObjectNullElementAt(p.IndexedObjectMetadata, 1, i) {
			continue
		}
		itemName := fmt.Sprintf("limbColliderPackage.items[%d]", i)
		if err := requireInt32(itemName+".version", p.Items[i].Version); err != nil {
			return err
		}
		if err := requireInt32(itemName+".target", p.Items[i].Target); err != nil {
			return err
		}
		if p.Items[i].Collider != nil && len(p.Items[i].ColliderRaw) != 0 {
			return fmt.Errorf("%s.collider and colliderRaw cannot both be populated", itemName)
		}
		if len(p.Items[i].ColliderRaw) != 0 {
			if err := validateRawMessagePackSlot(p.Items[i].ColliderRaw, itemName+".colliderRaw"); err != nil {
				return err
			}
			continue
		}
		if p.Items[i].Collider == nil {
			continue
		}
		colliderName := itemName + ".collider"
		actualType, err := validateColliderStatusForEncoding(p.Items[i].Collider, colliderName)
		if err != nil {
			return err
		}
		if actualType != ColliderTypeMaidProp {
			return fmt.Errorf("%s must be *ColliderMaidProp; got collider type %d", colliderName, actualType)
		}
	}
	return nil
}

// validateIKColliderPackageForEncoding 验证 IK 碰撞体包、效果器分组和 union 引用
// validateIKColliderPackageForEncoding validates an IK-collider package, effector groups, and union references
func validateIKColliderPackageForEncoding(p *IKColliderPackage) error {
	if err := requireInt32("ikColliderPackage.version", p.Version); err != nil {
		return err
	}
	for i := range p.Groups {
		if indexedObjectNullElementAt(p.IndexedObjectMetadata, 1, i) {
			continue
		}
		groupName := fmt.Sprintf("ikColliderPackage.groups[%d]", i)
		if err := requireInt32(groupName+".version", p.Groups[i].Version); err != nil {
			return err
		}
		if err := requireInt32(groupName+".target", p.Groups[i].Target); err != nil {
			return err
		}
		if err := validateColliderRefsForEncoding(p.Groups[i].Colliders, p.Groups[i].IndexedObjectMetadata, 2, groupName+".colliders"); err != nil {
			return err
		}
	}
	return nil
}

// cloneColliderStatusForEncoding 深复制具体碰撞体中的切片与原始稀疏槽位
// cloneColliderStatusForEncoding deep-clones slices and raw sparse slots in a concrete collider
func cloneColliderStatusForEncoding(status ColliderStatusUnion) ColliderStatusUnion {
	switch v := status.(type) {
	case *ColliderPlane:
		cloned := *v
		return &cloned
	case *ColliderCapsule:
		cloned := *v
		return &cloned
	case *ColliderSphere:
		cloned := *v
		return &cloned
	case *ColliderMaidProp:
		cloned := *v
		cloned.Reserved13 = cloneRawMessagePackSlot(v.Reserved13)
		cloned.Reserved14 = cloneRawMessagePackSlot(v.Reserved14)
		cloned.Reserved15 = cloneRawMessagePackSlot(v.Reserved15)
		cloned.CenterMpnList = cloneSlicePreserveNil(v.CenterMpnList)
		cloned.StartRadiusMpnList = cloneSlicePreserveNil(v.StartRadiusMpnList)
		cloned.EndRadiusMpnList = cloneSlicePreserveNil(v.EndRadiusMpnList)
		cloned.CenterMpnNameList = cloneSlicePreserveNil(v.CenterMpnNameList)
		cloned.StartRadiusMpnNameList = cloneSlicePreserveNil(v.StartRadiusMpnNameList)
		cloned.EndRadiusMpnNameList = cloneSlicePreserveNil(v.EndRadiusMpnNameList)
		return &cloned
	}
	return status
}

// validateMaidPropMPNList 验证 MPN 枚举列表中的值可由游戏 Int32 底层类型表示
// validateMaidPropMPNList verifies that values in an MPN enum list fit the game's Int32 underlying type
func validateMaidPropMPNList(values []int, name string) error {
	const (
		minMPN = int64(-1 << 31)
		maxMPN = int64(1<<31 - 1)
	)
	for i, value := range values {
		if int64(value) < minMPN || int64(value) > maxMPN {
			return fmt.Errorf("%s[%d]=%d is outside the Int32 range used by the game's MPN enum", name, i, value)
		}
	}
	return nil
}

// cloneColliderRefsForEncoding 深复制碰撞体引用的强类型与原始载荷
// cloneColliderRefsForEncoding deep-clones typed and raw payloads in collider references
func cloneColliderRefsForEncoding(refs []ColliderRef) []ColliderRef {
	cloned := cloneSlicePreserveNil(refs)
	for i := range cloned {
		cloned[i].Collider = cloneColliderStatusForEncoding(cloned[i].Collider)
		cloned[i].ColliderRaw = cloneRawMessagePackSlot(cloned[i].ColliderRaw)
	}
	return cloned
}

// normalizeColliderPackageForEncoding 复制通用碰撞体包的可变集合供编码使用
// normalizeColliderPackageForEncoding copies mutable collections in a generic collider package for encoding
func normalizeColliderPackageForEncoding(p *ColliderPackage) *ColliderPackage {
	cloned := *p
	cloned.Colliders = cloneColliderRefsForEncoding(p.Colliders)
	cloned.LimbEnableList = cloneSlicePreserveNil(p.LimbEnableList)
	return &cloned
}

// normalizeLimbColliderPackageForEncoding 复制 limb 包的条目与碰撞体载荷供编码使用
// normalizeLimbColliderPackageForEncoding copies limb-package items and collider payloads for encoding
func normalizeLimbColliderPackageForEncoding(p *LimbColliderPackage) *LimbColliderPackage {
	cloned := *p
	cloned.Items = cloneSlicePreserveNil(p.Items)
	for i := range cloned.Items {
		cloned.Items[i].Collider = cloneColliderStatusForEncoding(cloned.Items[i].Collider)
		cloned.Items[i].ColliderRaw = cloneRawMessagePackSlot(cloned.Items[i].ColliderRaw)
	}
	return &cloned
}

// indexedObjectNullElementAt 判断元数据是否将集合中的指定元素记录为 MessagePack nil
// indexedObjectNullElementAt reports whether metadata records a specified collection element as MessagePack nil
func indexedObjectNullElementAt(metadata *IndexedObjectMetadata, slot, element int) bool {
	if metadata == nil || metadata.NullElements == nil {
		return false
	}
	flags := metadata.NullElements[slot]
	return element >= 0 && element < len(flags) && flags[element]
}

// normalizeIKColliderPackageForEncoding 复制 IK 分组及其碰撞体引用供编码使用
// normalizeIKColliderPackageForEncoding copies IK groups and their collider references for encoding
func normalizeIKColliderPackageForEncoding(p *IKColliderPackage) *IKColliderPackage {
	cloned := *p
	cloned.Groups = cloneSlicePreserveNil(p.Groups)
	for i := range cloned.Groups {
		cloned.Groups[i].Colliders = cloneColliderRefsForEncoding(p.Groups[i].Colliders)
	}
	return &cloned
}

// decodeColliderObjectAsType 按 union 类型标记将 JSON 解码为具体碰撞体
// decodeColliderObjectAsType decodes JSON into a concrete collider according to its union type tag
func decodeColliderObjectAsType(raw json.RawMessage, typ int) (ColliderStatusUnion, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("collider is required")
	}
	switch typ {
	case ColliderTypePlane:
		var status ColliderPlane
		if err := json.Unmarshal(raw, &status); err != nil {
			return nil, err
		}
		return &status, nil
	case ColliderTypeCapsule:
		var status ColliderCapsule
		if err := json.Unmarshal(raw, &status); err != nil {
			return nil, err
		}
		return &status, nil
	case ColliderTypeSphere:
		var status ColliderSphere
		if err := json.Unmarshal(raw, &status); err != nil {
			return nil, err
		}
		return &status, nil
	case ColliderTypeMaidProp:
		var status ColliderMaidProp
		if err := json.Unmarshal(raw, &status); err != nil {
			return nil, err
		}
		return &status, nil
	default:
		return nil, fmt.Errorf("unknown collider type %d", typ)
	}
}
