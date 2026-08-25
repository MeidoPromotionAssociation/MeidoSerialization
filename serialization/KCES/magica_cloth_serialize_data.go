package KCES

// MagicaCloth2 的 ClothSerializeData Unity JSON 文档模型，.db2conf、.dsb2conf 与 .dsl2conf 的
// MessagePack 字符串载荷保存的就是这份文档
//
// 每个成员都是可选的：KCES 内的这些文件由不同时期的 MagicaCloth2 版本写出，成员集合随版本演进，
// 解码保留原文件出现过的成员，编码只写回这些成员，不会注入原文件没有的成员
//
// 枚举型 int32 成员按线格式原样保留，允许值未经核实，不做取值约束
//
// Unity JSON document model for the MagicaCloth2 ClothSerializeData held by the MessagePack string
// payload of .db2conf, .dsb2conf, and .dsl2conf files
//
// Every member is optional: these files were written by different MagicaCloth2 versions across the
// history of KCES and their member sets differ by version, so decoding keeps exactly the members the
// original file contained and encoding writes back only those members without injecting new ones
//
// Enum-typed int32 members are preserved as stored because their allowed values are unverified

// MagicaClothSerializeData 表示 MagicaCloth2 ClothSerializeData 的 Unity JSON 表示，成员顺序与 Unity JsonUtility 的写出顺序一致 / MagicaClothSerializeData represents the Unity JSON form of MagicaCloth2 ClothSerializeData with members ordered as Unity JsonUtility writes them
type MagicaClothSerializeData struct {
	ClothType                   *int32                             `json:"clothType,omitempty"`                   // 布料类型枚举，允许值未核实 / Cloth-type enum with unverified allowed values
	SourceRenderers             *[]UnityInstanceReference          `json:"sourceRenderers,omitempty"`             // 源 Renderer 引用数组 / Array of source Renderer references
	MeshWriteMode               *int32                             `json:"meshWriteMode,omitempty"`               // 网格写回模式枚举，较早写出的文件没有这个成员 / Mesh write-back mode enum, absent from files written by earlier versions
	PaintMode                   *int32                             `json:"paintMode,omitempty"`                   // 绘制模式枚举，允许值未核实 / Paint-mode enum with unverified allowed values
	PaintMaps                   *[]UnityInstanceReference          `json:"paintMaps,omitempty"`                   // 绘制贴图引用数组 / Array of paint-map references
	PaintMapUvChannel           *int32                             `json:"paintMapUvChannel,omitempty"`           // 绘制贴图使用的 UV 通道，较早写出的文件没有这个成员 / UV channel used by the paint maps, absent from files written by earlier versions
	RootBones                   *[]UnityInstanceReference          `json:"rootBones,omitempty"`                   // 根骨骼 Transform 引用数组 / Array of root-bone Transform references
	ConnectionMode              *int32                             `json:"connectionMode,omitempty"`              // 连接模式枚举，允许值未核实 / Connection-mode enum with unverified allowed values
	RotationalInterpolation     *float32                           `json:"rotationalInterpolation,omitempty"`     // 旋转插值系数 / Rotational interpolation factor
	RootRotation                *float32                           `json:"rootRotation,omitempty"`                // 根旋转系数 / Root rotation factor
	UpdateMode                  *int32                             `json:"updateMode,omitempty"`                  // 更新模式枚举，允许值未核实 / Update-mode enum with unverified allowed values
	AnimationPoseRatio          *float32                           `json:"animationPoseRatio,omitempty"`          // 动画姿态混合比例 / Animation pose blend ratio
	ReductionSetting            *MagicaReductionSettings           `json:"reductionSetting,omitempty"`            // 顶点缩减设置 / Vertex reduction settings
	CustomSkinningSetting       *MagicaCustomSkinningSettings      `json:"customSkinningSetting,omitempty"`       // 自定义蒙皮设置 / Custom skinning settings
	NormalAlignmentSetting      *MagicaNormalAlignmentSettings     `json:"normalAlignmentSetting,omitempty"`      // 法线对齐设置 / Normal alignment settings
	CullingSettings             *MagicaCullingSettings             `json:"cullingSettings,omitempty"`             // 剔除设置，较早写出的文件没有这个成员 / Culling settings, absent from files written by earlier versions
	NormalAxis                  *int32                             `json:"normalAxis,omitempty"`                  // 法线轴枚举，允许值未核实 / Normal-axis enum with unverified allowed values
	Gravity                     *float32                           `json:"gravity,omitempty"`                     // 重力强度 / Gravity magnitude
	GravityDirection            *Vector3                           `json:"gravityDirection,omitempty"`            // 重力方向向量 / Gravity direction vector
	GravityFalloff              *float32                           `json:"gravityFalloff,omitempty"`              // 重力衰减系数 / Gravity falloff factor
	StablizationTimeAfterReset  *float32                           `json:"stablizationTimeAfterReset,omitempty"`  // 重置后的稳定时间，键名的拼写沿用线格式 / Stabilization time after a reset, with the key spelled as stored
	BlendWeight                 *float32                           `json:"blendWeight,omitempty"`                 // 混合权重，较早写出的文件没有这个成员 / Blend weight, absent from files written by earlier versions
	Damping                     *MagicaCurveSerializeData          `json:"damping,omitempty"`                     // 阻尼值或阻尼曲线 / Damping value or damping curve
	Radius                      *MagicaCurveSerializeData          `json:"radius,omitempty"`                      // 粒子半径值或半径曲线 / Particle radius value or radius curve
	InertiaConstraint           *MagicaInertiaConstraint           `json:"inertiaConstraint,omitempty"`           // 惯性约束 / Inertia constraint
	TetherConstraint            *MagicaTetherConstraint            `json:"tetherConstraint,omitempty"`            // 系绳约束 / Tether constraint
	DistanceConstraint          *MagicaDistanceConstraint          `json:"distanceConstraint,omitempty"`          // 距离约束 / Distance constraint
	TriangleBendingConstraint   *MagicaTriangleBendingConstraint   `json:"triangleBendingConstraint,omitempty"`   // 三角面弯曲约束 / Triangle bending constraint
	AngleRestorationConstraint  *MagicaAngleRestorationConstraint  `json:"angleRestorationConstraint,omitempty"`  // 角度复原约束 / Angle restoration constraint
	AngleLimitConstraint        *MagicaAngleLimitConstraint        `json:"angleLimitConstraint,omitempty"`        // 角度限制约束 / Angle limit constraint
	MotionConstraint            *MagicaMotionConstraint            `json:"motionConstraint,omitempty"`            // 运动范围约束 / Motion range constraint
	ColliderCollisionConstraint *MagicaColliderCollisionConstraint `json:"colliderCollisionConstraint,omitempty"` // 碰撞体碰撞约束 / Collider collision constraint
	SelfCollisionConstraint     *MagicaSelfCollisionConstraint     `json:"selfCollisionConstraint,omitempty"`     // 自碰撞约束 / Self collision constraint
	Wind                        *MagicaWindSettings                `json:"wind,omitempty"`                        // 风力设置 / Wind settings
	SpringConstraint            *MagicaSpringConstraint            `json:"springConstraint,omitempty"`            // 弹簧约束，较早写出的文件没有这个成员 / Spring constraint, absent from files written by earlier versions
}

