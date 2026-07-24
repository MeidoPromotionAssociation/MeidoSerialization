package KCES

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// .menuassets、.materialassets、.model 与 .preset 共用的 Unity/Parts 数据类型及 Int32 校验
// Unity/Parts data types and Int32 validation shared by .menuassets, .materialassets, .model, and .preset

const (
	gameInt32Min int64 = -1 << 31
	gameInt32Max int64 = 1<<31 - 1
)

// Vector2 表示 UnityEngine.Vector2 的 MessagePack 数组布局
// Vector2 represents UnityEngine.Vector2 in MessagePack array layout
type Vector2 struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	X       float32  `json:"x"`         // X 轴分量 / X-axis component
	Y       float32  `json:"y"`         // Y 轴分量 / Y-axis component
}

// Vector2Int 表示 UnityEngine.Vector2Int 的 MessagePack 数组布局
// Vector2Int represents UnityEngine.Vector2Int in MessagePack array layout
type Vector2Int struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	X       int32    `json:"x"`         // X 轴整数分量 / Integer X-axis component
	Y       int32    `json:"y"`         // Y 轴整数分量 / Integer Y-axis component
}

// Vector3 表示 UnityEngine.Vector3 的 MessagePack 数组布局
// Vector3 represents UnityEngine.Vector3 in MessagePack array layout
type Vector3 struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	X       float32  `json:"x"`         // X 轴分量 / X-axis component
	Y       float32  `json:"y"`         // Y 轴分量 / Y-axis component
	Z       float32  `json:"z"`         // Z 轴分量 / Z-axis component
}

// Vector4 表示 UnityEngine.Vector4 或 Quaternion 的 MessagePack 数组布局
// Vector4 represents UnityEngine.Vector4 or Quaternion in MessagePack array layout
type Vector4 struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	X       float32  `json:"x"`         // X 轴分量或四元数 X / X-axis component or quaternion X
	Y       float32  `json:"y"`         // Y 轴分量或四元数 Y / Y-axis component or quaternion Y
	Z       float32  `json:"z"`         // Z 轴分量或四元数 Z / Z-axis component or quaternion Z
	W       float32  `json:"w"`         // W 分量或四元数 W / W component or quaternion W
}

// PartsColor 对应游戏源码 MaidInfinityColor.PartsColor，editing JSON 直接公开回调还原后的 m_grada
// MessagePack codec 在内部按 OnBeforeSerialize 和 OnAfterDeserialize 回调转换 Key 9 的 m_gradaBytes
// PartsColor corresponds to the game's MaidInfinityColor.PartsColor and exposes callback-restored m_grada directly in editing JSON
// The MessagePack codec internally converts the m_gradaBytes value at Key 9 according to the OnBeforeSerialize and OnAfterDeserialize callbacks
type PartsColor struct {
	_struct          struct{}          `codec:",toarray"`           // 强制按数组编码 / Forces array encoding
	MainHue          int32             `json:"m_nMainHue"`          // 主色相，对应 m_nMainHue / Main hue, matching m_nMainHue
	MainChroma       int32             `json:"m_nMainChroma"`       // 主色彩度，对应 m_nMainChroma / Main chroma, matching m_nMainChroma
	MainBrightness   int32             `json:"m_nMainBrightness"`   // 主色亮度，对应 m_nMainBrightness / Main brightness, matching m_nMainBrightness
	MainContrast     int32             `json:"m_nMainContrast"`     // 主色对比度，对应 m_nMainContrast / Main contrast, matching m_nMainContrast
	ShadowRate       int32             `json:"m_nShadowRate"`       // 阴影混合比例，对应 m_nShadowRate / Shadow blend rate, matching m_nShadowRate
	ShadowHue        int32             `json:"m_nShadowHue"`        // 阴影色相，对应 m_nShadowHue / Shadow hue, matching m_nShadowHue
	ShadowChroma     int32             `json:"m_nShadowChroma"`     // 阴影彩度，对应 m_nShadowChroma / Shadow chroma, matching m_nShadowChroma
	ShadowBrightness int32             `json:"m_nShadowBrightness"` // 阴影亮度，对应 m_nShadowBrightness / Shadow brightness, matching m_nShadowBrightness
	ShadowContrast   int32             `json:"m_nShadowContrast"`   // 阴影对比度，对应 m_nShadowContrast / Shadow contrast, matching m_nShadowContrast
	Grada            []PartsColorGrada `json:"m_grada" codec:"-"`   // 回调从 Key 9 字节完整还原的渐变色点 / Gradient-color points fully restored by the callback from the bytes at Key 9
}

