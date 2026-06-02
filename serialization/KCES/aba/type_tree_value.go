package aba

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// TypeTreeValue 是根据内嵌 TypeTree 解码出的 Unity 序列化值 / TypeTreeValue is a decoded Unity serialized value built from an embedded TypeTree
// 它只暴露 KCES 提取器所需的小型导航和转换 API / It intentionally exposes only the small navigation and conversion API needed by KCES extractors
type TypeTreeValue struct {
	TypeName string           // Unity 类型名 / Unity type name
	Name     string           // 字段名或节点名 / Field or node name
	Value    interface{}      // 原始标量值或字节数组 / Raw scalar value or byte array
	Children []*TypeTreeValue // 子节点列表 / Child node list
}

// ReadAssetValue 使用资源的 TypeTree 解码对象 / ReadAssetValue decodes an asset object with the asset's TypeTree
func (af *AssetsFile) ReadAssetValue(info *AssetInfo) (*TypeTreeValue, error) {
	tt, err := af.typeTreeForAsset(info)
	if err != nil {
		return nil, err
	}
	if len(tt.Nodes) == 0 {
		return nil, fmt.Errorf("type tree for class %d has no nodes", info.TypeId)
	}
	data, err := af.GetAssetData(info)
	if err != nil {
		return nil, err
	}

	order := af.byteOrder()
	r := &bufReader{data: data, pos: 0, order: order}
	root, next, err := readTypeTreeValue(tt, r, 0)
	if err != nil {
		return nil, err
	}
	if next != len(tt.Nodes) {
		return root, nil
	}
	return root, nil
}

func (af *AssetsFile) typeTreeForAsset(info *AssetInfo) (*TypeTreeType, error) {
	if !af.Metadata.TypeTreeEnabled {
		return nil, fmt.Errorf("assets file does not contain type tree metadata")
	}
	if af.Header.Version >= 16 {
		idx := int(info.TypeIdOrIndex)
		if idx < 0 || idx >= len(af.Metadata.TypeTreeTypes) {
			return nil, fmt.Errorf("type tree index %d out of range", idx)
		}
		return &af.Metadata.TypeTreeTypes[idx], nil
	}
	for i := range af.Metadata.TypeTreeTypes {
		if af.Metadata.TypeTreeTypes[i].TypeId == info.TypeId {
			return &af.Metadata.TypeTreeTypes[i], nil
		}
	}
	return nil, fmt.Errorf("type tree for class %d not found", info.TypeId)
}

