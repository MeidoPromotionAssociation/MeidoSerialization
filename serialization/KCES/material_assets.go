package KCES

// Material 表示 KCES 材质数据 / Material represents KCES material data
// 对应 C# Parts.Material，继承自 AMessagePackSerializationVersionControlIntKey / It maps C# Parts.Material, derived from AMessagePackSerializationVersionControlIntKey
// MessagePack indexed array 布局：/ MessagePack indexed-array layout:
//
//	[Key(0)] version       int            固定 1000 (FixVersion)
//	[Key(1)] id            uint64         材质 ID
//	[Key(2)] fileName      string         材质文件名（含 .mate 扩展名）
//	[Key(3)] shaderName    string         Shader 名称（如 "CM3D2/Toony_Lighted"）
//	[Key(4)] textureProps  TextureProp[]  纹理属性数组
//	[Key(5)] colorProps    ColorProp[]    颜色属性数组
//	[Key(6)] vectorProps   VectorProp[]   向量属性数组
//	[Key(7)] floatProps    FloatProp[]    浮点属性数组
type Material struct {
	_struct      struct{}      `codec:",toarray"`    // 强制按数组编码 / Forces array encoding
	Version      int           `json:"version"`      // 版本号，固定为 1000 / Version value, fixed to 1000
	ID           uint64        `json:"id"`           // 材质 ID，通常为 fileName 去扩展名后小写的 FNV hash / Material ID, usually lowercase extensionless fileName FNV hash
	FileName     string        `json:"fileName"`     // 材质文件名，如 "xxx.mate" / Material file name, for example "xxx.mate"
	ShaderName   string        `json:"shaderName"`   // Unity shader 名称 / Unity shader name
	TextureProps []TextureProp `json:"textureProps"` // 纹理属性数组 / Texture property array
	ColorProps   []ColorProp   `json:"colorProps"`   // 颜色属性数组 / Color property array
	VectorProps  []VectorProp  `json:"vectorProps"`  // 向量属性数组 / Vector property array
	FloatProps   []FloatProp   `json:"floatProps"`   // 浮点属性数组 / Float property array
}

// TextureProp 表示材质的纹理属性 / TextureProp represents a material texture property
// MessagePack indexed array: [type, fileName, ox, oy, sx, sy]
type TextureProp struct {
	_struct  struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type     int      `json:"type"`      // 属性类型枚举值，如 0=_MainTex, 1=_BumpMap / Property type enum, e.g. 0=_MainTex, 1=_BumpMap
	FileName string   `json:"fileName"`  // 纹理文件名 / Texture file name
	Ox       float32  `json:"ox"`        // 纹理偏移 X / Texture offset X
	Oy       float32  `json:"oy"`        // 纹理偏移 Y / Texture offset Y
	Sx       float32  `json:"sx"`        // 纹理缩放 X / Texture scale X
	Sy       float32  `json:"sy"`        // 纹理缩放 Y / Texture scale Y
}

// ColorProp 表示材质的颜色属性 / ColorProp represents a material color property
// MessagePack indexed array: [type, r, g, b, a]
type ColorProp struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int      `json:"type"`      // 属性类型枚举值，如 100=_Color, 101=_ShadowColor / Property type enum, e.g. 100=_Color, 101=_ShadowColor
	R       float32  `json:"r"`         // 红色分量 (0.0~1.0) / Red channel (0.0 to 1.0)
	G       float32  `json:"g"`         // 绿色分量 / Green channel
	B       float32  `json:"b"`         // 蓝色分量 / Blue channel
	A       float32  `json:"a"`         // 透明度分量 / Alpha channel
}

// VectorProp 表示材质的向量属性 / VectorProp represents a material vector property
// MessagePack indexed array: [type, x, y, z, w]
type VectorProp struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int      `json:"type"`      // 属性类型枚举值 / Property type enum value
	X       float32  `json:"x"`         // 向量 X 分量 / Vector X component
	Y       float32  `json:"y"`         // 向量 Y 分量 / Vector Y component
	Z       float32  `json:"z"`         // 向量 Z 分量 / Vector Z component
	W       float32  `json:"w"`         // 向量 W 分量 / Vector W component
}

// FloatProp 表示材质的浮点属性 / FloatProp represents a material float property
// MessagePack indexed array: [type, v]
type FloatProp struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int      `json:"type"`      // 属性类型枚举值，如 200=_Shininess, 202=_OutlineWidth / Property type enum, e.g. 200=_Shininess, 202=_OutlineWidth
	V       float32  `json:"v"`         // 浮点值 / Float value
}

// MaterialAssets 表示材质资源容器 / MaterialAssets represents a material asset container
// 对应 C# Parts.MaterialAssets，继承自 SerializPartsAssets<Material> / It maps C# Parts.MaterialAssets, derived from SerializPartsAssets<Material>
// MessagePack indexed array: [fileName, assetArray]
type MaterialAssets struct {
	_struct  struct{}   `codec:",toarray"`  // 强制按数组编码 / Forces array encoding
	FileName string     `json:"fileName"`   // 容器文件名，如 "xxx.materialassets" / Container file name, for example "xxx.materialassets"
	Assets   []Material `json:"assetArray"` // 材质数组，对应 C# assetArray / Material array, matching C# assetArray
}

const materialFixVersion = 1000

// DecodeMaterialAssets 从 Lz4Block 压缩的 MessagePack 数据解码 MaterialAssets
func DecodeMaterialAssets(data []byte) (*MaterialAssets, error) {
	assets := &MaterialAssets{}
	if err := decodeCompressedMsgpack(data, assets, "MaterialAssets"); err != nil {
		return nil, err
	}
	return assets, nil
}

// EncodeMaterialAssets 将 MaterialAssets 编码为 Lz4Block 压缩的 MessagePack 数据
func EncodeMaterialAssets(assets *MaterialAssets) ([]byte, error) {
	normalized := *assets
	normalized.Assets = append([]Material(nil), assets.Assets...)
	for i := range normalized.Assets {
		if normalized.Assets[i].Version == 0 {
			normalized.Assets[i].Version = materialFixVersion
		}
	}
	return encodeCompressedMsgpack(&normalized, "MaterialAssets")
}
