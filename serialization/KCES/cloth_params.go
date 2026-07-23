package KCES

import (
	"encoding/json"
	"fmt"
	"strings"
)

// .dsbconf 与 .dslconf 共用的 MagicaCloth 参数模型
// MagicaCloth parameter model shared by .dsbconf and .dslconf

// DecodeClothParamsFile 解码使用 ClothParams 线格式的 .dsbconf 或 .dslconf 文件
// DecodeClothParamsFile decodes a .dsbconf or .dslconf file that uses the ClothParams wire model
func DecodeClothParamsFile(data []byte, extension string) (*ClothParams, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	if ext != KCESDSBConfExtension && ext != KCESDSLConfExtension {
		return nil, fmt.Errorf("unsupported ClothParams extension %q: expected %s or %s", extension, KCESDSBConfExtension, KCESDSLConfExtension)
	}
	env, err := DecodeKCESPayload(data, ext)
	if err != nil {
		return nil, err
	}
	if env.ClothParams == nil {
		return nil, fmt.Errorf("payload is not ClothParams")
	}
	return env.ClothParams, nil
}

// EncodeClothParamsFile 编码使用 ClothParams 线格式的 .dsbconf 或 .dslconf 文件，空扩展名默认使用 .dsbconf
// EncodeClothParamsFile encodes a .dsbconf or .dslconf file using the ClothParams wire model, with an empty extension defaulting to .dsbconf
func EncodeClothParamsFile(params *ClothParams, extension string) ([]byte, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	if strings.TrimSpace(extension) == "" {
		ext = KCESDSBConfExtension
	} else if ext != KCESDSBConfExtension && ext != KCESDSLConfExtension {
		return nil, fmt.Errorf("unsupported ClothParams extension %q: expected %s or %s", extension, KCESDSBConfExtension, KCESDSLConfExtension)
	}
	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ext,
		LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
		Kind:           PayloadKindClothParams,
		ClothParams:    params,
	}
	return EncodeKCESPayload(env)
}

// BezierParam 对应 MagicaCloth.BezierParam
// BezierParam corresponds to MagicaCloth.BezierParam
type BezierParam struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	StartValue             float32     `json:"startValue"`    // 起始值 / Start value
	EndValue               float32     `json:"endValue"`      // 结束值 / End value
	UseEndValue            bool        `json:"useEndValue"`   // 是否使用结束值 / Whether the end value is used
	CurveValue             float32     `json:"curveValue"`    // 曲线值 / Curve value
	UseCurveValue          bool        `json:"useCurveValue"` // 是否使用曲线值 / Whether the curve value is used
}

// newBezierParam 按游戏字段顺序创建一个 BezierParam 值
// newBezierParam creates a BezierParam value in the game's field order
func newBezierParam(start, end float32, useEnd bool, curve float32, useCurve bool) BezierParam {
	return BezierParam{
		StartValue:    start,
		EndValue:      end,
		UseEndValue:   useEnd,
		CurveValue:    curve,
		UseCurveValue: useCurve,
	}
}

// ClothTeleportMode 表示 MagicaCloth 的传送处理模式
// ClothTeleportMode represents a MagicaCloth teleport handling mode
type ClothTeleportMode int32

const (
	ClothTeleportModeReset ClothTeleportMode = iota
	ClothTeleportModeKeep
)

// ClothAdjustMode 表示 MagicaCloth 的位置调整模式
// ClothAdjustMode represents a MagicaCloth position adjustment mode
type ClothAdjustMode int32

const (
	ClothAdjustModeFixed ClothAdjustMode = iota
	ClothAdjustModeXYMove
	ClothAdjustModeXZMove
	ClothAdjustModeYZMove
)

// ClothPenetrationMode 表示 MagicaCloth 的穿透修正模式
// ClothPenetrationMode represents a MagicaCloth penetration-correction mode
type ClothPenetrationMode int32

const (
	ClothPenetrationModeSurfacePenetration ClothPenetrationMode = iota
	ClothPenetrationModeColliderPenetration
)

// ClothPenetrationAxis 表示 MagicaCloth 的穿透修正轴
// ClothPenetrationAxis represents a MagicaCloth penetration-correction axis
type ClothPenetrationAxis int32

const (
	ClothPenetrationAxisX ClothPenetrationAxis = iota
	ClothPenetrationAxisY
	ClothPenetrationAxisZ
	ClothPenetrationAxisInverseX
	ClothPenetrationAxisInverseY
	ClothPenetrationAxisInverseZ
)

