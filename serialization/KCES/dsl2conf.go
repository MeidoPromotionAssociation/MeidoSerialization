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

// DecodeDSL2Conf 解码 .dsl2conf 的长度前缀 LZ4 MessagePack JSON 字符串载荷
// DecodeDSL2Conf decodes the length-prefixed LZ4 MessagePack JSON-string payload of a .dsl2conf file
func DecodeDSL2Conf(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeKCESPayloadVariants(data, dsl2confPayloadDescriptor, decodeDSL2ConfMessagePack)
}

// decodeDSL2ConfMessagePack 解码 .dsl2conf 的原生 MessagePack JSON 字符串载荷
// decodeDSL2ConfMessagePack decodes the native MessagePack JSON-string payload of a .dsl2conf file
func decodeDSL2ConfMessagePack(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeJSONStringMessagePack(data, dsl2confPayloadDescriptor)
}

// EncodeDSL2Conf 编码 .dsl2conf 的长度前缀 LZ4 MessagePack JSON 字符串载荷
// EncodeDSL2Conf encodes the length-prefixed LZ4 MessagePack JSON-string payload of a .dsl2conf file
func EncodeDSL2Conf(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeKCESPayloadVariant(env, dsl2confPayloadDescriptor, encodeDSL2ConfMessagePack)
}

// encodeDSL2ConfMessagePack 编码 .dsl2conf 的原生 MessagePack JSON 字符串载荷
// encodeDSL2ConfMessagePack encodes the native MessagePack JSON-string payload of a .dsl2conf file
func encodeDSL2ConfMessagePack(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeJSONStringMessagePack(env, dsl2confPayloadDescriptor)
}
