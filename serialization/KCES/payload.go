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

// KCES 物理与碰撞扩展名的统一封套和调度层；各扩展名的 wire 描述由对应文件声明。
//
// Unified envelope and dispatcher for KCES physics and collider extensions; each extension declares its wire descriptor in its own file.

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

type kcesPayloadDescriptor struct {
	Extension              string
	Kind                   string
	LengthPrefixed         bool
	ExportCMKind           string
	ExportCMStorageVariant string
}

// KCESPayloadDescriptor is the read-only wire contract for one supported
// payload extension. It is exported so schema and transport layers can use
// the same descriptor table as the native encoder without copying it.
type KCESPayloadDescriptor struct {
	Extension              string
	Kind                   string
	LengthPrefixed         bool
	ExportCMKind           string
	ExportCMStorageVariant string
}

var kcesPayloadDescriptors = [...]kcesPayloadDescriptor{
	dbconfPayloadDescriptor,
	dbcolPayloadDescriptor,
	db2confPayloadDescriptor,
	dsbconfPayloadDescriptor,
	dsb2confPayloadDescriptor,
	dslconfPayloadDescriptor,
	dsl2confPayloadDescriptor,
	dslcolPayloadDescriptor,
	ikcolPayloadDescriptor,
	ikcolBytesPayloadDescriptor,
	limbcolPayloadDescriptor,
}

var kcesPayloadDescriptorByExtension = func() map[string]kcesPayloadDescriptor {
	result := make(map[string]kcesPayloadDescriptor, len(kcesPayloadDescriptors))
	for _, descriptor := range kcesPayloadDescriptors {
		if descriptor.Extension == "" {
			panic("KCES payload descriptor has an empty extension")
		}
		if _, exists := result[descriptor.Extension]; exists {
			panic("duplicate KCES payload descriptor for " + descriptor.Extension)
		}
		result[descriptor.Extension] = descriptor
	}
	return result
}()

// DescribeKCESPayload returns the native/ExportCM wire descriptor for an
// extension. The returned value is a copy and cannot mutate the registry.
func DescribeKCESPayload(extension string) (KCESPayloadDescriptor, bool) {
	descriptor, ok := kcesPayloadDescriptorByExtension[NormalizeKCESPayloadExtension(extension)]
	if !ok {
		return KCESPayloadDescriptor{}, false
	}
	return KCESPayloadDescriptor{
		Extension: descriptor.Extension, Kind: descriptor.Kind,
		LengthPrefixed: descriptor.LengthPrefixed,
		ExportCMKind:   descriptor.ExportCMKind, ExportCMStorageVariant: descriptor.ExportCMStorageVariant,
	}, true
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
	ext := NormalizeKCESPayloadExtension(env.Extension)
	descriptor, ok := kcesPayloadDescriptorByExtension[ext]
	if !ok {
		if env.Extension == "" && env.Format == PayloadFormatKCESMessagePack && env.StorageVariant == PayloadStorageInt32LZ4MessagePack && env.Kind == PayloadKindRawMsgpack {
			if len(env.MsgpackTrailingData) != 0 {
				return nil, fmt.Errorf("raw-msgpack already stores the complete decompressed stream in msgpackBase64; msgpackTrailingData must be empty")
			}
			if env.MsgpackRootNil || payloadEnvelopeHasTypedRootExceptRaw(env) {
				return nil, fmt.Errorf("raw-msgpack envelope has incompatible typed or trailing fields")
			}
			if _, err := base64.StdEncoding.DecodeString(env.MsgpackBase64); err != nil {
				return nil, fmt.Errorf("decode msgpackBase64: %w", err)
			}
			return encodeKCESMessagePackPayload(env)
		}
		return nil, fmt.Errorf("unsupported or non-canonical KCES payload extension %q", env.Extension)
	}
	if env.Extension != ext {
		return nil, fmt.Errorf("unsupported or non-canonical KCES payload extension %q", env.Extension)
	}
	if env.Format == "" {
		return nil, fmt.Errorf("KCES payload format is required")
	}
	if env.StorageVariant == "" {
		return nil, fmt.Errorf("KCES payload storageVariant is required")
	}
	if env.Kind == "" {
		return nil, fmt.Errorf("KCES payload kind is required")
	}
	if err := validateKCESPayloadEnvelopeRoots(env, descriptor); err != nil {
		return nil, err
	}

	switch env.StorageVariant {
	case PayloadStorageInt32LZ4MessagePack:
		if env.Format != PayloadFormatKCESMessagePack {
			return nil, fmt.Errorf("payload format %q is incompatible with storageVariant %q", env.Format, env.StorageVariant)
		}
		if !descriptor.LengthPrefixed || !env.LengthPrefixed {
			return nil, fmt.Errorf("extension %q requires lengthPrefixed=true for storageVariant %q", ext, env.StorageVariant)
		}
		return encodeKCESMessagePackPayload(env)
	case PayloadStorageExportCMUnityJSON, PayloadStorageExportCMDotNetStringJSON:
		if env.Format != PayloadFormatKCESExportCM {
			return nil, fmt.Errorf("payload format %q is incompatible with storageVariant %q", env.Format, env.StorageVariant)
		}
		if env.LengthPrefixed {
			return nil, fmt.Errorf("ExportCM storageVariant %q requires lengthPrefixed=false", env.StorageVariant)
		}
		return encodeExportCMPayload(env, env.StorageVariant)
	default:
		return nil, fmt.Errorf("unsupported KCES payload storageVariant %q", env.StorageVariant)
	}
}

