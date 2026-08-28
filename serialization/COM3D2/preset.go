package COM3D2

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	kces "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/utilities"
)

// CM3D2_PRESET
// 角色预设文件
//
// 有两种 PRESET，一种是 CM3D2_PRESET，另一种是 CM3D2_PRESET_S，CM3D2_PRESET_S 已被官方废弃（不支持、不解析）
//
// - 版本范围：
//   - 1 <= version < 1560：某些官方预设文件（支持）
//   - CM3D2 的 1560 <= version < 20000（支持）
//   - COM3D2 使用 20000 <= version < 30000（支持）
//   - COM3D2.5 的 version >= 30000（支持）
//
// COM3D2 和 COM3D2.5 的 Preset 无结构差异，但 COM3D2 会拒绝读取版本大于等于 30000 的文件
// COM3D2.5 则无读取版本校验
//
// 子块版本差异（与实际内容功能相关）
//
// 1) CM3D2_MPROP_LIST（属性列表版本）
//   - version < 4：列表项前没有写入“键名”（MPN 字符串）
//   - version >= 4：每个属性项前新增写入“键名”（MPN 字符串）
//
// 2) CM3D2_MPROP（单个属性版本）
//   - version < 101：无 temp_value
//   - version >= 101：新增 temp_value
//   - version < 200：无子属性与附加数据（子属性数组SubProps/皮肤位置SkinPositions/附着位置AttachPositions/材质属性MaterialProps）
//   - version >= 200：新增 SubProps（子属性数组）、皮肤位置（SkinPositions）、顶点附着位置（AttachPositions） 和 材质属性（MaterialProps）
//   - version < 204：头部材质 ZTest 字段命名旧（读取时会被迁移为 _ZTest2，并调整值）
//   - version >= 204：材质 ZTest 字段使用新命名和 _ZTest2 规则
//   - version < 211：SubProp 无 TexMulAlpha
//   - version >= 211：SubProp 新增 TexMulAlpha
//   - version < 213：无骨骼长度 BoneLengths 块
//   - version >= 213：新增 BoneLengths 块
//
// 3) CM3D2_MULTI_COL（多颜色版本）
//   - version <= 1200：旧格式（固定顺序的若干部件，历史上有 7 或 9 项的差异）
//   - version > 1200：新格式（以部件名列举，直到读到 "MAX" 终止）
//
// 4) CM3D2_MAID_BODY（身体块版本）
//   - 当前仅签名和版本，无后续字段
// CM3D2_PRESET
// Character preset file
//
// There are two PRESET signatures, CM3D2_PRESET and CM3D2_PRESET_S, and the officially retired CM3D2_PRESET_S format is neither supported nor parsed
//
// - Version ranges:
//   - 1 <= version < 1560: some official preset files, supported
//   - 1560 <= version < 20000: CM3D2, supported
//   - 20000 <= version < 30000: COM3D2, supported
//   - version >= 30000: COM3D2.5, supported
//
// COM3D2 and COM3D2.5 presets have the same structure, but COM3D2 rejects files whose version is at least 30000
// COM3D2.5 performs no preset version check while reading
//
// Sub-block version differences that affect actual content
//
// 1) CM3D2_MPROP_LIST property-list version
//   - version < 4: no key name MPN string precedes a list entry
//   - version >= 4: each property entry is preceded by a key name MPN string
//
// 2) CM3D2_MPROP individual-property version
//   - version < 101: no temp_value
//   - version >= 101: temp_value is present
//   - version < 200: no SubProps, SkinPositions, AttachPositions, or MaterialProps extension data
//   - version >= 200: SubProps, SkinPositions, AttachPositions, and MaterialProps are present
//   - version < 204: the head-material ZTest property uses its old name and is migrated to _ZTest2 with an adjusted value while reading
//   - version >= 204: the material ZTest property follows the new _ZTest2 naming rule
//   - version < 211: SubProp has no TexMulAlpha
//   - version >= 211: SubProp includes TexMulAlpha
//   - version < 213: no BoneLengths block
//   - version >= 213: the BoneLengths block is present
//
// 3) CM3D2_MULTI_COL multi-color version
//   - version <= 1200: legacy layout with a fixed part order and historical seven-entry or nine-entry variants
//   - version > 1200: current layout listing part names until the MAX terminator
//
// 4) CM3D2_MAID_BODY body-block version
//   - The current block contains only its signature and version

// 以下常量定义预设应用范围
// The following constants define the preset application scope
const (
	// PresetTypeWear 表示仅应用服装属性
	// PresetTypeWear applies clothing properties only
	PresetTypeWear = 0
	// PresetTypeBody 表示仅应用身体属性
	// PresetTypeBody applies body properties only
	PresetTypeBody = 1
	// PresetTypeAll 表示应用全部属性
	// PresetTypeAll applies all properties
	PresetTypeAll = 2
)

// Preset 表示角色预设数据
// Preset represents character preset data
type Preset struct {
	Signature          string              `json:"Signature"`          // 文件签名 CM3D2_PRESET / File signature CM3D2_PRESET
	Version            int32               `json:"Version"`            // 预设格式版本 / Preset format version
	PresetType         int32               `json:"PresetType"`         // 预设类型，0 为服装、1 为身体、2 为全部 / Preset type where 0 is wear, 1 is body, and 2 is all
	ThumbLength        int32               `json:"ThumbLength"`        // 线格式中的 PNG 缩略图字节数 / PNG thumbnail byte count on the wire
	ThumbData          []byte              `json:"ThumbData"`          // PNG 缩略图数据 / PNG thumbnail data
	PresetPropertyList *PresetPropertyList `json:"PresetPropertyList"` // 预设属性列表 / Preset property list
	MultiColor         *MultiColor         `json:"MultiColor"`         // 部件颜色设置 / Part color settings
	BodyProperty       *BodyProperty       `json:"BodyProperty"`       // 身体块数据 / Body-block data
}

