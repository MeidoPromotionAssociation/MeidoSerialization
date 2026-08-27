package KCES

import "fmt"

// .undresspdat (UndressCore.PrecomputeTarget)
// 与同名 .undressdat 配对的预计算缓存，原生文件是 Unity JsonUtility 明文 JSON 文档
// WearSetuper 用 JsonUtility.FromJson<PrecomputeTarget> 读取它，两个文件缺少任一个都会让整套脱衣设置
// 直接中止，因此它们在编辑上是一个原子单位
//
// 文件内容全部由 .undressdat 派生：OneGroupLooker 把 (layer,label) 烘焙成序号表，meshReduction 记录逐组
// 的顶点退化目标，widthMeasurer 记录逐组的宽度测量结果。载入时 RestoreDictionary 按 (layer,label) 回指
// dataGroup，再由 InjectAll 把两张表注回运行时对象；查不到目标组时游戏只记录错误并丢弃该组的缓存
//
// .undresspdat (UndressCore.PrecomputeTarget)
// The precomputed cache paired with the .undressdat of the same name, whose native file is a Unity JsonUtility plain-JSON document
// WearSetuper reads it with JsonUtility.FromJson<PrecomputeTarget>, and a missing file on either side aborts
// the whole undress setup, which makes the pair one atomic unit for editing purposes
//
// Every value is derived from the .undressdat: OneGroupLooker bakes (layer,label) into an index table,
// meshReduction records the per-group vertex degeneracy targets, and widthMeasurer records the per-group
// width measurement results. While loading, RestoreDictionary resolves (layer,label) back to dataGroup and
// InjectAll injects both tables into the runtime objects; when a target group is missing the game only logs
// an error and drops that group's cache

const KCESUndressPartsDataExtension = ".undresspdat"

// UndressPrecomputeTarget 表示 UndressCore.PrecomputeTarget 的 Unity JSON 表示，成员顺序与 JsonUtility 的写出顺序一致
// UndressPrecomputeTarget represents the Unity JSON form of UndressCore.PrecomputeTarget with members ordered as JsonUtility writes them
type UndressPrecomputeTarget struct {
	EditVer                          *int32                     `json:"editVer,omitempty"`                          // 写出该文件的编辑器版本 / Version of the editor that wrote the file
	OneGroupLooker                   *UndressGroupLooker        `json:"OneGroupLooker,omitempty"`                   // 组序号表，键名的大写拼写沿用线格式 / Group index table, with the capitalized key spelled as stored
	WidthMeasurerValidPixelThreshold *float32                   `json:"WidthMeasurerValidPixelThreshold,omitempty"` // 宽度测量的有效像素阈值，键名的大写拼写沿用线格式 / Valid-pixel threshold of the width measurer, with the capitalized key spelled as stored
	MeshReduction                    *UndressMeshReductionTable `json:"meshReduction,omitempty"`                    // 逐组顶点退化表 / Per-group vertex degeneracy table
	WidthMeasurer                    *UndressWidthMeasurerTable `json:"widthMeasurer,omitempty"`                    // 逐组宽度测量表 / Per-group width measurement table
}

// UndressGroupLooker 表示 UndressCore.OneGroupLooker，只有烘焙出的 Targets 会写入文件
// UndressGroupLooker represents UndressCore.OneGroupLooker, of which only the baked Targets member is written to the file
type UndressGroupLooker struct {
	Targets *[]UndressGroupKey `json:"Targets,omitempty"` // 组键数组，下标即后两张表引用组时使用的 index / Group key array whose subscript is the index the two following tables use to refer to a group
}

// UndressGroupKey 表示 UndressCore.OneGroupKey，按剥法通道与组标签定位 .undressdat 中的一个组
// UndressGroupKey represents UndressCore.OneGroupKey, locating one group in the .undressdat by peel channel and group label
type UndressGroupKey struct {
	Lyr *int32  `json:"lyr,omitempty"` // 目标组的 layer，需与 dataGroup 中的 layer 对应 / Layer of the target group, which must match the layer in dataGroup
	Lbl *string `json:"lbl,omitempty"` // 目标组的 label，需与 dataGroup 中的 label 对应 / Label of the target group, which must match the label in dataGroup
}

// UndressMeshReductionTable 表示 PrecomputeTarget.TempMeshReduction
// UndressMeshReductionTable represents PrecomputeTarget.TempMeshReduction
type UndressMeshReductionTable struct {
	D *[]UndressMeshReductionEntry `json:"d,omitempty"` // 逐组顶点退化条目，键名的单字母缩写沿用线格式 / Per-group vertex degeneracy entries, with the single-letter key spelled as stored
}

// UndressMeshReductionEntry 表示 TempMeshReduction.Pac，一个组及其顶点退化数据
// UndressMeshReductionEntry represents TempMeshReduction.Pac, one group together with its vertex degeneracy data
type UndressMeshReductionEntry struct {
	V   *UndressGroupRef      `json:"v,omitempty"`   // 目标组引用 / Reference to the target group
	Dat *UndressMeshReduction `json:"dat,omitempty"` // 该组的顶点退化数据 / Vertex degeneracy data of the group
}

// UndressGroupRef 表示 UndressCore.OneGroupValue，按序号引用 OneGroupLooker.Targets 中的一个组
// UndressGroupRef represents UndressCore.OneGroupValue, referring by subscript to one group in OneGroupLooker.Targets
type UndressGroupRef struct {
	Index *int32 `json:"index,omitempty"` // OneGroupLooker.Targets 的下标 / Subscript into OneGroupLooker.Targets
}

// UndressMeshReduction 表示 UndressCore.MeshReductionDto，一个组在剥离前后各自的顶点退化目标
// UndressMeshReduction represents UndressCore.MeshReductionDto, the vertex degeneracy targets of one group before and after peeling
type UndressMeshReduction struct {
	P *UndressDegeneracyIndices `json:"p,omitempty"` // 前段退化目标，对应 OneGroup.DegeneracyTargetIndicesPre / Preceding degeneracy target matching OneGroup.DegeneracyTargetIndicesPre
	S *UndressDegeneracyIndices `json:"s,omitempty"` // 后段退化目标，对应 OneGroup.DegeneracyTargetIndicesSuf / Succeeding degeneracy target matching OneGroup.DegeneracyTargetIndicesSuf
}

// UndressDegeneracyIndices 表示 VPeelTransPatchCompressor.DegeneracyTargetIndices
// 运行时会把 idcs 列出的每个顶点都移动到 idx 顶点的位置上，从而缝合该处网格
// UndressDegeneracyIndices represents VPeelTransPatchCompressor.DegeneracyTargetIndices
// At runtime every vertex listed in idcs is moved onto the position of the idx vertex, welding the mesh there
type UndressDegeneracyIndices struct {
	Idcs *[]int32 `json:"idcs,omitempty"` // 被移动的顶点索引，空数组表示该项不生效 / Vertex indices to be moved, an empty array disables the item
	Idx  *int32   `json:"idx,omitempty"`  // 目标顶点索引，idcs 中的顶点都移动到它的位置 / Target vertex index that every vertex in idcs is moved onto
}

// UndressWidthMeasurerTable 表示 PrecomputeTarget.TempWidthMeasurer
// UndressWidthMeasurerTable represents PrecomputeTarget.TempWidthMeasurer
type UndressWidthMeasurerTable struct {
	D *[]UndressWidthMeasurerEntry `json:"d,omitempty"` // 逐组宽度测量条目，键名的单字母缩写沿用线格式 / Per-group width measurement entries, with the single-letter key spelled as stored
}

// UndressWidthMeasurerEntry 表示 TempWidthMeasurer.Pac，一个组及其宽度测量数据
// UndressWidthMeasurerEntry represents TempWidthMeasurer.Pac, one group together with its width measurement data
type UndressWidthMeasurerEntry struct {
	V   *UndressGroupRef      `json:"v,omitempty"`   // 目标组引用 / Reference to the target group
	Dat *UndressWidthMeasurer `json:"dat,omitempty"` // 该组的宽度测量数据 / Width measurement data of the group
}

// UndressWidthMeasurer 表示 UndressCore.WidthMeasurerDto
// UndressWidthMeasurer represents UndressCore.WidthMeasurerDto
type UndressWidthMeasurer struct {
	Hvp   *bool                     `json:"hvp,omitempty"`   // 该组是否有有效的宽度测量结果，对应 OneGroup.hasValidWidthMeasurer / Whether the group has a valid width measurement, matching OneGroup.hasValidWidthMeasurer
	Info  *UndressWidthMeasurerRail `json:"info,omitempty"`  // 正向剥离轨道的测量结果，对应 OneGroup.validPeelRailInfo / Measurement of the forward peel rail matching OneGroup.validPeelRailInfo
	InfoR *UndressWidthMeasurerRail `json:"infoR,omitempty"` // 反向剥离轨道的测量结果，对应 OneGroup.validPeelRailInfoRev / Measurement of the reverse peel rail matching OneGroup.validPeelRailInfoRev
}

// UndressWidthMeasurerRail 表示 WidthMeasurerDto.ReductionInfo，一条剥离轨道的测量结果
// UndressWidthMeasurerRail represents WidthMeasurerDto.ReductionInfo, the measurement of one peel rail
type UndressWidthMeasurerRail struct {
	VIdx *int32   `json:"vIdx,omitempty"` // 轨道上的顶点序号 / Vertex ordinal along the rail
	VPer *float32 `json:"vPer,omitempty"` // 该顶点处的归一化进度 / Normalized progress at that vertex
	MIdx *int32   `json:"mIdx,omitempty"` // 对应的网格顶点索引 / Corresponding mesh vertex index
}

// DecodeKCESUndressPartsData 解码一个完整的 .undresspdat 文档
// DecodeKCESUndressPartsData decodes one complete .undresspdat document
func DecodeKCESUndressPartsData(data []byte) (*UndressPrecomputeTarget, error) {
	var value UndressPrecomputeTarget
	if err := decodeUnityJSONDocument(data, &value, "KCES .undresspdat document"); err != nil {
		return nil, err
	}
	return &value, nil
}

// EncodeKCESUndressPartsData 编码一个完整的 .undresspdat 文档
// EncodeKCESUndressPartsData encodes one complete .undresspdat document
func EncodeKCESUndressPartsData(value *UndressPrecomputeTarget) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES .undresspdat document")
	}
	return encodeUnityJSONDocument(value, "KCES .undresspdat document")
}
