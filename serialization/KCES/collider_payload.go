package KCES

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ColliderPackage 表示通用碰撞体包 / ColliderPackage represents a generic collider package
type ColliderPackage struct {
	Version   int             `json:"version"`          // 版本号 / Version value
	Colliders []ColliderRef   `json:"colliders"`        // 碰撞体引用列表 / Collider reference list
	States    []ColliderState `json:"states,omitempty"` // 可选启用状态列表 / Optional enabled-state list
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
	Type     int                 `json:"type"`     // 碰撞体类型枚举 / Collider type enum
	Collider ColliderStatusUnion `json:"collider"` // 碰撞体对象数据 / Collider object data
}

type colliderRefAlias ColliderRef
type colliderRefJSON struct {
	Type     int             `json:"type"`
	Collider json.RawMessage `json:"collider"`
}

func (c *ColliderRef) UnmarshalJSON(data []byte) error {
	var raw colliderRefJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var collider ColliderStatusUnion
	switch raw.Type {
	case ColliderTypePlane:
		collider = &ColliderPlane{}
	case ColliderTypeCapsule:
		collider = &ColliderCapsule{}
	case ColliderTypeSphere:
		collider = &ColliderSphere{}
	case ColliderTypeMaidProp:
		collider = &ColliderMaidProp{}
	default:
		return fmt.Errorf("unknown collider type: %d", raw.Type)
	}
	if err := json.Unmarshal(raw.Collider, collider); err != nil {
		return err
	}
	*c = ColliderRef{
		Type:     raw.Type,
		Collider: collider,
	}
	return nil
}

func (c ColliderRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(colliderRefAlias(c))
}

// ColliderStatusUnion 表示强类型的碰撞体状态 / Strongly-typed collider status
type ColliderStatusUnion interface {
	toColliderType() int
}

// ColliderObject 表示游戏碰撞体基类字段 / ColliderObject represents shared base fields of game collider status
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

type ColliderPlane struct {
	ColliderObject
	Direction          int  `json:"direction"`          // 平面法线方向 / Plane normal direction
	IsDirectionInverse bool `json:"isDirectionInverse"` // 法线方向反转 / Reverse normal direction
}

func (*ColliderPlane) toColliderType() int { return ColliderTypePlane }

type ColliderCapsule struct {
	ColliderObject
	Direction          int     `json:"direction"`          // 胶囊主轴方向 / Capsule axis direction
	IsDirectionInverse bool    `json:"isDirectionInverse"` // 方向反转 / Direction reversed
	StartRadius        float32 `json:"startRadius"`        // 起点半径 / Start radius
	EndRadius          float32 `json:"endRadius"`          // 终点半径 / End radius
	Height             float32 `json:"height"`             // 长度 / Height
}

func (*ColliderCapsule) toColliderType() int { return ColliderTypeCapsule }

type ColliderSphere struct {
	ColliderObject
	Radius float32 `json:"radius"` // 半径 / Radius
}

func (*ColliderSphere) toColliderType() int { return ColliderTypeSphere }

type ColliderMaidProp struct {
	ColliderObject
	Direction              int      `json:"direction"`              // 胶囊主轴方向 / Capsule axis direction
	IsDirectionInverse     bool     `json:"isDirectionInverse"`     // 方向反转 / Direction reversed
	StartRadius            float32  `json:"startRadius"`            // 起点半径 / Start radius
	EndRadius              float32  `json:"endRadius"`              // 终点半径 / End radius
	Height                 float32  `json:"height"`                 // 长度 / Height
	CenterMpnList          []string `json:"centerMpnList"`          // 中心MPN列表（当前文件序列化通常已兼容到字符串）/ Center MPN list (string-oriented)
	CenterRateMax          Vector3  `json:"centerRateMax"`          // 中心最大比率 / Max center rate
	StartRadiusMpnList     []string `json:"startRadiusMpnList"`     // 起点半径MPN列表 / Start-radius MPN list
	MaxStartRadius         float32  `json:"maxStartRadius"`         // 起点半径最大值 / Max start radius
	EndRadiusMpnList       []string `json:"endRadiusMpnList"`       // 终点半径MPN列表 / End-radius MPN list
	MaxEndRadius           float32  `json:"maxEndRadius"`           // 终点半径最大值 / Max end radius
	CenterMpnNameList      []string `json:"centerMpnNameList"`      // 中心MPN名 / Center MPN names
	StartRadiusMpnNameList []string `json:"startRadiusMpnNameList"` // 起点半径MPN名 / Start-radius MPN names
	EndRadiusMpnNameList   []string `json:"endRadiusMpnNameList"`   // 终点半径MPN名 / End-radius MPN names
}

