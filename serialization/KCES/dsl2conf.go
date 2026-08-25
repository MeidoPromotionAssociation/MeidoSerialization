package KCES

// .dsl2conf
// DynamicSleeveBone 的新版配置文件。载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// MessagePack JSON 字符串
// .dsl2conf
// Newer DynamicSleeveBone configuration file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed MessagePack JSON string

const KCESDSL2ConfExtension = ".dsl2conf"

var dsl2confPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDSL2ConfExtension,
	Kind:           PayloadKindJSONString,
	LengthPrefixed: true,
}

// DecodeDSL2Conf 解码 .dsl2conf 的长度前缀 LZ4 MessagePack MagicaCloth ClothSerializeData 载荷
// DecodeDSL2Conf decodes the length-prefixed LZ4 MessagePack MagicaCloth ClothSerializeData payload of a .dsl2conf file
func DecodeDSL2Conf(data []byte) (*MagicaClothSerializeData, error) {
	return decodeJSONStringMessagePack(data, dsl2confPayloadDescriptor)
}

// EncodeDSL2Conf 编码 .dsl2conf 的长度前缀 LZ4 MessagePack MagicaCloth ClothSerializeData 载荷
// EncodeDSL2Conf encodes the length-prefixed LZ4 MessagePack MagicaCloth ClothSerializeData payload of a .dsl2conf file
func EncodeDSL2Conf(value *MagicaClothSerializeData) ([]byte, error) {
	return encodeJSONStringMessagePack(value, dsl2confPayloadDescriptor)
}
