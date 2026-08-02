package KCES

// KCMatExtension 是 KCMat 文件扩展名
// KCMatExtension is the KCMat file extension
const KCMatExtension = ".kcmat"

// DecodeKCMat 解码单个 Parts.Material 的 LZ4 MessagePack .kcmat 数据
// DecodeKCMat decodes LZ4 MessagePack .kcmat data containing one Parts.Material
func DecodeKCMat(data []byte) (*Material, error) {
	var value *Material
	if err := decodeCompressedMsgpack(data, &value, "KCMat"); err != nil {
		return nil, err
	}
	return value, nil
}

// EncodeKCMat 编码单个 Parts.Material 的 LZ4 MessagePack .kcmat 数据，并默认重算可确定的查找字段
// EncodeKCMat encodes LZ4 MessagePack .kcmat data containing one Parts.Material and recalculates determinable lookup fields by default
func EncodeKCMat(value *Material) ([]byte, error) {
	return EncodeKCMatWithOptions(value, nil)
}

// EncodeKCMatWithOptions 编码单个 Parts.Material 的 LZ4 MessagePack .kcmat 数据，并允许显式关闭或按输出文件名执行查找字段重算
// EncodeKCMatWithOptions encodes LZ4 MessagePack .kcmat data containing one Parts.Material and allows lookup-field recalculation to be explicitly disabled or performed from the output filename
func EncodeKCMatWithOptions(value *Material, options *LookupHashOptions) ([]byte, error) {
	return encodeCompressedMsgpack(cloneMaterialForEncoding(value, options, true), "KCMat")
}
