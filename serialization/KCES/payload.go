package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

const (
	PayloadFormatKCESMessagePack = "kces-msgpack-lz4"
	PayloadFormatKCESExportCM    = "kces-exportcm-sidecar"

	// StorageVariant is intentionally independent of Extension. ExportCM emits
	// COM3D2-compatible JSON sidecars with extensions that KCES also uses for
	// its own length-prefixed LZ4 MessagePack resources.
	PayloadStorageInt32LZ4MessagePack      = "int32-length-lz4-messagepack"
	PayloadStorageExportCMUnityJSON        = "exportcm-unity-json"
	PayloadStorageExportCMDotNetStringJSON = "exportcm-dotnet-string-unity-json"

	PayloadKindDynamicBoneStatus       = "dynamic-bone-status"
	PayloadKindJSONString              = "msgpack-json-string"
	PayloadKindRawMsgpack              = "raw-msgpack"
	PayloadKindColliderPackage         = "collider-package"
	PayloadKindLimbCollider            = "limb-collider-package"
	PayloadKindIKCollider              = "ik-collider-package"
	PayloadKindClothParams             = "cloth-params"
	PayloadKindExportCMDynamicBoneJSON = "exportcm-dynamic-bone-json"
	PayloadKindExportCMColliderJSON    = "exportcm-collider-json"
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

// KCESPayloadEnvelope is a JSON-editable envelope for both native KCES
// MessagePack resources and the COM3D2-compatible JSON sidecars written by
// KCES ExportCM. StorageVariant is authoritative because several extensions
// are shared by those incompatible wire formats.
type KCESPayloadEnvelope struct {
	Format              string               `json:"format"`                        // Envelope family: kces-msgpack-lz4 or kces-exportcm-sidecar
	Extension           string               `json:"extension"`                     // 原始文件扩展名，用于判定载荷类型 / Original file extension used to determine payload kind
	LengthPrefixed      bool                 `json:"lengthPrefixed"`                // 是否带 4 字节长度前缀 / Whether a 4-byte length prefix is present
	StorageVariant      string               `json:"storageVariant"`                // 实际 wire 形态；同一扩展名可能有 KCES 与 ExportCM 两种 wire / Exact wire variant; an extension can have both KCES and ExportCM forms
	Kind                string               `json:"kind"`                          // 解析后的载荷类型 / Decoded payload kind
	DynamicBone         *DynamicBoneStatus   `json:"dynamicBoneStatus,omitempty"`   // 动态骨骼配置载荷 / DynamicBone configuration payload
	ColliderPackage     *ColliderPackage     `json:"colliderPackage,omitempty"`     // 通用碰撞体包载荷 / Generic collider package payload
	LimbCollider        *LimbColliderPackage `json:"limbColliderPackage,omitempty"` // LimbColliderMgr 保存的碰撞体包 / Collider package saved by LimbColliderMgr
	IKCollider          *IKColliderPackage   `json:"ikColliderPackage,omitempty"`   // IKColliderSaveLoader 保存的碰撞体包 / Collider package saved by IKColliderSaveLoader
	ClothParams         *ClothParams         `json:"clothParams,omitempty"`         // MagicaCloth.ClothParams 载荷 / MagicaCloth.ClothParams payload
	Text                string               `json:"text,omitempty"`                // 字符串载荷原文 / Original text payload
	JSON                json.RawMessage      `json:"json,omitempty"`                // 当字符串载荷是 JSON 时的压缩 JSON / Compacted JSON when the text payload contains JSON
	MsgpackBase64       string               `json:"msgpackBase64,omitempty"`       // 未识别 MessagePack 载荷的 base64 数据 / Base64 data for unrecognized MessagePack payloads
	MsgpackJSONPreview  json.RawMessage      `json:"msgpackJsonPreview,omitempty"`  // 未识别载荷的 JSON 预览 / JSON preview for unrecognized payloads
	MsgpackRootNil      bool                 `json:"msgpackRootNil,omitempty"`      // 已识别根值是否为 MessagePack nil / Whether a recognized root value was MessagePack nil
	MsgpackTrailingData []byte               `json:"msgpackTrailingData,omitempty"` // 已识别根值之后 MessagePack-CSharp 未读取的原始字节 / Raw bytes left unread after a recognized root value
}

func (e *KCESPayloadEnvelope) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("KCES payload JSON is not valid UTF-8")
	}
	type envelopeAlias KCESPayloadEnvelope
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	var alias envelopeAlias
	if err := dec.Decode(&alias); err != nil {
		return err
	}
	var trailing interface{}
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("trailing content: %w", err)
	}
	*e = KCESPayloadEnvelope(alias)
	return nil
}

