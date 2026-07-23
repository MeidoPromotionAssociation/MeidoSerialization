package KCES

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// system.dat 内 MoveablePanelManager::SceneEdit::savedata 虚拟文件的 MessagePack 布局
// 该载荷没有独立磁盘扩展名
// MessagePack layout for the MoveablePanelManager::SceneEdit::savedata virtual file inside system.dat
// This payload has no standalone disk extension

const (
	moveablePanelMaxMessagePackDepth = 256
)

// MoveablePanelSaveData 表示 system.dat 中 EditData/MoveablePanelManager::SceneEdit::savedata 保存的原始未压缩 Standard MessagePack 值
// 两个切片有意保持顺序，因为 MoveablePanelManager 会遍历 MoveablePanelPosition 并调用 Transform.SetAsLastSibling 恢复面板同级顺序
// 将任一线格式列表转换为映射都会丢失游戏中可观察的状态
// MoveablePanelSaveData represents the raw uncompressed Standard MessagePack value stored at EditData/MoveablePanelManager::SceneEdit::savedata in system.dat
// Both slices intentionally retain order because MoveablePanelManager restores panel sibling order by iterating MoveablePanelPosition and calling Transform.SetAsLastSibling
// Converting either wire list to a map would lose observable game state
type MoveablePanelSaveData struct {
	MessagePackRootMetadata                                  // 根值 nil 与尾部字节元数据 / Root nil and trailing-byte metadata
	MoveablePanelPosition    []MoveablePanelPositionEntry    `json:"moveablePanelPosition"`    // Key 0 的有序面板名称和 anchoredPosition3D 列表 / Ordered panel-name and anchoredPosition3D list at Key 0
	MoveablePanelActiveState []MoveablePanelActiveStateEntry `json:"moveablePanelActiveState"` // Key 1 的有序面板名称和打开状态列表 / Ordered panel-name and open-state list at Key 1
	FieldCount               *int32                          `json:"fieldCount,omitempty"`     // 原始 indexed object 的槽位数，标准宽度 2 时可省略 / Slot count of the original indexed object, omittable for the standard width of 2
	FutureSlots              [][]byte                        `json:"futureSlots,omitempty"`    // Key 2 起未知槽位的完整 MessagePack 原始值 / Complete raw MessagePack values of unknown slots starting at Key 2
}

// MoveablePanelPositionEntry 表示 KeyValuePair<string, UnityEngine.Vector3>
// MessagePack KeyValuePair 格式化器始终使用精确的 array 2
// MoveablePanelPositionEntry represents KeyValuePair<string, UnityEngine.Vector3>
// The MessagePack KeyValuePair formatter always uses an exact array of 2
type MoveablePanelPositionEntry struct {
	PanelName      string  `json:"panelName"`                // KeyValuePair Key 中用于查找 MoveablePanel.PanelName 的名称 / Name in the KeyValuePair Key used to find MoveablePanel.PanelName
	PanelNameIsNil bool    `json:"panelNameIsNil,omitempty"` // 线格式中的 Key 是否显式为 MessagePack nil / Whether the wire Key was explicitly MessagePack nil
	Position       Vector3 `json:"position"`                 // KeyValuePair Value 中恢复到 RectTransform.anchoredPosition3D 的位置 / Position in the KeyValuePair Value restored to RectTransform.anchoredPosition3D
}

// MoveablePanelActiveStateEntry 表示 KeyValuePair<string, bool>
// MessagePack KeyValuePair 格式化器始终使用精确的 array 2
// MoveablePanelActiveStateEntry represents KeyValuePair<string, bool>
// The MessagePack KeyValuePair formatter always uses an exact array of 2
type MoveablePanelActiveStateEntry struct {
	PanelName      string `json:"panelName"`                // KeyValuePair Key 中用于匹配 MoveablePanel.PanelName 的名称 / Name in the KeyValuePair Key used to match MoveablePanel.PanelName
	PanelNameIsNil bool   `json:"panelNameIsNil,omitempty"` // 线格式中的 Key 是否显式为 MessagePack nil / Whether the wire Key was explicitly MessagePack nil
	Active         bool   `json:"active"`                   // KeyValuePair Value 中决定游戏显示或隐藏面板的打开状态 / Open state in the KeyValuePair Value used by the game to show or hide the panel
}