// ClothParams 对应 MagicaCloth.ClothParams
// MessagePack-CSharp 以 Key(0) 至 Key(82) 的 indexed array 写入，Key(4)、Key(5) 和 Key(56) 是当前游戏类型中的空洞并需要保留
// ClothParams corresponds to MagicaCloth.ClothParams
// MessagePack-CSharp writes Key(0) through Key(82) as an indexed array with sparse holes at Key(4), Key(5), and Key(56) that must be preserved
type ClothParams struct {
	_struct                          struct{}             `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata           `codec:"-"`          // 索引对象的线格式元数据 / Indexed-object wire metadata
	Radius                           BezierParam          `json:"radius"`                           // 粒子半径曲线参数 / Particle radius curve parameter
	Mass                             BezierParam          `json:"mass"`                             // 质量曲线参数 / Mass curve parameter
	UseGravity                       bool                 `json:"useGravity"`                       // 是否使用重力 / Whether gravity is enabled
	Gravity                          BezierParam          `json:"gravity"`                          // 重力强度曲线参数 / Gravity strength curve parameter
	Reserved04                       RawMessagePackSlot   `json:"reserved04,omitempty"`             // C# 无 Key(4)；原始稀疏槽位 / C# has no Key(4); raw sparse slot
	Reserved05                       RawMessagePackSlot   `json:"reserved05,omitempty"`             // C# 无 Key(5)；原始稀疏槽位 / C# has no Key(5); raw sparse slot
	UseDrag                          bool                 `json:"useDrag"`                          // 是否使用阻力 / Whether drag is enabled
	Drag                             BezierParam          `json:"drag"`                             // 阻力曲线参数 / Drag curve parameter
	UseMaxVelocity                   bool                 `json:"useMaxVelocity"`                   // 是否限制最大速度 / Whether maximum velocity is limited
	MaxVelocity                      BezierParam          `json:"maxVelocity"`                      // 最大速度曲线参数 / Maximum velocity curve parameter
	WorldMoveInfluence               BezierParam          `json:"worldMoveInfluence"`               // 世界移动影响曲线参数 / World movement influence curve parameter
	WorldRotationInfluence           BezierParam          `json:"worldRotationInfluence"`           // 世界旋转影响曲线参数 / World rotation influence curve parameter
	MassInfluence                    float32              `json:"massInfluence"`                    // 质量影响系数 / Mass influence factor
	WindInfluence                    float32              `json:"windInfluence"`                    // 风力影响系数 / Wind influence factor
	WindRandomScale                  float32              `json:"windRandomScale"`                  // 风力随机缩放 / Wind random scale
	UseDistanceDisable               bool                 `json:"useDistanceDisable"`               // 是否按距离禁用布料 / Whether cloth is disabled by distance
	DisableDistance                  float32              `json:"disableDistance"`                  // 禁用距离 / Disable distance
	DisableFadeDistance              float32              `json:"disableFadeDistance"`              // 禁用淡出距离 / Disable fade distance
	UseResetTeleport                 bool                 `json:"useResetTeleport"`                 // 是否在传送后重置 / Whether teleport reset is enabled
	TeleportDistance                 float32              `json:"teleportDistance"`                 // 触发传送的距离阈值 / Distance threshold for teleport handling
	TeleportRotation                 float32              `json:"teleportRotation"`                 // 触发传送的旋转阈值 / Rotation threshold for teleport handling
	UseClampDistanceRatio            bool                 `json:"useClampDistanceRatio"`            // 是否启用距离比例约束 / Whether distance-ratio clamp is enabled
	ClampDistanceMinRatio            float32              `json:"clampDistanceMinRatio"`            // 距离比例最小值 / Minimum distance ratio
	ClampDistanceMaxRatio            float32              `json:"clampDistanceMaxRatio"`            // 距离比例最大值 / Maximum distance ratio
	ClampDistanceVelocityInfluence   float32              `json:"clampDistanceVelocityInfluence"`   // 距离约束速度影响系数 / Velocity influence for distance clamp
	UseClampPositionLength           bool                 `json:"useClampPositionLength"`           // 是否启用位置长度约束 / Whether position-length clamp is enabled
	ClampPositionLength              BezierParam          `json:"clampPositionLength"`              // 位置长度约束曲线参数 / Position-length clamp curve parameter
	ClampPositionRatioX              float32              `json:"clampPositionRatioX"`              // X 轴位置约束比例 / X-axis position clamp ratio
	ClampPositionRatioY              float32              `json:"clampPositionRatioY"`              // Y 轴位置约束比例 / Y-axis position clamp ratio
	ClampPositionRatioZ              float32              `json:"clampPositionRatioZ"`              // Z 轴位置约束比例 / Z-axis position clamp ratio
	ClampPositionVelocityInfluence   float32              `json:"clampPositionVelocityInfluence"`   // 位置约束速度影响系数 / Velocity influence for position clamp
	UseClampRotation                 bool                 `json:"useClampRotation"`                 // 是否启用旋转约束 / Whether rotation clamp is enabled
	ClampRotationAngle               BezierParam          `json:"clampRotationAngle"`               // 旋转角度约束曲线参数 / Rotation angle clamp curve parameter
	ClampRotationVelocityInfluence   float32              `json:"clampRotationVelocityInfluence"`   // 旋转约束速度影响系数 / Velocity influence for rotation clamp
	RestoreDistanceVelocityInfluence float32              `json:"restoreDistanceVelocityInfluence"` // 距离恢复速度影响系数 / Velocity influence for distance restoration
	StructDistanceStiffness          BezierParam          `json:"structDistanceStiffness"`          // 结构距离刚性曲线参数 / Structural distance stiffness curve parameter
	UseBendDistance                  bool                 `json:"useBendDistance"`                  // 是否启用弯曲距离约束 / Whether bend-distance constraint is enabled
	BendDistanceMaxCount             int32                `json:"bendDistanceMaxCount"`             // 弯曲距离最大计算数量 / Maximum bend-distance count
	BendDistanceStiffness            BezierParam          `json:"bendDistanceStiffness"`            // 弯曲距离刚性曲线参数 / Bend-distance stiffness curve parameter
	UseNearDistance                  bool                 `json:"useNearDistance"`                  // 是否启用近邻距离约束 / Whether near-distance constraint is enabled
	NearDistanceMaxCount             int32                `json:"nearDistanceMaxCount"`             // 近邻距离最大计算数量 / Maximum near-distance count
	NearDistanceMaxDepth             float32              `json:"nearDistanceMaxDepth"`             // 近邻距离最大深度 / Maximum near-distance depth
	NearDistanceLength               BezierParam          `json:"nearDistanceLength"`               // 近邻距离长度曲线参数 / Near-distance length curve parameter
	NearDistanceStiffness            BezierParam          `json:"nearDistanceStiffness"`            // 近邻距离刚性曲线参数 / Near-distance stiffness curve parameter
	UseRestoreRotation               bool                 `json:"useRestoreRotation"`               // 是否启用旋转恢复 / Whether rotation restoration is enabled
	RestoreRotation                  BezierParam          `json:"restoreRotation"`                  // 旋转恢复曲线参数 / Rotation restoration curve parameter
	RestoreRotationVelocityInfluence float32              `json:"restoreRotationVelocityInfluence"` // 旋转恢复速度影响系数 / Velocity influence for rotation restoration
	UseSpring                        bool                 `json:"useSpring"`                        // 是否启用弹簧力 / Whether spring force is enabled
	SpringPower                      float32              `json:"springPower"`                      // 弹簧力强度 / Spring force power
	SpringRadius                     float32              `json:"springRadius"`                     // 弹簧影响半径 / Spring influence radius
	SpringScaleX                     float32              `json:"springScaleX"`                     // 弹簧 X 轴缩放 / Spring X-axis scale
	SpringScaleY                     float32              `json:"springScaleY"`                     // 弹簧 Y 轴缩放 / Spring Y-axis scale
	SpringScaleZ                     float32              `json:"springScaleZ"`                     // 弹簧 Z 轴缩放 / Spring Z-axis scale
	SpringIntensity                  float32              `json:"springIntensity"`                  // 弹簧强度 / Spring intensity
	SpringDirectionAtten             BezierParam          `json:"springDirectionAtten"`             // 弹簧方向衰减曲线参数 / Spring direction attenuation curve parameter
	SpringDistanceAtten              BezierParam          `json:"springDistanceAtten"`              // 弹簧距离衰减曲线参数 / Spring distance attenuation curve parameter
	Reserved56                       RawMessagePackSlot   `json:"reserved56,omitempty"`             // C# 无 Key(56)；原始稀疏槽位 / C# has no Key(56); raw sparse slot
	AdjustMode                       ClothAdjustMode      `json:"adjustMode"`                       // 调整模式枚举 / Adjustment mode enum
	AdjustRotationPower              float32              `json:"adjustRotationPower"`              // 调整旋转力度 / Adjustment rotation power
	UseTriangleBend                  bool                 `json:"useTriangleBend"`                  // 是否启用三角形弯曲 / Whether triangle bend is enabled
	TriangleBend                     BezierParam          `json:"triangleBend"`                     // 三角形弯曲曲线参数 / Triangle bend curve parameter
	UseVolume                        bool                 `json:"useVolume"`                        // 是否启用体积约束 / Whether volume constraint is enabled
	MaxVolumeLength                  float32              `json:"maxVolumeLength"`                  // 最大体积边长 / Maximum volume length
	VolumeStretchStiffness           BezierParam          `json:"volumeStretchStiffness"`           // 体积拉伸刚性曲线参数 / Volume stretch stiffness curve parameter
	VolumeShearStiffness             BezierParam          `json:"volumeShearStiffness"`             // 体积剪切刚性曲线参数 / Volume shear stiffness curve parameter
	UseCollision                     bool                 `json:"useCollision"`                     // 是否启用碰撞 / Whether collision is enabled
	Friction                         float32              `json:"friction"`                         // 摩擦系数 / Friction coefficient
	KeepInitialShape                 bool                 `json:"keepInitialShape"`                 // 是否保持初始形状 / Whether the initial shape is kept
	UsePenetration                   bool                 `json:"usePenetration"`                   // 是否启用穿透修正 / Whether penetration correction is enabled
	PenetrationMode                  ClothPenetrationMode `json:"penetrationMode"`                  // 穿透修正模式枚举 / Penetration correction mode enum
	PenetrationAxis                  ClothPenetrationAxis `json:"penetrationAxis"`                  // 穿透修正轴枚举 / Penetration correction axis enum
	PenetrationMaxDepth              float32              `json:"penetrationMaxDepth"`              // 最大穿透深度 / Maximum penetration depth
	PenetrationConnectDistance       BezierParam          `json:"penetrationConnectDistance"`       // 穿透连接距离曲线参数 / Penetration connection distance curve parameter
	PenetrationDistance              BezierParam          `json:"penetrationDistance"`              // 穿透距离曲线参数 / Penetration distance curve parameter
	PenetrationRadius                BezierParam          `json:"penetrationRadius"`                // 穿透半径曲线参数 / Penetration radius curve parameter
	UseLineAvarageRotation           bool                 `json:"useLineAvarageRotation"`           // 是否使用线段平均旋转，字段名保留游戏 Avarage 拼写 / Whether line average rotation is used, keeping the game's Avarage spelling
	UseFixedNonRotation              bool                 `json:"useFixedNonRotation"`              // 是否固定非旋转姿态 / Whether non-rotation pose is fixed
	GravityDirection                 Vector3              `json:"gravityDirection"`                 // 重力方向 / Gravity direction
	MaxMoveSpeed                     float32              `json:"maxMoveSpeed"`                     // 最大移动速度 / Maximum movement speed
	MaxRotationSpeed                 float32              `json:"maxRotationSpeed"`                 // 最大旋转速度 / Maximum rotation speed
	TeleportMode                     ClothTeleportMode    `json:"teleportMode"`                     // 传送处理模式枚举 / Teleport handling mode enum
	ResetStabilizationTime           float32              `json:"resetStabilizationTime"`           // 重置后稳定时间 / Stabilization time after reset
	ClampRotationVelocityLimit       float32              `json:"clampRotationVelocityLimit"`       // 旋转约束速度上限 / Rotation clamp velocity limit
}

// NewClothParams 返回与 MagicaCloth.ClothParams 的 C# 字段初始化器相同的默认值，直接在 Go 中创建配置时应使用此函数，MessagePack 解码和 JSON 反序列化只保留提供的线格式或编辑字段，不会运行此构造逻辑
// NewClothParams returns the same field defaults as the C# field initializers of MagicaCloth.ClothParams for direct Go construction, while MessagePack decoding and JSON unmarshalling deliberately retain only supplied wire or editing fields and do not run this constructor
func NewClothParams() *ClothParams {
	return &ClothParams{
		Radius:                           newBezierParam(0.02, 0.02, true, 0, false),
		Mass:                             newBezierParam(1, 1, true, 0, false),
		UseGravity:                       true,
		Gravity:                          newBezierParam(-9.8, -9.8, false, 0, false),
		UseDrag:                          true,
		Drag:                             newBezierParam(0.02, 0.02, true, 0, false),
		UseMaxVelocity:                   true,
		MaxVelocity:                      newBezierParam(3, 3, false, 0, false),
		WorldMoveInfluence:               newBezierParam(0.5, 0.5, false, 0, false),
		WorldRotationInfluence:           newBezierParam(0.5, 0.5, false, 0, false),
		MassInfluence:                    0.3,
		WindInfluence:                    1,
		WindRandomScale:                  0.7,
		DisableDistance:                  20,
		DisableFadeDistance:              5,
		TeleportDistance:                 0.2,
		TeleportRotation:                 45,
		UseClampDistanceRatio:            true,
		ClampDistanceMinRatio:            0.7,
		ClampDistanceMaxRatio:            1.1,
		ClampDistanceVelocityInfluence:   0.2,
		ClampPositionLength:              newBezierParam(0.03, 0.2, true, 0, false),
		ClampPositionRatioX:              1,
		ClampPositionRatioY:              1,
		ClampPositionRatioZ:              1,
		ClampPositionVelocityInfluence:   0.2,
		ClampRotationAngle:               newBezierParam(30, 30, true, 0, false),
		ClampRotationVelocityInfluence:   0.2,
		RestoreDistanceVelocityInfluence: 1,
		StructDistanceStiffness:          newBezierParam(1, 1, false, 0, false),
		BendDistanceMaxCount:             2,
		BendDistanceStiffness:            newBezierParam(0.5, 0.5, false, 0, false),
		NearDistanceMaxCount:             3,
		NearDistanceMaxDepth:             1,
		NearDistanceLength:               newBezierParam(0.1, 0.1, true, 0, false),
		NearDistanceStiffness:            newBezierParam(0.3, 0.3, false, 0, false),
		RestoreRotation:                  newBezierParam(0.3, 0.1, true, 0, false),
		RestoreRotationVelocityInfluence: 0.2,
		SpringPower:                      0.017,
		SpringRadius:                     0.1,
		SpringScaleX:                     1,
		SpringScaleY:                     1,
		SpringScaleZ:                     1,
		SpringIntensity:                  1,
		SpringDirectionAtten:             newBezierParam(1, 0, true, 0.234, true),
		SpringDistanceAtten:              newBezierParam(1, 0, true, 0.395, true),
		AdjustRotationPower:              5,
		TriangleBend:                     newBezierParam(0.5, 0.5, true, 0, false),
		MaxVolumeLength:                  0.1,
		VolumeStretchStiffness:           newBezierParam(0.5, 0.5, true, 0, false),
		VolumeShearStiffness:             newBezierParam(0.5, 0.5, true, 0, false),
		Friction:                         0.2,
		PenetrationAxis:                  ClothPenetrationAxisInverseZ,
		PenetrationMaxDepth:              1,
		PenetrationConnectDistance:       newBezierParam(0.2, 0.3, true, 0, false),
		PenetrationDistance:              newBezierParam(0.1, 0.2, true, 0, false),
		PenetrationRadius:                newBezierParam(0.3, 1, true, 0, false),
		UseLineAvarageRotation:           true,
		GravityDirection:                 Vector3{Y: 1},
		MaxMoveSpeed:                     10,
		MaxRotationSpeed:                 360,
		ResetStabilizationTime:           0.1,
		ClampRotationVelocityLimit:       1,
	}
}

// UnmarshalJSON 将省略的编辑字段保持在线格式模型零值，新建当前游戏对象时应显式调用 NewClothParams
// UnmarshalJSON keeps omitted editing fields at their wire-model zero values, and NewClothParams should be called explicitly for a new current-game object
func (p *ClothParams) UnmarshalJSON(data []byte) error {
	type plainClothParams ClothParams
	var value plainClothParams
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*p = ClothParams(value)
	return nil
}

// validateClothParamsForEncoding 验证由 C# Int32 或 Int32 底层枚举保存的布料字段
// validateClothParamsForEncoding validates cloth fields stored as C# Int32 values or enums with an Int32 underlying type
func validateClothParamsForEncoding(params *ClothParams) error {
	return nil
}
