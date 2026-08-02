package KCES

// KCModelExtension 是 KCModel 文件扩展名
// KCModelExtension is the KCModel file extension
const KCModelExtension = ".kcmodel"

// DecodeKCModel 解码与 Parts.Model 相同的 LZ4 MessagePack .kcmodel 数据
// DecodeKCModel decodes LZ4 MessagePack .kcmodel data that uses the Parts.Model layout
func DecodeKCModel(data []byte) (*Model, error) {
	return DecodeModel(data)
}

// EncodeKCModel 编码与 Parts.Model 相同的 LZ4 MessagePack .kcmodel 数据，并默认重算可确定的查找字段
// EncodeKCModel encodes LZ4 MessagePack .kcmodel data that uses the Parts.Model layout and recalculates determinable lookup fields by default
func EncodeKCModel(value *Model) ([]byte, error) {
	return EncodeKCModelWithOptions(value, nil)
}

// EncodeKCModelWithOptions 编码与 Parts.Model 相同的 LZ4 MessagePack .kcmodel 数据，并允许显式关闭或按输出文件名执行查找字段重算
// EncodeKCModelWithOptions encodes .kcmodel data using the Parts.Model layout and allows lookup-field recalculation to be explicitly disabled or performed from the output filename
func EncodeKCModelWithOptions(value *Model, options *LookupHashOptions) ([]byte, error) {
	return EncodeModelWithOptions(value, options)
}