// DecodeMoveablePanelSaveData 解码 MessagePackSerializer.Serialize<MoveablePanelSaveData> 写出的裸 Standard MessagePack 载荷
// 兼容行为遵循游戏格式化器而不假定每个 indexed array 都有唯一标准长度
// indexed object 格式化器先构造 MoveablePanelSaveData，因此缺失字段保持线模型零值，未知尾部键被消费后原样保存在 FutureSlots
// Vector3Formatter 接受任意长度数组，缺失的 x、y、z 默认为零并跳过 z 之后的分量，解码宽度与跳过值保存在 Vector3.IndexedObjectMetadata 中
// MessagePackReader.ReadSingle 接受 float32、float64 和所有整数编码并转换为 System.Single，可空列表与名称、重复名称及所有 IEEE-754 值均会保留
// 畸形 UTF-8 按 MessagePackReader.ReadString 的行为使用 U+FFFD 规范化
// DecodeMoveablePanelSaveData decodes the bare Standard MessagePack payload written by MessagePackSerializer.Serialize<MoveablePanelSaveData>
// Compatibility follows the game formatters instead of assuming every indexed array has one canonical length
// The indexed object formatter constructs MoveablePanelSaveData first, so absent fields keep wire-model zero values and unknown trailing keys are consumed and retained verbatim in FutureSlots
// Vector3Formatter accepts arrays of any length, defaults absent x, y, and z to zero, and skips later components while decoded width and skipped values remain in Vector3.IndexedObjectMetadata
// MessagePackReader.ReadSingle accepts float32, float64, and every integer encoding and converts to System.Single, while nullable lists and names, duplicate names, and all IEEE-754 values are preserved
// Malformed UTF-8 is normalized with U+FFFD to match MessagePackReader.ReadString
func DecodeMoveablePanelSaveData(data []byte) (*MoveablePanelSaveData, error) {
	reader := moveablePanelMessagePackReader{data: data}
	if reader.tryReadNil() {
		trailing, err := messagePackRootTrailingAfterParsed(data, reader.pos, "MoveablePanelSaveData")
		if err != nil {
			return nil, err
		}
		if len(trailing) != 0 {
			return &MoveablePanelSaveData{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
		}
		return nil, nil
	}
	fieldCount, err := reader.readArrayHeader("MoveablePanelSaveData")
	if err != nil {
		return nil, err
	}

	value := &MoveablePanelSaveData{}
	if fieldCount != 2 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount > 2 {
		value.FutureSlots = makeKCESCountedSliceForAppend[[]byte](uint64(fieldCount - 2))
	}
	for field := int64(0); field < fieldCount; field++ {
		start := reader.pos
		switch field {
		case 0:
			value.MoveablePanelPosition, err = reader.readPositionList()
		case 1:
			value.MoveablePanelActiveState, err = reader.readActiveStateList()
		default:
			err = reader.skipValue(0)
		}
		if err != nil {
			return nil, fmt.Errorf("MoveablePanelSaveData field %d: %w", field, err)
		}
		if field >= 2 {
			value.FutureSlots = append(value.FutureSlots, append([]byte(nil), reader.data[start:reader.pos]...))
		}
	}
	trailing, err := messagePackRootTrailingAfterParsed(data, reader.pos, "MoveablePanelSaveData")
	if err != nil {
		return nil, err
	}
	if err := validateMoveablePanelSaveData(value); err != nil {
		return nil, err
	}
	value.TrailingData = trailing
	return value, nil
}

// EncodeMoveablePanelSaveData 保留解码后的 indexed object 与 Vector3 宽度及未来槽位
// 新建 Vector3 使用游戏当前的 array 3 布局和 float32 标记 0xca，KeyValuePair 按游戏格式化器要求保持精确 array 2
// 调用者不会被修改
// EncodeMoveablePanelSaveData preserves decoded indexed object and Vector3 widths and future slots
// Newly created Vector3 values use the current game array-of-3 layout and float32 marker 0xca while KeyValuePair remains an exact array of 2 as required by the game formatter
// The caller is never modified
func EncodeMoveablePanelSaveData(value *MoveablePanelSaveData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if out, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.MoveablePanelPosition != nil || value.MoveablePanelActiveState != nil || value.FieldCount != nil || len(value.FutureSlots) != 0,
		"MoveablePanelSaveData",
	); handled {
		return out, err
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 2, value.FutureSlots, "MoveablePanelSaveData")
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.MoveablePanelPosition != nil {
		return nil, fmt.Errorf("MoveablePanelSaveData fieldCount %d would discard moveablePanelPosition", fieldCount)
	}
	if fieldCount < 2 && value.MoveablePanelActiveState != nil {
		return nil, fmt.Errorf("MoveablePanelSaveData fieldCount %d would discard moveablePanelActiveState", fieldCount)
	}
	if err := validateMoveablePanelSaveData(value); err != nil {
		return nil, err
	}

	out := make([]byte, 0, kcesInitialCollectionCapacity)
	out = appendMoveablePanelArrayHeader(out, fieldCount)
	if fieldCount >= 1 {
		if value.MoveablePanelPosition == nil {
			out = append(out, 0xc0)
		} else {
			out = appendMoveablePanelArrayHeader(out, int64(len(value.MoveablePanelPosition)))
			for index, entry := range value.MoveablePanelPosition {
				out = appendMoveablePanelArrayHeader(out, 2)
				if entry.PanelNameIsNil {
					out = append(out, 0xc0)
				} else {
					var err error
					out, err = appendMoveablePanelString(out, entry.PanelName)
					if err != nil {
						return nil, fmt.Errorf("MoveablePanelPosition[%d].PanelName: %w", index, err)
					}
				}
				encodedPosition, err := ct.EncodeIndexedMsgpack(&entry.Position)
				if err != nil {
					return nil, fmt.Errorf("MoveablePanelPosition[%d].Position: %w", index, err)
				}
				out = append(out, encodedPosition...)
			}
		}
	}

	if fieldCount >= 2 {
		if value.MoveablePanelActiveState == nil {
			out = append(out, 0xc0)
		} else {
			out = appendMoveablePanelArrayHeader(out, int64(len(value.MoveablePanelActiveState)))
			for index, entry := range value.MoveablePanelActiveState {
				out = appendMoveablePanelArrayHeader(out, 2)
				if entry.PanelNameIsNil {
					out = append(out, 0xc0)
				} else {
					var err error
					out, err = appendMoveablePanelString(out, entry.PanelName)
					if err != nil {
						return nil, fmt.Errorf("MoveablePanelActiveState[%d].PanelName: %w", index, err)
					}
				}
				if entry.Active {
					out = append(out, 0xc3)
				} else {
					out = append(out, 0xc2)
				}
			}
		}
	}
	for _, slot := range value.FutureSlots {
		out = append(out, slot...)
	}
	return appendMessagePackRootTrailing(out, value.MessagePackRootMetadata), nil
}