func (*ColliderMaidProp) toColliderType() int { return ColliderTypeMaidProp }

// ColliderState 表示碰撞体启用状态 / Collider state
type ColliderState struct {
	Version int  `json:"version"` // 版本号 / Version value
	Index   int  `json:"index"`   // 对应碰撞体索引 / Referenced collider index
	Enabled bool `json:"enabled"` // 是否启用 / Enabled
}

// LimbColliderPackage 对应 LimbColliderMgr 保存的 limb collider 包 / LimbColliderPackage maps the package saved by LimbColliderMgr
type LimbColliderPackage struct {
	Version int                `json:"version"` // 版本号 / Version value
	Items   []LimbColliderItem `json:"items"`   // limb 碰撞体条目列表 / limb collider item list
}

// LimbColliderItem 表示一个 limb 类型和碰撞体状态 / LimbColliderItem represents one limb type and collider status
type LimbColliderItem struct {
	Version  int                 `json:"version"`  // 版本号 / Version value
	Target   int                 `json:"target"`   // limbType 枚举值 / LimbType enum value
	Collider ColliderStatusUnion `json:"collider"` // 碰撞体状态 / Collider status
}

type limbColliderItemAlias LimbColliderItem
type limbColliderItemJSON struct {
	Version  int             `json:"version"`
	Target   int             `json:"target"`
	Collider json.RawMessage `json:"collider"`
}

func (item *LimbColliderItem) UnmarshalJSON(data []byte) error {
	var raw limbColliderItemJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	collider, err := decodeColliderObjectAsType(raw.Collider, inferColliderTypeFromJSON(raw.Collider))
	if err != nil {
		return err
	}
	*item = LimbColliderItem{
		Version:  raw.Version,
		Target:   raw.Target,
		Collider: collider,
	}
	return nil
}

func (item LimbColliderItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(limbColliderItemAlias(item))
}

// IKColliderPackage 对应 IKColliderSaveLoader 保存的 IK collider 包 / IKColliderPackage maps the package saved by IKColliderSaveLoader
type IKColliderPackage struct {
	Version int               `json:"version"` // 版本号 / Version value
	Groups  []IKColliderGroup `json:"groups"`  // IK 效果器分组列表 / IK effector group list
}

// IKColliderGroup 表示一个 IK 效果器的碰撞体列表 / IKColliderGroup represents colliders for one IK effector
type IKColliderGroup struct {
	Version   int           `json:"version"`   // 版本号 / Version value
	Target    int           `json:"target"`    // effectorType 枚举值 / Effector type enum
	Colliders []ColliderRef `json:"colliders"` // 该效果器关联的碰撞体引用列表 / Collider references associated with effector
}

func decodeColliderPackageRaw(raw interface{}) (*ColliderPackage, error) {
	arr, err := asRawArray(raw, "ColliderPackage")
	if err != nil {
		return nil, err
	}
	if len(arr) < 2 {
		return nil, fmt.Errorf("ColliderPackage: expected array(2+), got %d", len(arr))
	}

	version, err := rawInt(arr[0], "ColliderPackage.version")
	if err != nil {
		return nil, err
	}
	colliders, err := decodeColliderRefsRaw(arr[1], "ColliderPackage.colliders")
	if err != nil {
		return nil, err
	}

	var states []ColliderState
	if len(arr) > 2 && arr[2] != nil {
		states, err = decodeColliderStatesRaw(arr[2], "ColliderPackage.states")
		if err != nil {
			return nil, err
		}
	}

	return &ColliderPackage{
		Version:   version,
		Colliders: colliders,
		States:    states,
	}, nil
}

