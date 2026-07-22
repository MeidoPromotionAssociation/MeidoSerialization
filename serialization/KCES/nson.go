package KCES

// .nson
// KCES 用于替代部分旧版 .nei/CSV 数据的明文 JSON 资源，CsvManager 会直接读取 TextAsset.text。
// 编解码器不解释业务字段，只验证 JSON 并无损保留未修改的原始文本。
//
// .nson
// Plain-JSON KCES resource replacing some legacy .nei/CSV data; CsvManager reads TextAsset.text directly.
// The codec does not interpret domain fields: it validates JSON and losslessly preserves unchanged source text.

const KCESNSONExtension = ".nson"

var nsonJSONTextDescriptor = kcesJSONTextDescriptor{
	Extension: KCESNSONExtension,
}
