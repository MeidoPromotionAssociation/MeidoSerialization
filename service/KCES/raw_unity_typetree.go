package KCES

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

const (
	RawUnityTypeTreeFormat = "kces-unity-typetree"
	typeTreeInlineByteMax  = 256
	typeTreeBytePreviewMax = 64
)

// RawUnityTypeTreeEnvelope 是从 Unity TypeTree 元数据解码出的只读 JSON 视图 / RawUnityTypeTreeEnvelope is a read-only JSON view decoded from Unity TypeTree metadata
// 它只在源 .aba 上下文仍可用时生成，重打包仍以相邻 raw .bytes 文件为准 / It is generated while the source .aba context is still available, and repacking remains based on the adjacent raw .bytes file
type RawUnityTypeTreeEnvelope struct {
	Format   string             `json:"format"`             // 封套格式标识，固定为 kces-unity-typetree / Envelope format marker, fixed to kces-unity-typetree
	ClassID  int32              `json:"classId"`            // Unity ClassID / Unity ClassID
	TypeName string             `json:"typeName,omitempty"` // Unity 类型名 / Unity type name
	Name     string             `json:"name,omitempty"`     // 对象名称 / Object name
	PathID   int64              `json:"pathId,omitempty"`   // Unity PathID / Unity PathID
	LoadName string             `json:"loadName,omitempty"` // AssetBundle 加载名 / AssetBundle load name
	Value    *TypeTreeJSONValue `json:"value"`              // TypeTree 解码后的根值 / Root value decoded from TypeTree
}

// TypeTreeJSONValue 表示 TypeTreeValue 的 JSON 形态 / TypeTreeJSONValue represents the JSON form of TypeTreeValue
type TypeTreeJSONValue struct {
	TypeName string               `json:"typeName,omitempty"` // Unity 类型名 / Unity type name
	Name     string               `json:"name,omitempty"`     // 字段名或节点名 / Field or node name
	Value    interface{}          `json:"value,omitempty"`    // 标量值 / Scalar value
	Bytes    *TypeTreeJSONBytes   `json:"bytes,omitempty"`    // 字节数组摘要或内联数据 / Byte-array summary or inline data
	Children []*TypeTreeJSONValue `json:"children,omitempty"` // 子节点列表 / Child node list
}

// TypeTreeJSONBytes 表示 TypeTree 字节数组的 JSON 摘要 / TypeTreeJSONBytes represents a JSON summary of TypeTree byte arrays
type TypeTreeJSONBytes struct {
	Length        int64  `json:"length"`                  // 字节长度 / Byte length
	SHA256        string `json:"sha256"`                  // 原始字节 SHA256 / SHA256 of raw bytes
	DataBase64    string `json:"dataBase64,omitempty"`    // 小字节数组的完整 base64 / Full base64 for small byte arrays
	PreviewBase64 string `json:"previewBase64,omitempty"` // 大字节数组的预览 base64 / Preview base64 for large byte arrays
	Truncated     bool   `json:"truncated,omitempty"`     // 是否截断预览 / Whether the preview is truncated
}

func writeRawUnityTypeTreeSidecar(assetPath string, af *aba.AssetsFile, info *aba.AssetInfo, entry aba.AssetEntry, loadName string) error {
	data, err := marshalRawUnityTypeTreeSidecar(af, info, entry, loadName)
	if err != nil {
		return err
	}
	return os.WriteFile(typeTreeSidecarPath(assetPath), data, 0644)
}

