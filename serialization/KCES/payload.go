package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

const (
	PayloadFormatKCESMessagePack = "kces-msgpack-lz4"

	PayloadKindDynamicBoneStatus = "dynamic-bone-status"
	PayloadKindJSONString        = "msgpack-json-string"
	PayloadKindRawMsgpack        = "raw-msgpack"
	PayloadKindColliderPackage   = "collider-package"
	PayloadKindLimbCollider      = "limb-collider-package"
	PayloadKindIKCollider        = "ik-collider-package"
	PayloadKindClothParams       = "cloth-params"
)

var lengthPrefixedPayloadExts = map[string]struct{}{
	".dbconf":      {},
	".dbcol":       {},
	".db2conf":     {},
	".dsbconf":     {},
	".dsb2conf":    {},
	".dslconf":     {},
	".dsl2conf":    {},
	".dslcol":      {},
	".ikcol":       {},
	".limbcol":     {},
	".ikcol.bytes": {},
}

// KCESPayloadEnvelope 是 KCES 二进制载荷的 JSON 可编辑封套 / KCESPayloadEnvelope is a JSON-editable envelope for KCES binary payloads
// 载荷通常是 MessagePack-CSharp LZ4 数据，并可能带 BinaryWriter int32 长度前缀 / Payloads are usually MessagePack-CSharp LZ4 data and may include a BinaryWriter int32 length prefix
type KCESPayloadEnvelope struct {
	Format             string               `json:"format"`                        // 封套格式标识，固定为 kces-msgpack-lz4 / Envelope format marker, fixed to kces-msgpack-lz4
	Extension          string               `json:"extension"`                     // 原始文件扩展名，用于判定载荷类型 / Original file extension used to determine payload kind
	LengthPrefixed     bool                 `json:"lengthPrefixed"`                // 是否带 4 字节长度前缀 / Whether a 4-byte length prefix is present
	Kind               string               `json:"kind"`                          // 解析后的载荷类型 / Decoded payload kind
	DynamicBone        *DynamicBoneStatus   `json:"dynamicBoneStatus,omitempty"`   // 动态骨骼配置载荷 / DynamicBone configuration payload
	ColliderPackage    *ColliderPackage     `json:"colliderPackage,omitempty"`     // 通用碰撞体包载荷 / Generic collider package payload
	LimbCollider       *LimbColliderPackage `json:"limbColliderPackage,omitempty"` // LimbColliderMgr 保存的碰撞体包 / Collider package saved by LimbColliderMgr
	IKCollider         *IKColliderPackage   `json:"ikColliderPackage,omitempty"`   // IKColliderSaveLoader 保存的碰撞体包 / Collider package saved by IKColliderSaveLoader
	ClothParams        *ClothParams         `json:"clothParams,omitempty"`         // MagicaCloth.ClothParams 载荷 / MagicaCloth.ClothParams payload
	Text               string               `json:"text,omitempty"`                // 字符串载荷原文 / Original text payload
	JSON               json.RawMessage      `json:"json,omitempty"`                // 当字符串载荷是 JSON 时的压缩 JSON / Compacted JSON when the text payload contains JSON
	MsgpackBase64      string               `json:"msgpackBase64,omitempty"`       // 未识别 MessagePack 载荷的 base64 数据 / Base64 data for unrecognized MessagePack payloads
	MsgpackJSONPreview json.RawMessage      `json:"msgpackJsonPreview,omitempty"`  // 未识别载荷的 JSON 预览 / JSON preview for unrecognized payloads
}

func (e *KCESPayloadEnvelope) UnmarshalJSON(data []byte) error {
	type envelopeAlias KCESPayloadEnvelope
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var alias envelopeAlias
	if err := dec.Decode(&alias); err != nil {
		return err
	}
	*e = KCESPayloadEnvelope(alias)
	return nil
}

