package KCES

import "fmt"

// .undressdat (UndressCore.ArchiveTarget)
// KCES2 内衣「扒开／下拉」交互系统的设置数据，原生文件是 Unity JsonUtility 明文 JSON 文档
// WearSetuper 用 JsonUtility.FromJson<ArchiveTarget> 读取它，并配对读取同名 .undresspdat 预计算缓存
//
// 文件描述的是一件内衣可以被怎样剥开：layers 声明剥法通道，dataGroup 把网格顶点分成一圈圈可拉动的
// 组，hPeelLimits/vPeelExInfo/commonPeelInfo 给出限位与形变参数
//
// 载入时游戏会先调用 ArchiveTargetFormatter.CheckUpdate 按 format 做版本迁移，本库只忠实保留原值，
// 不执行任何迁移
//
// .undressdat (UndressCore.ArchiveTarget)
// Setup data for the KCES2 underwear peel-aside/pull-down interaction system, whose native file is a Unity JsonUtility plain-JSON document
// WearSetuper reads it with JsonUtility.FromJson<ArchiveTarget> together with the matching .undresspdat precomputed cache
//
// The file describes how one garment can be peeled: layers declare the peel channels, dataGroup splits
// mesh vertices into pullable rings of vertices, and hPeelLimits/vPeelExInfo/commonPeelInfo supply the
// limits and deformation parameters
//
// While loading, the game first calls ArchiveTargetFormatter.CheckUpdate to migrate by format; this
// library faithfully preserves the stored values and performs no migration

const KCESUndressDataExtension = ".undressdat"

// UndressArchiveTarget 表示 UndressCore.ArchiveTarget 的 Unity JSON 表示，成员顺序与 JsonUtility 的写出顺序一致
// UndressArchiveTarget represents the Unity JSON form of UndressCore.ArchiveTarget with members ordered as JsonUtility writes them
type UndressArchiveTarget struct {
	Format                      *string                   `json:"format,omitempty"`                      // 数据格式版本，形如 1.2.2，ArchiveTargetFormatter 据此迁移 / Data format version such as 1.2.2, used by ArchiveTargetFormatter to migrate
	EditVer                     *int32                    `json:"editVer,omitempty"`                     // 写出该文件的编辑器版本 / Version of the editor that wrote the file
	MeshRelPath                 *string                   `json:"meshRelPath,omitempty"`                 // 网格相对路径，缺少 peelCategory 时用于推断分类 / Mesh-relative path, also used to infer the category when peelCategory is missing
	FbxName                     *string                   `json:"fbxName,omitempty"`                     // 目标 FBX 名，setupDataType 为 None 时由内衣资源名回填 / Target FBX name, backfilled from the garment resource name when setupDataType is None
	SetupDataType               *int32                    `json:"setupDataType,omitempty"`               // 设置数据种类枚举 eSeupDataType：0 None、1 Body、2 Pants、3 Bra / Setup data kind enum eSeupDataType: 0 None, 1 Body, 2 Pants, 3 Bra
	Layers                      *[]UndressLayer           `json:"layers,omitempty"`                      // 剥法通道列表，游戏载入时补齐到 MyConstants.MAXLayerCount 即 15 条 / Peel channel list, which the game pads to MyConstants.MAXLayerCount, that is 15 entries, while loading
	SubMeshRootVertexIndices    *[]int32                  `json:"subMeshRootVertexIndices,omitempty"`    // 各子网格的根顶点索引 / Root vertex index of each sub-mesh
	SubMeshRootVertexSubIndices *[]UndressSubMeshSlideSub `json:"subMeshRootVertexSubIndices,omitempty"` // 各子网格根顶点的附加滑动索引 / Additional slide indices for each sub-mesh root vertex
	DataGroup                   *[]UndressGroup           `json:"dataGroup,omitempty"`                   // 顶点分组列表，每组是一圈可拉动的顶点 / Vertex group list where each group is one pullable ring of vertices
	TempIndex                   *int32                    `json:"tempIndex,omitempty"`                   // 编辑器分配新组标签时使用的计数器 / Counter the editor uses when allocating a new group label
	PeelCategory                *int32                    `json:"peelCategory,omitempty"`                // 剥离分类枚举 MyEnum.PeelCategory：0 Bra、1 Shorts、2 Other、3 Body、99 None / Peel category enum MyEnum.PeelCategory: 0 Bra, 1 Shorts, 2 Other, 3 Body, 99 None
	HPeelLimits                 *UndressPeelLimits        `json:"hPeelLimits,omitempty"`                 // 横向剥离的限位与逐组阈值 / Horizontal peel limits and per-group thresholds
	VPeelExInfo                 *UndressVPeelExInfo       `json:"vPeelExInfo,omitempty"`                 // 纵向剥离的附加形变参数 / Additional vertical peel deformation parameters
	CommonPeelInfo              *UndressCommonPeelInfo    `json:"commonPeelInfo,omitempty"`              // 全部剥法共用的参数 / Parameters shared by every peel mode
}