// validateMoveablePanelSaveData 验证列表长度以及面板名称 nil 状态和 UTF-8 表示
// validateMoveablePanelSaveData validates list lengths and panel-name nil state and UTF-8 representation
func validateMoveablePanelSaveData(value *MoveablePanelSaveData) error {
	if value == nil {
		return nil
	}
	if uint64(len(value.MoveablePanelPosition)) > math.MaxUint32 {
		return fmt.Errorf("MoveablePanelSaveData position list has %d entries, exceeding the MessagePack array32 limit", len(value.MoveablePanelPosition))
	}
	if uint64(len(value.MoveablePanelActiveState)) > math.MaxUint32 {
		return fmt.Errorf("MoveablePanelSaveData active-state list has %d entries, exceeding the MessagePack array32 limit", len(value.MoveablePanelActiveState))
	}

	for index, entry := range value.MoveablePanelPosition {
		if entry.PanelNameIsNil && entry.PanelName != "" {
			return fmt.Errorf("MoveablePanelPosition[%d].PanelNameIsNil would discard panelName", index)
		}
		if !entry.PanelNameIsNil && !utf8.ValidString(entry.PanelName) {
			return fmt.Errorf("MoveablePanelPosition[%d].PanelName is not valid UTF-8", index)
		}
	}

	for index, entry := range value.MoveablePanelActiveState {
		if entry.PanelNameIsNil && entry.PanelName != "" {
			return fmt.Errorf("MoveablePanelActiveState[%d].PanelNameIsNil would discard panelName", index)
		}
		if !entry.PanelNameIsNil && !utf8.ValidString(entry.PanelName) {
			return fmt.Errorf("MoveablePanelActiveState[%d].PanelName is not valid UTF-8", index)
		}
	}

	return nil
}

