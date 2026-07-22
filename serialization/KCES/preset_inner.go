package KCES

import (
	"bytes"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// .preset 的 maiddata 内部 BinaryWriter 模型，定义属性列表、颜色数据和身体数据的签名、版本及结构
// BinaryWriter models inside .preset maiddata, defining signatures, versions, and structures for property, color, and body data

const (
	KCESPresetPropertyListSignature       = "GP03_MPROP_LIST"
	KCESPresetPropertyListVersion   int32 = 1270
	KCESPresetPropertySignature           = "GP03_MPROP"
	KCESPresetPropertyVersion       int32 = 2100
	KCESPresetColorSignature              = "CM3D2_MULTI_COL"
	KCESPresetColorVersion          int32 = 1270
	KCESPresetBodySignature               = "CM3D2_MAID_BODY"
	KCESPresetBodyVersion           int32 = 1270

	maxKCESPresetInnerDepth = 64
)

// KCESPresetPropertyList 表示 MaidPresetCore.propData 中的 GP03_MPROP_LIST 块
// 属性使用切片而非映射，以便在 JSON 往返期间保留游戏字典的枚举顺序以及线格式中的重复键
// KCESPresetPropertyList represents the GP03_MPROP_LIST block stored in MaidPresetCore.propData
// Properties use a slice instead of a map so dictionary enumeration order and repeated wire keys survive a JSON round trip
type KCESPresetPropertyList struct {
	Signature    string                    `json:"signature"`              // 属性列表签名，当前游戏写入 GP03_MPROP_LIST / Property-list signature, written as GP03_MPROP_LIST by the current game
	Version      int32                     `json:"version"`                // 属性列表版本，当前游戏写入 1270 / Property-list version, written as 1270 by the current game
	Properties   []KCESPresetNamedProperty `json:"properties"`             // 按游戏字典枚举顺序保存的属性项 / Property entries in game dictionary enumeration order
	TrailingData []byte                    `json:"trailingData,omitempty"` // 当前已知属性列表结构之后的未解析字节 / Unparsed bytes following the currently known property-list layout
}

// KCESPresetNamedProperty 表示属性字典中单独保存键名与 MaidProp 值的一项
// KCESPresetNamedProperty represents one property-dictionary entry with its separately stored key and MaidProp value
type KCESPresetNamedProperty struct {
	Key      string             `json:"key"`      // Maid 属性字典中的键 / Key in the Maid property dictionary
	Property KCESPresetProperty `json:"property"` // 键对应的 MaidProp 二进制块 / MaidProp binary block associated with the key
}

// KCESPresetProperty 表示 MaidProp.Serialize 写出的当前 2100 版字段以及随后继承的 PropBase 二进制块
// KCESPresetProperty represents the current v2100 fields written by MaidProp.Serialize followed by the inherited PropBase binary block
type KCESPresetProperty struct {
	Signature          string                           `json:"signature"`          // MaidProp 签名，当前游戏写入 GP03_MPROP / MaidProp signature, written as GP03_MPROP by the current game
	Version            int32                            `json:"version"`            // MaidProp 版本，当前游戏写入 2100 / MaidProp version, written as 2100 by the current game
	Name               string                           `json:"name"`               // 属性名称，PropBase 反序列化时也用它解析 MPN / Property name, also used to resolve the MPN during PropBase deserialization
	DefaultValue       int32                            `json:"defaultValue"`       // 属性默认值，对应 MaidProp.def / Property default value corresponding to MaidProp.def
	Value              int32                            `json:"value"`              // 属性当前值，对应 MaidProp.value / Current property value corresponding to MaidProp.value
	TempValue          int32                            `json:"tempValue"`          // 属性临时值，对应 MaidProp.temp / Temporary property value corresponding to MaidProp.temp
	FileNameRID        uint64                           `json:"fileNameRid"`        // MaidProp 层保存的菜单文件 RID / Menu-file RID stored by the MaidProp layer
	Enabled            bool                             `json:"enabled"`            // MaidProp 层的待处理标志 bDut / MaidProp processing-pending flag bDut
	Max                int32                            `json:"max"`                // 属性允许的最大值 / Maximum allowed property value
	Min                int32                            `json:"min"`                // 属性允许的最小值 / Minimum allowed property value
	MaterialProperties []KCESPresetMaterialPropertySlot `json:"materialProperties"` // 按 SlotPair 分组的材质属性覆盖 / Material-property overrides grouped by SlotPair
	Base               KCESPresetPropBase               `json:"base"`               // 随后的 PropBase 继承字段 / Following inherited PropBase fields
}

// KCESPresetMaterialPropertySlot 表示 MaidProp.m_dicMaterialProp 中一个 SlotPair 分组
// KCESPresetMaterialPropertySlot represents one SlotPair group in MaidProp.m_dicMaterialProp
type KCESPresetMaterialPropertySlot struct {
	SlotID     string                            `json:"slotId"`     // TBody.SlotID 的字符串名称 / String name of TBody.SlotID
	SlotValue  int32                             `json:"slotValue"`  // SlotPair 的整数值，仅从属性版本 2002 起在线格式中存在 / Integer value of SlotPair, present on the wire only since property version 2002
	Properties []KCESPresetNamedMaterialProperty `json:"properties"` // 此槽位分组内按字典顺序保存的材质属性 / Material properties in this slot group in dictionary order
}

// KCESPresetNamedMaterialProperty 表示材质属性字典中带 RID 的命名项
// KCESPresetNamedMaterialProperty represents a named material-property dictionary entry with its RID
type KCESPresetNamedMaterialProperty struct {
	Key      string                          `json:"key"`      // 材质属性字典键 / Material-property dictionary key
	RID      uint64                          `json:"rid"`      // 与该覆盖值一同保存的资源 RID / Resource RID stored with this override
	Property KCESPresetMaterialPropertyValue `json:"property"` // MatPropSave 保存的属性值 / Property value stored by MatPropSave
}

// KCESPresetMaterialPropertyValue 表示 MatPropSave.Serialize 写出的材质覆盖值
// KCESPresetMaterialPropertyValue represents a material override written by MatPropSave.Serialize
type KCESPresetMaterialPropertyValue struct {
	MaterialNumber int32   `json:"materialNumber"` // 材质编号 / Material index
	PropertyName   *string `json:"propertyName"`   // 可空的材质属性名称 / Nullable material-property name
	TypeName       *string `json:"typeName"`       // 可空的属性类型名称 / Nullable property type name
	Value          *string `json:"value"`          // 可空的序列化属性值 / Nullable serialized property value
}

// KCESPresetPropBase 保留当前 PropBase.Serialize 的布局
// nil 切片与非 nil 空切片有意区分，因为每个集合前都在线格式中写入存在性布尔值
// KCESPresetPropBase preserves the current PropBase.Serialize layout
// Nil and non-nil empty slices are intentionally distinct because the wire writes a presence Boolean before each collection
type KCESPresetPropBase struct {
	Index                    int32                         `json:"index"`                    // 线格式中的属性索引，游戏反序列化后会用属性名称解析出的 MPN 覆盖此值 / Property index on the wire, overwritten after game deserialization by the MPN resolved from the property name
	Type                     string                        `json:"type"`                     // PropBase.Type 的字符串名称 / String name of PropBase.Type
	SubType                  string                        `json:"subType"`                  // 子属性使用的 PropBase.Type 字符串名称 / String name of the PropBase.Type used by sub-properties
	FileName                 *string                       `json:"fileName"`                 // 可空的当前菜单文件名 / Nullable current menu filename
	FileNameRID              uint64                        `json:"fileNameRid"`              // 当前菜单文件的 RID / RID of the current menu file
	Enabled                  bool                          `json:"enabled"`                  // PropBase 的待处理标志 bDut / PropBase processing-pending flag bDut
	BeforeFileNameRID        uint64                        `json:"beforeFileNameRid"`        // 前一个菜单文件的 RID / RID of the previous menu file
	Defines                  uint64                        `json:"defines"`                  // 应用当前属性时使用的 Menu.DEFINE 位掩码 / Menu.DEFINE bit mask used when applying the current property
	SavedTextureDataRID      uint64                        `json:"savedTextureDataRid"`      // 保存纹理编辑数据时对应的菜单文件 RID / Menu-file RID associated with the saved texture-edit data
	SavedTextureDataDefines  uint64                        `json:"savedTextureDataDefines"`  // 保存纹理编辑数据时对应的 Menu.DEFINE 位掩码 / Menu.DEFINE bit mask associated with the saved texture-edit data
	SavedTextureData         []KCESPresetNamedSavedTexture `json:"savedTextureData"`         // 按层名称保存的纹理编辑数据，nil 与空集合在线格式中不同 / Texture-edit data keyed by layer name, with nil distinct from an empty collection on the wire
	ShareInfinityColorData   bool                          `json:"shareInfinityColorData"`   // 旧版表示是否共享无限色数据，属性版本高于 2003 时游戏读取但丢弃此线格式值 / Indicates shared infinity-color data in older versions, while the game reads and discards this wire value after property version 2003
	EditBaseData             *KCESPresetEditBaseData       `json:"editBaseData"`             // EditCustomizeData.BaseData 的 Standard MessagePack 对象 / Standard MessagePack object for EditCustomizeData.BaseData
	SavedCutoutMaskRID       uint64                        `json:"savedCutoutMaskRid"`       // 保存裁切遮罩时对应的菜单文件 RID / Menu-file RID associated with the saved cutout mask
	SavedCutoutMask          *KCESPresetCutoutMask         `json:"savedCutoutMask"`          // 可空的材质裁切遮罩状态 / Nullable material cutout-mask state
	SavedPartHideRID         uint64                        `json:"savedPartHideRid"`         // 保存部件隐藏状态时对应的菜单文件 RID / Menu-file RID associated with the saved part-hide state
	SavedPartHide            []KCESPresetPartHide          `json:"savedPartHide"`            // 可空的部件隐藏状态列表 / Nullable list of part-hide states
	UsePartHide              bool                          `json:"usePartHide"`              // 部件隐藏状态需要重新应用的标志 dutPartHide / Flag dutPartHide indicating that part-hide state needs to be reapplied
	SavedAttachPositionRID   uint64                        `json:"savedAttachPositionRid"`   // 保存附着位置时对应的菜单文件 RID / Menu-file RID associated with the saved attachment positions
	SavedAttachPositions     []SavedAttachData             `json:"savedAttachPositions"`     // 可空的部件附着位置编辑列表 / Nullable list of edited part attachment positions
	NoScale                  bool                          `json:"noScale"`                  // 设置属性时请求不应用缩放的标志 / Flag requesting that scaling not be applied when setting the property
	SubPropertyIsTuftTexture bool                          `json:"subPropertyIsTuftTexture"` // 标记此属性的子属性用于房状发束纹理 / Marks this property's sub-properties as tuft-hair textures
	SavedHairLengthRID       uint64                        `json:"savedHairLengthRid"`       // 保存发长数据时对应的菜单文件 RID / Menu-file RID associated with the saved hair-length data
	SavedHairLengths         []KCESPresetSavedHairLength   `json:"savedHairLengths"`         // 可空的发长控制点列表 / Nullable list of hair-length control points
	SubProperties            []*KCESPresetSubProperty      `json:"subProperties"`            // 可空且允许包含 nil 项的 SubProp 列表 / Nullable SubProp list whose entries may also be nil
}

// KCESPresetNamedSavedTexture 表示 PropBase.savedTexDatas 字典中的一项
// KCESPresetNamedSavedTexture represents one entry in the PropBase.savedTexDatas dictionary
type KCESPresetNamedSavedTexture struct {
	Key   string                     `json:"key"`   // 纹理层或颜色变化数据的字典键 / Dictionary key for the texture layer or color-variation data
	Value KCESPresetSavedTextureData `json:"value"` // SavedTexData 二进制内容 / SavedTexData binary contents
}

// KCESPresetSavedTextureData 表示 SavedTexData.Serialize 写出的纹理编辑状态
// KCESPresetSavedTextureData represents texture-edit state written by SavedTexData.Serialize
type KCESPresetSavedTextureData struct {
	UseLayer               bool                          `json:"useLayer"`               // 是否使用保存的纹理层 / Whether the saved texture layer is used
	UseMultiplyAlpha       bool                          `json:"useMultiplyAlpha"`       // 是否使用乘算透明度 / Whether multiplied alpha is used
	MultiplyAlpha          float32                       `json:"multiplyAlpha"`          // 保存的乘算透明度值 / Saved multiplied-alpha value
	Masks                  []KCESPresetTextureMask       `json:"masks"`                  // 可空的纹理遮罩状态数组 / Nullable array of texture-mask states
	Transforms             []*KCESPresetTextureTransform `json:"transforms"`             // 可空的纹理变换列表，列表项本身不可为 nil / Nullable texture-transform list whose entries are not nullable
	InfinityColor          *KCESPresetInfinityColorData  `json:"infinityColor"`          // 可空的无限色纹理参数 / Nullable infinity-color texture parameters
	InfinityColorLinkLayer *string                       `json:"infinityColorLinkLayer"` // 可空的无限色数据链接层名称 / Nullable layer name linked to the infinity-color data
	UseAlphaMaskTransform  bool                          `json:"useAlphaMaskTransform"`  // 是否使用透明遮罩变换 / Whether alpha-mask transformation is used
}

// KCESPresetTextureMask 表示 TexLay.MaskData 的可空名称和遮罩开关
// KCESPresetTextureMask represents the nullable name and mask switch of TexLay.MaskData
type KCESPresetTextureMask struct {
	Name *string `json:"name"` // 可空的遮罩名称 / Nullable mask name
	Mask bool    `json:"mask"` // 遮罩启用值 / Mask enable value
}

// KCESPresetTextureTransform 表示 TexLay.TransTexData 的递归纹理变换状态
// KCESPresetTextureTransform represents recursive texture-transform state from TexLay.TransTexData
type KCESPresetTextureTransform struct {
	AreaUVDefault Vector4                     `json:"areaUvDefault"` // 序列化时保存的 TexLay.TransTexData.areaUVDefault 静态值 / Serialized static value of TexLay.TransTexData.areaUVDefault
	ScaleDefault  Vector2                     `json:"scaleDefault"`  // 序列化时保存的 TexLay.TransTexData.scaleDefault 静态值 / Serialized static value of TexLay.TransTexData.scaleDefault
	Position      Vector2                     `json:"position"`      // 纹理归一化位置 / Normalized texture position
	Scale         Vector2                     `json:"scale"`         // 纹理缩放 / Texture scale
	Rotation      float32                     `json:"rotation"`      // 纹理旋转角度 / Texture rotation angle
	AreaUV        Vector4                     `json:"areaUv"`        // 纹理作用区域的 UV 范围 / UV range of the texture application area
	SourcePixels  Vector2Int                  `json:"sourcePixels"`  // 源纹理像素尺寸 / Source texture pixel dimensions
	Default       *KCESPresetTextureTransform `json:"default"`       // 重置变换时使用的可空默认变换 / Nullable default transform used when resetting the transformation
}

// KCESPresetInfinityColorData 表示 InfinityColorTexMgr2.InfColData 的旧式 BinaryWriter 布局
// KCESPresetInfinityColorData represents the legacy BinaryWriter layout of InfinityColorTexMgr2.InfColData
type KCESPresetInfinityColorData struct {
	Independent    bool                         `json:"independent"`    // 是否使用独立多色数据 / Whether independent multi-color data is used
	ColorType      string                       `json:"colorType"`      // InfinityColorTexMgr2.InfColData.COLOR_TYPE 的名称 / Name of InfinityColorTexMgr2.InfColData.COLOR_TYPE
	PartsColorType string                       `json:"partsColorType"` // MaidInfinityColor.PARTS_COLOR 的名称 / Name of MaidInfinityColor.PARTS_COLOR
	Color          KCESPresetInfinityPartsColor `json:"color"`          // 基础无限色参数 / Base infinity-color parameters
	PartColors     []KCESPresetPartColorDef     `json:"partColors"`     // 可空的分部颜色定义列表 / Nullable list of per-part color definitions
	Gradation      *KCESPresetGradationColorDef `json:"gradation"`      // 可空的渐变色定义 / Nullable gradation-color definition
	GradationMugen bool                         `json:"gradationMugen"` // 渐变颜色是否使用无限色模式 / Whether gradation color uses infinity-color mode
}

// KCESPresetInfinityPartsColor 表示 MaidInfinityColor.PartsColor 的九个颜色参数及可选渐变点
// KCESPresetInfinityPartsColor represents the nine color parameters and optional gradation points of MaidInfinityColor.PartsColor
type KCESPresetInfinityPartsColor struct {
	MainHue          int32                               `json:"mainHue"`          // 主色相 / Main hue
	MainChroma       int32                               `json:"mainChroma"`       // 主彩度 / Main chroma
	MainBrightness   int32                               `json:"mainBrightness"`   // 主明度 / Main brightness
	MainContrast     int32                               `json:"mainContrast"`     // 主对比度 / Main contrast
	ShadowRate       int32                               `json:"shadowRate"`       // 阴影混合比例 / Shadow blend rate
	ShadowHue        int32                               `json:"shadowHue"`        // 阴影色相 / Shadow hue
	ShadowChroma     int32                               `json:"shadowChroma"`     // 阴影彩度 / Shadow chroma
	ShadowBrightness int32                               `json:"shadowBrightness"` // 阴影明度 / Shadow brightness
	ShadowContrast   int32                               `json:"shadowContrast"`   // 阴影对比度 / Shadow contrast
	Gradation        []KCESPresetInfinityPartsColorPoint `json:"gradation"`        // 渐变点颜色数组，零长度时游戏恢复为 nil / Gradation-point color array, restored as nil by the game when its length is zero
}

// KCESPresetInfinityPartsColorPoint 表示渐变中的一个九参数 PartsColor 点
// KCESPresetInfinityPartsColorPoint represents one nine-parameter PartsColor point in a gradation
type KCESPresetInfinityPartsColorPoint struct {
	MainHue          int32 `json:"mainHue"`          // 主色相 / Main hue
	MainChroma       int32 `json:"mainChroma"`       // 主彩度 / Main chroma
	MainBrightness   int32 `json:"mainBrightness"`   // 主明度 / Main brightness
	MainContrast     int32 `json:"mainContrast"`     // 主对比度 / Main contrast
	ShadowRate       int32 `json:"shadowRate"`       // 阴影混合比例 / Shadow blend rate
	ShadowHue        int32 `json:"shadowHue"`        // 阴影色相 / Shadow hue
	ShadowChroma     int32 `json:"shadowChroma"`     // 阴影彩度 / Shadow chroma
	ShadowBrightness int32 `json:"shadowBrightness"` // 阴影明度 / Shadow brightness
	ShadowContrast   int32 `json:"shadowContrast"`   // 阴影对比度 / Shadow contrast
}

// KCESPresetPartColorDef 表示 InfinityColorTexMgr2.PartColDef 的旧式线格式
// KCESPresetPartColorDef represents the legacy wire form of InfinityColorTexMgr2.PartColDef
type KCESPresetPartColorDef struct {
	PartName     *string                      `json:"partName"`        // 可空的部件名称 / Nullable part name
	Color        KCESPresetInfinityPartsColor `json:"color"`           // 此部件的多色参数 / Multi-color parameters for this part
	PatternScale Vector2                      `json:"patternScale"`    // 此部件的图案缩放 / Pattern scale for this part
	PatternRot   float32                      `json:"patternRotation"` // 此部件的图案旋转 / Pattern rotation for this part
}

// KCESPresetGradationColorDef 表示 InfinityColorTexMgr2.GradaColDef 的旧式线格式
// KCESPresetGradationColorDef represents the legacy wire form of InfinityColorTexMgr2.GradaColDef
type KCESPresetGradationColorDef struct {
	NotUse     *string                      `json:"notUse"`     // 游戏字段名为 notUse 的可空保留字符串，当前源码未赋予用途 / Nullable reserved string named notUse by the game, with no use assigned in the current source
	PointCount int32                        `json:"pointCount"` // 渐变点数量 gradaNum / Gradation-point count gradaNum
	Rates      []float32                    `json:"rates"`      // 可空的渐变点比率数组 / Nullable gradation-point rate array
	Ranges     []Vector4                    `json:"ranges"`     // 可空的渐变比率范围数组 / Nullable gradation-rate range array
	Color      KCESPresetInfinityPartsColor `json:"color"`      // 包含各渐变点颜色的多色参数 / Multi-color parameters containing the gradation-point colors
}

// KCESPresetCutoutMask 表示 MaterialMgr.CutoutMask 的三个线格式字段
// KCESPresetCutoutMask represents the three wire fields of MaterialMgr.CutoutMask
type KCESPresetCutoutMask struct {
	MaxLevel int32 `json:"maxLevel"` // 裁切遮罩最大等级 / Maximum cutout-mask level
	NowLevel int32 `json:"nowLevel"` // 裁切遮罩当前等级 / Current cutout-mask level
	Enabled  bool  `json:"enabled"`  // 裁切遮罩待应用标志 dut / Cutout-mask pending-application flag dut
}

// KCESPresetPartHide 表示 TBodySkin.PartHide 在线格式中保存的名称和启用值
// KCESPresetPartHide represents the name and enable value stored on the wire for TBodySkin.PartHide
type KCESPresetPartHide struct {
	PartName *string `json:"partName"` // 可空的身体部件名称 / Nullable body-part name
	Enabled  bool    `json:"enabled"`  // 是否隐藏此部件的保存值 / Saved value controlling whether this part is hidden
}

// KCESPresetSavedHairLength 表示 SavedHairLength 的部件名称和长度值
// KCESPresetSavedHairLength represents the part name and length value of SavedHairLength
type KCESPresetSavedHairLength struct {
	PartName *string `json:"partName"` // 可空的发长控制组名称 / Nullable hair-length control-group name
	Value    float32 `json:"value"`    // 保存的发长控制值，游戏默认值为 0.5 / Saved hair-length control value, defaulting to 0.5 in the game
}

// KCESPresetSubProperty 表示 SubProp.Serialize 写出的专用字段及其后继承的 PropBase 块
// KCESPresetSubProperty represents the dedicated fields written by SubProp.Serialize and its following inherited PropBase block
type KCESPresetSubProperty struct {
	Number                      int32                   `json:"number"`                      // 父属性列表中的子属性编号 nNo / Sub-property number nNo within the parent property list
	DefaultHokuroTattooSlotID   string                  `json:"defaultHokuroTattooSlotId"`   // 痣或纹身的默认放置槽位名称 / Default placement slot name for a mole or tattoo
	EditUnitData                *KCESPresetEditUnitData `json:"editUnitData"`                // EditCustomizeData.UnitData 的 Standard MessagePack 对象 / Standard MessagePack object for EditCustomizeData.UnitData
	SavedDefaultHokuroTattooRID uint64                  `json:"savedDefaultHokuroTattooRid"` // 保存默认痣或纹身位置时对应的 RID，仅从属性版本 2001 起存在 / RID associated with the saved default mole or tattoo position, present only since property version 2001
	Base                        KCESPresetPropBase      `json:"base"`                        // 随后的 PropBase 继承字段 / Following inherited PropBase fields
}

// KCESPresetColorData 表示 MaidInfinityColor.Serialize 的颜色预设块
// 当前 1270 版只写入 22 个枚举名称和 MAX 终止符，实际自定义颜色保存在 PropBase.savedTexDatas 中
// KCESPresetColorData represents the color preset block written by MaidInfinityColor.Serialize
// Current v1270 writes only 22 enum names and a MAX terminator, while actual customized colors live in PropBase.savedTexDatas
type KCESPresetColorData struct {
	Signature    string                  `json:"signature"`              // 颜色块签名 CM3D2_MULTI_COL / Color-block signature CM3D2_MULTI_COL
	Version      int32                   `json:"version"`                // 颜色块版本 / Color-block version
	PartCount    int32                   `json:"partCount"`              // 线格式中的部件数量，新版游戏读取后不使用此值 / Part count stored on the wire and ignored after reading by the newer game branch
	LegacyParts  []KCESPresetLegacyColor `json:"legacyParts,omitempty"`  // 版本不高于 1200 时按 PartCount 保存的旧式颜色值 / Legacy color values stored by PartCount for versions up to 1200
	PartNames    []string                `json:"partNames,omitempty"`    // 新版从线格式读取到大小写敏感 MAX 终止符之前的 PARTS_COLOR 名称 / Newer PARTS_COLOR names read until the case-sensitive MAX terminator
	TrailingData []byte                  `json:"trailingData,omitempty"` // 当前已知颜色块结构之后的未解析字节 / Unparsed bytes following the currently known color-block layout
}

// KCESPresetLegacyColor 表示版本 1201 前 CM3D2_MULTI_COL 中的一项
// KCES 改为 PARTS_COLOR 名称列表之前每项正好写入这十个值，此模型不执行迁移或默认颜色展开
// KCESPresetLegacyColor represents one pre-v1201 CM3D2_MULTI_COL entry
// Before KCES switched to a PARTS_COLOR name list each entry wrote exactly these ten values, with no migration or default-color expansion applied here
type KCESPresetLegacyColor struct {
	Use              bool  `json:"use"`              // 旧式颜色是否启用 / Whether the legacy color is enabled
	MainHue          int32 `json:"mainHue"`          // 主色相 / Main hue
	MainChroma       int32 `json:"mainChroma"`       // 主彩度 / Main chroma
	MainBrightness   int32 `json:"mainBrightness"`   // 主明度 / Main brightness
	MainContrast     int32 `json:"mainContrast"`     // 主对比度 / Main contrast
	ShadowRate       int32 `json:"shadowRate"`       // 阴影混合比例 / Shadow blend rate
	ShadowHue        int32 `json:"shadowHue"`        // 阴影色相 / Shadow hue
	ShadowChroma     int32 `json:"shadowChroma"`     // 阴影彩度 / Shadow chroma
	ShadowBrightness int32 `json:"shadowBrightness"` // 阴影明度 / Shadow brightness
	ShadowContrast   int32 `json:"shadowContrast"`   // 阴影对比度 / Shadow contrast
}

// KCESPresetBodyData 表示 Maid.SerializeBody 当前只含签名和版本的身体块，并保留后续未知字节
// KCESPresetBodyData represents the body block that currently contains only the signature and version in Maid.SerializeBody while preserving later unknown bytes
type KCESPresetBodyData struct {
	Signature    string `json:"signature"`              // 身体块签名 CM3D2_MAID_BODY / Body-block signature CM3D2_MAID_BODY
	Version      int32  `json:"version"`                // 身体块版本，当前游戏写入 1270 / Body-block version, written as 1270 by the current game
	TrailingData []byte `json:"trailingData,omitempty"` // 签名和版本之后的未知兼容字节，当前游戏不写入 / Unknown compatibility bytes after the signature and version, not written by the current game
}

// kcesPresetInnerReader 组合剩余长度可检查的字节读取器与 BinaryReader
// kcesPresetInnerReader combines a byte reader with observable remaining length and a BinaryReader
type kcesPresetInnerReader struct {
	r  *bytes.Reader        // 提供剩余字节数和底层读取位置 / Provides remaining-byte count and the underlying read position
	br *stream.BinaryReader // 按游戏 BinaryReader 线格式读取基础值 / Reads primitive values using the game's BinaryReader wire format
}

// newKCESPresetInnerReader 为 maiddata 内部块创建共享底层位置的长度检查读取器和 BinaryReader
// newKCESPresetInnerReader creates a length-checking reader and BinaryReader sharing one position for an inner maiddata block
func newKCESPresetInnerReader(data []byte) *kcesPresetInnerReader {
	r := bytes.NewReader(data)
	return &kcesPresetInnerReader{r: r, br: stream.NewBinaryReader(r)}
}

// readString 读取游戏 BinaryReader 字符串并拒绝无效 UTF-8
// readString reads a game BinaryReader string and rejects invalid UTF-8
func (r *kcesPresetInnerReader) readString(path string) (string, error) {
	value, err := r.br.ReadString()
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%s is not valid UTF-8", path)
	}
	return value, nil
}

