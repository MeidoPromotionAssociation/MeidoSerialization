package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

const (
	// KCMetaDataExtension 是 KCMetaData 文件扩展名
	// KCMetaDataExtension is the KCMetaData file extension
	KCMetaDataExtension        = ".kcmmeta"
	kcMetaDataFixVersion int32 = 10001
)

// KCMetaData 对应 ExportKCES.KCMetaData 的九槽未压缩 MessagePack 元数据 / KCMetaData matches the nine-slot uncompressed MessagePack metadata in ExportKCES.KCMetaData
type KCMetaData struct {
	_struct                         struct{}  `codec:",toarray"`                       // 强制按数组编码 / Forces array encoding
	Version                         int32     `json:"version"`                         // 导出元数据版本，当前 FixVersion 为 10001 / Export metadata version whose current FixVersion is 10001
	ModelFileName                   *string   `json:"modelFileName"`                   // 可空 KCModel 文件名 / Nullable KCModel filename
	FormTextureFileName             *string   `json:"formTextureFileName"`             // 可空形态纹理 KCTex 文件名 / Nullable form-texture KCTex filename
	MaterialPerOriginalMenuFileName []*string `json:"materialPerOriginalMenuFileName"` // 各材质对应的原始菜单文件名 / Original menu filenames corresponding to each material
	BuildVersion                    int32     `json:"buildVersion"`                    // 导出时记录的构建版本 / Build version recorded during export
	GameVersion                     int32     `json:"gameVersion"`                     // 导出时记录的游戏版本 / Game version recorded during export
	GUID                            *string   `json:"guid"`                            // 可空分享模型 GUID / Nullable shared-model GUID
	SuspendedSaveFileName           *string   `json:"suspendedSaveFileName"`           // 可空继续编辑 HairMake 存档文件名 / Nullable HairMake save filename used to continue editing
	MaterialPerOriginalMenuVersion  []int32   `json:"materialPerOriginalMenuVersion"`  // 各原始菜单的部件版本 / Parts versions of the original menus
}

// DecodeKCMetaData 解码未压缩的 .kcmmeta MessagePack 数据
// DecodeKCMetaData decodes uncompressed .kcmmeta MessagePack data
func DecodeKCMetaData(data []byte) (*KCMetaData, error) {
	var value *KCMetaData
	if err := msgpack.DecodeMsgpack(data, &value); err != nil {
		return nil, fmt.Errorf("decode KCMetaData msgpack: %w", err)
	}
	return value, nil
}

// EncodeKCMetaData 编码未压缩的 .kcmmeta MessagePack 数据并保留调用方版本
// EncodeKCMetaData encodes uncompressed .kcmmeta MessagePack data while preserving the caller version
func EncodeKCMetaData(value *KCMetaData) ([]byte, error) {
	return encodeUncompressedIndexedMsgpack(value, "KCMetaData")
}

// NewKCMetaData 创建当前 10001 版本的新 KCMetaData
// NewKCMetaData creates new KCMetaData using the current version 10001
func NewKCMetaData() *KCMetaData {
	return &KCMetaData{Version: kcMetaDataFixVersion}
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 KCMetaData
// CodecEncodeSelf encodes KCMetaData using the shared indexed-object rules
func (v KCMetaData) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 KCMetaData
// CodecDecodeSelf decodes KCMetaData using the shared indexed-object rules
func (v *KCMetaData) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }
