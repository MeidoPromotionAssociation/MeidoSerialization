package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/ugorji/go/codec"
)

// Vector2 表示 UnityEngine.Vector2 的 MessagePack 数组布局 / Vector2 represents UnityEngine.Vector2 in MessagePack array layout
type Vector2 struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	X       float32  `json:"x"`         // X 轴分量 / X-axis component
	Y       float32  `json:"y"`         // Y 轴分量 / Y-axis component
}

// Vector2Int 表示 UnityEngine.Vector2Int 的 MessagePack 数组布局 / Vector2Int represents UnityEngine.Vector2Int in MessagePack array layout
type Vector2Int struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	X       int      `json:"x"`         // X 轴整数分量 / Integer X-axis component
	Y       int      `json:"y"`         // Y 轴整数分量 / Integer Y-axis component
}

// Vector3 表示 UnityEngine.Vector3 的 MessagePack 数组布局 / Vector3 represents UnityEngine.Vector3 in MessagePack array layout
type Vector3 struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	X       float32  `json:"x"`         // X 轴分量 / X-axis component
	Y       float32  `json:"y"`         // Y 轴分量 / Y-axis component
	Z       float32  `json:"z"`         // Z 轴分量 / Z-axis component
}

// Vector4 表示 UnityEngine.Vector4 或 Quaternion 的 MessagePack 数组布局 / Vector4 represents UnityEngine.Vector4 or Quaternion in MessagePack array layout
type Vector4 struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	X       float32  `json:"x"`         // X 轴分量或四元数 X / X-axis component or quaternion X
	Y       float32  `json:"y"`         // Y 轴分量或四元数 Y / Y-axis component or quaternion Y
	Z       float32  `json:"z"`         // Z 轴分量或四元数 Z / Z-axis component or quaternion Z
	W       float32  `json:"w"`         // W 分量或四元数 W / W component or quaternion W
}

// PartsColor 对应游戏源码 MaidInfinityColor.PartsColor / PartsColor corresponds to the game's MaidInfinityColor.PartsColor
// 游戏在 OnBeforeSerialize 中把 m_grada 写入 m_gradaBytes / The game writes m_grada into m_gradaBytes during OnBeforeSerialize
type PartsColor struct {
	_struct          struct{}   `codec:",toarray"`           // 强制按数组编码 / Forces array encoding
	MainHue          int        `json:"m_nMainHue"`          // 主色相，对应 m_nMainHue / Main hue, matching m_nMainHue
	MainChroma       int        `json:"m_nMainChroma"`       // 主色彩度，对应 m_nMainChroma / Main chroma, matching m_nMainChroma
	MainBrightness   int        `json:"m_nMainBrightness"`   // 主色亮度，对应 m_nMainBrightness / Main brightness, matching m_nMainBrightness
	MainContrast     int        `json:"m_nMainContrast"`     // 主色对比度，对应 m_nMainContrast / Main contrast, matching m_nMainContrast
	ShadowRate       int        `json:"m_nShadowRate"`       // 阴影混合比例，对应 m_nShadowRate / Shadow blend rate, matching m_nShadowRate
	ShadowHue        int        `json:"m_nShadowHue"`        // 阴影色相，对应 m_nShadowHue / Shadow hue, matching m_nShadowHue
	ShadowChroma     int        `json:"m_nShadowChroma"`     // 阴影彩度，对应 m_nShadowChroma / Shadow chroma, matching m_nShadowChroma
	ShadowBrightness int        `json:"m_nShadowBrightness"` // 阴影亮度，对应 m_nShadowBrightness / Shadow brightness, matching m_nShadowBrightness
	ShadowContrast   int        `json:"m_nShadowContrast"`   // 阴影对比度，对应 m_nShadowContrast / Shadow contrast, matching m_nShadowContrast
	GradaBytes       GradaBytes `json:"m_gradaBytes"`        // 梯度色序列化字节；旧资源可能保存 bool/nil 占位 / Serialized gradient-color bytes; older assets may store bool/nil placeholders
}

// GradaBytes 保留 PartsColor.m_gradaBytes 的宽松历史形态 / GradaBytes preserves the loose historical shape of PartsColor.m_gradaBytes
// 当前游戏写 byte[]，旧资源可能写 false/null / Current game code writes byte[], while older assets may contain false/null
type GradaBytes struct {
	Value interface{} `json:"-"` // 原始 MessagePack 值，用于无损往返 / Raw MessagePack value for lossless round-trip
}

