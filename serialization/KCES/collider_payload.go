package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// .dbcol、.dslcol、.limbcol 与 .ikcol 系列共用的碰撞体结构、union 编码和校验逻辑。
//
// Collider structures, union encoding, and validation shared by .dbcol, .dslcol, .limbcol, and .ikcol families.

// ColliderPackage 表示通用碰撞体包 / ColliderPackage represents a generic collider package
type ColliderPackage struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int32           `json:"version"`                  // 版本号 / Version value
	Colliders              []ColliderRef   `json:"colliders"`                // 碰撞体引用列表 / Collider reference list
	LimbEnableList         []ColliderState `json:"limbEnableList,omitempty"` // DynamicYureBone.LimbColliderInfo 列表 / DynamicYureBone.LimbColliderInfo list
}

type colliderPackageJSON struct {
	*IndexedObjectMetadata
	Version        int32           `json:"version"`
	Colliders      []ColliderRef   `json:"colliders"`
	LimbEnableList []ColliderState `json:"limbEnableList,omitempty"`
	States         json.RawMessage `json:"states,omitempty"`
}

func (p ColliderPackage) MarshalJSON() ([]byte, error) {
	return json.Marshal(colliderPackageJSON{
		IndexedObjectMetadata: p.IndexedObjectMetadata,
		Version:               p.Version,
		Colliders:             p.Colliders,
		LimbEnableList:        p.LimbEnableList,
	})
}

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

// ColliderRef 表示带类型枚举的碰撞体引用 / ColliderRef represents a collider reference with its type enum
type ColliderRef struct {
	*IndexedObjectMetadata `codec:"-"`
	Type                   int32               `json:"type"`     // 碰撞体类型枚举 / Collider type enum
	Collider               ColliderStatusUnion `json:"collider"` // 碰撞体对象数据 / Collider object data
	// ColliderRaw preserves the concrete union payload when Type is newer than
	// this library. It is one complete MessagePack value encoded as base64 in
	// JSON. Known colliders use Collider and leave this field empty.
	ColliderRaw RawMessagePackSlot `json:"colliderRaw,omitempty" codec:"-"`
}

type colliderRefAlias ColliderRef
type colliderRefJSON struct {
	*IndexedObjectMetadata
	Type        int32              `json:"type"`
	Collider    json.RawMessage    `json:"collider"`
	ColliderRaw RawMessagePackSlot `json:"colliderRaw,omitempty"`
}

func (c *ColliderRef) UnmarshalJSON(data []byte) error {
	var raw colliderRefJSON
	if err := decodeColliderJSONStrict(data, &raw); err != nil {
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

func (c ColliderRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(colliderRefAlias(c))
}

// ColliderStatusUnion 表示强类型的碰撞体状态 / Strongly-typed collider status
type ColliderStatusUnion interface {
	toColliderType() int32
}

// ColliderObject 表示游戏碰撞体基类字段 / ColliderObject represents shared base fields of game collider status
type ColliderObject struct {
	Version       int32   `json:"version"`       // 版本号 / Version value
	ParentName    string  `json:"parentName"`    // 父对象名称 / Parent object name
	SelfName      string  `json:"selfName"`      // 自身对象名称 / Own object name
	LocalPosition Vector3 `json:"localPosition"` // 本地位置 / Local position
	LocalRotation Vector4 `json:"localRotation"` // 本地旋转四元数 / Local rotation quaternion
	LocalScale    Vector3 `json:"localScale"`    // 本地缩放 / Local scale
	Center        Vector3 `json:"center"`        // 碰撞体中心 / Collider center
	Bound         int32   `json:"bound"`         // 修正方式：0=Outside,1=Inside / Correction mode: 0=Outside,1=Inside
}

type ColliderPlane struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	ColliderObject         `codec:",inline"`
	Direction              int32 `json:"direction"`          // 平面法线方向 / Plane normal direction
	IsDirectionInverse     bool  `json:"isDirectionInverse"` // 法线方向反转 / Reverse normal direction
}

func (*ColliderPlane) toColliderType() int32 { return ColliderTypePlane }

type ColliderCapsule struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	ColliderObject         `codec:",inline"`
	Direction              int32   `json:"direction"`          // 胶囊主轴方向 / Capsule axis direction
	IsDirectionInverse     bool    `json:"isDirectionInverse"` // 方向反转 / Direction reversed
	StartRadius            float32 `json:"startRadius"`        // 起点半径 / Start radius
	EndRadius              float32 `json:"endRadius"`          // 终点半径 / End radius
	Height                 float32 `json:"height"`             // 长度 / Height
}

func (*ColliderCapsule) toColliderType() int32 { return ColliderTypeCapsule }

