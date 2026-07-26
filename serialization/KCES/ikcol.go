package KCES

// .ikcol
// 全身 IK 的碰撞体组文件。载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// IKColliderPackage MessagePack indexed-array
// .ikcol
// Full-body IK collider-group file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed IKColliderPackage MessagePack indexed array

const KCESIKColExtension = ".ikcol"

var ikcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESIKColExtension,
	Kind:           PayloadKindIKCollider,
	LengthPrefixed: true,
}

// DecodeIKCol 解码 .ikcol 的长度前缀 LZ4 MessagePack IKColliderPackage 载荷
// DecodeIKCol decodes the length-prefixed LZ4 MessagePack IKColliderPackage payload of an .ikcol file
func DecodeIKCol(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeKCESPayloadVariants(data, ikcolPayloadDescriptor, decodeIKColMessagePack)
}

// decodeIKColMessagePack 解码 .ikcol 的原生 IKColliderPackage MessagePack 载荷
// decodeIKColMessagePack decodes the native IKColliderPackage MessagePack payload of an .ikcol file
func decodeIKColMessagePack(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeIKColliderMessagePack(data, ikcolPayloadDescriptor)
}

// EncodeIKCol 编码 .ikcol 的长度前缀 LZ4 MessagePack IKColliderPackage 载荷
// EncodeIKCol encodes the length-prefixed LZ4 MessagePack IKColliderPackage payload of an .ikcol file
func EncodeIKCol(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeKCESPayloadVariant(env, ikcolPayloadDescriptor, encodeIKColMessagePack)
}

// encodeIKColMessagePack 编码 .ikcol 的原生 IKColliderPackage MessagePack 载荷
// encodeIKColMessagePack encodes the native IKColliderPackage MessagePack payload of an .ikcol file
func encodeIKColMessagePack(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeIKColliderMessagePack(env, ikcolPayloadDescriptor)
}
