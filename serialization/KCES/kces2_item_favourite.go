package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

const (
	// KCES2ItemFavouriteStateSettingFileName 是场景编辑保存目录中的虚拟文件名
	// KCES2ItemFavouriteStateSettingFileName is the virtual filename in the scene-edit save directory
	KCES2ItemFavouriteStateSettingFileName = "KCES2Edit::ItemFavouriteStateSetting"
	// KCES2ItemFavouriteStateSettingPath 是 system.dat 中收藏状态虚拟文件的完整路径
	// KCES2ItemFavouriteStateSettingPath is the complete path of the favourite-state virtual file in system.dat
	KCES2ItemFavouriteStateSettingPath = "EditData/" + KCES2ItemFavouriteStateSettingFileName
)

// KCES2ItemFavouriteStateSetting 对应 KCES2 收藏状态虚拟文件中的未压缩 MessagePack 列表 / KCES2ItemFavouriteStateSetting matches the uncompressed MessagePack list in the KCES2 favourite-state virtual file
type KCES2ItemFavouriteStateSetting []*KCES2ItemFavouriteStateData

// KCES2ItemFavouriteStateData 对应 KCES2ItemFavouriteManager.ItemFavouriteStateData 的两槽记录 / KCES2ItemFavouriteStateData matches the two-slot KCES2ItemFavouriteManager.ItemFavouriteStateData record
type KCES2ItemFavouriteStateData struct {
	_struct                  struct{} `codec:",toarray"`            // 强制按数组编码 / Forces array encoding
	ItemFileName             *string  `json:"itemFileName"`         // 可空菜单文件名 / Nullable menu filename
	ItemFavouriteStateString *string  `json:"favouriteStateString"` // 可空原始状态字符串，编解码器不会解析或过滤 / Nullable raw state string which the codec does not parse or filter
}

// DecodeKCES2ItemFavouriteStateSetting 解码收藏状态虚拟文件中的未压缩 MessagePack 列表
// DecodeKCES2ItemFavouriteStateSetting decodes the uncompressed MessagePack list stored in the favourite-state virtual file
func DecodeKCES2ItemFavouriteStateSetting(data []byte) (KCES2ItemFavouriteStateSetting, error) {
	var value KCES2ItemFavouriteStateSetting
	if err := msgpack.DecodeMsgpack(data, &value); err != nil {
		return nil, fmt.Errorf("decode KCES2 item favourite state setting msgpack: %w", err)
	}
	return value, nil
}

// EncodeKCES2ItemFavouriteStateSetting 编码收藏状态虚拟文件并原样保留状态字符串和 nil 项
// EncodeKCES2ItemFavouriteStateSetting encodes the favourite-state virtual file while preserving raw state strings and nil entries
func EncodeKCES2ItemFavouriteStateSetting(value KCES2ItemFavouriteStateSetting) ([]byte, error) {
	encoded, err := msgpack.EncodeMsgpack(value)
	if err != nil {
		return nil, fmt.Errorf("encode KCES2 item favourite state setting msgpack: %w", err)
	}
	return encoded, nil
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 KCES2ItemFavouriteStateData
// CodecEncodeSelf encodes KCES2ItemFavouriteStateData using the shared indexed-object rules
func (v KCES2ItemFavouriteStateData) CodecEncodeSelf(e *codec.Encoder) {
	msgpack.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 KCES2ItemFavouriteStateData
// CodecDecodeSelf decodes KCES2ItemFavouriteStateData using the shared indexed-object rules
func (v *KCES2ItemFavouriteStateData) CodecDecodeSelf(d *codec.Decoder) {
	msgpack.DecodeIndexedObjectSelf(d, v)
}
