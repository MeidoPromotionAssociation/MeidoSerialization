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