// readNullableString 读取 WriteNaS 使用的存在性布尔值及其后的可空字符串
// readNullableString reads the presence Boolean and following nullable string used by WriteNaS
func (r *kcesPresetInnerReader) readNullableString(path string) (*string, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	value, err := r.readString(path)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// readCount 读取非负 Int32 数量，并可按每项最少字节数在分配前检查剩余数据
// readCount reads a non-negative Int32 count and can check remaining data before allocation using a minimum byte size per item
func (r *kcesPresetInnerReader) readCount(path string, minimumBytes int) (int, error) {
	count, err := r.br.ReadInt32()
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	if count < 0 {
		return 0, fmt.Errorf("negative %s %d", path, count)
	}
	if minimumBytes > 0 && int64(count) > int64(r.r.Len()/minimumBytes) {
		return 0, fmt.Errorf("%s %d cannot fit in %d remaining bytes", path, count, r.r.Len())
	}
	return int(count), nil
}

// readBlob 读取 Int32 长度前缀的字节块并确认长度可容纳在剩余数据中
// readBlob reads an Int32-length-prefixed byte block and verifies that it fits in the remaining data
func (r *kcesPresetInnerReader) readBlob(path string) ([]byte, error) {
	length, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read %s length: %w", path, err)
	}
	if length < 0 {
		return nil, fmt.Errorf("negative %s length %d", path, length)
	}
	if int64(length) > int64(r.r.Len()) {
		return nil, fmt.Errorf("%s length %d exceeds %d remaining bytes", path, length, r.r.Len())
	}
	data, err := r.br.ReadBytes(int(length))
	if err != nil {
		return nil, fmt.Errorf("read %s data: %w", path, err)
	}
	return data, nil
}