func (p *ColliderPackage) toRaw() []interface{} {
	if p == nil {
		return nil
	}
	return []interface{}{
		int64(p.Version),
		colliderRefsToRaw(p.Colliders),
		colliderStatesToRaw(p.States),
	}
}

func decodeLimbColliderPackageRaw(raw interface{}) (*LimbColliderPackage, error) {
	arr, err := asRawArray(raw, "LimbColliderPackage")
	if err != nil {
		return nil, err
	}
	if len(arr) < 2 {
		return nil, fmt.Errorf("LimbColliderPackage: expected array(2+), got %d", len(arr))
	}
	version, err := rawInt(arr[0], "LimbColliderPackage.version")
	if err != nil {
		return nil, err
	}
	itemsArr, err := asRawArray(arr[1], "LimbColliderPackage.items")
	if err != nil {
		return nil, err
	}

	items := make([]LimbColliderItem, 0, len(itemsArr))
	for i, itemRaw := range itemsArr {
		itemArr, err := asRawArray(itemRaw, fmt.Sprintf("LimbColliderPackage.items[%d]", i))
		if err != nil {
			return nil, err
		}
		if len(itemArr) < 3 {
			return nil, fmt.Errorf("LimbColliderPackage.items[%d]: expected array(3+), got %d", i, len(itemArr))
		}
		itemVersion, err := rawInt(itemArr[0], fmt.Sprintf("LimbColliderPackage.items[%d].version", i))
		if err != nil {
			return nil, err
		}
		target, err := rawInt(itemArr[1], fmt.Sprintf("LimbColliderPackage.items[%d].target", i))
		if err != nil {
			return nil, err
		}
		colliderArr, err := asRawArray(itemArr[2], fmt.Sprintf("LimbColliderPackage.items[%d].collider", i))
		if err != nil {
			return nil, err
		}
		collider, err := decodeColliderObjectRaw(colliderArr, inferColliderTypeFromArray(colliderArr), fmt.Sprintf("LimbColliderPackage.items[%d].collider", i))
		if err != nil {
			return nil, err
		}
		items = append(items, LimbColliderItem{Version: itemVersion, Target: target, Collider: collider})
	}

	return &LimbColliderPackage{Version: version, Items: items}, nil
}

func (p *LimbColliderPackage) toRaw() []interface{} {
	if p == nil {
		return nil
	}
	items := make([]interface{}, 0, len(p.Items))
	for i := range p.Items {
		item := &p.Items[i]
		items = append(items, []interface{}{
			int64(item.Version),
			int64(item.Target),
			colliderStatusToRaw(item.Collider),
		})
	}
	return []interface{}{int64(p.Version), items}
}

func decodeIKColliderPackageRaw(raw interface{}) (*IKColliderPackage, error) {
	arr, err := asRawArray(raw, "IKColliderPackage")
	if err != nil {
		return nil, err
	}
	if len(arr) < 2 {
		return nil, fmt.Errorf("IKColliderPackage: expected array(2+), got %d", len(arr))
	}
	version, err := rawInt(arr[0], "IKColliderPackage.version")
	if err != nil {
		return nil, err
	}
	groupsArr, err := asRawArray(arr[1], "IKColliderPackage.groups")
	if err != nil {
		return nil, err
	}

	groups := make([]IKColliderGroup, 0, len(groupsArr))
	for i, groupRaw := range groupsArr {
		groupArr, err := asRawArray(groupRaw, fmt.Sprintf("IKColliderPackage.groups[%d]", i))
		if err != nil {
			return nil, err
		}
		if len(groupArr) < 3 {
			return nil, fmt.Errorf("IKColliderPackage.groups[%d]: expected array(3+), got %d", i, len(groupArr))
		}
		groupVersion, err := rawInt(groupArr[0], fmt.Sprintf("IKColliderPackage.groups[%d].version", i))
		if err != nil {
			return nil, err
		}
		target, err := rawInt(groupArr[1], fmt.Sprintf("IKColliderPackage.groups[%d].target", i))
		if err != nil {
			return nil, err
		}
		colliders, err := decodeColliderRefsRaw(groupArr[2], fmt.Sprintf("IKColliderPackage.groups[%d].colliders", i))
		if err != nil {
			return nil, err
		}
		groups = append(groups, IKColliderGroup{Version: groupVersion, Target: target, Colliders: colliders})
	}

	return &IKColliderPackage{Version: version, Groups: groups}, nil
}

