package aba

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// TypeTreeValue 是根据内嵌 TypeTree 解码出的 Unity 序列化值。
// 它只暴露 KCES 提取器所需的小型导航和转换 API。
//
// TypeTreeValue is a decoded Unity serialized value built from an embedded TypeTree.
// It intentionally exposes only the small navigation and conversion API needed by KCES extractors.
type TypeTreeValue struct {
	TypeName string           // Unity 类型名 / Unity type name
	Name     string           // 字段名或节点名 / Field or node name
	Value    interface{}      // 原始标量值或字节数组 / Raw scalar value or byte array
	Children []*TypeTreeValue // 子节点列表 / Child node list
}

// ReadAssetValue 使用资源的 TypeTree 解码对象 / ReadAssetValue decodes an asset object with the asset's TypeTree
func (af *AssetsFile) ReadAssetValue(info *AssetInfo) (*TypeTreeValue, error) {
	if af == nil {
		return nil, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, fmt.Errorf("nil asset info")
	}
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
	r := binaryio.NewEndianReader(data, order)
	root, next, err := readTypeTreeValue(tt, r, 0)
	if err != nil {
		return nil, err
	}
	if next != int64(len(tt.Nodes)) {
		return nil, fmt.Errorf("type tree for class %d stopped at node %d/%d", info.TypeId, next, len(tt.Nodes))
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("type tree for class %d left %d unread object bytes", info.TypeId, r.Remaining())
	}
	return root, nil
}

