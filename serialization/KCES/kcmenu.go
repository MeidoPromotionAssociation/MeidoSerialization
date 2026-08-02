package KCES

// KCMenuExtension 是 KCMenu 文件扩展名
// KCMenuExtension is the KCMenu file extension
const KCMenuExtension = ".kcmenu"

// DecodeKCMenu 解码单个 Parts.Menu 的 LZ4 MessagePack .kcmenu 数据
// DecodeKCMenu decodes LZ4 MessagePack .kcmenu data containing one Parts.Menu
func DecodeKCMenu(data []byte) (*Menu, error) {
	var value *Menu
	if err := decodeCompressedMsgpack(data, &value, "KCMenu"); err != nil {
		return nil, err
	}
	return value, nil
}

// EncodeKCMenu 编码单个 Parts.Menu 的 LZ4 MessagePack .kcmenu 数据，并默认重算可确定的查找字段
// EncodeKCMenu encodes LZ4 MessagePack .kcmenu data containing one Parts.Menu and recalculates determinable lookup fields by default
func EncodeKCMenu(value *Menu) ([]byte, error) {
	return EncodeKCMenuWithOptions(value, nil)
}

// EncodeKCMenuWithOptions 编码单个 Parts.Menu 的 LZ4 MessagePack .kcmenu 数据，并允许显式关闭或按输出文件名执行查找字段重算
// EncodeKCMenuWithOptions encodes LZ4 MessagePack .kcmenu data containing one Parts.Menu and allows lookup-field recalculation to be explicitly disabled or performed from the output filename
func EncodeKCMenuWithOptions(value *Menu, options *LookupHashOptions) ([]byte, error) {
	return encodeCompressedMsgpack(cloneMenuForEncoding(value, options, true), "KCMenu")
}
