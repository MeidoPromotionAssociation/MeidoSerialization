package KCES

// .dslcol
// DynamicSleeveBone 的碰撞体包文件。原生 KCES 载荷为 Int32 长度前缀加 LZ4 Block Array
// 压缩的 ColliderPackage MessagePack；ExportCM 变体则把 Unity JSON 写成 BinaryWriter 字符串
// .dslcol
// DynamicSleeveBone collider-package file. Native KCES data is an Int32 length prefix followed by an
// LZ4 Block Array-compressed ColliderPackage MessagePack value; the ExportCM variant stores Unity JSON as a BinaryWriter string

const KCESDSLColExtension = ".dslcol"

var dslcolPayloadDescriptor = kcesPayloadDescriptor{
	Extension:              KCESDSLColExtension,
	Kind:                   PayloadKindColliderPackage,
	LengthPrefixed:         true,
	ExportCMKind:           PayloadKindExportCMColliderJSON,
	ExportCMStorageVariant: PayloadStorageExportCMDotNetStringJSON,
}
