package KCES

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"unicode/utf8"
)

const (
	// SharePresetExtension 是 KCES2 分享预设文件扩展名
	// SharePresetExtension is the KCES2 shared-preset file extension
	SharePresetExtension = ".shpreset"
	// SharePresetSignature 是分享预设文件的七字节签名
	// SharePresetSignature is the seven-byte shared-preset file signature
	SharePresetSignature = "SH_PRES"
	// SharePresetVersion 是 KCES2 当前写出的分享预设容器版本
	// SharePresetVersion is the shared-preset container version currently written by KCES2
	SharePresetVersion uint32 = 2

	sharePresetHeaderSize uint32 = 19
)

// SharePreset 对应 ExportSharePreset 写出的分享容器 / SharePreset matches the sharing container written by ExportSharePreset
type SharePreset struct {
	Signature            string                  `json:"signature"`                      // 七字节 SH_PRES 签名 / Seven-byte SH_PRES signature
	Version              uint32                  `json:"version"`                        // 容器版本，KCES2 当前写入 2 / Container version currently written as 2 by KCES2
	ExportData           SharePresetExportData   `json:"exportData"`                     // 分享页面使用的角色摘要元数据 / Character-summary metadata used by the sharing page
	PresetData           []byte                  `json:"presetData"`                     // 完整内嵌 KCES .preset 字节 / Complete embedded KCES .preset bytes
	BaseThumbnail        *SharePresetThumbnail   `json:"baseThumbnail,omitempty"`        // 可空预设基础缩略图 / Nullable base preset thumbnail
	AdditionalThumbnails []*SharePresetThumbnail `json:"additionalThumbnails,omitempty"` // 可空附加截图数组 / Nullable additional screenshot array
	AppendedData         []byte                  `json:"appendedData,omitempty"`         // 主元数据之后由分享服务追加的原始字节 / Raw bytes appended by the sharing service after the primary metadata
}

// SharePresetExportData 对应 ExportSharePreset.ExportPresetData 中不含文件位置的角色摘要 / SharePresetExportData matches the character summary in ExportSharePreset.ExportPresetData without file positions
type SharePresetExportData struct {
	Gender      *string                    `json:"gender"`                // 可空性别文本，游戏写入 maid 或 man / Nullable gender text written by the game as maid or man
	Height      int32                      `json:"height"`                // 身高摘要值 / Height summary value
	Weight      int32                      `json:"weight"`                // 体重摘要值 / Weight summary value
	Bust        int32                      `json:"bust"`                  // 胸围摘要值 / Bust summary value
	Waist       int32                      `json:"waist"`                 // 腰围摘要值 / Waist summary value
	Hip         int32                      `json:"hip"`                   // 臀围摘要值 / Hip summary value
	ItemMPNs    map[string][]*string       `json:"itemMpns"`              // 各物品型 MPN 的可空菜单文件名数组 / Nullable menu-filename arrays for item MPNs
	ParamMPNs   map[string]int32           `json:"paramMpns"`             // 各参数型 MPN 的 Int32 值 / Int32 values for parameter MPNs
	ExtraFields map[string]json.RawMessage `json:"extraFields,omitempty"` // 当前源码未声明但需往返保留的元数据字段 / Metadata fields not declared by the current source but preserved during round trips
}

// SharePresetThumbnail 表示分享预设中带原始扩展名的一个图像载荷 / SharePresetThumbnail represents one image payload with its original extension in a shared preset
type SharePresetThumbnail struct {
	Extension *string `json:"extension"` // 可空且不带点的原始扩展名 / Nullable original extension without a leading dot
	Data      []byte  `json:"data"`      // 原始图像字节 / Raw image bytes
}

// sharePresetDataPosition 对应 Newtonsoft.Json 写出的 Tuple<UInt32, UInt32> / sharePresetDataPosition matches Tuple<UInt32, UInt32> written by Newtonsoft.Json
type sharePresetDataPosition struct {
	Offset uint32 `json:"Item1"` // 文件偏移，线格式元组键为 Item1 / File offset whose wire tuple key is Item1
	Length uint32 `json:"Item2"` // 字节长度，线格式元组键为 Item2 / Byte length whose wire tuple key is Item2
}

