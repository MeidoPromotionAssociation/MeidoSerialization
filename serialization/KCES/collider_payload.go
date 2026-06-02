package KCES

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ColliderPackage 表示通用碰撞体包 / ColliderPackage represents a generic collider package
type ColliderPackage struct {
	Version   int             `json:"version"`          // 版本号 / Version value
	Colliders []ColliderRef   `json:"colliders"`        // 碰撞体引用列表 / Collider reference list
	States    []ColliderState `json:"states,omitempty"` // 可选启用状态列表 / Optional enabled-state list
}

// ColliderRef 表示带类型枚举的碰撞体引用 / ColliderRef represents a collider reference with its type enum
type ColliderRef struct {
	Type     int            `json:"type"`     // 碰撞体类型枚举 / Collider type enum
	Collider ColliderObject `json:"collider"` // 碰撞体对象数据 / Collider object data
}

// ColliderObject 表示游戏碰撞体状态的公共基类字段 / ColliderObject represents common base fields of game collider status objects
// Tail 保留 NativeSphere/Capsule/Plane/MaidProp 等派生类型在 Key(7) 之后的字段 / Tail preserves derived NativeSphere, Capsule, Plane, and MaidProp fields after Key(7)
type ColliderObject struct {
	Version       int           `json:"version"`        // 版本号 / Version value
	ParentName    string        `json:"parentName"`     // 父对象名称 / Parent object name
	SelfName      string        `json:"selfName"`       // 自身对象名称 / Own object name
	LocalPosition Vector3       `json:"localPosition"`  // 本地位置 / Local position
	LocalRotation Vector4       `json:"localRotation"`  // 本地旋转四元数 / Local rotation quaternion
	LocalScale    Vector3       `json:"localScale"`     // 本地缩放 / Local scale
	Center        Vector3       `json:"center"`         // 碰撞体中心 / Collider center
	Tail          []interface{} `json:"tail,omitempty"` // 派生类型尾部字段 / Derived-type tail fields
}

// ColliderState 表示碰撞体启用状态 / ColliderState represents a collider enabled state
type ColliderState struct {
	Version int  `json:"version"` // 版本号 / Version value
	Index   int  `json:"index"`   // 对应碰撞体索引 / Referenced collider index
	Enabled bool `json:"enabled"` // 是否启用 / Whether the collider is enabled
}

// LimbColliderPackage 对应 LimbColliderMgr 保存的 limb collider 包 / LimbColliderPackage maps the package saved by LimbColliderMgr
type LimbColliderPackage struct {
	Version int                `json:"version"` // 版本号 / Version value
	Items   []LimbColliderItem `json:"items"`   // limb 碰撞体条目列表 / Limb collider item list
}

// LimbColliderItem 表示一个 limb 类型和碰撞体状态 / LimbColliderItem represents one limb type and collider status
type LimbColliderItem struct {
	Version  int            `json:"version"`  // 版本号 / Version value
	Target   int            `json:"target"`   // limbType 枚举值 / limbType enum value
	Collider ColliderObject `json:"collider"` // 碰撞体状态 / Collider status
}

// IKColliderPackage 对应 IKColliderSaveLoader 保存的 IK collider 包 / IKColliderPackage maps the package saved by IKColliderSaveLoader
type IKColliderPackage struct {
	Version int               `json:"version"` // 版本号 / Version value
	Groups  []IKColliderGroup `json:"groups"`  // IK 效果器分组列表 / IK effector group list
}

// IKColliderGroup 表示一个 IK 效果器的碰撞体列表 / IKColliderGroup represents colliders for one IK effector
type IKColliderGroup struct {
	Version   int           `json:"version"`   // 版本号 / Version value
	Target    int           `json:"target"`    // effectorType 枚举值 / effectorType enum value
	Colliders []ColliderRef `json:"colliders"` // 该效果器关联的碰撞体引用列表 / Collider references associated with this effector
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
		collider, err := decodeColliderObjectRaw(itemArr[2], fmt.Sprintf("LimbColliderPackage.items[%d].collider", i))
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
			item.Collider.toRaw(),
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
		collider, err := decodeColliderObjectRaw(refArr[1], fmt.Sprintf("%s[%d].collider", name, i))
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
		out = append(out, []interface{}{int64(ref.Type), ref.Collider.toRaw()})
	}
	return out
}

