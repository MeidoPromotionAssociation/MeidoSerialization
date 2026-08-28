package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

// HairMakeSaveExtension 是 HairMake 继续编辑存档文件扩展名
// HairMakeSaveExtension is the continue-editing HairMake save file extension
const HairMakeSaveExtension = ".harimakesave"

// HairMakeBuildData 对应 HairMake.HairMakeBuildData 的字符串键 MessagePack 根对象 / HairMakeBuildData matches the string-keyed MessagePack root object HairMake.HairMakeBuildData
type HairMakeBuildData struct {
	MixHair          *MixHairBuildData              `codec:"hair" json:"hair"`              // 混合头发构建数据 / Mixed-hair build data
	SilhouettePreset *HairSilhouettePresetParameter `codec:"preset" json:"preset"`          // 轮廓预设参数 / Silhouette preset parameters
	SilhouetteData   *HairSilhouetteParameter       `codec:"silhouette" json:"silhouette"`  // 轮廓形变数据 / Silhouette deformation data
	BuildVersion     int32                          `codec:"build_ver" json:"buildVersion"` // HairMake 构建版本 / HairMake build version
	GameVersion      int32                          `codec:"game_ver" json:"gameVersion"`   // 游戏版本 / Game version
}

// HairMakeSave 是 .harimakesave 根对象的语义别名 / HairMakeSave is the semantic alias for a .harimakesave root object
type HairMakeSave = HairMakeBuildData

// MixHairBuildData 对应 HairMake.MixHairBuildData 的字符串键 MessagePack 对象 / MixHairBuildData matches the string-keyed MessagePack object HairMake.MixHairBuildData
type MixHairBuildData struct {
	Version     int32                `codec:"v" json:"version"`        // 混合头发数据版本 / Mixed-hair data version
	GUID        *string              `codec:"guid" json:"guid"`        // 可空混合头发 GUID / Nullable mixed-hair GUID
	Type        int32                `codec:"type" json:"type"`        // 混合头发类型 / Mixed-hair type
	MixName     *string              `codec:"name" json:"name"`        // 可空混合头发名称 / Nullable mixed-hair name
	Category    int32                `codec:"cat" json:"category"`     // HairMake 头发分类 / HairMake hair category
	UserName    *string              `codec:"creater" json:"creator"`  // 可空创建者名称，线格式保留游戏 creater 拼写 / Nullable creator name with the game's creater spelling retained on the wire
	Date        *string              `codec:"date" json:"date"`        // 可空创建日期文本 / Nullable creation date text
	Description *string              `codec:"desc" json:"description"` // 可空说明文本 / Nullable description text
	Bunches     []*BunchDeformedData `codec:"bunchs" json:"bunches"`   // 可空发束形变列表，线格式保留游戏 bunchs 拼写 / Nullable deformed-bunch list with the game's bunchs spelling retained on the wire
}

// BunchDeformedData 对应 HairMake.BunchDeformedData 的发束实例变换和形变数据 / BunchDeformedData matches the instance transform and deformation data in HairMake.BunchDeformedData
type BunchDeformedData struct {
	HairID     *string            `codec:"hid" json:"hairId"`      // 可空基础头发标识 / Nullable base-hair identifier
	BunchNo    int32              `codec:"bno" json:"bunchNo"`     // 发束编号 / Bunch number
	Position   Vector3            `codec:"op" json:"position"`     // 发束局部位置 / Bunch local position
	Rotation   Vector4            `codec:"or" json:"rotation"`     // 发束局部旋转四元数 / Bunch local rotation quaternion
	Scale      Vector3            `codec:"os" json:"scale"`        // 发束局部缩放 / Bunch local scale
	DeformList []*BunchDeformData `codec:"dlst" json:"deformList"` // 可空形变步骤列表 / Nullable deformation-step list
	IsBase     bool               `codec:"base" json:"isBase"`     // 是否为基础发束 / Whether this is a base bunch
}

// BunchDeformData 对应 BunchDeformedData.DeformData 的基础框、使用框和参数 / BunchDeformData matches the base box, active box, and parameters in BunchDeformedData.DeformData
type BunchDeformData struct {
	BaseBox *BaseDeformBoxData    `codec:"bbox" json:"baseBox"`  // 可空基础形变框 / Nullable base deformation box
	UseBox  *BaseDeformBoxData    `codec:"ubox" json:"useBox"`   // 可空实际形变框 / Nullable active deformation box
	Param   *BunchDeformParameter `codec:"prm" json:"parameter"` // 可空发束形变参数 / Nullable bunch deformation parameters
}

