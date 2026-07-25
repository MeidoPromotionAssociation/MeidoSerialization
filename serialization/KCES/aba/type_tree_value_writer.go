package aba

import (
	"bytes"
	"fmt"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// EncodeAssetValue 按资源对应的 TypeTree 和对象字节序重新编码已解码值
// EncodeAssetValue re-encodes a decoded value using the asset TypeTree and object byte order
func (af *AssetsFile) EncodeAssetValue(info *AssetInfo, root *TypeTreeValue) ([]byte, error) {
	if af == nil {
		return nil, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, fmt.Errorf("nil asset info")
	}
	if root == nil {
		return nil, fmt.Errorf("nil asset value")
	}
	tt, err := af.typeTreeForAsset(info)
	if err != nil {
		return nil, err
	}
	if len(tt.Nodes) == 0 {
		return nil, fmt.Errorf("type tree for class %d has no nodes", info.TypeId)
	}

	var buf bytes.Buffer
	w := binaryio.NewEndianWriter(&buf, af.byteOrder())
	next, err := writeTypeTreeValue(tt, w, 0, root)
	if err != nil {
		return nil, err
	}
	if next != int64(len(tt.Nodes)) {
		return nil, fmt.Errorf("type tree for class %d stopped at node %d/%d", info.TypeId, next, len(tt.Nodes))
	}
	return buf.Bytes(), nil
}

// writeTypeTreeValue 按 TypeTree 先序节点递归写入一个值并返回下一个未消费节点索引
// writeTypeTreeValue recursively writes one value in TypeTree preorder and returns the next unconsumed node index
func writeTypeTreeValue(tt *TypeTreeType, w *binaryio.EndianWriter, idx int64, value *TypeTreeValue) (int64, error) {
	if tt == nil {
		return idx, fmt.Errorf("nil type tree")
	}
	if w == nil {
		return idx, fmt.Errorf("nil type tree writer")
	}
	if value == nil {
		return idx, fmt.Errorf("nil type tree value at node %d", idx)
	}
	if idx < 0 || idx >= int64(len(tt.Nodes)) {
		return idx, fmt.Errorf("type tree node index %d out of range", idx)
	}

	node := &tt.Nodes[idx]
	typeName := tt.GetTypeTreeString(node, true)
	if value.TypeName != "" && typeName != "" && value.TypeName != typeName {
		return idx, fmt.Errorf("type tree node %d expects %q but value is %q", idx, typeName, value.TypeName)
	}

	if isSpecialPrimitiveType(typeName) {
		if err := writePrimitiveTypeTreeValue(w, typeName, value.Value); err != nil {
			return idx + 1, fmt.Errorf("write %s %s: %w", typeName, value.Name, err)
		}
		if node.MetaFlags&0x4000 != 0 {
			if err := w.Align(4); err != nil {
				return skipSubtree(tt, idx), fmt.Errorf("align %s %s: %w", typeName, value.Name, err)
			}
		}
		return skipSubtree(tt, idx), nil
	}

	next := idx + 1
	if next < int64(len(tt.Nodes)) && tt.Nodes[next].Level > node.Level {
		if isArrayNode(node, typeName) {
			var err error
			next, err = writeTypeTreeArrayValue(tt, w, idx, value)
			if err != nil {
				return next, err
			}
		} else {
			childIndex := int64(0)
			for next < int64(len(tt.Nodes)) && tt.Nodes[next].Level > node.Level {
				if childIndex >= int64(len(value.Children)) {
					return next, fmt.Errorf("type tree value %s is missing child for node %d", value.Name, next)
				}
				var err error
				next, err = writeTypeTreeValue(tt, w, next, value.Children[childIndex])
				if err != nil {
					return next, err
				}
				childIndex++
			}
			if childIndex != int64(len(value.Children)) {
				return next, fmt.Errorf("type tree value %s has %d children but schema consumed %d", value.Name, len(value.Children), childIndex)
			}
		}
	} else if err := writePrimitiveTypeTreeValue(w, typeName, value.Value); err != nil {
		return next, fmt.Errorf("write %s %s: %w", typeName, value.Name, err)
	}

	if node.MetaFlags&0x4000 != 0 {
		if err := w.Align(4); err != nil {
			return next, fmt.Errorf("align %s %s: %w", typeName, value.Name, err)
		}
	}
	return next, nil
}

// writeTypeTreeArrayValue 写入 Unity 数组的长度、元素和数组级对齐
// writeTypeTreeArrayValue writes a Unity array length, elements, and array-level alignment
func writeTypeTreeArrayValue(tt *TypeTreeType, w *binaryio.EndianWriter, idx int64, value *TypeTreeValue) (int64, error) {
	node := &tt.Nodes[idx]
	next := idx + 1
	if next >= int64(len(tt.Nodes)) {
		return next, fmt.Errorf("array %s has no Array node", value.Name)
	}

	arrayNodeIdx := idx
	if tt.GetTypeTreeString(&tt.Nodes[next], false) == "Array" {
		arrayNodeIdx = next
		next++
	}
	if next >= int64(len(tt.Nodes)) {
		return next, fmt.Errorf("array %s missing size node", value.Name)
	}

	sizeNodeIdx := next
	if tt.GetTypeTreeString(&tt.Nodes[sizeNodeIdx], false) != "size" {
		for sizeNodeIdx < int64(len(tt.Nodes)) && tt.Nodes[sizeNodeIdx].Level > node.Level && tt.GetTypeTreeString(&tt.Nodes[sizeNodeIdx], false) != "size" {
			sizeNodeIdx++
		}
		if sizeNodeIdx >= int64(len(tt.Nodes)) || tt.Nodes[sizeNodeIdx].Level <= node.Level {
			return next, fmt.Errorf("array %s missing size node", value.Name)
		}
	}

	dataNodeIdx := sizeNodeIdx + 1
	for dataNodeIdx < int64(len(tt.Nodes)) && tt.Nodes[dataNodeIdx].Level > node.Level && tt.GetTypeTreeString(&tt.Nodes[dataNodeIdx], false) != "data" {
		dataNodeIdx++
	}
	if dataNodeIdx >= int64(len(tt.Nodes)) || tt.Nodes[dataNodeIdx].Level <= node.Level {
		return skipSubtree(tt, idx), fmt.Errorf("array %s missing data node", value.Name)
	}

	dataNode := &tt.Nodes[dataNodeIdx]
	elemNext := skipSubtree(tt, dataNodeIdx)
	elemType := tt.GetTypeTreeString(dataNode, true)
	if isByteElement(elemType) && elemNext == dataNodeIdx+1 {
		data, ok := value.Value.([]byte)
		if !ok {
			if len(value.Children) != 0 {
				return elemNext, fmt.Errorf("byte array %s has child values instead of contiguous bytes", value.Name)
			}
			data = []byte{}
		}
		if uint64(len(data)) > uint64(math.MaxInt32) {
			return elemNext, fmt.Errorf("array %s length %d exceeds Int32 wire range", value.Name, len(data))
		}
		if err := w.WriteInt32(int32(len(data))); err != nil {
			return elemNext, err
		}
		if err := w.WriteBytes(data); err != nil {
			return elemNext, err
		}
	} else {
		count := int64(len(value.Children))
		if count > int64(math.MaxInt32) {
			return elemNext, fmt.Errorf("array %s length %d exceeds Int32 wire range", value.Name, count)
		}
		if err := w.WriteInt32(int32(count)); err != nil {
			return elemNext, err
		}
		for elementIndex := int64(0); elementIndex < count; elementIndex++ {
			if _, err := writeTypeTreeValue(tt, w, dataNodeIdx, value.Children[elementIndex]); err != nil {
				return dataNodeIdx, fmt.Errorf("write %s[%d]: %w", value.Name, elementIndex, err)
			}
		}
	}

	if tt.Nodes[arrayNodeIdx].MetaFlags&0x4000 != 0 {
		if err := w.Align(4); err != nil {
			return skipSubtree(tt, idx), fmt.Errorf("align array %s: %w", value.Name, err)
		}
	}
	return skipSubtree(tt, idx), nil
}

// writePrimitiveTypeTreeValue 按 TypeTree 标量类型写入单个值
// writePrimitiveTypeTreeValue writes one value using a TypeTree scalar type
func writePrimitiveTypeTreeValue(w *binaryio.EndianWriter, typeName string, value interface{}) error {
	switch typeName {
	case "":
		return nil
	case "string":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("value is %T instead of string", value)
		}
		return w.WriteAlignedString(s)
	case "TypelessData":
		data, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("value is %T instead of []byte", value)
		}
		if uint64(len(data)) > uint64(math.MaxInt32) {
			return fmt.Errorf("TypelessData length %d exceeds Int32 wire range", len(data))
		}
		if err := w.WriteInt32(int32(len(data))); err != nil {
			return err
		}
		return w.WriteBytes(data)
	case "bool":
		b, ok := value.(bool)
		if !ok {
			return fmt.Errorf("value is %T instead of bool", value)
		}
		return w.WriteBool(b)
	case "char", "SInt8":
		n, ok := signedTypeTreeInteger(value)
		if !ok || n < math.MinInt8 || n > math.MaxInt8 {
			return fmt.Errorf("value %v is outside Int8 range", value)
		}
		return w.WriteByte(byte(int8(n)))
	case "UInt8", "unsigned char":
		n, ok := unsignedTypeTreeInteger(value)
		if !ok || n > math.MaxUint8 {
			return fmt.Errorf("value %v is outside UInt8 range", value)
		}
		return w.WriteByte(byte(n))
	case "short", "SInt16":
		n, ok := signedTypeTreeInteger(value)
		if !ok || n < math.MinInt16 || n > math.MaxInt16 {
			return fmt.Errorf("value %v is outside Int16 range", value)
		}
		return w.WriteInt16(int16(n))
	case "unsigned short", "UInt16":
		n, ok := unsignedTypeTreeInteger(value)
		if !ok || n > math.MaxUint16 {
			return fmt.Errorf("value %v is outside UInt16 range", value)
		}
		return w.WriteUInt16(uint16(n))
	case "int", "SInt32":
		n, ok := signedTypeTreeInteger(value)
		if !ok || n < math.MinInt32 || n > math.MaxInt32 {
			return fmt.Errorf("value %v is outside Int32 range", value)
		}
		return w.WriteInt32(int32(n))
	case "unsigned int", "UInt32", "Type*":
		n, ok := unsignedTypeTreeInteger(value)
		if !ok || n > math.MaxUint32 {
			return fmt.Errorf("value %v is outside UInt32 range", value)
		}
		return w.WriteUInt32(uint32(n))
	case "long long", "SInt64":
		n, ok := signedTypeTreeInteger(value)
		if !ok {
			return fmt.Errorf("value %v is outside Int64 range", value)
		}
		return w.WriteInt64(n)
	case "unsigned long long", "UInt64", "FileSize":
		n, ok := unsignedTypeTreeInteger(value)
		if !ok {
			return fmt.Errorf("value %v is outside UInt64 range", value)
		}
		return w.WriteUInt64(n)
	case "float":
		f, ok := typeTreeFloat32(value)
		if !ok {
			return fmt.Errorf("value %v is not a Float32", value)
		}
		return w.WriteFloat32(f)
	case "double":
		f, ok := typeTreeFloat64(value)
		if !ok {
			return fmt.Errorf("value %v is not a Float64", value)
		}
		return w.WriteFloat64(f)
	default:
		return fmt.Errorf("unsupported primitive type %q", typeName)
	}
}