// UndressLayer 表示 UndressCore.OneLayer，即一条剥法通道
// UndressLayer represents UndressCore.OneLayer, one peel channel
type UndressLayer struct {
	Label        *string `json:"label,omitempty"`        // 通道显示名，可为空 / Channel display name, may be empty
	FixMode      *int32  `json:"fixMode,omitempty"`      // 固定模式枚举 FixMode：0 Plain、1 Pre、2 Fix / Fix mode enum FixMode: 0 Plain, 1 Pre, 2 Fix
	AutoSortMode *int32  `json:"autoSortMode,omitempty"` // 组内顶点排序方向枚举 AutoSortMode：0 None、1 XAsc、2 XDesc、3 YAsc、4 YDesc / In-group vertex sort direction enum AutoSortMode: 0 None, 1 XAsc, 2 XDesc, 3 YAsc, 4 YDesc
	UseMode      *int32  `json:"useMode,omitempty"`      // 剥法枚举 UseMode：0 None、10 纵、14 纵逆、20/21 前横、30/31 股间、40/41 后横、50 已废弃、90 脱衣引导 / Peel mode enum UseMode: 0 None, 10 vertical, 14 vertical reverse, 20/21 front sideways, 30/31 crotch, 40/41 back sideways, 50 obsolete, 90 undress guide
}

// UndressSubMeshSlideSub 表示 ArchiveTarget.SubMeshSlideSub，三个分量全为负表示该项无效
// UndressSubMeshSlideSub represents ArchiveTarget.SubMeshSlideSub, where all three negative components mean the item is invalid
type UndressSubMeshSlideSub struct {
	Sub1 *int32 `json:"sub1,omitempty"` // 附加滑动顶点索引 1，负值表示未设置 / Additional slide vertex index 1, negative when unset
	Sub2 *int32 `json:"sub2,omitempty"` // 附加滑动顶点索引 2，负值表示未设置 / Additional slide vertex index 2, negative when unset
	Sub3 *int32 `json:"sub3,omitempty"` // 附加滑动顶点索引 3，负值表示未设置 / Additional slide vertex index 3, negative when unset
}

// UndressGroup 表示 UndressCore.OneGroup，即一圈可拉动顶点及其行为标记
// UndressGroup represents UndressCore.OneGroup, one pullable ring of vertices together with its behavior flags
type UndressGroup struct {
	Label           *string           `json:"label,omitempty"`           // 组标签，形如 Group_0000L；标签在不同 layer 之间会重复，(layer,label) 才是 .undresspdat 回指本组时使用的键 / Group label such as Group_0000L; labels repeat across layers, and the (layer, label) pair is the key .undresspdat uses to refer back to this group
	IsInactive      *bool             `json:"isInactive,omitempty"`      // 是否停用该组 / Whether the group is disabled
	IsFloat         *bool             `json:"isFloat,omitempty"`         // 该组是否浮起 / Whether the group floats
	IsOverPeel      *bool             `json:"isOverPeel,omitempty"`      // 是否允许超出常规范围继续剥离 / Whether peeling may continue past the normal range
	IsFloatGuide    *bool             `json:"isFloatGuide,omitempty"`    // 该组是否作为浮起引导线 / Whether the group itself serves as a float guide line
	IsSolid         *bool             `json:"isSolid,omitempty"`         // 该组是否作为实体处理 / Whether the group is treated as solid
	IsPaste         *bool             `json:"isPaste,omitempty"`         // 该组是否贴附到引导线 / Whether the group is pasted onto its guide lines
	IsToForce       *bool             `json:"isToForce,omitempty"`       // 该组是否强制参与 / Whether the group participates unconditionally
	SolidPriority   *int32            `json:"solidPriority,omitempty"`   // 实体处理的优先级 / Priority used when solid handling applies
	FloatGuideLine0 *string           `json:"floatGuideLine0,omitempty"` // 浮起引导线组标签之一，空串表示未设置 / One float guide line group label, empty when unset
	FloatGuideLine1 *string           `json:"floatGuideLine1,omitempty"` // 浮起引导线组标签之二，空串表示未设置 / The other float guide line group label, empty when unset
	PasteGuideLine0 *string           `json:"pasteGuideLine0,omitempty"` // 贴附引导线组标签之一，空串表示未设置 / One paste guide line group label, empty when unset
	PasteGuideLine1 *string           `json:"pasteGuideLine1,omitempty"` // 贴附引导线组标签之二，空串表示未设置 / The other paste guide line group label, empty when unset
	Weights         *[]float32        `json:"weights,omitempty"`         // 组内逐顶点权重，样本中通常为空数组 / Per-vertex weights inside the group, usually an empty array in samples
	Vertices        *[]Vector3        `json:"vertices,omitempty"`        // 组内顶点坐标，样本中通常为空数组 / Vertex positions inside the group, usually an empty array in samples
	Layer           *int32            `json:"layer,omitempty"`           // 所属层号：0 到 14 对应 layers 的下标，100 起是 MyEnum.LayerCategory 的子网格层，样本中还出现 15 到 17 这些超出通道上限的编辑层 / Owning layer number: 0 through 14 index layers, 100 and above are the MyEnum.LayerCategory sub-mesh layers, and samples also use 15 through 17, which are above the channel limit
	VCategory       *int32            `json:"vCategory,omitempty"`       // 边缘分类枚举 RetensionCategory：0 无、1 右前、2 右后、3 左前、4 左后、5 腹 / Edge category enum RetensionCategory: 0 none, 1 right front, 2 right back, 3 left front, 4 left back, 5 belly
	FloatMode       *uint32           `json:"floatMode,omitempty"`       // 浮起模式，保存 FloatMode 的无符号取值：0 NONE、1 前右、2 前左、3 后右、4 后左 / Float mode holding the unsigned FloatMode value: 0 NONE, 1 front right, 2 front left, 3 back right, 4 back left
	ExFixeds        *[]UndressExFixed `json:"exFixeds,omitempty"`        // 附加固定顶点对，样本中通常为空数组 / Additional fixed vertex pairs, usually an empty array in samples
	Indices         *[]int32          `json:"indices,omitempty"`         // 组内网格顶点索引，按 autoSortMode 排成一条拉动轨道 / Mesh vertex indices of the group, ordered into a pull rail by autoSortMode
}