// partsColorWire 表示 PartsColor 的固定十槽 MessagePack 布局，其中 Key 9 是回调使用的 byte[] / partsColorWire represents the fixed ten-slot MessagePack layout of PartsColor whose Key 9 is the byte array used by callbacks
type partsColorWire struct {
	_struct          struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	MainHue          int32    // 主色相 / Main hue
	MainChroma       int32    // 主色彩度 / Main chroma
	MainBrightness   int32    // 主色亮度 / Main brightness
	MainContrast     int32    // 主色对比度 / Main contrast
	ShadowRate       int32    // 阴影混合比例 / Shadow blend rate
	ShadowHue        int32    // 阴影色相 / Shadow hue
	ShadowChroma     int32    // 阴影彩度 / Shadow chroma
	ShadowBrightness int32    // 阴影亮度 / Shadow brightness
	ShadowContrast   int32    // 阴影对比度 / Shadow contrast
	GradaBytes       []byte   // SerializeGrada 产生的内部字节 / Internal bytes produced by SerializeGrada
}

// CodecEncodeSelf 按游戏回调将 typed m_grada 转换为内部 byte[] 并编码固定十槽对象
// CodecEncodeSelf converts typed m_grada to the callback byte array and encodes the fixed ten-slot object
func (p PartsColor) CodecEncodeSelf(e *codec.Encoder) {
	encoded, err := EncodePartsColorGrada(p.Grada)
	if err != nil {
		panic(err)
	}
	wire := partsColorWire{
		MainHue:          p.MainHue,
		MainChroma:       p.MainChroma,
		MainBrightness:   p.MainBrightness,
		MainContrast:     p.MainContrast,
		ShadowRate:       p.ShadowRate,
		ShadowHue:        p.ShadowHue,
		ShadowChroma:     p.ShadowChroma,
		ShadowBrightness: p.ShadowBrightness,
		ShadowContrast:   p.ShadowContrast,
		GradaBytes:       encoded,
	}
	ct.EncodeIndexedObjectSelf(e, &wire)
}

// CodecDecodeSelf 解码固定十槽对象并立即将 Key 9 的 byte[] 完整还原为 typed m_grada
// CodecDecodeSelf decodes the fixed ten-slot object and immediately restores the Key 9 byte array into typed m_grada
func (p *PartsColor) CodecDecodeSelf(d *codec.Decoder) {
	var wire partsColorWire
	ct.DecodeIndexedObjectSelf(d, &wire)
	if wire.GradaBytes == nil {
		*p = PartsColor{
			MainHue:          wire.MainHue,
			MainChroma:       wire.MainChroma,
			MainBrightness:   wire.MainBrightness,
			MainContrast:     wire.MainContrast,
			ShadowRate:       wire.ShadowRate,
			ShadowHue:        wire.ShadowHue,
			ShadowChroma:     wire.ShadowChroma,
			ShadowBrightness: wire.ShadowBrightness,
			ShadowContrast:   wire.ShadowContrast,
		}
		return
	}
	values, err := DecodePartsColorGrada(wire.GradaBytes)
	if err != nil {
		panic(fmt.Errorf("decode PartsColor.m_grada: %w", err))
	}
	*p = PartsColor{
		MainHue:          wire.MainHue,
		MainChroma:       wire.MainChroma,
		MainBrightness:   wire.MainBrightness,
		MainContrast:     wire.MainContrast,
		ShadowRate:       wire.ShadowRate,
		ShadowHue:        wire.ShadowHue,
		ShadowChroma:     wire.ShadowChroma,
		ShadowBrightness: wire.ShadowBrightness,
		ShadowContrast:   wire.ShadowContrast,
		Grada:            values,
	}
}