func (g GradaBytes) CodecEncodeSelf(e *codec.Encoder) {
	e.MustEncode(g.Value)
}

func (g *GradaBytes) CodecDecodeSelf(d *codec.Decoder) {
	var v interface{}
	d.MustDecode(&v)
	if b, ok := v.([]byte); ok {
		copied := append([]byte(nil), b...)
		g.Value = copied
		return
	}
	if s, ok := v.(string); ok {
		g.Value = []byte(s)
		return
	}
	g.Value = v
}

func (g GradaBytes) MarshalJSON() ([]byte, error) {
	switch v := g.Value.(type) {
	case nil:
		return []byte("null"), nil
	case []byte:
		return json.Marshal(v)
	case bool:
		return json.Marshal(v)
	default:
		return json.Marshal(v)
	}
}

func (g *GradaBytes) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		g.Value = nil
		return nil
	}

	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		g.Value = b
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return fmt.Errorf("m_gradaBytes string must be base64: %w", err)
		}
		g.Value = decoded
		return nil
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	g.Value = raw
	return nil
}

// PreMulTexDatas 对应 Parts.Menu.PreMulTexDatas 的贴图预合成记录 / PreMulTexDatas maps Parts.Menu.PreMulTexDatas texture pre-composition records
type PreMulTexDatas struct {
	_struct               struct{}       `codec:",toarray"`               // 强制按数组编码 / Forces array encoding
	Version               int            `json:"version"`                 // 版本号，游戏 FixVersion 为 1001 / Version value; the game's FixVersion is 1001
	SlotID                string         `json:"slotId"`                  // 目标槽位 ID / Target slot ID
	SaveTag               string         `json:"saveTag"`                 // 保存用图层标签 / Saved texture layer tag
	MatNo                 int            `json:"f_nMatNo"`                // 目标材质编号 / Target material index
	PropName              string         `json:"f_strPropName"`           // 目标材质属性名 / Target material property name
	LayerNo               int            `json:"f_nLayerNo"`              // 贴图层编号 / Texture layer number
	FileName              string         `json:"f_strFileName"`           // 合成源贴图文件名 / Source texture file name for composition
	BlendMode             string         `json:"f_eBlendMode"`            // 合成模式字符串 / Blend-mode string
	MaskParam             *MaskParam     `json:"maskParam"`               // 蒙版参数 / Mask parameters
	InfColParam           *InfColorParam `json:"infColParam"`             // 无限色参数 / Infinity-color parameters
	TexGroup              bool           `json:"f_bTexGroup"`             // 是否属于贴图组 / Whether this layer belongs to a texture group
	LayNoInGroup          int            `json:"f_nLayNoInGroup"`         // 组内层编号 / Layer index inside the group
	Alpha                 float32        `json:"f_fAlpha"`                // 合成透明度 / Composition alpha
	TargetBodyTexSize     int            `json:"f_nTargetBodyTexSize"`    // 目标身体贴图尺寸 / Target body texture size
	PosDefHokuroTatooSlot string         `json:"posDefHokuroTatooSlotId"` // 默认痣/纹身位置槽位 / Default mole/tattoo position slot
	PreMaskData           []MaskData     `json:"preMaskData"`             // 预计算蒙版数据 / Precomputed mask data
	PreTransTexData       []TransTexData `json:"preTransTexData"`         // 预计算贴图变换数据 / Precomputed texture transform data
	PreInfColData         *InfColData    `json:"preInfColData"`           // 预计算无限色数据 / Precomputed infinity-color data
	PreTexCompoTypeStr    string         `json:"preTexCompoTypeStr"`      // 预合成系统材质模式字符串 / Pre-composition system material mode string
}

// TransTexData 对应 TexLay.TransTexData，描述合成贴图的平移/缩放/旋转 / TransTexData maps TexLay.TransTexData and describes texture translation/scale/rotation
type TransTexData struct {
	_struct      struct{}      `codec:",toarray"`    // 强制按数组编码 / Forces array encoding
	Pos          Vector2       `json:"pos"`          // 贴图中心位置，通常为目标 RT 归一化坐标 / Texture center position, usually normalized in the target render texture
	Scale        Vector2       `json:"scale"`        // 贴图缩放，负值表示翻转 / Texture scale; negative values indicate flipping
	RotDeg       float32       `json:"rotDeg"`       // 以角度表示的旋转量 / Rotation in degrees
	AreaUV       Vector4       `json:"areaUV"`       // 使用的源贴图 UV 区域 / Source texture UV area
	SrcTexPixcel Vector2Int    `json:"srcTexPixcel"` // 源贴图像素尺寸，保留游戏字段原拼写 / Source texture pixel size, preserving the game's spelling
	DefTrans     *TransTexData `json:"defTrans"`     // 默认变换，用于 ResetTrans 恢复 / Default transform used by ResetTrans
}