// PresetMetadata 表示仅包含略缩图的的角色预设数据，不包含实际数据
// PresetMetadata represents character preset header and thumbnail data without the actual property data
type PresetMetadata struct {
	Signature   string `json:"Signature"`   // 文件签名 CM3D2_PRESET / File signature CM3D2_PRESET
	Version     int32  `json:"Version"`     // 预设格式版本 / Preset format version
	PresetType  int32  `json:"PresetType"`  // 预设类型，0 为服装、1 为身体、2 为全部 / Preset type where 0 is wear, 1 is body, and 2 is all
	ThumbLength int32  `json:"ThumbLength"` // 线格式中的 PNG 缩略图字节数 / PNG thumbnail byte count on the wire
	ThumbData   []byte `json:"ThumbData"`   // PNG 缩略图数据 / PNG thumbnail data
}

// PresetPropertyList 表示预设属性列表
// PresetPropertyList represents a preset property list
type PresetPropertyList struct {
	Signature        string                    `json:"Signature"`                 // 文件签名 CM3D2_MPROP_LIST / File signature CM3D2_MPROP_LIST
	Version          int32                     `json:"Version"`                   // 属性列表格式版本 / Property-list format version
	PropertyCount    int32                     `json:"PropertyCount"`             // 线格式中的主属性数量 / Main-property count on the wire
	PresetProperties map[string]PresetProperty `json:"PresetProperties"`          // 以前置 MPN 键名索引的主属性映射 / Main-property map indexed by the leading MPN key
	PropertyOrder    []string                  `json:"PropertyOrder,omitempty"`   // 主属性在线格式中的顺序 / Main-property order on the wire
	MaidPropOther    []NamedPresetProperty     `json:"MaidPropOther"`             // COM3D2.5 扩展属性及其前置键名 / COM3D2.5 extension properties and their leading keys
	PartsColorOther  *MultiColor               `json:"PartsColorOther,omitempty"` // COM3D2.5 另一身体体系的 CM3D2_MULTI_COL 块 / CM3D2_MULTI_COL block for COM3D2.5's other body system
	CRCPreset        *kces.ExpandedKCESPreset  `json:"CRCPreset,omitempty"`       // COM3D2.5 保存的 KCES VirtualDirectory 预设块 / KCES VirtualDirectory preset block stored by COM3D2.5
}

// NamedPresetProperty 保留 MPROP_LIST 中属性的前置键及其顺序
// NamedPresetProperty preserves a property's leading key and order in MPROP_LIST
type NamedPresetProperty struct {
	Key      string         `json:"Key"`      // 属性前置键名 / Leading property key
	Property PresetProperty `json:"Property"` // 属性数据 / Property data
}

// PresetProperty 表示单个属性
// PresetProperty represents one property
type PresetProperty struct {
	Signature                string                                 `json:"Signature"`                          // 文件签名 CM3D2_MPROP / File signature CM3D2_MPROP
	Version                  int32                                  `json:"Version"`                            // 属性格式版本 / Property format version
	Index                    int32                                  `json:"Index"`                              // 线格式中的 MPN 索引，游戏随后按 Name 重新解析 / MPN index on the wire, subsequently reparsed by the game from Name
	Name                     string                                 `json:"Name"`                               // MPN 枚举名称 / MPN enum name
	Type                     int32                                  `json:"Type"`                               // 属性类别编号 / Property category number
	DefaultValue             int32                                  `json:"DefaultValue"`                       // 默认值 / Default value
	Value                    int32                                  `json:"Value"`                              // 当前值 / Current value
	TempValue                int32                                  `json:"TempValue"`                          // 编辑中的临时值 / Temporary editing value
	LinkMaxValue             int32                                  `json:"LinkMaxValue"`                       // 限制当前值上限的关联属性索引，0 表示无关联 / Linked property index that caps the current value, with 0 meaning none
	FileName                 string                                 `json:"FileName"`                           // 属性所选菜单文件名 / Menu filename selected by the property
	FileNameRID              int32                                  `json:"FileNameRID"`                        // 线格式中的小写文件名哈希，游戏会为非空文件名重新计算 / Lowercase filename hash on the wire, recalculated by the game for a nonempty filename
	IsDut                    bool                                   `json:"IsDut"`                              // 属性待处理标志，游戏反序列化后强制设为 true / Property pending-processing flag, forced to true by the game after deserialization
	Max                      int32                                  `json:"Max"`                                // 最大值 / Maximum value
	Min                      int32                                  `json:"Min"`                                // 最小值 / Minimum value
	SubProps                 []*SubProp                             `json:"SubProps"`                           // 子属性列表，nil 元素对应线格式中的 exists=false / Sub-property list where a nil element represents exists=false on the wire
	SkinPositions            map[int32]BoneAttachPosEntry           `json:"SkinPositions"`                      // 按 slotID 索引的皮肤位置映射 / Skin-position map indexed by slotID
	SkinPositionOrder        []int32                                `json:"SkinPositionOrder,omitempty"`        // 皮肤位置在线格式中的槽位顺序 / Skin-position slot order on the wire
	AttachPositions          map[int32]map[string]VtxAttachPosEntry `json:"AttachPositions"`                    // 按 slotID 和附着点名索引的顶点附着位置 / Vertex attachment positions indexed by slotID and attachment-point name
	AttachPositionOrder      []int32                                `json:"AttachPositionOrder,omitempty"`      // 顶点附着位置在线格式中的槽位顺序 / Vertex-attachment slot order on the wire
	AttachPositionNameOrders map[int32][]string                     `json:"AttachPositionNameOrders,omitempty"` // 各槽位附着点在线格式中的名称顺序 / Attachment-point name order on the wire for each slot
	AttachPositionSlotNames  map[int32]string                       `json:"AttachPositionSlotNames,omitempty"`  // v2003 起每个顶点附着槽位写入的槽位名，包括空映射 / Slot name written for each vertex-attachment slot since v2003, including an empty map
	MaterialProps            map[int32]MatPropSaveEntry             `json:"MaterialProps"`                      // 按 slotID 索引的材质属性覆盖 / Material-property overrides indexed by slotID
	MaterialPropOrder        []int32                                `json:"MaterialPropOrder,omitempty"`        // 材质属性在线格式中的槽位顺序 / Material-property slot order on the wire
	BoneLengths              map[int32]BoneLengthEntry              `json:"BoneLengths"`                        // 按 slotID 索引的骨骼长度设置 / Bone-length settings indexed by slotID
	BoneLengthOrder          []int32                                `json:"BoneLengthOrder,omitempty"`          // 骨骼长度在线格式中的槽位顺序 / Bone-length slot order on the wire
	IsCrcParts               bool                                   `json:"IsCrcParts"`                         // CRC、CRX 或 GP03 部件标志 / CRC, CRX, or GP03 part flag
}

