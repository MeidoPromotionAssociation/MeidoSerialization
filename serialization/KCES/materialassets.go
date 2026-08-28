package KCES

import (
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

// .materialassets
// KCES 材质资源容器，在 .aba 的 TextAsset 中保存 Parts.Material 数组及其材质属性
// 载荷使用 LZ4 Block Array 压缩的 MessagePack indexed-array，当前 Material 固定版本为 1000
//
// .materialassets
// KCES material-resource container storing Parts.Material entries and their properties in a TextAsset inside an .aba file
// The payload is an LZ4 Block Array-compressed MessagePack indexed array, with current Material fixed version 1000

// Material 表示 KCES 材质数据
// 对应 C# Parts.Material，继承自 AMessagePackSerializationVersionControlIntKey
// MessagePack indexed array 在 KCES 中保存版本、ID、文件名、着色器名和四类属性数组，KCES2 追加 keyword 与渲染队列
//
// Material represents KCES material data
// It matches C# Parts.Material derived from AMessagePackSerializationVersionControlIntKey
// Its MessagePack indexed array stores the version, ID, filename, shader name, and four property arrays in KCES, then appends keywords and the render queue in KCES2
type Material struct {
	_struct           struct{}       `codec:",toarray" kces:"widths=8,10"`          // 强制按数组编码并接受 KCES 与 KCES2 布局 / Forces array encoding and accepts KCES and KCES2 layouts
	Version           int32          `json:"version"`                               // 存储的版本；当前游戏 FixVersion 为 1000 / Stored version; current-game FixVersion is 1000
	ID                uint64         `json:"id"`                                    // 材质文件名的 FNV-1a 64 位哈希，写入默认按当前大小写重算且可显式保留 / FNV-1a 64-bit hash of the material filename, recalculated with its current casing by default during encoding and explicitly preservable
	FileName          *string        `json:"fileName"`                              // 可空材质文件名，如 "xxx.mate" / Nullable material file name, for example "xxx.mate"
	ShaderName        *string        `json:"shaderName"`                            // 可空 Unity shader 名称 / Nullable Unity shader name
	TextureProps      []*TextureProp `json:"textureProps"`                          // 可空纹理属性对象数组 / Array of nullable texture-property objects
	ColorProps        []*ColorProp   `json:"colorProps"`                            // 可空颜色属性对象数组 / Array of nullable color-property objects
	VectorProps       []*VectorProp  `json:"vectorProps"`                           // 可空向量属性对象数组 / Array of nullable vector-property objects
	FloatProps        []*FloatProp   `json:"floatProps"`                            // 可空浮点属性对象数组 / Array of nullable float-property objects
	KeywordProps      []*KeywordProp `json:"keywordProps"`                          // 可空着色器关键字属性对象数组 / Array of nullable shader-keyword property objects
	RenderQueue       int32          `json:"renderQueue"`                           // Unity 渲染队列 / Unity render queue
	IndexedArrayWidth int32          `codec:"-" json:"indexedArrayWidth,omitempty"` // 解码时记录的线格式数组宽度，并非游戏成员 / Wire array width recorded during decoding, not a game member
}

// TextureProp 表示材质的纹理属性，MessagePack indexed array 依次保存类型、文件名、偏移和缩放
// TextureProp represents a material texture property whose MessagePack indexed array stores type, filename, offset, and scale in order
type TextureProp struct {
	_struct  struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type     int32    `json:"type"`      // 属性类型枚举值，如 0=_MainTex, 1=_BumpMap / Property type enum, e.g. 0=_MainTex, 1=_BumpMap
	FileName *string  `json:"fileName"`  // 可空纹理文件名 / Nullable texture file name
	Ox       float32  `json:"ox"`        // 纹理偏移 X / Texture offset X
	Oy       float32  `json:"oy"`        // 纹理偏移 Y / Texture offset Y
	Sx       float32  `json:"sx"`        // 纹理缩放 X / Texture scale X
	Sy       float32  `json:"sy"`        // 纹理缩放 Y / Texture scale Y
}

// ColorProp 表示材质的颜色属性，MessagePack indexed array 依次保存类型与 RGBA 分量
// ColorProp represents a material color property whose MessagePack indexed array stores the type and RGBA components in order
type ColorProp struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int32    `json:"type"`      // 属性类型枚举值，如 100=_Color, 101=_ShadowColor / Property type enum, e.g. 100=_Color, 101=_ShadowColor
	R       float32  `json:"r"`         // 红色分量 (0.0~1.0) / Red channel (0.0 to 1.0)
	G       float32  `json:"g"`         // 绿色分量 / Green channel
	B       float32  `json:"b"`         // 蓝色分量 / Blue channel
	A       float32  `json:"a"`         // 透明度分量 / Alpha channel
}

