package KCES

// .dbcol
// DynamicBone 的碰撞体包文件。原生 KCES 载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// ColliderPackage MessagePack
// .dbcol
// DynamicBone collider-package file. Native KCES data is an Int32 length prefix followed by an
// LZ4 Block Array-compressed ColliderPackage MessagePack value

const KCESDBColExtension = ".dbcol"

var dbcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDBColExtension,
	Kind:           PayloadKindColliderPackage,
	LengthPrefixed: true,
}

// DecodeDBCol 解码 .dbcol 的长度前缀 LZ4 MessagePack ColliderPackage 载荷
// DecodeDBCol decodes the length-prefixed LZ4 MessagePack ColliderPackage payload of a .dbcol file
func DecodeDBCol(data []byte) (*ColliderPackage, error) {
	return decodeColliderPackageMessagePack(data, dbcolPayloadDescriptor)
}

// EncodeDBCol 编码 .dbcol 的长度前缀 LZ4 MessagePack ColliderPackage 载荷
// EncodeDBCol encodes the length-prefixed LZ4 MessagePack ColliderPackage payload of a .dbcol file
func EncodeDBCol(value *ColliderPackage) ([]byte, error) {
	return encodeColliderPackageMessagePack(value, dbcolPayloadDescriptor)
}
