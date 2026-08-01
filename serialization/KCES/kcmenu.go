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

// EncodeKCMenu 编码单个 Parts.Menu 的 LZ4 MessagePack .kcmenu 数据
// EncodeKCMenu encodes LZ4 MessagePack .kcmenu data containing one Parts.Menu
func EncodeKCMenu(value *Menu) ([]byte, error) {
	return encodeCompressedMsgpack(value, "KCMenu")
}
