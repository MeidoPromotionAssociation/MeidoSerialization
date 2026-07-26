package KCES

// .ikcol.bytes
// Unity Resources 中使用复合扩展名保存的 IK 碰撞体文件，wire 布局与 .ikcol 完全相同
// 识别时必须匹配完整复合扩展名，不能退化为普通 .bytes 文件
// .ikcol.bytes
// IK collider file stored under a compound extension in Unity Resources, with the same wire layout as .ikcol
// Detection must match the complete compound extension instead of treating it as an ordinary .bytes file

const KCESIKColBytesExtension = ".ikcol.bytes"

var ikcolBytesPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESIKColBytesExtension,
	Kind:           PayloadKindIKCollider,
	LengthPrefixed: true,
}

// DecodeIKColBytes 解码 .ikcol.bytes 的长度前缀 LZ4 MessagePack IKColliderPackage 载荷
// DecodeIKColBytes decodes the length-prefixed LZ4 MessagePack IKColliderPackage payload of an .ikcol.bytes file
func DecodeIKColBytes(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeKCESPayloadVariants(data, ikcolBytesPayloadDescriptor, decodeIKColBytesMessagePack)
}

// decodeIKColBytesMessagePack 解码 .ikcol.bytes 的原生 IKColliderPackage MessagePack 载荷
// decodeIKColBytesMessagePack decodes the native IKColliderPackage MessagePack payload of an .ikcol.bytes file
func decodeIKColBytesMessagePack(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeIKColliderMessagePack(data, ikcolBytesPayloadDescriptor)
}

// EncodeIKColBytes 编码 .ikcol.bytes 的长度前缀 LZ4 MessagePack IKColliderPackage 载荷
// EncodeIKColBytes encodes the length-prefixed LZ4 MessagePack IKColliderPackage payload of an .ikcol.bytes file
func EncodeIKColBytes(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeKCESPayloadVariant(env, ikcolBytesPayloadDescriptor, encodeIKColBytesMessagePack)
}

// encodeIKColBytesMessagePack 编码 .ikcol.bytes 的原生 IKColliderPackage MessagePack 载荷
// encodeIKColBytesMessagePack encodes the native IKColliderPackage MessagePack payload of an .ikcol.bytes file
func encodeIKColBytesMessagePack(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeIKColliderMessagePack(env, ikcolBytesPayloadDescriptor)
}