func (af *AssetsFile) byteOrder() binary.ByteOrder {
	if af.Header.Endianness {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func readTypeTreeValue(tt *TypeTreeType, r *bufReader, idx int) (*TypeTreeValue, int, error) {
	if idx < 0 || idx >= len(tt.Nodes) {
		return nil, idx, io.ErrUnexpectedEOF
	}
	node := &tt.Nodes[idx]
	v := &TypeTreeValue{
		TypeName: tt.GetTypeTreeString(node, true),
		Name:     tt.GetTypeTreeString(node, false),
	}

	if isSpecialPrimitiveType(v.TypeName) {
		if err := readPrimitiveValue(r, v); err != nil {
			return nil, idx + 1, fmt.Errorf("read %s %s: %w", v.TypeName, v.Name, err)
		}
		if node.MetaFlags&0x4000 != 0 {
			r.align4()
		}
		return v, skipSubtree(tt, idx), nil
	}

	next := idx + 1
	if next < len(tt.Nodes) && tt.Nodes[next].Level > node.Level {
		if isArrayNode(node, v.TypeName) {
			arr, n, err := readArrayValue(tt, r, idx, v)
			if err != nil {
				return nil, n, err
			}
			next = n
			v.Children = arr.Children
			v.Value = arr.Value
		} else {
			for next < len(tt.Nodes) && tt.Nodes[next].Level > node.Level {
				child, n, err := readTypeTreeValue(tt, r, next)
				if err != nil {
					return nil, n, err
				}
				v.Children = append(v.Children, child)
				next = n
			}
		}
	} else if err := readPrimitiveValue(r, v); err != nil {
		return nil, next, fmt.Errorf("read %s %s: %w", v.TypeName, v.Name, err)
	}

	if node.MetaFlags&0x4000 != 0 {
		r.align4()
	}
	return v, next, nil
}

func readArrayValue(tt *TypeTreeType, r *bufReader, idx int, v *TypeTreeValue) (*TypeTreeValue, int, error) {
	node := &tt.Nodes[idx]
	next := idx + 1
	if next >= len(tt.Nodes) {
		return v, next, fmt.Errorf("array %s has no Array node", v.Name)
	}

	// Unity arrays are encoded as:
	//   vector/list/TypelessData field
	//     Array
	//       int size
	//       T data
	// Some versions mark the field itself with TypeFlags; this reader accepts
	// both shapes and falls back to the first descendant named "size"/"data".
	arrayLevel := tt.Nodes[next].Level
	if tt.GetTypeTreeString(&tt.Nodes[next], false) == "Array" {
		next++
	}

	if next >= len(tt.Nodes) {
		return v, next, fmt.Errorf("array %s missing size node", v.Name)
	}
	sizeNodeIdx := next
	if tt.GetTypeTreeString(&tt.Nodes[sizeNodeIdx], false) != "size" {
		for sizeNodeIdx < len(tt.Nodes) && tt.Nodes[sizeNodeIdx].Level > node.Level && tt.GetTypeTreeString(&tt.Nodes[sizeNodeIdx], false) != "size" {
			sizeNodeIdx++
		}
		if sizeNodeIdx >= len(tt.Nodes) || tt.Nodes[sizeNodeIdx].Level <= node.Level {
			return v, next, fmt.Errorf("array %s missing size node", v.Name)
		}
	}

	sizeRaw, err := r.readInt32()
	if err != nil {
		return nil, sizeNodeIdx + 1, err
	}
	if sizeRaw < 0 {
		return nil, sizeNodeIdx + 1, fmt.Errorf("negative array size %d for %s", sizeRaw, v.Name)
	}
	size := int(sizeRaw)

	dataNodeIdx := sizeNodeIdx + 1
	for dataNodeIdx < len(tt.Nodes) && tt.Nodes[dataNodeIdx].Level > node.Level && tt.GetTypeTreeString(&tt.Nodes[dataNodeIdx], false) != "data" {
		dataNodeIdx++
	}
	if dataNodeIdx >= len(tt.Nodes) || tt.Nodes[dataNodeIdx].Level <= node.Level {
		// Empty or malformed arrays can still be skipped structurally.
		return v, skipSubtree(tt, idx), nil
	}

	dataNode := &tt.Nodes[dataNodeIdx]
	elemNext := skipSubtree(tt, dataNodeIdx)
	elemType := tt.GetTypeTreeString(dataNode, true)

	if size == 0 {
		v.Children = []*TypeTreeValue{}
		return v, skipSubtree(tt, idx), nil
	}

	if isByteElement(elemType) && elemNext == dataNodeIdx+1 {
		if r.pos+size > len(r.data) {
			return nil, elemNext, io.ErrUnexpectedEOF
		}
		buf := make([]byte, size)
		copy(buf, r.data[r.pos:r.pos+size])
		r.pos += size
		v.Value = buf
		return v, skipSubtree(tt, idx), nil
	}

	v.Children = make([]*TypeTreeValue, 0, size)
	for i := 0; i < size; i++ {
		child, _, err := readTypeTreeValue(tt, r, dataNodeIdx)
		if err != nil {
			return nil, dataNodeIdx, fmt.Errorf("read %s[%d]: %w", v.Name, i, err)
		}
		child.Name = fmt.Sprintf("data[%d]", i)
		v.Children = append(v.Children, child)
	}

	// The actual metadata subtree is consumed once structurally, regardless of
	// element count. arrayLevel is intentionally kept to document the accepted
	// Unity shape and avoid accidental simplification.
	_ = arrayLevel
	return v, skipSubtree(tt, idx), nil
}

func readPrimitiveValue(r *bufReader, v *TypeTreeValue) error {
	switch v.TypeName {
	case "string":
		s, err := r.readAlignedString()
		v.Value = s
		return err
	case "TypelessData":
		size, err := r.readInt32()
		if err != nil {
			return err
		}
		if size < 0 || r.pos+int(size) > len(r.data) {
			return fmt.Errorf("invalid TypelessData size %d", size)
		}
		buf := make([]byte, size)
		copy(buf, r.data[r.pos:r.pos+int(size)])
		r.pos += int(size)
		v.Value = buf
		return nil
	case "bool":
		b, err := r.readByte()
		v.Value = b != 0
		return err
	case "char", "SInt8":
		b, err := r.readByte()
		v.Value = int64(int8(b))
		return err
	case "UInt8", "unsigned char":
		b, err := r.readByte()
		v.Value = int64(b)
		return err
	case "short", "SInt16":
		u, err := r.readUint16()
		v.Value = int64(int16(u))
		return err
	case "unsigned short", "UInt16":
		u, err := r.readUint16()
		v.Value = int64(u)
		return err
	case "int", "SInt32":
		i, err := r.readInt32()
		v.Value = int64(i)
		return err
	case "unsigned int", "UInt32", "Type*":
		u, err := r.readUint32()
		v.Value = int64(u)
		return err
	case "long long", "SInt64":
		i, err := r.readInt64()
		v.Value = i
		return err
	case "unsigned long long", "UInt64":
		u, err := r.readUint64()
		v.Value = int64(u)
		return err
	case "float":
		f, err := r.readFloat32()
		v.Value = f
		return err
	case "double":
		u, err := r.readUint64()
		v.Value = math.Float64frombits(u)
		return err
	default:
		if v.TypeName == "" {
			return nil
		}
		return fmt.Errorf("unsupported primitive type %q", v.TypeName)
	}
}

func isSpecialPrimitiveType(typeName string) bool {
	switch typeName {
	case "string", "TypelessData":
		return true
	default:
		return false
	}
}

func isArrayNode(node *TypeTreeNode, typeName string) bool {
	if node.TypeFlags&0x01 != 0 {
		return true
	}
	switch typeName {
	case "vector", "list", "Array", "TypelessData", "staticvector":
		return true
	default:
		return false
	}
}

func isByteElement(typeName string) bool {
	switch typeName {
	case "UInt8", "unsigned char", "SInt8", "char":
		return true
	default:
		return false
	}
}

func skipSubtree(tt *TypeTreeType, idx int) int {
	level := tt.Nodes[idx].Level
	idx++
	for idx < len(tt.Nodes) && tt.Nodes[idx].Level > level {
		idx++
	}
	return idx
}

// Field returns the first direct child with the requested field name.
func (v *TypeTreeValue) Field(name string) *TypeTreeValue {
	if v == nil {
		return nil
	}
	for _, child := range v.Children {
		if child != nil && child.Name == name {
			return child
		}
	}
	return nil
}

// FieldPath walks through direct children by field names.
func (v *TypeTreeValue) FieldPath(names ...string) *TypeTreeValue {
	cur := v
	for _, name := range names {
		cur = cur.Field(name)
		if cur == nil {
			return nil
		}
	}
	return cur
}

func (v *TypeTreeValue) String() (string, bool) {
	if v == nil {
		return "", false
	}
	s, ok := v.Value.(string)
	return s, ok
}

func (v *TypeTreeValue) Int64() (int64, bool) {
	if v == nil {
		return 0, false
	}
	switch x := v.Value.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		return int64(x), true
	default:
		return 0, false
	}
}

func (v *TypeTreeValue) Float32() (float32, bool) {
	if v == nil {
		return 0, false
	}
	switch x := v.Value.(type) {
	case float32:
		return x, true
	case float64:
		return float32(x), true
	default:
		return 0, false
	}
}

func (v *TypeTreeValue) Bytes() ([]byte, bool) {
	if v == nil {
		return nil, false
	}
	if b, ok := v.Value.([]byte); ok {
		return b, true
	}
	if len(v.Children) == 0 {
		return nil, false
	}
	out := make([]byte, len(v.Children))
	for i, child := range v.Children {
		n, ok := child.Int64()
		if !ok || n < 0 || n > 255 {
			return nil, false
		}
		out[i] = byte(n)
	}
	return out, true
}
