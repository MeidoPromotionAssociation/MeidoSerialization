package KCES

// .ikcol
// 全身 IK 的碰撞体组文件。载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// IKColliderPackage MessagePack indexed-array。
//
// .ikcol
// Full-body IK collider-group file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed IKColliderPackage MessagePack indexed array.

const KCESIKColExtension = ".ikcol"

var ikcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESIKColExtension,
	Kind:           PayloadKindIKCollider,
	LengthPrefixed: true,
}
