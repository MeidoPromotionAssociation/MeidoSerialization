package COM3D2

import (
	"errors"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/utilities"
)

// CM3D2_PRESET
// 角色预设文件
//
//有两种 PRESET 一种 CM3D2_PRESET 一种 CM3D2_PRESET_S， CM3D2_PRESET_S 已官方废弃（不支持、不解析）
//
// - 版本范围：
//	 1 ≤ version < 1560 某些官方预设文件（支持）
//   * CM3D2 的 1560 ≤ version < 20000（支持）
//   * COM3D2 使用 20000 ≤ version < 30000（支持）
//   * COM3D2.5 的 version ≥ 30000（支持）
//
//   COM3D2 和 COM3D2.5 的 Preset 无结构差异，但 COM3D2 会拒绝读取版本大于等于 30000 的文件
//	 COM3D2.5 则无读取版本校验
//
// 子块版本差异（与实际内容功能相关）
//
// 1) CM3D2_MPROP_LIST（属性列表版本）
// - version < 4：列表项前没有写入“键名”（MPN 字符串）
// - version ≥ 4：每个属性项前新增写入“键名”（MPN 字符串）
//
// 2) CM3D2_MPROP（单个属性版本）
// - version < 101：无 temp_value
// - version ≥ 101：新增 temp_value
// - version < 200：无子属性与附加数据（SubProps/皮肤位置/附着位置/材质属性）
// - version ≥ 200：新增
//   * SubProps（子属性数组）
//   * 皮肤位置（SkinPositions）
//   * 顶点附着位置（AttachPositions）
//   * 材质属性（MaterialProps）
// - version < 204：头部材质 ZTest 字段命名旧（读取时会被迁移为 _ZTest2，并调整值）
// - version ≥ 204：材质 ZTest 字段使用新命名/_ZTest2 规则
// - version < 211：SubProp 无 TexMulAlpha
// - version ≥ 211：SubProp 新增 TexMulAlpha
// - version < 213：无骨骼长度（BoneLengths）块
// - version ≥ 213：新增 BoneLengths 块
//
// 3) CM3D2_MULTI_COL（多颜色版本）
// - version ≤ 1200：旧格式（固定顺序的若干部件，历史上有 7 或 9 项的差异）
// - version > 1200：新格式（以部件名列举，直到读到 "MAX" 终止）
//
// 4) CM3D2_MAID_BODY（身体块版本）
// - 当前仅签名+版本，无后续字段；

// 预设类型常量
const (
	PresetTypeWear = 0 // 衣服
	PresetTypeBody = 1 // 身体
	PresetTypeAll  = 2 // 全部
)

// Preset 表示角色预设数据
type Preset struct {
	Signature          string              `json:"Signature"`          // "CM3D2_PRESET"
	Version            int32               `json:"Version"`            // 版本号  大于等于 30000 的是 COM3D2.5 格式，大于等于 1560 且小于 20000 的是 CM3D2 格式，版本号介于 20000 到 30000 之间的是 COM3D2 格式
	PresetType         int32               `json:"PresetType"`         // 预设类型：0=衣服, 1=身体, 2=全部
	ThumbLength        int32               `json:"ThumbLength"`        // 略缩图数据长度
	ThumbData          []byte              `json:"ThumbData"`          // 略缩图数据，PNG格式
	PresetPropertyList *PresetPropertyList `json:"PresetPropertyList"` // 预设属性列表
	MultiColor         *MultiColor         `json:"MultiColor"`         // 颜色设置
	BodyProperty       *BodyProperty       `json:"BodyProperty"`       // 身体属性
}

// PresetMetadata 表示仅包含略缩图的的角色预设数据，不包含实际数据
type PresetMetadata struct {
	Signature   string `json:"Signature"`   // "CM3D2_PRESET"
	Version     int32  `json:"Version"`     // 版本号
	PresetType  int32  `json:"PresetType"`  // 预设类型：0=衣服, 1=身体, 2=全部
	ThumbLength int32  `json:"ThumbLength"` // 略缩图数据长度
	ThumbData   []byte `json:"ThumbData"`   // 略缩图数据，PNG格式
}

// PresetPropertyList 表示预设属性列表
type PresetPropertyList struct {
	Signature          string                    `json:"Signature"`               // "CM3D2_MPROP_LIST"
	Version            int32                     `json:"Version"`                 // 版本号
	PropertyCount      int32                     `json:"PropertyCount"`           // 属性数量
	PresetProperties   map[string]PresetProperty `json:"PresetProperties"`        // 属性映射表
	PropertyOrder      []string                  `json:"PropertyOrder,omitempty"` // 保留 wire 中主属性的顺序
	MaidPropOther      []NamedPresetProperty     `json:"MaidPropOther"`           // COM3D2.5 扩展属性
	PartsColorOtherBin []byte                    `json:"PartsColorOtherBin"`      // COM3D2.5 扩展颜色块（原始字节）
	CRCPresetBin       []byte                    `json:"CRCPresetBin"`            // COM3D2.5 CRC 预设块（原始字节）
}

// NamedPresetProperty 保留 MPROP_LIST 中属性前置键及其顺序。
type NamedPresetProperty struct {
	Key      string         `json:"Key"`
	Property PresetProperty `json:"Property"`
}

// PresetProperty 表示单个属性
type PresetProperty struct {
	Signature                string                               `json:"Signature"`     // "CM3D2_MPROP"
	Version                  int32                                `json:"Version"`       // 版本号
	Index                    int32                                `json:"Index"`         // 索引
	Name                     string                               `json:"Name"`          // 名称
	Type                     int32                                `json:"Type"`          // 类型
	DefaultValue             int32                                `json:"DefaultValue"`  // 默认值
	Value                    int32                                `json:"Value"`         // 当前值
	TempValue                int32                                `json:"TempValue"`     // 临时值
	LinkMaxValue             int32                                `json:"LinkMaxValue"`  // 链接最大值
	FileName                 string                               `json:"FileName"`      // 文件名
	FileNameRID              int32                                `json:"FileNameRID"`   // 文件名哈希值  this.strFileName.ToLower().GetHashCode();
	IsDut                    bool                                 `json:"IsDut"`         // 是否使用
	Max                      int32                                `json:"Max"`           // 最大值
	Min                      int32                                `json:"Min"`           // 最小值
	SubProps                 []*SubProp                           `json:"SubProps"`      // 子属性列表；nil 元素对应 wire 中的 exists=false
	SkinPositions            map[int]BoneAttachPosEntry           `json:"SkinPositions"` // 皮肤位置 slotID -> (RID, BoneAttachPos)
	SkinPositionOrder        []int                                `json:"SkinPositionOrder,omitempty"`
	AttachPositions          map[int]map[string]VtxAttachPosEntry `json:"AttachPositions"` // 附件位置 slotID -> name -> (RID, VtxAttachPos)
	AttachPositionOrder      []int                                `json:"AttachPositionOrder,omitempty"`
	AttachPositionNameOrders map[int][]string                     `json:"AttachPositionNameOrders,omitempty"`
	// AttachPositionSlotNames preserves the COM3D2.5 v2003+ SlotID name
	// written once per outer AttachPositions entry, including an empty map.
	AttachPositionSlotNames map[int]string           `json:"AttachPositionSlotNames,omitempty"`
	MaterialProps           map[int]MatPropSaveEntry `json:"MaterialProps"` // 材质属性 slotID -> (RID, MatPropSave)
	MaterialPropOrder       []int                    `json:"MaterialPropOrder,omitempty"`
	BoneLengths             map[int]BoneLengthEntry  `json:"BoneLengths"` // 骨骼长度 slotID -> (RID, map[name]len)
	BoneLengthOrder         []int                    `json:"BoneLengthOrder,omitempty"`
	IsCrcParts              bool                     `json:"IsCrcParts"` // CRC/GP03 部件标记
}

type BoneAttachPosEntry struct {
	SlotName      string        `json:"SlotName,omitempty"`
	RID           int32         `json:"RID"`           // C# KeyValuePair<int, BoneAttachPos>.Key
	BoneAttachPos BoneAttachPos `json:"BoneAttachPos"` // C# ...Value
}

type VtxAttachPosEntry struct {
	RID          int32        `json:"RID"`
	VtxAttachPos VtxAttachPos `json:"VtxAttachPos"`
}