// BoneAttachPosEntry 保存槽位皮肤位置及其来源菜单哈希
// BoneAttachPosEntry stores a slot skin position and its source-menu hash
type BoneAttachPosEntry struct {
	SlotName      string        `json:"SlotName,omitempty"` // v2003 起在线格式中用于解析槽位枚举的名称 / Name used to resolve the slot enum on the wire since v2003
	RID           int32         `json:"RID"`                // 来源菜单文件名哈希，用于确认数据属于当前菜单项 / Source-menu filename hash used to verify that the data belongs to the current menu item
	BoneAttachPos BoneAttachPos `json:"BoneAttachPos"`      // 皮肤位置数据 / Skin-position data
}

// VtxAttachPosEntry 保存顶点附着位置及其来源菜单哈希
// VtxAttachPosEntry stores a vertex attachment position and its source-menu hash
type VtxAttachPosEntry struct {
	RID          int32        `json:"RID"`          // 来源菜单文件名哈希，用于确认数据属于当前菜单项 / Source-menu filename hash used to verify that the data belongs to the current menu item
	VtxAttachPos VtxAttachPos `json:"VtxAttachPos"` // 顶点附着位置数据 / Vertex attachment-position data
}

// MatPropSaveEntry 保存材质属性覆盖及其来源菜单哈希
// MatPropSaveEntry stores a material-property override and its source-menu hash
type MatPropSaveEntry struct {
	SlotName    string      `json:"SlotName,omitempty"` // v2003 起在线格式中用于解析槽位枚举的名称 / Name used to resolve the slot enum on the wire since v2003
	RID         int32       `json:"RID"`                // 来源菜单文件名哈希，用于确认数据属于当前菜单项 / Source-menu filename hash used to verify that the data belongs to the current menu item
	MatPropSave MatPropSave `json:"MatPropSave"`        // 材质属性覆盖数据 / Material-property override data
}

// BoneLengthEntry 保存槽位骨骼长度设置及其来源菜单哈希
// BoneLengthEntry stores slot bone-length settings and their source-menu hash
type BoneLengthEntry struct {
	SlotName    string             `json:"SlotName,omitempty"`    // v2003 起在线格式中用于解析槽位枚举的名称 / Name used to resolve the slot enum on the wire since v2003
	RID         int32              `json:"RID"`                   // 来源菜单文件名哈希，用于确认数据属于当前菜单项 / Source-menu filename hash used to verify that the data belongs to the current menu item
	Lengths     map[string]float32 `json:"Lengths"`               // 按骨骼名索引的长度值 / Length values indexed by bone name
	LengthOrder []string           `json:"LengthOrder,omitempty"` // 骨骼名称在线格式中的顺序 / Bone-name order on the wire
}

// SubProp 表示子属性
// SubProp represents a sub-property
type SubProp struct {
	IsDut       bool    `json:"IsDut"`       // 子属性待处理标志，游戏读取后强制设为 true / Sub-property pending-processing flag, forced to true by the game after reading
	FileName    string  `json:"FileName"`    // 子属性所选菜单文件名 / Menu filename selected by the sub-property
	FileNameRID int32   `json:"FileNameRID"` // 线格式中的小写文件名哈希，游戏读取后重新计算 / Lowercase filename hash on the wire, recalculated by the game after reading
	TexMulAlpha float32 `json:"TexMulAlpha"` // 纹理合成透明度倍率 / Texture-composition alpha multiplier
}

// BoneAttachPos 表示骨骼附着位置
// BoneAttachPos represents a bone attachment position
type BoneAttachPos struct {
	Enable      bool                  `json:"Enable"`                // 是否启用 / Whether the position is enabled
	PosRotScale PositionRotationScale `json:"PositionRotationScale"` // 局部位置、旋转和缩放 / Local position, rotation, and scale
}

