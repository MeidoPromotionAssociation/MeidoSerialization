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
func DecodeLimbCol(data []byte) (*LimbColliderPackage, error) {
	return decodeLimbColliderMessagePack(data, limbcolPayloadDescriptor)
}

// EncodeLimbCol 编码 .limbcol 的长度前缀 LZ4 MessagePack LimbColliderPackage 载荷
// EncodeLimbCol encodes the length-prefixed LZ4 MessagePack LimbColliderPackage payload of a .limbcol file
func EncodeLimbCol(value *LimbColliderPackage) ([]byte, error) {
	return encodeLimbColliderMessagePack(value, limbcolPayloadDescriptor)
}