// DynamicBoneStatus 对应 KCES DynamicBoneStatus / DynamicBoneStatus corresponds to KCES DynamicBoneStatus
// 游戏以 MessagePack indexed-array 写入，version 在 Key(0)，字段在 Key(1)..Key(15) / The game writes MessagePack indexed-array data with version at Key(0) and fields at Key(1)..Key(15)
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

// NewDynamicBoneStatus returns the current-game defaults for callers creating
// a new object explicitly. Decoders do not invoke this constructor or inject
// these values into an existing/short wire object.
func NewDynamicBoneStatus() *DynamicBoneStatus {
	return &DynamicBoneStatus{
		Version:    1000,
		Damping:    0.6,
		Elasticity: 0.1,
		Stiffness:  0.1,
		Gravity:    Vector3{Y: -0.05},
	}
}

// UnmarshalJSON decodes only fields present in the editing document. Game
// constructor defaults remain available through NewDynamicBoneStatus and are
// never injected while deserializing existing data.
func (s *DynamicBoneStatus) UnmarshalJSON(data []byte) error {
	type dynamicBoneStatusJSON DynamicBoneStatus
	var value dynamicBoneStatusJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = DynamicBoneStatus(value)
	return nil
}

// DynamicBoneAnimationFrame 表示 DynamicBoneStatus 的动画关键帧 / DynamicBoneAnimationFrame represents one animation keyframe in DynamicBoneStatus
type DynamicBoneAnimationFrame struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Time                   float32 `json:"time"`       // 关键帧时间 / Keyframe time
	Value                  float32 `json:"value"`      // 关键帧值 / Keyframe value
	InTangent              float32 `json:"inTangent"`  // 入切线 / Incoming tangent
	OutTangent             float32 `json:"outTangent"` // 出切线 / Outgoing tangent
}

func DecodeKCESPayload(data []byte, extension string) (*KCESPayloadEnvelope, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	messagePackEnvelope, messagePackErr := decodeKCESMessagePackPayload(data, ext)
	if !isExportCMPayloadExtension(ext) {
		return messagePackEnvelope, messagePackErr
	}

	exportEnvelope, exportErr := decodeExportCMPayload(data, ext)
	if messagePackErr == nil && exportErr == nil {
		return nil, fmt.Errorf("%s payload is ambiguous: both the KCES int32+LZ4 MessagePack and ExportCM JSON decoders accepted it", ext)
	}
	if messagePackErr == nil {
		return messagePackEnvelope, nil
	}
	if exportErr == nil {
		return exportEnvelope, nil
	}
	return nil, fmt.Errorf("decode %s payload as KCES MessagePack: %v; decode as ExportCM JSON: %w", ext, messagePackErr, exportErr)
}

