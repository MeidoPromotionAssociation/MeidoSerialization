package KCES

// .dslcol
// DynamicSleeveBone 的碰撞体包文件。原生 KCES 载荷为 Int32 长度前缀加 LZ4 Block Array
// 压缩的 ColliderPackage MessagePack；ExportCM 变体则把 Unity JSON 写成 BinaryWriter 字符串
// .dslcol
// DynamicSleeveBone collider-package file. Native KCES data is an Int32 length prefix followed by an
// LZ4 Block Array-compressed ColliderPackage MessagePack value; the ExportCM variant stores Unity JSON as a BinaryWriter string

const KCESDSLColExtension = ".dslcol"

var dslcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:              KCESDSLColExtension,
	Kind:                   PayloadKindColliderPackage,
	LengthPrefixed:         true,
	ExportCMKind:           PayloadKindExportCMColliderJSON,
	ExportCMStorageVariant: PayloadStorageExportCMDotNetStringJSON,
}

// DecodeDSLCol 解码 .dslcol 的原生 ColliderPackage 或 ExportCM BinaryWriter 字符串 JSON 线格式并拒绝歧义输入
// DecodeDSLCol decodes the native ColliderPackage or ExportCM BinaryWriter string JSON wire format of a .dslcol file and rejects ambiguous input
func DecodeDSLCol(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeKCESPayloadVariants(data, dslcolPayloadDescriptor, decodeDSLColMessagePack)
}

// decodeDSLColMessagePack 解码 .dslcol 的原生 ColliderPackage MessagePack 载荷
// decodeDSLColMessagePack decodes the native ColliderPackage MessagePack payload of a .dslcol file
func decodeDSLColMessagePack(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeColliderPackageMessagePack(data, dslcolPayloadDescriptor)
}

// EncodeDSLCol 按封套声明的原生 KCES 或 ExportCM 存储变体编码 .dslcol 载荷
// EncodeDSLCol encodes a .dslcol payload using the native KCES or ExportCM storage variant declared by the envelope
func EncodeDSLCol(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeKCESPayloadVariant(env, dslcolPayloadDescriptor, encodeDSLColMessagePack)
}

// encodeDSLColMessagePack 编码 .dslcol 的原生 ColliderPackage MessagePack 载荷
// encodeDSLColMessagePack encodes the native ColliderPackage MessagePack payload of a .dslcol file
func encodeDSLColMessagePack(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeColliderPackageMessagePack(env, dslcolPayloadDescriptor)
}
