package KCES

// .dsbconf
// DynamicSkirtBone 的旧版 MagicaCloth 参数文件。载荷为 Int32 长度前缀加 LZ4 Block Array
// 压缩的 ClothParams MessagePack indexed-array
// .dsbconf
// Legacy DynamicSkirtBone MagicaCloth parameter file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed ClothParams MessagePack indexed array

const KCESDSBConfExtension = ".dsbconf"

var dsbconfPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDSBConfExtension,
	Kind:           PayloadKindClothParams,
	LengthPrefixed: true,
}

// DecodeDSBConf 解码 .dsbconf 的长度前缀 LZ4 MessagePack ClothParams 载荷
// DecodeDSBConf decodes the length-prefixed LZ4 MessagePack ClothParams payload of a .dsbconf file
func DecodeDSBConf(data []byte) (*ClothParams, error) {
	return decodeClothParamsMessagePack(data, dsbconfPayloadDescriptor)
}

// EncodeDSBConf 编码 .dsbconf 的长度前缀 LZ4 MessagePack ClothParams 载荷
// EncodeDSBConf encodes the length-prefixed LZ4 MessagePack ClothParams payload of a .dsbconf file
func EncodeDSBConf(value *ClothParams) ([]byte, error) {
	return encodeClothParamsMessagePack(value, dsbconfPayloadDescriptor)
}