// VtxAttachPos 表示顶点附着位置
// VtxAttachPos represents a vertex attachment position
type VtxAttachPos struct {
	Enable      bool                  `json:"Enable"`                // 是否启用 / Whether the attachment is enabled
	VtxCount    int32                 `json:"VtxCount"`              // 创建附着数据时的网格顶点数，用于拒绝不匹配的网格 / Mesh vertex count when the attachment was created, used to reject a mismatched mesh
	VtxIdx      int32                 `json:"VtxIdx"`                // 目标顶点索引 / Target vertex index
	PosRotScale PositionRotationScale `json:"PositionRotationScale"` // 相对于目标顶点的位置、旋转和缩放 / Position, rotation, and scale relative to the target vertex
}

// MatPropSave 表示材质属性保存
// MatPropSave represents a saved material-property override
type MatPropSave struct {
	MatId    int32  `json:"MatId"`    // 槽位内的材质索引 / Material index within the slot
	PropName string `json:"PropName"` // 着色器属性名称 / Shader property name
	TypeName string `json:"TypeName"` // 游戏用于解释 Value 的属性类型名 / Property type name used by the game to interpret Value
	Value    string `json:"Value"`    // 字符串编码的属性值 / String-encoded property value
}

// MultiColor 表示多颜色设置
// MultiColor represents multi-color settings
type MultiColor struct {
	Signature   string       `json:"Signature"`   // 文件签名 CM3D2_MULTI_COL / File signature CM3D2_MULTI_COL
	Version     int32        `json:"Version"`     // 多颜色格式版本 / Multi-color format version
	PartCount   int32        `json:"PartCount"`   // wire 头部颜色数量；新布局读取时游戏忽略该值 / Wire header count; ignored by the new-layout reader
	PartNames   []string     `json:"PartNames"`   // 新布局中每项的原始名称及顺序；旧布局无名称 / Raw names/order in the new layout; absent in the legacy layout
	PartsColors []PartsColor `json:"PartsColors"` // wire 上实际存在的颜色，不展开游戏默认值 / Colors physically present on the wire, without game defaults
}

// PartsColor 表示部件颜色
// PartsColor represents one part color
type PartsColor struct {
	IsUse            bool  `json:"IsUse"`            // 是否启用该部件颜色 / Whether this part color is enabled
	MainHue          int32 `json:"MainHue"`          // 主色相 / Main hue
	MainChroma       int32 `json:"MainChroma"`       // 主色度 / Main chroma
	MainBrightness   int32 `json:"MainBrightness"`   // 主亮度 / Main brightness
	MainContrast     int32 `json:"MainContrast"`     // 主对比度 / Main contrast
	ShadowRate       int32 `json:"ShadowRate"`       // 阴影混合比例 / Shadow blend rate
	ShadowHue        int32 `json:"ShadowHue"`        // 阴影色相 / Shadow hue
	ShadowChroma     int32 `json:"ShadowChroma"`     // 阴影色度 / Shadow chroma
	ShadowBrightness int32 `json:"ShadowBrightness"` // 阴影亮度 / Shadow brightness
	ShadowContrast   int32 `json:"ShadowContrast"`   // 阴影对比度 / Shadow contrast
}

// BodyProperty 表示身体属性
// BodyProperty represents the body block
type BodyProperty struct {
	Signature string `json:"Signature"` // 文件签名 CM3D2_MAID_BODY / File signature CM3D2_MAID_BODY
	Version   int32  `json:"Version"`   // 身体块格式版本 / Body-block format version
	// 是的，确实没有其他内容
	// Yes, there really is nothing else
}

// validatePresetSignature 验证读取或写出的块签名是否为预期值
// validatePresetSignature verifies that a block signature being read or written has the expected value
func validatePresetSignature(field, got, want string) error {
	if got != want {
		return fmt.Errorf("invalid %s signature: expected %q, got %q", field, want, got)
	}
	return nil
}

// validatePresetCount 验证线格式中的 Int32 集合数量不是负数
// validatePresetCount verifies that an Int32 collection count on the wire is not negative
func validatePresetCount(field string, count int32) error {
	if count < 0 {
		return fmt.Errorf("invalid %s: %d", field, count)
	}
	return nil
}

// presetPropertyHasIsCrcParts 判断属性版本是否包含 IsCrcParts 字段
// presetPropertyHasIsCrcParts reports whether a property version includes the IsCrcParts field
func presetPropertyHasIsCrcParts(version int32) bool {
	return (version >= 2001 && version < 20000) || version >= 30000
}

// presetPropertyHasSlotNames 判断属性版本是否在数字槽位 ID 后包含槽位名称
// presetPropertyHasSlotNames reports whether a property version includes a slot name after each numeric slot ID
func presetPropertyHasSlotNames(version int32) bool {
	return (version >= 2003 && version < 20000) || version >= 30000
}

// presetPropertyListHasExtensions 判断属性列表版本是否包含 COM3D2.5 扩展块
// presetPropertyListHasExtensions reports whether a property-list version includes the COM3D2.5 extension blocks
func presetPropertyListHasExtensions(version int32) bool {
	return (version >= 2001 && version < 20000) || version >= 30000
}