func (p *IKColliderPackage) toRaw() []interface{} {
	if p == nil {
		return nil
	}
	groups := make([]interface{}, 0, len(p.Groups))
	for i := range p.Groups {
		group := &p.Groups[i]
		groups = append(groups, []interface{}{
			int64(group.Version),
			int64(group.Target),
			colliderRefsToRaw(group.Colliders),
		})
	}
	return []interface{}{int64(p.Version), groups}
}

func decodeColliderRefsRaw(raw interface{}, name string) ([]ColliderRef, error) {
	refsArr, err := asRawArray(raw, name)
	if err != nil {
		return nil, err
	}
	refs := make([]ColliderRef, 0, len(refsArr))
	for i, refRaw := range refsArr {
		refArr, err := asRawArray(refRaw, fmt.Sprintf("%s[%d]", name, i))
		if err != nil {
			return nil, err
		}
		if len(refArr) < 2 {
			return nil, fmt.Errorf("%s[%d]: expected array(2+), got %d", name, i, len(refArr))
		}
		typ, err := rawInt(refArr[0], fmt.Sprintf("%s[%d].type", name, i))
		if err != nil {
			return nil, err
		}
		colliderArr, err := asRawArray(refArr[1], fmt.Sprintf("%s[%d].collider", name, i))
		if err != nil {
			return nil, err
		}
		collider, err := decodeColliderObjectRaw(colliderArr, typ, fmt.Sprintf("%s[%d].collider", name, i))
		if err != nil {
			return nil, err
		}
		refs = append(refs, ColliderRef{Type: typ, Collider: collider})
	}
	return refs, nil
}

func colliderRefsToRaw(refs []ColliderRef) []interface{} {
	out := make([]interface{}, 0, len(refs))
	for i := range refs {
		ref := &refs[i]
		out = append(out, []interface{}{int64(ref.Type), colliderStatusToRaw(ref.Collider)})
	}
	return out
}

