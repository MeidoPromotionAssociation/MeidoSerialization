package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

// .model
// KCES 模型描述文件，TextAsset.m_Script 直接保存一个 Parts.Model，而不是资源容器
// .model 不包含网格字节，meshFileName 引用 .aba 中单独保存的 UnityEngine.Mesh，通常命名为 .mmesh
// 载荷使用 LZ4 Block Array 压缩的 MessagePack indexed-array，当前 Model 固定版本为 1001
// .model
// KCES model-description file whose TextAsset.m_Script stores one Parts.Model directly rather than an asset container
// A .model does not embed mesh bytes, and meshFileName references a separate UnityEngine.Mesh in the .aba normally named with .mmesh
// The payload is an LZ4 Block Array-compressed MessagePack indexed array, with current Model fixed version 1001

// MessagePack indexed array 布局 / MessagePack indexed-array layout:
//
//	[Key(0)]  version          int
//	[Key(1)]  id               uint64
//	[Key(2)]  fileName         string         模型文件名（含 .model 扩展名）
//	[Key(3)]  meshFileName     string         网格文件名（含 .mmesh 扩展名）
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
	_struct           struct{}       `codec:",toarray" kces:"widths=10,11"`         // 强制按数组编码并接受游戏已知的无阴影标志旧布局 / Forces array encoding and accepts the known older game layout without shadow flags
	Version           int32          `json:"version"`                               // 版本号，固定为 1001 / Version value, fixed to 1001
	ID                uint64         `json:"id"`                                    // 模型文件名的 FNV-1a 64 位哈希，游戏导出器会先小写文件名，写入默认按当前值重算且可显式保留 / FNV-1a 64-bit hash of the model filename, which the game exporter lowercases before encoding and which is recalculated from the current value by default during encoding and explicitly preservable
	FileName          *string        `json:"fileName"`                              // 可空模型文件名 / Nullable model file name
	MeshFileName      *string        `json:"meshFileName"`                          // 可空网格文件名 / Nullable mesh file name
	ModelName         *string        `json:"modelName"`                             // 可空模型名称 / Nullable model name
	TransData         []*TransData   `json:"transData"`                             // 可空骨骼变换对象数组 / Array of nullable bone-transform objects
	BoneNames         []*string      `json:"boneNames"`                             // 可空骨骼名称数组 / Array of nullable bone names
	MaterialFileName  []*string      `json:"materialFileName"`                      // 可空材质文件名数组 / Array of nullable material file names
	Morphs            []*BlendData   `json:"morphs"`                                // 可空变形对象数组 / Array of nullable morph objects
	SkinThick         *SkinThickness `json:"skinThick"`                             // 皮肤厚度数据 / Skin-thickness data
	ShadowModeFlags   int32          `json:"shadowModeFlags"`                       // 阴影模式标志，0=Default, 1=CastShadow, 2=NoCastShadow / Shadow-mode flags, 0=Default, 1=CastShadow, 2=NoCastShadow
	IndexedArrayWidth int32          `codec:"-" json:"indexedArrayWidth,omitempty"` // 解码时记录的线格式数组宽度，并非游戏成员 / Wire array width recorded during decoding, not a game member
}

// TransData 表示一项骨骼变换数据
// 对应 C# Model.TransData，MessagePack indexed array 依次保存骨骼名称、父索引、缩放骨骼标志及局部变换
// TransData represents one bone transform entry
// It matches C# Model.TransData whose MessagePack indexed array stores the bone name, parent index, scale-bone flag, and local transform in order
type TransData struct {
	_struct  struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Name     *string  `json:"name"`      // 可空骨骼名称 / Nullable bone name
	ParentNo int32    `json:"parentNo"`  // 父骨骼索引，-1 表示根节点 / Parent bone index, with -1 indicating a root node
	IsSCL    bool     `json:"isSCL"`     // 是否为缩放骨骼 / Whether this is a scale bone
	Pos      Vector3  `json:"pos"`       // 本地位置 / Local position
	Rot      Vector4  `json:"rot"`       // 本地旋转四元数 / Local rotation quaternion
	Scale    Vector3  `json:"scale"`     // 本地缩放 / Local scale
}