type ColliderSphere struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	ColliderObject         `codec:",inline"`
	Radius                 float32 `json:"radius"` // 半径 / Radius
}

func (*ColliderSphere) toColliderType() int32 { return ColliderTypeSphere }

type ColliderMaidProp struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	ColliderObject         `codec:",inline"`
	Direction              int32              `json:"direction"`              // 胶囊主轴方向 / Capsule axis direction
	IsDirectionInverse     bool               `json:"isDirectionInverse"`     // 方向反转 / Direction reversed
	StartRadius            float32            `json:"startRadius"`            // 起点半径 / Start radius
	EndRadius              float32            `json:"endRadius"`              // 终点半径 / End radius
	Height                 float32            `json:"height"`                 // 长度 / Height
	Reserved13             RawMessagePackSlot `json:"reserved13,omitempty"`   // C# has no Key(13); normally MessagePack nil
	Reserved14             RawMessagePackSlot `json:"reserved14,omitempty"`   // C# has no Key(14); normally MessagePack nil
	Reserved15             RawMessagePackSlot `json:"reserved15,omitempty"`   // C# has no Key(15); normally MessagePack nil
	CenterMpnList          []int32            `json:"centerMpnList"`          // 中心MPN枚举列表，对应 C# List<MPN> / Center MPN enum list, matching C# List<MPN>
	CenterRateMax          Vector3            `json:"centerRateMax"`          // 中心最大比率 / Max center rate
	StartRadiusMpnList     []int32            `json:"startRadiusMpnList"`     // 起点半径MPN枚举列表 / Start-radius MPN enum list
	MaxStartRadius         float32            `json:"maxStartRadius"`         // 起点半径最大值 / Max start radius
	EndRadiusMpnList       []int32            `json:"endRadiusMpnList"`       // 终点半径MPN枚举列表 / End-radius MPN enum list
	MaxEndRadius           float32            `json:"maxEndRadius"`           // 终点半径最大值 / Max end radius
	CenterMpnNameList      []string           `json:"centerMpnNameList"`      // 中心MPN名 / Center MPN names
	StartRadiusMpnNameList []string           `json:"startRadiusMpnNameList"` // 起点半径MPN名 / Start-radius MPN names
	EndRadiusMpnNameList   []string           `json:"endRadiusMpnNameList"`   // 终点半径MPN名 / End-radius MPN names
}

func (*ColliderMaidProp) toColliderType() int32 { return ColliderTypeMaidProp }

func newColliderObject(version int32) ColliderObject {
	return ColliderObject{
		Version:       version,
		LocalRotation: Vector4{W: 1},
		LocalScale:    Vector3{X: 1, Y: 1, Z: 1},
	}
}

// NewColliderPlane, NewColliderCapsule, NewColliderSphere and
// NewColliderMaidProp reproduce the current C# constructors/field
// initializers for callers explicitly creating a new object. Wire and JSON
// decoding always start from zero values and never run game migrations.
func NewColliderPlane() *ColliderPlane {
	return &ColliderPlane{
		ColliderObject: newColliderObject(colliderStatusFixVersion),
		Direction:      VectorTypeY,
	}
}

func NewColliderCapsule() *ColliderCapsule {
	return &ColliderCapsule{
		ColliderObject: newColliderObject(colliderStatusFixVersion),
		Direction:      VectorTypeY,
		StartRadius:    0.5,
		EndRadius:      0.5,
	}
}

func NewColliderSphere() *ColliderSphere {
	return &ColliderSphere{
		ColliderObject: newColliderObject(colliderStatusFixVersion),
		Radius:         0.5,
	}
}

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
		CenterMpnNameList:      []string{},
		StartRadiusMpnNameList: []string{},
		EndRadiusMpnNameList:   []string{},
	}
}

func (v *ColliderPlane) UnmarshalJSON(data []byte) error {
	type colliderPlaneJSON ColliderPlane
	var value colliderPlaneJSON
	if err := decodeColliderJSONStrict(data, &value); err != nil {
		return err
	}
	*v = ColliderPlane(value)
	return nil
}

func (v *ColliderCapsule) UnmarshalJSON(data []byte) error {
	type colliderCapsuleJSON ColliderCapsule
	var value colliderCapsuleJSON
	if err := decodeColliderJSONStrict(data, &value); err != nil {
		return err
	}
	*v = ColliderCapsule(value)
	return nil
}

func (v *ColliderSphere) UnmarshalJSON(data []byte) error {
	type colliderSphereJSON ColliderSphere
	var value colliderSphereJSON
	if err := decodeColliderJSONStrict(data, &value); err != nil {
		return err
	}
	*v = ColliderSphere(value)
	return nil
}

