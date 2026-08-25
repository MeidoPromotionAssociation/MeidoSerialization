package KCES

// .dslcol
// DynamicSleeveBone 的碰撞体包文件。原生 KCES 载荷为 Int32 长度前缀加 LZ4 Block Array
// 压缩的 ColliderPackage MessagePack
// .dslcol
// DynamicSleeveBone collider-package file. Native KCES data is an Int32 length prefix followed by an
// LZ4 Block Array-compressed ColliderPackage MessagePack value

const KCESDSLColExtension = ".dslcol"

var dslcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDSLColExtension,
	Kind:           PayloadKindColliderPackage,
	LengthPrefixed: true,
}

// DecodeDSLCol 解码 .dslcol 的长度前缀 LZ4 MessagePack ColliderPackage 载荷
// DecodeDSLCol decodes the length-prefixed LZ4 MessagePack ColliderPackage payload of a .dslcol file
func DecodeDSLCol(data []byte) (*ColliderPackage, error) {
	return decodeColliderPackageMessagePack(data, dslcolPayloadDescriptor)
}

// EncodeDSLCol 编码 .dslcol 的长度前缀 LZ4 MessagePack ColliderPackage 载荷
// EncodeDSLCol encodes the length-prefixed LZ4 MessagePack ColliderPackage payload of a .dslcol file
func EncodeDSLCol(value *ColliderPackage) ([]byte, error) {
	return encodeColliderPackageMessagePack(value, dslcolPayloadDescriptor)
}