// readPresetByteBlock 读取以 Int32 字节数为前缀的预设二进制块
// readPresetByteBlock reads a preset binary block prefixed by an Int32 byte count
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
	data, err := reader.ReadBytes(int64(length))
	if err != nil {
		return nil, fmt.Errorf("read %s data failed: %w", field, err)
	}
	return data, nil
}

// writePresetByteBlock 写入以 Int32 字节数为前缀的预设二进制块
// writePresetByteBlock writes a preset binary block prefixed by an Int32 byte count
func writePresetByteBlock(writer *stream.BinaryWriter, field string, data []byte) error {
	length, err := collectionCountInt32(field+" length", int64(len(data)))
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

// validatePresetMapLength 验证映射表项数可由线格式中的 Int32 表示
// validatePresetMapLength verifies that a map entry count is representable by an Int32 on the wire
func validatePresetMapLength(field string, length int64) error {
	_, err := collectionCountInt32(field, length)
	return err
}

// readPresetSlot 读取数字槽位 ID，并在版本支持时继续读取槽位名称
// readPresetSlot reads a numeric slot ID followed by a slot name when the version supports it
func readPresetSlot(reader *stream.BinaryReader, version int32, field string) (int32, string, error) {
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
	return slotID, slotName, nil
}

// ReadPreset 从 r 中读取 Preset
// ReadPreset reads a Preset from r
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

// readPreset 解析一个 preset 对象但不检查 EOF，供有明确外层边界的内部格式复用
// readPreset parses one preset object without checking EOF so formats with an explicit outer boundary can reuse it
func readPreset(reader *stream.BinaryReader) (*Preset, error) {
	p := &Preset{}

	// 1. 签名
	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .Preset signature failed: %w", err)
	}
	if err := validatePresetSignature(".Preset", sig, PresetSignature); err != nil {
		return nil, err
	}
	p.Signature = sig

	// 2. 版本
	// 2. Version
	version, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset version failed: %w", err)
	}
	p.Version = version

	// 3. 预设类型
	// 3. Preset type
	presetType, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset presetType failed: %w", err)
	}
	p.PresetType = presetType

	// 4. 缩略图长度
	// 4. Thumbnail length
	thumbLength, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset ThumbLength failed: %w", err)
	}
	if err := validatePresetCount(".Preset ThumbLength", thumbLength); err != nil {
		return nil, err
	}
	p.ThumbLength = thumbLength

	// 5. 缩略图数据
	// 5. Thumbnail data
	if p.ThumbLength > 0 {
		p.ThumbData, err = reader.ReadBytes(int64(p.ThumbLength))
		if err != nil {
			return nil, fmt.Errorf("read .Preset ThumbData failed: %w", err)
		}
	}

	// 6. 属性列表
	// 6. Property list
	p.PresetPropertyList, err = readPresetPropertyList(reader)
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList failed: %w", err)
	}

	// 7. 多颜色块
	// 7. Multi-color block
	if version >= 2 && (version < 2000 || 10000 <= version) {
		mc, err := readMultiColor(reader)
		if err != nil {
			return nil, fmt.Errorf("read .Preset MultiColor failed: %w", err)
		}
		p.MultiColor = mc
	}

	// 8. 身体块
	// 8. Body block
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
// ReadPresetMetadata reads PresetMetadata from r
func ReadPresetMetadata(reader *stream.BinaryReader) (*PresetMetadata, error) {
	p := &PresetMetadata{}

	// 1. 签名
	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .Preset signature failed: %w", err)
	}
	if err := validatePresetSignature(".Preset", sig, PresetSignature); err != nil {
		return nil, err
	}
	p.Signature = sig

	// 2. 版本
	// 2. Version
	version, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset version failed: %w", err)
	}
	p.Version = version

	// 3. 预设类型
	// 3. Preset type
	presetType, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset presetType failed: %w", err)
	}
	p.PresetType = presetType

	// 4. 缩略图长度
	// 4. Thumbnail length
	thumbLength, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset ThumbLength failed: %w", err)
	}
	if err := validatePresetCount(".Preset ThumbLength", thumbLength); err != nil {
		return nil, err
	}
	p.ThumbLength = thumbLength

	// 5. 缩略图数据
	// 5. Thumbnail data
	if p.ThumbLength > 0 {
		p.ThumbData, err = reader.ReadBytes(int64(p.ThumbLength))
		if err != nil {
			return nil, fmt.Errorf("read .Preset ThumbData failed: %w", err)
		}
	}

	return p, nil
}

// readPresetPropertyList 从 r 中读取 PresetPropertyList
// readPresetPropertyList reads a PresetPropertyList from r
func readPresetPropertyList(reader *stream.BinaryReader) (*PresetPropertyList, error) {
	ppl := &PresetPropertyList{PresetProperties: map[string]PresetProperty{}}

	// 1. 签名
	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList signature failed: %w", err)
	}
	if err := validatePresetSignature("PresetPropertyList", sig, PresetPropertyListSignature); err != nil {
		return nil, err
	}
	ppl.Signature = sig

	// 2. 版本
	// 2. Version
	version, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList version failed: %w", err)
	}
	ppl.Version = version

	// 3. 属性数量
	// 3. Property count
	count, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList propertyCount failed: %w", err)
	}
	if err := validatePresetCount("PresetPropertyList propertyCount", count); err != nil {
		return nil, err
	}
	ppl.PropertyCount = count

	// 4. 预设属性
	// 4. Preset properties
	for i := int32(0); i < count; i++ {
		var key string
		hasStoredKey := version >= 4
		if hasStoredKey {
			// 新版：SerializeProp 会先写 key（MPN 名称字符串）
			// The current layout writes the key MPN name string first in SerializeProp
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
			// The legacy layout has no stored key, so use prop.Name as the key
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
		partsColorOtherData, err := readPresetByteBlock(reader, "PartsColorOtherBin")
		if err != nil {
			return nil, err
		}
		if len(partsColorOtherData) != 0 {
			ppl.PartsColorOther, err = DecodeMultiColorBlock(partsColorOtherData)
			if err != nil {
				return nil, fmt.Errorf("decode PartsColorOtherBin: %w", err)
			}
		}
		crcPresetData, err := readPresetByteBlock(reader, "CRCPresetBin")
		if err != nil {
			return nil, err
		}
		if len(crcPresetData) != 0 {
			ppl.CRCPreset, err = kces.DecodeExpandedKCESPreset(crcPresetData)
			if err != nil {
				return nil, fmt.Errorf("decode CRCPresetBin: %w", err)
			}
		}
	}

	return ppl, nil
}