// VectorProp 表示材质的向量属性，MessagePack indexed array 依次保存类型与 XYZW 分量
// VectorProp represents a material vector property whose MessagePack indexed array stores the type and XYZW components in order
type VectorProp struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int32    `json:"type"`      // 属性类型枚举值 / Property type enum value
	X       float32  `json:"x"`         // 向量 X 分量 / Vector X component
	Y       float32  `json:"y"`         // 向量 Y 分量 / Vector Y component
	Z       float32  `json:"z"`         // 向量 Z 分量 / Vector Z component
	W       float32  `json:"w"`         // 向量 W 分量 / Vector W component
}

// FloatProp 表示材质的浮点属性，MessagePack indexed array 依次保存类型和值
// FloatProp represents a material float property whose MessagePack indexed array stores the type and value in order
type FloatProp struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int32    `json:"type"`      // 属性类型枚举值，如 200=_Shininess, 202=_OutlineWidth / Property type enum, e.g. 200=_Shininess, 202=_OutlineWidth
	V       float32  `json:"v"`         // 浮点值 / Float value
}

// KeywordProp 表示材质的 Shader keyword 开关 / KeywordProp represents a material Shader keyword switch
type KeywordProp struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int32    `json:"type"`      // 属性类型枚举值 / Property type enum value
	Value   bool     `json:"value"`     // 是否启用 / Whether the keyword is enabled
}

// MaterialAssets 表示材质资源容器
// 对应 C# Parts.MaterialAssets，继承自 SerializPartsAssets<Material>
// MessagePack indexed array 依次保存容器文件名和材质数组
// MaterialAssets represents a material asset container
// It matches C# Parts.MaterialAssets derived from SerializPartsAssets<Material>
// Its MessagePack indexed array stores the container filename followed by the material array
type MaterialAssets struct {
	_struct  struct{}    `codec:",toarray"`  // 强制按数组编码 / Forces array encoding
	FileName *string     `json:"fileName"`   // 可空容器文件名，如 "xxx.materialassets" / Nullable container file name, for example "xxx.materialassets"
	Assets   []*Material `json:"assetArray"` // 可空材质对象数组，对应 C# assetArray / Array of nullable material objects matching C# assetArray
}

const materialFixVersion = 1000

// MaterialExtension 是容器内材质条目虚拟文件名使用的扩展名
// KCES 没有独立的材质文件，该扩展名只出现在 .materialassets 条目的 fileName 中，游戏按它补全无扩展名的查找名
// MaterialExtension is the extension used by the virtual filenames of material entries inside a container
// KCES has no standalone material files, so this extension appears only in the fileName of .materialassets entries, where the game uses it to complete extensionless lookup names
const MaterialExtension = ".mate"

const (
	materialLegacyWidth = 8
	materialKCES2Width  = 10
)

// MessagePackIndexedObjectWidth 返回 Material 应写出的 indexed-array 宽度
// MessagePackIndexedObjectWidth returns the indexed-array width that Material should emit
func (v *Material) MessagePackIndexedObjectWidth() int32 {
	if v.IndexedArrayWidth == 0 {
		return materialLegacyWidth
	}
	return v.IndexedArrayWidth
}

// SetMessagePackIndexedObjectWidth 设置 Material 应写出的 indexed-array 宽度
// SetMessagePackIndexedObjectWidth sets the indexed-array width that Material should emit
func (v *Material) SetMessagePackIndexedObjectWidth(width int32) {
	v.IndexedArrayWidth = width
}

// DecodeMaterialAssets 从 Lz4BlockArray 压缩的 MessagePack 数据解码 MaterialAssets
// DecodeMaterialAssets decodes MaterialAssets from Lz4BlockArray-compressed MessagePack data
func DecodeMaterialAssets(data []byte) (*MaterialAssets, error) {
	var assets *MaterialAssets
	if err := decodeCompressedMsgpack(data, &assets, "MaterialAssets"); err != nil {
		return nil, err
	}
	return assets, nil
}

// EncodeMaterialAssets 将 MaterialAssets 编码为 Lz4BlockArray 压缩的 MessagePack 数据，并默认按每个 Material 自身字段重算可确定的查找字段
// EncodeMaterialAssets encodes MaterialAssets as Lz4BlockArray-compressed MessagePack data and recalculates determinable lookup fields from each Material's own fields by default
func EncodeMaterialAssets(assets *MaterialAssets) ([]byte, error) {
	return EncodeMaterialAssetsWithOptions(assets, nil)
}