// UnityInstanceReference 表示 Unity JsonUtility 写出的 UnityEngine.Object 引用，instanceID 为 0 表示空引用 / UnityInstanceReference is the UnityEngine.Object reference emitted by Unity JsonUtility, where instanceID 0 means a null reference
type UnityInstanceReference struct {
	InstanceID *int32 `json:"instanceID,omitempty"` // Unity 运行时实例 ID，跨会话没有稳定含义 / Unity runtime instance ID with no stable meaning across sessions
}

// MagicaReductionSettings 表示 ClothSerializeData 的 reductionSetting 成员 / MagicaReductionSettings represents the reductionSetting member of ClothSerializeData
type MagicaReductionSettings struct {
	SimpleDistance *float32 `json:"simpleDistance,omitempty"` // 简单合并距离 / Simple merge distance
	ShapeDistance  *float32 `json:"shapeDistance,omitempty"`  // 形状合并距离 / Shape merge distance
}

// MagicaCustomSkinningSettings 表示 ClothSerializeData 的 customSkinningSetting 成员 / MagicaCustomSkinningSettings represents the customSkinningSetting member of ClothSerializeData
type MagicaCustomSkinningSettings struct {
	Enable        *bool                     `json:"enable,omitempty"`        // 是否启用自定义蒙皮 / Whether custom skinning is enabled
	SkinningBones *[]UnityInstanceReference `json:"skinningBones,omitempty"` // 参与蒙皮的骨骼引用数组 / Array of skinning bone references
}

