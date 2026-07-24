package KCES

// .undressdat
// KCES 脱衣规则的明文 JSON 资源，编解码只保留规范化 JSON 语义
// .undressdat
// Plain-JSON KCES undressing-rule resource whose codec retains only normalized JSON semantics

const KCESUndressDataExtension = ".undressdat"

var undressdatJSONTextDescriptor = kcesJSONTextDescriptor{
	Extension: KCESUndressDataExtension,
}