func (af *AssetsFile) typeTreeForAsset(info *AssetInfo) (*TypeTreeType, error) {
	if af == nil {
		return nil, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, fmt.Errorf("nil asset info")
	}
	if !af.Metadata.TypeTreeEnabled {
		return nil, fmt.Errorf("assets file does not contain type tree metadata")
	}
	if af.Header.Version >= 16 {
		idx := int64(info.TypeIdOrIndex)
		if idx < 0 || idx >= int64(len(af.Metadata.TypeTreeTypes)) {
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

func readTypeTreeValue(tt *TypeTreeType, r *binaryio.EndianReader, idx int64) (*TypeTreeValue, int64, error) {
	if tt == nil {
		return nil, idx, fmt.Errorf("nil type tree")
	}
	if r == nil {
		return nil, idx, fmt.Errorf("nil type tree reader")
	}
	if idx < 0 || idx >= int64(len(tt.Nodes)) {
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
			if err := alignReader4(r); err != nil {
				return nil, skipSubtree(tt, idx), fmt.Errorf("align %s %s: %w", v.TypeName, v.Name, err)
			}
		}
		return v, skipSubtree(tt, idx), nil
	}

	next := idx + 1
	if next < int64(len(tt.Nodes)) && tt.Nodes[next].Level > node.Level {
		if isArrayNode(node, v.TypeName) {
			arr, n, err := readArrayValue(tt, r, idx, v)
			if err != nil {
				return nil, n, err
			}
			next = n
			v.Children = arr.Children
			v.Value = arr.Value
		} else {
			for next < int64(len(tt.Nodes)) && tt.Nodes[next].Level > node.Level {
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
		if err := alignReader4(r); err != nil {
			return nil, next, fmt.Errorf("align %s %s: %w", v.TypeName, v.Name, err)
		}
	}
	return v, next, nil
}

func readArrayValue(tt *TypeTreeType, r *binaryio.EndianReader, idx int64, v *TypeTreeValue) (*TypeTreeValue, int64, error) {
	node := &tt.Nodes[idx]
	next := idx + 1
	if next >= int64(len(tt.Nodes)) {
		return v, next, fmt.Errorf("array %s has no Array node", v.Name)
	}

	// Unity arrays are encoded as:
	//   vector/list/TypelessData field
	//     Array
	//       int size
	//       T data
	// Some versions mark the field itself with TypeFlags; this reader accepts
	// both shapes and falls back to the first descendant named "size"/"data".
	arrayNodeIdx := idx
	if tt.GetTypeTreeString(&tt.Nodes[next], false) == "Array" {
		arrayNodeIdx = next
		next++
	}
	alignArray := func() error {
		// In Unity type trees the 4-byte alignment flag normally belongs to
		// the nested `Array` node, not to its vector/list field.  The bytes for
		// the whole array (including all elements) must be consumed before the
		// alignment is applied.
		if tt.Nodes[arrayNodeIdx].MetaFlags&0x4000 != 0 {
			if err := alignReader4(r); err != nil {
				return fmt.Errorf("align array %s: %w", v.Name, err)
			}
		}
		return nil
	}

	if next >= int64(len(tt.Nodes)) {
		return v, next, fmt.Errorf("array %s missing size node", v.Name)
	}
	sizeNodeIdx := next
	if tt.GetTypeTreeString(&tt.Nodes[sizeNodeIdx], false) != "size" {
		for sizeNodeIdx < int64(len(tt.Nodes)) && tt.Nodes[sizeNodeIdx].Level > node.Level && tt.GetTypeTreeString(&tt.Nodes[sizeNodeIdx], false) != "size" {
			sizeNodeIdx++
		}
		if sizeNodeIdx >= int64(len(tt.Nodes)) || tt.Nodes[sizeNodeIdx].Level <= node.Level {
			return v, next, fmt.Errorf("array %s missing size node", v.Name)
		}
	}

	sizeRaw, err := r.ReadInt32()
	if err != nil {
		return nil, sizeNodeIdx + 1, err
	}
	if sizeRaw < 0 {
		return nil, sizeNodeIdx + 1, fmt.Errorf("negative array size %d for %s", sizeRaw, v.Name)
	}
	size := int64(sizeRaw)

	dataNodeIdx := sizeNodeIdx + 1
	for dataNodeIdx < int64(len(tt.Nodes)) && tt.Nodes[dataNodeIdx].Level > node.Level && tt.GetTypeTreeString(&tt.Nodes[dataNodeIdx], false) != "data" {
		dataNodeIdx++
	}
	if dataNodeIdx >= int64(len(tt.Nodes)) || tt.Nodes[dataNodeIdx].Level <= node.Level {
		return nil, skipSubtree(tt, idx), fmt.Errorf("array %s missing data node", v.Name)
	}

	dataNode := &tt.Nodes[dataNodeIdx]
	elemNext := skipSubtree(tt, dataNodeIdx)
	elemType := tt.GetTypeTreeString(dataNode, true)
	if minBytes := minimumTypeTreeValueBytes(tt, dataNodeIdx); minBytes > 0 && int64(size) > int64(r.Remaining())/int64(minBytes) {
		return nil, elemNext, fmt.Errorf("array %s size %d requires at least %d bytes but only %d remain", v.Name, size, int64(size)*int64(minBytes), r.Remaining())
	}

	if size == 0 {
		v.Children = []*TypeTreeValue{}
		if err := alignArray(); err != nil {
			return nil, skipSubtree(tt, idx), err
		}
		return v, skipSubtree(tt, idx), nil
	}

	if isByteElement(elemType) && elemNext == dataNodeIdx+1 {
		buf, err := r.ReadBytes(int(size))
		if err != nil {
			return nil, elemNext, err
		}
		v.Value = buf
		if err := alignArray(); err != nil {
			return nil, skipSubtree(tt, idx), err
		}
		return v, skipSubtree(tt, idx), nil
	}

	v.Children = makeABACountedSliceForAppend[*TypeTreeValue](size)
	for i := int64(0); i < size; i++ {
		before := r.Pos()
		child, _, err := readTypeTreeValue(tt, r, dataNodeIdx)
		if err != nil {
			return nil, dataNodeIdx, fmt.Errorf("read %s[%d]: %w", v.Name, i, err)
		}
		if r.Pos() == before {
			return nil, dataNodeIdx, fmt.Errorf("array %s element type %q consumes no bytes", v.Name, elemType)
		}
		child.Name = fmt.Sprintf("data[%d]", i)
		v.Children = append(v.Children, child)
	}

	// The actual metadata subtree is consumed once structurally, regardless of
	// element count.
	if err := alignArray(); err != nil {
		return nil, skipSubtree(tt, idx), err
	}
	return v, skipSubtree(tt, idx), nil
}

func readPrimitiveValue(r *binaryio.EndianReader, v *TypeTreeValue) error {
	switch v.TypeName {
	case "string":
		s, err := r.ReadAlignedString()
		v.Value = s
		return err
	case "TypelessData":
		size, err := r.ReadInt32()
		if err != nil {
			return err
		}
		if size < 0 || int64(size) > int64(r.Remaining()) {
			return fmt.Errorf("invalid TypelessData size %d", size)
		}
		buf, err := r.ReadBytes(int(size))
		if err != nil {
			return err
		}
		v.Value = buf
		return nil
	case "bool":
		b, err := r.ReadByte()
		v.Value = b != 0
		return err
	case "char", "SInt8":
		b, err := r.ReadByte()
		v.Value = int64(int8(b))
		return err
	case "UInt8", "unsigned char":
		b, err := r.ReadByte()
		v.Value = int64(b)
		return err
	case "short", "SInt16":
		u, err := r.ReadUInt16()
		v.Value = int64(int16(u))
		return err
	case "unsigned short", "UInt16":
		u, err := r.ReadUInt16()
		v.Value = int64(u)
		return err
	case "int", "SInt32":
		i, err := r.ReadInt32()
		v.Value = int64(i)
		return err
	case "unsigned int", "UInt32", "Type*":
		u, err := r.ReadUInt32()
		v.Value = int64(u)
		return err
	case "long long", "SInt64":
		i, err := r.ReadInt64()
		v.Value = i
		return err
	case "unsigned long long", "UInt64":
		u, err := r.ReadUInt64()
		v.Value = u
		return err
	case "float":
		f, err := r.ReadFloat32()
		v.Value = f
		return err
	case "double":
		u, err := r.ReadUInt64()
		v.Value = math.Float64frombits(u)
		return err
	default:
		if v.TypeName == "" {
			return nil
		}
		return fmt.Errorf("unsupported primitive type %q", v.TypeName)
	}
}

func alignReader4(r *binaryio.EndianReader) error {
	if r == nil {
		return fmt.Errorf("nil reader")
	}
	pos := r.Pos()
	if pos < 0 || pos > r.Len() {
		return io.ErrUnexpectedEOF
	}
	padding := (-pos) & 3
	if padding > r.Remaining() {
		return io.ErrUnexpectedEOF
	}
	r.Skip(padding)
	return nil
}

func minimumTypeTreeValueBytes(tt *TypeTreeType, idx int64) int64 {
	if tt == nil || idx < 0 || idx >= int64(len(tt.Nodes)) {
		return 0
	}
	node := &tt.Nodes[idx]
	typeName := tt.GetTypeTreeString(node, true)
	switch typeName {
	case "string", "TypelessData":
		return 4
	case "bool", "char", "SInt8", "UInt8", "unsigned char":
		return 1
	case "short", "SInt16", "unsigned short", "UInt16":
		return 2
	case "int", "SInt32", "unsigned int", "UInt32", "Type*", "float":
		return 4
	case "long long", "SInt64", "unsigned long long", "UInt64", "double":
		return 8
	}

	next := idx + 1
	if next >= int64(len(tt.Nodes)) || tt.Nodes[next].Level <= node.Level {
		return 0
	}
	if isArrayNode(node, typeName) {
		return 4
	}
	total := int64(0)
	for next < int64(len(tt.Nodes)) && tt.Nodes[next].Level > node.Level {
		total += minimumTypeTreeValueBytes(tt, next)
		next = skipSubtree(tt, next)
	}
	return total
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

func skipSubtree(tt *TypeTreeType, idx int64) int64 {
	level := tt.Nodes[idx].Level
	idx++
	for idx < int64(len(tt.Nodes)) && tt.Nodes[idx].Level > level {
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
	case uint32:
		return int64(x), true
	case uint64:
		if x > math.MaxInt64 {
			return 0, false
		}
		return int64(x), true
	default:
		return 0, false
	}
}

// UInt64 returns a non-negative integer without losing values above MaxInt64.
func (v *TypeTreeValue) UInt64() (uint64, bool) {
	if v == nil {
		return 0, false
	}
	switch x := v.Value.(type) {
	case uint64:
		return x, true
	case uint32:
		return uint64(x), true
	case int64:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
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