func decodeColliderObjectRaw(arr []interface{}, typ int, name string) (ColliderStatusUnion, error) {
	if arr == nil {
		return nil, fmt.Errorf("%s: collider raw array is nil", name)
	}
	if len(arr) < 8 {
		return nil, fmt.Errorf("%s: expected array(8+), got %d", name, len(arr))
	}
	version, err := rawInt(arr[0], name+".version")
	if err != nil {
		return nil, err
	}
	parentName, err := rawString(arr[1], name+".parentName")
	if err != nil {
		return nil, err
	}
	selfName, err := rawString(arr[2], name+".selfName")
	if err != nil {
		return nil, err
	}
	localPosition, err := rawVector3(arr[3], name+".localPosition")
	if err != nil {
		return nil, err
	}
	localRotation, err := rawVector4(arr[4], name+".localRotation")
	if err != nil {
		return nil, err
	}
	localScale, err := rawVector3(arr[5], name+".localScale")
	if err != nil {
		return nil, err
	}
	center, err := rawVector3(arr[6], name+".center")
	if err != nil {
		return nil, err
	}
	bound, err := rawInt(arr[7], name+".bound")
	if err != nil {
		return nil, err
	}
	base := ColliderObject{
		Version:       version,
		ParentName:    parentName,
		SelfName:      selfName,
		LocalPosition: localPosition,
		LocalRotation: localRotation,
		LocalScale:    localScale,
		Center:        center,
		Bound:         bound,
	}

	switch typ {
	case ColliderTypePlane:
		if len(arr) < 10 {
			return nil, fmt.Errorf("%s: expected array(10), got %d for Plane", name, len(arr))
		}
		direction, err := rawInt(arr[8], name+".direction")
		if err != nil {
			return nil, err
		}
		isDirectionInverse, err := rawBool(arr[9], name+".isDirectionInverse")
		if err != nil {
			return nil, err
		}
		return &ColliderPlane{
			ColliderObject:     base,
			Direction:          direction,
			IsDirectionInverse: isDirectionInverse,
		}, nil
	case ColliderTypeCapsule:
		if len(arr) < 13 {
			return nil, fmt.Errorf("%s: expected array(13), got %d for Capsule", name, len(arr))
		}
		direction, err := rawInt(arr[8], name+".direction")
		if err != nil {
			return nil, err
		}
		isDirectionInverse, err := rawBool(arr[9], name+".isDirectionInverse")
		if err != nil {
			return nil, err
		}
		startRadius, err := rawFloat32(arr[10], name+".startRadius")
		if err != nil {
			return nil, err
		}
		endRadius, err := rawFloat32(arr[11], name+".endRadius")
		if err != nil {
			return nil, err
		}
		height, err := rawFloat32(arr[12], name+".height")
		if err != nil {
			return nil, err
		}
		return &ColliderCapsule{
			ColliderObject:     base,
			Direction:          direction,
			IsDirectionInverse: isDirectionInverse,
			StartRadius:        startRadius,
			EndRadius:          endRadius,
			Height:             height,
		}, nil
	case ColliderTypeSphere:
		if len(arr) < 9 {
			return nil, fmt.Errorf("%s: expected array(9), got %d for Sphere", name, len(arr))
		}
		radius, err := rawFloat32(arr[8], name+".radius")
		if err != nil {
			return nil, err
		}
		return &ColliderSphere{
			ColliderObject: base,
			Radius:         radius,
		}, nil
case ColliderTypeMaidProp:
		if len(arr) < 19 {
			return nil, fmt.Errorf("%s: expected array(19+), got %d for MaidProp", name, len(arr))
		}
		direction, err := rawInt(arr[8], name+".direction")
		if err != nil {
			return nil, err
		}
		isDirectionInverse, err := rawBool(arr[9], name+".isDirectionInverse")
		if err != nil {
			return nil, err
		}
		startRadius, err := rawFloat32(arr[10], name+".startRadius")
		if err != nil {
			return nil, err
		}
		endRadius, err := rawFloat32(arr[11], name+".endRadius")
		if err != nil {
			return nil, err
		}
		height, err := rawFloat32(arr[12], name+".height")
		if err != nil {
			return nil, err
		}
		fieldOffset := 13
		if looksLikeMaidPropColliderStatusAt(arr, 16) {
			fieldOffset = 16
		} else if !looksLikeMaidPropColliderStatusAt(arr, 13) {
			return nil, fmt.Errorf("%s: invalid MaidProp collider payload", name)
		}

		centerMpnList, err := rawStringArrayAt(arr, fieldOffset, name+".centerMpnList")
		if err != nil {
			return nil, err
		}
		centerRateMax, err := rawVector3At(arr, fieldOffset+1, name+".centerRateMax")
		if err != nil {
			return nil, err
		}
		startRadiusMpnList, err := rawStringArrayAt(arr, fieldOffset+2, name+".startRadiusMpnList")
		if err != nil {
			return nil, err
		}
		maxStartRadius, err := rawFloat32At(arr, fieldOffset+3, name+".maxStartRadius")
		if err != nil {
			return nil, err
		}
		endRadiusMpnList, err := rawStringArrayAt(arr, fieldOffset+4, name+".endRadiusMpnList")
		if err != nil {
			return nil, err
		}
		maxEndRadius, err := rawFloat32At(arr, fieldOffset+5, name+".maxEndRadius")
		if err != nil {
			return nil, err
		}
		centerMpnNameList := []string{}
		startRadiusMpnNameList := []string{}
		endRadiusMpnNameList := []string{}
		if version >= 1002 {
			centerMpnNameList, err = rawStringArrayAt(arr, fieldOffset+6, name+".centerMpnNameList")
			if err != nil {
				return nil, err
			}
			startRadiusMpnNameList, err = rawStringArrayAt(arr, fieldOffset+7, name+".startRadiusMpnNameList")
			if err != nil {
				return nil, err
			}
			endRadiusMpnNameList, err = rawStringArrayAt(arr, fieldOffset+8, name+".endRadiusMpnNameList")
			if err != nil {
				return nil, err
			}
		}
		return &ColliderMaidProp{
			ColliderObject:         base,
			Direction:              direction,
			IsDirectionInverse:     isDirectionInverse,
			StartRadius:            startRadius,
			EndRadius:              endRadius,
			Height:                 height,
			CenterMpnList:          centerMpnList,
			CenterRateMax:          centerRateMax,
			StartRadiusMpnList:     startRadiusMpnList,
			MaxStartRadius:         maxStartRadius,
			EndRadiusMpnList:       endRadiusMpnList,
			MaxEndRadius:           maxEndRadius,
			CenterMpnNameList:      centerMpnNameList,
			StartRadiusMpnNameList: startRadiusMpnNameList,
			EndRadiusMpnNameList:   endRadiusMpnNameList,
		}, nil
	default:
		return nil, fmt.Errorf("%s: unknown collider type %d", name, typ)
	}
}