func payloadEnvelopeHasTypedRootExceptRaw(env *KCESPayloadEnvelope) bool {
	return env.DynamicBone != nil || env.ColliderPackage != nil || env.LimbCollider != nil || env.IKCollider != nil || env.ClothParams != nil || env.Text != "" || len(env.JSON) != 0
}

func validateKCESPayloadEnvelopeRoots(env *KCESPayloadEnvelope, descriptor kcesPayloadDescriptor) error {
	if env.StorageVariant == PayloadStorageInt32LZ4MessagePack {
		if env.Kind != descriptor.Kind {
			return fmt.Errorf("extension %q with storageVariant %q requires kind %q, got %q", descriptor.Extension, env.StorageVariant, descriptor.Kind, env.Kind)
		}
		if env.Format != PayloadFormatKCESMessagePack || env.LengthPrefixed != descriptor.LengthPrefixed {
			return fmt.Errorf("extension %q requires native tuple format=%q, storageVariant=%q, kind=%q, lengthPrefixed=%v", descriptor.Extension, PayloadFormatKCESMessagePack, PayloadStorageInt32LZ4MessagePack, descriptor.Kind, descriptor.LengthPrefixed)
		}
		if env.MsgpackRootNil {
			if payloadEnvelopeHasTypedRoot(env) {
				return fmt.Errorf("msgpackRootNil cannot be combined with a populated payload root")
			}
			return nil
		}
		if len(env.MsgpackJSONPreview) != 0 {
			return fmt.Errorf("msgpackJsonPreview is only valid for raw-msgpack payloads")
		}
		activePresent := false
		switch descriptor.Kind {
		case PayloadKindDynamicBoneStatus:
			activePresent = env.DynamicBone != nil
		case PayloadKindColliderPackage:
			activePresent = env.ColliderPackage != nil
		case PayloadKindLimbCollider:
			activePresent = env.LimbCollider != nil
		case PayloadKindIKCollider:
			activePresent = env.IKCollider != nil
		case PayloadKindClothParams:
			activePresent = env.ClothParams != nil
		case PayloadKindJSONString:
			if env.Text == "" && len(env.JSON) == 0 {
				return fmt.Errorf("string payload requires text or json")
			}
		default:
			return fmt.Errorf("unsupported native payload kind %q for extension %q", descriptor.Kind, descriptor.Extension)
		}
		if !activePresent && descriptor.Kind != PayloadKindJSONString {
			return fmt.Errorf("payload kind %q requires its typed root", descriptor.Kind)
		}
		if env.Kind != PayloadKindJSONString && (env.Text != "" || len(env.JSON) != 0) {
			return fmt.Errorf("text/json fields are inactive for payload kind %q", env.Kind)
		}
		if env.MsgpackBase64 != "" {
			return fmt.Errorf("msgpackBase64 is inactive for payload kind %q", env.Kind)
		}
		for name, present := range map[string]bool{
			"dynamicBoneStatus": env.DynamicBone != nil, "colliderPackage": env.ColliderPackage != nil,
			"limbColliderPackage": env.LimbCollider != nil, "ikColliderPackage": env.IKCollider != nil,
			"clothParams": env.ClothParams != nil,
		} {
			if present && name != nativeRootFieldName(descriptor.Kind) {
				return fmt.Errorf("%s is inactive for payload kind %q", name, descriptor.Kind)
			}
		}
		return nil
	}
	if descriptor.ExportCMKind == "" || env.Format != PayloadFormatKCESExportCM || env.Kind != descriptor.ExportCMKind || env.StorageVariant != descriptor.ExportCMStorageVariant || env.LengthPrefixed {
		return fmt.Errorf("extension %q has an inconsistent ExportCM payload tuple", descriptor.Extension)
	}
	if env.MsgpackRootNil || len(env.MsgpackTrailingData) != 0 || env.MsgpackBase64 != "" || len(env.MsgpackJSONPreview) != 0 || env.DynamicBone != nil || env.ColliderPackage != nil || env.LimbCollider != nil || env.IKCollider != nil || env.ClothParams != nil {
		return fmt.Errorf("ExportCM payload has inactive native MessagePack fields")
	}
	if env.Text == "" && len(env.JSON) == 0 {
		return fmt.Errorf("ExportCM payload requires text or json")
	}
	return nil
}

