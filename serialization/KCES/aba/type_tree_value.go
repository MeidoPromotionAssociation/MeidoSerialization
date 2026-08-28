package aba

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio"
)

// TypeTreeValue 是根据内嵌 TypeTree 解码且仅暴露 KCES 所需导航 API 的 Unity 序列化值 / TypeTreeValue is a Unity serialized value decoded from an embedded TypeTree with only the navigation API needed by KCES
type TypeTreeValue struct {
	TypeName   string           // Unity 类型名 / Unity type name
	Name       string           // 字段名或节点名 / Field or node name
	Value      interface{}      // 原始标量值或字节数组 / Raw scalar value or byte array
	Children   []*TypeTreeValue // 子节点列表 / Child node list
	NodeIndex  int64            // 对应 TypeTree 节点索引 / Corresponding TypeTree node index
	ByteOffset int64            // 值在对象数据中的起始偏移 / Start offset of the value in object data
	ByteSize   int64            // 值及其尾部对齐占用的字节数 / Byte count occupied by the value and trailing alignment
}

// ReadAssetValue 使用资源的 TypeTree 解码对象，并确认类型树和对象数据均被完整消费
// ReadAssetValue decodes an asset object with its TypeTree and verifies that both the tree and object data are fully consumed
func (af *AssetsFile) ReadAssetValue(info *AssetInfo) (*TypeTreeValue, error) {
	root, consumed, objectSize, err := af.readAssetValuePrefix(info)
	if err != nil {
		return nil, err
	}
	if consumed != objectSize {
		return nil, fmt.Errorf("type tree for class %d left %d unread object bytes", info.TypeId, objectSize-consumed)
	}
	return root, nil
}

// readAssetValuePrefix 使用 TypeTree 解码对象字段并返回已消费字节数，供 AudioClip 等在字段树后附加内联载荷的布局使用
// readAssetValuePrefix decodes object fields with a TypeTree and returns the consumed byte count for layouts such as AudioClip that append inline payload bytes after the field tree
func (af *AssetsFile) readAssetValuePrefix(info *AssetInfo) (*TypeTreeValue, int64, int64, error) {
	if af == nil {
		return nil, 0, 0, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, 0, 0, fmt.Errorf("nil asset info")
	}
	tt, err := af.typeTreeForAsset(info)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(tt.Nodes) == 0 {
		return nil, 0, 0, fmt.Errorf("type tree for class %d has no nodes", info.TypeId)
	}
	data, err := af.GetAssetData(info)
	if err != nil {
		return nil, 0, 0, err
	}

	order := af.byteOrder()
	r := binaryio.NewEndianReader(data, order)
	root, next, err := readTypeTreeValue(tt, r, 0)
	if err != nil {
		return nil, 0, 0, err
	}
	if next != int64(len(tt.Nodes)) {
		return nil, 0, 0, fmt.Errorf("type tree for class %d stopped at node %d/%d", info.TypeId, next, len(tt.Nodes))
	}
	return root, r.Pos(), int64(len(data)), nil
}

// typeTreeForAsset 按 SerializedFile 版本为对象选择对应的 TypeTree
// v16 及以后使用 AssetInfo.TypeIdOrIndex 作为类型树索引，更早版本按 class ID 查找
// typeTreeForAsset selects the TypeTree for an asset according to the SerializedFile version
// Versions 16 and later use AssetInfo.TypeIdOrIndex as the type-tree index, while earlier versions search by class ID
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

// AssetTypeTree 返回指定对象使用的 TypeTree 深拷贝，使调用方可以安全地保存或修改独立对象布局
// AssetTypeTree returns a deep copy of the TypeTree used by an object so callers can safely retain or modify a standalone layout
func (af *AssetsFile) AssetTypeTree(info *AssetInfo) (TypeTreeType, error) {
	tree, err := af.typeTreeForAsset(info)
	if err != nil {
		return TypeTreeType{}, err
	}
	if len(tree.Nodes) == 0 {
		return TypeTreeType{}, fmt.Errorf("type tree for class %d has no nodes", info.TypeId)
	}
	return cloneTypeTreeType(tree), nil
}

// AssetHasTypeTree 判断指定对象是否拥有可用于完整编解码的 TypeTree
// AssetHasTypeTree reports whether an object has a TypeTree capable of complete decoding and encoding
func (af *AssetsFile) AssetHasTypeTree(info *AssetInfo) bool {
	tree, err := af.typeTreeForAsset(info)
	return err == nil && len(tree.Nodes) != 0
}

