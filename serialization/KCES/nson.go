package KCES

// .nson
// KCES 用于替代部分旧版 .nei/CSV 数据的明文 JSON 资源（nson 是一种二进制编码的 JSON），CsvManager 会直接读取 TextAsset.text
// 编解码器不解释业务字段，只验证并保留规范化后的 JSON 语义内容
// .nson
// Plain-JSON KCES resource replacing some legacy .nei/CSV data (nson is a binary-encoded form of JSON); CsvManager reads TextAsset.text directly
// The codec does not interpret domain fields and retains only validated normalized JSON semantics

const KCESNSONExtension = ".nson"

var nsonJSONTextDescriptor = kcesJSONTextDescriptor{
	Extension: KCESNSONExtension,
}