func nativeRootFieldName(kind string) string {
	switch kind {
	case PayloadKindDynamicBoneStatus:
		return "dynamicBoneStatus"
	case PayloadKindColliderPackage:
		return "colliderPackage"
	case PayloadKindLimbCollider:
		return "limbColliderPackage"
	case PayloadKindIKCollider:
		return "ikColliderPackage"
	case PayloadKindClothParams:
		return "clothParams"
	default:
		return ""
	}
}

func encodeKCESMessagePackPayload(env *KCESPayloadEnvelope) ([]byte, error) {
	ext := NormalizeKCESPayloadExtension(env.Extension)
	kind := env.Kind
	if kind == "" {
		kind = payloadKindForExtension(ext)
	}
	if descriptor, ok := kcesPayloadDescriptorByExtension[ext]; ok && kind != descriptor.Kind {
		return nil, fmt.Errorf("extension %q with storageVariant %q requires kind %q, got %q", ext, PayloadStorageInt32LZ4MessagePack, descriptor.Kind, kind)
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

// editableMessagePackJSONString 将 JSON 语义内容编码为 MessagePack 字符串。
// editableMessagePackJSONString encodes the semantic JSON content as a MessagePack string.
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

func StripLengthPrefix(data []byte) ([]byte, bool, error) {
	if len(data) < 4 {
		return data, false, nil
	}
	n := int64(binary.LittleEndian.Uint32(data[:4]))
	if n == int64(len(data)-4) {
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
	descriptor, ok := kcesPayloadDescriptorByExtension[NormalizeKCESPayloadExtension(extension)]
	return ok && descriptor.LengthPrefixed
}

func IsKCESPayloadExtension(extension string) bool {
	return NormalizeKCESPayloadExtension(extension) != ""
}

func NormalizeKCESPayloadExtension(pathOrExt string) string {
	lower := strings.ToLower(strings.TrimSpace(filepath.ToSlash(pathOrExt)))
	if lower == "" {
		return ""
	}
	if strings.HasSuffix(lower, ikcolBytesPayloadDescriptor.Extension) {
		return ikcolBytesPayloadDescriptor.Extension
	}
	ext := filepath.Ext(lower)
	if _, ok := kcesPayloadDescriptorByExtension[ext]; ok {
		return ext
	}
	return ""
}

func payloadKindForExtension(ext string) string {
	if descriptor, ok := kcesPayloadDescriptorByExtension[NormalizeKCESPayloadExtension(ext)]; ok {
		return descriptor.Kind
	}
	return PayloadKindRawMsgpack
}