func colliderStatusToRaw(status ColliderStatusUnion) []interface{} {
	switch v := status.(type) {
	case *ColliderPlane:
		return colliderPlaneToRaw(v)
	case *ColliderCapsule:
		return colliderCapsuleToRaw(v)
	case *ColliderSphere:
		return colliderSphereToRaw(v)
	case *ColliderMaidProp:
		return colliderMaidPropToRaw(v)
	default:
		panic(fmt.Sprintf("unsupported collider status type: %T", status))
	}
}

func colliderPlaneToRaw(v *ColliderPlane) []interface{} {
	out := colliderBaseToRaw(&v.ColliderObject)
	out = append(out, int64(v.Direction), v.IsDirectionInverse)
	return out
}

func colliderCapsuleToRaw(v *ColliderCapsule) []interface{} {
	out := colliderBaseToRaw(&v.ColliderObject)
	out = append(out, int64(v.Direction), v.IsDirectionInverse, v.StartRadius, v.EndRadius, v.Height)
	return out
}

func colliderSphereToRaw(v *ColliderSphere) []interface{} {
	out := colliderBaseToRaw(&v.ColliderObject)
	out = append(out, v.Radius)
	return out
}

func colliderMaidPropToRaw(v *ColliderMaidProp) []interface{} {
	out := colliderCapsuleToRaw(&ColliderCapsule{
		ColliderObject:     v.ColliderObject,
		Direction:          v.Direction,
		IsDirectionInverse: v.IsDirectionInverse,
		StartRadius:        v.StartRadius,
		EndRadius:          v.EndRadius,
		Height:             v.Height,
	})
	if v.Version == 1001 {
		out = append(out, nil, nil, nil)
	}
	out = append(
		out,
		stringSliceToRaw(v.CenterMpnList),
		vector3ToRaw(v.CenterRateMax),
		stringSliceToRaw(v.StartRadiusMpnList),
		v.MaxStartRadius,
		stringSliceToRaw(v.EndRadiusMpnList),
		v.MaxEndRadius,
	)
	if v.Version >= 1002 {
		out = append(
			out,
			stringSliceToRaw(v.CenterMpnNameList),
			stringSliceToRaw(v.StartRadiusMpnNameList),
			stringSliceToRaw(v.EndRadiusMpnNameList),
		)
	}
	return out
}

func colliderBaseToRaw(base *ColliderObject) []interface{} {
	return []interface{}{
		int64(base.Version),
		base.ParentName,
		base.SelfName,
		vector3ToRaw(base.LocalPosition),
		vector4ToRaw(base.LocalRotation),
		vector3ToRaw(base.LocalScale),
		vector3ToRaw(base.Center),
		base.Bound,
	}
}

func decodeColliderStatesRaw(raw interface{}, name string) ([]ColliderState, error) {
	statesArr, err := asRawArray(raw, name)
	if err != nil {
		return nil, err
	}
	states := make([]ColliderState, 0, len(statesArr))
	for i, stateRaw := range statesArr {
		stateArr, err := asRawArray(stateRaw, fmt.Sprintf("%s[%d]", name, i))
		if err != nil {
			return nil, err
		}
		if len(stateArr) < 3 {
			return nil, fmt.Errorf("%s[%d]: expected array(3+), got %d", name, i, len(stateArr))
		}
		version, err := rawInt(stateArr[0], fmt.Sprintf("%s[%d].version", name, i))
		if err != nil {
			return nil, err
		}
		index, err := rawInt(stateArr[1], fmt.Sprintf("%s[%d].index", name, i))
		if err != nil {
			return nil, err
		}
		enabled, err := rawBool(stateArr[2], fmt.Sprintf("%s[%d].enabled", name, i))
		if err != nil {
			return nil, err
		}
		states = append(states, ColliderState{Version: version, Index: index, Enabled: enabled})
	}
	return states, nil
}

