package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/strictjson"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// .dbcol、.dslcol、.limbcol 与 .ikcol 系列共用的碰撞体结构、union 编码和校验逻辑
// Collider structures, union encoding, and validation shared by .dbcol, .dslcol, .limbcol, and .ikcol families

// ColliderPackage 表示通用碰撞体包
// ColliderPackage represents a generic collider package
type ColliderPackage struct {
	_struct        struct{}         `codec:",toarray"`      // 强制按数组编码 / Forces array encoding
	Version        int32            `json:"version"`        // 版本号 / Version value
	Colliders      []*ColliderRef   `json:"colliders"`      // 可空碰撞体引用列表 / List of nullable collider references
	LimbEnableList []*ColliderState `json:"limbEnableList"` // 可空 DynamicYureBone.LimbColliderInfo 列表 / List of nullable DynamicYureBone.LimbColliderInfo objects
}

// colliderPackageJSON 表示 ColliderPackage 的 JSON 兼容视图
// colliderPackageJSON represents the JSON compatibility view of ColliderPackage
type colliderPackageJSON struct {
	Version        int32            `json:"version"`          // 版本号 / Version value
	Colliders      []*ColliderRef   `json:"colliders"`        // 可空碰撞体引用列表 / List of nullable collider references
	LimbEnableList []*ColliderState `json:"limbEnableList"`   // 可空 limb 启用状态列表 / List of nullable limb enable-state objects
	States         json.RawMessage  `json:"states,omitempty"` // 仅用于拒绝旧版 states 字段 / Used only to reject the legacy states field
}

// MarshalJSON 将 ColliderPackage 编码为当前 JSON 字段名
// MarshalJSON encodes ColliderPackage using the current JSON field names
func (p ColliderPackage) MarshalJSON() ([]byte, error) {
	return json.Marshal(colliderPackageJSON{
		Version:        p.Version,
		Colliders:      p.Colliders,
		LimbEnableList: p.LimbEnableList,
	})
}