// InfColorParam 对应 TexLay.InfColorParam，描述无限色合成输入 / InfColorParam maps TexLay.InfColorParam and describes infinity-color composition input
type InfColorParam struct {
	_struct                  struct{}     `codec:",toarray"`                // 强制按数组编码 / Forces array encoding
	Tag                      string       `json:"tag"`                      // 无限色目标标签 / Infinity-color target tag
	InfColType               int          `json:"infColType"`               // 颜色类型枚举：NONE/INF_COLOR/PART_COLOR/GRADA_COLOR / Color type enum
	InfColorID               int          `json:"infColorId"`               // MaidInfinityColor.PARTS_COLOR 枚举值 / MaidInfinityColor.PARTS_COLOR enum value
	IsIndependenceMultiColor bool         `json:"isIndependenceMultiColor"` // 是否使用独立多色数据 / Whether independent multi-color data is used
	PC                       PartsColor   `json:"pc"`                       // 单色无限色参数 / Single infinity-color parameters
	IDTexName                []string     `json:"idTexName"`                // ID 贴图文件名列表 / ID texture file-name list
	PartCols                 []PartColDef `json:"partCols"`                 // 分部颜色定义列表 / Part-color definition list
	GradeCols                *GradaColDef `json:"gradeCols"`                // 渐变色定义，字段名沿用游戏 gradeCols 拼写 / Gradient color definition, keeping the game's spelling
	GradaLines               []Vector4    `json:"gradaLines"`               // 渐变线段数组 / Gradient line array
	IDTexIsRGB               bool         `json:"idTexIsRGB"`               // 是否把 ID 贴图按 RGB 通道分区解释 / Whether ID textures are interpreted by RGB channels
	GradaIsMugen             bool         `json:"gradaIsMugen"`             // 渐变是否使用无限色表 / Whether the gradient uses the infinity-color table
}

// MaskData 对应 TexLay.MaskData，记录单个蒙版开关 / MaskData maps TexLay.MaskData and stores one mask toggle
type MaskData struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Name    string   `json:"name"`      // 蒙版名称 / Mask name
	Mask    bool     `json:"mask"`      // 是否启用该蒙版 / Whether this mask is enabled
}

// MaskParam 对应 TexLay.MaskParam，描述蒙版贴图和区域 / MaskParam maps TexLay.MaskParam and describes mask texture and ranges
type MaskParam struct {
	_struct           struct{}   `codec:",toarray"`         // 强制按数组编码 / Forces array encoding
	MaskData          []MaskData `json:"maskData"`          // 每个蒙版槽位的启用状态 / Enabled state for each mask slot
	MaskTexName       string     `json:"maskTexName"`       // 蒙版贴图文件名 / Mask texture file name
	MaskRanges        []Vector4  `json:"maskRanges"`        // 蒙版 UV/范围数组 / Mask UV/range array
	LinkMaskName      string     `json:"linkMaskName"`      // 关联蒙版名称 / Linked mask name
	LinkMaskNo        int        `json:"linkMaskNo"`        // 关联蒙版编号 / Linked mask index
	ShareRtTargetPart string     `json:"shareRtTargetPart"` // 共享 RenderTexture 的目标部件名 / Target part name for shared RenderTexture
}

// PartColDef 对应 InfinityColorTexMgr2.PartColDef，描述 ID 贴图某个部位颜色 / PartColDef maps InfinityColorTexMgr2.PartColDef and describes one ID-texture part color
type PartColDef struct {
	_struct      struct{}   `codec:",toarray"`    // 强制按数组编码 / Forces array encoding
	PartName     string     `json:"part_name"`    // 部位名称，字段名对应 part_name / Part name, matching part_name
	MultiCol     PartsColor `json:"multi_col"`    // 部位颜色参数 / Part color parameters
	PatternScale Vector2    `json:"patternScale"` // 纹样缩放 / Pattern texture scale
	PatternRot   float32    `json:"patternRot"`   // 纹样旋转角度 / Pattern texture rotation in degrees
}

