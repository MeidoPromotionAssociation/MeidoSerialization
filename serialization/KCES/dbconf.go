package KCES

import (
	"encoding/json"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// .dbconf
// DynamicBone 的旧版参数文件。文件先写入 Int32 压缩数据长度，再写入 LZ4 Block Array 压缩的
// DynamicBoneStatus MessagePack indexed-array；当前对象版本为 1000。ExportCM 也会使用同一扩展名
// 写出直接 UTF-8 Unity JSON。
//
// .dbconf
// Legacy DynamicBone parameter file. It stores an Int32 compressed-data length followed by an
// LZ4 Block Array-compressed DynamicBoneStatus MessagePack indexed array; the current object version is 1000.
// ExportCM also writes direct UTF-8 Unity JSON with this extension.

const KCESDBConfExtension = ".dbconf"

var dbconfPayloadDescriptor = kcesPayloadDescriptor{
	Extension:              KCESDBConfExtension,
	Kind:                   PayloadKindDynamicBoneStatus,
	LengthPrefixed:         true,
	ExportCMKind:           PayloadKindExportCMDynamicBoneJSON,
	ExportCMStorageVariant: PayloadStorageExportCMUnityJSON,
}

// DynamicBoneStatus 对应游戏的 DynamicBoneStatus。
// 游戏按 MessagePack indexed-array 写入：版本位于 Key(0)，其余字段位于 Key(1) 至 Key(15)。
//
// DynamicBoneStatus corresponds to the game's DynamicBoneStatus.
// The game writes it as a MessagePack indexed array with the version at Key(0) and fields at Key(1) through Key(15).
type DynamicBoneStatus struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Version                int                         `json:"version"`             // 版本号，通常为 1000 / Version value, usually 1000
	Damping                float32                     `json:"damping"`             // 阻尼值 / Damping value
	DampingKeyFrames       []DynamicBoneAnimationFrame `json:"dampingKeyFrames"`    // 阻尼动画关键帧 / Damping animation keyframes
	Elasticity             float32                     `json:"elasticity"`          // 弹性值 / Elasticity value
	ElasticityKeyFrames    []DynamicBoneAnimationFrame `json:"elasticityKeyFrames"` // 弹性动画关键帧 / Elasticity animation keyframes
	Stiffness              float32                     `json:"stiffness"`           // 刚性值 / Stiffness value
	StiffnessKeyFrames     []DynamicBoneAnimationFrame `json:"stiffnessKeyFrames"`  // 刚性动画关键帧 / Stiffness animation keyframes
	Inert                  float32                     `json:"inert"`               // 惯性值 / Inert value
	InertKeyFrames         []DynamicBoneAnimationFrame `json:"inertKeyFrames"`      // 惯性动画关键帧 / Inert animation keyframes
	Radius                 float32                     `json:"radius"`              // 碰撞半径 / Collision radius
	RadiusKeyFrames        []DynamicBoneAnimationFrame `json:"radiusKeyFrames"`     // 半径动画关键帧 / Radius animation keyframes
	EndLength              float32                     `json:"endLength"`           // 末端长度 / End length
	EndOffset              Vector3                     `json:"endOffset"`           // 末端偏移 / End offset
	Gravity                Vector3                     `json:"gravity"`             // 重力向量 / Gravity vector
	Force                  Vector3                     `json:"force"`               // 外力向量 / External force vector
	FreezeAxis             int                         `json:"freezeAxis"`          // 冻结轴枚举 / Freeze-axis enum
}

// DynamicBoneAnimationFrame 表示 DynamicBoneStatus 的一个动画关键帧。
//
// DynamicBoneAnimationFrame represents one animation keyframe in DynamicBoneStatus.
type DynamicBoneAnimationFrame struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Time                   float32 `json:"time"`       // 关键帧时间 / Keyframe time
	Value                  float32 `json:"value"`      // 关键帧值 / Keyframe value
	InTangent              float32 `json:"inTangent"`  // 入切线 / Incoming tangent
	OutTangent             float32 `json:"outTangent"` // 出切线 / Outgoing tangent
}

func (v DynamicBoneStatus) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

func (v *DynamicBoneStatus) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v DynamicBoneAnimationFrame) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

func (v *DynamicBoneAnimationFrame) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// NewDynamicBoneStatus 返回当前游戏构造新对象时使用的默认值。
// 解码已有文件时不会注入这些默认值。
//
// NewDynamicBoneStatus returns the defaults used by the current game when constructing a new object.
// These defaults are not injected while decoding an existing file.
func NewDynamicBoneStatus() *DynamicBoneStatus {
	return &DynamicBoneStatus{
		Version:    1000,
		Damping:    0.6,
		Elasticity: 0.1,
		Stiffness:  0.1,
		Gravity:    Vector3{Y: -0.05},
	}
}

func (s *DynamicBoneStatus) UnmarshalJSON(data []byte) error {
	type dynamicBoneStatusJSON DynamicBoneStatus
	var value dynamicBoneStatusJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = DynamicBoneStatus(value)
	return nil
}

func DecodeDynamicBoneStatusFile(data []byte) (*DynamicBoneStatus, error) {
	env, err := DecodeKCESPayload(data, KCESDBConfExtension)
	if err != nil {
		return nil, err
	}
	if env.DynamicBone == nil {
		return nil, fmt.Errorf("payload is not DynamicBoneStatus")
	}
	return env.DynamicBone, nil
}

func EncodeDynamicBoneStatusFile(status *DynamicBoneStatus) ([]byte, error) {
	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      KCESDBConfExtension,
		LengthPrefixed: true,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
		Kind:           PayloadKindDynamicBoneStatus,
		DynamicBone:    status,
	}
	return EncodeKCESPayload(env)
}

func validateDynamicBoneStatusForEncoding(status *DynamicBoneStatus) error {
	if err := requireInt32("dynamicBoneStatus.version", status.Version); err != nil {
		return err
	}
	return requireInt32("dynamicBoneStatus.freezeAxis", status.FreezeAxis)
}

func normalizeDynamicBoneStatusForEncoding(status *DynamicBoneStatus) *DynamicBoneStatus {
	normalized := *status
	normalized.DampingKeyFrames = cloneSlicePreserveNil(status.DampingKeyFrames)
	normalized.ElasticityKeyFrames = cloneSlicePreserveNil(status.ElasticityKeyFrames)
	normalized.StiffnessKeyFrames = cloneSlicePreserveNil(status.StiffnessKeyFrames)
	normalized.InertKeyFrames = cloneSlicePreserveNil(status.InertKeyFrames)
	normalized.RadiusKeyFrames = cloneSlicePreserveNil(status.RadiusKeyFrames)
	return &normalized
}
