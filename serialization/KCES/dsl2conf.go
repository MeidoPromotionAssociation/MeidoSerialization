package KCES

// .dsl2conf
// DynamicSleeveBone 的新版配置文件。载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// MessagePack JSON 字符串。
//
// .dsl2conf
// Newer DynamicSleeveBone configuration file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed MessagePack JSON string.

const KCESDSL2ConfExtension = ".dsl2conf"

var dsl2confPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDSL2ConfExtension,
	Kind:           PayloadKindJSONString,
	LengthPrefixed: true,
}
