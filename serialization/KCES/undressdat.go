package KCES

// .undressdat
// KCES 脱衣规则的明文 JSON 资源。编解码时保留原始 UTF-8 文本；只有规范化 JSON 内容发生修改时，
// 才重新生成带两个空格缩进和末尾换行的文本
// .undressdat
// Plain-JSON KCES undressing-rule resource. The codec preserves the original UTF-8 text and regenerates it
// with two-space indentation and a trailing newline only when the normalized JSON content changes

const KCESUndressDataExtension = ".undressdat"

var undressdatJSONTextDescriptor = kcesJSONTextDescriptor{
	Extension: KCESUndressDataExtension,
}
