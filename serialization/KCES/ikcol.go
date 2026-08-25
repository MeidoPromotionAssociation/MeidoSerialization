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
// DecodeIKCol decodes the length-prefixed LZ4 MessagePack IKColliderPackage payload of a .ikcol file
func DecodeIKCol(data []byte) (*IKColliderPackage, error) {
	return decodeIKColliderMessagePack(data, ikcolPayloadDescriptor)
}

// EncodeIKCol 编码 .ikcol 的长度前缀 LZ4 MessagePack IKColliderPackage 载荷
// EncodeIKCol encodes the length-prefixed LZ4 MessagePack IKColliderPackage payload of a .ikcol file
func EncodeIKCol(value *IKColliderPackage) ([]byte, error) {
	return encodeIKColliderMessagePack(value, ikcolPayloadDescriptor)
}