// EncodeMaterialAssetsWithOptions 将 MaterialAssets 编码为 Lz4BlockArray 压缩的 MessagePack 数据，并允许显式关闭每个 Material 自身可确定的查找字段重算
// EncodeMaterialAssetsWithOptions encodes MaterialAssets as Lz4BlockArray-compressed MessagePack data and allows recalculation of lookup fields determined by each Material itself to be explicitly disabled
func EncodeMaterialAssetsWithOptions(assets *MaterialAssets, options *LookupHashOptions) ([]byte, error) {
	if assets == nil {
		return encodeCompressedMsgpack(nil, "MaterialAssets")
	}
	normalized := *assets
	normalized.Assets = cloneSlicePreserveNil(assets.Assets)
	if ShouldRecalculateLookupHashes(options) {
		for index, material := range normalized.Assets {
			normalized.Assets[index] = cloneMaterialForEncoding(material, options, false)
		}
	}
	return encodeCompressedMsgpack(&normalized, "MaterialAssets")
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 Material
// CodecEncodeSelf encodes Material using the shared indexed-object rules
func (v Material) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Material
// CodecDecodeSelf decodes Material using the shared indexed-object rules
func (v *Material) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 TextureProp
// CodecEncodeSelf encodes TextureProp using the shared indexed-object rules
func (v TextureProp) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 TextureProp
// CodecDecodeSelf decodes TextureProp using the shared indexed-object rules
func (v *TextureProp) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColorProp
// CodecEncodeSelf encodes ColorProp using the shared indexed-object rules
func (v ColorProp) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColorProp
// CodecDecodeSelf decodes ColorProp using the shared indexed-object rules
func (v *ColorProp) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 VectorProp
// CodecEncodeSelf encodes VectorProp using the shared indexed-object rules
func (v VectorProp) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 VectorProp
// CodecDecodeSelf decodes VectorProp using the shared indexed-object rules
func (v *VectorProp) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 FloatProp
// CodecEncodeSelf encodes FloatProp using the shared indexed-object rules
func (v FloatProp) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 FloatProp
// CodecDecodeSelf decodes FloatProp using the shared indexed-object rules
func (v *FloatProp) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 KeywordProp
// CodecEncodeSelf encodes KeywordProp using the shared indexed-object rules
func (v KeywordProp) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 KeywordProp
// CodecDecodeSelf decodes KeywordProp using the shared indexed-object rules
func (v *KeywordProp) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 MaterialAssets
// CodecEncodeSelf encodes MaterialAssets using the shared indexed-object rules
func (v MaterialAssets) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 MaterialAssets
// CodecDecodeSelf decodes MaterialAssets using the shared indexed-object rules
func (v *MaterialAssets) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// NewMaterial 创建使用当前固定版本的新材质
// NewMaterial creates a new material with the current fixed version
func NewMaterial() *Material {
	return &Material{Version: materialFixVersion}
}

// NewKCES2Material 创建使用 KCES2 10 槽布局的新材质
// NewKCES2Material creates a new material using the 10-slot KCES2 layout
func NewKCES2Material() *Material {
	material := NewMaterial()
	material.SetMessagePackIndexedObjectWidth(materialKCES2Width)
	return material
}

// normalizeMaterialLookupFields 重算游戏可从 Material 自身文件名确定的查找字段，缺少文件名时保留原 ID
// normalizeMaterialLookupFields recalculates the game lookup field determined by a Material filename and preserves the existing ID when the filename is absent
func normalizeMaterialLookupFields(material *Material) {
	if material == nil || material.FileName == nil {
		return
	}
	material.ID = ct.HashString(*material.FileName)
}

// cloneMaterialForEncoding 复制单个 Material，并按默认值或显式选项使用外部文件名和自身字段处理查找字段
// cloneMaterialForEncoding copies one Material and handles its lookup field from the external filename and the material's own fields under the default or explicitly selected behavior
func cloneMaterialForEncoding(material *Material, options *LookupHashOptions, useExternalFileName bool) *Material {
	if material == nil {
		return nil
	}
	normalized := *material
	if ShouldRecalculateLookupHashes(options) && useExternalFileName && options != nil && options.FileName != "" {
		fileName := options.FileName
		normalized.FileName = &fileName
	}
	if ShouldRecalculateLookupHashes(options) {
		normalizeMaterialLookupFields(&normalized)
	}
	return &normalized
}