// moveablePanelMessagePackReader 提供面板状态载荷所需的有界 MessagePack 读取和跳过操作
// moveablePanelMessagePackReader provides bounded MessagePack read and skip operations needed by panel-state payloads
type moveablePanelMessagePackReader struct {
	data []byte // 完整输入字节 / Complete input bytes
	pos  int64  // 下一个待读取字节的位置 / Position of the next byte to read
}

// tryReadNil 在当前位置为 MessagePack nil 时消费它并返回 true
// tryReadNil consumes MessagePack nil at the current position and reports whether it was present
func (reader *moveablePanelMessagePackReader) tryReadNil() bool {
	if reader.pos < int64(len(reader.data)) && reader.data[reader.pos] == 0xc0 {
		reader.pos++
		return true
	}
	return false
}

// remaining 返回尚未消费的输入字节数
// remaining returns the number of unconsumed input bytes
func (reader *moveablePanelMessagePackReader) remaining() int64 {
	return int64(len(reader.data)) - reader.pos
}

// readByte 读取单个字节并在输入耗尽时返回带标签错误
// readByte reads one byte and returns a label-qualified error when input is exhausted
func (reader *moveablePanelMessagePackReader) readByte(label string) (byte, error) {
	if reader.pos >= int64(len(reader.data)) {
		return 0, fmt.Errorf("%s: unexpected EOF", label)
	}
	value := reader.data[reader.pos]
	reader.pos++
	return value, nil
}

// readBytes 读取指定长度的连续字节而不复制并推进当前位置
// readBytes reads a contiguous byte range without copying and advances the current position
func (reader *moveablePanelMessagePackReader) readBytes(length int64, label string) ([]byte, error) {
	if length < 0 || length > reader.remaining() {
		return nil, fmt.Errorf("%s: unexpected EOF (need %d bytes, have %d)", label, length, reader.remaining())
	}
	value := reader.data[reader.pos : reader.pos+length]
	reader.pos += length
	return value, nil
}

// readArrayHeader 读取 fixarray、array16 或 array32 的元素数量
// readArrayHeader reads an element count from fixarray, array16, or array32
func (reader *moveablePanelMessagePackReader) readArrayHeader(label string) (int64, error) {
	code, err := reader.readByte(label)
	if err != nil {
		return 0, err
	}
	var count uint64
	switch {
	case code >= 0x90 && code <= 0x9f:
		count = uint64(code & 0x0f)
	case code == 0xdc:
		bytes, err := reader.readBytes(2, label+" array16 header")
		if err != nil {
			return 0, err
		}
		count = uint64(binary.BigEndian.Uint16(bytes))
	case code == 0xdd:
		bytes, err := reader.readBytes(4, label+" array32 header")
		if err != nil {
			return 0, err
		}
		count = uint64(binary.BigEndian.Uint32(bytes))
	default:
		return 0, fmt.Errorf("%s: expected MessagePack array, got code 0x%02x", label, code)
	}
	return int64(count), nil
}