// MagicaNormalAlignmentSettings 表示 ClothSerializeData 的 normalAlignmentSetting 成员 / MagicaNormalAlignmentSettings represents the normalAlignmentSetting member of ClothSerializeData
type MagicaNormalAlignmentSettings struct {
	AlignmentMode       *int32                  `json:"alignmentMode,omitempty"`       // 法线对齐模式枚举，允许值未核实 / Normal alignment mode enum with unverified allowed values
	AdjustmentTransform *UnityInstanceReference `json:"adjustmentTransform,omitempty"` // 调整基准 Transform 引用 / Reference to the adjustment Transform
}

// MagicaCullingSettings 表示 ClothSerializeData 的 cullingSettings 成员 / MagicaCullingSettings represents the cullingSettings member of ClothSerializeData
type MagicaCullingSettings struct {
	CameraCullingMode              *int32                    `json:"cameraCullingMode,omitempty"`              // 摄像机剔除模式枚举，允许值未核实 / Camera culling mode enum with unverified allowed values
	CameraCullingMethod            *int32                    `json:"cameraCullingMethod,omitempty"`            // 摄像机剔除方式枚举，允许值未核实 / Camera culling method enum with unverified allowed values
	CameraCullingRenderers         *[]UnityInstanceReference `json:"cameraCullingRenderers,omitempty"`         // 参与摄像机剔除判定的 Renderer 引用数组 / Array of Renderer references used for camera culling
	DistanceCullingLength          *MagicaToggleValue        `json:"distanceCullingLength,omitempty"`          // 距离剔除长度开关值，较早写出的文件没有这个成员 / Toggled distance-culling length, absent from files written by earlier versions
	DistanceCullingFadeRatio       *float32                  `json:"distanceCullingFadeRatio,omitempty"`       // 距离剔除淡出比例，较早写出的文件没有这个成员 / Distance-culling fade ratio, absent from files written by earlier versions
	DistanceCullingReferenceObject *UnityInstanceReference   `json:"distanceCullingReferenceObject,omitempty"` // 距离剔除参考对象引用，较早写出的文件没有这个成员 / Reference object for distance culling, absent from files written by earlier versions
}