// validateKCESPresetInnerString 验证字符串为 UTF-8 且字节长度可由游戏 Int32 线格式表示
// validateKCESPresetInnerString verifies that a string is UTF-8 and its byte length fits the game's Int32 wire range
func validateKCESPresetInnerString(value, path string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", path)
	}
	if uint64(len(value)) > uint64(math.MaxInt32) {
		return fmt.Errorf("%s is %d bytes, exceeds Int32", path, len(value))
	}
	return nil
}

// validateKCESPresetInnerNullableString 在非 nil 时验证可空字符串
// validateKCESPresetInnerNullableString validates a nullable string when it is non-nil
func validateKCESPresetInnerNullableString(value *string, path string) error {
	if value == nil {
		return nil
	}
	return validateKCESPresetInnerString(*value, path)
}

// writeKCESPresetInnerNullableString 按游戏 WriteNaS 布局写入存在性布尔值和可选字符串
// writeKCESPresetInnerNullableString writes a presence Boolean and optional string using the game's WriteNaS layout
func writeKCESPresetInnerNullableString(bw *stream.BinaryWriter, value *string) error {
	if err := bw.WriteBool(value != nil); err != nil {
		return err
	}
	if value != nil {
		return bw.WriteString(*value)
	}
	return nil
}

// validateKCESPresetInnerSliceLength 验证集合长度可由游戏 Int32 数量字段表示
// validateKCESPresetInnerSliceLength verifies that a collection length fits the game's Int32 count field
func validateKCESPresetInnerSliceLength(length int, path string) error {
	if uint64(length) > uint64(math.MaxInt32) {
		return fmt.Errorf("%s length %d exceeds Int32", path, length)
	}
	return nil
}

// validateKCESPresetInnerBlob 验证字节块长度可由游戏 Int32 长度前缀表示
// validateKCESPresetInnerBlob verifies that a byte-block length fits the game's Int32 length prefix
func validateKCESPresetInnerBlob(data []byte, path string) error {
	if uint64(len(data)) > uint64(math.MaxInt32) {
		return fmt.Errorf("%s length %d exceeds Int32", path, len(data))
	}
	return nil
}

// writeKCESPresetInnerBlob 写入 Int32 长度前缀及其后的原始字节
// writeKCESPresetInnerBlob writes an Int32 length prefix followed by raw bytes
func writeKCESPresetInnerBlob(bw *stream.BinaryWriter, data []byte) error {
	if err := validateKCESPresetInnerBlob(data, "blob"); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(data))); err != nil {
		return err
	}
	return bw.WriteBytes(data)
}

// DecodeKCESPresetColorData 解码 maiddata 的 CM3D2_MULTI_COL 块并保留已知结构后的字节
// DecodeKCESPresetColorData decodes the CM3D2_MULTI_COL block in maiddata and preserves bytes after the known layout
func DecodeKCESPresetColorData(data []byte) (*KCESPresetColorData, error) {
	r := newKCESPresetInnerReader(data)
	signature, err := r.readString("KCES preset color signature")
	if err != nil {
		return nil, err
	}
	if signature != KCESPresetColorSignature {
		return nil, fmt.Errorf("invalid KCES preset color signature %q", signature)
	}
	version, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCES preset color version: %w", err)
	}
	partCount, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCES preset color partCount: %w", err)
	}
	if partCount < 0 {
		return nil, fmt.Errorf("negative KCES preset color partCount %d", partCount)
	}
	result := &KCESPresetColorData{Signature: signature, Version: version, PartCount: partCount}
	if version <= 1200 {
		const legacyColorBytes = 1 + 9*4
		if int64(partCount) > int64(r.r.Len()/legacyColorBytes) {
			return nil, fmt.Errorf("KCES preset legacy color partCount %d cannot fit in %d remaining bytes", partCount, r.r.Len())
		}
		result.LegacyParts = makeKCESCountedSliceForAppend[KCESPresetLegacyColor](uint64(partCount))
		for index := int32(0); index < partCount; index++ {
			entry := KCESPresetLegacyColor{}
			if entry.Use, err = r.br.ReadBool(); err != nil {
				return nil, fmt.Errorf("read KCES preset legacyParts[%d].use: %w", index, err)
			}
			fields := []*int32{&entry.MainHue, &entry.MainChroma, &entry.MainBrightness, &entry.MainContrast, &entry.ShadowRate, &entry.ShadowHue, &entry.ShadowChroma, &entry.ShadowBrightness, &entry.ShadowContrast}
			for fieldIndex, field := range fields {
				v, readErr := r.br.ReadInt32()
				if readErr != nil {
					return nil, fmt.Errorf("read KCES preset legacyParts[%d] field[%d]: %w", index, fieldIndex, readErr)
				}
				*field = v
			}
			result.LegacyParts = append(result.LegacyParts, entry)
		}
	} else {
		// 当前游戏在此分支故意忽略 partCount，并持续读取名称直到精确匹配大小写敏感的 MAX
		// 因此单独保留线格式中的数量，而不将它规范化为 len(PartNames)
		// The current game deliberately ignores partCount in this branch and reads names until the exact case-sensitive string MAX
		// The stored count is therefore preserved independently instead of being normalized to len(PartNames)
		for index := 0; ; index++ {
			name, readErr := r.readString(fmt.Sprintf("KCES preset color partNames[%d]", index))
			if readErr != nil {
				return nil, readErr
			}
			if name == "MAX" {
				break
			}
			result.PartNames = append(result.PartNames, name)
		}
	}
	if r.r.Len() != 0 {
		result.TrailingData, err = r.br.ReadBytes(r.r.Len())
		if err != nil {
			return nil, fmt.Errorf("read KCES preset color trailingData: %w", err)
		}
	}
	return result, nil
}

