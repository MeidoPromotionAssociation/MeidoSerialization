package KCES

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/strictjson"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

// KCES 物理与碰撞扩展名的调度层，各扩展名的线格式描述由对应文件声明
//
// 每个扩展名的编辑 JSON 根就是该扩展名的载荷对象本身，没有额外封套：目标格式完全由文件名决定，
// 而这些文件只有一种线格式，即 Int32 长度前缀加 LZ4 Block Array 压缩的 MessagePack。
// 根为 JSON null 表示 MessagePack 根值为 nil
//
// Dispatcher for KCES physics and collider extensions, with each extension declaring its wire descriptor in its own file
//
// The editing JSON root of every extension is the payload object itself with no surrounding envelope:
// the destination format is determined entirely by the file name, and these files have exactly one wire
// format, an Int32 length prefix followed by LZ4 Block Array-compressed MessagePack.
// A JSON null root represents a nil MessagePack root value

const (
	PayloadKindDynamicBoneStatus = "dynamic-bone-status"
	PayloadKindJSONString        = "msgpack-json-string"
	PayloadKindColliderPackage   = "collider-package"
	PayloadKindLimbCollider      = "limb-collider-package"
	PayloadKindIKCollider        = "ik-collider-package"
	PayloadKindClothParams       = "cloth-params"
)

// kcesPayloadDescriptor 描述一个 KCES 扩展名的原生载荷线格式 / kcesPayloadDescriptor describes the native payload wire format of one KCES extension
type kcesPayloadDescriptor struct {
	Extension      string // 文件扩展名 / File extension
	Kind           string // 原生 KCES 载荷类型 / Native KCES payload kind
	LengthPrefixed bool   // 原生载荷是否要求 Int32 长度前缀 / Whether the native payload requires an Int32 length prefix
}

// KCESPayloadDescriptor 是一个受支持载荷扩展名的只读线格式约定 / KCESPayloadDescriptor is the read-only wire contract for one supported payload extension
type KCESPayloadDescriptor struct {
	Extension      string // 文件扩展名 / File extension
	Kind           string // 原生 KCES 载荷类型 / Native KCES payload kind
	LengthPrefixed bool   // 原生载荷是否要求 Int32 长度前缀 / Whether the native payload requires an Int32 length prefix
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

// DescribeKCESPayload 返回扩展名的原生线格式描述符，返回副本不能修改注册表
// DescribeKCESPayload returns the native wire descriptor for an extension as a copy that cannot mutate the registry
func DescribeKCESPayload(extension string) (KCESPayloadDescriptor, bool) {
	descriptor, ok := kcesPayloadDescriptorByExtension[NormalizeKCESPayloadExtension(extension)]
	if !ok {
		return KCESPayloadDescriptor{}, false
	}
	return KCESPayloadDescriptor{
		Extension: descriptor.Extension, Kind: descriptor.Kind,
		LengthPrefixed: descriptor.LengthPrefixed,
	}, true
}

// DecodeKCESPayload 按扩展名解码载荷并返回该扩展名的载荷根对象，根为 nil 表示 MessagePack 根值为 nil
// DecodeKCESPayload decodes a payload by extension and returns that extension's payload root object, where a nil root means a nil MessagePack root value
func DecodeKCESPayload(data []byte, extension string) (any, error) {
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

// EncodeKCESPayload 按扩展名编码载荷根对象，并要求根对象类型与扩展名声明的载荷类型一致
// EncodeKCESPayload encodes a payload root object by extension and requires the root type to match the payload type declared by that extension
func EncodeKCESPayload(value any, extension string) ([]byte, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	descriptor, ok := kcesPayloadDescriptorByExtension[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported KCES payload extension %q", extension)
	}
	switch typed := value.(type) {
	case nil:
		return encodeNilKCESPayloadRoot(descriptor)
	case *DynamicBoneStatus:
		if descriptor.Kind != PayloadKindDynamicBoneStatus {
			return nil, newKCESPayloadRootTypeError(descriptor, value)
		}
		return encodeDynamicBoneStatusMessagePack(typed, descriptor)
	case *ColliderPackage:
		if descriptor.Kind != PayloadKindColliderPackage {
			return nil, newKCESPayloadRootTypeError(descriptor, value)
		}
		return encodeColliderPackageMessagePack(typed, descriptor)
	case *LimbColliderPackage:
		if descriptor.Kind != PayloadKindLimbCollider {
			return nil, newKCESPayloadRootTypeError(descriptor, value)
		}
		return encodeLimbColliderMessagePack(typed, descriptor)
	case *IKColliderPackage:
		if descriptor.Kind != PayloadKindIKCollider {
			return nil, newKCESPayloadRootTypeError(descriptor, value)
		}
		return encodeIKColliderMessagePack(typed, descriptor)
	case *ClothParams:
		if descriptor.Kind != PayloadKindClothParams {
			return nil, newKCESPayloadRootTypeError(descriptor, value)
		}
		return encodeClothParamsMessagePack(typed, descriptor)
	case *MagicaClothSerializeData:
		if descriptor.Kind != PayloadKindJSONString {
			return nil, newKCESPayloadRootTypeError(descriptor, value)
		}
		return encodeJSONStringMessagePack(typed, descriptor)
	default:
		return nil, newKCESPayloadRootTypeError(descriptor, value)
	}
}

// newKCESPayloadRootTypeError 报告根对象类型与扩展名声明的载荷类型不一致
// newKCESPayloadRootTypeError reports that the root object type does not match the payload type declared by the extension
func newKCESPayloadRootTypeError(descriptor kcesPayloadDescriptor, value any) error {
	return fmt.Errorf("extension %q requires payload kind %q, got root type %T", descriptor.Extension, descriptor.Kind, value)
}

// encodeNilKCESPayloadRoot 将 nil 载荷根编码为 MessagePack nil 根值
// encodeNilKCESPayloadRoot encodes a nil payload root as a nil MessagePack root value
func encodeNilKCESPayloadRoot(descriptor kcesPayloadDescriptor) ([]byte, error) {
	data, err := msgpack.EncodeMsgpack(nil)
	if err != nil {
		return nil, fmt.Errorf("encode nil %s payload root: %w", descriptor.Extension, err)
	}
	return encodeKCESMessagePackRoot(data, descriptor)
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
				return fmt.Errorf("%s payload is missing its required int32 length prefix: file is only %d bytes%s", descriptor.Extension, len(data), describeExportCMSidecar(data))
			}
			declared := binary.LittleEndian.Uint32(data[:4])
			return fmt.Errorf("%s payload has invalid int32 length prefix: declared %d bytes, actual %d%s", descriptor.Extension, declared, len(data)-4, describeExportCMSidecar(data))
		}
	}

	decompressed, err := msgpack.DecompressLz4BlockArray(payload)
	if err != nil {
		return fmt.Errorf("decompress %s payload: %w", descriptor.Extension, err)
	}
	return msgpack.DecodeMsgpack(decompressed, out)
}