func (v *ColliderMaidProp) UnmarshalJSON(data []byte) error {
	type colliderMaidPropJSON ColliderMaidProp
	var value colliderMaidPropJSON
	if err := decodeColliderJSONStrict(data, &value); err != nil {
		return err
	}
	*v = ColliderMaidProp(value)
	return nil
}

// ColliderState 表示 DynamicYureBone.LimbColliderInfo。
// ColliderState represents DynamicYureBone.LimbColliderInfo.
type ColliderState struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int32 `json:"version"`  // 版本号 / Version value
	LimbType               int32 `json:"limbType"` // limbType 枚举值 / LimbType enum value
	IsEnable               bool  `json:"isEnable"` // 是否启用 / Enabled
}

type colliderStateJSON struct {
	*IndexedObjectMetadata
	Version  int32           `json:"version"`
	LimbType *int32          `json:"limbType,omitempty"`
	IsEnable *bool           `json:"isEnable,omitempty"`
	Index    json.RawMessage `json:"index,omitempty"`
	Enabled  json.RawMessage `json:"enabled,omitempty"`
}

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

// LimbColliderPackage 对应 LimbColliderMgr 保存的 limb collider 包 / LimbColliderPackage maps the package saved by LimbColliderMgr
type LimbColliderPackage struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int32              `json:"version"` // 版本号 / Version value
	Items                  []LimbColliderItem `json:"items"`   // limb 碰撞体条目列表 / limb collider item list
}

// LimbColliderItem 表示一个 limb 类型和碰撞体状态 / LimbColliderItem represents one limb type and collider status
type LimbColliderItem struct {
	*IndexedObjectMetadata `codec:"-"`
	Version                int32               `json:"version"`  // 版本号 / Version value
	Target                 int32               `json:"target"`   // limbType 枚举值 / LimbType enum value
	Collider               ColliderStatusUnion `json:"collider"` // 碰撞体状态 / Collider status
	ColliderRaw            RawMessagePackSlot  `json:"colliderRaw,omitempty" codec:"-"`
}

type limbColliderItemAlias LimbColliderItem
type limbColliderItemJSON struct {
	*IndexedObjectMetadata
	Version     int32              `json:"version"`
	Target      int32              `json:"target"`
	Collider    json.RawMessage    `json:"collider"`
	ColliderRaw RawMessagePackSlot `json:"colliderRaw,omitempty"`
}

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
		// C# LimbColliderMgr.LimbColliderData.Key(2) is a concrete
		// NativeMaidPropColliderStatus, not ANativeColliderStatus's union.
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

func (item LimbColliderItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(limbColliderItemAlias(item))
}

// IKColliderPackage 对应 IKColliderSaveLoader 保存的 IK collider 包 / IKColliderPackage maps the package saved by IKColliderSaveLoader
type IKColliderPackage struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int32             `json:"version"` // 版本号 / Version value
	Groups                 []IKColliderGroup `json:"groups"`  // IK 效果器分组列表 / IK effector group list
}

// IKColliderGroup 表示一个 IK 效果器的碰撞体列表 / IKColliderGroup represents colliders for one IK effector
type IKColliderGroup struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int32         `json:"version"`   // 版本号 / Version value
	Target                 int32         `json:"target"`    // effectorType 枚举值 / Effector type enum
	Colliders              []ColliderRef `json:"colliders"` // 该效果器关联的碰撞体引用列表 / Collider references associated with effector
}

// colliderRefWire is the MessagePack-CSharp union wrapper
// [unionTag, concreteObject]. The concrete object stays raw only long enough
// to select its formatter; all array framing and metadata handling remains in
// the shared indexed-object codec.
type colliderRefWire struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Type                   int32              `json:"type"`
	Collider               RawMessagePackSlot `json:"collider"`
}

