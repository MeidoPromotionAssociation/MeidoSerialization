package KCES

// .dslconf
// DynamicSleeveBone 的旧版 MagicaCloth 参数文件。载荷为 Int32 长度前缀加 LZ4 Block Array
// 压缩的 ClothParams MessagePack indexed-array。
//
// .dslconf
// Legacy DynamicSleeveBone MagicaCloth parameter file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed ClothParams MessagePack indexed array.

const KCESDSLConfExtension = ".dslconf"

var dslconfPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDSLConfExtension,
	Kind:           PayloadKindClothParams,
	LengthPrefixed: true,
}