// UndressExFixed 表示 OneGroup.ExFixed，一对附加固定顶点
// UndressExFixed represents OneGroup.ExFixed, one pair of additional fixed vertices
type UndressExFixed struct {
	FromIndex *int32 `json:"fromIndex,omitempty"` // 源顶点索引 / Source vertex index
	ToIndex   *int32 `json:"toIndex,omitempty"`   // 目标顶点索引 / Destination vertex index
}

// UndressPeelLimits 表示 ArchiveTarget.PeelLimits，横向剥离的限位集合
// format_version 有独立于顶层 format 的迁移链（PeelLimits.CheckVer，0 到 1 到 2），且 1 到 2 会就地改写 thrs 的数值
// UndressPeelLimits represents ArchiveTarget.PeelLimits, the horizontal peel limit set
// format_version has its own migration chain independent of the top-level format (PeelLimits.CheckVer, 0 to 1 to 2), and 1 to 2 rewrites the thrs values in place
type UndressPeelLimits struct {
	FormatVersion     *int32                     `json:"format_version,omitempty"`    // 限位结构自身的版本号，键名的下划线拼写沿用线格式 / Version of the limit structure itself, with the underscored key spelled as stored
	Heads             *[]float32                 `json:"heads,omitempty"`             // 逐档剥离进度下限，游戏保证 heads 不大于同档 tails / Per-step peel progress lower bound, which the game keeps no greater than the tails value of the same step
	Thrs              *[]UndressPeelThreshold    `json:"thrs,omitempty"`              // 逐组剥离阈值 / Per-group peel threshold
	ManualLimitPac    *UndressPeelLimitRange     `json:"manualLimitPac,omitempty"`    // 手动限位区间，valid 为真时覆盖自动限位 / Manual limit range that overrides the automatic limits when valid is true
	Tails             *[]float32                 `json:"tails,omitempty"`             // 逐档剥离进度上限 / Per-step peel progress upper bound
	HPeelSelectLimits *[]UndressHPeelSelectLimit `json:"hPeelSelectLimits,omitempty"` // 逐组是否参与横向剥离 / Whether each group participates in horizontal peeling
}

// UndressPeelThreshold 表示 PeelLimits.ThrSet，一条逐组剥离阈值
// UndressPeelThreshold represents PeelLimits.ThrSet, one per-group peel threshold
type UndressPeelThreshold struct {
	Label *string  `json:"label,omitempty"` // 目标组标签，需与 dataGroup 中的 label 对应 / Target group label, which must match a label in dataGroup
	Thr   *float32 `json:"thr,omitempty"`   // 该组的剥离阈值 / Peel threshold of the group
}

