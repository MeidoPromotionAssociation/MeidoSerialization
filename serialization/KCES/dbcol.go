package KCES

// .dbcol
// DynamicBone 的碰撞体包文件。原生 KCES 载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// ColliderPackage MessagePack；ExportCM 也会使用同一扩展名写出直接 UTF-8 Unity JSON
// .dbcol
// DynamicBone collider-package file. Native KCES data is an Int32 length prefix followed by an
// LZ4 Block Array-compressed ColliderPackage MessagePack value; ExportCM also writes direct UTF-8 Unity JSON with this extension

const KCESDBColExtension = ".dbcol"

var dbcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:              KCESDBColExtension,
	Kind:                   PayloadKindColliderPackage,
	LengthPrefixed:         true,
	ExportCMKind:           PayloadKindExportCMColliderJSON,
	ExportCMStorageVariant: PayloadStorageExportCMUnityJSON,
}

// DecodeDBCol 解码 .dbcol 的原生 ColliderPackage 或 ExportCM Unity JSON 线格式并拒绝歧义输入
// DecodeDBCol decodes the native ColliderPackage or ExportCM Unity JSON wire format of a .dbcol file and rejects ambiguous input
func DecodeDBCol(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeKCESPayloadVariants(data, dbcolPayloadDescriptor, decodeDBColMessagePack)
}

// decodeDBColMessagePack 解码 .dbcol 的原生 ColliderPackage MessagePack 载荷
// decodeDBColMessagePack decodes the native ColliderPackage MessagePack payload of a .dbcol file
func decodeDBColMessagePack(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeColliderPackageMessagePack(data, dbcolPayloadDescriptor)
}

// EncodeDBCol 按封套声明的原生 KCES 或 ExportCM 存储变体编码 .dbcol 载荷
// EncodeDBCol encodes a .dbcol payload using the native KCES or ExportCM storage variant declared by the envelope
func EncodeDBCol(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeKCESPayloadVariant(env, dbcolPayloadDescriptor, encodeDBColMessagePack)
}

// encodeDBColMessagePack 编码 .dbcol 的原生 ColliderPackage MessagePack 载荷
// encodeDBColMessagePack encodes the native ColliderPackage MessagePack payload of a .dbcol file
func encodeDBColMessagePack(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeColliderPackageMessagePack(env, dbcolPayloadDescriptor)
}
