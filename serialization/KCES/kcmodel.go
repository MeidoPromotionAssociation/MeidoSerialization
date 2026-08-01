package KCES

// KCModelExtension 是 KCModel 文件扩展名
// KCModelExtension is the KCModel file extension
const KCModelExtension = ".kcmodel"

// DecodeKCModel 解码与 Parts.Model 相同的 LZ4 MessagePack .kcmodel 数据
// DecodeKCModel decodes LZ4 MessagePack .kcmodel data that uses the Parts.Model layout
func DecodeKCModel(data []byte) (*Model, error) {
	return DecodeModel(data)
}

// EncodeKCModel 编码与 Parts.Model 相同的 LZ4 MessagePack .kcmodel 数据
// EncodeKCModel encodes LZ4 MessagePack .kcmodel data that uses the Parts.Model layout
func EncodeKCModel(value *Model) ([]byte, error) {
	return EncodeModel(value)
}