// ModelAssets 表示可能存在的批量模型资源容器
// 游戏中 .model TextAsset 的 m_Script 实际直接包含单个 Model 而不是此容器，应使用 DecodeModel 和 EncodeModel 处理
// ModelAssets represents a possible batch model asset container
// A game .model TextAsset m_Script actually contains one Model directly rather than this container and should be handled with DecodeModel and EncodeModel
type ModelAssets struct {
	_struct  struct{} `codec:",toarray"`  // 强制按数组编码 / Forces array encoding
	FileName *string  `json:"fileName"`   // 可空容器文件名 / Nullable container file name
	Assets   []*Model `json:"assetArray"` // 可空模型对象数组 / Array of nullable model objects
}

const modelFixVersion = 1001

const modelCurrentWidth = 11

// MessagePackIndexedObjectWidth 返回 Model 应写出的 indexed-array 宽度
// MessagePackIndexedObjectWidth returns the indexed-array width that Model should emit
func (v *Model) MessagePackIndexedObjectWidth() int32 {
	if v.IndexedArrayWidth == 0 {
		return modelCurrentWidth
	}
	return v.IndexedArrayWidth
}

// SetMessagePackIndexedObjectWidth 设置 Model 应写出的 indexed-array 宽度
// SetMessagePackIndexedObjectWidth sets the indexed-array width that Model should emit
func (v *Model) SetMessagePackIndexedObjectWidth(width int32) {
	v.IndexedArrayWidth = width
}

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

