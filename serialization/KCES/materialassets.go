package KCES

import "fmt"

// .materialassets
// KCES 材质资源容器，在 .aba 的 TextAsset 中保存 Parts.Material 数组及其材质属性
// 载荷使用 LZ4 Block Array 压缩的 MessagePack indexed-array，当前 Material 固定版本为 1000
//
// .materialassets
// KCES material-resource container storing Parts.Material entries and their properties in a TextAsset inside an .aba file
// The payload is an LZ4 Block Array-compressed MessagePack indexed array, with current Material fixed version 1000

// Material 表示 KCES 材质数据
// 对应 C# Parts.Material，继承自 AMessagePackSerializationVersionControlIntKey
// MessagePack indexed array 依次保存版本、ID、文件名、着色器名和四类属性数组
//
// Material represents KCES material data
// It matches C# Parts.Material derived from AMessagePackSerializationVersionControlIntKey
// Its MessagePack indexed array stores the version, ID, filename, shader name, and four property arrays in order
type Material struct {
	_struct                struct{}      `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`   // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int           `json:"version"`      // 存储的版本；当前游戏 FixVersion 为 1000 / Stored version; current-game FixVersion is 1000
	ID                     uint64        `json:"id"`           // 材质 ID，通常为 fileName 去扩展名后小写的 FNV hash / Material ID, usually lowercase extensionless fileName FNV hash
	FileName               string        `json:"fileName"`     // 材质文件名，如 "xxx.mate" / Material file name, for example "xxx.mate"
	ShaderName             string        `json:"shaderName"`   // Unity shader 名称 / Unity shader name
	TextureProps           []TextureProp `json:"textureProps"` // 纹理属性数组 / Texture property array
	ColorProps             []ColorProp   `json:"colorProps"`   // 颜色属性数组 / Color property array
	VectorProps            []VectorProp  `json:"vectorProps"`  // 向量属性数组 / Vector property array
	FloatProps             []FloatProp   `json:"floatProps"`   // 浮点属性数组 / Float property array
}

// TextureProp 表示材质的纹理属性，MessagePack indexed array 依次保存类型、文件名、偏移和缩放
// TextureProp represents a material texture property whose MessagePack indexed array stores type, filename, offset, and scale in order
type TextureProp struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	Type                   int         `json:"type"`     // 属性类型枚举值，如 0=_MainTex, 1=_BumpMap / Property type enum, e.g. 0=_MainTex, 1=_BumpMap
	FileName               string      `json:"fileName"` // 纹理文件名 / Texture file name
	Ox                     float32     `json:"ox"`       // 纹理偏移 X / Texture offset X
	Oy                     float32     `json:"oy"`       // 纹理偏移 Y / Texture offset Y
	Sx                     float32     `json:"sx"`       // 纹理缩放 X / Texture scale X
	Sy                     float32     `json:"sy"`       // 纹理缩放 Y / Texture scale Y
}

// ColorProp 表示材质的颜色属性，MessagePack indexed array 依次保存类型与 RGBA 分量
// ColorProp represents a material color property whose MessagePack indexed array stores the type and RGBA components in order
type ColorProp struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	Type                   int         `json:"type"` // 属性类型枚举值，如 100=_Color, 101=_ShadowColor / Property type enum, e.g. 100=_Color, 101=_ShadowColor
	R                      float32     `json:"r"`    // 红色分量 (0.0~1.0) / Red channel (0.0 to 1.0)
	G                      float32     `json:"g"`    // 绿色分量 / Green channel
	B                      float32     `json:"b"`    // 蓝色分量 / Blue channel
	A                      float32     `json:"a"`    // 透明度分量 / Alpha channel
}

// VectorProp 表示材质的向量属性，MessagePack indexed array 依次保存类型与 XYZW 分量
// VectorProp represents a material vector property whose MessagePack indexed array stores the type and XYZW components in order
type VectorProp struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	Type                   int         `json:"type"` // 属性类型枚举值 / Property type enum value
	X                      float32     `json:"x"`    // 向量 X 分量 / Vector X component
	Y                      float32     `json:"y"`    // 向量 Y 分量 / Vector Y component
	Z                      float32     `json:"z"`    // 向量 Z 分量 / Vector Z component
	W                      float32     `json:"w"`    // 向量 W 分量 / Vector W component
}