// GradaColDef 对应 InfinityColorTexMgr2.GradaColDef，描述渐变色定义 / GradaColDef maps InfinityColorTexMgr2.GradaColDef and describes a gradient color definition
type GradaColDef struct {
	_struct         struct{}   `codec:",toarray"`       // 强制按数组编码 / Forces array encoding
	NotUse          string     `json:"notUse"`          // 游戏保留字段 notUse / Game-reserved notUse field
	GradaNum        int        `json:"gradaNum"`        // 渐变点数量 / Number of gradient points
	GradaRates      []float32  `json:"gradaRates"`      // 渐变点位置比例 / Gradient point rates
	GradaRateRanges []Vector4  `json:"gradaRateRanges"` // 渐变点影响范围 / Gradient point influence ranges
	MultiCol        PartsColor `json:"multi_col"`       // 渐变用多色数据 / Multi-color data used by the gradient
}

// InfColData 对应 InfinityColorTexMgr2.InfColData，保存应用后的无限色数据 / InfColData maps InfinityColorTexMgr2.InfColData and stores applied infinity-color data
type InfColData struct {
	_struct                  struct{}     `codec:",toarray"`                // 强制按数组编码 / Forces array encoding
	IsIndependenceMultiColor bool         `json:"isIndependenceMultiColor"` // 是否使用独立多色表 / Whether independent multi-color table data is used
	InfColType               int          `json:"infColType"`               // 无限色类型枚举 / Infinity-color type enum
	PartsColorType           int          `json:"partsColorType"`           // MaidInfinityColor.PARTS_COLOR 枚举值 / MaidInfinityColor.PARTS_COLOR enum value
	ColData                  PartsColor   `json:"colData"`                  // 单色无限色数据 / Single infinity-color data
	PartColDefs              []PartColDef `json:"partColDefs"`              // 分部颜色数据 / Part-color data
	GradaColDef              *GradaColDef `json:"gradaColDef"`              // 渐变色数据 / Gradient color data
	GradaIsMugen             bool         `json:"gradaIsMugen"`             // 渐变是否按无限色处理 / Whether the gradient is treated as infinity color
}

// Colvari 对应 Parts.Menu.Colvari，保存一个颜色变体菜单的入口信息 / Colvari maps Parts.Menu.Colvari and stores color-variant menu entry data
type Colvari struct {
	_struct      struct{}      `codec:",toarray"`    // 强制按数组编码 / Forces array encoding
	Version      int           `json:"version"`      // 版本号，游戏 FixVersion 为 1000 / Version value; the game's FixVersion is 1000
	IconColor    PartsColor    `json:"iconColor"`    // 颜色变体图标色 / Color-variant icon color
	IconFileName string        `json:"iconFileName"` // 颜色变体图标文件名 / Color-variant icon file name
	ReqDefine    string        `json:"reqDefine"`    // 启用该变体所需 DEFINE / DEFINE requirement for enabling this variant
	ColvariDatas []ColvariData `json:"colvariDatas"` // 颜色变体数据列表 / Color-variant data list
}

// ColvariData 对应 Parts.Menu.Colvari.ColvariData，描述一条颜色变体规则 / ColvariData maps Parts.Menu.Colvari.ColvariData and describes one color-variant rule
type ColvariData struct {
	_struct                 struct{}     `codec:",toarray"`               // 强制按数组编码 / Forces array encoding
	Version                 int          `json:"version"`                 // 版本号，游戏 FixVersion 为 1000 / Version value; the game's FixVersion is 1000
	MPN                     string       `json:"mpn"`                     // 目标 MPN 名称，多个值用竖线分隔 / Target MPN names, pipe-separated when multiple
	LayerName               string       `json:"layerName"`               // 保存到 PropBase.savedTexDatas 的图层名 / Layer name saved into PropBase.savedTexDatas
	ColorType               int          `json:"colorType"`               // 主颜色类型枚举 / Primary color type enum
	MaskData                []MaskData   `json:"maskData"`                // 该变体的蒙版状态 / Mask state for this variant
	Alpha                   float32      `json:"alpha"`                   // 乘算透明度 / Multiplicative alpha
	ColData                 PartsColor   `json:"colData"`                 // 单色颜色数据 / Single color data
	PartColDefs             []PartColDef `json:"partColDefs"`             // 分部颜色定义 / Part-color definitions
	GradaColDef             *GradaColDef `json:"gradaColDef"`             // 渐变色定义 / Gradient color definition
	MamaFileName            string       `json:"mamaFileName"`            // 关联的 MAMA 文件名 / Related MAMA file name
	ColorTypeSub            int          `json:"colorTypeSub"`            // 渐变/复合颜色的子类型 / Subtype for gradient or compound color
	UseType                 uint8        `json:"useType"`                 // 使用标志，bit0=alpha，bit1=color / Use flags, bit0=alpha and bit1=color
	SaveInfColDataLinkLayer string       `json:"saveInfColDataLinkLayer"` // 共享无限色数据的源图层名 / Source layer name for shared infinity-color data
	ViewName                string       `json:"viewName"`                // 编辑界面显示名 / Display name in the edit UI
}