// UndressPeelLimitRange 表示 PeelLimits.LimitPac，一段手动限位区间
// UndressPeelLimitRange represents PeelLimits.LimitPac, one manual limit range
type UndressPeelLimitRange struct {
	Valid *bool    `json:"valid,omitempty"` // 是否启用该手动区间 / Whether the manual range is enabled
	Begin *float32 `json:"begin,omitempty"` // 区间起点 / Range start
	End   *float32 `json:"end,omitempty"`   // 区间终点 / Range end
}

// UndressHPeelSelectLimit 表示 ArchiveTarget.HPeelSelectLimit，一条逐组横向剥离开关
// UndressHPeelSelectLimit represents ArchiveTarget.HPeelSelectLimit, one per-group horizontal peel switch
type UndressHPeelSelectLimit struct {
	Label *string `json:"label,omitempty"` // 目标组标签，空串表示该项无效 / Target group label, empty when the item is invalid
	Value *int32  `json:"value,omitempty"` // 0 表示该组不参与横向剥离 / A value of 0 excludes the group from horizontal peeling
}

// UndressVPeelExInfo 表示 ArchiveTarget.VPeelExInfo，纵向剥离的附加形变参数
// 单位由游戏侧的属性访问器确定：retension 三项是百分比，folding 三项是毫米，两项 adjustLength 直接是 Unity 单位
// UndressVPeelExInfo represents ArchiveTarget.VPeelExInfo, the additional vertical peel deformation parameters
// Units follow the game-side property accessors: the three retension members are percentages, the three folding members are millimeters, and the two adjustLength members are already in Unity units
type UndressVPeelExInfo struct {
	FrontAdjustLength              *float32 `json:"frontAdjustLength,omitempty"`              // 正面调整长度，Unity 单位 / Front adjustment length in Unity units
	BackAdjustLength               *float32 `json:"backAdjustLength,omitempty"`               // 背面调整长度，Unity 单位 / Back adjustment length in Unity units
	RetensionWidthPar              *float32 `json:"retensionWidthPar,omitempty"`              // 边缘勒入宽度百分比，游戏侧除以 100 使用 / Edge bite-in width percentage, divided by 100 on the game side
	RetensionDepthFrontPar         *float32 `json:"retensionDepthFrontPar,omitempty"`         // 正面边缘勒入深度百分比，游戏侧除以 100 使用 / Front edge bite-in depth percentage, divided by 100 on the game side
	RetensionDepthBackPar          *float32 `json:"retensionDepthBackPar,omitempty"`          // 背面边缘勒入深度百分比，游戏侧除以 100 使用 / Back edge bite-in depth percentage, divided by 100 on the game side
	VPeelVerticalFoldingWidthFront *float32 `json:"vPeelVerticalFoldingWidthFront,omitempty"` // 正面纵向折叠宽度，毫米 / Front vertical folding width in millimeters
	VPeelVerticalFoldingWidthBack  *float32 `json:"vPeelVerticalFoldingWidthBack,omitempty"`  // 背面纵向折叠宽度，毫米 / Back vertical folding width in millimeters
	VPeelFoldingWidth              *float32 `json:"vPeelFoldingWidth,omitempty"`              // 折叠宽度，毫米 / Folding width in millimeters
	VPeelFoldingCorrectWidth       *float32 `json:"vPeelFoldingCorrectWidth,omitempty"`       // 侧面折叠宽度的修正量，毫米，与 vPeelFoldingWidth 相加后使用 / Side folding width correction in millimeters, added to vPeelFoldingWidth before use
}

// UndressCommonPeelInfo 表示 ArchiveTarget.CommonPeelInfo，全部剥法共用的参数
// UndressCommonPeelInfo represents ArchiveTarget.CommonPeelInfo, the parameters shared by every peel mode
type UndressCommonPeelInfo struct {
	FixedPullLength *float32 `json:"fixedPullLength,omitempty"` // 固定拉动长度，毫米，游戏侧除以 1000 使用 / Fixed pull length in millimeters, divided by 1000 on the game side
}

// DecodeKCESUndressData 解码一个完整的 .undressdat 文档
// DecodeKCESUndressData decodes one complete .undressdat document
func DecodeKCESUndressData(data []byte) (*UndressArchiveTarget, error) {
	var value UndressArchiveTarget
	if err := decodeUnityJSONDocument(data, &value, "KCES .undressdat document"); err != nil {
		return nil, err
	}
	return &value, nil
}

// EncodeKCESUndressData 编码一个完整的 .undressdat 文档
// EncodeKCESUndressData encodes one complete .undressdat document
func EncodeKCESUndressData(value *UndressArchiveTarget) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES .undressdat document")
	}
	return encodeUnityJSONDocument(value, "KCES .undressdat document")
}