func decodeKCESMessagePackPayload(data []byte, extension string) (*KCESPayloadEnvelope, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	payload, lengthPrefixed, err := StripLengthPrefix(data)
	if err != nil {
		return nil, err
	}
	// Every currently supported game payload extension is written with
	// BinaryWriter.Write(byte[].Length) before the Lz4BlockArray bytes. Treating
	// a mismatched first int32 as an optional prefix makes a payload which this
	// library accepts but the game cannot load.
	if IsLengthPrefixedKCESPayloadExtension(ext) && !lengthPrefixed {
		if len(data) < 4 {
			return nil, fmt.Errorf("%s payload is missing its required int32 length prefix: file is only %d bytes", ext, len(data))
		}
		declared := binary.LittleEndian.Uint32(data[:4])
		return nil, fmt.Errorf("%s payload has invalid int32 length prefix: declared %d bytes, actual %d", ext, declared, len(data)-4)
	}

	decompressed, err := ct.DecompressLz4BlockArray(payload)
	if err != nil {
		return nil, fmt.Errorf("decompress %s payload: %w", ext, err)
	}

	env := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      ext,
		LengthPrefixed: lengthPrefixed,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
	}
	kind := payloadKindForExtension(ext)
	var root, trailing []byte
	if kind != PayloadKindRawMsgpack {
		root, trailing, err = ct.SplitFirstMsgpackValue(decompressed)
		if err != nil {
			return nil, fmt.Errorf("split %s payload root MessagePack value: %w", ext, err)
		}
		if len(root) == 1 && root[0] == 0xc0 {
			env.Kind = kind
			env.MsgpackRootNil = true
			env.MsgpackTrailingData = append([]byte(nil), trailing...)
			return env, nil
		}
	}
	decodeRoot := func(out interface{}) error {
		return ct.DecodeMsgpack(root, out)
	}

	switch kind {
	case PayloadKindDynamicBoneStatus:
		status := &DynamicBoneStatus{}
		if err := decodeRoot(status); err != nil {
			return nil, fmt.Errorf("decode DynamicBoneStatus: %w", err)
		}
		if err := validateDynamicBoneStatusForEncoding(status); err != nil {
			return nil, fmt.Errorf("validate decoded DynamicBoneStatus: %w", err)
		}
		env.Kind = PayloadKindDynamicBoneStatus
		env.DynamicBone = status
	case PayloadKindJSONString:
		var text string
		if err := decodeRoot(&text); err != nil {
			return nil, fmt.Errorf("decode JSON string payload: %w", err)
		}
		if !json.Valid([]byte(text)) {
			return nil, fmt.Errorf("decode JSON string payload: inner Magica JSON is invalid")
		}
		env.Kind = PayloadKindJSONString
		env.Text = text
		var compact bytes.Buffer
		if err := json.Compact(&compact, []byte(text)); err != nil {
			return nil, fmt.Errorf("compact inner Magica JSON: %w", err)
		}
		env.JSON = append(json.RawMessage(nil), compact.Bytes()...)
	case PayloadKindColliderPackage:
		pkg := &ColliderPackage{}
		if err := decodeRoot(pkg); err != nil {
			return nil, fmt.Errorf("decode ColliderPackage msgpack: %w", err)
		}
		if err := validateColliderPackageForEncoding(pkg); err != nil {
			return nil, fmt.Errorf("validate decoded ColliderPackage: %w", err)
		}
		env.Kind = PayloadKindColliderPackage
		env.ColliderPackage = pkg
	case PayloadKindLimbCollider:
		pkg := &LimbColliderPackage{}
		if err := decodeRoot(pkg); err != nil {
			return nil, fmt.Errorf("decode LimbColliderPackage msgpack: %w", err)
		}
		if err := validateLimbColliderPackageForEncoding(pkg); err != nil {
			return nil, fmt.Errorf("validate decoded LimbColliderPackage: %w", err)
		}
		env.Kind = PayloadKindLimbCollider
		env.LimbCollider = pkg
	case PayloadKindIKCollider:
		pkg := &IKColliderPackage{}
		if err := decodeRoot(pkg); err != nil {
			return nil, fmt.Errorf("decode IKColliderPackage msgpack: %w", err)
		}
		if err := validateIKColliderPackageForEncoding(pkg); err != nil {
			return nil, fmt.Errorf("validate decoded IKColliderPackage: %w", err)
		}
		env.Kind = PayloadKindIKCollider
		env.IKCollider = pkg
	case PayloadKindClothParams:
		params := &ClothParams{}
		if err := decodeRoot(params); err != nil {
			return nil, fmt.Errorf("decode ClothParams: %w", err)
		}
		if err := validateClothParamsForEncoding(params); err != nil {
			return nil, fmt.Errorf("validate decoded ClothParams: %w", err)
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
	if env.Kind != PayloadKindRawMsgpack {
		env.MsgpackTrailingData = append([]byte(nil), trailing...)
	}

	return env, nil
}

func EncodeKCESPayload(env *KCESPayloadEnvelope) ([]byte, error) {
	if env == nil {
		return nil, fmt.Errorf("nil KCES payload envelope")
	}
	storageVariant := env.StorageVariant
	if storageVariant == "" {
		if env.Format == PayloadFormatKCESExportCM {
			return nil, fmt.Errorf("KCES ExportCM payload storageVariant is required")
		}
		// Editing JSON emitted before storageVariant was introduced always
		// represented the original int32+LZ4 MessagePack wire.
		storageVariant = PayloadStorageInt32LZ4MessagePack
	}

	switch storageVariant {
	case PayloadStorageInt32LZ4MessagePack:
		if env.Format != "" && env.Format != PayloadFormatKCESMessagePack {
			return nil, fmt.Errorf("payload format %q is incompatible with storageVariant %q", env.Format, storageVariant)
		}
		return encodeKCESMessagePackPayload(env)
	case PayloadStorageExportCMUnityJSON, PayloadStorageExportCMDotNetStringJSON:
		if env.Format != PayloadFormatKCESExportCM {
			return nil, fmt.Errorf("payload format %q is incompatible with storageVariant %q", env.Format, storageVariant)
		}
		if len(env.MsgpackTrailingData) != 0 {
			return nil, fmt.Errorf("msgpackTrailingData cannot be represented by ExportCM storageVariant %q", storageVariant)
		}
		if env.MsgpackRootNil {
			return nil, fmt.Errorf("msgpackRootNil cannot be represented by ExportCM storageVariant %q", storageVariant)
		}
		return encodeExportCMPayload(env, storageVariant)
	default:
		return nil, fmt.Errorf("unsupported KCES payload storageVariant %q", storageVariant)
	}
}

func encodeKCESMessagePackPayload(env *KCESPayloadEnvelope) ([]byte, error) {
	ext := NormalizeKCESPayloadExtension(env.Extension)
	kind := env.Kind
	if kind == "" {
		kind = payloadKindForExtension(ext)
	}

	var msgpackData []byte
	var err error
	if env.MsgpackRootNil {
		if kind == PayloadKindRawMsgpack {
			return nil, fmt.Errorf("raw-msgpack already stores the complete decompressed stream in msgpackBase64; msgpackRootNil must be false")
		}
		if payloadEnvelopeHasTypedRoot(env) {
			return nil, fmt.Errorf("msgpackRootNil would discard populated payload fields")
		}
		msgpackData = []byte{0xc0}
	} else {
		switch kind {
		case PayloadKindDynamicBoneStatus:
			if env.DynamicBone == nil {
				return nil, fmt.Errorf("dynamicBoneStatus is required")
			}
			if err := validateDynamicBoneStatusForEncoding(env.DynamicBone); err != nil {
				return nil, err
			}
			normalized := normalizeDynamicBoneStatusForEncoding(env.DynamicBone)
			msgpackData, err = ct.EncodeIndexedMsgpack(normalized)
		case PayloadKindJSONString:
			text, selectErr := editableMessagePackJSONString(env)
			if selectErr != nil {
				return nil, selectErr
			}
			msgpackData, err = ct.EncodeMsgpack(text)
		case PayloadKindColliderPackage:
			if env.ColliderPackage == nil {
				return nil, fmt.Errorf("colliderPackage is required")
			}
			if err := validateColliderPackageForEncoding(env.ColliderPackage); err != nil {
				return nil, err
			}
			normalized := normalizeColliderPackageForEncoding(env.ColliderPackage)
			msgpackData, err = ct.EncodeIndexedMsgpack(normalized)
		case PayloadKindLimbCollider:
			if env.LimbCollider == nil {
				return nil, fmt.Errorf("limbColliderPackage is required")
			}
			if err := validateLimbColliderPackageForEncoding(env.LimbCollider); err != nil {
				return nil, err
			}
			normalized := normalizeLimbColliderPackageForEncoding(env.LimbCollider)
			msgpackData, err = ct.EncodeIndexedMsgpack(normalized)
		case PayloadKindIKCollider:
			if env.IKCollider == nil {
				return nil, fmt.Errorf("ikColliderPackage is required")
			}
			if err := validateIKColliderPackageForEncoding(env.IKCollider); err != nil {
				return nil, err
			}
			normalized := normalizeIKColliderPackageForEncoding(env.IKCollider)
			msgpackData, err = ct.EncodeIndexedMsgpack(normalized)
		case PayloadKindClothParams:
			if env.ClothParams == nil {
				return nil, fmt.Errorf("clothParams is required")
			}
			if err := validateClothParamsForEncoding(env.ClothParams); err != nil {
				return nil, err
			}
			msgpackData, err = ct.EncodeIndexedMsgpack(env.ClothParams)
		case PayloadKindRawMsgpack:
			if len(env.MsgpackTrailingData) != 0 {
				return nil, fmt.Errorf("raw-msgpack already stores the complete decompressed stream in msgpackBase64; msgpackTrailingData must be empty")
			}
			msgpackData, err = base64.StdEncoding.DecodeString(env.MsgpackBase64)
			if err != nil {
				return nil, fmt.Errorf("decode msgpackBase64: %w", err)
			}
		default:
			return nil, fmt.Errorf("unsupported KCES payload kind %q", kind)
		}
	}
	if err != nil {
		return nil, err
	}
	if kind != PayloadKindRawMsgpack {
		msgpackData = append(msgpackData, env.MsgpackTrailingData...)
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

func payloadEnvelopeHasTypedRoot(env *KCESPayloadEnvelope) bool {
	return env.DynamicBone != nil ||
		env.ColliderPackage != nil ||
		env.LimbCollider != nil ||
		env.IKCollider != nil ||
		env.ClothParams != nil ||
		env.Text != "" ||
		len(env.JSON) != 0 ||
		env.MsgpackBase64 != "" ||
		len(env.MsgpackJSONPreview) != 0
}

// editableMessagePackJSONString selects the exact string to store in the
// MessagePack payload. Text is the original wire string and JSON is its
// editable parsed view. An unchanged JSON view keeps Text byte-for-byte,
// including insignificant whitespace; an actual edit is emitted as compact
// JSON. No game migration or JsonUtility callback is run.
func editableMessagePackJSONString(env *KCESPayloadEnvelope) (string, error) {
	if len(env.JSON) != 0 {
		if !utf8.Valid(env.JSON) {
			return "", fmt.Errorf("json payload is not valid UTF-8")
		}
		var compactJSON bytes.Buffer
		if err := json.Compact(&compactJSON, env.JSON); err != nil {
			return "", fmt.Errorf("json payload is invalid: %w", err)
		}

		if env.Text != "" && utf8.ValidString(env.Text) {
			var compactText bytes.Buffer
			if err := json.Compact(&compactText, []byte(env.Text)); err == nil && bytes.Equal(compactText.Bytes(), compactJSON.Bytes()) {
				return env.Text, nil
			}
		}
		return compactJSON.String(), nil
	}

	if !utf8.ValidString(env.Text) {
		return "", fmt.Errorf("inner Magica JSON payload is not valid UTF-8")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(env.Text)); err != nil {
		return "", fmt.Errorf("inner Magica JSON payload is invalid: %w", err)
	}
	return env.Text, nil
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