// readPositionList 读取可空的有序 KeyValuePair<string, Vector3> 列表并要求每项宽度为 2
// readPositionList reads a nullable ordered KeyValuePair<string, Vector3> list and requires every pair to have width 2
func (reader *moveablePanelMessagePackReader) readPositionList() ([]MoveablePanelPositionEntry, error) {
	if reader.tryReadNil() {
		return nil, nil
	}
	count, err := reader.readArrayHeader("position list")
	if err != nil {
		return nil, err
	}
	if count > reader.remaining()/3 {
		return nil, fmt.Errorf("position list: unexpected EOF before %d entries", count)
	}
	result := makeKCESCountedSliceForAppend[MoveablePanelPositionEntry](uint64(count))
	for index := int64(0); index < count; index++ {
		pairCount, err := reader.readArrayHeader(fmt.Sprintf("position entry %d KeyValuePair", index))
		if err != nil {
			return nil, err
		}
		if pairCount != 2 {
			return nil, fmt.Errorf("position entry %d KeyValuePair must be array(2), got array(%d)", index, pairCount)
		}
		name, isNil, err := reader.readString(fmt.Sprintf("position entry %d name", index))
		if err != nil {
			return nil, err
		}
		position, err := reader.readVector3(fmt.Sprintf("position entry %d Vector3", index))
		if err != nil {
			return nil, err
		}
		result = append(result, MoveablePanelPositionEntry{PanelName: name, PanelNameIsNil: isNil, Position: position})
	}
	return result, nil
}

// readActiveStateList 读取可空的有序 KeyValuePair<string, bool> 列表并要求每项宽度为 2
// readActiveStateList reads a nullable ordered KeyValuePair<string, bool> list and requires every pair to have width 2
func (reader *moveablePanelMessagePackReader) readActiveStateList() ([]MoveablePanelActiveStateEntry, error) {
	if reader.tryReadNil() {
		return nil, nil
	}
	count, err := reader.readArrayHeader("active-state list")
	if err != nil {
		return nil, err
	}
	if count > reader.remaining()/3 {
		return nil, fmt.Errorf("active-state list: unexpected EOF before %d entries", count)
	}
	result := makeKCESCountedSliceForAppend[MoveablePanelActiveStateEntry](uint64(count))
	for index := int64(0); index < count; index++ {
		pairCount, err := reader.readArrayHeader(fmt.Sprintf("active-state entry %d KeyValuePair", index))
		if err != nil {
			return nil, err
		}
		if pairCount != 2 {
			return nil, fmt.Errorf("active-state entry %d KeyValuePair must be array(2), got array(%d)", index, pairCount)
		}
		name, isNil, err := reader.readString(fmt.Sprintf("active-state entry %d name", index))
		if err != nil {
			return nil, err
		}
		active, err := reader.readBool(fmt.Sprintf("active-state entry %d value", index))
		if err != nil {
			return nil, err
		}
		result = append(result, MoveablePanelActiveStateEntry{PanelName: name, PanelNameIsNil: isNil, Active: active})
	}
	return result, nil
}

// readString 读取可为 nil 的 MessagePack 字符串，并按游戏行为替换畸形 UTF-8
// readString reads a nullable MessagePack string and replaces malformed UTF-8 using game behavior
func (reader *moveablePanelMessagePackReader) readString(label string) (string, bool, error) {
	code, err := reader.readByte(label)
	if err != nil {
		return "", false, err
	}
	if code == 0xc0 {
		return "", true, nil
	}
	var length uint64
	switch {
	case code >= 0xa0 && code <= 0xbf:
		length = uint64(code & 0x1f)
	case code == 0xd9:
		value, err := reader.readByte(label + " str8 length")
		if err != nil {
			return "", false, err
		}
		length = uint64(value)
	case code == 0xda:
		value, err := reader.readBytes(2, label+" str16 length")
		if err != nil {
			return "", false, err
		}
		length = uint64(binary.BigEndian.Uint16(value))
	case code == 0xdb:
		value, err := reader.readBytes(4, label+" str32 length")
		if err != nil {
			return "", false, err
		}
		length = uint64(binary.BigEndian.Uint32(value))
	default:
		return "", false, fmt.Errorf("%s: expected MessagePack string, got code 0x%02x", label, code)
	}
	if length > uint64(reader.remaining()) {
		return "", false, fmt.Errorf("%s: unexpected EOF (string length %d, have %d)", label, length, reader.remaining())
	}
	bytes, err := reader.readBytes(int64(length), label)
	if err != nil {
		return "", false, err
	}
	if !utf8.Valid(bytes) {
		return strings.ToValidUTF8(string(bytes), "\uFFFD"), false, nil
	}
	return string(bytes), false, nil
}