// sharePresetThumbnailPosition 对应 Newtonsoft.Json 写出的 Tuple<UInt32, UInt32, String> / sharePresetThumbnailPosition matches Tuple<UInt32, UInt32, String> written by Newtonsoft.Json
type sharePresetThumbnailPosition struct {
	Offset    uint32  `json:"Item1"` // 文件偏移，线格式元组键为 Item1 / File offset whose wire tuple key is Item1
	Length    uint32  `json:"Item2"` // 字节长度，线格式元组键为 Item2 / Byte length whose wire tuple key is Item2
	Extension *string `json:"Item3"` // 可空原始扩展名，线格式元组键为 Item3 / Nullable original extension whose wire tuple key is Item3
}

// sharePresetWireMetadata 对应 ExportSharePreset.ExportPresetData 的完整 JSON 布局 / sharePresetWireMetadata matches the complete JSON layout of ExportSharePreset.ExportPresetData
type sharePresetWireMetadata struct {
	Gender                       *string                         `json:"gender"`        // 可空性别文本 / Nullable gender text
	Height                       int32                           `json:"height"`        // 身高摘要值 / Height summary value
	Weight                       int32                           `json:"weight"`        // 体重摘要值 / Weight summary value
	Bust                         int32                           `json:"bust"`          // 胸围摘要值 / Bust summary value
	Waist                        int32                           `json:"weist"`         // 腰围摘要值，线格式键保留游戏的 weist 拼写 / Waist summary value whose wire key retains the game's weist spelling
	Hip                          int32                           `json:"hip"`           // 臀围摘要值 / Hip summary value
	ItemMPNs                     map[string][]*string            `json:"itemMpns"`      // 物品型 MPN 文件名表 / Item-MPN filename table
	ParameterMPNs                map[string]int32                `json:"paramMpns"`     // 参数型 MPN 数值表，线格式键为 paramMpns / Parameter-MPN value table whose wire key is paramMpns
	PresetDataPosition           *sharePresetDataPosition        `json:"presetDataPos"` // 内嵌预设位置，线格式键为 presetDataPos / Embedded-preset position whose wire key is presetDataPos
	BaseThumbnailPosition        *sharePresetThumbnailPosition   `json:"thumBasePos"`   // 可空基础缩略图位置，线格式键为 thumBasePos / Nullable base-thumbnail position whose wire key is thumBasePos
	AdditionalThumbnailPositions []*sharePresetThumbnailPosition `json:"thumAddPos"`    // 可空附加截图位置数组，线格式键为 thumAddPos / Nullable additional-thumbnail positions whose wire key is thumAddPos
}

// DecodeSharePreset 解码 SH_PRES 容器并保留版本、全部图像、未知元数据字段和服务端追加字节
// DecodeSharePreset decodes an SH_PRES container while preserving its version, every image, unknown metadata fields, and server-appended bytes
func DecodeSharePreset(data []byte) (*SharePreset, error) {
	if uint64(len(data)) < uint64(sharePresetHeaderSize) {
		return nil, fmt.Errorf("SharePreset data is too short: %d bytes", len(data))
	}
	if string(data[:len(SharePresetSignature)]) != SharePresetSignature {
		return nil, fmt.Errorf("unsupported SharePreset signature %q", string(data[:len(SharePresetSignature)]))
	}
	version := binary.LittleEndian.Uint32(data[7:11])
	metadataOffset := binary.LittleEndian.Uint32(data[11:15])
	metadataLength := binary.LittleEndian.Uint32(data[15:19])
	metadataEnd := uint64(metadataOffset) + uint64(metadataLength)
	if metadataOffset < sharePresetHeaderSize || metadataEnd > uint64(len(data)) {
		return nil, fmt.Errorf("SharePreset metadata range [%d,%d) exceeds %d bytes", metadataOffset, metadataEnd, len(data))
	}
	metadataBytes := data[int(metadataOffset):int(metadataEnd)]
	if !utf8.Valid(metadataBytes) {
		return nil, fmt.Errorf("SharePreset metadata is not valid UTF-8")
	}
	var metadata sharePresetWireMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("decode SharePreset metadata JSON: %w", err)
	}
	if metadata.PresetDataPosition == nil {
		return nil, fmt.Errorf("SharePreset metadata presetDataPos is required")
	}

	expectedOffset := sharePresetHeaderSize
	presetData, nextOffset, err := sharePresetReadRegion(data, metadataOffset, metadata.PresetDataPosition.Offset, metadata.PresetDataPosition.Length, expectedOffset, "presetDataPos")
	if err != nil {
		return nil, err
	}
	expectedOffset = nextOffset

	var baseThumbnail *SharePresetThumbnail
	if metadata.BaseThumbnailPosition != nil {
		thumbnailData, nextOffset, err := sharePresetReadRegion(data, metadataOffset, metadata.BaseThumbnailPosition.Offset, metadata.BaseThumbnailPosition.Length, expectedOffset, "thumBasePos")
		if err != nil {
			return nil, err
		}
		expectedOffset = nextOffset
		baseThumbnail = &SharePresetThumbnail{Extension: cloneSharePresetString(metadata.BaseThumbnailPosition.Extension), Data: thumbnailData}
	}

	var additionalThumbnails []*SharePresetThumbnail
	if metadata.AdditionalThumbnailPositions != nil {
		additionalThumbnails = make([]*SharePresetThumbnail, len(metadata.AdditionalThumbnailPositions))
		for index, position := range metadata.AdditionalThumbnailPositions {
			if position == nil {
				return nil, fmt.Errorf("SharePreset metadata thumAddPos[%d] is null", index)
			}
			thumbnailData, nextOffset, err := sharePresetReadRegion(data, metadataOffset, position.Offset, position.Length, expectedOffset, fmt.Sprintf("thumAddPos[%d]", index))
			if err != nil {
				return nil, err
			}
			expectedOffset = nextOffset
			additionalThumbnails[index] = &SharePresetThumbnail{Extension: cloneSharePresetString(position.Extension), Data: thumbnailData}
		}
	}
	if expectedOffset != metadataOffset {
		return nil, fmt.Errorf("SharePreset payload regions end at %d, metadata starts at %d", expectedOffset, metadataOffset)
	}

	extraFields, err := sharePresetExtraMetadataFields(metadataBytes)
	if err != nil {
		return nil, err
	}
	return &SharePreset{
		Signature: SharePresetSignature,
		Version:   version,
		ExportData: SharePresetExportData{
			Gender: cloneSharePresetString(metadata.Gender), Height: metadata.Height, Weight: metadata.Weight,
			Bust: metadata.Bust, Waist: metadata.Waist, Hip: metadata.Hip,
			ItemMPNs: cloneSharePresetItemMPNs(metadata.ItemMPNs), ParamMPNs: cloneSharePresetParameterMPNs(metadata.ParameterMPNs), ExtraFields: extraFields,
		},
		PresetData:           presetData,
		BaseThumbnail:        baseThumbnail,
		AdditionalThumbnails: additionalThumbnails,
		AppendedData:         append([]byte(nil), data[int(metadataEnd):]...),
	}, nil
}