func decodeColliderObjectRaw(raw interface{}, name string) (ColliderObject, error) {
	arr, err := asRawArray(raw, name)
	if err != nil {
		return ColliderObject{}, err
	}
	if len(arr) < 7 {
		return ColliderObject{}, fmt.Errorf("%s: expected array(7+), got %d", name, len(arr))
	}
	version, err := rawInt(arr[0], name+".version")
	if err != nil {
		return ColliderObject{}, err
	}
	parentName, err := rawString(arr[1], name+".parentName")
	if err != nil {
		return ColliderObject{}, err
	}
	selfName, err := rawString(arr[2], name+".selfName")
	if err != nil {
		return ColliderObject{}, err
	}
	localPosition, err := rawVector3(arr[3], name+".localPosition")
	if err != nil {
		return ColliderObject{}, err
	}
	localRotation, err := rawVector4(arr[4], name+".localRotation")
	if err != nil {
		return ColliderObject{}, err
	}
	localScale, err := rawVector3(arr[5], name+".localScale")
	if err != nil {
		return ColliderObject{}, err
	}
	center, err := rawVector3(arr[6], name+".center")
	if err != nil {
		return ColliderObject{}, err
	}

	tail := make([]interface{}, 0, len(arr)-7)
	for _, v := range arr[7:] {
		tail = append(tail, normalizePayloadJSONValue(v))
	}

	return ColliderObject{
		Version:       version,
		ParentName:    parentName,
		SelfName:      selfName,
		LocalPosition: localPosition,
		LocalRotation: localRotation,
		LocalScale:    localScale,
		Center:        center,
		Tail:          tail,
	}, nil
}

func (c *ColliderObject) toRaw() []interface{} {
	if c == nil {
		return nil
	}
	out := []interface{}{
		int64(c.Version),
		c.ParentName,
		c.SelfName,
		vector3ToRaw(c.LocalPosition),
		vector4ToRaw(c.LocalRotation),
		vector3ToRaw(c.LocalScale),
		vector3ToRaw(c.Center),
	}
	for _, v := range c.Tail {
		out = append(out, payloadJSONValueToRaw(v))
	}
	return out
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

func rawString(v interface{}, name string) (string, error) {
	if s, ok := toStringVal(v); ok {
		return s, nil
	}
	return "", fmt.Errorf("%s: expected string, got %T", name, v)
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

func normalizePayloadJSONValue(v interface{}) interface{} {
	switch x := v.(type) {
	case json.Number:
		return x
	case []interface{}:
		out := make([]interface{}, len(x))
		for i := range x {
			out[i] = normalizePayloadJSONValue(x[i])
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, v := range x {
			out[k] = normalizePayloadJSONValue(v)
		}
		return out
	case uint64:
		return json.Number(strconv.FormatUint(x, 10))
	case int64:
		return json.Number(strconv.FormatInt(x, 10))
	case uint:
		return json.Number(strconv.FormatUint(uint64(x), 10))
	case int:
		return json.Number(strconv.FormatInt(int64(x), 10))
	case float64:
		return jsonNumberForFloat(x, 64)
	case float32:
		return jsonNumberForFloat(float64(x), 32)
	default:
		return v
	}
}

func payloadJSONValueToRaw(v interface{}) interface{} {
	switch x := v.(type) {
	case json.Number:
		s := x.String()
		if strings.ContainsAny(s, ".eE") {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
			return s
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			return u
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return s
	case []interface{}:
		out := make([]interface{}, len(x))
		for i := range x {
			out[i] = payloadJSONValueToRaw(x[i])
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, v := range x {
			out[k] = payloadJSONValueToRaw(v)
		}
		return out
	default:
		return x
	}
}