// MagicaInertiaConstraint 表示 ClothSerializeData 的 inertiaConstraint 成员，成员顺序按当前 KCES 写出的布局，早期布局的成员顺序不同但集合是它的子集加上 movementInertia 与 rotationInertia / MagicaInertiaConstraint represents the inertiaConstraint member of ClothSerializeData, ordered as the current KCES layout writes it, while the earlier layout orders its members differently and is otherwise a subset plus movementInertia and rotationInertia
type MagicaInertiaConstraint struct {
	Anchor                   *UnityInstanceReference `json:"anchor,omitempty"`                   // 惯性锚点 Transform 引用 / Reference to the inertia anchor Transform
	AnchorInertia            *float32                `json:"anchorInertia,omitempty"`            // 锚点惯性系数 / Anchor inertia factor
	MovementInertia          *float32                `json:"movementInertia,omitempty"`          // 早期布局的移动惯性系数，当前布局改用 worldInertia 与 localInertia / Movement inertia factor in the earlier layout, replaced by worldInertia and localInertia in the current layout
	RotationInertia          *float32                `json:"rotationInertia,omitempty"`          // 早期布局的旋转惯性系数，当前布局改用 worldInertia 与 localInertia / Rotation inertia factor in the earlier layout, replaced by worldInertia and localInertia in the current layout
	WorldInertia             *float32                `json:"worldInertia,omitempty"`             // 世界空间惯性系数 / World-space inertia factor
	MovementInertiaSmoothing *float32                `json:"movementInertiaSmoothing,omitempty"` // 移动惯性平滑系数 / Movement inertia smoothing factor
	MovementSpeedLimit       *MagicaToggleValue      `json:"movementSpeedLimit,omitempty"`       // 移动速度上限开关值 / Toggled movement speed limit
	RotationSpeedLimit       *MagicaToggleValue      `json:"rotationSpeedLimit,omitempty"`       // 旋转速度上限开关值 / Toggled rotation speed limit
	LocalInertia             *float32                `json:"localInertia,omitempty"`             // 局部空间惯性系数 / Local-space inertia factor
	LocalMovementSpeedLimit  *MagicaToggleValue      `json:"localMovementSpeedLimit,omitempty"`  // 局部移动速度上限开关值 / Toggled local movement speed limit
	LocalRotationSpeedLimit  *MagicaToggleValue      `json:"localRotationSpeedLimit,omitempty"`  // 局部旋转速度上限开关值 / Toggled local rotation speed limit
	DepthInertia             *float32                `json:"depthInertia,omitempty"`             // 按深度衰减的惯性系数 / Depth-scaled inertia factor
	CentrifualAcceleration   *float32                `json:"centrifualAcceleration,omitempty"`   // 离心加速度系数，键名的拼写沿用线格式 / Centrifugal acceleration factor, with the key spelled as stored
	ParticleSpeedLimit       *MagicaToggleValue      `json:"particleSpeedLimit,omitempty"`       // 粒子速度上限开关值 / Toggled particle speed limit
	TeleportMode             *int32                  `json:"teleportMode,omitempty"`             // 传送处理模式枚举，允许值未核实 / Teleport handling mode enum with unverified allowed values
	TeleportDistance         *float32                `json:"teleportDistance,omitempty"`         // 判定为传送的位移阈值 / Displacement threshold treated as a teleport
	TeleportRotation         *float32                `json:"teleportRotation,omitempty"`         // 判定为传送的旋转阈值 / Rotation threshold treated as a teleport
}

// MagicaTetherConstraint 表示 ClothSerializeData 的 tetherConstraint 成员 / MagicaTetherConstraint represents the tetherConstraint member of ClothSerializeData
type MagicaTetherConstraint struct {
	DistanceCompression *float32 `json:"distanceCompression,omitempty"` // 距离压缩系数 / Distance compression factor
}

// MagicaDistanceConstraint 表示 ClothSerializeData 的 distanceConstraint 成员 / MagicaDistanceConstraint represents the distanceConstraint member of ClothSerializeData
type MagicaDistanceConstraint struct {
	Stiffness *MagicaCurveSerializeData `json:"stiffness,omitempty"` // 刚性值或刚性曲线 / Stiffness value or stiffness curve
}

// MagicaTriangleBendingConstraint 表示 ClothSerializeData 的 triangleBendingConstraint 成员 / MagicaTriangleBendingConstraint represents the triangleBendingConstraint member of ClothSerializeData
type MagicaTriangleBendingConstraint struct {
	Stiffness *float32 `json:"stiffness,omitempty"` // 刚性值 / Stiffness value
}

// MagicaAngleRestorationConstraint 表示 ClothSerializeData 的 angleRestorationConstraint 成员 / MagicaAngleRestorationConstraint represents the angleRestorationConstraint member of ClothSerializeData
type MagicaAngleRestorationConstraint struct {
	UseAngleRestoration *bool                     `json:"useAngleRestoration,omitempty"` // 是否启用角度复原 / Whether angle restoration is enabled
	Stiffness           *MagicaCurveSerializeData `json:"stiffness,omitempty"`           // 刚性值或刚性曲线 / Stiffness value or stiffness curve
	VelocityAttenuation *float32                  `json:"velocityAttenuation,omitempty"` // 速度衰减系数 / Velocity attenuation factor
	GravityFalloff      *float32                  `json:"gravityFalloff,omitempty"`      // 重力衰减系数 / Gravity falloff factor
}

// MagicaAngleLimitConstraint 表示 ClothSerializeData 的 angleLimitConstraint 成员 / MagicaAngleLimitConstraint represents the angleLimitConstraint member of ClothSerializeData
type MagicaAngleLimitConstraint struct {
	UseAngleLimit *bool                     `json:"useAngleLimit,omitempty"` // 是否启用角度限制 / Whether the angle limit is enabled
	LimitAngle    *MagicaCurveSerializeData `json:"limitAngle,omitempty"`    // 限制角度值或角度曲线 / Limit angle value or angle curve
	Stiffness     *float32                  `json:"stiffness,omitempty"`     // 刚性值 / Stiffness value
}