// byteOrder 返回当前 AssetsFile 头部声明的对象字节序
// byteOrder returns the object byte order declared by the AssetsFile header
func (af *AssetsFile) byteOrder() binary.ByteOrder {
	if af.Header.Endianness {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// readTypeTreeValue 按 TypeTree 的先序节点递归读取一个值，并返回下一个未消费节点索引
// readTypeTreeValue recursively reads one value in TypeTree preorder and returns the next unconsumed node index
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
	start := r.Pos()
	v := &TypeTreeValue{
		TypeName:   tt.GetTypeTreeString(node, true),
		Name:       tt.GetTypeTreeString(node, false),
		NodeIndex:  idx,
		ByteOffset: start,
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
		v.ByteSize = r.Pos() - start
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
	v.ByteSize = r.Pos() - start
	return v, next, nil
}

// readArrayValue 读取 Unity 数组的 size 和 data 子树，并为字节数组使用连续字节路径
// readArrayValue reads a Unity array's size and data subtrees and uses a contiguous-byte path for byte arrays
func readArrayValue(tt *TypeTreeType, r *binaryio.EndianReader, idx int64, v *TypeTreeValue) (*TypeTreeValue, int64, error) {
	node := &tt.Nodes[idx]
	next := idx + 1
	if next >= int64(len(tt.Nodes)) {
		return v, next, fmt.Errorf("array %s has no Array node", v.Name)
	}

	// Unity 数组通常编码为 vector、list 或 TypelessData 字段下的 Array、int size 和 T data
	// 某些版本把 TypeFlags 放在字段自身，本读取器兼容两种形状，并回退到首个名为 size 或 data 的后代节点
	// Unity arrays are normally encoded as a vector, list, or TypelessData field containing Array, int size, and T data
	// Some versions put TypeFlags on the field itself; this reader accepts both shapes and falls back to the first descendant named size or data
	arrayNodeIdx := idx
	if tt.GetTypeTreeString(&tt.Nodes[next], false) == "Array" {
		arrayNodeIdx = next
		next++
	}
	alignArray := func() error {
		// Unity TypeTree 中四字节对齐标志通常属于嵌套 Array 节点而非 vector 或 list 字段
		// 必须先消费整个数组（包括所有元素）的字节，再应用对齐
		// In Unity TypeTrees the four-byte alignment flag normally belongs to the nested Array node rather than its vector or list field
		// The bytes for the whole array, including all elements, must be consumed before alignment is applied
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
	if minBytes := minimumTypeTreeValueBytes(tt, dataNodeIdx); minBytes > 0 && size > r.Remaining()/minBytes {
		return nil, elemNext, fmt.Errorf("array %s size %d requires at least %d bytes but only %d remain", v.Name, size, size*minBytes, r.Remaining())
	}

	if size == 0 {
		v.Children = []*TypeTreeValue{}
		if err := alignArray(); err != nil {
			return nil, skipSubtree(tt, idx), err
		}
		return v, skipSubtree(tt, idx), nil
	}

	if isByteElement(elemType) && elemNext == dataNodeIdx+1 {
		buf, err := r.ReadBytes(size)
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

	// 元数据子树只按结构消费一次，与元素数量无关
	// The metadata subtree is consumed once structurally regardless of element count
	if err := alignArray(); err != nil {
		return nil, skipSubtree(tt, idx), err
	}
	return v, skipSubtree(tt, idx), nil
}

// readPrimitiveValue 根据 TypeTree 类型名读取一个标量、字符串或 TypelessData 值
// readPrimitiveValue reads one scalar, string, or TypelessData value according to the TypeTree type name
func readPrimitiveValue(r *binaryio.EndianReader, v *TypeTreeValue) error {
	switch v.TypeName {
	case "string":
		s, err := r.ReadAlignedString()
		v.Value = s
		return err
	case "TypelessData":
		sizeRaw, err := r.ReadInt32()
		if err != nil {
			return err
		}
		if sizeRaw < 0 {
			return fmt.Errorf("invalid TypelessData size %d", sizeRaw)
		}
		size := int64(sizeRaw)
		if size > r.Remaining() {
			return fmt.Errorf("invalid TypelessData size %d", sizeRaw)
		}
		buf, err := r.ReadBytes(size)
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
	case "unsigned long long", "UInt64", "FileSize":
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

// alignReader4 将读取位置推进到下一个四字节边界
// alignReader4 advances the reader to the next four-byte boundary
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

// minimumTypeTreeValueBytes 计算节点值的保守最小字节数，用于校验数组计数的可行性
// minimumTypeTreeValueBytes computes a conservative minimum byte count for a node to validate array counts
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
	case "long long", "SInt64", "unsigned long long", "UInt64", "FileSize", "double":
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

// isSpecialPrimitiveType 判断类型是否需要专门的变长读取逻辑
// isSpecialPrimitiveType reports whether a type requires dedicated variable-length reading logic
func isSpecialPrimitiveType(typeName string) bool {
	switch typeName {
	case "string", "TypelessData":
		return true
	default:
		return false
	}
}

// isArrayNode 根据 TypeFlags 或类型名判断 TypeTree 节点是否表示数组
// isArrayNode reports whether a TypeTree node represents an array from TypeFlags or its type name
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

// isByteElement 判断数组元素是否可以按连续字节读取
// isByteElement reports whether array elements can be read as contiguous bytes
func isByteElement(typeName string) bool {
	switch typeName {
	case "UInt8", "unsigned char", "SInt8", "char":
		return true
	default:
		return false
	}
}

// skipSubtree 返回跳过节点及其所有后代后的下一个节点索引
// skipSubtree returns the next node index after a node and all of its descendants
func skipSubtree(tt *TypeTreeType, idx int64) int64 {
	level := tt.Nodes[idx].Level
	idx++
	for idx < int64(len(tt.Nodes)) && tt.Nodes[idx].Level > level {
		idx++
	}
	return idx
}

// Field 返回名称匹配的第一个直接子节点
// Field returns the first direct child with the requested field name
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

// FieldPath 按字段名依次遍历直接子节点
// FieldPath walks through direct children by field names
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

// String 将节点值断言为字符串并返回是否成功
// String asserts the node value as a string and reports whether it succeeded
func (v *TypeTreeValue) String() (string, bool) {
	if v == nil {
		return "", false
	}
	s, ok := v.Value.(string)
	return s, ok
}

// Int64 将支持的有符号或可表示无符号整数转换为 Int64
// Int64 converts supported signed or representable unsigned integers to Int64
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

// UInt64 返回非负整数，并保留大于 MaxInt64 的 UInt64 值
// UInt64 returns a non-negative integer without losing values above MaxInt64
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

// Float32 将 Float32 或 Float64 节点值转换为 Float32
// Float32 converts a Float32 or Float64 node value to Float32
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

// Bytes 返回节点中的原始字节，或将子节点整数转换为字节数组
// Bytes returns raw bytes in the node or converts integer child nodes to a byte array
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