// EncodeSharePreset 编码 SH_PRES 容器并原样写出调用方版本、载荷和服务端追加字节
// EncodeSharePreset encodes an SH_PRES container while emitting the caller-supplied version, payloads, and server-appended bytes unchanged
func EncodeSharePreset(value *SharePreset) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil SharePreset")
	}
	if value.Signature != SharePresetSignature {
		return nil, fmt.Errorf("unsupported SharePreset signature %q", value.Signature)
	}
	if err := validateSharePresetStrings(value); err != nil {
		return nil, err
	}

	out := make([]byte, sharePresetHeaderSize)
	copy(out, SharePresetSignature)
	binary.LittleEndian.PutUint32(out[7:11], value.Version)
	expectedOffset := uint64(sharePresetHeaderSize)
	presetPosition, nextOffset, err := sharePresetAppendRegion(&out, value.PresetData, expectedOffset, "presetData")
	if err != nil {
		return nil, err
	}
	expectedOffset = nextOffset

	var basePosition *sharePresetThumbnailPosition
	if value.BaseThumbnail != nil {
		position, nextOffset, err := sharePresetAppendRegion(&out, value.BaseThumbnail.Data, expectedOffset, "baseThumbnail")
		if err != nil {
			return nil, err
		}
		expectedOffset = nextOffset
		basePosition = &sharePresetThumbnailPosition{Offset: position.Offset, Length: position.Length, Extension: cloneSharePresetString(value.BaseThumbnail.Extension)}
	}

	var additionalPositions []*sharePresetThumbnailPosition
	if value.AdditionalThumbnails != nil {
		additionalPositions = make([]*sharePresetThumbnailPosition, len(value.AdditionalThumbnails))
		for index, thumbnail := range value.AdditionalThumbnails {
			if thumbnail == nil {
				return nil, fmt.Errorf("SharePreset additionalThumbnails[%d] is nil", index)
			}
			position, nextOffset, err := sharePresetAppendRegion(&out, thumbnail.Data, expectedOffset, fmt.Sprintf("additionalThumbnails[%d]", index))
			if err != nil {
				return nil, err
			}
			expectedOffset = nextOffset
			additionalPositions[index] = &sharePresetThumbnailPosition{Offset: position.Offset, Length: position.Length, Extension: cloneSharePresetString(thumbnail.Extension)}
		}
	}
	metadata := sharePresetWireMetadata{
		Gender: cloneSharePresetString(value.ExportData.Gender), Height: value.ExportData.Height, Weight: value.ExportData.Weight,
		Bust: value.ExportData.Bust, Waist: value.ExportData.Waist, Hip: value.ExportData.Hip,
		ItemMPNs: cloneSharePresetItemMPNs(value.ExportData.ItemMPNs), ParameterMPNs: cloneSharePresetParameterMPNs(value.ExportData.ParamMPNs),
		PresetDataPosition: presetPosition, BaseThumbnailPosition: basePosition, AdditionalThumbnailPositions: additionalPositions,
	}
	metadataJSON, err := encodeSharePresetMetadata(metadata, value.ExportData.ExtraFields)
	if err != nil {
		return nil, err
	}
	if expectedOffset > math.MaxUint32 || uint64(len(metadataJSON)) > math.MaxUint32 {
		return nil, fmt.Errorf("SharePreset metadata offset or length exceeds UInt32")
	}
	binary.LittleEndian.PutUint32(out[11:15], uint32(expectedOffset))
	binary.LittleEndian.PutUint32(out[15:19], uint32(len(metadataJSON)))
	if uint64(len(out))+uint64(len(metadataJSON))+uint64(len(value.AppendedData)) > uint64(gameInt32Max) {
		return nil, fmt.Errorf("SharePreset total length exceeds the C# byte-array limit")
	}
	out = append(out, metadataJSON...)
	out = append(out, value.AppendedData...)
	return out, nil
}

// NewSharePreset 创建使用当前签名和版本且带空 MPN 字典的新分享预设
// NewSharePreset creates a new shared preset using the current signature and version with empty MPN maps
func NewSharePreset() *SharePreset {
	return &SharePreset{
		Signature: SharePresetSignature,
		Version:   SharePresetVersion,
		ExportData: SharePresetExportData{
			ItemMPNs:  make(map[string][]*string),
			ParamMPNs: make(map[string]int32),
		},
	}
}

// sharePresetReadRegion 按游戏写出的连续顺序复制一个 UInt32 位置元组引用的载荷
// sharePresetReadRegion copies one payload referenced by a UInt32 position tuple in the contiguous order written by the game
func sharePresetReadRegion(data []byte, metadataOffset, offset, length, expectedOffset uint32, path string) ([]byte, uint32, error) {
	if offset != expectedOffset {
		return nil, 0, fmt.Errorf("SharePreset metadata %s offset is %d, expected contiguous offset %d", path, offset, expectedOffset)
	}
	end := uint64(offset) + uint64(length)
	if end > uint64(metadataOffset) {
		return nil, 0, fmt.Errorf("SharePreset metadata %s range [%d,%d) overlaps metadata at %d", path, offset, end, metadataOffset)
	}
	return append([]byte(nil), data[int(offset):int(end)]...), uint32(end), nil
}

// sharePresetAppendRegion 追加一个载荷并返回可写入 JSON 的 UInt32 位置元组
// sharePresetAppendRegion appends one payload and returns its UInt32 position tuple for JSON metadata
func sharePresetAppendRegion(out *[]byte, data []byte, offset uint64, path string) (*sharePresetDataPosition, uint64, error) {
	if offset > math.MaxUint32 || uint64(len(data)) > math.MaxUint32 || offset+uint64(len(data)) > math.MaxUint32 {
		return nil, 0, fmt.Errorf("SharePreset %s offset or length exceeds UInt32", path)
	}
	position := &sharePresetDataPosition{Offset: uint32(offset), Length: uint32(len(data))}
	*out = append(*out, data...)
	return position, offset + uint64(len(data)), nil
}