// MagicaMotionConstraint 表示 ClothSerializeData 的 motionConstraint 成员 / MagicaMotionConstraint represents the motionConstraint member of ClothSerializeData
type MagicaMotionConstraint struct {
	UseMaxDistance   *bool                     `json:"useMaxDistance,omitempty"`   // 是否启用最大距离限制 / Whether the maximum distance limit is enabled
	MaxDistance      *MagicaCurveSerializeData `json:"maxDistance,omitempty"`      // 最大距离值或距离曲线 / Maximum distance value or distance curve
	UseBackstop      *bool                     `json:"useBackstop,omitempty"`      // 是否启用背面阻挡 / Whether the backstop is enabled
	BackstopRadius   *float32                  `json:"backstopRadius,omitempty"`   // 背面阻挡半径 / Backstop radius
	BackstopDistance *MagicaCurveSerializeData `json:"backstopDistance,omitempty"` // 背面阻挡距离值或距离曲线 / Backstop distance value or distance curve
	Stiffness        *float32                  `json:"stiffness,omitempty"`        // 刚性值 / Stiffness value
}

// MagicaColliderCollisionConstraint 表示 ClothSerializeData 的 colliderCollisionConstraint 成员 / MagicaColliderCollisionConstraint represents the colliderCollisionConstraint member of ClothSerializeData
type MagicaColliderCollisionConstraint struct {
	Mode           *int32                    `json:"mode,omitempty"`           // 碰撞模式枚举，允许值未核实 / Collision mode enum with unverified allowed values
	Friction       *float32                  `json:"friction,omitempty"`       // 摩擦系数 / Friction factor
	ColliderList   *[]UnityInstanceReference `json:"colliderList,omitempty"`   // 参与碰撞的碰撞体组件引用数组 / Array of collider component references used for collision
	CollisionBones *[]UnityInstanceReference `json:"collisionBones,omitempty"` // 参与碰撞的骨骼引用数组，较早写出的文件没有这个成员 / Array of collision bone references, absent from files written by earlier versions
	LimitDistance  *MagicaCurveSerializeData `json:"limitDistance,omitempty"`  // 限制距离值或距离曲线，较早写出的文件没有这个成员 / Limit distance value or distance curve, absent from files written by earlier versions
}

// MagicaSelfCollisionConstraint 表示 ClothSerializeData 的 selfCollisionConstraint 成员 / MagicaSelfCollisionConstraint represents the selfCollisionConstraint member of ClothSerializeData
type MagicaSelfCollisionConstraint struct {
	SelfMode         *int32                    `json:"selfMode,omitempty"`         // 自碰撞模式枚举，允许值未核实 / Self collision mode enum with unverified allowed values
	SurfaceThickness *MagicaCurveSerializeData `json:"surfaceThickness,omitempty"` // 表面厚度值或厚度曲线 / Surface thickness value or thickness curve
	SyncMode         *int32                    `json:"syncMode,omitempty"`         // 同步模式枚举，允许值未核实 / Sync mode enum with unverified allowed values
	SyncPartner      *UnityInstanceReference   `json:"syncPartner,omitempty"`      // 同步对象引用 / Reference to the sync partner
	ClothMass        *float32                  `json:"clothMass,omitempty"`        // 布料质量 / Cloth mass
}

// MagicaWindSettings 表示 ClothSerializeData 的 wind 成员 / MagicaWindSettings represents the wind member of ClothSerializeData
type MagicaWindSettings struct {
	Influence       *float32 `json:"influence,omitempty"`       // 风力影响强度 / Wind influence magnitude
	Frequency       *float32 `json:"frequency,omitempty"`       // 风力频率 / Wind frequency
	Turbulence      *float32 `json:"turbulence,omitempty"`      // 风力扰动强度 / Wind turbulence magnitude
	Blend           *float32 `json:"blend,omitempty"`           // 风力混合系数 / Wind blend factor
	Synchronization *float32 `json:"synchronization,omitempty"` // 风力同步系数 / Wind synchronization factor
	DepthWeight     *float32 `json:"depthWeight,omitempty"`     // 按深度加权的系数 / Depth-based weighting factor
	MovingWind      *float32 `json:"movingWind,omitempty"`      // 移动风力系数 / Moving wind factor
}

