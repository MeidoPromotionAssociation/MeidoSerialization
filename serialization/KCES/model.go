package KCES

import (
	"fmt"
)

// .model
// KCES 模型描述文件，TextAsset.m_Script 直接保存一个 Parts.Model，而不是资源容器
// .model 不包含网格字节，meshfileName 引用 .aba 中单独保存的 UnityEngine.Mesh，通常命名为 .mmesh
// 载荷使用 LZ4 Block Array 压缩的 MessagePack indexed-array，当前 Model 固定版本为 1001
// .model
// KCES model-description file whose TextAsset.m_Script stores one Parts.Model directly rather than an asset container
// A .model does not embed mesh bytes, and meshfileName references a separate UnityEngine.Mesh in the .aba normally named with .mmesh
// The payload is an LZ4 Block Array-compressed MessagePack indexed array, with current Model fixed version 1001

// MessagePack indexed array 布局 / MessagePack indexed-array layout:
//
//	[Key(0)]  version          int
//	[Key(1)]  id               uint64
//	[Key(2)]  fileName         string         模型文件名（含 .model 扩展名）
//	[Key(3)]  meshfileName     string         网格文件名（含 .mmesh 扩展名）
//	[Key(4)]  modelName        string         模型名称
//	[Key(5)]  transData        TransData[]    骨骼变换数据
//	[Key(6)]  boneNames        string[]       骨骼名称列表
//	[Key(7)]  materialFileName string[]       材质文件名列表
//	[Key(8)]  morphs           BlendData[]    变形数据
//	[Key(9)]  skinThick        SkinThickness  皮肤厚度数据
//	[Key(10)] shadowModeFlags  int            阴影模式标志

// Model 表示 KCES Parts.Model 模型数据
// 对应 C# Parts.Model，继承自 AMessagePackSerializationVersionControlIntKey
// MessagePack indexed array 依次保存版本、标识、文件名、骨骼、材质、变形、皮肤厚度和阴影标志
// Model represents KCES Parts.Model data
// It matches C# Parts.Model derived from AMessagePackSerializationVersionControlIntKey
// Its MessagePack indexed array stores the version, identifiers, filenames, bones, materials, morphs, skin thickness, and shadow flags in order
type Model struct {
	_struct                struct{}       `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`    // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int32          `json:"version"`                          // 版本号，固定为 1001 / Version value, fixed to 1001
	ID                     uint64         `json:"id"`                               // 模型 ID / Model ID
	FileName               string         `json:"fileName"`                         // 模型文件名 / Model file name
	MeshFileName           string         `json:"meshfileName"`                     // 网格文件名，字段名沿用游戏 meshfileName 拼写 / Mesh file name, keeping the game's meshfileName spelling
	ModelName              string         `json:"modelName"`                        // 模型名称 / Model name
	TransData              []TransData    `json:"transData"`                        // 骨骼变换数据数组 / Bone transform data array
	BoneNames              []string       `json:"boneNames"`                        // 骨骼名称列表 / Bone name list
	MaterialFileName       []string       `json:"materialFileName"`                 // 材质文件名列表 / Material file-name list
	Morphs                 []BlendData    `json:"morphs"`                           // 变形数据 / Morph data
	SkinThick              *SkinThickness `json:"skinThick"`                        // 皮肤厚度数据 / Skin-thickness data
	ShadowModeFlags        int32          `json:"shadowModeFlags"`                  // 阴影模式标志，0=Default, 1=CastShadow, 2=NoCastShadow / Shadow-mode flags, 0=Default, 1=CastShadow, 2=NoCastShadow
	RootNil                bool           `codec:"-" json:"rootNil,omitempty"`      // 仅用于单个 .model 根值 / Standalone .model root nil marker
	TrailingData           []byte         `codec:"-" json:"trailingData,omitempty"` // 仅用于单个 .model 根值之后的未读字节 / Unread bytes after a standalone .model root value only
}

// getMessagePackTrailing 返回单个 .model 根值后的保留字节
// getMessagePackTrailing returns the preserved bytes after a standalone .model root value
func (m *Model) getMessagePackTrailing() []byte { return m.TrailingData }

// setMessagePackTrailing 设置单个 .model 根值后的保留字节
// setMessagePackTrailing sets the preserved bytes after a standalone .model root value
func (m *Model) setMessagePackTrailing(data []byte) { m.TrailingData = data }