// readVector3 读取任意 indexed array 宽度的 Unity Vector3 并保留其结构元数据
// readVector3 reads a Unity Vector3 with any indexed array width and preserves its structural metadata
func (reader *moveablePanelMessagePackReader) readVector3(label string) (Vector3, error) {
	start := reader.pos
	count, err := reader.readArrayHeader(label)
	if err != nil {
		return Vector3{}, err
	}
	for index := int64(0); index < count; index++ {
		if err := reader.skipValue(0); err != nil {
			return Vector3{}, fmt.Errorf("%s component %d: %w", label, index, err)
		}
	}

	var value Vector3
	if err := ct.DecodeMsgpack(reader.data[start:reader.pos], &value); err != nil {
		return Vector3{}, fmt.Errorf("%s: %w", label, err)
	}
	// Vector3Formatter 对已知分量调用 ReadSingle，因此会拒绝 nil 而不是将其视为 float32 零值
	// 共享 indexed object 编解码器会记录这类 nil 标量槽位，使结构元数据得以保留而不放宽游戏接受的线格式语法
	// Vector3Formatter calls ReadSingle for known components and therefore rejects nil instead of treating it as float32 zero
	// The shared indexed object codec records such nil scalar slots so structural metadata can be retained without weakening the wire grammar accepted by the game
	if value.IndexedObjectMetadata != nil && len(value.NilSlots) != 0 {
		return Vector3{}, fmt.Errorf("%s component %d is nil; the game requires a number", label, value.NilSlots[0])
	}
	return value, nil
}

// readSingle 按 MessagePackReader.ReadSingle 规则将浮点或整数编码转换为 float32
// readSingle converts floating-point or integer encodings to float32 using MessagePackReader.ReadSingle rules
func (reader *moveablePanelMessagePackReader) readSingle(label string) (float32, error) {
	code, err := reader.readByte(label)
	if err != nil {
		return 0, err
	}
	switch {
	case code <= 0x7f:
		return float32(code), nil
	case code >= 0xe0:
		return float32(int8(code)), nil
	}

	switch code {
	case 0xca:
		bytes, err := reader.readBytes(4, label+" float32")
		if err != nil {
			return 0, err
		}
		return math.Float32frombits(binary.BigEndian.Uint32(bytes)), nil
	case 0xcb:
		bytes, err := reader.readBytes(8, label+" float64")
		if err != nil {
			return 0, err
		}
		return float32(math.Float64frombits(binary.BigEndian.Uint64(bytes))), nil
	case 0xcc:
		value, err := reader.readByte(label + " uint8")
		return float32(value), err
	case 0xcd:
		bytes, err := reader.readBytes(2, label+" uint16")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint16(bytes)), nil
	case 0xce:
		bytes, err := reader.readBytes(4, label+" uint32")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint32(bytes)), nil
	case 0xcf:
		bytes, err := reader.readBytes(8, label+" uint64")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint64(bytes)), nil
	case 0xd0:
		value, err := reader.readByte(label + " int8")
		return float32(int8(value)), err
	case 0xd1:
		bytes, err := reader.readBytes(2, label+" int16")
		if err != nil {
			return 0, err
		}
		return float32(int16(binary.BigEndian.Uint16(bytes))), nil
	case 0xd2:
		bytes, err := reader.readBytes(4, label+" int32")
		if err != nil {
			return 0, err
		}
		return float32(int32(binary.BigEndian.Uint32(bytes))), nil
	case 0xd3:
		bytes, err := reader.readBytes(8, label+" int64")
		if err != nil {
			return 0, err
		}
		return float32(int64(binary.BigEndian.Uint64(bytes))), nil
	default:
		return 0, fmt.Errorf("%s: expected a MessagePack number accepted by ReadSingle, got code 0x%02x", label, code)
	}
}

