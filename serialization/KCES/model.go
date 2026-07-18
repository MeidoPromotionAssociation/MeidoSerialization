package KCES

import (
	"fmt"
)

// .model
// KCES 模型描述文件，TextAsset.m_Script 直接保存一个 Parts.Model，而不是资源容器。
// .model 不包含网格字节；meshfileName 引用 .aba 中单独保存的 UnityEngine.Mesh（通常命名为 .mmesh）。
// 载荷使用 LZ4 Block 压缩的 MessagePack indexed-array；当前 Model 固定版本为 1001。
//
// .model
// KCES model-description file whose TextAsset.m_Script stores one Parts.Model directly rather than an asset container.
// A .model does not embed mesh bytes: meshfileName references a separate UnityEngine.Mesh in the .aba, normally named with .mmesh.
// The payload is an LZ4 Block-compressed MessagePack indexed array; the current Model fixed version is 1001.

// Model 表示模型数据 / Model represents KCES Parts.Model data
// 对应 C# Parts.Model，继承自 AMessagePackSerializationVersionControlIntKey / Matches C# Parts.Model, derived from AMessagePackSerializationVersionControlIntKey
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
type Model struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Version                int            `json:"version"`                          // 版本号，固定为 1001 / Version value, fixed to 1001
	ID                     uint64         `json:"id"`                               // 模型 ID / Model ID
	FileName               string         `json:"fileName"`                         // 模型文件名 / Model file name
	MeshFileName           string         `json:"meshfileName"`                     // 网格文件名，字段名沿用游戏 meshfileName 拼写 / Mesh file name, keeping the game's meshfileName spelling
	ModelName              string         `json:"modelName"`                        // 模型名称 / Model name
	TransData              []TransData    `json:"transData"`                        // 骨骼变换数据数组 / Bone transform data array
	BoneNames              []string       `json:"boneNames"`                        // 骨骼名称列表 / Bone name list
	MaterialFileName       []string       `json:"materialFileName"`                 // 材质文件名列表 / Material file-name list
	Morphs                 []BlendData    `json:"morphs"`                           // 变形数据 / Morph data
	SkinThick              *SkinThickness `json:"skinThick"`                        // 皮肤厚度数据 / Skin-thickness data
	ShadowModeFlags        int            `json:"shadowModeFlags"`                  // 阴影模式标志，0=Default, 1=CastShadow, 2=NoCastShadow / Shadow-mode flags, 0=Default, 1=CastShadow, 2=NoCastShadow
	RootNil                bool           `codec:"-" json:"rootNil,omitempty"`      // 仅用于单个 .model 根值 / Standalone .model root nil marker
	TrailingData           []byte         `codec:"-" json:"trailingData,omitempty"` // 仅用于单个 .model 根值之后的未读字节 / Unread bytes after a standalone .model root value only
}

func (m *Model) getMessagePackTrailing() []byte     { return m.TrailingData }
func (m *Model) setMessagePackTrailing(data []byte) { m.TrailingData = data }
func (m *Model) getMessagePackRootNil() bool        { return m.RootNil }
func (m *Model) setMessagePackRootNil(value bool)   { m.RootNil = value }

// TransData 表示骨骼变换数据 / TransData represents one bone transform entry
// 对应 C# Model.TransData，MessagePack indexed array / Matches C# Model.TransData as a MessagePack indexed array:
//
//	[Key(0)] name     string      骨骼名称
//	[Key(1)] paretnNo int         父骨骼索引（-1 表示根节点）
//	[Key(2)] isSCL    bool        是否为缩放骨骼
//	[Key(3)] pos      [x,y,z]     位置 (Vector3)
//	[Key(4)] rot      [x,y,z,w]   旋转 (Quaternion)
//	[Key(5)] scale    [x,y,z]     缩放 (Vector3)
type TransData struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Name                   string  `json:"name"`     // 骨骼名称 / Bone name
	ParentNo               int     `json:"paretnNo"` // 父骨骼索引，-1 表示根节点，字段名保留游戏 paretnNo 拼写 / Parent bone index, -1 means root, keeping the game's paretnNo spelling
	IsSCL                  bool    `json:"isSCL"`    // 是否为缩放骨骼 / Whether this is a scale bone
	Pos                    Vector3 `json:"pos"`      // 本地位置 / Local position
	Rot                    Vector4 `json:"rot"`      // 本地旋转四元数 / Local rotation quaternion
	Scale                  Vector3 `json:"scale"`    // 本地缩放 / Local scale
}

// ModelAssets 表示模型资源容器 / ModelAssets represents a model asset container
// 注意：游戏中 .model TextAsset 的 m_Script 直接包含单个 Model（不是容器）/ Note: game .model TextAsset m_Script stores one Model directly, not a container
// 使用 DecodeModel / EncodeModel 处理 / Use DecodeModel and EncodeModel for real .model payloads
// ModelAssets 仅用于可能存在的批量场景 / ModelAssets is kept only for possible batch scenarios
type ModelAssets struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	FileName               string  `json:"fileName"`                         // 容器文件名 / Container file name
	Assets                 []Model `json:"assetArray"`                       // 模型数组 / Model array
	RootNil                bool    `codec:"-" json:"rootNil,omitempty"`      // 根 MessagePack 值是否为 nil / Whether the root MessagePack value was nil
	TrailingData           []byte  `codec:"-" json:"trailingData,omitempty"` // 根 MessagePack 值之后游戏未读取的字节 / Bytes left unread after the root MessagePack value
}

func (a *ModelAssets) getMessagePackTrailing() []byte     { return a.TrailingData }
func (a *ModelAssets) setMessagePackTrailing(data []byte) { a.TrailingData = data }
func (a *ModelAssets) getMessagePackRootNil() bool        { return a.RootNil }
func (a *ModelAssets) setMessagePackRootNil(value bool)   { a.RootNil = value }

const modelFixVersion = 1001

func validateModelForEncoding(model *Model) error {
	return validateGameInt32Fields(model)
}

func validateDecodedModel(model *Model) error {
	return validateGameInt32Fields(model)
}

// DecodeModel 从 Lz4Block 压缩的 MessagePack 数据解码单个 Model。
// 这是 .model TextAsset m_Script 的正确解码方式。
// 游戏通过 PartsUtility.Deserialize<Model>(GameResource.LoadBinary(...)) 加载。
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

// EncodeModel 将单个 Model 编码为 Lz4Block 压缩的 MessagePack 数据。
// 生成的数据可直接作为 .model TextAsset 的 m_Script。
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

// DecodeModelAssets 从 Lz4Block 压缩的 MessagePack 数据解码 ModelAssets 容器
func DecodeModelAssets(data []byte) (*ModelAssets, error) {
	assets := &ModelAssets{}
	if err := decodeCompressedMsgpack(data, assets, "ModelAssets"); err != nil {
		return nil, err
	}
	if err := validateGameInt32Fields(assets); err != nil {
		return nil, fmt.Errorf("validate decoded ModelAssets integer field: %w", err)
	}
	for i := range assets.Assets {
		if err := validateDecodedModel(&assets.Assets[i]); err != nil {
			return nil, fmt.Errorf("validate decoded ModelAssets assetArray[%d]: %w", i, err)
		}
	}
	return assets, nil
}

// EncodeModelAssets 将 ModelAssets 编码为 Lz4Block 压缩的 MessagePack 数据
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

func NewModel() *Model {
	return &Model{Version: modelFixVersion}
}
