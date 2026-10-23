package CRC3D3

import (
	"fmt"
	"os"

	serializationCRC3D3 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/CRC3D3"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/COM3D2"
)

// GameCRC3D3 是 CRC3D3 在文件类型判定中使用的游戏名
// GameCRC3D3 is the game name CRC3D3 uses in file type detection
const GameCRC3D3 = "CRC3D3"

// NeimpFileType 是 .neimp 在文件类型判定中使用的类型名，与 `--type neimp` 的取值一致
// NeimpFileType is the type name .neimp uses in file type detection and matches the `--type neimp` value
const NeimpFileType = "neimp"

// FileTypeService 提供 CRC3D3 文件的类型判定
// FileTypeService detects CRC3D3 file types
type FileTypeService struct{}

// TryFileTypeDetermine 判定路径是否为 .neimp 或其 JSON，并返回文件信息
// 只有后缀匹配且内容能真正解出 MessagePack 值树时才认领，因此损坏的文件不会被误报成 .neimp
// TryFileTypeDetermine reports whether a path is a .neimp file or its JSON and returns the file information
// A path is claimed only when the suffix matches and the content really decodes into a MessagePack value tree, so a corrupt file is never misreported as .neimp
func (s *FileTypeService) TryFileTypeDetermine(path string) (info COM3D2Service.FileInfo, matched bool, err error) {
	info = COM3D2Service.FileInfo{
		Path:          path,
		FileType:      COM3D2Service.UnknownFileType,
		StorageFormat: COM3D2Service.FormatUnknown,
		Game:          COM3D2Service.UnknowGame,
		Signature:     COM3D2Service.UnknownSignature,
	}
	jsonForm := IsCRC3D3NeimpJSONFile(path)
	if !jsonForm && !IsCRC3D3NeimpFile(path) {
		return info, false, nil
	}

	stat, err := os.Stat(path)
	if err != nil {
		return info, false, err
	}
	if stat.IsDir() {
		return info, false, fmt.Errorf("%q is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return info, false, err
	}

	if jsonForm {
		if _, err := serializationCRC3D3.DecodeNeimpJSON(data); err != nil {
			return info, false, nil
		}
		info.StorageFormat = COM3D2Service.FormatJSON
	} else {
		if _, err := serializationCRC3D3.DecodeNeimp(data); err != nil {
			return info, false, nil
		}
		info.StorageFormat = COM3D2Service.FormatBinary
	}

	info.FileType = NeimpFileType
	info.Game = GameCRC3D3
	// MessagePack 没有固定签名，也没有独立于容器的版本号
	// MessagePack carries no fixed signature and no version apart from the container itself
	info.Signature = COM3D2Service.SignatureNone
	info.Size = stat.Size()
	return info, true, nil
}