// FloatProp 表示材质的浮点属性，MessagePack indexed array 依次保存类型和值
// FloatProp represents a material float property whose MessagePack indexed array stores the type and value in order
type FloatProp struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	Type                   int         `json:"type"` // 属性类型枚举值，如 200=_Shininess, 202=_OutlineWidth / Property type enum, e.g. 200=_Shininess, 202=_OutlineWidth
	V                      float32     `json:"v"`    // 浮点值 / Float value
}

// MaterialAssets 表示材质资源容器
// 对应 C# Parts.MaterialAssets，继承自 SerializPartsAssets<Material>
// MessagePack indexed array 依次保存容器文件名和材质数组
// MaterialAssets represents a material asset container
// It matches C# Parts.MaterialAssets derived from SerializPartsAssets<Material>
// Its MessagePack indexed array stores the container filename followed by the material array
type MaterialAssets struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	FileName               string      `json:"fileName"`                         // 容器文件名，如 "xxx.materialassets" / Container file name, for example "xxx.materialassets"
	Assets                 []Material  `json:"assetArray"`                       // 材质数组，对应 C# assetArray / Material array, matching C# assetArray
	RootNil                bool        `codec:"-" json:"rootNil,omitempty"`      // 根 MessagePack 值是否为 nil / Whether the root MessagePack value was nil
	TrailingData           []byte      `codec:"-" json:"trailingData,omitempty"` // 根 MessagePack 值之后游戏未读取的字节 / Bytes left unread after the root MessagePack value
}

// getMessagePackTrailing 返回根 MessagePack 值后的保留字节
// getMessagePackTrailing returns the preserved bytes after the root MessagePack value
func (a *MaterialAssets) getMessagePackTrailing() []byte { return a.TrailingData }

// setMessagePackTrailing 设置根 MessagePack 值后的保留字节
// setMessagePackTrailing sets the preserved bytes after the root MessagePack value
func (a *MaterialAssets) setMessagePackTrailing(data []byte) { a.TrailingData = data }

// getMessagePackRootNil 返回根 MessagePack 值是否为 nil
// getMessagePackRootNil reports whether the root MessagePack value was nil
func (a *MaterialAssets) getMessagePackRootNil() bool { return a.RootNil }

// setMessagePackRootNil 设置根 MessagePack 值的 nil 标记
// setMessagePackRootNil sets the nil marker for the root MessagePack value
func (a *MaterialAssets) setMessagePackRootNil(value bool) { a.RootNil = value }

const materialFixVersion = 1000

// DecodeMaterialAssets 从 Lz4BlockArray 压缩的 MessagePack 数据解码 MaterialAssets
// DecodeMaterialAssets decodes MaterialAssets from Lz4BlockArray-compressed MessagePack data
func DecodeMaterialAssets(data []byte) (*MaterialAssets, error) {
	assets := &MaterialAssets{}
	if err := decodeCompressedMsgpack(data, assets, "MaterialAssets"); err != nil {
		return nil, err
	}
	if err := validateGameInt32Fields(assets); err != nil {
		return nil, fmt.Errorf("decode MaterialAssets integer field: %w", err)
	}
	return assets, nil
}

// EncodeMaterialAssets 将 MaterialAssets 编码为 Lz4BlockArray 压缩的 MessagePack 数据
// EncodeMaterialAssets encodes MaterialAssets as Lz4BlockArray-compressed MessagePack data
func EncodeMaterialAssets(assets *MaterialAssets) ([]byte, error) {
	if assets == nil {
		return nil, fmt.Errorf("MaterialAssets is nil")
	}
	normalized := *assets
	normalized.Assets = cloneSlicePreserveNil(assets.Assets)
	if err := validateGameInt32Fields(&normalized); err != nil {
		return nil, fmt.Errorf("encode MaterialAssets integer field: %w", err)
	}
	return encodeCompressedMsgpack(&normalized, "MaterialAssets")
}

// NewMaterial 创建使用当前固定版本的新材质
// NewMaterial creates a new material with the current fixed version
func NewMaterial() *Material {
	return &Material{Version: materialFixVersion}
}