// PartsColorGrada 表示一个 m_grada 元素，SerializeGrada 按小端顺序写入这九个有符号 Int32 字段，并且不会递归序列化元素自身的 m_grada 或 m_gradaBytes 字段
// PartsColorGrada represents one m_grada element, with SerializeGrada writing these nine signed Int32 fields in little-endian order without recursively serializing the element's own m_grada or m_gradaBytes fields
type PartsColorGrada struct {
	MainHue          int32 `json:"m_nMainHue"`          // 主色相 / Main hue
	MainChroma       int32 `json:"m_nMainChroma"`       // 主色彩度 / Main chroma
	MainBrightness   int32 `json:"m_nMainBrightness"`   // 主色亮度 / Main brightness
	MainContrast     int32 `json:"m_nMainContrast"`     // 主色对比度 / Main contrast
	ShadowRate       int32 `json:"m_nShadowRate"`       // 阴影混合比例 / Shadow blend rate
	ShadowHue        int32 `json:"m_nShadowHue"`        // 阴影色相 / Shadow hue
	ShadowChroma     int32 `json:"m_nShadowChroma"`     // 阴影彩度 / Shadow chroma
	ShadowBrightness int32 `json:"m_nShadowBrightness"` // 阴影亮度 / Shadow brightness
	ShadowContrast   int32 `json:"m_nShadowContrast"`   // 阴影对比度 / Shadow contrast
}

const partsColorGradaRecordBytes = 9 * 4

// DecodePartsColorGrada 实现 MaidInfinityColor.PartsColor.DeserializeGrada 并要求声明记录完整消费输入
// DecodePartsColorGrada implements MaidInfinityColor.PartsColor.DeserializeGrada and requires the declared records to consume the complete input
func DecodePartsColorGrada(data []byte) ([]PartsColorGrada, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("gradient byte stream is %d bytes; need the Int32 count", len(data))
	}
	count := int64(int32(binary.LittleEndian.Uint32(data[:4])))
	if count < 0 {
		return nil, fmt.Errorf("gradient color count is negative: %d", count)
	}
	remaining := int64(len(data)) - 4
	if count > remaining/partsColorGradaRecordBytes {
		return nil, fmt.Errorf("gradient color count %d cannot fit in %d payload bytes", count, remaining)
	}
	expected := int64(4) + count*partsColorGradaRecordBytes
	if int64(len(data)) != expected {
		return nil, fmt.Errorf("gradient byte stream has %d trailing bytes", int64(len(data))-expected)
	}

	values := make([]PartsColorGrada, count)
	var offset int64 = 4
	readInt32 := func() int32 {
		value := int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		return value
	}
	for index := int64(0); index < int64(len(values)); index++ {
		value := &values[index]
		value.MainHue = readInt32()
		value.MainChroma = readInt32()
		value.MainBrightness = readInt32()
		value.MainContrast = readInt32()
		value.ShadowRate = readInt32()
		value.ShadowHue = readInt32()
		value.ShadowChroma = readInt32()
		value.ShadowBrightness = readInt32()
		value.ShadowContrast = readInt32()
	}
	if count == 0 {
		// DeserializeGrada 在数量为零时赋值 null 而不是空数组
		// DeserializeGrada assigns null rather than an empty array for a zero count
		values = nil
	}
	return values, nil
}

// EncodePartsColorGrada 按 SerializeGrada 写出的布局编码梯度色数组
// EncodePartsColorGrada encodes a gradient-color array using the layout written by SerializeGrada
func EncodePartsColorGrada(values []PartsColorGrada) ([]byte, error) {
	if int64(len(values)) > gameInt32Max {
		return nil, fmt.Errorf("gradient color count %d exceeds Int32", len(values))
	}
	size := int64(4) + int64(len(values))*partsColorGradaRecordBytes
	out := make([]byte, size)
	binary.LittleEndian.PutUint32(out[:4], uint32(len(values)))
	var offset int64 = 4
	writeInt32 := func(value int32) {
		binary.LittleEndian.PutUint32(out[offset:offset+4], uint32(value))
		offset += 4
	}
	for index := int64(0); index < int64(len(values)); index++ {
		value := &values[index]
		writeInt32(value.MainHue)
		writeInt32(value.MainChroma)
		writeInt32(value.MainBrightness)
		writeInt32(value.MainContrast)
		writeInt32(value.ShadowRate)
		writeInt32(value.ShadowHue)
		writeInt32(value.ShadowChroma)
		writeInt32(value.ShadowBrightness)
		writeInt32(value.ShadowContrast)
	}
	return out, nil
}

// MarshalJSON 只输出 typed m_grada，不公开内部 byte[] 载体
// MarshalJSON emits only typed m_grada and does not expose the internal byte-array carrier
func (p PartsColor) MarshalJSON() ([]byte, error) {
	type partsColorJSON PartsColor
	return json.Marshal(partsColorJSON(p))
}