// encodeSharePresetMetadata 合并当前已知字段与未知字段并按游戏缩进风格编码 UTF-8 JSON
// encodeSharePresetMetadata merges current known fields with unknown fields and encodes UTF-8 JSON using the game's indentation style
func encodeSharePresetMetadata(metadata sharePresetWireMetadata, extraFields map[string]json.RawMessage) ([]byte, error) {
	knownJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("encode SharePreset metadata fields: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(knownJSON, &fields); err != nil {
		return nil, fmt.Errorf("prepare SharePreset metadata fields: %w", err)
	}
	for key, raw := range extraFields {
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("SharePreset extra metadata field %q conflicts with a known field", key)
		}
		if !utf8.ValidString(key) || !json.Valid(raw) {
			return nil, fmt.Errorf("SharePreset extra metadata field %q is not valid UTF-8 JSON", key)
		}
		fields[key] = append(json.RawMessage(nil), raw...)
	}
	encoded, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode SharePreset metadata JSON: %w", err)
	}
	return bytes.ReplaceAll(encoded, []byte("\n"), []byte("\r\n")), nil
}

// sharePresetExtraMetadataFields 复制 ExportPresetData 当前未声明的 JSON 字段
// sharePresetExtraMetadataFields copies JSON fields not currently declared by ExportPresetData
func sharePresetExtraMetadataFields(data []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("decode SharePreset metadata fields: %w", err)
	}
	for _, key := range []string{"gender", "height", "weight", "bust", "weist", "hip", "itemMpns", "paramMpns", "presetDataPos", "thumBasePos", "thumAddPos"} {
		delete(fields, key)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	for key, raw := range fields {
		fields[key] = append(json.RawMessage(nil), raw...)
	}
	return fields, nil
}

// validateSharePresetStrings 拒绝编码时会被 JSON 隐式替换的无效 UTF-8 文本
// validateSharePresetStrings rejects invalid UTF-8 text that JSON encoding would otherwise replace implicitly
func validateSharePresetStrings(value *SharePreset) error {
	if value.ExportData.Gender != nil && !utf8.ValidString(*value.ExportData.Gender) {
		return fmt.Errorf("SharePreset exportData.gender is not valid UTF-8")
	}
	for key, names := range value.ExportData.ItemMPNs {
		if !utf8.ValidString(key) {
			return fmt.Errorf("SharePreset exportData.itemMpns contains an invalid UTF-8 key")
		}
		for index, name := range names {
			if name != nil && !utf8.ValidString(*name) {
				return fmt.Errorf("SharePreset exportData.itemMpns[%q][%d] is not valid UTF-8", key, index)
			}
		}
	}
	for key := range value.ExportData.ParamMPNs {
		if !utf8.ValidString(key) {
			return fmt.Errorf("SharePreset exportData.paramMpns contains an invalid UTF-8 key")
		}
	}
	if value.BaseThumbnail != nil && value.BaseThumbnail.Extension != nil && !utf8.ValidString(*value.BaseThumbnail.Extension) {
		return fmt.Errorf("SharePreset baseThumbnail.extension is not valid UTF-8")
	}
	for index, thumbnail := range value.AdditionalThumbnails {
		if thumbnail != nil && thumbnail.Extension != nil && !utf8.ValidString(*thumbnail.Extension) {
			return fmt.Errorf("SharePreset additionalThumbnails[%d].extension is not valid UTF-8", index)
		}
	}
	return nil
}

// cloneSharePresetString 复制可空字符串指针
// cloneSharePresetString copies a nullable string pointer
func cloneSharePresetString(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

// cloneSharePresetItemMPNs 深复制可空物品 MPN 字典及其字符串指针
// cloneSharePresetItemMPNs deep-copies a nullable item-MPN map and its string pointers
func cloneSharePresetItemMPNs(value map[string][]*string) map[string][]*string {
	if value == nil {
		return nil
	}
	result := make(map[string][]*string, len(value))
	for key, names := range value {
		if names == nil {
			result[key] = nil
			continue
		}
		copiedNames := make([]*string, len(names))
		for index, name := range names {
			copiedNames[index] = cloneSharePresetString(name)
		}
		result[key] = copiedNames
	}
	return result
}

// cloneSharePresetParameterMPNs 复制可空参数 MPN 字典
// cloneSharePresetParameterMPNs copies a nullable parameter-MPN map
func cloneSharePresetParameterMPNs(value map[string]int32) map[string]int32 {
	if value == nil {
		return nil
	}
	result := make(map[string]int32, len(value))
	for key, number := range value {
		result[key] = number
	}
	return result
}