// BaseDeformBoxData 对应 HairMake.BaseDeformBoxData 的形变框几何和忽略列表 / BaseDeformBoxData matches the deformation-box geometry and ignore lists in HairMake.BaseDeformBoxData
type BaseDeformBoxData struct {
	Version         int32   `codec:"v" json:"version"`           // 形变框版本 / Deformation-box version
	Bunch           int32   `codec:"bno" json:"bunch"`           // 所属发束编号 / Owning bunch number
	No              int32   `codec:"no" json:"number"`           // 框编号 / Box number
	Start           Vector3 `codec:"sp" json:"start"`            // 框中心线起点 / Box center-line start
	End             Vector3 `codec:"ep" json:"end"`              // 框中心线终点 / Box center-line end
	DivisionCount   int32   `codec:"dn" json:"divisionCount"`    // 中心线分段数量 / Center-line division count
	UpDownVector    Vector3 `codec:"uv" json:"upDownVector"`     // 上下方向向量 / Up-down direction vector
	ForwardVector   Vector3 `codec:"fv" json:"forwardVector"`    // 前后方向向量 / Forward-backward direction vector
	SideVector      Vector3 `codec:"sv" json:"sideVector"`       // 左右方向向量 / Side direction vector
	IsMesh          bool    `codec:"bm" json:"isMesh"`           // 是否使用网格模式 / Whether mesh mode is used
	IgnoreTriangles []int32 `codec:"igt" json:"ignoreTriangles"` // 忽略的三角形索引 / Ignored triangle indices
	IgnoreGroups    []int32 `codec:"igg" json:"ignoreGroups"`    // 忽略的分组索引 / Ignored group indices
}

// BunchDeformParameter 对应 HairMake.BunchDeformParameter 的长度、宽度、移动、卷曲和卷绕参数 / BunchDeformParameter matches the length, width, movement, curl, and roll parameters in HairMake.BunchDeformParameter
type BunchDeformParameter struct {
	Version      int32     `codec:"v" json:"version"`           // 形变参数版本 / Deformation-parameter version
	DeformFlags  int32     `codec:"df" json:"deformFlags"`      // DeformBitFlg 标志 / DeformBitFlg flags
	LengthPowers []float32 `codec:"nl" json:"lengthPowers"`     // 各控制点长度形变量 / Length deformation at each control point
	WidthH       []Vector2 `codec:"nwh" json:"horizontalWidth"` // 各控制点水平方向宽度形变量 / Horizontal width deformation at each control point
	WidthV       []Vector2 `codec:"nwv" json:"verticalWidth"`   // 各控制点垂直方向宽度形变量 / Vertical width deformation at each control point
	MovePowers   []Vector3 `codec:"nmp" json:"movePowers"`      // 各控制点移动形变量 / Movement deformation at each control point
	CurlPosition float32   `codec:"cp" json:"curlPosition"`     // 卷曲位置 / Curl position
	CurlRadius   float32   `codec:"cr" json:"curlRadius"`       // 卷曲角度 / Curl angle
	CurlPower    float32   `codec:"cw" json:"curlPower"`        // 卷曲强度 / Curl strength
	RollParams   []float32 `codec:"rpl" json:"rollParameters"`  // 卷绕参数 / Roll parameters
}

// HairSilhouetteParameter 对应 HairMake.HairSilhouetteParameter 的三轴分割点和形变值 / HairSilhouetteParameter matches the three-axis division points and deformation values in HairMake.HairSilhouetteParameter
type HairSilhouetteParameter struct {
	XDivisionPoints []int32   `codec:"xdiv" json:"xDivisionPoints"` // X 轴分割点 / X-axis division points
	YDivisionPoints []int32   `codec:"ydiv" json:"yDivisionPoints"` // Y 轴分割点 / Y-axis division points
	ZDivisionPoints []int32   `codec:"zdiv" json:"zDivisionPoints"` // Z 轴分割点 / Z-axis division points
	DeformValues    []Vector3 `codec:"values" json:"deformValues"`  // 各网格点形变值 / Deformation values at each lattice point
}

// HairSilhouettePresetParameter 对应 HairMake.HairSilhouettePresetParameter 的预设编号和混合参数 / HairSilhouettePresetParameter matches the preset number and blend parameters in HairMake.HairSilhouettePresetParameter
type HairSilhouettePresetParameter struct {
	Number  int32   `codec:"no" json:"number"`     // 轮廓预设编号 / Silhouette preset number
	Version int32   `codec:"ver" json:"version"`   // 轮廓预设版本 / Silhouette preset version
	Deform  float32 `codec:"deform" json:"deform"` // 预设形变量 / Preset deformation amount
	Blend   float32 `codec:"blend" json:"blend"`   // 预设混合量 / Preset blend amount
}

// DecodeHairMakeSave 解码未压缩的字符串键 .harimakesave MessagePack 数据
// DecodeHairMakeSave decodes uncompressed string-keyed .harimakesave MessagePack data
func DecodeHairMakeSave(data []byte) (*HairMakeSave, error) {
	var value *HairMakeSave
	if err := msgpack.DecodeMsgpack(data, &value); err != nil {
		return nil, fmt.Errorf("decode HairMakeSave msgpack: %w", err)
	}
	return value, nil
}

// EncodeHairMakeSave 编码未压缩的字符串键 .harimakesave MessagePack 数据
// EncodeHairMakeSave encodes uncompressed string-keyed .harimakesave MessagePack data
func EncodeHairMakeSave(value *HairMakeSave) ([]byte, error) {
	if value == nil {
		return msgpack.EncodeMsgpack(nil)
	}
	encoded, err := msgpack.EncodeMsgpack(value)
	if err != nil {
		return nil, fmt.Errorf("encode HairMakeSave msgpack: %w", err)
	}
	return encoded, nil
}
