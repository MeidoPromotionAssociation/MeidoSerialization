package KCES

// .limbcol
// LimbColliderMgr 使用的肢体碰撞体文件。载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// LimbColliderPackage MessagePack indexed-array
// .limbcol
// Limb collider file consumed by LimbColliderMgr. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed LimbColliderPackage MessagePack indexed array

const KCESLimbColExtension = ".limbcol"

var limbcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESLimbColExtension,
	Kind:           PayloadKindLimbCollider,
	LengthPrefixed: true,
}

// DecodeLimbCol 解码 .limbcol 的长度前缀 LZ4 MessagePack LimbColliderPackage 载荷
// DecodeLimbCol decodes the length-prefixed LZ4 MessagePack LimbColliderPackage payload of a .limbcol file
func DecodeLimbCol(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeKCESPayloadVariants(data, limbcolPayloadDescriptor, decodeLimbColMessagePack)
}

// decodeLimbColMessagePack 解码 .limbcol 的原生 LimbColliderPackage MessagePack 载荷
// decodeLimbColMessagePack decodes the native LimbColliderPackage MessagePack payload of a .limbcol file
func decodeLimbColMessagePack(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeLimbColliderMessagePack(data, limbcolPayloadDescriptor)
}

// EncodeLimbCol 编码 .limbcol 的长度前缀 LZ4 MessagePack LimbColliderPackage 载荷
// EncodeLimbCol encodes the length-prefixed LZ4 MessagePack LimbColliderPackage payload of a .limbcol file
func EncodeLimbCol(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeKCESPayloadVariant(env, limbcolPayloadDescriptor, encodeLimbColMessagePack)
}

// encodeLimbColMessagePack 编码 .limbcol 的原生 LimbColliderPackage MessagePack 载荷
// encodeLimbColMessagePack encodes the native LimbColliderPackage MessagePack payload of a .limbcol file
func encodeLimbColMessagePack(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeLimbColliderMessagePack(env, limbcolPayloadDescriptor)
}