// BlendData 对应游戏 BlendData，保存模型 morph 顶点差分 / BlendData maps the game's BlendData and stores model morph vertex deltas
type BlendData struct {
	_struct struct{}  `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Name    string    `json:"name"`      // morph 名称 / Morph name
	VIndex  []int     `json:"v_index"`   // 受影响顶点索引 / Affected vertex indices
	Vert    []Vector3 `json:"vert"`      // 顶点位置差分 / Vertex position deltas
	Norm    []Vector3 `json:"norm"`      // 法线差分 / Normal deltas
	Tan     []Vector4 `json:"tan"`       // 切线差分 / Tangent deltas
}

// SkinThickness 对应 Parts.Model.SkinThickness，保存皮肤厚度修正 / SkinThickness maps Parts.Model.SkinThickness and stores skin-thickness correction data
type SkinThickness struct {
	_struct struct{}                  `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Use     bool                      `json:"use"`       // 是否启用皮肤厚度修正 / Whether skin-thickness correction is enabled
	Groups  map[string]ThicknessGroup `json:"groups"`    // 按组名索引的厚度修正组 / Thickness correction groups keyed by group name
}

// ThicknessGroup 对应 Parts.Model.SkinThickness.Group / ThicknessGroup maps Parts.Model.SkinThickness.Group
type ThicknessGroup struct {
	_struct         struct{}         `codec:",toarray"`      // 强制按数组编码 / Forces array encoding
	GroupName       string           `json:"groupName"`      // 厚度组名称 / Thickness group name
	StartBoneName   string           `json:"startBoneName"`  // 线段起始骨骼名 / Segment start bone name
	EndBoneName     string           `json:"endBoneName"`    // 线段结束骨骼名 / Segment end bone name
	StepAngleDegree int              `json:"stepAngleDgree"` // 角度采样步进，字段名保留游戏拼写 stepAngleDgree / Angle sampling step, preserving the game's stepAngleDgree spelling
	Points          []ThicknessPoint `json:"points"`         // 厚度采样点列表 / Thickness sample points
}

// ThicknessPoint 对应 Parts.Model.SkinThickness.Group.Point / ThicknessPoint maps Parts.Model.SkinThickness.Group.Point
type ThicknessPoint struct {
	_struct                struct{}               `codec:",toarray"`              // 强制按数组编码 / Forces array encoding
	TargetBoneName         string                 `json:"targetBoneName"`         // 采样点目标骨骼名 / Target bone name for this sample point
	RatioSegmentStartToEnd float32                `json:"ratioSegmentStartToEnd"` // 点位于起止骨骼线段上的比例 / Ratio along the start-to-end bone segment
	DistanceParAngle       []ThicknessDefPerAngle `json:"distanceParAngle"`       // 按角度记录的默认距离，字段名保留游戏拼写 Par / Default distances by angle, preserving the game's Par spelling
}

// ThicknessDefPerAngle 对应 Parts.Model.SkinThickness.Group.Point.DefPerAngle / ThicknessDefPerAngle maps Parts.Model.SkinThickness.Group.Point.DefPerAngle
type ThicknessDefPerAngle struct {
	_struct         struct{} `codec:",toarray"`       // 强制按数组编码 / Forces array encoding
	AngleDegree     int      `json:"angleDgree"`      // 角度，字段名保留游戏拼写 angleDgree / Angle in degrees, preserving the game's angleDgree spelling
	VertexIndex     int      `json:"vidx"`            // 顶点索引 vidx / Vertex index, vidx
	DefaultDistance float32  `json:"defaultDistance"` // 默认厚度距离 / Default thickness distance
}

// TupleStringInt 对应 C# Tuple<string,int> 的 MessagePack 数组布局 / TupleStringInt maps the MessagePack array layout of C# Tuple<string,int>
type TupleStringInt struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Item1   string   `json:"item1"`     // 第一个元组值 / First tuple value
	Item2   int      `json:"item2"`     // 第二个元组值 / Second tuple value
}
