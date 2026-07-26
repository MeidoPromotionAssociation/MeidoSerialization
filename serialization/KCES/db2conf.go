package KCES

// .db2conf
// DynamicBone 的新版配置文件。载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的 MessagePack 字符串，
// 字符串内容是游戏交给 Unity JSON 系统处理的配置文档
// .db2conf
// Newer DynamicBone configuration file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed MessagePack string containing the configuration document consumed by Unity JSON

const KCESDB2ConfExtension = ".db2conf"

var db2confPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDB2ConfExtension,
	Kind:           PayloadKindJSONString,
	LengthPrefixed: true,
}

// DecodeDB2Conf 解码 .db2conf 的长度前缀 LZ4 MessagePack JSON 字符串载荷
// DecodeDB2Conf decodes the length-prefixed LZ4 MessagePack JSON-string payload of a .db2conf file
func DecodeDB2Conf(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeKCESPayloadVariants(data, db2confPayloadDescriptor, decodeDB2ConfMessagePack)
}

// decodeDB2ConfMessagePack 解码 .db2conf 的原生 MessagePack JSON 字符串载荷
// decodeDB2ConfMessagePack decodes the native MessagePack JSON-string payload of a .db2conf file
func decodeDB2ConfMessagePack(data []byte) (*KCESPayloadEnvelope, error) {
	return decodeJSONStringMessagePack(data, db2confPayloadDescriptor)
}

// EncodeDB2Conf 编码 .db2conf 的长度前缀 LZ4 MessagePack JSON 字符串载荷
// EncodeDB2Conf encodes the length-prefixed LZ4 MessagePack JSON-string payload of a .db2conf file
func EncodeDB2Conf(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeKCESPayloadVariant(env, db2confPayloadDescriptor, encodeDB2ConfMessagePack)
}

// encodeDB2ConfMessagePack 编码 .db2conf 的原生 MessagePack JSON 字符串载荷
// encodeDB2ConfMessagePack encodes the native MessagePack JSON-string payload of a .db2conf file
func encodeDB2ConfMessagePack(env *KCESPayloadEnvelope) ([]byte, error) {
	return encodeJSONStringMessagePack(env, db2confPayloadDescriptor)
}