func (v colliderRefWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

func (v *colliderRefWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

type limbColliderItemWire struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int32              `json:"version"`
	Target                 int32              `json:"target"`
	Collider               RawMessagePackSlot `json:"collider"`
}

func (v limbColliderItemWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

func (v *limbColliderItemWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

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

func (c *ColliderRef) CodecDecodeSelf(d *codec.Decoder) {
	var root codec.Raw
	d.MustDecode(&root)
	if len(root) == 0 { // null ANativeColliderStatus list element
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
			// A nil union tag cannot select a known formatter. Preserve the second
			// slot instead of guessing that the zero Go int32 meant Plane.
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

func (item *LimbColliderItem) CodecDecodeSelf(d *codec.Decoder) {
	var root codec.Raw
	d.MustDecode(&root)
	if len(root) == 0 { // null LimbColliderData list element
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
		return nil, nil // normal MessagePack nil
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

func indexedObjectSlotPresent(metadata *IndexedObjectMetadata, slot, known int64) bool {
	count := known
	if metadata != nil && metadata.FieldCount != nil {
		count = int64(*metadata.FieldCount)
	}
	return slot >= 0 && slot < count
}

func indexedObjectSlotIsNil(metadata *IndexedObjectMetadata, slot int64) bool {
	if metadata == nil {
		return false
	}
	for _, candidate := range metadata.NilSlots {
		if int64(candidate) == int64(slot) {
			return true
		}
	}
	return false
}

func cloneRawMessagePackSlot(raw RawMessagePackSlot) RawMessagePackSlot {
	if raw == nil {
		return nil
	}
	return append(RawMessagePackSlot(nil), raw...)
}

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

func validateColliderRefsForEncoding(refs []ColliderRef, metadata *IndexedObjectMetadata, slot int64, name string) error {
	for i := range refs {
		if indexedObjectNullElementAt(metadata, slot, int64(i)) {
			continue
		}
		refName := fmt.Sprintf("%s[%d]", name, i)
		if refs[i].Collider != nil && len(refs[i].ColliderRaw) != 0 {
			return fmt.Errorf("%s.collider and colliderRaw cannot both be populated", refName)
		}
		if len(refs[i].ColliderRaw) != 0 {
			if err := validateRawMessagePackSlot(refs[i].ColliderRaw, refName+".colliderRaw"); err != nil {
				return err
			}
			continue
		}
		// MessagePack-CSharp unions can carry nil. A short wrapper can also omit
		// the concrete slot; its FieldCount metadata distinguishes the cases.
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

func validateColliderPackageForEncoding(p *ColliderPackage) error {
	if err := validateColliderRefsForEncoding(p.Colliders, p.IndexedObjectMetadata, 1, "colliderPackage.colliders"); err != nil {
		return err
	}
	for i := range p.LimbEnableList {
		if indexedObjectNullElementAt(p.IndexedObjectMetadata, 2, int64(i)) {
			continue
		}
	}
	return nil
}

func validateLimbColliderPackageForEncoding(p *LimbColliderPackage) error {
	for i := range p.Items {
		if indexedObjectNullElementAt(p.IndexedObjectMetadata, 1, int64(i)) {
			continue
		}
		itemName := fmt.Sprintf("limbColliderPackage.items[%d]", i)
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

func validateIKColliderPackageForEncoding(p *IKColliderPackage) error {
	for i := range p.Groups {
		if indexedObjectNullElementAt(p.IndexedObjectMetadata, 1, int64(i)) {
			continue
		}
		groupName := fmt.Sprintf("ikColliderPackage.groups[%d]", i)
		if err := validateColliderRefsForEncoding(p.Groups[i].Colliders, p.Groups[i].IndexedObjectMetadata, 2, groupName+".colliders"); err != nil {
			return err
		}
	}
	return nil
}

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

func cloneColliderRefsForEncoding(refs []ColliderRef) []ColliderRef {
	cloned := cloneSlicePreserveNil(refs)
	for i := range cloned {
		cloned[i].Collider = cloneColliderStatusForEncoding(cloned[i].Collider)
		cloned[i].ColliderRaw = cloneRawMessagePackSlot(cloned[i].ColliderRaw)
	}
	return cloned
}

func normalizeColliderPackageForEncoding(p *ColliderPackage) *ColliderPackage {
	cloned := *p
	cloned.Colliders = cloneColliderRefsForEncoding(p.Colliders)
	cloned.LimbEnableList = cloneSlicePreserveNil(p.LimbEnableList)
	return &cloned
}

func normalizeLimbColliderPackageForEncoding(p *LimbColliderPackage) *LimbColliderPackage {
	cloned := *p
	cloned.Items = cloneSlicePreserveNil(p.Items)
	for i := range cloned.Items {
		cloned.Items[i].Collider = cloneColliderStatusForEncoding(cloned.Items[i].Collider)
		cloned.Items[i].ColliderRaw = cloneRawMessagePackSlot(cloned.Items[i].ColliderRaw)
	}
	return &cloned
}

func indexedObjectNullElementAt(metadata *IndexedObjectMetadata, slot, element int64) bool {
	if metadata == nil || metadata.NullElements == nil {
		return false
	}
	flags := metadata.NullElements[int32(slot)]
	return element >= 0 && element < int64(len(flags)) && flags[element]
}

func normalizeIKColliderPackageForEncoding(p *IKColliderPackage) *IKColliderPackage {
	cloned := *p
	cloned.Groups = cloneSlicePreserveNil(p.Groups)
	for i := range cloned.Groups {
		cloned.Groups[i].Colliders = cloneColliderRefsForEncoding(p.Groups[i].Colliders)
	}
	return &cloned
}

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

func decodeColliderJSONStrict(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("trailing content: %w", err)
	}
	return nil
}
