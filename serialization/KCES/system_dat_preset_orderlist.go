package KCES

import (
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// system.dat 内 color_preset/.../preset_orderlist 虚拟文件的 MessagePack 布局
// 该载荷没有独立磁盘扩展名
// MessagePack layout for color_preset/.../preset_orderlist virtual files inside system.dat
// This payload has no standalone disk extension

const (
	// ColorPresetOrderListVersion 是 KCES 1.34.4 中 ColorPresetProvider.PresetOrderList.FixVersion 的值
	// ColorPresetOrderListVersion is the value of ColorPresetProvider.PresetOrderList.FixVersion in KCES 1.34.4
	ColorPresetOrderListVersion = 1000

	// KCES 搭载的 MessagePack-CSharp 中 MessagePackSerializerOptions.CompressionMinLength 默认为 64
	// Lz4BlockArray 选项会原样写出更短的序列化值，仅包装达到或超过此边界的值
	// MessagePackSerializerOptions.CompressionMinLength defaults to 64 in the MessagePack-CSharp version shipped with KCES
	// Lz4BlockArray options write shorter serialized values unchanged and wrap only values at or above this boundary
	colorPresetOrderListCompressionMinLength = 64
)

// ColorPresetOrderList 表示 system.dat/EditData/color_preset 下名为 preset_orderlist 的 VirtualFile 载荷
// 它对应 MaidEdit.ColorPresetProvider.PresetOrderList，Key 0 是继承版本，Key 1 是 ID 顺序列表
// IDOrderList 及其项目都可为 nil，游戏加载时会报告空、nil 和重复 ID 但不会拒绝序列化对象，因此此无损载荷层保留这些值
// ColorPresetOrderList represents the payload in a VirtualFile named preset_orderlist below system.dat/EditData/color_preset
// It corresponds to MaidEdit.ColorPresetProvider.PresetOrderList with the inherited version at Key 0 and the ID order list at Key 1
// IDOrderList and its entries may be nil, and the game reports empty, nil, and duplicate IDs while loading without rejecting the serialized object, so this lossless payload layer preserves them
type ColorPresetOrderList struct {
	Version     int32     `json:"version"`     // Key 0 的版本，当前 FixVersion 为 1000 / Version at Key 0, with a current FixVersion of 1000
	IDOrderList []*string `json:"idOrderList"` // Key 1 的可空颜色预设 instanceGuid 顺序列表 / Nullable ordered list of color-preset instanceGuid values at Key 1
}

// NewColorPresetOrderList 返回 C# 构造流程产生的状态
// AMessagePackSerializationVersionControlIntKey 将版本初始化为 FixVersion，而没有初始化器的 idOrderList 自动属性保持 nil
// NewColorPresetOrderList returns the state produced by the C# construction flow
// AMessagePackSerializationVersionControlIntKey initializes the version to FixVersion while the idOrderList auto-property has no initializer and remains nil
func NewColorPresetOrderList() *ColorPresetOrderList {
	return &ColorPresetOrderList{Version: ColorPresetOrderListVersion}
}

// DecodeColorPresetOrderList 解码直接保存在 preset_orderlist VirtualFile 中的 StandardLz4BlockArray 字节
// 此载荷没有 BinaryWriter 长度前缀，因为 SetFileData 直接接收 MessagePackSerializer.Serialize 的结果，LoadPresetOrder 也将完整 VirtualFile 字节交给反序列化器
// 根对象必须使用固定两槽布局并完整消费解压后的输入，空 Migrate 实现不会被调用，旧版本解码后也不重写
// DecodeColorPresetOrderList decodes StandardLz4BlockArray bytes stored directly in the preset_orderlist VirtualFile
// This payload has no BinaryWriter length prefix because SetFileData receives MessagePackSerializer.Serialize output directly and LoadPresetOrder passes complete VirtualFile bytes to the deserializer
// The root must use the fixed two-slot layout and consume all decompressed input, while the empty Migrate implementation is not invoked and legacy versions are not rewritten after decoding
func DecodeColorPresetOrderList(data []byte) (*ColorPresetOrderList, error) {
	raw, err := ct.DecompressLz4BlockArray(data)
	if err != nil {
		return nil, fmt.Errorf("decompress ColorPresetOrderList Lz4BlockArray: %w", err)
	}

	r := simpleEditDataReader{data: raw}
	if r.tryReadNil() {
		if err := r.requireEOF("ColorPresetOrderList"); err != nil {
			return nil, err
		}
		return nil, nil
	}
	fieldCount, err := r.readArrayLength("ColorPresetOrderList")
	if err != nil {
		return nil, err
	}
	if err := r.requirePossibleValues(fieldCount, "ColorPresetOrderList fields"); err != nil {
		return nil, err
	}

	if fieldCount != 2 {
		return nil, fmt.Errorf("unsupported ColorPresetOrderList indexed-array width %d, expected 2", fieldCount)
	}
	value := &ColorPresetOrderList{}
	value.Version, err = r.readInt32("ColorPresetOrderList.version")
	if err != nil {
		return nil, err
	}
	value.IDOrderList, err = readColorPresetOrderStringList(&r)
	if err != nil {
		return nil, err
	}
	if err := r.requireEOF("ColorPresetOrderList"); err != nil {
		return nil, err
	}
	return value, nil
}

// EncodeColorPresetOrderList 使用固定两槽 indexed object 和 StandardLz4BlockArray 规则写出调用者版本
// EncodeColorPresetOrderList uses a fixed two-slot indexed object and StandardLz4BlockArray rules while preserving the caller version
func EncodeColorPresetOrderList(value *ColorPresetOrderList) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if int64(len(value.IDOrderList)) > math.MaxInt32 {
		return nil, fmt.Errorf("ColorPresetOrderList.idOrderList length %d exceeds the C# Int32 array-header limit", len(value.IDOrderList))
	}
	for index, id := range value.IDOrderList {
		if id == nil {
			continue
		}
		if !utf8.ValidString(*id) {
			return nil, fmt.Errorf("ColorPresetOrderList.idOrderList[%d] is not valid UTF-8", index)
		}
		if uint64(len(*id)) > math.MaxUint32 {
			return nil, fmt.Errorf("ColorPresetOrderList.idOrderList[%d] is too large for MessagePack str32", index)
		}
	}

	raw := simpleEditDataAppendArrayHeader(nil, 2)
	raw = simpleEditDataAppendInt32(raw, value.Version)
	if value.IDOrderList == nil {
		raw = append(raw, 0xc0)
	} else {
		raw = simpleEditDataAppendArrayHeader(raw, int64(len(value.IDOrderList)))
		for _, id := range value.IDOrderList {
			if id == nil {
				raw = append(raw, 0xc0)
			} else {
				raw = simpleEditDataAppendString(raw, *id)
			}
		}
	}
	return encodeColorPresetOrderListRaw(raw)
}