// readPresetProperty 按 MaidProp.Deserialize 的布局读取单个属性，格式与 MaidProp.Serialize 对齐
// readPresetProperty reads one property using the MaidProp.Deserialize layout corresponding to MaidProp.Serialize
func readPresetProperty(reader *stream.BinaryReader) (*PresetProperty, error) {
	prop := &PresetProperty{}

	// 1. 签名
	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetProperty signature failed: %w", err)
	}
	if err := validatePresetSignature("PresetProperty", sig, PresetPropertySignature); err != nil {
		return nil, err
	}
	prop.Signature = sig

	// 2. 版本
	// 2. Version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetProperty version failed: %w", err)
	}
	prop.Version = ver

	// 3. 索引
	// 3. Index
	idx, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read prop.idx failed: %w", err)
	}
	prop.Index = idx

	// 4. 名称
	// 4. Name
	name, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read prop.name failed: %w", err)
	}
	prop.Name = name

	// 5. 基本数值
	// 5. Basic values
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
	// Sub-properties for ver >= 200
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
		for i := int32(0); i < cnt; i++ {
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
			prop.SkinPositions = makeCountedMap[int32, BoneAttachPosEntry](nSkin)
		}
		for i := int32(0); i < nSkin; i++ {
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
			prop.AttachPositions = makeCountedMap[int32, map[string]VtxAttachPosEntry](nAttach)
			prop.AttachPositionNameOrders = makeCountedMap[int32, []string](nAttach)
			if presetPropertyHasSlotNames(ver) {
				prop.AttachPositionSlotNames = makeCountedMap[int32, string](nAttach)
			}
		}
		for i := int32(0); i < nAttach; i++ {
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
			for j := int32(0); j < inner; j++ {
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
			prop.MaterialProps = makeCountedMap[int32, MatPropSaveEntry](nMat)
		}
		for i := int32(0); i < nMat; i++ {
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
				prop.BoneLengths = makeCountedMap[int32, BoneLengthEntry](nBone)
			}
			for i := int32(0); i < nBone; i++ {
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
				for j := int32(0); j < inner; j++ {
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

// readMultiColor 读取多颜色线格式，不执行 MaidParts.DeserializePre 的默认色展开
// readMultiColor reads the multi-color wire format without expanding the defaults applied by MaidParts.DeserializePre
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

	// 新布局由 MAX 终止，不使用 PartCount 限制循环，并保留名称、重复项、顺序和未来新增的枚举名
	// The current layout is terminated by MAX and does not use PartCount to bound this loop, preserving names, duplicates, order, and future enum names
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

// DecodeMultiColorBlock 解码一个完整的 CM3D2_MULTI_COL 字节块，供顶层预设和 COM3D2.5 的 PartsColorOther 字段共用
// DecodeMultiColorBlock decodes one complete CM3D2_MULTI_COL byte block for both top-level presets and COM3D2.5's PartsColorOther field
func DecodeMultiColorBlock(data []byte) (*MultiColor, error) {
	r := bytes.NewReader(data)
	value, err := readMultiColor(stream.NewBinaryReader(r))
	if err != nil {
		return nil, err
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("MultiColor block has %d trailing bytes", r.Len())
	}
	return value, nil
}

// EncodeMultiColorBlock 写入一个完整的 CM3D2_MULTI_COL 字节块，并保留 value 中的版本与新旧布局
// EncodeMultiColorBlock writes one complete CM3D2_MULTI_COL byte block while retaining the version and current or legacy layout stored in value
func EncodeMultiColorBlock(value *MultiColor) ([]byte, error) {
	var out bytes.Buffer
	if err := dumpMultiColor(stream.NewBinaryWriter(&out), value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// readPartsColor 按 MaidParts.PartsColor 的线格式读取一个部件颜色并返回字段数量
// readPartsColor reads one part color using the MaidParts.PartsColor wire layout and returns its field count
func readPartsColor(reader *stream.BinaryReader, pc *PartsColor) (int64, error) {
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

// readBodyProperty 按 Maid.DeserializeBodyRead 的布局读取仅含签名和版本的身体块
// readBodyProperty reads the signature-and-version-only body block using the Maid.DeserializeBodyRead layout
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
	// This block currently has no additional fields
	return bp, nil
}

// orderedPresetPropertyKeys 合并属性映射与保存的线格式顺序并验证二者一致
// orderedPresetPropertyKeys merges the property map with its saved wire order and verifies their consistency
func orderedPresetPropertyKeys(ppl *PresetPropertyList) ([]string, error) {
	if err := validatePresetMapLength("PresetPropertyList propertyCount", int64(len(ppl.PresetProperties))); err != nil {
		return nil, err
	}
	return utilities.MergeOrderedMapKeys(ppl.PresetProperties, ppl.PropertyOrder, "PresetPropertyList PropertyOrder")
}

// orderedPresetIntMapKeys 合并 int 键映射与保存的线格式顺序并验证二者一致
// orderedPresetIntMapKeys merges an int-keyed map with its saved wire order and verifies their consistency
func orderedPresetIntMapKeys[V any](values map[int32]V, order []int32, label string) ([]int32, error) {
	return utilities.MergeOrderedMapKeys(values, order, label+" order")
}

// orderedPresetStringMapKeys 合并字符串键映射与保存的线格式顺序并验证二者一致
// orderedPresetStringMapKeys merges a string-keyed map with its saved wire order and verifies their consistency
func orderedPresetStringMapKeys[V any](values map[string]V, order []string, label string) ([]string, error) {
	return utilities.MergeOrderedMapKeys(values, order, label+" order")
}

// validatePresetPropertyForDump 验证属性数据满足其版本门槛、数量范围和顺序约束
// validatePresetPropertyForDump verifies that property data satisfies its version gates, count ranges, and ordering constraints
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
	if err := validatePresetMapLength("subProp count", int64(len(pp.SubProps))); err != nil {
		return err
	}
	if err := validatePresetMapLength("skinPos count", int64(len(pp.SkinPositions))); err != nil {
		return err
	}
	if err := validatePresetMapLength("attachPos count", int64(len(pp.AttachPositions))); err != nil {
		return err
	}
	if err := validatePresetMapLength("matProp count", int64(len(pp.MaterialProps))); err != nil {
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
	validateSlot := func(field, slotName string) error {
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
		if err := validateSlot("skinPos", pp.SkinPositions[slot].SlotName); err != nil {
			return err
		}
	}
	attachSlots, err := orderedPresetIntMapKeys(pp.AttachPositions, pp.AttachPositionOrder, "AttachPositions")
	if err != nil {
		return err
	}
	for _, slot := range attachSlots {
		if err := validateSlot("attachPos", pp.AttachPositionSlotNames[slot]); err != nil {
			return err
		}
		values := pp.AttachPositions[slot]
		if err := validatePresetMapLength("attachPos inner count", int64(len(values))); err != nil {
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
		if err := validateSlot("matProp", pp.MaterialProps[slot].SlotName); err != nil {
			return err
		}
	}
	if pp.Version >= 213 {
		if err := validatePresetMapLength("boneLen count", int64(len(pp.BoneLengths))); err != nil {
			return err
		}
		boneSlots, err := orderedPresetIntMapKeys(pp.BoneLengths, pp.BoneLengthOrder, "BoneLengths")
		if err != nil {
			return err
		}
		for _, slot := range boneSlots {
			entry := pp.BoneLengths[slot]
			if err := validateSlot("boneLen", entry.SlotName); err != nil {
				return err
			}
			if err := validatePresetMapLength("boneLen inner count", int64(len(entry.Lengths))); err != nil {
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

// validatePresetPropertyListForDump 验证属性列表及其扩展块可被当前版本完整写出并返回主属性顺序
// validatePresetPropertyListForDump verifies that a property list and its extension blocks can be fully written by the current version and returns the main-property order
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
		if err := validatePresetMapLength("MaidPropOther count", int64(len(ppl.MaidPropOther))); err != nil {
			return nil, err
		}
		for i := range ppl.MaidPropOther {
			if err := validatePresetPropertyForDump(&ppl.MaidPropOther[i].Property); err != nil {
				return nil, fmt.Errorf("MaidPropOther[%d]: %w", i, err)
			}
		}
		if _, _, err := encodePresetPropertyListExtensionBlocks(ppl); err != nil {
			return nil, err
		}
	} else if len(ppl.MaidPropOther) != 0 || ppl.PartsColorOther != nil || ppl.CRCPreset != nil {
		return nil, fmt.Errorf("PresetPropertyList version %d cannot encode COM3D2.5 extension data", ppl.Version)
	}
	return keys, nil
}

// encodePresetPropertyListExtensionBlocks 编码 PartsColorOther 和 CRCPreset 的长度前缀载荷
// encodePresetPropertyListExtensionBlocks encodes the length-prefixed payloads for PartsColorOther and CRCPreset
func encodePresetPropertyListExtensionBlocks(ppl *PresetPropertyList) (partsColorOtherData, crcPresetData []byte, err error) {
	if ppl.PartsColorOther != nil {
		partsColorOtherData, err = EncodeMultiColorBlock(ppl.PartsColorOther)
		if err != nil {
			return nil, nil, fmt.Errorf("encode PartsColorOther: %w", err)
		}
	}
	if _, err := collectionCountInt32("PartsColorOtherBin length", int64(len(partsColorOtherData))); err != nil {
		return nil, nil, err
	}
	if ppl.CRCPreset != nil {
		crcPresetData, err = kces.EncodeExpandedKCESPreset(ppl.CRCPreset)
		if err != nil {
			return nil, nil, fmt.Errorf("encode CRCPreset: %w", err)
		}
	}
	if _, err := collectionCountInt32("CRCPresetBin length", int64(len(crcPresetData))); err != nil {
		return nil, nil, err
	}
	return partsColorOtherData, crcPresetData, nil
}

// validatePresetForDump 验证预设的签名、缩略图长度以及由版本控制的子块
// validatePresetForDump verifies the preset signature, thumbnail length, and version-gated sub-blocks
func validatePresetForDump(p *Preset) error {
	if p == nil {
		return fmt.Errorf("Preset is nil")
	}
	if err := validatePresetSignature(".Preset", p.Signature, PresetSignature); err != nil {
		return err
	}
	if _, err := collectionCountInt32(".Preset ThumbLength", int64(len(p.ThumbData))); err != nil {
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

// writePresetSlot 写入数字槽位 ID，并在版本支持时继续写入槽位名称
// writePresetSlot writes a numeric slot ID followed by a slot name when the version supports it
func writePresetSlot(writer *stream.BinaryWriter, version int32, field string, slot int32, slotName string) error {
	if err := writer.WriteInt32(slot); err != nil {
		return fmt.Errorf("write %s slotID failed: %w", field, err)
	}
	if presetPropertyHasSlotNames(version) {
		if err := writer.WriteString(slotName); err != nil {
			return fmt.Errorf("write %s slotName failed: %w", field, err)
		}
	}
	return nil
}

// Dump 将 Preset 写入 w
// Dump writes the Preset to w
func (p *Preset) Dump(w io.Writer) error {
	if err := validatePresetForDump(p); err != nil {
		return fmt.Errorf("write .Preset failed: %w", err)
	}
	p.ThumbLength = int32(len(p.ThumbData))
	writer := stream.NewBinaryWriter(w)

	// 1. 签名
	// 1. Signature
	if err := writer.WriteString(p.Signature); err != nil {
		return fmt.Errorf("write preset signature failed: %w", err)
	}

	// 2. 版本
	// 2. Version
	if err := writer.WriteInt32(p.Version); err != nil {
		return fmt.Errorf("write preset version failed: %w", err)
	}

	// 3. 预设类型
	// 3. Preset type
	if err := writer.WriteInt32(p.PresetType); err != nil {
		return fmt.Errorf("write preset type failed: %w", err)
	}

	// 4. 缩略图
	// 4. Thumbnail
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

	// 5. 属性列表
	// 5. Property list
	if err := dumpPresetPropertyList(writer, p.PresetPropertyList); err != nil {
		return fmt.Errorf("write PresetPropertyList failed: %w", err)
	}

	// 6. 多颜色块
	// 6. Multi-color block
	if p.Version >= 2 && (p.Version < 2000 || 10000 <= p.Version) {
		if err := dumpMultiColor(writer, p.MultiColor); err != nil {
			return fmt.Errorf("write MultiColor failed: %w", err)
		}
	}

	// 7. 身体块
	// 7. Body block
	if p.Version >= 200 && (p.Version < 2000 || 10000 <= p.Version) {
		if err := dumpBodyProperty(writer, p.BodyProperty); err != nil {
			return fmt.Errorf("write Body failed: %w", err)
		}
	}

	return nil
}

// dumpPresetPropertyList 按 Maid.SerializeProp 的布局写入属性列表
// dumpPresetPropertyList writes a property list using the Maid.SerializeProp layout
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
		// Write the key MPN string only when the list version is at least 4
		if ppl.Version >= 4 {
			if err := writer.WriteString(k); err != nil {
				return fmt.Errorf("write prop key failed: %w", err)
			}
		}
		// 复制映射值以取得可传递给写出函数的指针
		// Copy the map value to obtain a pointer that can be passed to the writer
		prop := v
		if err := writePresetProperty(writer, &prop); err != nil {
			return fmt.Errorf("write prop '%s' failed: %w", k, err)
		}
	}

	if presetPropertyListHasExtensions(ppl.Version) {
		partsColorOtherData, crcPresetData, err := encodePresetPropertyListExtensionBlocks(ppl)
		if err != nil {
			return err
		}
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
		if err := writePresetByteBlock(writer, "PartsColorOtherBin", partsColorOtherData); err != nil {
			return err
		}
		if err := writePresetByteBlock(writer, "CRCPresetBin", crcPresetData); err != nil {
			return err
		}
	}

	return nil
}

// writePresetProperty 按 MaidProp.Serialize 的布局写入单个属性
// writePresetProperty writes one property using the MaidProp.Serialize layout
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
	// Write TempValue only when ver >= 101
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
	// Write sub-properties and extension blocks only when ver >= 200 to match the reader
	if ver >= 200 {
		// 子属性
		// Sub-properties
		nSub := int32(len(pp.SubProps))
		if err := writer.WriteInt32(nSub); err != nil {
			return fmt.Errorf("write prop sub count failed: %w", err)
		}
		for i := int32(0); i < nSub; i++ {
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
			// Write TexMulAlpha only when ver >= 211
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

// validateMultiColorForDump 验证多颜色数据符合其新旧布局和 MAX 终止符约束
// validateMultiColorForDump verifies that multi-color data satisfies its current or legacy layout and MAX terminator constraints
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

// dumpMultiColor 按保存的版本忠实写回新布局或旧布局
// dumpMultiColor faithfully writes the current or legacy layout selected by the stored version
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

// writePartsColor 按 MaidParts.PartsColor 的线格式写入一个部件颜色
// writePartsColor writes one part color using the MaidParts.PartsColor wire layout
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

// dumpBodyProperty 按 Maid.SerializeBody 的布局写入仅含签名和版本的身体块
// dumpBodyProperty writes the signature-and-version-only body block using the Maid.SerializeBody layout
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
