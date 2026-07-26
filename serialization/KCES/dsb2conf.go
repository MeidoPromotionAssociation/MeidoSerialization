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

// DecodeDSB2Conf 解码 .dsb2conf 的长度前缀 LZ4 MessagePack JSON 字符串载荷
// DecodeDSB2Conf decodes the length-prefixed LZ4 MessagePack JSON-string payload of a .dsb2conf file
func DecodeDSB2Conf(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeKCESPayloadVariants(data, dsb2confPayloadDescriptor, decodeDSB2ConfMessagePack)
}

// decodeDSB2ConfMessagePack 解码 .dsb2conf 的原生 MessagePack JSON 字符串载荷
// decodeDSB2ConfMessagePack decodes the native MessagePack JSON-string payload of a .dsb2conf file
func decodeDSB2ConfMessagePack(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeJSONStringMessagePack(data, dsb2confPayloadDescriptor)
}

// EncodeDSB2Conf 编码 .dsb2conf 的长度前缀 LZ4 MessagePack JSON 字符串载荷
// EncodeDSB2Conf encodes the length-prefixed LZ4 MessagePack JSON-string payload of a .dsb2conf file
func EncodeDSB2Conf(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeKCESPayloadVariant(env, dsb2confPayloadDescriptor, encodeDSB2ConfMessagePack)
}

// encodeDSB2ConfMessagePack 编码 .dsb2conf 的原生 MessagePack JSON 字符串载荷
// encodeDSB2ConfMessagePack encodes the native MessagePack JSON-string payload of a .dsb2conf file
func encodeDSB2ConfMessagePack(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeJSONStringMessagePack(env, dsb2confPayloadDescriptor)
}
