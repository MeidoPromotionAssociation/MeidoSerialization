package KCES

// .dsb2conf
// DynamicSkirtBone 的新版配置文件。载荷为 Int32 长度前缀加 LZ4 Block Array 压缩的
// MessagePack JSON 字符串。
//
// .dsb2conf
// Newer DynamicSkirtBone configuration file. The payload is an Int32 length prefix followed by an
// LZ4 Block Array-compressed MessagePack JSON string.

const KCESDSB2ConfExtension = ".dsb2conf"

var dsb2confPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESDSB2ConfExtension,
	Kind:           PayloadKindJSONString,
	LengthPrefixed: true,
}