// UnmarshalJSON 解码 ColliderPackage 并拒绝已移除的 states 字段
// UnmarshalJSON decodes ColliderPackage and rejects the removed states field
func (p *ColliderPackage) UnmarshalJSON(data []byte) error {
	var raw colliderPackageJSON
	if err := decodeColliderJSONStrict(data, &raw); err != nil {
		return err
	}
	if len(raw.States) > 0 {
		return fmt.Errorf(`colliderPackage.states is no longer supported; use "limbEnableList"`)
	}
	*p = ColliderPackage{
		Version:        raw.Version,
		Colliders:      raw.Colliders,
		LimbEnableList: raw.LimbEnableList,
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
	Type     int32               `json:"type"`     // 碰撞体类型枚举 / Collider type enum
	Collider ColliderStatusUnion `json:"collider"` // 碰撞体对象数据 / Collider object data
}

// colliderRefAlias 避免 ColliderRef.MarshalJSON 递归调用自身
// colliderRefAlias prevents ColliderRef.MarshalJSON from recursively calling itself
type colliderRefAlias ColliderRef

// colliderRefJSON 表示按 type 判别具体碰撞体类型的 JSON 输入
// colliderRefJSON represents JSON input whose concrete collider type is selected by type
type colliderRefJSON struct {
	Type     int32           `json:"type"`     // union 类型标记 / Union type tag
	Collider json.RawMessage `json:"collider"` // 强类型碰撞体 JSON / Typed collider JSON
}

// UnmarshalJSON 按 Type 解码已知碰撞体并拒绝未知 union 标记
// UnmarshalJSON decodes a known collider according to Type and rejects unknown union tags
func (c *ColliderRef) UnmarshalJSON(data []byte) error {
	var raw colliderRefJSON
	if err := decodeColliderJSONStrict(data, &raw); err != nil {
		return err
	}
	if _, known := newColliderStatusForType(raw.Type); !known {
		return fmt.Errorf("unsupported collider type %d", raw.Type)
	}
	var collider ColliderStatusUnion
	if !jsonMessageIsNullOrMissing(raw.Collider) {
		var err error
		collider, err = decodeColliderObjectAsType(raw.Collider, raw.Type)
		if err != nil {
			return err
		}
	}
	*c = ColliderRef{
		Type:     raw.Type,
		Collider: collider,
	}
	return nil
}

// MarshalJSON 编码碰撞体引用及其强类型载荷
// MarshalJSON encodes a collider reference and its typed payload
func (c ColliderRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(colliderRefAlias(c))
}

// ColliderStatusUnion 表示强类型碰撞体状态可实现的 union 接口
// ColliderStatusUnion represents the union interface implemented by typed collider states
type ColliderStatusUnion interface {
	toColliderType() int32
}

// ColliderObject 表示游戏 ANativeColliderStatus 的共用基类字段
// ColliderObject represents shared base fields from the game's ANativeColliderStatus
type ColliderObject struct {
	Version       int32   `json:"version"`       // 版本号 / Version value
	ParentName    *string `json:"parentName"`    // 可空父对象名称 / Nullable parent object name
	SelfName      *string `json:"selfName"`      // 可空自身对象名称 / Nullable own object name
	LocalPosition Vector3 `json:"localPosition"` // 本地位置 / Local position
	LocalRotation Vector4 `json:"localRotation"` // 本地旋转四元数 / Local rotation quaternion
	LocalScale    Vector3 `json:"localScale"`    // 本地缩放 / Local scale
	Center        Vector3 `json:"center"`        // 碰撞体中心 / Collider center
	Bound         int32   `json:"bound"`         // 修正方式：0=Outside,1=Inside / Correction mode: 0=Outside,1=Inside
}

// ColliderPlane 对应游戏 NativePlaneColliderStatus
// ColliderPlane corresponds to the game's NativePlaneColliderStatus
type ColliderPlane struct {
	_struct            struct{}          `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	ColliderObject     `codec:",inline"` // ANativeColliderStatus 基类字段 / ANativeColliderStatus base fields
	Direction          int32             `json:"direction"`          // 平面法线方向 / Plane normal direction
	IsDirectionInverse bool              `json:"isDirectionInverse"` // 法线方向反转 / Reverse normal direction
}

// toColliderType 返回 Plane union 类型标记
// toColliderType returns the Plane union type tag
func (*ColliderPlane) toColliderType() int32 { return ColliderTypePlane }

// ColliderCapsule 对应游戏 NativeCapsuleColliderStatus
// ColliderCapsule corresponds to the game's NativeCapsuleColliderStatus
type ColliderCapsule struct {
	_struct            struct{}          `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	ColliderObject     `codec:",inline"` // ANativeColliderStatus 基类字段 / ANativeColliderStatus base fields
	Direction          int32             `json:"direction"`          // 胶囊主轴方向 / Capsule axis direction
	IsDirectionInverse bool              `json:"isDirectionInverse"` // 方向反转 / Direction reversed
	StartRadius        float32           `json:"startRadius"`        // 起点半径 / Start radius
	EndRadius          float32           `json:"endRadius"`          // 终点半径 / End radius
	Height             float32           `json:"height"`             // 长度 / Height
}

// toColliderType 返回 Capsule union 类型标记
// toColliderType returns the Capsule union type tag
func (*ColliderCapsule) toColliderType() int32 { return ColliderTypeCapsule }

// ColliderSphere 对应游戏 NativeSphereColliderStatus
// ColliderSphere corresponds to the game's NativeSphereColliderStatus
type ColliderSphere struct {
	_struct        struct{}          `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	ColliderObject `codec:",inline"` // ANativeColliderStatus 基类字段 / ANativeColliderStatus base fields
	Radius         float32           `json:"radius"` // 半径 / Radius
}

// toColliderType 返回 Sphere union 类型标记
// toColliderType returns the Sphere union type tag
func (*ColliderSphere) toColliderType() int32 { return ColliderTypeSphere }

// ColliderMaidProp 对应游戏 NativeMaidPropColliderStatus
// ColliderMaidProp corresponds to the game's NativeMaidPropColliderStatus
type ColliderMaidProp struct {
	_struct                struct{}          `codec:",toarray" kces:"nil=13,14,15;widths=22,25"` // 强制按数组编码并接受枚举列表版及名称列表版布局 / Forces array encoding and accepts enum-list and name-list layouts
	ColliderObject         `codec:",inline"` // NativeCapsuleColliderStatus 与基类字段 / NativeCapsuleColliderStatus and base fields
	Direction              int32             `json:"direction"`              // 胶囊主轴方向 / Capsule axis direction
	IsDirectionInverse     bool              `json:"isDirectionInverse"`     // 方向反转 / Direction reversed
	StartRadius            float32           `json:"startRadius"`            // 起点半径 / Start radius
	EndRadius              float32           `json:"endRadius"`              // 终点半径 / End radius
	Height                 float32           `json:"height"`                 // 长度 / Height
	CenterMpnList          []int32           `json:"centerMpnList"`          // 中心MPN枚举列表，对应 C# List<MPN> / Center MPN enum list, matching C# List<MPN>
	CenterRateMax          Vector3           `json:"centerRateMax"`          // 中心最大比率 / Max center rate
	StartRadiusMpnList     []int32           `json:"startRadiusMpnList"`     // 起点半径MPN枚举列表 / Start-radius MPN enum list
	MaxStartRadius         float32           `json:"maxStartRadius"`         // 起点半径最大值 / Max start radius
	EndRadiusMpnList       []int32           `json:"endRadiusMpnList"`       // 终点半径MPN枚举列表 / End-radius MPN enum list
	MaxEndRadius           float32           `json:"maxEndRadius"`           // 终点半径最大值 / Max end radius
	CenterMpnNameList      []*string         `json:"centerMpnNameList"`      // 可空中心 MPN 名称列表 / List of nullable center MPN names
	StartRadiusMpnNameList []*string         `json:"startRadiusMpnNameList"` // 可空起点半径 MPN 名称列表 / List of nullable start-radius MPN names
	EndRadiusMpnNameList   []*string         `json:"endRadiusMpnNameList"`   // 可空终点半径 MPN 名称列表 / List of nullable end-radius MPN names
}

// toColliderType 返回 MaidPropCol union 类型标记
// toColliderType returns the MaidPropCol union type tag
func (*ColliderMaidProp) toColliderType() int32 { return ColliderTypeMaidProp }

// newColliderObject 创建使用游戏基类字段初始化默认值的碰撞体对象
// newColliderObject creates a collider object with the game's base-field initializer defaults
func newColliderObject(version int32) ColliderObject {
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
		CenterMpnList:          []int32{},
		StartRadiusMpnList:     []int32{},
		MaxStartRadius:         1,
		EndRadiusMpnList:       []int32{},
		MaxEndRadius:           1,
		CenterMpnNameList:      []*string{},
		StartRadiusMpnNameList: []*string{},
		EndRadiusMpnNameList:   []*string{},
	}
}

// UnmarshalJSON 解码 ColliderPlane 而不注入构造默认值
// UnmarshalJSON decodes ColliderPlane without injecting constructor defaults
func (v *ColliderPlane) UnmarshalJSON(data []byte) error {
	type colliderPlaneJSON ColliderPlane
	var value colliderPlaneJSON
	if err := decodeColliderJSONStrict(data, &value); err != nil {
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
	if err := decodeColliderJSONStrict(data, &value); err != nil {
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
	if err := decodeColliderJSONStrict(data, &value); err != nil {
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
	if err := decodeColliderJSONStrict(data, &value); err != nil {
		return err
	}
	*v = ColliderMaidProp(value)
	return nil
}

// ColliderState 表示 DynamicYureBone.LimbColliderInfo
// ColliderState represents DynamicYureBone.LimbColliderInfo
type ColliderState struct {
	_struct  struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version  int32    `json:"version"`   // 版本号 / Version value
	LimbType int32    `json:"limbType"`  // limbType 枚举值 / LimbType enum value
	IsEnable bool     `json:"isEnable"`  // 是否启用 / Enabled
}

// colliderStateJSON 表示 ColliderState 的 JSON 兼容输入
// colliderStateJSON represents JSON compatibility input for ColliderState
type colliderStateJSON struct {
	Version  int32           `json:"version"`            // 版本号 / Version value
	LimbType *int32          `json:"limbType,omitempty"` // 可选 limb 类型 / Optional limb type
	IsEnable *bool           `json:"isEnable,omitempty"` // 可选启用状态 / Optional enable state
	Index    json.RawMessage `json:"index,omitempty"`    // 仅用于拒绝旧版 index 字段 / Used only to reject the legacy index field
	Enabled  json.RawMessage `json:"enabled,omitempty"`  // 仅用于拒绝旧版 enabled 字段 / Used only to reject the legacy enabled field
}

// UnmarshalJSON 解码 ColliderState 并拒绝已移除的旧字段名
// UnmarshalJSON decodes ColliderState and rejects removed legacy field names
func (s *ColliderState) UnmarshalJSON(data []byte) error {
	var raw colliderStateJSON
	if err := decodeColliderJSONStrict(data, &raw); err != nil {
		return err
	}
	if len(raw.Index) > 0 {
		return fmt.Errorf(`collider state "index" is no longer supported; use "limbType"`)
	}
	if len(raw.Enabled) > 0 {
		return fmt.Errorf(`collider state "enabled" is no longer supported; use "isEnable"`)
	}
	value := ColliderState{Version: raw.Version}
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
	_struct struct{}            `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version int32               `json:"version"`   // 版本号 / Version value
	Items   []*LimbColliderItem `json:"items"`     // 可空 limb 碰撞体条目列表 / List of nullable limb collider items
}

// LimbColliderItem 表示一个 limb 类型和 NativeMaidPropColliderStatus
// LimbColliderItem represents one limb type and NativeMaidPropColliderStatus
type LimbColliderItem struct {
	Version  int32             `json:"version"`  // 版本号 / Version value
	Target   int32             `json:"target"`   // limbType 枚举值 / LimbType enum value
	Collider *ColliderMaidProp `json:"collider"` // 游戏声明的 NativeMaidPropColliderStatus / NativeMaidPropColliderStatus declared by the game
}

// limbColliderItemAlias 避免 LimbColliderItem.MarshalJSON 递归调用自身
// limbColliderItemAlias prevents LimbColliderItem.MarshalJSON from recursively calling itself
type limbColliderItemAlias LimbColliderItem

// IKColliderPackage 对应 IKColliderSaveLoader 保存的 IK collider 包
// IKColliderPackage maps the IK-collider package saved by IKColliderSaveLoader
type IKColliderPackage struct {
	_struct struct{}           `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version int32              `json:"version"`   // 版本号 / Version value
	Groups  []*IKColliderGroup `json:"groups"`    // 可空 IK 效果器分组列表 / List of nullable IK effector groups
}

// IKColliderGroup 表示一个 IK 效果器的碰撞体列表
// IKColliderGroup represents colliders for one IK effector
type IKColliderGroup struct {
	_struct   struct{}       `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version   int32          `json:"version"`   // 版本号 / Version value
	Target    int32          `json:"target"`    // effectorType 枚举值 / Effector type enum
	Colliders []*ColliderRef `json:"colliders"` // 可空碰撞体引用列表 / List of nullable collider references
}

// colliderRefWire 是依次保存 unionTag 和 concreteObject 的 MessagePack-CSharp union 封套
// 具体对象只在选择格式化器之前保持原始值，全部数组框架和元数据处理仍由共用 indexed-object 编解码器负责
// colliderRefWire is the MessagePack-CSharp union wrapper storing unionTag followed by concreteObject
// The concrete object remains raw only long enough to select its formatter, while all array framing and metadata handling stays in the shared indexed-object codec
type colliderRefWire struct {
	_struct  struct{}  `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type     int32     `json:"type"`      // union 类型标记 / Union type tag
	Collider codec.Raw `json:"-"`         // 仅在读取判别标记后选择具体类型 / Used only to select the concrete type after reading the discriminator
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
	_struct  struct{}  `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version  int32     `json:"version"`   // 条目版本 / Item version
	Target   int32     `json:"target"`    // limb 类型枚举 / Limb type enum
	Collider codec.Raw `json:"-"`         // 临时捕获 NativeMaidPropColliderStatus 值 / Temporarily captures the NativeMaidPropColliderStatus value
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

// CodecEncodeSelf 将 ColliderRef 的强类型载荷编码为 MessagePack-CSharp union
// CodecEncodeSelf encodes the typed payload of ColliderRef as a MessagePack-CSharp union
func (c ColliderRef) CodecEncodeSelf(e *codec.Encoder) {
	if _, known := newColliderStatusForType(c.Type); !known {
		panic(fmt.Errorf("unsupported collider type %d", c.Type))
	}
	if c.Collider != nil {
		actualType, err := validateColliderStatusForEncoding(c.Collider, "ColliderRef.collider")
		if err != nil {
			panic(err)
		}
		if c.Type != actualType {
			panic(fmt.Errorf("ColliderRef.type is %d, but collider concrete type requires %d", c.Type, actualType))
		}
	}
	raw, err := encodeColliderStatusSlot(c.Collider, "ColliderRef.collider")
	if err != nil {
		panic(err)
	}
	wire := colliderRefWire{
		Type:     c.Type,
		Collider: raw,
	}
	ct.EncodeIndexedObjectSelf(e, &wire)
}

// CodecDecodeSelf 解码 MessagePack-CSharp union 并拒绝未知类型标记
// CodecDecodeSelf decodes a MessagePack-CSharp union and rejects unknown type tags
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
	status, known := newColliderStatusForType(wire.Type)
	if !known {
		panic(fmt.Errorf("unsupported collider type %d", wire.Type))
	}
	result := ColliderRef{
		Type: wire.Type,
	}
	if len(wire.Collider) != 0 {
		if err := ct.DecodeMsgpack(wire.Collider, status); err != nil {
			panic(fmt.Errorf("decode ColliderRef type %d payload: %w", wire.Type, err))
		}
		result.Collider = status
	}
	*c = result
}

// CodecEncodeSelf 将 LimbColliderItem 的固定 MaidProp 碰撞体编码为三槽布局
// CodecEncodeSelf encodes the fixed MaidProp collider of LimbColliderItem in its three-slot layout
func (item LimbColliderItem) CodecEncodeSelf(e *codec.Encoder) {
	raw, err := encodeColliderStatusSlot(item.Collider, "LimbColliderItem.collider")
	if err != nil {
		panic(err)
	}
	wire := limbColliderItemWire{
		Version:  item.Version,
		Target:   item.Target,
		Collider: raw,
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
		Version: wire.Version,
		Target:  wire.Target,
	}
	if len(wire.Collider) != 0 {
		status := &ColliderMaidProp{}
		if err := ct.DecodeMsgpack(wire.Collider, status); err != nil {
			panic(fmt.Errorf("decode LimbColliderItem.collider as NativeMaidPropColliderStatus: %w", err))
		}
		result.Collider = status
	}
	*item = result
}

// encodeColliderStatusSlot 将强类型碰撞体转换为仅供判别解码使用的临时 MessagePack 槽位
// encodeColliderStatusSlot converts a typed collider into a temporary MessagePack slot used only for discriminated decoding
func encodeColliderStatusSlot(status ColliderStatusUnion, name string) (codec.Raw, error) {
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
	return codec.Raw(encoded), nil
}

// newColliderStatusForType 为已知 union 类型标记创建零值具体碰撞体
// newColliderStatusForType creates a zero-value concrete collider for a known union type tag
func newColliderStatusForType(typ int32) (ColliderStatusUnion, bool) {
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
func validateColliderStatusForEncoding(status ColliderStatusUnion, name string) (int32, error) {
	switch v := status.(type) {
	case *ColliderPlane:
		if v == nil {
			return -1, fmt.Errorf("%s is nil (*ColliderPlane)", name)
		}
		return ColliderTypePlane, nil
	case *ColliderCapsule:
		if v == nil {
			return -1, fmt.Errorf("%s is nil (*ColliderCapsule)", name)
		}
		return ColliderTypeCapsule, nil
	case *ColliderSphere:
		if v == nil {
			return -1, fmt.Errorf("%s is nil (*ColliderSphere)", name)
		}
		return ColliderTypeSphere, nil
	case *ColliderMaidProp:
		if v == nil {
			return -1, fmt.Errorf("%s is nil (*ColliderMaidProp)", name)
		}
		return ColliderTypeMaidProp, nil
	default:
		return -1, fmt.Errorf("%s has unsupported type %T", name, status)
	}
}

// validateColliderRefsForEncoding 验证碰撞体引用的 union 标记和载荷一致性
// validateColliderRefsForEncoding validates union tags and payload consistency in collider references
func validateColliderRefsForEncoding(refs []*ColliderRef, name string) error {
	for i := range refs {
		refName := fmt.Sprintf("%s[%d]", name, i)
		if refs[i] == nil {
			continue
		}
		if _, known := newColliderStatusForType(refs[i].Type); !known {
			return fmt.Errorf("unsupported collider type %d", refs[i].Type)
		}
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
	if err := validateColliderRefsForEncoding(p.Colliders, "colliderPackage.colliders"); err != nil {
		return err
	}
	return nil
}

// validateLimbColliderPackageForEncoding 验证 LimbColliderMgr 包及其固定 MaidProp 碰撞体条目
// validateLimbColliderPackageForEncoding validates a LimbColliderMgr package and its fixed MaidProp collider items
func validateLimbColliderPackageForEncoding(_ *LimbColliderPackage) error {
	return nil
}

// validateIKColliderPackageForEncoding 验证 IK 碰撞体包、效果器分组和 union 引用
// validateIKColliderPackageForEncoding validates an IK-collider package, effector groups, and union references
func validateIKColliderPackageForEncoding(p *IKColliderPackage) error {
	for i := range p.Groups {
		if p.Groups[i] == nil {
			continue
		}
		groupName := fmt.Sprintf("ikColliderPackage.groups[%d]", i)
		if err := validateColliderRefsForEncoding(p.Groups[i].Colliders, groupName+".colliders"); err != nil {
			return err
		}
	}
	return nil
}

// cloneColliderStatusForEncoding 深复制具体碰撞体中的切片
// cloneColliderStatusForEncoding deep-clones slices in a concrete collider
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
func validateMaidPropMPNList(values []int32, name string) error {
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

// cloneColliderRefsForEncoding 深复制碰撞体引用的强类型载荷
// cloneColliderRefsForEncoding deep-clones typed payloads in collider references
func cloneColliderRefsForEncoding(refs []*ColliderRef) []*ColliderRef {
	cloned := cloneSlicePreserveNil(refs)
	for i := range cloned {
		if cloned[i] == nil {
			continue
		}
		ref := *cloned[i]
		ref.Collider = cloneColliderStatusForEncoding(ref.Collider)
		cloned[i] = &ref
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
		if cloned.Items[i] == nil {
			continue
		}
		item := *cloned.Items[i]
		cloned.Items[i] = &item
		if item.Collider != nil {
			collider := *item.Collider
			cloned.Items[i].Collider = &collider
			cloned.Items[i].Collider.CenterMpnList = cloneSlicePreserveNil(item.Collider.CenterMpnList)
			cloned.Items[i].Collider.StartRadiusMpnList = cloneSlicePreserveNil(item.Collider.StartRadiusMpnList)
			cloned.Items[i].Collider.EndRadiusMpnList = cloneSlicePreserveNil(item.Collider.EndRadiusMpnList)
			cloned.Items[i].Collider.CenterMpnNameList = cloneSlicePreserveNil(item.Collider.CenterMpnNameList)
			cloned.Items[i].Collider.StartRadiusMpnNameList = cloneSlicePreserveNil(item.Collider.StartRadiusMpnNameList)
			cloned.Items[i].Collider.EndRadiusMpnNameList = cloneSlicePreserveNil(item.Collider.EndRadiusMpnNameList)
		}
	}
	return &cloned
}

// normalizeIKColliderPackageForEncoding 复制 IK 分组及其碰撞体引用供编码使用
// normalizeIKColliderPackageForEncoding copies IK groups and their collider references for encoding
func normalizeIKColliderPackageForEncoding(p *IKColliderPackage) *IKColliderPackage {
	cloned := *p
	cloned.Groups = cloneSlicePreserveNil(p.Groups)
	for i := range cloned.Groups {
		if cloned.Groups[i] == nil {
			continue
		}
		group := *cloned.Groups[i]
		group.Colliders = cloneColliderRefsForEncoding(group.Colliders)
		cloned.Groups[i] = &group
	}
	return &cloned
}

// decodeColliderObjectAsType 按 union 类型标记将 JSON 解码为具体碰撞体
// decodeColliderObjectAsType decodes JSON into a concrete collider according to its union type tag
func decodeColliderObjectAsType(raw json.RawMessage, typ int32) (ColliderStatusUnion, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("collider is required")
	}
	switch typ {
	case ColliderTypePlane:
		var status ColliderPlane
		if err := decodeColliderJSONStrict(raw, &status); err != nil {
			return nil, err
		}
		return &status, nil
	case ColliderTypeCapsule:
		var status ColliderCapsule
		if err := decodeColliderJSONStrict(raw, &status); err != nil {
			return nil, err
		}
		return &status, nil
	case ColliderTypeSphere:
		var status ColliderSphere
		if err := decodeColliderJSONStrict(raw, &status); err != nil {
			return nil, err
		}
		return &status, nil
	case ColliderTypeMaidProp:
		var status ColliderMaidProp
		if err := decodeColliderJSONStrict(raw, &status); err != nil {
			return nil, err
		}
		return &status, nil
	default:
		return nil, fmt.Errorf("unknown collider type %d", typ)
	}
}

// decodeColliderJSONStrict 严格解码唯一碰撞体 JSON 值并按 typed 模型校验 null
// decodeColliderJSONStrict strictly decodes one collider JSON value and validates null according to the typed model
func decodeColliderJSONStrict(data []byte, out any) error {
	return strictjson.Decode(data, out)
}