// UnmarshalJSON 解码 typed m_grada，内部 byte[] 将在 MessagePack 编码时重新生成
// UnmarshalJSON decodes typed m_grada while the internal byte array is rebuilt during MessagePack encoding
func (p *PartsColor) UnmarshalJSON(data []byte) error {
	type partsColorJSON PartsColor
	var decoded partsColorJSON
	if err := decodeKCESJSONStrict(data, &decoded); err != nil {
		return err
	}
	*p = PartsColor(decoded)
	return nil
}

// PreMulTexDatas 对应 Parts.Menu.PreMulTexDatas 的贴图预合成记录
// PreMulTexDatas maps Parts.Menu.PreMulTexDatas texture pre-composition records
type PreMulTexDatas struct {
	_struct               struct{}        `codec:",toarray"`               // 强制按数组编码 / Forces array encoding
	Version               int32           `json:"version"`                 // 版本号，游戏 FixVersion 为 1001 / Version value; the game's FixVersion is 1001
	SlotID                *string         `json:"slotId"`                  // 可空目标槽位 ID / Nullable target slot ID
	SaveTag               *string         `json:"saveTag"`                 // 可空保存用图层标签 / Nullable saved texture layer tag
	MatNo                 int32           `json:"f_nMatNo"`                // 目标材质编号 / Target material index
	PropName              *string         `json:"f_strPropName"`           // 可空目标材质属性名 / Nullable target material property name
	LayerNo               int32           `json:"f_nLayerNo"`              // 贴图层编号 / Texture layer number
	FileName              *string         `json:"f_strFileName"`           // 可空合成源贴图文件名 / Nullable source texture file name for composition
	BlendMode             *string         `json:"f_eBlendMode"`            // 可空合成模式字符串 / Nullable blend-mode string
	MaskParam             *MaskParam      `json:"maskParam"`               // 蒙版参数 / Mask parameters
	InfColParam           *InfColorParam  `json:"infColParam"`             // 无限色参数 / Infinity-color parameters
	TexGroup              bool            `json:"f_bTexGroup"`             // 是否属于贴图组 / Whether this layer belongs to a texture group
	LayNoInGroup          int32           `json:"f_nLayNoInGroup"`         // 组内层编号 / Layer index inside the group
	Alpha                 float32         `json:"f_fAlpha"`                // 合成透明度 / Composition alpha
	TargetBodyTexSize     int32           `json:"f_nTargetBodyTexSize"`    // 目标身体贴图尺寸 / Target body texture size
	PosDefHokuroTatooSlot *string         `json:"posDefHokuroTatooSlotId"` // 可空默认痣/纹身位置槽位 / Nullable default mole/tattoo position slot
	PreMaskData           []*MaskData     `json:"preMaskData"`             // 可空预计算蒙版对象数组 / Array of nullable precomputed mask objects
	PreTransTexData       []*TransTexData `json:"preTransTexData"`         // 可空预计算贴图变换对象列表 / List of nullable precomputed texture-transform objects
	PreInfColData         *InfColData     `json:"preInfColData"`           // 预计算无限色数据 / Precomputed infinity-color data
	PreTexCompoTypeStr    *string         `json:"preTexCompoTypeStr"`      // 可空预合成系统材质模式字符串 / Nullable pre-composition system material mode string
}

// NewPreMulTexDatas 返回 C# 基类构造函数和字段初始化器在 MessagePack 或 JSON 成员赋值前设置的默认值
// NewPreMulTexDatas returns the defaults installed by the C# base constructor and field initializers before MessagePack or JSON member assignment
func NewPreMulTexDatas() *PreMulTexDatas {
	alpha := "Alpha"
	return &PreMulTexDatas{
		Version:            preMulTexDatasFixVersion,
		LayNoInGroup:       -1,
		Alpha:              1,
		PreTexCompoTypeStr: &alpha,
	}
}

// UnmarshalJSON 解码 PreMulTexDatas 的 JSON 表示而不注入构造默认值
// UnmarshalJSON decodes the JSON representation of PreMulTexDatas without injecting constructor defaults
func (p *PreMulTexDatas) UnmarshalJSON(data []byte) error {
	type preMulTexDatasJSON PreMulTexDatas
	var value preMulTexDatasJSON
	if err := decodeKCESJSONStrict(data, &value); err != nil {
		return err
	}
	*p = PreMulTexDatas(value)
	return nil
}

// TransTexData 对应 TexLay.TransTexData，描述合成贴图的平移、缩放和旋转
// TransTexData maps TexLay.TransTexData and describes texture translation, scale, and rotation
type TransTexData struct {
	_struct      struct{}      `codec:",toarray"`    // 强制按数组编码 / Forces array encoding
	Pos          Vector2       `json:"pos"`          // 贴图中心位置，通常为目标 RT 归一化坐标 / Texture center position, usually normalized in the target render texture
	Scale        Vector2       `json:"scale"`        // 贴图缩放，负值表示翻转 / Texture scale; negative values indicate flipping
	RotDeg       float32       `json:"rotDeg"`       // 以角度表示的旋转量 / Rotation in degrees
	AreaUV       Vector4       `json:"areaUV"`       // 使用的源贴图 UV 区域 / Source texture UV area
	SrcTexPixcel Vector2Int    `json:"srcTexPixcel"` // 源贴图像素尺寸，保留游戏字段原拼写 / Source texture pixel size, preserving the game's spelling
	DefTrans     *TransTexData `json:"defTrans"`     // 默认变换，用于 ResetTrans 恢复 / Default transform used by ResetTrans
}

// NewTransTexData 创建使用游戏字段初始化默认变换的新记录
// NewTransTexData creates a new record with the game's field-initializer transform defaults
func NewTransTexData() *TransTexData {
	return &TransTexData{
		Scale:  Vector2{X: 1, Y: 1},
		AreaUV: Vector4{Z: 1, W: 1},
	}
}

// UnmarshalJSON 解码 TransTexData 的 JSON 表示而不注入构造默认值
// UnmarshalJSON decodes the JSON representation of TransTexData without injecting constructor defaults
func (v *TransTexData) UnmarshalJSON(data []byte) error {
	type transTexDataJSON TransTexData
	var value transTexDataJSON
	if err := decodeKCESJSONStrict(data, &value); err != nil {
		return err
	}
	*v = TransTexData(value)
	return nil
}

// InfColorParam 对应 TexLay.InfColorParam，描述无限色合成输入
// InfColorParam maps TexLay.InfColorParam and describes infinity-color composition input
type InfColorParam struct {
	_struct                  struct{}      `codec:",toarray"`                // 强制按数组编码 / Forces array encoding
	Tag                      *string       `json:"tag"`                      // 可空无限色目标标签 / Nullable infinity-color target tag
	InfColType               int32         `json:"infColType"`               // 颜色类型枚举：NONE/INF_COLOR/PART_COLOR/GRADA_COLOR / Color type enum
	InfColorID               int32         `json:"infColorId"`               // MaidInfinityColor.PARTS_COLOR 枚举值 / MaidInfinityColor.PARTS_COLOR enum value
	IsIndependenceMultiColor bool          `json:"isIndependenceMultiColor"` // 是否使用独立多色数据 / Whether independent multi-color data is used
	PC                       PartsColor    `json:"pc"`                       // 单色无限色参数 / Single infinity-color parameters
	IDTexName                []*string     `json:"idTexName"`                // 可空 ID 贴图文件名列表 / List of nullable ID texture file names
	PartCols                 []*PartColDef `json:"partCols"`                 // 可空分部颜色定义对象列表 / List of nullable part-color definition objects
	GradeCols                *GradaColDef  `json:"gradeCols"`                // 渐变色定义，字段名沿用游戏 gradeCols 拼写 / Gradient color definition, keeping the game's spelling
	GradaLines               []Vector4     `json:"gradaLines"`               // 渐变线段数组 / Gradient line array
	IDTexIsRGB               bool          `json:"idTexIsRGB"`               // 是否把 ID 贴图按 RGB 通道分区解释 / Whether ID textures are interpreted by RGB channels
	GradaIsMugen             bool          `json:"gradaIsMugen"`             // 渐变是否使用无限色表 / Whether the gradient uses the infinity-color table
}

// NewInfColorParam 创建使用游戏默认无颜色 ID 的无限色参数
// NewInfColorParam creates infinity-color parameters with the game's default no-color ID
func NewInfColorParam() *InfColorParam {
	return &InfColorParam{InfColorID: -1}
}

// UnmarshalJSON 解码 InfColorParam 的 JSON 表示而不注入构造默认值
// UnmarshalJSON decodes the JSON representation of InfColorParam without injecting constructor defaults
func (v *InfColorParam) UnmarshalJSON(data []byte) error {
	type infColorParamJSON InfColorParam
	var value infColorParamJSON
	if err := decodeKCESJSONStrict(data, &value); err != nil {
		return err
	}
	*v = InfColorParam(value)
	return nil
}

// MaskData 对应 TexLay.MaskData，记录单个蒙版开关
// MaskData maps TexLay.MaskData and stores one mask toggle
type MaskData struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Name    *string  `json:"name"`      // 可空蒙版名称 / Nullable mask name
	Mask    bool     `json:"mask"`      // 是否启用该蒙版 / Whether this mask is enabled
}

// MaskParam 对应 TexLay.MaskParam，描述蒙版贴图和区域
// MaskParam maps TexLay.MaskParam and describes mask texture and ranges
type MaskParam struct {
	_struct           struct{}    `codec:",toarray"`         // 强制按数组编码 / Forces array encoding
	MaskData          []*MaskData `json:"maskData"`          // 可空蒙版对象列表 / List of nullable mask objects
	MaskTexName       *string     `json:"maskTexName"`       // 可空蒙版贴图文件名 / Nullable mask texture file name
	MaskRanges        []Vector4   `json:"maskRanges"`        // 蒙版 UV/范围数组 / Mask UV/range array
	LinkMaskName      *string     `json:"linkMaskName"`      // 可空关联蒙版名称 / Nullable linked mask name
	LinkMaskNo        int32       `json:"linkMaskNo"`        // 关联蒙版编号 / Linked mask index
	ShareRtTargetPart *string     `json:"shareRtTargetPart"` // 可空共享 RenderTexture 目标部件名 / Nullable target part name for shared RenderTexture
}

// PartColDef 对应 InfinityColorTexMgr2.PartColDef，描述 ID 贴图的一个部位颜色
// PartColDef maps InfinityColorTexMgr2.PartColDef and describes one ID-texture part color
type PartColDef struct {
	_struct      struct{}   `codec:",toarray"`    // 强制按数组编码 / Forces array encoding
	PartName     *string    `json:"part_name"`    // 可空部位名称，字段名对应 part_name / Nullable part name, matching part_name
	MultiCol     PartsColor `json:"multi_col"`    // 部位颜色参数 / Part color parameters
	PatternScale Vector2    `json:"patternScale"` // 纹样缩放 / Pattern texture scale
	PatternRot   float32    `json:"patternRot"`   // 纹样旋转角度 / Pattern texture rotation in degrees
}

// NewPartColDef 创建使用游戏默认纹样缩放的新部位颜色定义
// NewPartColDef creates a new part-color definition with the game's default pattern scale
func NewPartColDef() *PartColDef {
	return &PartColDef{PatternScale: Vector2{X: 1, Y: 1}}
}

// UnmarshalJSON 解码 PartColDef 的 JSON 表示而不注入构造默认值
// UnmarshalJSON decodes the JSON representation of PartColDef without injecting constructor defaults
func (v *PartColDef) UnmarshalJSON(data []byte) error {
	type partColDefJSON PartColDef
	var value partColDefJSON
	if err := decodeKCESJSONStrict(data, &value); err != nil {
		return err
	}
	*v = PartColDef(value)
	return nil
}

// GradaColDef 对应 InfinityColorTexMgr2.GradaColDef，描述渐变色定义
// GradaColDef maps InfinityColorTexMgr2.GradaColDef and describes a gradient color definition
type GradaColDef struct {
	_struct         struct{}   `codec:",toarray"`       // 强制按数组编码 / Forces array encoding
	NotUse          *string    `json:"notUse"`          // 用途未知的可空字符串字段 notUse / Nullable string field notUse whose purpose is unknown
	GradaNum        int32      `json:"gradaNum"`        // 渐变点数量 / Number of gradient points
	GradaRates      []float32  `json:"gradaRates"`      // 渐变点位置比例 / Gradient point rates
	GradaRateRanges []Vector4  `json:"gradaRateRanges"` // 渐变点影响范围 / Gradient point influence ranges
	MultiCol        PartsColor `json:"multi_col"`       // 渐变用多色数据 / Multi-color data used by the gradient
}

// InfColData 对应 InfinityColorTexMgr2.InfColData，保存应用后的无限色数据
// InfColData maps InfinityColorTexMgr2.InfColData and stores applied infinity-color data
type InfColData struct {
	_struct                  struct{}      `codec:",toarray"`                // 强制按数组编码 / Forces array encoding
	IsIndependenceMultiColor bool          `json:"isIndependenceMultiColor"` // 是否使用独立多色表 / Whether independent multi-color table data is used
	InfColType               int32         `json:"infColType"`               // 无限色类型枚举 / Infinity-color type enum
	PartsColorType           int32         `json:"partsColorType"`           // MaidInfinityColor.PARTS_COLOR 枚举值 / MaidInfinityColor.PARTS_COLOR enum value
	ColData                  PartsColor    `json:"colData"`                  // 单色无限色数据 / Single infinity-color data
	PartColDefs              []*PartColDef `json:"partColDefs"`              // 可空分部颜色数据对象列表 / List of nullable part-color data objects
	GradaColDef              *GradaColDef  `json:"gradaColDef"`              // 渐变色数据 / Gradient color data
	GradaIsMugen             bool          `json:"gradaIsMugen"`             // 渐变是否按无限色处理 / Whether the gradient is treated as infinity color
}

// NewInfColData 创建使用游戏默认无部件颜色 ID 的无限色数据
// NewInfColData creates infinity-color data with the game's default no-part-color ID
func NewInfColData() *InfColData {
	return &InfColData{PartsColorType: -1}
}

// UnmarshalJSON 解码 InfColData 的 JSON 表示而不注入构造默认值
// UnmarshalJSON decodes the JSON representation of InfColData without injecting constructor defaults
func (v *InfColData) UnmarshalJSON(data []byte) error {
	type infColDataJSON InfColData
	var value infColDataJSON
	if err := decodeKCESJSONStrict(data, &value); err != nil {
		return err
	}
	*v = InfColData(value)
	return nil
}

// Colvari 对应 Parts.Menu.Colvari，保存一个颜色变体菜单的入口信息
// Colvari maps Parts.Menu.Colvari and stores color-variant menu entry data
type Colvari struct {
	_struct      struct{}       `codec:",toarray"`    // 强制按数组编码 / Forces array encoding
	Version      int32          `json:"version"`      // 版本号，游戏 FixVersion 为 1000 / Version value; the game's FixVersion is 1000
	IconColor    PartsColor     `json:"iconColor"`    // 颜色变体图标色 / Color-variant icon color
	IconFileName *string        `json:"iconFileName"` // 可空颜色变体图标文件名 / Nullable color-variant icon file name
	ReqDefine    *string        `json:"reqDefine"`    // 可空 DEFINE 要求 / Nullable DEFINE requirement
	ColvariDatas []*ColvariData `json:"colvariDatas"` // 可空颜色变体数据对象列表 / List of nullable color-variant data objects
}

// ColvariData 对应 Parts.Menu.Colvari.ColvariData，描述一条颜色变体规则
// ColvariData maps Parts.Menu.Colvari.ColvariData and describes one color-variant rule
type ColvariData struct {
	_struct                 struct{}      `codec:",toarray"`               // 强制按数组编码 / Forces array encoding
	Version                 int32         `json:"version"`                 // 版本号，游戏 FixVersion 为 1000 / Version value; the game's FixVersion is 1000
	MPN                     *string       `json:"mpn"`                     // 可空目标 MPN 名称，多个值用竖线分隔 / Nullable target MPN names, pipe-separated when multiple
	LayerName               *string       `json:"layerName"`               // 可空保存图层名 / Nullable saved layer name
	ColorType               int32         `json:"colorType"`               // 主颜色类型枚举 / Primary color type enum
	MaskData                []*MaskData   `json:"maskData"`                // 可空蒙版状态对象列表 / List of nullable mask-state objects
	Alpha                   float32       `json:"alpha"`                   // 乘算透明度 / Multiplicative alpha
	ColData                 PartsColor    `json:"colData"`                 // 单色颜色数据 / Single color data
	PartColDefs             []*PartColDef `json:"partColDefs"`             // 可空分部颜色定义对象列表 / List of nullable part-color definitions
	GradaColDef             *GradaColDef  `json:"gradaColDef"`             // 渐变色定义 / Gradient color definition
	MamaFileName            *string       `json:"mamaFileName"`            // 可空关联 MAMA 文件名 / Nullable related MAMA file name
	ColorTypeSub            int32         `json:"colorTypeSub"`            // 渐变/复合颜色的子类型 / Subtype for gradient or compound color
	UseType                 uint8         `json:"useType"`                 // 使用标志，bit0=alpha，bit1=color / Use flags, bit0=alpha and bit1=color
	SaveInfColDataLinkLayer *string       `json:"saveInfColDataLinkLayer"` // 可空共享无限色数据源图层名 / Nullable source layer name for shared infinity-color data
	ViewName                *string       `json:"viewName"`                // 可空编辑界面显示名 / Nullable display name in the edit UI
}