// EncodeKCESPresetColorData 按版本写回旧式颜色项或以 MAX 终止的新版名称列表
// EncodeKCESPresetColorData writes either legacy color entries or the newer MAX-terminated name list according to the version
func EncodeKCESPresetColorData(value *KCESPresetColorData) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES preset colorData")
	}
	signature := value.Signature
	if signature != KCESPresetColorSignature {
		return nil, fmt.Errorf("invalid KCES preset color signature %q", signature)
	}
	version := value.Version
	if err := validateKCESPresetInnerSliceLength(len(value.LegacyParts), "KCES preset color legacyParts"); err != nil {
		return nil, err
	}
	if err := validateKCESPresetInnerSliceLength(len(value.PartNames), "KCES preset color partNames"); err != nil {
		return nil, err
	}
	if value.PartCount < 0 {
		return nil, fmt.Errorf("negative KCES preset color partCount %d", value.PartCount)
	}
	if version <= 1200 {
		if int(value.PartCount) != len(value.LegacyParts) {
			return nil, fmt.Errorf("KCES preset legacy color partCount %d does not match legacyParts length %d", value.PartCount, len(value.LegacyParts))
		}
		if len(value.PartNames) != 0 {
			return nil, fmt.Errorf("KCES preset color version %d uses legacyParts, not partNames", version)
		}
	} else {
		if len(value.LegacyParts) != 0 {
			return nil, fmt.Errorf("KCES preset color version %d uses partNames, not legacyParts", version)
		}
		for index, name := range value.PartNames {
			if err := validateKCESPresetInnerString(name, fmt.Sprintf("KCES preset color partNames[%d]", index)); err != nil {
				return nil, err
			}
			if name == "MAX" {
				return nil, fmt.Errorf("KCES preset color partNames[%d] is the reserved terminator MAX", index)
			}
		}
	}
	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	for _, writeErr := range []error{bw.WriteString(signature), bw.WriteInt32(version), bw.WriteInt32(value.PartCount)} {
		if writeErr != nil {
			return nil, writeErr
		}
	}
	if version <= 1200 {
		for index := range value.LegacyParts {
			entry := &value.LegacyParts[index]
			if err := bw.WriteBool(entry.Use); err != nil {
				return nil, err
			}
			for _, field := range []int32{entry.MainHue, entry.MainChroma, entry.MainBrightness, entry.MainContrast, entry.ShadowRate, entry.ShadowHue, entry.ShadowChroma, entry.ShadowBrightness, entry.ShadowContrast} {
				if err := bw.WriteInt32(field); err != nil {
					return nil, err
				}
			}
		}
	} else {
		for _, name := range value.PartNames {
			if err := bw.WriteString(name); err != nil {
				return nil, err
			}
		}
		if err := bw.WriteString("MAX"); err != nil {
			return nil, err
		}
	}
	if err := bw.WriteBytes(value.TrailingData); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// NewKCESPresetColorData 创建与当前游戏 1270 版二十二个 PARTS_COLOR 名称一致的颜色块