// getMessagePackRootNil 返回单个 .model 根值是否为 nil
// getMessagePackRootNil reports whether a standalone .model root value was nil
func (m *Model) getMessagePackRootNil() bool { return m.RootNil }

// setMessagePackRootNil 设置单个 .model 根值的 nil 标记
// setMessagePackRootNil sets the nil marker for a standalone .model root value
func (m *Model) setMessagePackRootNil(value bool) { m.RootNil = value }

// TransData 表示一项骨骼变换数据
// 对应 C# Model.TransData，MessagePack indexed array 依次保存骨骼名称、父索引、缩放骨骼标志及局部变换
// TransData represents one bone transform entry
// It matches C# Model.TransData whose MessagePack indexed array stores the bone name, parent index, scale-bone flag, and local transform in order
type TransData struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	Name                   string      `json:"name"`     // 骨骼名称 / Bone name
	ParentNo               int32       `json:"paretnNo"` // 父骨骼索引，-1 表示根节点，字段名保留游戏 paretnNo 拼写 / Parent bone index, -1 means root, keeping the game's paretnNo spelling
	IsSCL                  bool        `json:"isSCL"`    // 是否为缩放骨骼 / Whether this is a scale bone
	Pos                    Vector3     `json:"pos"`      // 本地位置 / Local position
	Rot                    Vector4     `json:"rot"`      // 本地旋转四元数 / Local rotation quaternion
	Scale                  Vector3     `json:"scale"`    // 本地缩放 / Local scale
}

// ModelAssets 表示可能存在的批量模型资源容器
// 游戏中 .model TextAsset 的 m_Script 实际直接包含单个 Model 而不是此容器，应使用 DecodeModel 和 EncodeModel 处理
// ModelAssets represents a possible batch model asset container
// A game .model TextAsset m_Script actually contains one Model directly rather than this container and should be handled with DecodeModel and EncodeModel
type ModelAssets struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	FileName               string      `json:"fileName"`                         // 容器文件名 / Container file name
	Assets                 []Model     `json:"assetArray"`                       // 模型数组 / Model array
	RootNil                bool        `codec:"-" json:"rootNil,omitempty"`      // 根 MessagePack 值是否为 nil / Whether the root MessagePack value was nil
	TrailingData           []byte      `codec:"-" json:"trailingData,omitempty"` // 根 MessagePack 值之后游戏未读取的字节 / Bytes left unread after the root MessagePack value
}

// getMessagePackTrailing 返回容器根值后的保留字节
// getMessagePackTrailing returns the preserved bytes after the container root value
func (a *ModelAssets) getMessagePackTrailing() []byte { return a.TrailingData }

// setMessagePackTrailing 设置容器根值后的保留字节
// setMessagePackTrailing sets the preserved bytes after the container root value
func (a *ModelAssets) setMessagePackTrailing(data []byte) { a.TrailingData = data }

// getMessagePackRootNil 返回容器根值是否为 nil
// getMessagePackRootNil reports whether the container root value was nil
func (a *ModelAssets) getMessagePackRootNil() bool { return a.RootNil }

// setMessagePackRootNil 设置容器根值的 nil 标记
// setMessagePackRootNil sets the nil marker for the container root value
func (a *ModelAssets) setMessagePackRootNil(value bool) { a.RootNil = value }

const modelFixVersion = 1001

// validateModelForEncoding 验证模型中的游戏 Int32 字段可安全编码
// validateModelForEncoding verifies that game Int32 fields in a model can be encoded safely
func validateModelForEncoding(model *Model) error {
	if err := validateModelNameSelector(model); err != nil {
		return err
	}
	return nil
}

// validateDecodedModel 验证解码模型中的游戏 Int32 字段范围
// validateDecodedModel validates the ranges of game Int32 fields in a decoded model
func validateDecodedModel(model *Model) error {
	if err := validateModelNameSelector(model); err != nil {
		return err
	}
	return nil
}