func colliderStatesToRaw(states []ColliderState) []interface{} {
	out := make([]interface{}, 0, len(states))
	for i := range states {
		state := &states[i]
		out = append(out, []interface{}{int64(state.Version), int64(state.Index), state.Enabled})
	}
	return out
}

func asRawArray(v interface{}, name string) ([]interface{}, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s: expected array, got %T", name, v)
	}
	return arr, nil
}

func rawInt(v interface{}, name string) (int, error) {
	if n, ok := toIntVal(v); ok {
		return n, nil
	}
	return 0, fmt.Errorf("%s: expected int, got %T", name, v)
}

func rawFloat32(v interface{}, name string) (float32, error) {
	f, ok := toFloat32(v)
	if !ok {
		return 0, fmt.Errorf("%s: expected float, got %T", name, v)
	}
	return f, nil
}

func rawString(v interface{}, name string) (string, error) {
	if s, ok := toStringVal(v); ok {
		return s, nil
	}
	return "", fmt.Errorf("%s: expected string, got %T", name, v)
}

func rawStringArray(v interface{}, name string) ([]string, error) {
	if v == nil {
		return []string{}, nil
	}
	arr, err := asRawArray(v, name)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(arr))
	for i, item := range arr {
		s := ""
		switch x := item.(type) {
		case string:
			s = x
		case int64, uint64, int, uint, float64, float32, json.Number:
			str, err := toJSONNumberString(item)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: expected string/number, got %T", name, i, item)
			}
			s = str
		default:
			return nil, fmt.Errorf("%s[%d]: expected string/number, got %T", name, i, item)
		}
		out = append(out, s)
	}
	return out, nil
}

func rawStringArrayAt(arr []interface{}, index int, name string) ([]string, error) {
	if index >= len(arr) {
		return []string{}, nil
	}
	return rawStringArray(arr[index], name)
}

func rawVector3At(arr []interface{}, index int, name string) (Vector3, error) {
	if index >= len(arr) {
		return Vector3{}, nil
	}
	return rawVector3(arr[index], name)
}

func rawFloat32At(arr []interface{}, index int, name string) (float32, error) {
	if index >= len(arr) {
		return 0, nil
	}
	return rawFloat32(arr[index], name)
}

func rawBool(v interface{}, name string) (bool, error) {
	if b, ok := toBool(v); ok {
		return b, nil
	}
	return false, fmt.Errorf("%s: expected bool, got %T", name, v)
}

func rawVector3(v interface{}, name string) (Vector3, error) {
	arr, err := asRawArray(v, name)
	if err != nil {
		return Vector3{}, err
	}
	if len(arr) < 3 {
		return Vector3{}, fmt.Errorf("%s: expected array(3+), got %d", name, len(arr))
	}
	x, ok := toFloat32(arr[0])
	if !ok {
		return Vector3{}, fmt.Errorf("%s.x: expected float, got %T", name, arr[0])
	}
	y, ok := toFloat32(arr[1])
	if !ok {
		return Vector3{}, fmt.Errorf("%s.y: expected float, got %T", name, arr[1])
	}
	z, ok := toFloat32(arr[2])
	if !ok {
		return Vector3{}, fmt.Errorf("%s.z: expected float, got %T", name, arr[2])
	}
	return Vector3{X: x, Y: y, Z: z}, nil
}

