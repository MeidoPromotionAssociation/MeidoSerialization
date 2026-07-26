package KCES

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/strictjson"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
)

// KCES 物理与碰撞扩展名的统一封套和调度层，各扩展名的线格式描述由对应文件声明
// Unified envelope and dispatcher for KCES physics and collider extensions, with each extension declaring its wire descriptor in its own file

const (
	PayloadFormatKCESMessagePack = "kces-msgpack-lz4"
	PayloadFormatKCESExportCM    = "kces-exportcm-sidecar"

	// StorageVariant 有意独立于 Extension，因为 ExportCM 写出的 COM3D2 兼容 JSON 旁车会复用 KCES 自有长度前缀 LZ4 MessagePack 资源的扩展名
	// StorageVariant is intentionally independent of Extension because ExportCM emits COM3D2-compatible JSON sidecars with extensions also used by KCES for its own length-prefixed LZ4 MessagePack resources
	PayloadStorageInt32LZ4MessagePack      = "int32-length-lz4-messagepack"
	PayloadStorageExportCMUnityJSON        = "exportcm-unity-json"
	PayloadStorageExportCMDotNetStringJSON = "exportcm-dotnet-string-unity-json"

	PayloadKindDynamicBoneStatus       = "dynamic-bone-status"
	PayloadKindJSONString              = "msgpack-json-string"
	PayloadKindColliderPackage         = "collider-package"
	PayloadKindLimbCollider            = "limb-collider-package"
	PayloadKindIKCollider              = "ik-collider-package"
	PayloadKindClothParams             = "cloth-params"
	PayloadKindExportCMDynamicBoneJSON = "exportcm-dynamic-bone-json"
	PayloadKindExportCMColliderJSON    = "exportcm-collider-json"
)

// kcesPayloadDescriptor 描述一个 KCES 扩展名支持的原生与 ExportCM 载荷线格式 / kcesPayloadDescriptor describes the native and ExportCM payload wire formats supported by one KCES extension
type kcesPayloadDescriptor struct {
	Extension              string // 文件扩展名 / File extension
	Kind                   string // 原生 KCES 载荷类型 / Native KCES payload kind
	LengthPrefixed         bool   // 原生载荷是否要求 Int32 长度前缀 / Whether the native payload requires an Int32 length prefix
	ExportCMKind           string // ExportCM 旁车载荷类型，空字符串表示不支持 / ExportCM sidecar payload kind, empty when unsupported
	ExportCMStorageVariant string // ExportCM 旁车的存储变体 / ExportCM sidecar storage variant
}