type MatPropSaveEntry struct {
	SlotName    string      `json:"SlotName,omitempty"`
	RID         int32       `json:"RID"`
	MatPropSave MatPropSave `json:"MatPropSave"`
}

type BoneLengthEntry struct {
	SlotName    string             `json:"SlotName,omitempty"`
	RID         int32              `json:"RID"`
	Lengths     map[string]float32 `json:"Lengths"`
	LengthOrder []string           `json:"LengthOrder,omitempty"`
}

// SubProp 表示子属性
type SubProp struct {
	IsDut       bool    `json:"IsDut"`       // 是否使用
	FileName    string  `json:"FileName"`    // 文件名
	FileNameRID int32   `json:"FileNameRID"` // 文件名哈希值
	TexMulAlpha float32 `json:"TexMulAlpha"` // 纹理乘法透明度
}

// BoneAttachPos 表示骨骼附着位置
type BoneAttachPos struct {
	Enable      bool                  `json:"Enable"`                // 是否启用
	PosRotScale PositionRotationScale `json:"PositionRotationScale"` // 位置、旋转、缩放
}

// VtxAttachPos 表示顶点附着位置
type VtxAttachPos struct {
	Enable      bool                  `json:"Enable"`                // 是否启用
	VtxCount    int32                 `json:"VtxCount"`              // 顶点数量
	VtxIdx      int32                 `json:"VtxIdx"`                // 顶点索引
	PosRotScale PositionRotationScale `json:"PositionRotationScale"` // 位置、旋转、缩放
}

// MatPropSave 表示材质属性保存
type MatPropSave struct {
	MatId    int32  `json:"MatId"`    // 材质编号
	PropName string `json:"PropName"` // 属性名称
	TypeName string `json:"TypeName"` // 类型名称
	Value    string `json:"Value"`    // 属性值
}

// MultiColor 表示多颜色设置
type MultiColor struct {
	Signature   string       `json:"Signature"`   // "CM3D2_MULTI_COL"
	Version     int32        `json:"Version"`     // 版本号
	PartCount   int32        `json:"PartCount"`   // wire 头部颜色数量；新布局读取时游戏忽略该值 / Wire header count; ignored by the new-layout reader
	PartNames   []string     `json:"PartNames"`   // 新布局中每项的原始名称及顺序；旧布局无名称 / Raw names/order in the new layout; absent in the legacy layout
	PartsColors []PartsColor `json:"PartsColors"` // wire 上实际存在的颜色，不展开游戏默认值 / Colors physically present on the wire, without game defaults
}

// PartsColor 表示部件颜色
type PartsColor struct {
	IsUse            bool  `json:"IsUse"`            // 是否使用
	MainHue          int32 `json:"MainHue"`          // 主色相
	MainChroma       int32 `json:"MainChroma"`       // 主色度
	MainBrightness   int32 `json:"MainBrightness"`   // 主亮度
	MainContrast     int32 `json:"MainContrast"`     // 主对比度
	ShadowRate       int32 `json:"ShadowRate"`       // 阴影比例
	ShadowHue        int32 `json:"ShadowHue"`        // 阴影色相
	ShadowChroma     int32 `json:"ShadowChroma"`     // 阴影色度
	ShadowBrightness int32 `json:"ShadowBrightness"` // 阴影亮度
	ShadowContrast   int32 `json:"ShadowContrast"`   // 阴影对比度
}

// BodyProperty 表示身体属性
type BodyProperty struct {
	Signature string `json:"Signature"` // "CM3D2_MAID_BODY"
	Version   int32  `json:"Version"`   // 版本号
	// 是的确实没有别的东西
}

func validatePresetSignature(field, got, want string) error {
	if got != want {
		return fmt.Errorf("invalid %s signature: expected %q, got %q", field, want, got)
	}
	return nil
}

func validatePresetCount(field string, count int32) error {
	if count < 0 {
		return fmt.Errorf("invalid %s: %d", field, count)
	}
	return nil
}

func presetPropertyHasIsCrcParts(version int32) bool {
	return (version >= 2001 && version < 20000) || version >= 30000
}

func presetPropertyHasSlotNames(version int32) bool {
	return (version >= 2003 && version < 20000) || version >= 30000
}

func presetPropertyListHasExtensions(version int32) bool {
	return (version >= 2001 && version < 20000) || version >= 30000
}

func readPresetByteBlock(reader *stream.BinaryReader, field string) ([]byte, error) {
	length, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read %s length failed: %w", field, err)
	}
	if err := validatePresetCount(field, length); err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}
	data, err := reader.ReadBytes(int(length))
	if err != nil {
		return nil, fmt.Errorf("read %s data failed: %w", field, err)
	}
	return data, nil
}

func writePresetByteBlock(writer *stream.BinaryWriter, field string, data []byte) error {
	length, err := collectionCountInt32(field+" length", len(data))
	if err != nil {
		return err
	}
	if err := writer.WriteInt32(length); err != nil {
		return fmt.Errorf("write %s length failed: %w", field, err)
	}
	if len(data) != 0 {
		if err := writer.WriteBytes(data); err != nil {
			return fmt.Errorf("write %s data failed: %w", field, err)
		}
	}
	return nil
}

func validatePresetMapLength(field string, length int) error {
	_, err := collectionCountInt32(field, length)
	return err
}

func validatePresetSlotID(field string, slot int) error {
	if int64(slot) < -1<<31 || int64(slot) > 1<<31-1 {
		return fmt.Errorf("invalid %s slotID: %d is outside Int32", field, slot)
	}
	return nil
}

func readPresetSlot(reader *stream.BinaryReader, version int32, field string) (int, string, error) {
	slotID, err := reader.ReadInt32()
	if err != nil {
		return 0, "", fmt.Errorf("read %s slotID failed: %w", field, err)
	}
	var slotName string
	if presetPropertyHasSlotNames(version) {
		slotName, err = reader.ReadString()
		if err != nil {
			return 0, "", fmt.Errorf("read %s slotName failed: %w", field, err)
		}
	}
	return int(slotID), slotName, nil
}

// ReadPreset 从 r 中读取 Preset
func ReadPreset(r io.Reader) (*Preset, error) {
	reader := stream.NewBinaryReader(r)
	p, err := readPreset(reader)
	if err != nil {
		return nil, err
	}
	if _, err := reader.ReadByte(); err == nil {
		return nil, fmt.Errorf("read .Preset failed: unexpected trailing data")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("verify .Preset trailing data failed: %w", err)
	}
	return p, nil
}

// readPreset 解析一个 preset 对象但不检查 EOF，供有明确外层边界的内部格式复用。
func readPreset(reader *stream.BinaryReader) (*Preset, error) {
	p := &Preset{}

	// 1. signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .Preset signature failed: %w", err)
	}
	if err := validatePresetSignature(".Preset", sig, PresetSignature); err != nil {
		return nil, err
	}
	p.Signature = sig

	// 2. version
	version, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset version failed: %w", err)
	}
	p.Version = version

	// 3. presetType
	presetType, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset presetType failed: %w", err)
	}
	p.PresetType = presetType

	// 4. ThumbLength
	thumbLength, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset ThumbLength failed: %w", err)
	}
	if err := validatePresetCount(".Preset ThumbLength", thumbLength); err != nil {
		return nil, err
	}
	p.ThumbLength = thumbLength

	// 5. ThumbData
	if p.ThumbLength > 0 {
		p.ThumbData, err = reader.ReadBytes(int(p.ThumbLength))
		if err != nil {
			return nil, fmt.Errorf("read .Preset ThumbData failed: %w", err)
		}
	}

	// 6. listMprop
	p.PresetPropertyList, err = readPresetPropertyList(reader)
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList failed: %w", err)
	}

	// 7. MultiColor
	if version >= 2 && (version < 2000 || 10000 <= version) {
		mc, err := readMultiColor(reader)
		if err != nil {
			return nil, fmt.Errorf("read .Preset MultiColor failed: %w", err)
		}
		p.MultiColor = mc
	}

	// 8. Body
	if version >= 200 && (version < 2000 || 10000 <= version) {
		bp, err := readBodyProperty(reader)
		if err != nil {
			return nil, fmt.Errorf("read .Preset Body failed: %w", err)
		}
		p.BodyProperty = bp
	}

	return p, nil
}

