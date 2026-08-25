package KCES

// .dsb2conf
// DynamicSkirtBone 的新版配置文件。载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// MessagePack JSON 字符串
// .dsb2conf
// Newer DynamicSkirtBone configuration file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed MessagePack JSON string

const KCESDSB2ConfExtension = ".dsb2conf"

var dsb2confPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDSB2ConfExtension,
	Kind:           PayloadKindJSONString,
	LengthPrefixed: true,
}

// DecodeDSB2Conf 解码 .dsb2conf 的长度前缀 LZ4 MessagePack MagicaCloth ClothSerializeData 载荷
// DecodeDSB2Conf decodes the length-prefixed LZ4 MessagePack MagicaCloth ClothSerializeData payload of a .dsb2conf file
func DecodeDSB2Conf(data []byte) (*MagicaClothSerializeData, error) {
	return decodeJSONStringMessagePack(data, dsb2confPayloadDescriptor)
}

// EncodeDSB2Conf 编码 .dsb2conf 的长度前缀 LZ4 MessagePack MagicaCloth ClothSerializeData 载荷
// EncodeDSB2Conf encodes the length-prefixed LZ4 MessagePack MagicaCloth ClothSerializeData payload of a .dsb2conf file
func EncodeDSB2Conf(value *MagicaClothSerializeData) ([]byte, error) {
	return encodeJSONStringMessagePack(value, dsb2confPayloadDescriptor)
}