// KCESPayloadDescriptor 是一个受支持载荷扩展名的只读线格式约定 / KCESPayloadDescriptor is the read-only wire contract for one supported payload extension
type KCESPayloadDescriptor struct {
	Extension              string // 文件扩展名 / File extension
	Kind                   string // 原生 KCES 载荷类型 / Native KCES payload kind
	LengthPrefixed         bool   // 原生载荷是否要求 Int32 长度前缀 / Whether the native payload requires an Int32 length prefix
	ExportCMKind           string // ExportCM 旁车载荷类型 / ExportCM sidecar payload kind
	ExportCMStorageVariant string // ExportCM 旁车存储变体 / ExportCM sidecar storage variant
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

// DescribeKCESPayload 返回扩展名的原生与 ExportCM 线格式描述符，返回副本不能修改注册表
// DescribeKCESPayload returns the native and ExportCM wire descriptor for an extension as a copy that cannot mutate the registry
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

// KCESPayloadEnvelope 是原生 KCES MessagePack 与 ExportCM JSON 旁车共用的可编辑封套，StorageVariant 是写出时的权威判定字段 / KCESPayloadEnvelope is the editable envelope shared by native KCES MessagePack and ExportCM JSON sidecars, with StorageVariant authoritative during encoding
type KCESPayloadEnvelope struct {
	Format          string               `json:"format"`                        // 封套族，kces-msgpack-lz4 或 kces-exportcm-sidecar / Envelope family, either kces-msgpack-lz4 or kces-exportcm-sidecar
	Extension       string               `json:"extension"`                     // 原始文件扩展名，用于判定载荷类型 / Original file extension used to determine payload kind
	StorageVariant  string               `json:"storageVariant"`                // 实际 wire 形态；同一扩展名可能有 KCES 与 ExportCM 两种 wire / Exact wire variant; an extension can have both KCES and ExportCM forms
	Kind            string               `json:"kind"`                          // 解析后的载荷类型 / Decoded payload kind
	DynamicBone     *DynamicBoneStatus   `json:"dynamicBoneStatus,omitempty"`   // 动态骨骼配置载荷 / DynamicBone configuration payload
	ColliderPackage *ColliderPackage     `json:"colliderPackage,omitempty"`     // 通用碰撞体包载荷 / Generic collider package payload
	LimbCollider    *LimbColliderPackage `json:"limbColliderPackage,omitempty"` // LimbColliderMgr 保存的碰撞体包 / Collider package saved by LimbColliderMgr
	IKCollider      *IKColliderPackage   `json:"ikColliderPackage,omitempty"`   // IKColliderSaveLoader 保存的碰撞体包 / Collider package saved by IKColliderSaveLoader
	ClothParams     *ClothParams         `json:"clothParams,omitempty"`         // MagicaCloth.ClothParams 载荷 / MagicaCloth.ClothParams payload
	JSON            json.RawMessage      `json:"json,omitempty"`                // 当字符串载荷是 JSON 时的压缩 JSON / Compacted JSON when the text payload contains JSON
}

// kcesPayloadEnvelopeJSON 以原始 JSON 值区分 union 分支字段缺失与显式 null / kcesPayloadEnvelopeJSON distinguishes missing union branch fields from explicit null by retaining each JSON value
type kcesPayloadEnvelopeJSON struct {
	Format          string          `json:"format"`                        // 封套族 / Envelope family
	Extension       string          `json:"extension"`                     // 原始文件扩展名 / Original file extension
	StorageVariant  string          `json:"storageVariant"`                // 实际线格式变体 / Exact wire variant
	Kind            string          `json:"kind"`                          // union 判别类型 / Union discriminator
	DynamicBone     json.RawMessage `json:"dynamicBoneStatus,omitempty"`   // 动态骨骼分支原始 JSON 值 / Raw JSON value for the DynamicBone branch
	ColliderPackage json.RawMessage `json:"colliderPackage,omitempty"`     // 通用碰撞体分支原始 JSON 值 / Raw JSON value for the generic collider branch
	LimbCollider    json.RawMessage `json:"limbColliderPackage,omitempty"` // LimbCollider 分支原始 JSON 值 / Raw JSON value for the LimbCollider branch
	IKCollider      json.RawMessage `json:"ikColliderPackage,omitempty"`   // IKCollider 分支原始 JSON 值 / Raw JSON value for the IKCollider branch
	ClothParams     json.RawMessage `json:"clothParams,omitempty"`         // ClothParams 分支原始 JSON 值 / Raw JSON value for the ClothParams branch
	JSON            json.RawMessage `json:"json,omitempty"`                // JSON 语义分支原始值 / Raw value for the semantic JSON branch
}

// MarshalJSON 仅写出 kind 对应的活动 union 分支并让类型化 nil 根显式成为 JSON null
// MarshalJSON emits only the union branch selected by kind and represents a typed nil root as explicit JSON null
func (e KCESPayloadEnvelope) MarshalJSON() ([]byte, error) {
	active := payloadRootFieldName(e.Kind)
	if active == "" {
		return nil, fmt.Errorf("unsupported KCES payload kind %q", e.Kind)
	}
	if err := validateKCESPayloadJSONInactiveRoots(&e, active); err != nil {
		return nil, err
	}

	raw := kcesPayloadEnvelopeJSON{
		Format:         e.Format,
		Extension:      e.Extension,
		StorageVariant: e.StorageVariant,
		Kind:           e.Kind,
	}
	var err error
	switch active {
	case "dynamicBoneStatus":
		raw.DynamicBone, err = json.Marshal(e.DynamicBone)
	case "colliderPackage":
		raw.ColliderPackage, err = json.Marshal(e.ColliderPackage)
	case "limbColliderPackage":
		raw.LimbCollider, err = json.Marshal(e.LimbCollider)
	case "ikColliderPackage":
		raw.IKCollider, err = json.Marshal(e.IKCollider)
	case "clothParams":
		raw.ClothParams, err = json.Marshal(e.ClothParams)
	case "json":
		if e.JSON == nil {
			raw.JSON = json.RawMessage("null")
		} else {
			raw.JSON = append(json.RawMessage(nil), e.JSON...)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("marshal active payload field %s: %w", active, err)
	}
	return json.Marshal(raw)
}

// UnmarshalJSON 严格解码载荷封套 JSON，要求活动 union 分支存在，并拒绝未知字段、非活动分支或尾随值
// UnmarshalJSON strictly decodes payload-envelope JSON, requires the active union branch, and rejects unknown fields, inactive branches, or trailing values
func (e *KCESPayloadEnvelope) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("KCES payload JSON is not valid UTF-8")
	}
	var raw kcesPayloadEnvelopeJSON
	if err := decodeKCESJSONStrict(data, &raw); err != nil {
		return err
	}
	active := payloadRootFieldName(raw.Kind)
	if active == "" {
		return fmt.Errorf("unsupported KCES payload kind %q", raw.Kind)
	}
	if err := validateKCESPayloadJSONRootPresence(&raw, active); err != nil {
		return err
	}

	value := KCESPayloadEnvelope{
		Format:         raw.Format,
		Extension:      raw.Extension,
		StorageVariant: raw.StorageVariant,
		Kind:           raw.Kind,
	}
	var err error
	switch active {
	case "dynamicBoneStatus":
		err = decodeKCESJSONStrict(raw.DynamicBone, &value.DynamicBone)
	case "colliderPackage":
		err = decodeKCESJSONStrict(raw.ColliderPackage, &value.ColliderPackage)
	case "limbColliderPackage":
		err = decodeKCESJSONStrict(raw.LimbCollider, &value.LimbCollider)
	case "ikColliderPackage":
		err = decodeKCESJSONStrict(raw.IKCollider, &value.IKCollider)
	case "clothParams":
		err = decodeKCESJSONStrict(raw.ClothParams, &value.ClothParams)
	case "json":
		trimmed := bytes.TrimSpace(raw.JSON)
		if bytes.Equal(trimmed, []byte("null")) && raw.Kind == PayloadKindJSONString {
			value.JSON = nil
		} else {
			var compact bytes.Buffer
			if compactErr := json.Compact(&compact, trimmed); compactErr != nil {
				err = compactErr
			} else {
				value.JSON = append(json.RawMessage(nil), compact.Bytes()...)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("decode active payload field %s: %w", active, err)
	}
	*e = value
	return nil
}

// validateKCESPayloadJSONInactiveRoots 拒绝 Go 封套中与 kind 不匹配且实际携带值的 union 分支
// validateKCESPayloadJSONInactiveRoots rejects populated Go envelope branches that do not match kind
func validateKCESPayloadJSONInactiveRoots(e *KCESPayloadEnvelope, active string) error {
	for _, root := range []struct {
		name    string
		present bool
	}{
		{name: "dynamicBoneStatus", present: e.DynamicBone != nil},
		{name: "colliderPackage", present: e.ColliderPackage != nil},
		{name: "limbColliderPackage", present: e.LimbCollider != nil},
		{name: "ikColliderPackage", present: e.IKCollider != nil},
		{name: "clothParams", present: e.ClothParams != nil},
		{name: "json", present: len(e.JSON) != 0},
	} {
		if root.present && root.name != active {
			return fmt.Errorf("%s is inactive for payload kind %q", root.name, e.Kind)
		}
	}
	return nil
}

// validateKCESPayloadJSONRootPresence 要求活动分支字段出现并拒绝所有出现的非活动分支，包括显式 null
// validateKCESPayloadJSONRootPresence requires the active branch field and rejects every present inactive branch including explicit null
func validateKCESPayloadJSONRootPresence(raw *kcesPayloadEnvelopeJSON, active string) error {
	for _, root := range []struct {
		name string
		data json.RawMessage
	}{
		{name: "dynamicBoneStatus", data: raw.DynamicBone},
		{name: "colliderPackage", data: raw.ColliderPackage},
		{name: "limbColliderPackage", data: raw.LimbCollider},
		{name: "ikColliderPackage", data: raw.IKCollider},
		{name: "clothParams", data: raw.ClothParams},
		{name: "json", data: raw.JSON},
	} {
		if root.name == active && len(root.data) == 0 {
			return fmt.Errorf("payload kind %q requires field %s", raw.Kind, active)
		}
		if root.name != active && len(root.data) != 0 {
			return fmt.Errorf("%s is inactive for payload kind %q", root.name, raw.Kind)
		}
	}
	return nil
}

// decodeKCESJSONStrict 解码单个完整 JSON 值并递归拒绝结构体未知字段
// decodeKCESJSONStrict decodes one complete JSON value and recursively rejects unknown struct fields
func decodeKCESJSONStrict(data []byte, out any) error {
	return strictjson.Decode(data, out)
}

// kcesPayloadNativeDecoder 解码一个扩展名的原生 KCES MessagePack 载荷 / kcesPayloadNativeDecoder decodes the native KCES MessagePack payload for one extension
type kcesPayloadNativeDecoder func(data []byte) (*KCESPayloadEnvelope, error)

// kcesPayloadNativeEncoder 编码一个扩展名的原生 KCES MessagePack 载荷 / kcesPayloadNativeEncoder encodes the native KCES MessagePack payload for one extension
type kcesPayloadNativeEncoder func(env *KCESPayloadEnvelope) ([]byte, error)

// DecodeKCESPayload 作为兼容入口按扩展名分派到独立的载荷解码器
// DecodeKCESPayload dispatches to an independent extension payload decoder as a compatibility entry point
func DecodeKCESPayload(data []byte, extension string) (*KCESPayloadEnvelope, error) {
	switch NormalizeKCESPayloadExtension(extension) {
	case KCESDBConfExtension:
		return DecodeDBConf(data)
	case KCESDBColExtension:
		return DecodeDBCol(data)
	case KCESDB2ConfExtension:
		return DecodeDB2Conf(data)
	case KCESDSBConfExtension:
		return DecodeDSBConf(data)
	case KCESDSB2ConfExtension:
		return DecodeDSB2Conf(data)
	case KCESDSLConfExtension:
		return DecodeDSLConf(data)
	case KCESDSL2ConfExtension:
		return DecodeDSL2Conf(data)
	case KCESDSLColExtension:
		return DecodeDSLCol(data)
	case KCESIKColExtension:
		return DecodeIKCol(data)
	case KCESIKColBytesExtension:
		return DecodeIKColBytes(data)
	case KCESLimbColExtension:
		return DecodeLimbCol(data)
	default:
		return nil, fmt.Errorf("unsupported KCES payload extension %q", extension)
	}
}

// decodeKCESPayloadVariants 尝试一个扩展名声明的原生 KCES 与 ExportCM 线格式并拒绝歧义结果
// decodeKCESPayloadVariants tries the native KCES and ExportCM wire formats declared by one extension and rejects ambiguous results
func decodeKCESPayloadVariants(data []byte, descriptor kcesPayloadDescriptor, decodeNative kcesPayloadNativeDecoder) (*KCESPayloadEnvelope, error) {
	messagePackEnvelope, messagePackErr := decodeNative(data)
	if descriptor.ExportCMKind == "" {
		return messagePackEnvelope, messagePackErr
	}

	exportEnvelope, exportErr := decodeExportCMPayload(data, descriptor.Extension)
	if messagePackErr == nil && exportErr == nil {
		return nil, fmt.Errorf("%s payload is ambiguous: both the KCES int32+LZ4 MessagePack and ExportCM JSON decoders accepted it", descriptor.Extension)
	}
	if messagePackErr == nil {
		return messagePackEnvelope, nil
	}
	if exportErr == nil {
		return exportEnvelope, nil
	}
	return nil, fmt.Errorf("decode %s payload as KCES MessagePack: %v; decode as ExportCM JSON: %w", descriptor.Extension, messagePackErr, exportErr)
}

// decodeKCESMessagePackRoot 验证扩展名要求的长度前缀并解压、严格解码唯一的 MessagePack 根值
// decodeKCESMessagePackRoot validates the extension's required length prefix, decompresses the payload, and strictly decodes its sole MessagePack root value
func decodeKCESMessagePackRoot(data []byte, descriptor kcesPayloadDescriptor, out interface{}) error {
	payload := data
	if descriptor.LengthPrefixed {
		var prefixed bool
		var err error
		payload, prefixed, err = StripLengthPrefix(data)
		if err != nil {
			return err
		}
		if !prefixed {
			if len(data) < 4 {
				return fmt.Errorf("%s payload is missing its required int32 length prefix: file is only %d bytes", descriptor.Extension, len(data))
			}
			declared := binary.LittleEndian.Uint32(data[:4])
			return fmt.Errorf("%s payload has invalid int32 length prefix: declared %d bytes, actual %d", descriptor.Extension, declared, len(data)-4)
		}
	}

	decompressed, err := msgpack.DecompressLz4BlockArray(payload)
	if err != nil {
		return fmt.Errorf("decompress %s payload: %w", descriptor.Extension, err)
	}
	return msgpack.DecodeMsgpack(decompressed, out)
}

// newKCESMessagePackEnvelope 创建一个扩展名的原生 KCES MessagePack 编辑封套
// newKCESMessagePackEnvelope creates a native KCES MessagePack editing envelope for one extension
func newKCESMessagePackEnvelope(descriptor kcesPayloadDescriptor) *KCESPayloadEnvelope {
	return &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      descriptor.Extension,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
		Kind:           descriptor.Kind,
	}
}

// EncodeKCESPayload 作为兼容入口按扩展名分派到独立的载荷编码器
// EncodeKCESPayload dispatches to an independent extension payload encoder as a compatibility entry point
func EncodeKCESPayload(env *KCESPayloadEnvelope) ([]byte, error) {
	if env == nil {
		return nil, fmt.Errorf("nil KCES payload envelope")
	}
	ext := NormalizeKCESPayloadExtension(env.Extension)
	if ext == "" || env.Extension != ext {
		return nil, fmt.Errorf("unsupported or non-canonical KCES payload extension %q", env.Extension)
	}
	switch ext {
	case KCESDBConfExtension:
		return EncodeDBConf(env)
	case KCESDBColExtension:
		return EncodeDBCol(env)
	case KCESDB2ConfExtension:
		return EncodeDB2Conf(env)
	case KCESDSBConfExtension:
		return EncodeDSBConf(env)
	case KCESDSB2ConfExtension:
		return EncodeDSB2Conf(env)
	case KCESDSLConfExtension:
		return EncodeDSLConf(env)
	case KCESDSL2ConfExtension:
		return EncodeDSL2Conf(env)
	case KCESDSLColExtension:
		return EncodeDSLCol(env)
	case KCESIKColExtension:
		return EncodeIKCol(env)
	case KCESIKColBytesExtension:
		return EncodeIKColBytes(env)
	case KCESLimbColExtension:
		return EncodeLimbCol(env)
	default:
		return nil, fmt.Errorf("unsupported KCES payload extension %q", env.Extension)
	}
}

// encodeKCESPayloadVariant 校验封套并按 StorageVariant 调用一个扩展名的原生或 ExportCM 编码器
// encodeKCESPayloadVariant validates an envelope and invokes one extension's native or ExportCM encoder according to StorageVariant
func encodeKCESPayloadVariant(env *KCESPayloadEnvelope, descriptor kcesPayloadDescriptor, encodeNative kcesPayloadNativeEncoder) ([]byte, error) {
	if env == nil {
		return nil, fmt.Errorf("nil %s payload envelope", descriptor.Extension)
	}
	if env.Extension != descriptor.Extension {
		return nil, fmt.Errorf("%s encoder requires canonical extension %q, got %q", descriptor.Extension, descriptor.Extension, env.Extension)
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
		if !descriptor.LengthPrefixed {
			return nil, fmt.Errorf("extension %q does not declare the required int32-length MessagePack storage", descriptor.Extension)
		}
		return encodeNative(env)
	case PayloadStorageExportCMUnityJSON, PayloadStorageExportCMDotNetStringJSON:
		if env.Format != PayloadFormatKCESExportCM {
			return nil, fmt.Errorf("payload format %q is incompatible with storageVariant %q", env.Format, env.StorageVariant)
		}
		return encodeExportCMPayload(env, env.StorageVariant)
	default:
		return nil, fmt.Errorf("unsupported KCES payload storageVariant %q", env.StorageVariant)
	}
}

// validateKCESPayloadEnvelopeRoots 验证封套 tuple 与活动 union 分支符合指定扩展名的约定
// validateKCESPayloadEnvelopeRoots validates that the envelope tuple and active union branch match the specified extension contract
func validateKCESPayloadEnvelopeRoots(env *KCESPayloadEnvelope, descriptor kcesPayloadDescriptor) error {
	if env.StorageVariant == PayloadStorageInt32LZ4MessagePack {
		if env.Kind != descriptor.Kind {
			return fmt.Errorf("extension %q with storageVariant %q requires kind %q, got %q", descriptor.Extension, env.StorageVariant, descriptor.Kind, env.Kind)
		}
		if env.Format != PayloadFormatKCESMessagePack {
			return fmt.Errorf("extension %q requires native tuple format=%q, storageVariant=%q, kind=%q", descriptor.Extension, PayloadFormatKCESMessagePack, PayloadStorageInt32LZ4MessagePack, descriptor.Kind)
		}
		switch descriptor.Kind {
		case PayloadKindDynamicBoneStatus:
		case PayloadKindColliderPackage:
		case PayloadKindLimbCollider:
		case PayloadKindIKCollider:
		case PayloadKindClothParams:
		case PayloadKindJSONString:
		default:
			return fmt.Errorf("unsupported native payload kind %q for extension %q", descriptor.Kind, descriptor.Extension)
		}
		if env.Kind != PayloadKindJSONString && len(env.JSON) != 0 {
			return fmt.Errorf("json is inactive for payload kind %q", env.Kind)
		}
		for name, present := range map[string]bool{
			"dynamicBoneStatus": env.DynamicBone != nil, "colliderPackage": env.ColliderPackage != nil,
			"limbColliderPackage": env.LimbCollider != nil, "ikColliderPackage": env.IKCollider != nil,
			"clothParams": env.ClothParams != nil,
		} {
			if present && name != payloadRootFieldName(descriptor.Kind) {
				return fmt.Errorf("%s is inactive for payload kind %q", name, descriptor.Kind)
			}
		}
		return nil
	}
	if descriptor.ExportCMKind == "" || env.Format != PayloadFormatKCESExportCM || env.Kind != descriptor.ExportCMKind || env.StorageVariant != descriptor.ExportCMStorageVariant {
		return fmt.Errorf("extension %q has an inconsistent ExportCM payload tuple", descriptor.Extension)
	}
	if env.DynamicBone != nil || env.ColliderPackage != nil || env.LimbCollider != nil || env.IKCollider != nil || env.ClothParams != nil {
		return fmt.Errorf("ExportCM payload has inactive native MessagePack fields")
	}
	if len(env.JSON) == 0 {
		return fmt.Errorf("ExportCM payload requires json")
	}
	return nil
}

// payloadRootFieldName 返回 kind 在 editing JSON 中选择的活动 union 字段
// payloadRootFieldName returns the active editing-JSON union field selected by kind
func payloadRootFieldName(kind string) string {
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
	case PayloadKindJSONString, PayloadKindExportCMDynamicBoneJSON, PayloadKindExportCMColliderJSON:
		return "json"
	default:
		return ""
	}
}