// NewKCESPresetColorData creates a color block matching the current game's 22 PARTS_COLOR names for version 1270
func NewKCESPresetColorData() *KCESPresetColorData {
	return &KCESPresetColorData{
		Signature: KCESPresetColorSignature,
		Version:   KCESPresetColorVersion,
		PartCount: 22,
		PartNames: []string{"HAIR", "EYE_BROW", "UNDER_HAIR", "ASS_HAIR", "SKIN", "HAIR_OUTLINE", "SKIN_OUTLINE", "EYE_WHITE", "HOKURO", "TATOO", "SOBAKASU", "MATSUGE_UP", "MATSUGE_LOW", "FUTAE", "PART_COLOR", "GRADA_COLOR", "MAKE", "MUGEN_COLOR", "HIGE", "SHIMI", "SHIWA", "BODY_HAIR"},
	}
}

// DecodeKCESPresetBodyData 解码当前仅含签名和版本的身体块并保留其后字节
// DecodeKCESPresetBodyData decodes the body block that currently contains only a signature and version and preserves later bytes
func DecodeKCESPresetBodyData(data []byte) (*KCESPresetBodyData, error) {
	r := newKCESPresetInnerReader(data)
	signature, err := r.readString("KCES preset body signature")
	if err != nil {
		return nil, err
	}
	if signature != KCESPresetBodySignature {
		return nil, fmt.Errorf("invalid KCES preset body signature %q", signature)
	}
	version, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCES preset body version: %w", err)
	}
	result := &KCESPresetBodyData{Signature: signature, Version: version}
	if r.r.Len() != 0 {
		result.TrailingData, err = r.br.ReadBytes(r.r.Len())
		if err != nil {
			return nil, fmt.Errorf("read KCES preset body trailingData: %w", err)
		}
	}
	return result, nil
}

