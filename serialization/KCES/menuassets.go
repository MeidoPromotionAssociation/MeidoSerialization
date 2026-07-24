package KCES

// .menuassets
// KCES 菜单资源容器，在 .aba 的 TextAsset 中保存 Parts.Menu 数组
// 载荷使用 LZ4 Block Array 压缩的 MessagePack indexed-array，当前 Menu 固定版本为 1005
// .menuassets
// KCES menu-resource container storing an array of Parts.Menu values in a TextAsset inside an .aba file
// The payload is an LZ4 Block Array-compressed MessagePack indexed array, with current Menu fixed version 1005

// MessagePack indexed array 布局（31 个字段，Key(0)~Key(30)）/ MessagePack indexed-array layout with 31 fields from Key(0) to Key(30)
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

// Menu 表示 Parts.Menu 的菜单数据
// 对应 C# Parts.Menu，继承自 AMessagePackSerializationVersionControlIntKey
// MessagePack indexed array 包含 Key(0) 至 Key(30) 的 31 个槽位，Key(24) 在 C# 中没有成员
// Menu represents menu data from Parts.Menu
// It matches C# Parts.Menu derived from AMessagePackSerializationVersionControlIntKey
// Its MessagePack indexed array contains 31 slots from Key(0) through Key(30), with no C# member at Key(24)
type Menu struct {
	_struct                    struct{}                   `codec:",toarray" kces:"nil=24;widths=21,22,27,28,31"` // 强制按数组编码并声明游戏已知历史宽度与固定 nil Key / Forces array encoding and declares known game widths plus the fixed nil key
	Version                    int32                      `json:"version"`                                       // 存储的版本；当前游戏 FixVersion 为 1005 / Stored version; current-game FixVersion is 1005
	GUID                       uint64                     `json:"guid"`                                          // 全局唯一标识 / Global unique identifier
	ID                         uint64                     `json:"id"`                                            // 菜单 ID / Menu ID
	FileName                   *string                    `json:"fileName"`                                      // 可空菜单文件名，如 xxx.menu / Nullable menu file name such as xxx.menu
	ItemName                   *string                    `json:"itemName"`                                      // 可空物品显示名称 / Nullable display name of the item
	IconFileName               *string                    `json:"iconFileName"`                                  // 可空图标文件名 / Nullable icon file name
	InfoText                   *string                    `json:"infoText"`                                      // 可空说明文本 / Nullable description text
	Priority                   int32                      `json:"priority"`                                      // 优先级 / Priority
	ParentID                   uint64                     `json:"parentId"`                                      // 父菜单 ID，0 表示无父级 / Parent menu ID, zero means no parent
	IsMan                      bool                       `json:"isMan"`                                         // 是否为男性用 / Whether this menu is for male characters
	IsDiff                     bool                       `json:"isDiff"`                                        // 是否为差分 / Whether this menu is a variation
	IsDelete                   bool                       `json:"isDelete"`                                      // 是否为删除项 / Whether this menu removes an item
	Commands                   []*Command                 `json:"commandList"`                                   // 可空命令对象数组 / Array of nullable command objects
	CategoryText               *string                    `json:"categoryText"`                                  // 可空分类文本，通常为 MPN 枚举名 / Nullable category text, usually an MPN enum name
	ColorSetText               *string                    `json:"colorSetText"`                                  // 可空颜色集文本，通常为 MPN 枚举名 / Nullable color-set text, usually an MPN enum name
	DefineTagNames             uint64                     `json:"defineTagNames"`                                // DEFINE 标志位 / DEFINE flag bits
	PreMulTexDatas             map[uint64]*PreMulTexDatas `json:"preMulTexDatas"`                                // 可空预乘纹理数据对象表 / Map of nullable pre-multiplied texture data objects
	ColvariFileNameExp         *string                    `json:"colvariFileNameExp"`                            // 可空颜色变体文件名表达式 / Nullable color-variant file-name expression
	ColvariInfo                *Colvari                   `json:"colvariInfo"`                                   // 颜色变体信息 / Color-variant information
	SrcFileHashCRC32           uint32                     `json:"srcFileHashCRC32"`                              // 源文件 CRC32 哈希 / Source-file CRC32 hash
	DefineFirst                uint64                     `json:"defineFirst"`                                   // 首要 DEFINE 标志位 / Primary DEFINE flag bits
	PartsVer                   *TupleStringInt            `json:"partsVer"`                                      // 部件版本元组 / Parts version tuple
	IsRecommendMan             bool                       `json:"isRecommendMan"`                                // 是否推荐男性使用 / Whether male use is recommended
	TargetBodyType             int32                      `json:"targetBodyType"`                                // 目标体型枚举，0=None, 1=Woman, 2=Man / Target body-type enum, 0=None, 1=Woman, 2=Man
	Attribute                  uint64                     `json:"attribute"`                                     // 属性标志位 / Attribute flag bits
	HideInEdit                 bool                       `json:"hideInEdit"`                                    // 是否在编辑界面隐藏 / Whether hidden in edit mode
	ToeLockSlotId              *string                    `json:"toeLockSlotId"`                                 // 可空脚趾锁定槽位 ID / Nullable toe-lock slot ID
	ExportModelFormTextureName *string                    `json:"exportModelFormTextureName"`                    // 可空导出模型纹理名 / Nullable exported model texture name
	IsHarayureAvailable        int32                      `json:"isHarayureAvailable"`                           // 腹揺れ可用性枚举，0=None, 1=Available, 2=Disable / Belly-jiggle availability enum, 0=None, 1=Available, 2=Disable
	SkirtPhys                  int32                      `json:"skirt_phys"`                                    // 裙子物理类型 / Skirt physics type
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