func rawVector4(v interface{}, name string) (Vector4, error) {
	arr, err := asRawArray(v, name)
	if err != nil {
		return Vector4{}, err
	}
	if len(arr) < 4 {
		return Vector4{}, fmt.Errorf("%s: expected array(4+), got %d", name, len(arr))
	}
	x, ok := toFloat32(arr[0])
	if !ok {
		return Vector4{}, fmt.Errorf("%s.x: expected float, got %T", name, arr[0])
	}
	y, ok := toFloat32(arr[1])
	if !ok {
		return Vector4{}, fmt.Errorf("%s.y: expected float, got %T", name, arr[1])
	}
	z, ok := toFloat32(arr[2])
	if !ok {
		return Vector4{}, fmt.Errorf("%s.z: expected float, got %T", name, arr[2])
	}
	w, ok := toFloat32(arr[3])
	if !ok {
		return Vector4{}, fmt.Errorf("%s.w: expected float, got %T", name, arr[3])
	}
	return Vector4{X: x, Y: y, Z: z, W: w}, nil
}

func vector3ToRaw(v Vector3) []interface{} {
	return []interface{}{v.X, v.Y, v.Z}
}

func vector4ToRaw(v Vector4) []interface{} {
	return []interface{}{v.X, v.Y, v.Z, v.W}
}

func stringSliceToRaw(values []string) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, s := range values {
		out = append(out, s)
	}
	return out
}

func inferColliderTypeFromArray(arr []interface{}) int {
	if len(arr) < 1 {
		return -1
	}
	switch len(arr) {
	case 9:
		return ColliderTypeSphere
	case 10:
		return ColliderTypePlane
	default:
		if len(arr) >= 19 && (looksLikeMaidPropColliderStatusAt(arr, 13) || looksLikeMaidPropColliderStatusAt(arr, 16)) {
			return ColliderTypeMaidProp
		}
		if len(arr) >= 13 {
			return ColliderTypeCapsule
		}
		return -1
	}
}

func looksLikeMaidPropColliderStatusAt(arr []interface{}, fieldOffset int) bool {
	if len(arr) < fieldOffset+6 {
		return false
	}
	if _, err := rawStringArray(arr[fieldOffset], "infer."+strconv.Itoa(fieldOffset)); err != nil {
		return false
	}
	if _, err := rawVector3(arr[fieldOffset+1], "infer."+strconv.Itoa(fieldOffset+1)); err != nil {
		return false
	}
	if _, err := rawStringArray(arr[fieldOffset+2], "infer."+strconv.Itoa(fieldOffset+2)); err != nil {
		return false
	}
	if _, err := rawFloat32(arr[fieldOffset+3], "infer."+strconv.Itoa(fieldOffset+3)); err != nil {
		return false
	}
	if _, err := rawStringArray(arr[fieldOffset+4], "infer."+strconv.Itoa(fieldOffset+4)); err != nil {
		return false
	}
	if _, err := rawFloat32(arr[fieldOffset+5], "infer."+strconv.Itoa(fieldOffset+5)); err != nil {
		return false
	}
	return true
}

func inferColliderTypeFromJSON(raw json.RawMessage) int {
	var tmp map[string]interface{}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return -1
	}
	if tmp == nil {
		return -1
	}
	if _, ok := tmp["centerMpnList"]; ok {
		return ColliderTypeMaidProp
	}
	if _, ok := tmp["radius"]; ok {
		return ColliderTypeSphere
	}
	if _, ok := tmp["startRadius"]; ok {
		if _, ok2 := tmp["centerMpnNameList"]; ok2 {
			return ColliderTypeMaidProp
		}
		return ColliderTypeCapsule
	}
	if _, ok := tmp["direction"]; ok {
		return ColliderTypePlane
	}
	return -1
}

func decodeColliderObjectAsType(raw json.RawMessage, typ int) (ColliderStatusUnion, error) {
	switch typ {
	case ColliderTypePlane:
		var status ColliderPlane
		if err := json.Unmarshal(raw, &status); err != nil {
			return nil, err
		}
		if status.Direction == 0 && status.IsDirectionInverse == false && status.Bound == 0 && status.LocalPosition == (Vector3{}) {
			return nil, fmt.Errorf("invalid Plane collider JSON")
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

func toJSONNumberString(v interface{}) (string, error) {
	switch x := v.(type) {
	case json.Number:
		return x.String(), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case int:
		return strconv.Itoa(x), nil
	case uint:
		return strconv.FormatUint(uint64(x), 10), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), nil
	default:
		return "", fmt.Errorf("not a number: %T", v)
	}
}