// ReadPresetMetadata 从 r 中读取 PresetMetadata
func ReadPresetMetadata(reader *stream.BinaryReader) (*PresetMetadata, error) {
	p := &PresetMetadata{}

	// 1. signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .Preset signature failed: %w", err)
	}
	if err := validatePresetSignature(".Preset", sig, PresetSignature); err != nil {
		return nil, err
	}
	p.Signature = sig

	// 2. version
	version, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset version failed: %w", err)
	}
	p.Version = version

	// 3. presetType
	presetType, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset presetType failed: %w", err)
	}
	p.PresetType = presetType

	// 4. ThumbLength
	thumbLength, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset ThumbLength failed: %w", err)
	}
	if err := validatePresetCount(".Preset ThumbLength", thumbLength); err != nil {
		return nil, err
	}
	p.ThumbLength = thumbLength

	// 5. ThumbData
	if p.ThumbLength > 0 {
		p.ThumbData, err = reader.ReadBytes(int(p.ThumbLength))
		if err != nil {
			return nil, fmt.Errorf("read .Preset ThumbData failed: %w", err)
		}
	}

	return p, nil
}

// readPresetPropertyList 从 r 中读取 PresetPropertyList
func readPresetPropertyList(reader *stream.BinaryReader) (*PresetPropertyList, error) {
	ppl := &PresetPropertyList{PresetProperties: map[string]PresetProperty{}}

	// 1. signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList signature failed: %w", err)
	}
	if err := validatePresetSignature("PresetPropertyList", sig, PresetPropertyListSignature); err != nil {
		return nil, err
	}
	ppl.Signature = sig

	// 2. version
	version, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList version failed: %w", err)
	}
	ppl.Version = version

	//3. PropertyCount
	count, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList propertyCount failed: %w", err)
	}
	if err := validatePresetCount("PresetPropertyList propertyCount", count); err != nil {
		return nil, err
	}
	ppl.PropertyCount = count

	// 4. PresetProperties
	for i := 0; i < int(count); i++ {
		var key string
		hasStoredKey := version >= 4
		if hasStoredKey {
			// 新版：SerializeProp 会先写 key（MPN 名称字符串）
			k, err := reader.ReadString()
			if err != nil {
				return nil, fmt.Errorf("read Prop key[%d] failed: %w", i, err)
			}
			key = k
		}
		prop, err := readPresetProperty(reader)
		if err != nil {
			return nil, fmt.Errorf("read Prop[%d] failed: %w", i, err)
		}
		if !hasStoredKey {
			// 旧版未写 key，用 prop.Name 作为 key
			key = prop.Name
		}
		if _, exists := ppl.PresetProperties[key]; exists {
			return nil, fmt.Errorf("read Prop[%d] failed: duplicate property key %q", i, key)
		}
		ppl.PresetProperties[key] = *prop
		ppl.PropertyOrder = append(ppl.PropertyOrder, key)
	}

	if presetPropertyListHasExtensions(version) {
		otherCount, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read MaidPropOther count failed: %w", err)
		}
		if err := validatePresetCount("MaidPropOther count", otherCount); err != nil {
			return nil, err
		}
		if otherCount > 0 {
			ppl.MaidPropOther = makeCountedSliceForAppend[NamedPresetProperty](otherCount)
		}
		for i := int32(0); i < otherCount; i++ {
			key, err := reader.ReadString()
			if err != nil {
				return nil, fmt.Errorf("read MaidPropOther[%d] key failed: %w", i, err)
			}
			prop, err := readPresetProperty(reader)
			if err != nil {
				return nil, fmt.Errorf("read MaidPropOther[%d] failed: %w", i, err)
			}
			ppl.MaidPropOther = append(ppl.MaidPropOther, NamedPresetProperty{Key: key, Property: *prop})
		}
		ppl.PartsColorOtherBin, err = readPresetByteBlock(reader, "PartsColorOtherBin")
		if err != nil {
			return nil, err
		}
		ppl.CRCPresetBin, err = readPresetByteBlock(reader, "CRCPresetBin")
		if err != nil {
			return nil, err
		}
	}

	return ppl, nil
}

// 读取单个属性：MaidProp.Deserialize（格式对齐 MaidProp.Serialize）
func readPresetProperty(reader *stream.BinaryReader) (*PresetProperty, error) {
	prop := &PresetProperty{}

	// 1. signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetProperty signature failed: %w", err)
	}
	if err := validatePresetSignature("PresetProperty", sig, PresetPropertySignature); err != nil {
		return nil, err
	}
	prop.Signature = sig

	// 2. version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetProperty version failed: %w", err)
	}
	prop.Version = ver

	// 3. index
	idx, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read prop.idx failed: %w", err)
	}
	prop.Index = idx

	// 4. name
	name, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read prop.name failed: %w", err)
	}
	prop.Name = name

	// 5. 基本数值
	if prop.Type, err = reader.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read prop.type failed: %w", err)
	}
	if prop.DefaultValue, err = reader.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read prop.default failed: %w", err)
	}
	if prop.Value, err = reader.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read prop.value failed: %w", err)
	}
	if ver >= 101 {
		if prop.TempValue, err = reader.ReadInt32(); err != nil {
			return nil, fmt.Errorf("read prop.temp_value failed: %w", err)
		}
	}
	if prop.LinkMaxValue, err = reader.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read prop.linkMax failed: %w", err)
	}
	if prop.FileName, err = reader.ReadString(); err != nil {
		return nil, fmt.Errorf("read prop.fileName failed: %w", err)
	}
	if prop.FileNameRID, err = reader.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read prop.fileNameRID failed: %w", err)
	}
	if prop.IsDut, err = reader.ReadBool(); err != nil {
		return nil, fmt.Errorf("read prop.isDut failed: %w", err)
	}
	if prop.Max, err = reader.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read prop.max failed: %w", err)
	}
	if prop.Min, err = reader.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read prop.min failed: %w", err)
	}

	// 子属性（ver >= 200）
	if ver >= 200 {
		cnt, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read subProp count failed: %w", err)
		}
		if err := validatePresetCount("subProp count", cnt); err != nil {
			return nil, err
		}
		if cnt > 0 {
			prop.SubProps = makeCountedSliceForAppend[*SubProp](cnt)
		}
		for i := 0; i < int(cnt); i++ {
			exists, err := reader.ReadBool()
			if err != nil {
				return nil, fmt.Errorf("read subProp[%d] exists failed: %w", i, err)
			}
			if !exists {
				prop.SubProps = append(prop.SubProps, nil)
				continue
			}
			sp := &SubProp{}
			if sp.IsDut, err = reader.ReadBool(); err != nil {
				return nil, fmt.Errorf("read subProp[%d].IsDut failed: %w", i, err)
			}
			if sp.FileName, err = reader.ReadString(); err != nil {
				return nil, fmt.Errorf("read subProp[%d].FileName failed: %w", i, err)
			}
			if sp.FileNameRID, err = reader.ReadInt32(); err != nil {
				return nil, fmt.Errorf("read subProp[%d].FileNameRID failed: %w", i, err)
			}
			if ver >= 211 {
				if sp.TexMulAlpha, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read subProp[%d].TexMulAlpha failed: %w", i, err)
				}
			}
			prop.SubProps = append(prop.SubProps, sp)
		}

		// 皮肤位置：slotID, RID, BoneAttachPos
		nSkin, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read skinPos count failed: %w", err)
		}
		if err := validatePresetCount("skinPos count", nSkin); err != nil {
			return nil, err
		}
		if nSkin > 0 {
			prop.SkinPositions = makeCountedMap[int, BoneAttachPosEntry](nSkin)
		}
		for i := 0; i < int(nSkin); i++ {
			slotID, slotName, err := readPresetSlot(reader, ver, fmt.Sprintf("skinPos[%d]", i))
			if err != nil {
				return nil, err
			}
			if _, exists := prop.SkinPositions[slotID]; exists {
				return nil, fmt.Errorf("read skinPos[%d] failed: duplicate slotID %d", i, slotID)
			}
			rid, err := reader.ReadInt32()
			if err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rid failed: %w", i, err)
			}
			var b BoneAttachPos
			if b.Enable, err = reader.ReadBool(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].enable failed: %w", i, err)
			}
			if b.PosRotScale.Position.X, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].pos.x failed: %w", i, err)
			}
			if b.PosRotScale.Position.Y, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].pos.y failed: %w", i, err)
			}
			if b.PosRotScale.Position.Z, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].pos.z failed: %w", i, err)
			}
			if b.PosRotScale.Rotation.X, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rot.x failed: %w", i, err)
			}
			if b.PosRotScale.Rotation.Y, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rot.y failed: %w", i, err)
			}
			if b.PosRotScale.Rotation.Z, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rot.z failed: %w", i, err)
			}
			if b.PosRotScale.Rotation.W, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rot.w failed: %w", i, err)
			}
			if b.PosRotScale.Scale.X, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].scale.x failed: %w", i, err)
			}
			if b.PosRotScale.Scale.Y, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].scale.y failed: %w", i, err)
			}
			if b.PosRotScale.Scale.Z, err = reader.ReadFloat32(); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].scale.z failed: %w", i, err)
			}

			prop.SkinPositions[slotID] = BoneAttachPosEntry{SlotName: slotName, RID: rid, BoneAttachPos: b}
			prop.SkinPositionOrder = append(prop.SkinPositionOrder, slotID)
		}

		// 附着位置：slotID, count, (name, RID, VtxAttachPos)*
		nAttach, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read attachPos count failed: %w", err)
		}
		if err := validatePresetCount("attachPos count", nAttach); err != nil {
			return nil, err
		}
		if nAttach > 0 {
			prop.AttachPositions = makeCountedMap[int, map[string]VtxAttachPosEntry](nAttach)
			prop.AttachPositionNameOrders = makeCountedMap[int, []string](nAttach)
			if presetPropertyHasSlotNames(ver) {
				prop.AttachPositionSlotNames = makeCountedMap[int, string](nAttach)
			}
		}
		for i := 0; i < int(nAttach); i++ {
			slotID, slotName, err := readPresetSlot(reader, ver, fmt.Sprintf("attachPos[%d]", i))
			if err != nil {
				return nil, err
			}
			if _, exists := prop.AttachPositions[slotID]; exists {
				return nil, fmt.Errorf("read attachPos[%d] failed: duplicate slotID %d", i, slotID)
			}
			inner, err := reader.ReadInt32()
			if err != nil {
				return nil, fmt.Errorf("read attachPos[%d].innerCount failed: %w", i, err)
			}
			if err := validatePresetCount("attachPos inner count", inner); err != nil {
				return nil, fmt.Errorf("read attachPos[%d] failed: %w", i, err)
			}
			mp := makeCountedMap[string, VtxAttachPosEntry](inner)
			nameOrder := makeCountedSliceForAppend[string](inner)
			for j := 0; j < int(inner); j++ {
				key, err := reader.ReadString()
				if err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].key failed: %w", i, j, err)
				}
				rid, err := reader.ReadInt32()
				if err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rid failed: %w", i, j, err)
				}
				var v VtxAttachPos
				if v.Enable, err = reader.ReadBool(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].enable failed: %w", i, j, err)
				}
				if v.VtxCount, err = reader.ReadInt32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].vtxCount failed: %w", i, j, err)
				}
				if v.VtxIdx, err = reader.ReadInt32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].vtxIdx failed: %w", i, j, err)
				}
				if v.PosRotScale.Position.X, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].pos.x failed: %w", i, j, err)
				}
				if v.PosRotScale.Position.Y, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].pos.y failed: %w", i, j, err)
				}
				if v.PosRotScale.Position.Z, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].pos.z failed: %w", i, j, err)
				}
				if v.PosRotScale.Rotation.X, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rot.x failed: %w", i, j, err)
				}
				if v.PosRotScale.Rotation.Y, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rot.y failed: %w", i, j, err)
				}
				if v.PosRotScale.Rotation.Z, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rot.z failed: %w", i, j, err)
				}
				if v.PosRotScale.Rotation.W, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rot.w failed: %w", i, j, err)
				}
				if v.PosRotScale.Scale.X, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].scale.x failed: %w", i, j, err)
				}
				if v.PosRotScale.Scale.Y, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].scale.y failed: %w", i, j, err)
				}
				if v.PosRotScale.Scale.Z, err = reader.ReadFloat32(); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].scale.z failed: %w", i, j, err)
				}

				if _, exists := mp[key]; exists {
					return nil, fmt.Errorf("read attachPos[%d][%d] failed: duplicate key %q", i, j, key)
				}
				mp[key] = VtxAttachPosEntry{RID: rid, VtxAttachPos: v}
				nameOrder = append(nameOrder, key)
			}
			prop.AttachPositions[slotID] = mp
			prop.AttachPositionOrder = append(prop.AttachPositionOrder, slotID)
			prop.AttachPositionNameOrders[slotID] = nameOrder
			if presetPropertyHasSlotNames(ver) {
				prop.AttachPositionSlotNames[slotID] = slotName
			}
		}

		// 材质属性：slotID, RID, MatPropSave
		nMat, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read matProp count failed: %w", err)
		}
		if err := validatePresetCount("matProp count", nMat); err != nil {
			return nil, err
		}
		if nMat > 0 {
			prop.MaterialProps = makeCountedMap[int, MatPropSaveEntry](nMat)
		}
		for i := 0; i < int(nMat); i++ {
			slotID, slotName, err := readPresetSlot(reader, ver, fmt.Sprintf("matProp[%d]", i))
			if err != nil {
				return nil, err
			}
			if _, exists := prop.MaterialProps[slotID]; exists {
				return nil, fmt.Errorf("read matProp[%d] failed: duplicate slotID %d", i, slotID)
			}
			rid, err := reader.ReadInt32()
			if err != nil {
				return nil, fmt.Errorf("read matProp[%d].rid failed: %w", i, err)
			}
			var m MatPropSave
			if m.MatId, err = reader.ReadInt32(); err != nil {
				return nil, fmt.Errorf("read matProp[%d].matId failed: %w", i, err)
			}
			if m.PropName, err = reader.ReadString(); err != nil {
				return nil, fmt.Errorf("read matProp[%d].propName failed: %w", i, err)
			}
			if m.TypeName, err = reader.ReadString(); err != nil {
				return nil, fmt.Errorf("read matProp[%d].typeName failed: %w", i, err)
			}
			if m.Value, err = reader.ReadString(); err != nil {
				return nil, fmt.Errorf("read matProp[%d].value failed: %w", i, err)
			}
			prop.MaterialProps[slotID] = MatPropSaveEntry{SlotName: slotName, RID: rid, MatPropSave: m}
			prop.MaterialPropOrder = append(prop.MaterialPropOrder, slotID)
		}

		// 骨骼长度（ver >= 213）：slotID, RID, count, (name,float)*
		if ver >= 213 {
			nBone, err := reader.ReadInt32()
			if err != nil {
				return nil, fmt.Errorf("read boneLen count failed: %w", err)
			}
			if err := validatePresetCount("boneLen count", nBone); err != nil {
				return nil, err
			}
			if nBone > 0 {
				prop.BoneLengths = makeCountedMap[int, BoneLengthEntry](nBone)
			}
			for i := 0; i < int(nBone); i++ {
				slotID, slotName, err := readPresetSlot(reader, ver, fmt.Sprintf("boneLen[%d]", i))
				if err != nil {
					return nil, err
				}
				if _, exists := prop.BoneLengths[slotID]; exists {
					return nil, fmt.Errorf("read boneLen[%d] failed: duplicate slotID %d", i, slotID)
				}
				rid, err := reader.ReadInt32()
				if err != nil {
					return nil, fmt.Errorf("read boneLen[%d].rid failed: %w", i, err)
				}
				inner, err := reader.ReadInt32()
				if err != nil {
					return nil, fmt.Errorf("read boneLen[%d].inner failed: %w", i, err)
				}
				if err := validatePresetCount("boneLen inner count", inner); err != nil {
					return nil, fmt.Errorf("read boneLen[%d] failed: %w", i, err)
				}
				m := makeCountedMap[string, float32](inner)
				lengthOrder := makeCountedSliceForAppend[string](inner)
				for j := 0; j < int(inner); j++ {
					k, err := reader.ReadString()
					if err != nil {
						return nil, fmt.Errorf("read boneLen[%d][%d].name failed: %w", i, j, err)
					}
					v, err := reader.ReadFloat32()
					if err != nil {
						return nil, fmt.Errorf("read boneLen[%d][%d].value failed: %w", i, j, err)
					}
					if _, exists := m[k]; exists {
						return nil, fmt.Errorf("read boneLen[%d][%d] failed: duplicate key %q", i, j, k)
					}
					m[k] = v
					lengthOrder = append(lengthOrder, k)
				}
				prop.BoneLengths[slotID] = BoneLengthEntry{SlotName: slotName, RID: rid, Lengths: m, LengthOrder: lengthOrder}
				prop.BoneLengthOrder = append(prop.BoneLengthOrder, slotID)
			}
		}
	}

	if presetPropertyHasIsCrcParts(ver) {
		if prop.IsCrcParts, err = reader.ReadBool(); err != nil {
			return nil, fmt.Errorf("read prop.isCrcParts failed: %w", err)
		}
	}

	return prop, nil
}

// 读取多颜色 wire，不执行 MaidParts.DeserializePre 的默认色展开。
func readMultiColor(reader *stream.BinaryReader) (*MultiColor, error) {
	mc := &MultiColor{}

	signature, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read MultiColor signature failed: %w", err)
	}
	if err := validatePresetSignature("MultiColor", signature, MultiColorSignature); err != nil {
		return nil, err
	}
	mc.Signature = signature

	version, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read MultiColor version failed: %w", err)
	}
	mc.Version = version

	count, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read MultiColor count failed: %w", err)
	}
	if err := validatePresetCount("MultiColor count", count); err != nil {
		return nil, err
	}
	mc.PartCount = count

	if version <= 1200 {
		if count > 0 {
			mc.PartsColors = makeCountedSliceForAppend[PartsColor](count)
		}
		for index := int32(0); index < count; index++ {
			var color PartsColor
			if _, err := readPartsColor(reader, &color); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d] failed: %w", index, err)
			}
			mc.PartsColors = append(mc.PartsColors, color)
		}
		return mc, nil
	}

	// The current layout is terminated by MAX and does not use PartCount to
	// bound this loop. Retain names, duplicates, order, and future enum names.
	for entry := int32(0); ; entry++ {
		name, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("read MultiColor entry name failed: %w", err)
		}
		if name == "MAX" {
			break
		}
		var color PartsColor
		if _, err := readPartsColor(reader, &color); err != nil {
			return nil, fmt.Errorf("read MultiColor[%d] failed: %w", entry, err)
		}
		mc.PartNames = append(mc.PartNames, name)
		mc.PartsColors = append(mc.PartsColors, color)
	}
	return mc, nil
}

func readPartsColor(reader *stream.BinaryReader, pc *PartsColor) (int, error) {
	var err error
	if pc.IsUse, err = reader.ReadBool(); err != nil {
		return 0, fmt.Errorf("read isUse failed: %w", err)
	}
	if pc.MainHue, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read mainHue failed: %w", err)
	}
	if pc.MainChroma, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read mainChroma failed: %w", err)
	}
	if pc.MainBrightness, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read mainBrightness failed: %w", err)
	}
	if pc.MainContrast, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read mainContrast failed: %w", err)
	}
	if pc.ShadowRate, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read shadowRate failed: %w", err)
	}
	if pc.ShadowHue, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read shadowHue failed: %w", err)
	}
	if pc.ShadowChroma, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read shadowChroma failed: %w", err)
	}
	if pc.ShadowBrightness, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read shadowBrightness failed: %w", err)
	}
	if pc.ShadowContrast, err = reader.ReadInt32(); err != nil {
		return 0, fmt.Errorf("read shadowContrast failed: %w", err)
	}
	return 10, nil
}

// 读取身体块：Maid.DeserializeBodyRead
func readBodyProperty(reader *stream.BinaryReader) (*BodyProperty, error) {
	bp := &BodyProperty{}
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read Body signature failed: %w", err)
	}
	if err := validatePresetSignature("Body", sig, BodyPropertySignature); err != nil {
		return nil, err
	}
	bp.Signature = sig
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read Body version failed: %w", err)
	}
	bp.Version = ver
	// 该块目前没有更多字段
	return bp, nil
}

func orderedPresetPropertyKeys(ppl *PresetPropertyList) ([]string, error) {
	if err := validatePresetMapLength("PresetPropertyList propertyCount", len(ppl.PresetProperties)); err != nil {
		return nil, err
	}
	return utilities.MergeOrderedMapKeys(ppl.PresetProperties, ppl.PropertyOrder, "PresetPropertyList PropertyOrder")
}

func orderedPresetIntMapKeys[V any](values map[int]V, order []int, label string) ([]int, error) {
	return utilities.MergeOrderedMapKeys(values, order, label+" order")
}

func orderedPresetStringMapKeys[V any](values map[string]V, order []string, label string) ([]string, error) {
	return utilities.MergeOrderedMapKeys(values, order, label+" order")
}

func validatePresetPropertyForDump(pp *PresetProperty) error {
	if pp == nil {
		return fmt.Errorf("PresetProperty is nil")
	}
	if err := validatePresetSignature("PresetProperty", pp.Signature, PresetPropertySignature); err != nil {
		return err
	}
	if pp.Version < 101 && pp.TempValue != 0 {
		return fmt.Errorf("TempValue cannot be encoded by PresetProperty version %d", pp.Version)
	}
	if pp.Version < 200 {
		if len(pp.SubProps) != 0 || len(pp.SkinPositions) != 0 || len(pp.SkinPositionOrder) != 0 ||
			len(pp.AttachPositions) != 0 || len(pp.AttachPositionOrder) != 0 || len(pp.AttachPositionNameOrders) != 0 || len(pp.AttachPositionSlotNames) != 0 ||
			len(pp.MaterialProps) != 0 || len(pp.MaterialPropOrder) != 0 || len(pp.BoneLengths) != 0 || len(pp.BoneLengthOrder) != 0 {
			return fmt.Errorf("PresetProperty version %d cannot encode version-200 extension data", pp.Version)
		}
		if !presetPropertyHasIsCrcParts(pp.Version) && pp.IsCrcParts {
			return fmt.Errorf("IsCrcParts cannot be encoded by PresetProperty version %d", pp.Version)
		}
		return nil
	}
	if err := validatePresetMapLength("subProp count", len(pp.SubProps)); err != nil {
		return err
	}
	if err := validatePresetMapLength("skinPos count", len(pp.SkinPositions)); err != nil {
		return err
	}
	if err := validatePresetMapLength("attachPos count", len(pp.AttachPositions)); err != nil {
		return err
	}
	if err := validatePresetMapLength("matProp count", len(pp.MaterialProps)); err != nil {
		return err
	}
	if pp.Version < 213 && len(pp.BoneLengths) != 0 {
		return fmt.Errorf("BoneLengths requires PresetProperty version >= 213")
	}
	for index, subProp := range pp.SubProps {
		if subProp != nil && pp.Version < 211 && subProp.TexMulAlpha != 0 {
			return fmt.Errorf("subProp[%d] TexMulAlpha requires PresetProperty version >= 211", index)
		}
	}
	hasSlotNames := presetPropertyHasSlotNames(pp.Version)
	validateSlot := func(field string, slot int, slotName string) error {
		if err := validatePresetSlotID(field, slot); err != nil {
			return err
		}
		if !hasSlotNames && slotName != "" {
			return fmt.Errorf("%s slotName requires PresetProperty version 2003-19999 or >= 30000", field)
		}
		return nil
	}
	skinSlots, err := orderedPresetIntMapKeys(pp.SkinPositions, pp.SkinPositionOrder, "SkinPositions")
	if err != nil {
		return err
	}
	for _, slot := range skinSlots {
		if err := validateSlot("skinPos", slot, pp.SkinPositions[slot].SlotName); err != nil {
			return err
		}
	}
	attachSlots, err := orderedPresetIntMapKeys(pp.AttachPositions, pp.AttachPositionOrder, "AttachPositions")
	if err != nil {
		return err
	}
	for _, slot := range attachSlots {
		if err := validateSlot("attachPos", slot, pp.AttachPositionSlotNames[slot]); err != nil {
			return err
		}
		values := pp.AttachPositions[slot]
		if err := validatePresetMapLength("attachPos inner count", len(values)); err != nil {
			return err
		}
		if _, err := orderedPresetStringMapKeys(values, pp.AttachPositionNameOrders[slot], fmt.Sprintf("AttachPositions[%d]", slot)); err != nil {
			return err
		}
	}
	materialSlots, err := orderedPresetIntMapKeys(pp.MaterialProps, pp.MaterialPropOrder, "MaterialProps")
	if err != nil {
		return err
	}
	for _, slot := range materialSlots {
		if err := validateSlot("matProp", slot, pp.MaterialProps[slot].SlotName); err != nil {
			return err
		}
	}
	if pp.Version >= 213 {
		if err := validatePresetMapLength("boneLen count", len(pp.BoneLengths)); err != nil {
			return err
		}
		boneSlots, err := orderedPresetIntMapKeys(pp.BoneLengths, pp.BoneLengthOrder, "BoneLengths")
		if err != nil {
			return err
		}
		for _, slot := range boneSlots {
			entry := pp.BoneLengths[slot]
			if err := validateSlot("boneLen", slot, entry.SlotName); err != nil {
				return err
			}
			if err := validatePresetMapLength("boneLen inner count", len(entry.Lengths)); err != nil {
				return err
			}
			if _, err := orderedPresetStringMapKeys(entry.Lengths, entry.LengthOrder, fmt.Sprintf("BoneLengths[%d].Lengths", slot)); err != nil {
				return err
			}
		}
	}
	if !presetPropertyHasIsCrcParts(pp.Version) && pp.IsCrcParts {
		return fmt.Errorf("IsCrcParts cannot be encoded by PresetProperty version %d", pp.Version)
	}
	return nil
}

func validatePresetPropertyListForDump(ppl *PresetPropertyList) ([]string, error) {
	if ppl == nil {
		return nil, fmt.Errorf("PresetPropertyList is nil")
	}
	if err := validatePresetSignature("PresetPropertyList", ppl.Signature, PresetPropertyListSignature); err != nil {
		return nil, err
	}
	keys, err := orderedPresetPropertyKeys(ppl)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		prop := ppl.PresetProperties[key]
		if ppl.Version < 4 && key != prop.Name {
			return nil, fmt.Errorf("PresetPropertyList version %d cannot encode property key %q separately from property name %q", ppl.Version, key, prop.Name)
		}
		if err := validatePresetPropertyForDump(&prop); err != nil {
			return nil, fmt.Errorf("property %q: %w", key, err)
		}
	}
	if presetPropertyListHasExtensions(ppl.Version) {
		if err := validatePresetMapLength("MaidPropOther count", len(ppl.MaidPropOther)); err != nil {
			return nil, err
		}
		for i := range ppl.MaidPropOther {
			if err := validatePresetPropertyForDump(&ppl.MaidPropOther[i].Property); err != nil {
				return nil, fmt.Errorf("MaidPropOther[%d]: %w", i, err)
			}
		}
		if _, err := collectionCountInt32("PartsColorOtherBin length", len(ppl.PartsColorOtherBin)); err != nil {
			return nil, err
		}
		if _, err := collectionCountInt32("CRCPresetBin length", len(ppl.CRCPresetBin)); err != nil {
			return nil, err
		}
	} else if len(ppl.MaidPropOther) != 0 || len(ppl.PartsColorOtherBin) != 0 || len(ppl.CRCPresetBin) != 0 {
		return nil, fmt.Errorf("PresetPropertyList version %d cannot encode COM3D2.5 extension data", ppl.Version)
	}
	return keys, nil
}

func validatePresetForDump(p *Preset) error {
	if p == nil {
		return fmt.Errorf("Preset is nil")
	}
	if err := validatePresetSignature(".Preset", p.Signature, PresetSignature); err != nil {
		return err
	}
	if _, err := collectionCountInt32(".Preset ThumbLength", len(p.ThumbData)); err != nil {
		return err
	}
	if _, err := validatePresetPropertyListForDump(p.PresetPropertyList); err != nil {
		return err
	}
	hasMultiColor := p.Version >= 2 && (p.Version < 2000 || 10000 <= p.Version)
	if hasMultiColor {
		if p.MultiColor == nil {
			return fmt.Errorf("MultiColor is nil for .Preset version %d", p.Version)
		}
		if err := validatePresetSignature("MultiColor", p.MultiColor.Signature, MultiColorSignature); err != nil {
			return err
		}
		if err := validateMultiColorForDump(p.MultiColor); err != nil {
			return err
		}
	} else if p.MultiColor != nil {
		return fmt.Errorf(".Preset version %d cannot encode MultiColor", p.Version)
	}
	hasBody := p.Version >= 200 && (p.Version < 2000 || 10000 <= p.Version)
	if hasBody {
		if p.BodyProperty == nil {
			return fmt.Errorf("BodyProperty is nil for .Preset version %d", p.Version)
		}
		if err := validatePresetSignature("Body", p.BodyProperty.Signature, BodyPropertySignature); err != nil {
			return err
		}
	} else if p.BodyProperty != nil {
		return fmt.Errorf(".Preset version %d cannot encode BodyProperty", p.Version)
	}
	return nil
}

func writePresetSlot(writer *stream.BinaryWriter, version int32, field string, slot int, slotName string) error {
	if err := validatePresetSlotID(field, slot); err != nil {
		return err
	}
	if err := writer.WriteInt32(int32(slot)); err != nil {
		return fmt.Errorf("write %s slotID failed: %w", field, err)
	}
	if presetPropertyHasSlotNames(version) {
		if err := writer.WriteString(slotName); err != nil {
			return fmt.Errorf("write %s slotName failed: %w", field, err)
		}
	}
	return nil
}

func (p *Preset) Dump(w io.Writer) error {
	if err := validatePresetForDump(p); err != nil {
		return fmt.Errorf("write .Preset failed: %w", err)
	}
	p.ThumbLength = int32(len(p.ThumbData))
	writer := stream.NewBinaryWriter(w)

	// 1. signature
	if err := writer.WriteString(p.Signature); err != nil {
		return fmt.Errorf("write preset signature failed: %w", err)
	}

	// 2. version
	if err := writer.WriteInt32(p.Version); err != nil {
		return fmt.Errorf("write preset version failed: %w", err)
	}

	// 3. presetType
	if err := writer.WriteInt32(p.PresetType); err != nil {
		return fmt.Errorf("write preset type failed: %w", err)
	}

	// 4. thumb
	if len(p.ThumbData) > 0 {
		if err := writer.WriteInt32(int32(len(p.ThumbData))); err != nil {
			return fmt.Errorf("write thumb length failed: %w", err)
		}
		if err := writer.WriteBytes(p.ThumbData); err != nil {
			return fmt.Errorf("write thumb data failed: %w", err)
		}
	} else {
		if err := writer.WriteInt32(0); err != nil {
			return fmt.Errorf("write thumb length(0) failed: %w", err)
		}
	}

	// 5. mprop list
	if err := dumpPresetPropertyList(writer, p.PresetPropertyList); err != nil {
		return fmt.Errorf("write PresetPropertyList failed: %w", err)
	}

	// 6. multicolor
	if p.Version >= 2 && (p.Version < 2000 || 10000 <= p.Version) {
		if err := dumpMultiColor(writer, p.MultiColor); err != nil {
			return fmt.Errorf("write MultiColor failed: %w", err)
		}
	}

	// 7. body
	if p.Version >= 200 && (p.Version < 2000 || 10000 <= p.Version) {
		if err := dumpBodyProperty(writer, p.BodyProperty); err != nil {
			return fmt.Errorf("write Body failed: %w", err)
		}
	}

	return nil
}

// 写入属性列表：Maid.SerializeProp
func dumpPresetPropertyList(writer *stream.BinaryWriter, ppl *PresetPropertyList) error {
	keys, err := validatePresetPropertyListForDump(ppl)
	if err != nil {
		return fmt.Errorf("write PresetPropertyList failed: %w", err)
	}
	ppl.PropertyCount = int32(len(keys))

	if err := writer.WriteString(ppl.Signature); err != nil {
		return fmt.Errorf("write preset property list signature failed: %w", err)
	}

	if err := writer.WriteInt32(ppl.Version); err != nil {
		return fmt.Errorf("write preset property list version failed: %w", err)
	}

	count := int32(len(keys))
	if err := writer.WriteInt32(count); err != nil {
		return fmt.Errorf("write preset property list count failed: %w", err)
	}

	for _, k := range keys {
		v := ppl.PresetProperties[k]
		// 仅当列表版本 >= 4 时写 key（MPN 字符串）
		if ppl.Version >= 4 {
			if err := writer.WriteString(k); err != nil {
				return fmt.Errorf("write prop key failed: %w", err)
			}
		}
		prop := v // copy
		if err := writePresetProperty(writer, &prop); err != nil {
			return fmt.Errorf("write prop '%s' failed: %w", k, err)
		}
	}

	if presetPropertyListHasExtensions(ppl.Version) {
		if err := writer.WriteInt32(int32(len(ppl.MaidPropOther))); err != nil {
			return fmt.Errorf("write MaidPropOther count failed: %w", err)
		}
		for i := range ppl.MaidPropOther {
			entry := &ppl.MaidPropOther[i]
			if err := writer.WriteString(entry.Key); err != nil {
				return fmt.Errorf("write MaidPropOther[%d] key failed: %w", i, err)
			}
			if err := writePresetProperty(writer, &entry.Property); err != nil {
				return fmt.Errorf("write MaidPropOther[%d] failed: %w", i, err)
			}
		}
		if err := writePresetByteBlock(writer, "PartsColorOtherBin", ppl.PartsColorOtherBin); err != nil {
			return err
		}
		if err := writePresetByteBlock(writer, "CRCPresetBin", ppl.CRCPresetBin); err != nil {
			return err
		}
	}

	return nil
}

// 写入单个属性：MaidProp.Serialize
func writePresetProperty(writer *stream.BinaryWriter, pp *PresetProperty) error {
	if err := validatePresetPropertyForDump(pp); err != nil {
		return fmt.Errorf("write PresetProperty failed: %w", err)
	}
	if err := writer.WriteString(pp.Signature); err != nil {
		return fmt.Errorf("write prop signature failed: %w", err)
	}
	ver := pp.Version
	if err := writer.WriteInt32(ver); err != nil {
		return fmt.Errorf("write prop version failed: %w", err)
	}
	if err := writer.WriteInt32(pp.Index); err != nil {
		return fmt.Errorf("write prop index failed: %w", err)
	}
	if err := writer.WriteString(pp.Name); err != nil {
		return fmt.Errorf("write prop name failed: %w", err)
	}
	if err := writer.WriteInt32(pp.Type); err != nil {
		return fmt.Errorf("write prop type failed: %w", err)
	}
	if err := writer.WriteInt32(pp.DefaultValue); err != nil {
		return fmt.Errorf("write prop default value failed: %w", err)
	}
	if err := writer.WriteInt32(pp.Value); err != nil {
		return fmt.Errorf("write prop value failed: %w", err)
	}
	// ver >= 101 才写 TempValue
	if ver >= 101 {
		if err := writer.WriteInt32(pp.TempValue); err != nil {
			return fmt.Errorf("write prop temp value failed: %w", err)
		}
	}
	if err := writer.WriteInt32(pp.LinkMaxValue); err != nil {
		return fmt.Errorf("write prop link max value failed: %w", err)
	}
	if err := writer.WriteString(pp.FileName); err != nil {
		return fmt.Errorf("write prop file name failed: %w", err)
	}
	if err := writer.WriteInt32(pp.FileNameRID); err != nil {
		return fmt.Errorf("write prop file name rid failed: %w", err)
	}
	if err := writer.WriteBool(pp.IsDut); err != nil {
		return fmt.Errorf("write prop is dut failed: %w", err)
	}
	if err := writer.WriteInt32(pp.Max); err != nil {
		return fmt.Errorf("write prop max failed: %w", err)
	}
	if err := writer.WriteInt32(pp.Min); err != nil {
		return fmt.Errorf("write prop min failed: %w", err)
	}
	// 仅当 ver >= 200 时才写入子属性与附加块（与读取保持一致）
	if ver >= 200 {
		// 子属性
		nSub := int32(len(pp.SubProps))
		if err := writer.WriteInt32(nSub); err != nil {
			return fmt.Errorf("write prop sub count failed: %w", err)
		}
		for i := 0; i < int(nSub); i++ {
			sp := pp.SubProps[i]
			exists := sp != nil
			if err := writer.WriteBool(exists); err != nil {
				return fmt.Errorf("write prop sub exists failed: %w", err)
			}
			if !exists {
				continue
			}
			if err := writer.WriteBool(sp.IsDut); err != nil {
				return fmt.Errorf("write prop sub is dut failed: %w", err)
			}
			if err := writer.WriteString(sp.FileName); err != nil {
				return fmt.Errorf("write prop sub file name failed: %w", err)
			}
			if err := writer.WriteInt32(sp.FileNameRID); err != nil {
				return fmt.Errorf("write prop sub file name rid failed: %w", err)
			}
			// ver >= 211 才写 TexMulAlpha
			if ver >= 211 {
				if err := writer.WriteFloat32(sp.TexMulAlpha); err != nil {
					return fmt.Errorf("write prop sub tex mul alpha failed: %w", err)
				}
			}
		}

		// 皮肤位置：PropertyCount -> [slotID, RID, data...]
		if len(pp.SkinPositions) == 0 {
			if err := writer.WriteInt32(0); err != nil {
				return fmt.Errorf("write prop skin position count failed: %w", err)
			}
		} else {
			if err := writer.WriteInt32(int32(len(pp.SkinPositions))); err != nil {
				return fmt.Errorf("write prop skin position count failed: %w", err)
			}
			slots, err := orderedPresetIntMapKeys(pp.SkinPositions, pp.SkinPositionOrder, "SkinPositions")
			if err != nil {
				return err
			}
			for _, slot := range slots {
				e := pp.SkinPositions[slot]
				if err := writePresetSlot(writer, ver, "skinPos", slot, e.SlotName); err != nil {
					return err
				}
				if err := writer.WriteInt32(e.RID); err != nil {
					return fmt.Errorf("write prop skin position rid failed: %w", err)
				}
				b := e.BoneAttachPos
				if err := writer.WriteBool(b.Enable); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos enable failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Position.X); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos position x failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Position.Y); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos position y failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Position.Z); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos position z failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Rotation.X); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos rotation x failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Rotation.Y); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos rotation y failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Rotation.Z); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos rotation z failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Rotation.W); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos rotation w failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Scale.X); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos scale x failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Scale.Y); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos scale y failed: %w", err)
				}
				if err := writer.WriteFloat32(b.PosRotScale.Scale.Z); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos scale z failed: %w", err)
				}
			}
		}

		// 附着位置：slotID -> map[name]Entry
		if len(pp.AttachPositions) == 0 {
			if err := writer.WriteInt32(0); err != nil {
				return fmt.Errorf("write prop attach position count failed: %w", err)
			}
		} else {
			if err := writer.WriteInt32(int32(len(pp.AttachPositions))); err != nil {
				return fmt.Errorf("write prop attach position count failed: %w", err)
			}
			slots, err := orderedPresetIntMapKeys(pp.AttachPositions, pp.AttachPositionOrder, "AttachPositions")
			if err != nil {
				return err
			}
			for _, slot := range slots {
				mp := pp.AttachPositions[slot]
				if err := writePresetSlot(writer, ver, "attachPos", slot, pp.AttachPositionSlotNames[slot]); err != nil {
					return err
				}
				if err := writer.WriteInt32(int32(len(mp))); err != nil {
					return fmt.Errorf("write prop attach position name count failed: %w", err)
				}
				names, err := orderedPresetStringMapKeys(mp, pp.AttachPositionNameOrders[slot], fmt.Sprintf("AttachPositions[%d]", slot))
				if err != nil {
					return err
				}
				for _, name := range names {
					e := mp[name]
					if err := writer.WriteString(name); err != nil {
						return fmt.Errorf("write prop attach position name failed: %w", err)
					}
					if err := writer.WriteInt32(e.RID); err != nil {
						return fmt.Errorf("write prop attach position rid failed: %w", err)
					}
					v := e.VtxAttachPos
					if err := writer.WriteBool(v.Enable); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos enable failed: %w", err)
					}
					if err := writer.WriteInt32(v.VtxCount); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos vtx count failed: %w", err)
					}
					if err := writer.WriteInt32(v.VtxIdx); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos vtx idx failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Position.X); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos position x failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Position.Y); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos position y failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Position.Z); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos position z failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Rotation.X); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos rotation x failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Rotation.Y); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos rotation y failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Rotation.Z); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos rotation z failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Rotation.W); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos rotation w failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Scale.X); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos scale x failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Scale.Y); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos scale y failed: %w", err)
					}
					if err := writer.WriteFloat32(v.PosRotScale.Scale.Z); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos scale z failed: %w", err)
					}
				}
			}
		}

		// 材质属性：slotID -> Entry
		if len(pp.MaterialProps) == 0 {
			if err := writer.WriteInt32(0); err != nil {
				return fmt.Errorf("write prop material prop count failed: %w", err)
			}
		} else {
			if err := writer.WriteInt32(int32(len(pp.MaterialProps))); err != nil {
				return fmt.Errorf("write prop material prop count failed: %w", err)
			}
			slots, err := orderedPresetIntMapKeys(pp.MaterialProps, pp.MaterialPropOrder, "MaterialProps")
			if err != nil {
				return err
			}
			for _, slot := range slots {
				e := pp.MaterialProps[slot]
				if err := writePresetSlot(writer, ver, "matProp", slot, e.SlotName); err != nil {
					return err
				}
				if err := writer.WriteInt32(e.RID); err != nil {
					return fmt.Errorf("write prop material prop rid failed: %w", err)
				}
				m := e.MatPropSave
				if err := writer.WriteInt32(m.MatId); err != nil {
					return fmt.Errorf("write prop material prop mat id failed: %w", err)
				}
				if err := writer.WriteString(m.PropName); err != nil {
					return fmt.Errorf("write prop material prop prop name failed: %w", err)
				}
				if err := writer.WriteString(m.TypeName); err != nil {
					return fmt.Errorf("write prop material prop type name failed: %w", err)
				}
				if err := writer.WriteString(m.Value); err != nil {
					return fmt.Errorf("write prop material prop value failed: %w", err)
				}
			}
		}

		// 骨骼长度块仅在 ver >= 213 时写入
		if ver >= 213 {
			if len(pp.BoneLengths) == 0 {
				if err := writer.WriteInt32(0); err != nil {
					return fmt.Errorf("write prop bone length count failed: %w", err)
				}
			} else {
				if err := writer.WriteInt32(int32(len(pp.BoneLengths))); err != nil {
					return fmt.Errorf("write prop bone length count failed: %w", err)
				}
				slots, err := orderedPresetIntMapKeys(pp.BoneLengths, pp.BoneLengthOrder, "BoneLengths")
				if err != nil {
					return err
				}
				for _, slot := range slots {
					e := pp.BoneLengths[slot]
					if err := writePresetSlot(writer, ver, "boneLen", slot, e.SlotName); err != nil {
						return err
					}
					if err := writer.WriteInt32(e.RID); err != nil {
						return fmt.Errorf("write prop bone length rid failed: %w", err)
					}
					if err := writer.WriteInt32(int32(len(e.Lengths))); err != nil {
						return fmt.Errorf("write prop bone length len count failed: %w", err)
					}
					names, err := orderedPresetStringMapKeys(e.Lengths, e.LengthOrder, fmt.Sprintf("BoneLengths[%d].Lengths", slot))
					if err != nil {
						return err
					}
					for _, k := range names {
						v := e.Lengths[k]
						if err := writer.WriteString(k); err != nil {
							return fmt.Errorf("write prop bone length len name failed: %w", err)
						}
						if err := writer.WriteFloat32(v); err != nil {
							return fmt.Errorf("write prop bone length len value failed: %w", err)
						}
					}
				}
			}
		}
	}
	if presetPropertyHasIsCrcParts(ver) {
		if err := writer.WriteBool(pp.IsCrcParts); err != nil {
			return fmt.Errorf("write prop isCrcParts failed: %w", err)
		}
	}

	return nil
}

func validateMultiColorForDump(mc *MultiColor) error {
	if mc == nil {
		return fmt.Errorf("MultiColor is nil")
	}
	if err := validatePresetSignature("MultiColor", mc.Signature, MultiColorSignature); err != nil {
		return err
	}
	if err := validatePresetCount("MultiColor count", mc.PartCount); err != nil {
		return err
	}
	if mc.Version <= 1200 {
		if len(mc.PartNames) != 0 {
			return fmt.Errorf("legacy MultiColor version %d cannot encode PartNames", mc.Version)
		}
		if int64(mc.PartCount) != int64(len(mc.PartsColors)) {
			return fmt.Errorf("legacy MultiColor PartCount %d does not match %d PartsColors", mc.PartCount, len(mc.PartsColors))
		}
		return nil
	}
	if len(mc.PartNames) != len(mc.PartsColors) {
		return fmt.Errorf("MultiColor PartNames length %d does not match PartsColors length %d", len(mc.PartNames), len(mc.PartsColors))
	}
	for index, name := range mc.PartNames {
		if name == "MAX" {
			return fmt.Errorf("MultiColor PartNames[%d] uses reserved terminator MAX", index)
		}
	}
	return nil
}

// 写入多颜色：按存储版本忠实写回旧/新布局。
func dumpMultiColor(writer *stream.BinaryWriter, mc *MultiColor) error {
	if err := validateMultiColorForDump(mc); err != nil {
		return fmt.Errorf("write MultiColor failed: %w", err)
	}
	if err := writer.WriteString(mc.Signature); err != nil {
		return fmt.Errorf("write prop multi color name failed: %w", err)
	}
	if err := writer.WriteInt32(mc.Version); err != nil {
		return fmt.Errorf("write prop multi color version failed: %w", err)
	}
	if err := writer.WriteInt32(mc.PartCount); err != nil {
		return fmt.Errorf("write prop multi color len count failed: %w", err)
	}

	if mc.Version <= 1200 {
		for index := range mc.PartsColors {
			if err := writePartsColor(writer, &mc.PartsColors[index]); err != nil {
				return fmt.Errorf("write prop multi color[%d] failed: %w", index, err)
			}
		}
		return nil
	}

	for index, name := range mc.PartNames {
		if err := writer.WriteString(name); err != nil {
			return fmt.Errorf("write prop multi color[%d] name failed: %w", index, err)
		}
		if err := writePartsColor(writer, &mc.PartsColors[index]); err != nil {
			return fmt.Errorf("write prop multi color[%d] failed: %w", index, err)
		}
	}
	if err := writer.WriteString("MAX"); err != nil {
		return fmt.Errorf("write prop multi color max failed: %w", err)
	}
	return nil
}

func writePartsColor(writer *stream.BinaryWriter, color *PartsColor) error {
	if err := writer.WriteBool(color.IsUse); err != nil {
		return err
	}
	for _, value := range []int32{
		color.MainHue, color.MainChroma, color.MainBrightness, color.MainContrast,
		color.ShadowRate, color.ShadowHue, color.ShadowChroma, color.ShadowBrightness, color.ShadowContrast,
	} {
		if err := writer.WriteInt32(value); err != nil {
			return err
		}
	}
	return nil
}

// 写入身体块：Maid.SerializeBody（仅头+版本）
func dumpBodyProperty(writer *stream.BinaryWriter, bp *BodyProperty) error {
	if bp == nil {
		return fmt.Errorf("write Body failed: BodyProperty is nil")
	}
	if err := validatePresetSignature("Body", bp.Signature, BodyPropertySignature); err != nil {
		return fmt.Errorf("write Body failed: %w", err)
	}

	if err := writer.WriteString(bp.Signature); err != nil {
		return fmt.Errorf("write prop body property signature failed: %w", err)
	}
	if err := writer.WriteInt32(bp.Version); err != nil {
		return fmt.Errorf("write prop body property version failed: %w", err)
	}
	return nil
}