// BlendData 对应游戏 BlendData，保存模型 morph 顶点差分
// BlendData maps the game's BlendData and stores model morph vertex deltas
type BlendData struct {
	_struct struct{}  `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Name    *string   `json:"name"`      // 可空 morph 名称 / Nullable morph name
	VIndex  []int32   `json:"v_index"`   // 受影响顶点索引 / Affected vertex indices
	Vert    []Vector3 `json:"vert"`      // 顶点位置差分 / Vertex position deltas
	Norm    []Vector3 `json:"norm"`      // 法线差分 / Normal deltas
	Tan     []Vector4 `json:"tan"`       // 切线差分 / Tangent deltas
}

// SkinThickness 对应 Parts.Model.SkinThickness，保存皮肤厚度修正
// SkinThickness maps Parts.Model.SkinThickness and stores skin-thickness correction data
type SkinThickness struct {
	_struct struct{}                   `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Use     bool                       `json:"use"`       // 是否启用皮肤厚度修正 / Whether skin-thickness correction is enabled
	Groups  map[string]*ThicknessGroup `json:"groups"`    // 按组名索引的可空厚度修正组 / Nullable thickness correction groups keyed by group name
}

// ThicknessGroup 对应 Parts.Model.SkinThickness.Group
// ThicknessGroup maps Parts.Model.SkinThickness.Group
type ThicknessGroup struct {
	_struct         struct{}          `codec:",toarray"`      // 强制按数组编码 / Forces array encoding
	GroupName       *string           `json:"groupName"`      // 可空厚度组名称 / Nullable thickness group name
	StartBoneName   *string           `json:"startBoneName"`  // 可空线段起始骨骼名 / Nullable segment start bone name
	EndBoneName     *string           `json:"endBoneName"`    // 可空线段结束骨骼名 / Nullable segment end bone name
	StepAngleDegree int32             `json:"stepAngleDgree"` // 角度采样步进，字段名保留游戏拼写 stepAngleDgree / Angle sampling step, preserving the game's stepAngleDgree spelling
	Points          []*ThicknessPoint `json:"points"`         // 可空厚度采样点列表 / List of nullable thickness sample points
}

// ThicknessPoint 对应 Parts.Model.SkinThickness.Group.Point
// ThicknessPoint maps Parts.Model.SkinThickness.Group.Point
type ThicknessPoint struct {
	_struct                struct{}                `codec:",toarray"`              // 强制按数组编码 / Forces array encoding
	TargetBoneName         *string                 `json:"targetBoneName"`         // 可空采样点目标骨骼名 / Nullable target bone name for this sample point
	RatioSegmentStartToEnd float32                 `json:"ratioSegmentStartToEnd"` // 点位于起止骨骼线段上的比例 / Ratio along the start-to-end bone segment
	DistanceParAngle       []*ThicknessDefPerAngle `json:"distanceParAngle"`       // 可空按角度默认距离对象列表，字段名保留游戏拼写 Par / List of nullable default-distance objects by angle, preserving the game's Par spelling
}

// ThicknessDefPerAngle 对应 Parts.Model.SkinThickness.Group.Point.DefPerAngle
// ThicknessDefPerAngle maps Parts.Model.SkinThickness.Group.Point.DefPerAngle
type ThicknessDefPerAngle struct {
	_struct         struct{} `codec:",toarray"`       // 强制按数组编码 / Forces array encoding
	AngleDegree     int32    `json:"angleDgree"`      // 角度，字段名保留游戏拼写 angleDgree / Angle in degrees, preserving the game's angleDgree spelling
	VertexIndex     int32    `json:"vidx"`            // 顶点索引 vidx / Vertex index, vidx
	DefaultDistance float32  `json:"defaultDistance"` // 默认厚度距离 / Default thickness distance
}

// TupleStringInt 对应 C# Tuple<string,int> 的 MessagePack 数组布局
// TupleStringInt maps the MessagePack array layout of C# Tuple<string,int>
type TupleStringInt struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Item1   *string  `json:"item1"`     // 可空第一个元组字符串值 / Nullable first tuple string value
	Item2   int32    `json:"item2"`     // 第二个元组值 / Second tuple value
}