// EncodeKCESPresetBodyData 验证身体块签名并写回版本及保留字节
// EncodeKCESPresetBodyData validates the body-block signature and writes its version and preserved bytes
func EncodeKCESPresetBodyData(value *KCESPresetBodyData) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES preset bodyData")
	}
	signature := value.Signature
	version := value.Version
	if signature != KCESPresetBodySignature {
		return nil, fmt.Errorf("invalid KCES preset body signature %q", signature)
	}
	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	if err := bw.WriteString(signature); err != nil {
		return nil, err
	}
	if err := bw.WriteInt32(version); err != nil {
		return nil, err
	}
	if err := bw.WriteBytes(value.TrailingData); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// NewKCESPresetBodyData 创建使用当前游戏签名和 1270 版本的空身体块
// NewKCESPresetBodyData creates an empty body block using the current game signature and version 1270
func NewKCESPresetBodyData() *KCESPresetBodyData {
	return &KCESPresetBodyData{Signature: KCESPresetBodySignature, Version: KCESPresetBodyVersion}
}

// readKCESPresetFloat32 读取一个 Float32 并为错误附加字段路径
// readKCESPresetFloat32 reads one Float32 and annotates errors with the field path
func readKCESPresetFloat32(r *kcesPresetInnerReader, path string) (float32, error) {
	value, err := r.br.ReadFloat32()
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	return value, nil
}

