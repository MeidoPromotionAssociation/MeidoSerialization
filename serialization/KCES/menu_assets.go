package KCES

// Menu 表示菜单数据 / Menu represents KCES Parts.Menu data
// 对应 C# Parts.Menu，继承自 AMessagePackSerializationVersionControlIntKey / Matches C# Parts.Menu, derived from AMessagePackSerializationVersionControlIntKey
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
type Menu struct {
	_struct                    struct{}                  `codec:",toarray"`                               // 强制按数组编码 / Forces array encoding
	Version                    int                       `json:"version"`                                 // 版本号，固定为 1005 / Version value, fixed to 1005
	GUID                       uint64                    `json:"guid"`                                    // 全局唯一标识 / Global unique identifier
	ID                         uint64                    `json:"id"`                                      // 菜单 ID / Menu ID
	FileName                   string                    `json:"fileName"`                                // 菜单文件名，如 xxx.menu / Menu file name such as xxx.menu
	ItemName                   string                    `json:"itemName"`                                // 物品显示名称 / Display name of the item
	IconFileName               string                    `json:"iconFileName"`                            // 图标文件名 / Icon file name
	InfoText                   string                    `json:"infoText"`                                // 说明文本 / Description text
	Priority                   int                       `json:"priority"`                                // 优先级 / Priority
	ParentID                   uint64                    `json:"parentId"`                                // 父菜单 ID，0 表示无父级 / Parent menu ID, zero means no parent
	IsMan                      bool                      `json:"isMan"`                                   // 是否为男性用 / Whether this menu is for male characters
	IsDiff                     bool                      `json:"isDiff"`                                  // 是否为差分 / Whether this menu is a variation
	IsDelete                   bool                      `json:"isDelete"`                                // 是否为删除项 / Whether this menu removes an item
	Commands                   []Command                 `json:"commandList"`                             // 命令列表 / Command list
	CategoryText               string                    `json:"categoryText"`                            // 分类文本，通常为 MPN 枚举名 / Category text, usually an MPN enum name
	ColorSetText               string                    `json:"colorSetText"`                            // 颜色集文本，通常为 MPN 枚举名 / Color-set text, usually an MPN enum name
	DefineTagNames             uint64                    `json:"defineTagNames"`                          // DEFINE 标志位 / DEFINE flag bits
	PreMulTexDatas             map[uint64]PreMulTexDatas `json:"preMulTexDatas"`                          // 预乘纹理数据表 / Pre-multiplied texture data map
	ColvariFileNameExp         string                    `json:"colvariFileNameExp"`                      // 颜色变体文件名表达式 / Color-variant file-name expression
	ColvariInfo                *Colvari                  `json:"colvariInfo"`                             // 颜色变体信息 / Color-variant information
	SrcFileHashCRC32           uint32                    `json:"srcFileHashCRC32"`                        // 源文件 CRC32 哈希 / Source-file CRC32 hash
	DefineFirst                uint64                    `json:"defineFirst"`                             // 首要 DEFINE 标志位 / Primary DEFINE flag bits
	PartsVer                   *TupleStringInt           `json:"partsVer"`                                // 部件版本元组 / Parts version tuple
	IsRecommendMan             bool                      `json:"isRecommendMan"`                          // 是否推荐男性使用 / Whether male use is recommended
	TargetBodyType             int                       `json:"targetBodyType"`                          // 目标体型枚举，0=None, 1=Woman, 2=Man / Target body-type enum, 0=None, 1=Woman, 2=Man
	Reserved24                 interface{}               `codec:"reserved24" json:"reserved24,omitempty"` // C# 当前未使用的 Key(24)，用于维持数组对齐 / Currently unused C# Key(24), kept for array alignment
	Attribute                  uint64                    `json:"attribute"`                               // 属性标志位 / Attribute flag bits
	HideInEdit                 bool                      `json:"hideInEdit"`                              // 是否在编辑界面隐藏 / Whether hidden in edit mode
	ToeLockSlotId              string                    `json:"toeLockSlotId"`                           // 脚趾锁定槽位 ID / Toe-lock slot ID
	ExportModelFormTextureName string                    `json:"exportModelFormTextureName"`              // 导出模型纹理名 / Exported model texture name
	IsHarayureAvailable        int                       `json:"isHarayureAvailable"`                     // 腹揺れ可用性枚举，0=None, 1=Available, 2=Disable / Belly-jiggle availability enum, 0=None, 1=Available, 2=Disable
	SkirtPhys                  int                       `json:"skirt_phys"`                              // 裙子物理类型 / Skirt physics type
}

// Command 表示菜单命令 / Command represents one Parts.Menu command
// MessagePack indexed array: [type, args]
type Command struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Type    int      `json:"type"`      // 命令类型枚举值 / Command type enum value
	Args    []string `json:"args"`      // 命令参数字符串数组 / Command argument string array
}

// MenuAssets 表示菜单资源容器 / MenuAssets represents a menu asset container
// 对应 C# Parts.MenuAssets，继承自 SerializPartsAssets<Menu> / Matches C# Parts.MenuAssets derived from SerializPartsAssets<Menu>
// MessagePack indexed array: [fileName, assetArray]
type MenuAssets struct {
	_struct  struct{} `codec:",toarray"`  // 强制按数组编码 / Forces array encoding
	FileName string   `json:"fileName"`   // 容器文件名，如 xxx.menuassets / Container file name such as xxx.menuassets
	Assets   []Menu   `json:"assetArray"` // 菜单数组 / Menu array
}

const menuFixVersion = 1005

// DecodeMenuAssets 从 Lz4Block 压缩的 MessagePack 数据解码 MenuAssets
func DecodeMenuAssets(data []byte) (*MenuAssets, error) {
	assets := &MenuAssets{}
	if err := decodeCompressedMsgpack(data, assets, "MenuAssets"); err != nil {
		return nil, err
	}
	return assets, nil
}

// EncodeMenuAssets 将 MenuAssets 编码为 Lz4Block 压缩的 MessagePack 数据
func EncodeMenuAssets(assets *MenuAssets) ([]byte, error) {
	normalized := *assets
	normalized.Assets = append([]Menu(nil), assets.Assets...)
	for i := range normalized.Assets {
		if normalized.Assets[i].Version == 0 {
			normalized.Assets[i].Version = menuFixVersion
		}
	}
	return encodeCompressedMsgpack(&normalized, "MenuAssets")
}