// MagicaSpringConstraint 表示 ClothSerializeData 的 springConstraint 成员 / MagicaSpringConstraint represents the springConstraint member of ClothSerializeData
type MagicaSpringConstraint struct {
	UseSpring        *bool    `json:"useSpring,omitempty"`        // 是否启用弹簧约束 / Whether the spring constraint is enabled
	SpringPower      *float32 `json:"springPower,omitempty"`      // 弹簧强度 / Spring power
	LimitDistance    *float32 `json:"limitDistance,omitempty"`    // 限制距离 / Limit distance
	NormalLimitRatio *float32 `json:"normalLimitRatio,omitempty"` // 法线方向限制比例 / Normal-direction limit ratio
	SpringNoise      *float32 `json:"springNoise,omitempty"`      // 弹簧噪声强度 / Spring noise magnitude
}

// MagicaToggleValue 表示由一个数值和一个启用标志组成的开关值对 / MagicaToggleValue is the pair of one numeric value and one enable flag
type MagicaToggleValue struct {
	Value *float32 `json:"value,omitempty"` // 数值 / Numeric value
	Use   *bool    `json:"use,omitempty"`   // 是否启用该数值 / Whether the value is in use
}

// MagicaCurveSerializeData 表示可以退化为常量的曲线参数，useCurve 为 false 时游戏取 value / MagicaCurveSerializeData represents a curve parameter that can degrade to a constant, with the game taking value when useCurve is false
type MagicaCurveSerializeData struct {
	Value    *float32             `json:"value,omitempty"`    // 常量取值 / Constant value
	UseCurve *bool                `json:"useCurve,omitempty"` // 是否改用曲线取值 / Whether the curve is used instead
	Curve    *UnityAnimationCurve `json:"curve,omitempty"`    // 曲线 / The curve
}

// UnityAnimationCurve 表示 Unity JsonUtility 写出的 UnityEngine.AnimationCurve / UnityAnimationCurve is the UnityEngine.AnimationCurve form emitted by Unity JsonUtility
type UnityAnimationCurve struct {
	SerializedVersion *string          `json:"serializedVersion,omitempty"` // Unity 序列化版本，按字符串写出 / Unity serialization version written as a string
	Curve             *[]UnityKeyframe `json:"m_Curve,omitempty"`           // 关键帧数组 / Array of keyframes
	PreInfinity       *int32           `json:"m_PreInfinity,omitempty"`     // 曲线起点外推模式 / Pre-extrapolation mode of the curve
	PostInfinity      *int32           `json:"m_PostInfinity,omitempty"`    // 曲线终点外推模式 / Post-extrapolation mode of the curve
	RotationOrder     *int32           `json:"m_RotationOrder,omitempty"`   // 旋转顺序枚举，允许值未核实 / Rotation-order enum with unverified allowed values
}

// UnityKeyframe 表示 Unity JsonUtility 写出的 UnityEngine.Keyframe / UnityKeyframe is the UnityEngine.Keyframe form emitted by Unity JsonUtility
type UnityKeyframe struct {
	SerializedVersion *string  `json:"serializedVersion,omitempty"` // Unity 序列化版本，按字符串写出 / Unity serialization version written as a string
	Time              *float32 `json:"time,omitempty"`              // 关键帧时间 / Keyframe time
	Value             *float32 `json:"value,omitempty"`             // 关键帧值 / Keyframe value
	InSlope           *float32 `json:"inSlope,omitempty"`           // 入斜率 / Incoming slope
	OutSlope          *float32 `json:"outSlope,omitempty"`          // 出斜率 / Outgoing slope
	TangentMode       *int32   `json:"tangentMode,omitempty"`       // 切线模式枚举，允许值未核实 / Tangent-mode enum with unverified allowed values
	WeightedMode      *int32   `json:"weightedMode,omitempty"`      // 权重模式枚举，允许值未核实 / Weighted-mode enum with unverified allowed values
	InWeight          *float32 `json:"inWeight,omitempty"`          // 入权重 / Incoming weight
	OutWeight         *float32 `json:"outWeight,omitempty"`         // 出权重 / Outgoing weight
}
