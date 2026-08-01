package KCES

import (
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

// .menuassets
// KCES 菜单资源容器，在 .aba 的 TextAsset 中保存 Parts.Menu 数组
// 载荷使用 LZ4 Block Array 压缩的 MessagePack indexed-array，当前 Menu 固定版本为 1005
// .menuassets
// KCES menu-resource container storing an array of Parts.Menu values in a TextAsset inside an .aba file
// The payload is an LZ4 Block Array-compressed MessagePack indexed array, with current Menu fixed version 1005

// MessagePack indexed array 布局（KCES 为 Key(0)~Key(30)，KCES2 追加 Key(31)）/ MessagePack indexed-array layout using Key(0) through Key(30) in KCES and appending Key(31) in KCES2
//
//	[Key(0)]  version                    int
//	[Key(1)]  guid                       uint64
//	[Key(2)]  id                         uint64
//	[Key(3)]  fileName                   string
//	[Key(4)]  itemName                   string
//	[Key(5)]  iconFileName               string
//	[Key(6)]  infoText                   string
//	[Key(7)]  priority                   int
//	[Key(8)]  parentId                   uint64
//	[Key(9)]  isMan                      bool
//	[Key(10)] isDiff                     bool
//	[Key(11)] isDelete                   bool
//	[Key(12)] commandList                Command[]
//	[Key(13)] categoryText               string
//	[Key(14)] colorSetText               string
//	[Key(15)] defineTagNames             uint64 (DEFINE flags)
//	[Key(16)] preMulTexDatas             map<uint64, PreMulTexDatas>
//	[Key(17)] colvariFileNameExp         string
//	[Key(18)] colvariInfo                Colvari
//	[Key(19)] srcFileHashCRC32           uint32
//	[Key(20)] defineFirst                uint64 (DEFINE flags)
//	[Key(21)] partsVer                   [string, int] (Tuple)
//	[Key(22)] isRecommendMan             bool
//	[Key(23)] targetBodyType             int (enum)
//	[Key(24)] (reserved/nil)
//	[Key(25)] attribute                  uint64 (Attribute flags)
//	[Key(26)] hideInEdit                 bool
//	[Key(27)] toeLockSlotId              string
//	[Key(28)] exportModelFormTextureName string
//	[Key(29)] isHarayureAvailable        int (enum)
//	[Key(30)] skirt_phys                 int
//	[Key(31)] hairMake                   HairMake

// Menu 表示 Parts.Menu 的菜单数据
// 对应 C# Parts.Menu，继承自 AMessagePackSerializationVersionControlIntKey
// MessagePack indexed array 在 KCES 中包含 Key(0) 至 Key(30)，KCES2 在 Key(31) 追加 HairMake，Key(24) 在 C# 中没有成员
// Menu represents menu data from Parts.Menu
// It matches C# Parts.Menu derived from AMessagePackSerializationVersionControlIntKey
// Its MessagePack indexed array contains Key(0) through Key(30) in KCES and appends HairMake at Key(31) in KCES2, with no C# member at Key(24)
type Menu struct {
	_struct                    struct{}                   `codec:",toarray" kces:"nil=24;widths=21,22,27,28,31,32"` // 强制按数组编码并声明游戏已知历史宽度与固定 nil Key / Forces array encoding and declares known game widths plus the fixed nil key
	Version                    int32                      `json:"version"`                                          // 存储的版本；当前游戏 FixVersion 为 1005 / Stored version; current-game FixVersion is 1005
	GUID                       uint64                     `json:"guid"`                                             // 来源 GUID 字符串的大小写无关 FNV-1a 64 位哈希 / Case-insensitive FNV-1a 64-bit hash of the source GUID
	ID                         uint64                     `json:"id"`                                               // Menu 文件名的大小写无关 FNV-1a 64 位哈希 / Case-insensitive FNV-1a 64-bit hash of the menu filename
	FileName                   *string                    `json:"fileName"`                                         // 可空菜单文件名，如 xxx.menu / Nullable menu file name such as xxx.menu
	ItemName                   *string                    `json:"itemName"`                                         // 可空物品显示名称 / Nullable display name of the item
	IconFileName               *string                    `json:"iconFileName"`                                     // 可空图标文件名 / Nullable icon file name
	InfoText                   *string                    `json:"infoText"`                                         // 可空说明文本 / Nullable description text
	Priority                   int32                      `json:"priority"`                                         // 优先级 / Priority
	ParentID                   uint64                     `json:"parentId"`                                         // 父菜单 ID，0 表示无父级 / Parent menu ID, zero means no parent
	IsMan                      bool                       `json:"isMan"`                                            // 是否为男性用 / Whether this menu is for male characters
	IsDiff                     bool                       `json:"isDiff"`                                           // 是否为差分 / Whether this menu is a variation
	IsDelete                   bool                       `json:"isDelete"`                                         // 是否为删除项 / Whether this menu removes an item
	Commands                   []*Command                 `json:"commandList"`                                      // 可空命令对象数组 / Array of nullable command objects
	CategoryText               *string                    `json:"categoryText"`                                     // 可空分类文本，通常为 MPN 枚举名 / Nullable category text, usually an MPN enum name
	ColorSetText               *string                    `json:"colorSetText"`                                     // 可空颜色集文本，通常为 MPN 枚举名 / Nullable color-set text, usually an MPN enum name
	DefineTagNames             uint64                     `json:"defineTagNames"`                                   // DEFINE 标志位 / DEFINE flag bits
	PreMulTexDatas             map[uint64]*PreMulTexDatas `json:"preMulTexDatas"`                                   // 可空预乘纹理数据对象表 / Map of nullable pre-multiplied texture data objects
	ColvariFileNameExp         *string                    `json:"colvariFileNameExp"`                               // 可空颜色变体文件名表达式 / Nullable color-variant file-name expression
	ColvariInfo                *Colvari                   `json:"colvariInfo"`                                      // 颜色变体信息 / Color-variant information
	SrcFileHashCRC32           uint32                     `json:"srcFileHashCRC32"`                                 // 源文件 CRC32 哈希 / Source-file CRC32 hash
	DefineFirst                uint64                     `json:"defineFirst"`                                      // 首要 DEFINE 标志位 / Primary DEFINE flag bits
	PartsVer                   *TupleStringInt            `json:"partsVer"`                                         // 部件版本元组 / Parts version tuple
	IsRecommendMan             bool                       `json:"isRecommendMan"`                                   // 是否推荐男性使用 / Whether male use is recommended
	TargetBodyType             int32                      `json:"targetBodyType"`                                   // 目标体型枚举，0=None, 1=Woman, 2=Man / Target body-type enum, 0=None, 1=Woman, 2=Man
	Attribute                  uint64                     `json:"attribute"`                                        // 属性标志位 / Attribute flag bits
	HideInEdit                 bool                       `json:"hideInEdit"`                                       // 是否在编辑界面隐藏 / Whether hidden in edit mode
	ToeLockSlotId              *string                    `json:"toeLockSlotId"`                                    // 可空脚趾锁定槽位 ID / Nullable toe-lock slot ID
	ExportModelFormTextureName *string                    `json:"exportModelFormTextureName"`                       // 可空导出模型纹理名 / Nullable exported model texture name
	IsHarayureAvailable        int32                      `json:"isHarayureAvailable"`                              // 腹揺れ可用性枚举，0=None, 1=Available, 2=Disable / Belly-jiggle availability enum, 0=None, 1=Available, 2=Disable
	SkirtPhys                  int32                      `json:"skirt_phys"`                                       // 裙子物理类型 / Skirt physics type
	HairMake                   *HairMake                  `json:"hairMake"`                                         // KCES2 HairMake 导出信息 / KCES2 HairMake export information
	IndexedArrayWidth          int32                      `codec:"-" json:"indexedArrayWidth,omitempty"`            // 解码时记录的线格式数组宽度，并非游戏成员 / Wire array width recorded during decoding, not a game member
}

// HairMake 表示 KCES2 Menu 中的 HairMake 导出信息 / HairMake represents HairMake export information embedded in a KCES2 Menu
type HairMake struct {
	_struct                         struct{}  `codec:",toarray"`                             // 强制按数组编码 / Forces array encoding
	Version                         int32     `json:"version"`                               // 版本号，当前为 1001 / Version value, currently 1001
	ExportedGUID                    *string   `json:"exportedGuid"`                          // 导出 GUID / Exported GUID
	ExportedHairBuildVersion        int32     `json:"exportedHairBuildVersion"`              // 导出 HairMake 构建版本 / Exported HairMake build version
	ExportedHairGameVersion         int32     `json:"exportedHairGameVersion"`               // 导出游戏版本 / Exported game version
	SuspendedSaveFileName           *string   `json:"suspendedSaveFileName"`                 // 继续编辑存档文件名 / Continue-editing save filename
	MaterialPerOriginalMenuFileName []*string `json:"materialPerOriginalMenuFileName"`       // 原始菜单文件名 / Original menu filenames
	MaterialPerOriginalMenuVersion  []int32   `json:"materialPerOriginalMenuVersion"`        // 原始菜单版本 / Original menu versions
	IndexedArrayWidth               int32     `codec:"-" json:"indexedArrayWidth,omitempty"` // 解码时记录的线格式数组宽度，并非游戏成员 / Wire array width recorded during decoding, not a game member
}

const (
	menuLegacyWidth   = 31
	menuKCES2Width    = 32
	hairMakeWireWidth = 7
)

// MessagePackIndexedObjectWidth 返回 Menu 应写出的 indexed-array 宽度
// MessagePackIndexedObjectWidth returns the indexed-array width that Menu should emit
func (v *Menu) MessagePackIndexedObjectWidth() int32 {
	if v.IndexedArrayWidth == 0 {
		return menuLegacyWidth
	}
	return v.IndexedArrayWidth
}

// SetMessagePackIndexedObjectWidth 设置 Menu 应写出的 indexed-array 宽度
// SetMessagePackIndexedObjectWidth sets the indexed-array width that Menu should emit
func (v *Menu) SetMessagePackIndexedObjectWidth(width int32) {
	v.IndexedArrayWidth = width
}

// MessagePackIndexedObjectWidth 返回 HairMake 应写出的 indexed-array 宽度
// MessagePackIndexedObjectWidth returns the indexed-array width that HairMake should emit
func (v *HairMake) MessagePackIndexedObjectWidth() int32 {
	if v.IndexedArrayWidth == 0 {
		return hairMakeWireWidth
	}
	return v.IndexedArrayWidth
}

// SetMessagePackIndexedObjectWidth 设置 HairMake 应写出的 indexed-array 宽度
// SetMessagePackIndexedObjectWidth sets the indexed-array width that HairMake should emit
func (v *HairMake) SetMessagePackIndexedObjectWidth(width int32) {
	v.IndexedArrayWidth = width
}

// Command 表示一条 Parts.Menu 菜单命令
// MessagePack indexed array 依次保存命令类型和参数数组
// Command represents one Parts.Menu command
// Its MessagePack indexed array stores the command type followed by the argument array
type Command struct {
	_struct struct{}  `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int32     `json:"type"`      // 命令类型枚举值 / Command type enum value
	Args    []*string `json:"args"`      // 可空命令参数字符串数组 / Array of nullable command-argument strings
}

// MenuAssets 表示菜单资源容器
// 对应 C# Parts.MenuAssets，继承自 SerializPartsAssets<Menu>
// MessagePack indexed array 依次保存容器文件名和菜单数组
// MenuAssets represents a menu asset container
// It matches C# Parts.MenuAssets derived from SerializPartsAssets<Menu>
// Its MessagePack indexed array stores the container filename followed by the menu array
type MenuAssets struct {
	_struct  struct{} `codec:",toarray"`  // 强制按数组编码 / Forces array encoding
	FileName *string  `json:"fileName"`   // 可空容器文件名，如 xxx.menuassets / Nullable container file name such as xxx.menuassets
	Assets   []*Menu  `json:"assetArray"` // 可空菜单对象数组 / Array of nullable menu objects
}

const menuFixVersion = 1005

const (
	preMulTexDatasFixVersion = 1001
	colvariFixVersion        = 1000
	colvariDataFixVersion    = 1000
)

// DecodeMenuAssets 从 Lz4BlockArray 压缩的 MessagePack 数据解码 MenuAssets
// DecodeMenuAssets decodes MenuAssets from Lz4BlockArray-compressed MessagePack data
func DecodeMenuAssets(data []byte) (*MenuAssets, error) {
	var assets *MenuAssets
	if err := decodeCompressedMsgpack(data, &assets, "MenuAssets"); err != nil {
		return nil, err
	}
	return assets, nil
}

// EncodeMenuAssets 将 MenuAssets 编码为 Lz4BlockArray 压缩的 MessagePack 数据
// EncodeMenuAssets encodes MenuAssets as Lz4BlockArray-compressed MessagePack data
func EncodeMenuAssets(assets *MenuAssets) ([]byte, error) {
	if assets == nil {
		return encodeCompressedMsgpack(nil, "MenuAssets")
	}
	normalized := *assets
	normalized.Assets = cloneSlicePreserveNil(assets.Assets)
	return encodeCompressedMsgpack(&normalized, "MenuAssets")
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 Menu
// CodecEncodeSelf encodes Menu using the shared indexed-object rules
func (v Menu) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Menu
// CodecDecodeSelf decodes Menu using the shared indexed-object rules
func (v *Menu) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 HairMake
// CodecEncodeSelf encodes HairMake using the shared indexed-object rules
func (v HairMake) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 HairMake
// CodecDecodeSelf decodes HairMake using the shared indexed-object rules
func (v *HairMake) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Command
// CodecEncodeSelf encodes Command using the shared indexed-object rules
func (v Command) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Command
// CodecDecodeSelf decodes Command using the shared indexed-object rules
func (v *Command) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 MenuAssets
// CodecEncodeSelf encodes MenuAssets using the shared indexed-object rules
func (v MenuAssets) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 MenuAssets
// CodecDecodeSelf decodes MenuAssets using the shared indexed-object rules
func (v *MenuAssets) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// NewMenu 创建使用当前固定版本及游戏默认分类文本的新菜单
// NewMenu creates a new menu with the current fixed version and the game's default category text
func NewMenu() *Menu {
	defaultMPN := "null_mpn"
	return &Menu{
		Version:      menuFixVersion,
		CategoryText: &defaultMPN,
		ColorSetText: &defaultMPN,
	}
}

// NewKCES2Menu 创建使用 KCES2 32 槽布局的新菜单
// NewKCES2Menu creates a new menu using the 32-slot KCES2 layout
func NewKCES2Menu() *Menu {
	menu := NewMenu()
	menu.SetMessagePackIndexedObjectWidth(menuKCES2Width)
	return menu
}

// NewHairMake 创建使用当前固定版本的新 HairMake 导出信息
// NewHairMake creates new HairMake export information using the current fixed version
func NewHairMake() *HairMake {
	return &HairMake{Version: 1001}
}