func marshalRawUnityTypeTreeSidecar(af *aba.AssetsFile, info *aba.AssetInfo, entry aba.AssetEntry, loadName string) ([]byte, error) {
	root, err := af.ReadAssetValue(info)
	if err != nil {
		return nil, err
	}
	envelope := &RawUnityTypeTreeEnvelope{
		Format:   RawUnityTypeTreeFormat,
		ClassID:  entry.TypeId,
		TypeName: entry.TypeName,
		Name:     entry.Name,
		PathID:   entry.PathId,
		LoadName: loadName,
		Value:    typeTreeJSONValue(root),
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal TypeTree sidecar: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

func typeTreeSidecarPath(assetPath string) string {
	return assetPath + ".typetree.json"
}

func readRawUnityTypeTreeSidecar(assetPath string) (*RawUnityTypeTreeEnvelope, error) {
	data, err := os.ReadFile(typeTreeSidecarPath(assetPath))
	if err != nil {
		return nil, err
	}
	var envelope RawUnityTypeTreeEnvelope
	if err := json.Unmarshal(trimJSONUTF8BOM(data), &envelope); err != nil {
		return nil, err
	}
	if envelope.Format != RawUnityTypeTreeFormat {
		return nil, fmt.Errorf("unsupported TypeTree sidecar format %q", envelope.Format)
	}
	return &envelope, nil
}

func writeRawUnityTypeTreeEnvelope(assetPath string, envelope *RawUnityTypeTreeEnvelope) error {
	data, err := marshalRawUnityTypeTreeEnvelope(envelope)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	return os.WriteFile(typeTreeSidecarPath(assetPath), data, 0644)
}

func marshalRawUnityTypeTreeEnvelope(envelope *RawUnityTypeTreeEnvelope) ([]byte, error) {
	if envelope == nil {
		return nil, nil
	}
	if envelope.Format == "" {
		envelope.Format = RawUnityTypeTreeFormat
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal TypeTree sidecar: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

func typeTreeJSONValue(v *aba.TypeTreeValue) *TypeTreeJSONValue {
	if v == nil {
		return nil
	}
	out := &TypeTreeJSONValue{
		TypeName: v.TypeName,
		Name:     v.Name,
	}
	if b, ok := v.Value.([]byte); ok {
		out.Bytes = typeTreeJSONBytes(b)
	} else if v.Value != nil {
		out.Value = v.Value
	}
	if len(v.Children) > 0 {
		out.Children = make([]*TypeTreeJSONValue, 0, len(v.Children))
		for _, child := range v.Children {
			out.Children = append(out.Children, typeTreeJSONValue(child))
		}
	}
	return out
}

// editableTypeTreeJSONValue 将 TypeTree 值转换为可完整重编码的 JSON，并对 64 位整数使用十进制字符串避免精度损失
// editableTypeTreeJSONValue converts a TypeTree value into fully re-encodable JSON and uses decimal strings for 64-bit integers to avoid precision loss
func editableTypeTreeJSONValue(value *aba.TypeTreeValue) *TypeTreeJSONValue {
	if value == nil {
		return nil
	}
	out := &TypeTreeJSONValue{TypeName: value.TypeName, Name: value.Name}
	if data, ok := value.Value.([]byte); ok {
		sum := sha256.Sum256(data)
		out.Bytes = &TypeTreeJSONBytes{
			Length:     int64(len(data)),
			SHA256:     hex.EncodeToString(sum[:]),
			DataBase64: base64.StdEncoding.EncodeToString(data),
		}
	} else if value.Value != nil {
		switch value.TypeName {
		case "long long", "SInt64":
			if number, ok := value.Value.(int64); ok {
				out.Value = strconv.FormatInt(number, 10)
			} else {
				out.Value = value.Value
			}
		case "unsigned long long", "UInt64", "FileSize":
			if number, ok := value.Value.(uint64); ok {
				out.Value = strconv.FormatUint(number, 10)
			} else {
				out.Value = value.Value
			}
		default:
			out.Value = value.Value
		}
	}
	if len(value.Children) != 0 {
		out.Children = make([]*TypeTreeJSONValue, 0, len(value.Children))
		for _, child := range value.Children {
			out.Children = append(out.Children, editableTypeTreeJSONValue(child))
		}
	}
	return out
}

// editableTypeTreeValueFromJSON 将可编辑 JSON 节点恢复为 TypeTreeValue，并忽略非语义摘要字段
// editableTypeTreeValueFromJSON restores an editable JSON node as a TypeTreeValue and ignores non-semantic digest fields
func editableTypeTreeValueFromJSON(value *TypeTreeJSONValue) (*aba.TypeTreeValue, error) {
	if value == nil {
		return nil, fmt.Errorf("nil TypeTree JSON value")
	}
	out := &aba.TypeTreeValue{TypeName: value.TypeName, Name: value.Name}
	if value.Bytes != nil {
		if value.Value != nil || len(value.Children) != 0 {
			return nil, fmt.Errorf("%s %s combines bytes with scalar or child values", value.TypeName, value.Name)
		}
		if value.Bytes.DataBase64 == "" && value.Bytes.Length != 0 {
			return nil, fmt.Errorf("byte array %s has no dataBase64", value.Name)
		}
		data, err := base64.StdEncoding.DecodeString(value.Bytes.DataBase64)
		if err != nil {
			return nil, fmt.Errorf("decode byte array %s: %w", value.Name, err)
		}
		out.Value = data
	} else if value.Value != nil {
		scalar, err := editableTypeTreeScalarFromJSON(value.TypeName, value.Value)
		if err != nil {
			return nil, fmt.Errorf("decode %s %s: %w", value.TypeName, value.Name, err)
		}
		out.Value = scalar
	}
	if len(value.Children) != 0 {
		out.Children = make([]*aba.TypeTreeValue, 0, len(value.Children))
		for childIndex, child := range value.Children {
			decoded, err := editableTypeTreeValueFromJSON(child)
			if err != nil {
				return nil, fmt.Errorf("child[%d]: %w", childIndex, err)
			}
			out.Children = append(out.Children, decoded)
		}
	}
	return out, nil
}

// editableTypeTreeScalarFromJSON 按 TypeTree 标量类型恢复 encoding/json 解码后的值
// editableTypeTreeScalarFromJSON restores a value decoded by encoding/json according to its TypeTree scalar type
func editableTypeTreeScalarFromJSON(typeName string, value interface{}) (interface{}, error) {
	switch typeName {
	case "string":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("value is %T instead of string", value)
		}
		return text, nil
	case "bool":
		boolean, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("value is %T instead of bool", value)
		}
		return boolean, nil
	case "char", "SInt8", "short", "SInt16", "int", "SInt32":
		return editableJSONSignedInteger(value, math.MinInt32, math.MaxInt32)
	case "UInt8", "unsigned char", "unsigned short", "UInt16", "unsigned int", "UInt32", "Type*":
		return editableJSONUnsignedInteger(value, math.MaxUint32)
	case "long long", "SInt64":
		return editableJSONSignedInteger(value, math.MinInt64, math.MaxInt64)
	case "unsigned long long", "UInt64", "FileSize":
		return editableJSONUnsignedInteger(value, math.MaxUint64)
	case "float":
		number, err := editableJSONFloat64(value)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < -math.MaxFloat32 || number > math.MaxFloat32 {
			return nil, fmt.Errorf("value %v is outside Float32 range", value)
		}
		return float32(number), nil
	case "double":
		number, err := editableJSONFloat64(value)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Errorf("value %v is not a finite Float64", value)
		}
		return number, nil
	default:
		return value, nil
	}
}

// editableJSONSignedInteger 将 JSON 数字或十进制字符串转换为指定范围内的 Int64
// editableJSONSignedInteger converts a JSON number or decimal string to an Int64 inside the requested range
func editableJSONSignedInteger(value interface{}, minimum int64, maximum int64) (int64, error) {
	var number int64
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, err
		}
		number = parsed
	case json.Number:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		if err != nil {
			return 0, err
		}
		number = parsed
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed || typed < float64(math.MinInt64) || typed >= -float64(math.MinInt64) {
			return 0, fmt.Errorf("value %v is not an exact Int64", value)
		}
		number = int64(typed)
	default:
		return 0, fmt.Errorf("value is %T instead of an integer", value)
	}
	if number < minimum || number > maximum {
		return 0, fmt.Errorf("value %d is outside [%d,%d]", number, minimum, maximum)
	}
	return number, nil
}