// describeExportCMSidecar 在输入疑似 ExportCM 写给 COM3D2.5 的 JSON 中间产物时补充一句说明，其余情况返回空字符串
// describeExportCMSidecar adds one explanatory clause when the input looks like the JSON intermediate ExportCM writes for COM3D2.5 and returns an empty string otherwise
func describeExportCMSidecar(data []byte) string {
	if !looksLikeExportCMSidecar(data) {
		return ""
	}
	return "; this looks like the JSON that KCES ExportCM writes for COM3D2.5 to read, which reuses KCES extensions but is not a KCES resource, and is not supported"
}

// looksLikeExportCMSidecar 判断输入是否为直接 UTF-8 Unity JSON 或包在 BinaryWriter 字符串中的 Unity JSON
// looksLikeExportCMSidecar reports whether the input is direct UTF-8 Unity JSON or Unity JSON wrapped in a BinaryWriter string
func looksLikeExportCMSidecar(data []byte) bool {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		data = data[3:]
	}
	if startsWithJSONObject(data) {
		return true
	}
	// ExportCM 的 .dslcol 把 UTF-8 Unity JSON 写成 BinaryWriter 的 7 位变长长度前缀字符串
	// The ExportCM .dslcol variant writes UTF-8 Unity JSON as a BinaryWriter string with a 7-bit variable-length prefix
	for index := 0; index < len(data) && index < 5; index++ {
		if data[index]&0x80 != 0 {
			continue
		}
		return startsWithJSONObject(data[index+1:])
	}
	return false
}

// startsWithJSONObject 判断字节序列跳过前导空白后是否以 JSON 对象或数组开头
// startsWithJSONObject reports whether a byte sequence begins with a JSON object or array after leading whitespace
func startsWithJSONObject(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	return len(trimmed) != 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

// decodeKCESJSONStrict 解码单个完整 JSON 值并递归拒绝结构体未知字段
// decodeKCESJSONStrict decodes one complete JSON value and recursively rejects unknown struct fields
func decodeKCESJSONStrict(data []byte, out any) error {
	return strictjson.Decode(data, out)
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