// signedTypeTreeInteger 将受支持的 TypeTree 整数值转换为 Int64
// signedTypeTreeInteger converts a supported TypeTree integer value to Int64
func signedTypeTreeInteger(value interface{}) (int64, bool) {
	switch n := value.(type) {
	case int64:
		return n, true
	case uint64:
		if n > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(n), true
	case uint32:
		return int64(n), true
	default:
		return 0, false
	}
}

// unsignedTypeTreeInteger 将受支持的非负 TypeTree 整数值转换为 UInt64
// unsignedTypeTreeInteger converts a supported non-negative TypeTree integer value to UInt64
func unsignedTypeTreeInteger(value interface{}) (uint64, bool) {
	switch n := value.(type) {
	case uint64:
		return n, true
	case uint32:
		return uint64(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	default:
		return 0, false
	}
}

// typeTreeFloat32 将受支持的 TypeTree 浮点值转换为 Float32
// typeTreeFloat32 converts a supported TypeTree floating-point value to Float32
func typeTreeFloat32(value interface{}) (float32, bool) {
	switch f := value.(type) {
	case float32:
		return f, true
	case float64:
		return float32(f), true
	default:
		return 0, false
	}
}

// typeTreeFloat64 将受支持的 TypeTree 浮点值转换为 Float64
// typeTreeFloat64 converts a supported TypeTree floating-point value to Float64
func typeTreeFloat64(value interface{}) (float64, bool) {
	switch f := value.(type) {
	case float64:
		return f, true
	case float32:
		return float64(f), true
	default:
		return 0, false
	}
}