// readBool 只接受 MessagePack false 或 true 标记
// readBool accepts only the MessagePack false or true markers
func (reader *moveablePanelMessagePackReader) readBool(label string) (bool, error) {
	code, err := reader.readByte(label)
	if err != nil {
		return false, err
	}
	switch code {
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	default:
		return false, fmt.Errorf("%s: expected MessagePack bool, got code 0x%02x", label, code)
	}
}

// skipValue 为未知 indexed object 字段和 Vector3 分量镜像 MessagePackReader.Skip，并施加明确递归限制
// skipValue mirrors MessagePackReader.Skip for unknown indexed object fields and Vector3 components while enforcing an explicit recursion limit
func (reader *moveablePanelMessagePackReader) skipValue(depth int64) error {
	if depth >= moveablePanelMaxMessagePackDepth {
		return fmt.Errorf("MessagePack nesting exceeds %d", moveablePanelMaxMessagePackDepth)
	}
	code, err := reader.readByte("skipped MessagePack value")
	if err != nil {
		return err
	}

	switch {
	case code <= 0x7f, code >= 0xe0, code == 0xc0, code == 0xc2, code == 0xc3:
		return nil
	case code >= 0xa0 && code <= 0xbf:
		_, err = reader.readBytes(int64(code&0x1f), "skipped fixstr")
		return err
	case code >= 0x90 && code <= 0x9f:
		return reader.skipArray(int64(code&0x0f), depth)
	case code >= 0x80 && code <= 0x8f:
		return reader.skipMap(int64(code&0x0f), depth)
	}

	// 固定扩展长度包含一个类型代码字节
	// Fixed extension lengths include one type-code byte
	fixedPayload := map[byte]int64{
		0xca: 4, 0xcb: 8,
		0xcc: 1, 0xcd: 2, 0xce: 4, 0xcf: 8,
		0xd0: 1, 0xd1: 2, 0xd2: 4, 0xd3: 8,
		0xd4: 2, // one-byte extension payload plus type code
		0xd5: 3,
		0xd6: 5,
		0xd7: 9,
		0xd8: 17,
	}
	if length, ok := fixedPayload[code]; ok {
		_, err = reader.readBytes(length, "skipped scalar")
		return err
	}

	switch code {
	case 0xc4, 0xc7, 0xd9:
		lengthByte, err := reader.readByte("skipped length")
		if err != nil {
			return err
		}
		length := int64(lengthByte)
		// ext8 长度不包含其类型代码字节
		// The ext8 length excludes its type-code byte
		if code == 0xc7 {
			length++
		}
		_, err = reader.readBytes(length, "skipped payload")
		return err
	case 0xc5, 0xc8, 0xda:
		lengthBytes, err := reader.readBytes(2, "skipped length")
		if err != nil {
			return err
		}
		length := uint64(binary.BigEndian.Uint16(lengthBytes))
		if code == 0xc8 {
			length++
		}
		return reader.skipBytePayload(length)
	case 0xc6, 0xc9, 0xdb:
		lengthBytes, err := reader.readBytes(4, "skipped length")
		if err != nil {
			return err
		}
		length := uint64(binary.BigEndian.Uint32(lengthBytes))
		if code == 0xc9 {
			length++
		}
		return reader.skipBytePayload(length)
	case 0xdc:
		bytes, err := reader.readBytes(2, "skipped array16 length")
		if err != nil {
			return err
		}
		return reader.skipArray(int64(binary.BigEndian.Uint16(bytes)), depth)
	case 0xdd:
		bytes, err := reader.readBytes(4, "skipped array32 length")
		if err != nil {
			return err
		}
		count := int64(binary.BigEndian.Uint32(bytes))
		return reader.skipArray(count, depth)
	case 0xde:
		bytes, err := reader.readBytes(2, "skipped map16 length")
		if err != nil {
			return err
		}
		return reader.skipMap(int64(binary.BigEndian.Uint16(bytes)), depth)
	case 0xdf:
		bytes, err := reader.readBytes(4, "skipped map32 length")
		if err != nil {
			return err
		}
		count := int64(binary.BigEndian.Uint32(bytes))
		return reader.skipMap(count, depth)
	case 0xc1:
		return fmt.Errorf("reserved MessagePack code 0xc1")
	default:
		return fmt.Errorf("unsupported MessagePack code 0x%02x", code)
	}
}