// encodeColorPresetOrderListRaw 按 MessagePack-CSharp 的 64 字节阈值决定原样写出或 Lz4BlockArray 包装
// encodeColorPresetOrderListRaw chooses unchanged output or Lz4BlockArray wrapping using the MessagePack-CSharp 64-byte threshold
func encodeColorPresetOrderListRaw(raw []byte) ([]byte, error) {
	if len(raw) < colorPresetOrderListCompressionMinLength {
		return raw, nil
	}
	encoded, err := ct.CompressLz4BlockArray(raw)
	if err != nil {
		return nil, fmt.Errorf("compress ColorPresetOrderList Lz4BlockArray: %w", err)
	}
	return encoded, nil
}

// readColorPresetOrderStringList 读取 Key 1 中列表本身及其项目都可为 nil 的字符串数组
// readColorPresetOrderStringList reads the Key 1 string array whose list and entries may both be nil
func readColorPresetOrderStringList(r *simpleEditDataReader) ([]*string, error) {
	const path = "ColorPresetOrderList.idOrderList"
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := r.readArrayLength(path)
	if err != nil {
		return nil, err
	}
	if err := r.requirePossibleValues(count, path+" entries"); err != nil {
		return nil, err
	}

	result := makeKCESCountedSliceForAppend[*string](uint64(count))
	for index := int64(0); index < count; index++ {
		if r.tryReadNil() {
			result = append(result, nil)
			continue
		}
		id, err := r.readString(fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return nil, err
		}
		idCopy := id
		result = append(result, &idCopy)
	}
	return result, nil
}