// editableJSONUnsignedInteger 将 JSON 数字或十进制字符串转换为指定范围内的 UInt64
// editableJSONUnsignedInteger converts a JSON number or decimal string to a UInt64 inside the requested range
func editableJSONUnsignedInteger(value interface{}, maximum uint64) (uint64, error) {
	var number uint64
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.ParseUint(typed, 10, 64)
		if err != nil {
			return 0, err
		}
		number = parsed
	case json.Number:
		parsed, err := strconv.ParseUint(string(typed), 10, 64)
		if err != nil {
			return 0, err
		}
		number = parsed
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed || typed < 0 || typed >= math.Exp2(64) {
			return 0, fmt.Errorf("value %v is not an exact UInt64", value)
		}
		number = uint64(typed)
	default:
		return 0, fmt.Errorf("value is %T instead of an unsigned integer", value)
	}
	if number > maximum {
		return 0, fmt.Errorf("value %d exceeds %d", number, maximum)
	}
	return number, nil
}

// editableJSONFloat64 将 JSON 数字恢复为 Float64
// editableJSONFloat64 restores a JSON number as a Float64
func editableJSONFloat64(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case json.Number:
		return strconv.ParseFloat(string(typed), 64)
	case float64:
		return typed, nil
	default:
		return 0, fmt.Errorf("value is %T instead of a number", value)
	}
}

// typeTreeJSONBytes 为 TypeTree 字节值生成固定宽度长度、摘要及可选预览
// typeTreeJSONBytes creates a fixed-width length, digest, and optional preview for a TypeTree byte value
func typeTreeJSONBytes(data []byte) *TypeTreeJSONBytes {
	sum := sha256.Sum256(data)
	out := &TypeTreeJSONBytes{
		Length: int64(len(data)),
		SHA256: hex.EncodeToString(sum[:]),
	}
	if len(data) <= typeTreeInlineByteMax {
		out.DataBase64 = base64.StdEncoding.EncodeToString(data)
		return out
	}
	previewLen := typeTreeBytePreviewMax
	if len(data) < previewLen {
		previewLen = len(data)
	}
	out.PreviewBase64 = base64.StdEncoding.EncodeToString(data[:previewLen])
	out.Truncated = true
	return out
}