// skipBytePayload 按 uint64 长度跳过原始载荷并先验证剩余字节
// skipBytePayload skips a raw payload by uint64 length after validating remaining bytes
func (reader *moveablePanelMessagePackReader) skipBytePayload(length uint64) error {
	if length > uint64(reader.remaining()) {
		return fmt.Errorf("skipped payload: unexpected EOF (need %d bytes, have %d)", length, reader.remaining())
	}
	reader.pos += int64(length)
	return nil
}

// skipArray 逐项跳过数组并在递归前按剩余字节限制数量
// skipArray skips an array item by item and bounds its count by remaining bytes before recursion
func (reader *moveablePanelMessagePackReader) skipArray(count, depth int64) error {
	if count > reader.remaining() {
		return fmt.Errorf("skipped array: unexpected EOF before %d entries", count)
	}
	for index := int64(0); index < count; index++ {
		if err := reader.skipValue(depth + 1); err != nil {
			return fmt.Errorf("skipped array item %d: %w", index, err)
		}
	}
	return nil
}

// skipMap 逐项跳过映射键和值并在递归前按剩余字节限制数量
// skipMap skips map keys and values one pair at a time and bounds their count by remaining bytes before recursion
func (reader *moveablePanelMessagePackReader) skipMap(count, depth int64) error {
	if count > reader.remaining()/2 {
		return fmt.Errorf("skipped map: unexpected EOF before %d entries", count)
	}
	for index := int64(0); index < count; index++ {
		if err := reader.skipValue(depth + 1); err != nil {
			return fmt.Errorf("skipped map key %d: %w", index, err)
		}
		if err := reader.skipValue(depth + 1); err != nil {
			return fmt.Errorf("skipped map value %d: %w", index, err)
		}
	}
	return nil
}

// appendMoveablePanelArrayHeader 以可表示数量的最短 MessagePack 数组头追加元素数
// appendMoveablePanelArrayHeader appends an element count using the shortest MessagePack array header that can represent it
func appendMoveablePanelArrayHeader(out []byte, count int64) []byte {
	switch {
	case count < 16:
		return append(out, 0x90|byte(count))
	case count <= math.MaxUint16:
		return append(out, 0xdc, byte(count>>8), byte(count))
	default:
		return append(out, 0xdd, byte(uint32(count)>>24), byte(uint32(count)>>16), byte(uint32(count)>>8), byte(count))
	}
}

// appendMoveablePanelString 验证 UTF-8 后以可表示字节长度的最短 MessagePack 字符串形式追加值
// appendMoveablePanelString validates UTF-8 and appends a value using the shortest MessagePack string form that can represent its byte length
func appendMoveablePanelString(out []byte, value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("value is not valid UTF-8")
	}
	length := int64(len(value))
	switch {
	case length < 32:
		out = append(out, 0xa0|byte(length))
	case length <= math.MaxUint8:
		out = append(out, 0xd9, byte(length))
	case length <= math.MaxUint16:
		out = append(out, 0xda, byte(length>>8), byte(length))
	case length <= math.MaxUint32:
		out = append(out, 0xdb, byte(length>>24), byte(length>>16), byte(length>>8), byte(length))
	default:
		return nil, fmt.Errorf("string length %d exceeds MessagePack str32", length)
	}
	return append(out, value...), nil
}

// appendMoveablePanelFloat32 使用标准 MessagePack float32 标记和大端位模式追加值
// appendMoveablePanelFloat32 appends a value using the canonical MessagePack float32 marker and big-endian bit pattern
func appendMoveablePanelFloat32(out []byte, value float32) []byte {
	bits := math.Float32bits(value)
	return append(out, 0xca, byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))
}
