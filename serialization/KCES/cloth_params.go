package KCES

// BezierParam 对应 MagicaCloth.BezierParam / BezierParam corresponds to MagicaCloth.BezierParam
type BezierParam struct {
	_struct       struct{} `codec:",toarray"`     // 强制按数组编码 / Forces array encoding
	StartValue    float32  `json:"startValue"`    // 起始值 / Start value
	EndValue      float32  `json:"endValue"`      // 结束值 / End value
	UseEndValue   bool     `json:"useEndValue"`   // 是否使用结束值 / Whether the end value is used
	CurveValue    float32  `json:"curveValue"`    // 曲线值 / Curve value
	UseCurveValue bool     `json:"useCurveValue"` // 是否使用曲线值 / Whether the curve value is used
}

type ClothTeleportMode int

const (
	ClothTeleportModeReset ClothTeleportMode = iota
	ClothTeleportModeKeep
)

type ClothAdjustMode int

const (
	ClothAdjustModeFixed ClothAdjustMode = iota
	ClothAdjustModeXYMove
	ClothAdjustModeXZMove
	ClothAdjustModeYZMove
)

type ClothPenetrationMode int

const (
	ClothPenetrationModeSurfacePenetration ClothPenetrationMode = iota
	ClothPenetrationModeColliderPenetration
)

type ClothPenetrationAxis int

const (
	ClothPenetrationAxisX ClothPenetrationAxis = iota
	ClothPenetrationAxisY
	ClothPenetrationAxisZ
	ClothPenetrationAxisInverseX
	ClothPenetrationAxisInverseY
	ClothPenetrationAxisInverseZ
)

// ClothParams 对应 MagicaCloth.ClothParams / ClothParams corresponds to MagicaCloth.ClothParams
// MessagePack-CSharp 以 Key(0)..Key(82) 的 indexed array 写入，Key(4)、Key(5)、Key(56) 是当前游戏类型中的空洞并需要保留 / MessagePack-CSharp writes keys 0..82 as an indexed array, with sparse holes at Key(4), Key(5), and Key(56)
type ClothParams struct {
	_struct                          struct{}             `codec:",toarray"`                        // 强制按数组编码 / Forces array encoding
	Radius                           BezierParam          `json:"radius"`                           // 粒子半径曲线参数 / Particle radius curve parameter
	Mass                             BezierParam          `json:"mass"`                             // 质量曲线参数 / Mass curve parameter
	UseGravity                       bool                 `json:"useGravity"`                       // 是否使用重力 / Whether gravity is enabled
	Gravity                          BezierParam          `json:"gravity"`                          // 重力强度曲线参数 / Gravity strength curve parameter
	Reserved04                       interface{}          `json:"reserved04,omitempty"`             // 当前游戏未使用的 Key(4) 占位 / Placeholder for currently unused game Key(4)
	Reserved05                       interface{}          `json:"reserved05,omitempty"`             // 当前游戏未使用的 Key(5) 占位 / Placeholder for currently unused game Key(5)
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
	BendDistanceMaxCount             int                  `json:"bendDistanceMaxCount"`             // 弯曲距离最大计算数量 / Maximum bend-distance count
	BendDistanceStiffness            BezierParam          `json:"bendDistanceStiffness"`            // 弯曲距离刚性曲线参数 / Bend-distance stiffness curve parameter
	UseNearDistance                  bool                 `json:"useNearDistance"`                  // 是否启用近邻距离约束 / Whether near-distance constraint is enabled
	NearDistanceMaxCount             int                  `json:"nearDistanceMaxCount"`             // 近邻距离最大计算数量 / Maximum near-distance count
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
	Reserved56                       interface{}          `json:"reserved56,omitempty"`             // 当前游戏未使用的 Key(56) 占位 / Placeholder for currently unused game Key(56)
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
