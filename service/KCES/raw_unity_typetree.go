package KCES

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

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
	Length        int    `json:"length"`                  // 字节长度 / Byte length
	SHA256        string `json:"sha256"`                  // 原始字节 SHA256 / SHA256 of raw bytes
	DataBase64    string `json:"dataBase64,omitempty"`    // 小字节数组的完整 base64 / Full base64 for small byte arrays
	PreviewBase64 string `json:"previewBase64,omitempty"` // 大字节数组的预览 base64 / Preview base64 for large byte arrays
	Truncated     bool   `json:"truncated,omitempty"`     // 是否截断预览 / Whether the preview is truncated
}

func writeRawUnityTypeTreeSidecar(assetPath string, af *aba.AssetsFile, info *aba.AssetInfo, entry aba.AssetEntry, loadName string) error {
	root, err := af.ReadAssetValue(info)
	if err != nil {
		return err
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
		return fmt.Errorf("marshal TypeTree sidecar: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(typeTreeSidecarPath(assetPath), data, 0644)
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
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if envelope.Format != RawUnityTypeTreeFormat {
		return nil, fmt.Errorf("unsupported TypeTree sidecar format %q", envelope.Format)
	}
	return &envelope, nil
}

func writeRawUnityTypeTreeEnvelope(assetPath string, envelope *RawUnityTypeTreeEnvelope) error {
	if envelope == nil {
		return nil
	}
	if envelope.Format == "" {
		envelope.Format = RawUnityTypeTreeFormat
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal TypeTree sidecar: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(typeTreeSidecarPath(assetPath), data, 0644)
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

func typeTreeJSONBytes(data []byte) *TypeTreeJSONBytes {
	sum := sha256.Sum256(data)
	out := &TypeTreeJSONBytes{
		Length: len(data),
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
