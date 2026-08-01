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

// EncodeKCMat 编码单个 Parts.Material 的 LZ4 MessagePack .kcmat 数据
// EncodeKCMat encodes LZ4 MessagePack .kcmat data containing one Parts.Material
func EncodeKCMat(value *Material) ([]byte, error) {
	return encodeCompressedMsgpack(value, "KCMat")
}