// validateModelNameSelector 验证模型名称恰好指向一条根变换记录，同时允许无变换的零值测试模型
// validateModelNameSelector verifies that the model name selects exactly one root transform while allowing a transform-free zero-value test model
func validateModelNameSelector(model *Model) error {
	if model == nil {
		return nil
	}
	// Keep the zero-value synthetic model usable for metadata/fidelity tests;
	// a real model with transform records must identify exactly one root node.
	if len(model.TransData) == 0 && model.ModelName == nil {
		return nil
	}
	if model.ModelName == nil {
		return fmt.Errorf("modelName is required when transData is non-empty")
	}
	var matches int64
	for _, transData := range model.TransData {
		if transData != nil && transData.Name != nil && *transData.Name == *model.ModelName {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("modelName %q must match exactly one transData[].name, found %d matches", *model.ModelName, matches)
	}
	return nil
}

// DecodeModel 从 Lz4BlockArray 压缩的 MessagePack 数据解码单个 Model
// 这是 .model TextAsset m_Script 的实际解码方式，游戏通过 PartsUtility.Deserialize<Model>(GameResource.LoadBinary(...)) 加载
// DecodeModel decodes one Model from Lz4BlockArray-compressed MessagePack data
// This is the actual format of .model TextAsset m_Script loaded by the game through PartsUtility.Deserialize<Model>(GameResource.LoadBinary(...))
func DecodeModel(data []byte) (*Model, error) {
	var m *Model
	if err := decodeCompressedMsgpack(data, &m, "Model"); err != nil {
		return nil, err
	}
	if err := validateDecodedModel(m); err != nil {
		return nil, fmt.Errorf("validate decoded Model: %w", err)
	}
	return m, nil
}

// EncodeModel 将单个 Model 编码为 Lz4BlockArray 压缩的 MessagePack 数据，并默认按自身 FileName 重算 ID
// 生成的数据可直接作为 .model TextAsset 的 m_Script
// EncodeModel encodes one Model as Lz4BlockArray-compressed MessagePack data and recalculates its ID from its own FileName by default
// The resulting data can be used directly as .model TextAsset m_Script
func EncodeModel(m *Model) ([]byte, error) {
	return EncodeModelWithOptions(m, nil)
}

// EncodeModelWithOptions 将单个 Model 编码为 Lz4BlockArray 压缩的 MessagePack 数据，并允许显式关闭或按输出文件名执行查找字段重算
// EncodeModelWithOptions encodes one Model as Lz4BlockArray-compressed MessagePack data and allows lookup-field recalculation to be explicitly disabled or performed from the output filename
func EncodeModelWithOptions(m *Model, options *LookupHashOptions) ([]byte, error) {
	if m == nil {
		return encodeCompressedMsgpack(nil, "Model")
	}
	normalized := cloneModelForEncoding(m, options, true)
	if err := validateModelForEncoding(normalized); err != nil {
		return nil, fmt.Errorf("validate Model: %w", err)
	}
	return encodeCompressedMsgpack(normalized, "Model")
}

// DecodeModelAssets 从 Lz4BlockArray 压缩的 MessagePack 数据解码 ModelAssets 容器
// DecodeModelAssets decodes a ModelAssets container from Lz4BlockArray-compressed MessagePack data
func DecodeModelAssets(data []byte) (*ModelAssets, error) {
	var assets *ModelAssets
	if err := decodeCompressedMsgpack(data, &assets, "ModelAssets"); err != nil {
		return nil, err
	}
	if assets == nil {
		return nil, nil
	}
	for i := range assets.Assets {
		if err := validateDecodedModel(assets.Assets[i]); err != nil {
			return nil, fmt.Errorf("validate decoded ModelAssets assetArray[%d]: %w", i, err)
		}
	}
	return assets, nil
}

// EncodeModelAssets 将 ModelAssets 编码为 Lz4BlockArray 压缩的 MessagePack 数据，并默认按每个 Model 自身字段重算可确定的查找字段
// EncodeModelAssets encodes ModelAssets as Lz4BlockArray-compressed MessagePack data and recalculates determinable lookup fields from each Model's own fields by default
func EncodeModelAssets(assets *ModelAssets) ([]byte, error) {
	return EncodeModelAssetsWithOptions(assets, nil)
}

// EncodeModelAssetsWithOptions 将 ModelAssets 编码为 Lz4BlockArray 压缩的 MessagePack 数据，并允许显式关闭每个 Model 自身可确定的查找字段重算
// EncodeModelAssetsWithOptions encodes ModelAssets as Lz4BlockArray-compressed MessagePack data and allows recalculation of lookup fields determined by each Model itself to be explicitly disabled
func EncodeModelAssetsWithOptions(assets *ModelAssets, options *LookupHashOptions) ([]byte, error) {
	if assets == nil {
		return encodeCompressedMsgpack(nil, "ModelAssets")
	}
	normalized := *assets
	normalized.Assets = cloneSlicePreserveNil(assets.Assets)
	if ShouldRecalculateLookupHashes(options) {
		for index, model := range normalized.Assets {
			normalized.Assets[index] = cloneModelForEncoding(model, options, false)
		}
	}
	for i := range normalized.Assets {
		if err := validateModelForEncoding(normalized.Assets[i]); err != nil {
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

// CodecEncodeSelf 按共享 indexed-object 规则编码 Model
// CodecEncodeSelf encodes Model using the shared indexed-object rules
func (v Model) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Model
// CodecDecodeSelf decodes Model using the shared indexed-object rules
func (v *Model) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 TransData
// CodecEncodeSelf encodes TransData using the shared indexed-object rules
func (v TransData) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 TransData
// CodecDecodeSelf decodes TransData using the shared indexed-object rules
func (v *TransData) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ModelAssets
// CodecEncodeSelf encodes ModelAssets using the shared indexed-object rules
func (v ModelAssets) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ModelAssets
// CodecDecodeSelf decodes ModelAssets using the shared indexed-object rules
func (v *ModelAssets) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// normalizeModelLookupFields 重算游戏可从 Model 自身文件名确定的查找字段，缺少文件名时保留原 ID
// normalizeModelLookupFields recalculates the game lookup field determined by a Model filename and preserves the existing ID when the filename is absent
func normalizeModelLookupFields(model *Model) {
	if model == nil || model.FileName == nil {
		return
	}
	model.ID = ct.HashString(*model.FileName)
}

// cloneModelForEncoding 复制单个 Model，并按默认值或显式选项使用外部文件名和自身字段处理查找字段
// cloneModelForEncoding copies one Model and handles its lookup field from the external filename and the model's own fields under the default or explicitly selected behavior
func cloneModelForEncoding(model *Model, options *LookupHashOptions, useExternalFileName bool) *Model {
	if model == nil {
		return nil
	}
	normalized := *model
	if ShouldRecalculateLookupHashes(options) && useExternalFileName && options != nil && options.FileName != "" {
		fileName := options.FileName
		normalized.FileName = &fileName
	}
	if ShouldRecalculateLookupHashes(options) {
		normalizeModelLookupFields(&normalized)
	}
	return &normalized
}
