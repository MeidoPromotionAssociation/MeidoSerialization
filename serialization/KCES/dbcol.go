package KCES

// .dbcol
// DynamicBone 的碰撞体包文件。原生 KCES 载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// ColliderPackage MessagePack；ExportCM 也会使用同一扩展名写出直接 UTF-8 Unity JSON。
//
// .dbcol
// DynamicBone collider-package file. Native KCES data is an Int32 length prefix followed by an
// LZ4 Block Array-compressed ColliderPackage MessagePack value; ExportCM also writes direct UTF-8 Unity JSON with this extension.

const KCESDBColExtension = ".dbcol"

var dbcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:              KCESDBColExtension,
	Kind:                   PayloadKindColliderPackage,
	LengthPrefixed:         true,
	ExportCMKind:           PayloadKindExportCMColliderJSON,
	ExportCMStorageVariant: PayloadStorageExportCMUnityJSON,
}