// DynamicBoneStatus 对应 KCES DynamicBoneStatus / DynamicBoneStatus corresponds to KCES DynamicBoneStatus
// 游戏以 MessagePack indexed-array 写入，version 在 Key(0)，字段在 Key(1)..Key(15) / The game writes MessagePack indexed-array data with version at Key(0) and fields at Key(1)..Key(15)
type DynamicBoneStatus struct {
	_struct             struct{}                    `codec:",toarray"`           // 强制按数组编码 / Forces array encoding
	Version             int                         `json:"version"`             // 版本号，通常为 1000 / Version value, usually 1000
	Damping             float32                     `json:"damping"`             // 阻尼值 / Damping value
	DampingKeyFrames    []DynamicBoneAnimationFrame `json:"dampingKeyFrames"`    // 阻尼动画关键帧 / Damping animation keyframes
	Elasticity          float32                     `json:"elasticity"`          // 弹性值 / Elasticity value
	ElasticityKeyFrames []DynamicBoneAnimationFrame `json:"elasticityKeyFrames"` // 弹性动画关键帧 / Elasticity animation keyframes
	Stiffness           float32                     `json:"stiffness"`           // 刚性值 / Stiffness value
	StiffnessKeyFrames  []DynamicBoneAnimationFrame `json:"stiffnessKeyFrames"`  // 刚性动画关键帧 / Stiffness animation keyframes
	Inert               float32                     `json:"inert"`               // 惯性值 / Inert value
	InertKeyFrames      []DynamicBoneAnimationFrame `json:"inertKeyFrames"`      // 惯性动画关键帧 / Inert animation keyframes
	Radius              float32                     `json:"radius"`              // 碰撞半径 / Collision radius
	RadiusKeyFrames     []DynamicBoneAnimationFrame `json:"radiusKeyFrames"`     // 半径动画关键帧 / Radius animation keyframes
	EndLength           float32                     `json:"endLength"`           // 末端长度 / End length
	EndOffset           Vector3                     `json:"endOffset"`           // 末端偏移 / End offset
	Gravity             Vector3                     `json:"gravity"`             // 重力向量 / Gravity vector
	Force               Vector3                     `json:"force"`               // 外力向量 / External force vector
	FreezeAxis          int                         `json:"freezeAxis"`          // 冻结轴枚举 / Freeze-axis enum
}

// DynamicBoneAnimationFrame 表示 DynamicBoneStatus 的动画关键帧 / DynamicBoneAnimationFrame represents one animation keyframe in DynamicBoneStatus
type DynamicBoneAnimationFrame struct {
	_struct    struct{} `codec:",toarray"`  // 强制按数组编码 / Forces array encoding
	Time       float32  `json:"time"`       // 关键帧时间 / Keyframe time
	Value      float32  `json:"value"`      // 关键帧值 / Keyframe value
	InTangent  float32  `json:"inTangent"`  // 入切线 / Incoming tangent
	OutTangent float32  `json:"outTangent"` // 出切线 / Outgoing tangent
}