// writeKCESPresetVector2 按 Unity Vector2 的两个 Float32 分量写入值
// writeKCESPresetVector2 writes a value as the two Float32 components of a Unity Vector2
func writeKCESPresetVector2(bw *stream.BinaryWriter, value Vector2) error {
	return bw.WriteFloat2([2]float32{value.X, value.Y})
}

// writeKCESPresetVector4 按 Unity Vector4 的四个 Float32 分量写入值
// writeKCESPresetVector4 writes a value as the four Float32 components of a Unity Vector4
func writeKCESPresetVector4(bw *stream.BinaryWriter, value Vector4) error {
	return bw.WriteFloat4([4]float32{value.X, value.Y, value.Z, value.W})
}

// readKCESPresetVector2 读取 Unity Vector2 的两个 Float32 分量
// readKCESPresetVector2 reads the two Float32 components of a Unity Vector2
func readKCESPresetVector2(r *kcesPresetInnerReader, path string) (Vector2, error) {
	value, err := r.br.ReadFloat2()
	if err != nil {
		return Vector2{}, fmt.Errorf("read %s: %w", path, err)
	}
	return Vector2{X: value[0], Y: value[1]}, nil
}

// readKCESPresetVector4 读取 Unity Vector4 的四个 Float32 分量
// readKCESPresetVector4 reads the four Float32 components of a Unity Vector4
func readKCESPresetVector4(r *kcesPresetInnerReader, path string) (Vector4, error) {
	value, err := r.br.ReadFloat4()
	if err != nil {
		return Vector4{}, fmt.Errorf("read %s: %w", path, err)
	}
	return Vector4{X: value[0], Y: value[1], Z: value[2], W: value[3]}, nil
}

// readKCESPresetVector2Int 读取 Unity Vector2Int 的两个 Int32 分量
// readKCESPresetVector2Int reads the two Int32 components of a Unity Vector2Int
func readKCESPresetVector2Int(r *kcesPresetInnerReader, path string) (Vector2Int, error) {
	x, err := r.br.ReadInt32()
	if err != nil {
		return Vector2Int{}, fmt.Errorf("read %s.x: %w", path, err)
	}
	y, err := r.br.ReadInt32()
	if err != nil {
		return Vector2Int{}, fmt.Errorf("read %s.y: %w", path, err)
	}
	return Vector2Int{X: int(x), Y: int(y)}, nil
}

// writeKCESPresetVector2Int 验证主机整数范围后写入 Unity Vector2Int 的两个 Int32 分量
// writeKCESPresetVector2Int validates host integer ranges before writing the two Int32 components of a Unity Vector2Int
func writeKCESPresetVector2Int(bw *stream.BinaryWriter, value Vector2Int) error {
	if err := requireInt32("KCES preset Vector2Int.x", value.X); err != nil {
		return err
	}
	if err := requireInt32("KCES preset Vector2Int.y", value.Y); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(value.X)); err != nil {
		return err
	}
	return bw.WriteInt32(int32(value.Y))
}