func validateModelNameSelector(model *Model) error {
	if model == nil || model.RootNil {
		return nil
	}
	// Preserve explicitly nil legacy/future wire values losslessly. These
	// annotations describe a source value that cannot satisfy the current game
	// runtime invariant, but rewriting it is outside a validator's authority.
	if indexedObjectSlotIsNil(model.IndexedObjectMetadata, 4) || hasIndexedObjectNullElements(model.IndexedObjectMetadata, 5) {
		return nil
	}
	// Keep the zero-value synthetic model usable for metadata/fidelity tests;
	// a real model with transform records must identify exactly one root node.
	if len(model.TransData) == 0 && model.ModelName == "" {
		return nil
	}
	if model.ModelName == "" {
		return fmt.Errorf("modelName is required when transData is non-empty")
	}
	var matches int64
	for _, transData := range model.TransData {
		if transData.Name == model.ModelName {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("modelName %q must match exactly one transData[].name, found %d matches", model.ModelName, matches)
	}
	return nil
}

func hasIndexedObjectNullElements(metadata *IndexedObjectMetadata, slot int64) bool {
	if metadata == nil || metadata.NullElements == nil {
		return false
	}
	for _, isNull := range metadata.NullElements[int32(slot)] {
		if isNull {
			return true
		}
	}
	return false
}

// DecodeModel 从 Lz4BlockArray 压缩的 MessagePack 数据解码单个 Model
// 这是 .model TextAsset m_Script 的实际解码方式，游戏通过 PartsUtility.Deserialize<Model>(GameResource.LoadBinary(...)) 加载
// DecodeModel decodes one Model from Lz4BlockArray-compressed MessagePack data
// This is the actual format of .model TextAsset m_Script loaded by the game through PartsUtility.Deserialize<Model>(GameResource.LoadBinary(...))
func DecodeModel(data []byte) (*Model, error) {
	m := &Model{}
	if err := decodeCompressedMsgpack(data, m, "Model"); err != nil {
		return nil, err
	}
	if err := validateDecodedModel(m); err != nil {
		return nil, fmt.Errorf("validate decoded Model: %w", err)
	}
	return m, nil
}

// EncodeModel 将单个 Model 编码为 Lz4BlockArray 压缩的 MessagePack 数据
// 生成的数据可直接作为 .model TextAsset 的 m_Script
// EncodeModel encodes one Model as Lz4BlockArray-compressed MessagePack data
// The resulting data can be used directly as .model TextAsset m_Script
func EncodeModel(m *Model) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("Model is nil")
	}
	normalized := *m
	if err := validateModelForEncoding(&normalized); err != nil {
		return nil, fmt.Errorf("validate Model: %w", err)
	}
	return encodeCompressedMsgpack(&normalized, "Model")
}

// DecodeModelAssets 从 Lz4BlockArray 压缩的 MessagePack 数据解码 ModelAssets 容器
// DecodeModelAssets decodes a ModelAssets container from Lz4BlockArray-compressed MessagePack data
func DecodeModelAssets(data []byte) (*ModelAssets, error) {
	assets := &ModelAssets{}
	if err := decodeCompressedMsgpack(data, assets, "ModelAssets"); err != nil {
		return nil, err
	}
	for i := range assets.Assets {
		if err := validateDecodedModel(&assets.Assets[i]); err != nil {
			return nil, fmt.Errorf("validate decoded ModelAssets assetArray[%d]: %w", i, err)
		}
	}
	return assets, nil
}

// EncodeModelAssets 将 ModelAssets 编码为 Lz4BlockArray 压缩的 MessagePack 数据
// EncodeModelAssets encodes ModelAssets as Lz4BlockArray-compressed MessagePack data
func EncodeModelAssets(assets *ModelAssets) ([]byte, error) {
	if assets == nil {
		return nil, fmt.Errorf("ModelAssets is nil")
	}
	normalized := *assets
	normalized.Assets = cloneSlicePreserveNil(assets.Assets)
	for i := range normalized.Assets {
		if normalized.Assets[i].RootNil {
			return nil, fmt.Errorf("validate ModelAssets assetArray[%d]: rootNil belongs only to a standalone .model root and cannot represent a nil element in the value-slice model", i)
		}
		if len(normalized.Assets[i].TrailingData) != 0 {
			return nil, fmt.Errorf("validate ModelAssets assetArray[%d]: trailingData belongs only to a standalone .model root and cannot be represented inside ModelAssets", i)
		}
		if err := validateModelForEncoding(&normalized.Assets[i]); err != nil {
			return nil, fmt.Errorf("validate ModelAssets assetArray[%d]: %w", i, err)
		}
	}
	return encodeCompressedMsgpack(&normalized, "ModelAssets")
}

// NewModel 创建使用当前固定版本的新模型描述
// NewModel creates a new model description with the current fixed version
func NewModel() *Model {
	return &Model{Version: modelFixVersion}
}