func DecodeKCESPayload(data []byte, extension string) (*KCESPayloadEnvelope, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	payload, lengthPrefixed, err := StripLengthPrefix(data)
	if err != nil {
		return nil, err
	}

	decompressed, err := ct.DecompressLz4BlockArray(payload)
	if err != nil {
		return nil, fmt.Errorf("decompress %s payload: %w", ext, err)
	}

	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ext,
		LengthPrefixed: lengthPrefixed,
	}

	switch payloadKindForExtension(ext) {
	case PayloadKindDynamicBoneStatus:
		status := &DynamicBoneStatus{}
		if err := ct.DecodeMsgpack(decompressed, status); err != nil {
			return nil, fmt.Errorf("decode DynamicBoneStatus: %w", err)
		}
		env.Kind = PayloadKindDynamicBoneStatus
		env.DynamicBone = status
	case PayloadKindJSONString:
		var text string
		if err := ct.DecodeMsgpack(decompressed, &text); err != nil {
			return nil, fmt.Errorf("decode JSON string payload: %w", err)
		}
		env.Kind = PayloadKindJSONString
		env.Text = text
		if json.Valid([]byte(text)) {
			var compact bytes.Buffer
			if err := json.Compact(&compact, []byte(text)); err == nil {
				env.JSON = append(json.RawMessage(nil), compact.Bytes()...)
			}
		}
	case PayloadKindColliderPackage:
		var raw interface{}
		if err := ct.DecodeMsgpack(decompressed, &raw); err != nil {
			return nil, fmt.Errorf("decode ColliderPackage msgpack: %w", err)
		}
		pkg, err := decodeColliderPackageRaw(raw)
		if err != nil {
			return nil, fmt.Errorf("decode ColliderPackage: %w", err)
		}
		env.Kind = PayloadKindColliderPackage
		env.ColliderPackage = pkg
	case PayloadKindLimbCollider:
		var raw interface{}
		if err := ct.DecodeMsgpack(decompressed, &raw); err != nil {
			return nil, fmt.Errorf("decode LimbColliderPackage msgpack: %w", err)
		}
		pkg, err := decodeLimbColliderPackageRaw(raw)
		if err != nil {
			return nil, fmt.Errorf("decode LimbColliderPackage: %w", err)
		}
		env.Kind = PayloadKindLimbCollider
		env.LimbCollider = pkg
	case PayloadKindIKCollider:
		var raw interface{}
		if err := ct.DecodeMsgpack(decompressed, &raw); err != nil {
			return nil, fmt.Errorf("decode IKColliderPackage msgpack: %w", err)
		}
		pkg, err := decodeIKColliderPackageRaw(raw)
		if err != nil {
			return nil, fmt.Errorf("decode IKColliderPackage: %w", err)
		}
		env.Kind = PayloadKindIKCollider
		env.IKCollider = pkg
	case PayloadKindClothParams:
		params := &ClothParams{}
		if err := ct.DecodeMsgpack(decompressed, params); err != nil {
			return nil, fmt.Errorf("decode ClothParams: %w", err)
		}
		env.Kind = PayloadKindClothParams
		env.ClothParams = params
	default:
		env.Kind = PayloadKindRawMsgpack
		env.MsgpackBase64 = base64.StdEncoding.EncodeToString(decompressed)
		var raw interface{}
		if err := ct.DecodeMsgpack(decompressed, &raw); err == nil {
			if preview, err := json.Marshal(raw); err == nil {
				env.MsgpackJSONPreview = preview
			}
		}
	}

	return env, nil
}

func EncodeKCESPayload(env *KCESPayloadEnvelope) ([]byte, error) {
	if env == nil {
		return nil, fmt.Errorf("nil KCES payload envelope")
	}
	ext := NormalizeKCESPayloadExtension(env.Extension)
	kind := env.Kind
	if kind == "" {
		kind = payloadKindForExtension(ext)
	}

	var msgpackData []byte
	var err error
	switch kind {
	case PayloadKindDynamicBoneStatus:
		if env.DynamicBone == nil {
			return nil, fmt.Errorf("dynamicBoneStatus is required")
		}
		normalizeDynamicBoneStatus(env.DynamicBone)
		msgpackData, err = ct.EncodeMsgpack(env.DynamicBone)
	case PayloadKindJSONString:
		text := env.Text
		if len(env.JSON) > 0 && !bytes.Equal(bytes.TrimSpace(env.JSON), []byte("null")) {
			var compact bytes.Buffer
			if err := json.Compact(&compact, env.JSON); err != nil {
				return nil, fmt.Errorf("json payload is invalid: %w", err)
			}
			text = compact.String()
		}
		msgpackData, err = ct.EncodeMsgpack(text)
	case PayloadKindColliderPackage:
		if env.ColliderPackage == nil {
			return nil, fmt.Errorf("colliderPackage is required")
		}
		msgpackData, err = ct.EncodeMsgpack(env.ColliderPackage.toRaw())
	case PayloadKindLimbCollider:
		if env.LimbCollider == nil {
			return nil, fmt.Errorf("limbColliderPackage is required")
		}
		msgpackData, err = ct.EncodeMsgpack(env.LimbCollider.toRaw())
	case PayloadKindIKCollider:
		if env.IKCollider == nil {
			return nil, fmt.Errorf("ikColliderPackage is required")
		}
		msgpackData, err = ct.EncodeMsgpack(env.IKCollider.toRaw())
	case PayloadKindClothParams:
		if env.ClothParams == nil {
			return nil, fmt.Errorf("clothParams is required")
		}
		msgpackData, err = ct.EncodeMsgpack(env.ClothParams)
	case PayloadKindRawMsgpack:
		msgpackData, err = base64.StdEncoding.DecodeString(env.MsgpackBase64)
		if err != nil {
			return nil, fmt.Errorf("decode msgpackBase64: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported KCES payload kind %q", kind)
	}
	if err != nil {
		return nil, err
	}

	compressed, err := ct.CompressLz4BlockArray(msgpackData)
	if err != nil {
		return nil, fmt.Errorf("compress %s payload: %w", ext, err)
	}
	if env.LengthPrefixed || IsLengthPrefixedKCESPayloadExtension(ext) {
		return AddLengthPrefix(compressed), nil
	}
	return compressed, nil
}

func DecodeDynamicBoneStatusFile(data []byte) (*DynamicBoneStatus, error) {
	env, err := DecodeKCESPayload(data, ".dbconf")
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
		Extension:      ".dbconf",
		LengthPrefixed: true,
		Kind:           PayloadKindDynamicBoneStatus,
		DynamicBone:    status,
	}
	return EncodeKCESPayload(env)
}