// encodeKCESMessagePackRoot 压缩已编码的 MessagePack 根值并按扩展名约定添加长度前缀
// encodeKCESMessagePackRoot compresses an encoded MessagePack root value and adds the length prefix declared by the extension
func encodeKCESMessagePackRoot(msgpackData []byte, descriptor kcesPayloadDescriptor) ([]byte, error) {
	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		return nil, fmt.Errorf("compress %s payload: %w", descriptor.Extension, err)
	}
	if descriptor.LengthPrefixed {
		return AddLengthPrefix(compressed), nil
	}
	return compressed, nil
}

// StripLengthPrefix 仅在首个小端 UInt32 与剩余字节数完全一致时移除长度前缀
// StripLengthPrefix removes a length prefix only when the first little-endian UInt32 exactly matches the remaining byte count
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

// AddLengthPrefix 在载荷前添加小端 UInt32 字节数
// AddLengthPrefix prepends the payload byte count as a little-endian UInt32
func AddLengthPrefix(payload []byte) []byte {
	out := make([]byte, 4, len(payload)+4)
	binary.LittleEndian.PutUint32(out[:4], uint32(len(payload)))
	return append(out, payload...)
}

// IsLengthPrefixedKCESPayloadExtension 判断扩展名的原生载荷是否要求长度前缀
// IsLengthPrefixedKCESPayloadExtension reports whether an extension requires a length prefix for its native payload
func IsLengthPrefixedKCESPayloadExtension(extension string) bool {
	descriptor, ok := kcesPayloadDescriptorByExtension[NormalizeKCESPayloadExtension(extension)]
	return ok && descriptor.LengthPrefixed
}

// IsKCESPayloadExtension 判断扩展名或路径是否属于支持的 KCES 载荷格式
// IsKCESPayloadExtension reports whether an extension or path belongs to a supported KCES payload format
func IsKCESPayloadExtension(extension string) bool {
	return NormalizeKCESPayloadExtension(extension) != ""
}

// NormalizeKCESPayloadExtension 从路径或扩展名提取规范化的受支持扩展名
// NormalizeKCESPayloadExtension extracts a normalized supported extension from a path or extension
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

// payloadKindForExtension 返回扩展名声明的原生载荷类型，未知扩展名返回空字符串
// payloadKindForExtension returns the native payload kind declared by an extension and an empty string for an unknown extension
func payloadKindForExtension(ext string) string {
	if descriptor, ok := kcesPayloadDescriptorByExtension[NormalizeKCESPayloadExtension(ext)]; ok {
		return descriptor.Kind
	}
	return ""
}