func DecodeClothParamsFile(data []byte, extension string) (*ClothParams, error) {
	env, err := DecodeKCESPayload(data, extension)
	if err != nil {
		return nil, err
	}
	if env.ClothParams == nil {
		return nil, fmt.Errorf("payload is not ClothParams")
	}
	return env.ClothParams, nil
}

func EncodeClothParamsFile(params *ClothParams, extension string) ([]byte, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	if ext == "" {
		ext = ".dsbconf"
	}
	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ext,
		LengthPrefixed: true,
		Kind:           PayloadKindClothParams,
		ClothParams:    params,
	}
	return EncodeKCESPayload(env)
}

func StripLengthPrefix(data []byte) ([]byte, bool, error) {
	if len(data) < 4 {
		return data, false, nil
	}
	n := int(binary.LittleEndian.Uint32(data[:4]))
	if n == len(data)-4 {
		return data[4:], true, nil
	}
	return data, false, nil
}

func AddLengthPrefix(payload []byte) []byte {
	out := make([]byte, 4, len(payload)+4)
	binary.LittleEndian.PutUint32(out[:4], uint32(len(payload)))
	return append(out, payload...)
}

func IsLengthPrefixedKCESPayloadExtension(extension string) bool {
	_, ok := lengthPrefixedPayloadExts[NormalizeKCESPayloadExtension(extension)]
	return ok
}

func IsKCESPayloadExtension(extension string) bool {
	return NormalizeKCESPayloadExtension(extension) != ""
}

func NormalizeKCESPayloadExtension(pathOrExt string) string {
	lower := strings.ToLower(strings.TrimSpace(filepath.ToSlash(pathOrExt)))
	if lower == "" {
		return ""
	}
	if strings.HasSuffix(lower, ".ikcol.bytes") {
		return ".ikcol.bytes"
	}
	ext := filepath.Ext(lower)
	switch ext {
	case ".dbconf", ".dbcol", ".db2conf", ".dsbconf", ".dsb2conf", ".dslconf", ".dsl2conf", ".dslcol", ".ikcol", ".limbcol":
		return ext
	default:
		return ""
	}
}

func payloadKindForExtension(ext string) string {
	switch NormalizeKCESPayloadExtension(ext) {
	case ".dbconf":
		return PayloadKindDynamicBoneStatus
	case ".db2conf", ".dsb2conf", ".dsl2conf":
		return PayloadKindJSONString
	case ".dsbconf", ".dslconf":
		return PayloadKindClothParams
	case ".dbcol", ".dslcol":
		return PayloadKindColliderPackage
	case ".limbcol":
		return PayloadKindLimbCollider
	case ".ikcol", ".ikcol.bytes":
		return PayloadKindIKCollider
	default:
		return PayloadKindRawMsgpack
	}
}

func normalizeDynamicBoneStatus(status *DynamicBoneStatus) {
	if status.Version == 0 {
		status.Version = 1000
	}
	if status.DampingKeyFrames == nil {
		status.DampingKeyFrames = []DynamicBoneAnimationFrame{}
	}
	if status.ElasticityKeyFrames == nil {
		status.ElasticityKeyFrames = []DynamicBoneAnimationFrame{}
	}
	if status.StiffnessKeyFrames == nil {
		status.StiffnessKeyFrames = []DynamicBoneAnimationFrame{}
	}
	if status.InertKeyFrames == nil {
		status.InertKeyFrames = []DynamicBoneAnimationFrame{}
	}
	if status.RadiusKeyFrames == nil {
		status.RadiusKeyFrames = []DynamicBoneAnimationFrame{}
	}
}
