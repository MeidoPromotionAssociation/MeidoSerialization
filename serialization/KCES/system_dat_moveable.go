package KCES

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// system.dat 内 MoveablePanelManager::SceneEdit::savedata 虚拟文件的 MessagePack 布局
// 该载荷没有独立磁盘扩展名
// MessagePack layout for the MoveablePanelManager::SceneEdit::savedata virtual file inside system.dat
// This payload has no standalone disk extension

// MoveablePanelSaveData 表示 system.dat 中 EditData/MoveablePanelManager::SceneEdit::savedata 保存的类型化未压缩 Standard MessagePack 对象
// 两个切片有意保持顺序，因为 MoveablePanelManager 会遍历 MoveablePanelPosition 并调用 Transform.SetAsLastSibling 恢复面板同级顺序
// 将任一线格式列表转换为映射都会丢失游戏中可观察的状态
// MoveablePanelSaveData represents the typed uncompressed Standard MessagePack object stored at EditData/MoveablePanelManager::SceneEdit::savedata in system.dat
// Both slices intentionally retain order because MoveablePanelManager restores panel sibling order by iterating MoveablePanelPosition and calling Transform.SetAsLastSibling
// Converting either wire list to a map would lose observable game state
type MoveablePanelSaveData struct {
	MoveablePanelPosition    []MoveablePanelPositionEntry    `json:"moveablePanelPosition"`    // Key 0 的有序面板名称和 anchoredPosition3D 列表 / Ordered panel-name and anchoredPosition3D list at Key 0
	MoveablePanelActiveState []MoveablePanelActiveStateEntry `json:"moveablePanelActiveState"` // Key 1 的有序面板名称和打开状态列表 / Ordered panel-name and open-state list at Key 1
}

// MoveablePanelPositionEntry 表示 KeyValuePair<string, UnityEngine.Vector3>
// MessagePack KeyValuePair 格式化器始终使用精确的 array 2
// MoveablePanelPositionEntry represents KeyValuePair<string, UnityEngine.Vector3>
// The MessagePack KeyValuePair formatter always uses an exact array of 2
type MoveablePanelPositionEntry struct {
	PanelName *string `json:"panelName"` // KeyValuePair Key 中用于查找 MoveablePanel.PanelName 的可空名称 / Nullable name in the KeyValuePair Key used to find MoveablePanel.PanelName
	Position  Vector3 `json:"position"`  // KeyValuePair Value 中恢复到 RectTransform.anchoredPosition3D 的位置 / Position in the KeyValuePair Value restored to RectTransform.anchoredPosition3D
}

// MoveablePanelActiveStateEntry 表示 KeyValuePair<string, bool>
// MessagePack KeyValuePair 格式化器始终使用精确的 array 2
// MoveablePanelActiveStateEntry represents KeyValuePair<string, bool>
// The MessagePack KeyValuePair formatter always uses an exact array of 2
type MoveablePanelActiveStateEntry struct {
	PanelName *string `json:"panelName"` // KeyValuePair Key 中用于匹配 MoveablePanel.PanelName 的可空名称 / Nullable name in the KeyValuePair Key used to match MoveablePanel.PanelName
	Active    bool    `json:"active"`    // KeyValuePair Value 中决定游戏显示或隐藏面板的打开状态 / Open state in the KeyValuePair Value used by the game to show or hide the panel
}

// DecodeMoveablePanelSaveData 解码 MessagePackSerializer.Serialize<MoveablePanelSaveData> 写出的裸 Standard MessagePack 载荷
// 根对象固定为两槽，KeyValuePair 固定为两槽，Vector3 固定为三槽，并要求完整消费输入
// MessagePackReader.ReadSingle 接受 float32、float64 和所有整数编码并转换为 System.Single，可空列表与名称、重复名称及所有 IEEE-754 值均会保留
// 畸形 UTF-8 按 MessagePackReader.ReadString 的行为使用 U+FFFD 规范化
// DecodeMoveablePanelSaveData decodes the bare Standard MessagePack payload written by MessagePackSerializer.Serialize<MoveablePanelSaveData>
// The root object uses exactly two slots, KeyValuePair uses exactly two slots, Vector3 uses exactly three slots, and the complete input must be consumed
// MessagePackReader.ReadSingle accepts float32, float64, and every integer encoding and converts to System.Single, while nullable lists and names, duplicate names, and all IEEE-754 values are preserved
// Malformed UTF-8 is normalized with U+FFFD to match MessagePackReader.ReadString
func DecodeMoveablePanelSaveData(data []byte) (*MoveablePanelSaveData, error) {
	reader := moveablePanelMessagePackReader{data: data}
	if reader.tryReadNil() {
		if reader.remaining() != 0 {
			return nil, fmt.Errorf("MoveablePanelSaveData has %d trailing bytes", reader.remaining())
		}
		return nil, nil
	}
	fieldCount, err := reader.readArrayHeader("MoveablePanelSaveData")
	if err != nil {
		return nil, err
	}

	if fieldCount != 2 {
		return nil, fmt.Errorf("unsupported MoveablePanelSaveData indexed-array width %d, expected 2", fieldCount)
	}
	value := &MoveablePanelSaveData{}
	for field := int64(0); field < 2; field++ {
		switch field {
		case 0:
			value.MoveablePanelPosition, err = reader.readPositionList()
		case 1:
			value.MoveablePanelActiveState, err = reader.readActiveStateList()
		}
		if err != nil {
			return nil, fmt.Errorf("MoveablePanelSaveData field %d: %w", field, err)
		}
	}
	if reader.remaining() != 0 {
		return nil, fmt.Errorf("MoveablePanelSaveData has %d trailing bytes", reader.remaining())
	}
	if err := validateMoveablePanelSaveData(value); err != nil {
		return nil, err
	}
	return value, nil
}

// EncodeMoveablePanelSaveData 使用固定两槽根对象、两槽 KeyValuePair 与三槽 Vector3 布局
// Vector3 使用 float32 标记 0xca，调用者不会被修改
// 调用者不会被修改
// EncodeMoveablePanelSaveData uses a fixed two-slot root, two-slot KeyValuePair, and three-slot Vector3 layout
// Vector3 values use the float32 marker 0xca and the caller is never modified
func EncodeMoveablePanelSaveData(value *MoveablePanelSaveData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if err := validateMoveablePanelSaveData(value); err != nil {
		return nil, err
	}

	out := make([]byte, 0, kcesInitialCollectionCapacity)
	out = appendMoveablePanelArrayHeader(out, 2)
	{
		if value.MoveablePanelPosition == nil {
			out = append(out, 0xc0)
		} else {
			out = appendMoveablePanelArrayHeader(out, int64(len(value.MoveablePanelPosition)))
			for index, entry := range value.MoveablePanelPosition {
				out = appendMoveablePanelArrayHeader(out, 2)
				if entry.PanelName == nil {
					out = append(out, 0xc0)
				} else {
					var err error
					out, err = appendMoveablePanelString(out, *entry.PanelName)
					if err != nil {
						return nil, fmt.Errorf("MoveablePanelPosition[%d].PanelName: %w", index, err)
					}
				}
				out = appendMoveablePanelArrayHeader(out, 3)
				out = appendMoveablePanelFloat32(out, entry.Position.X)
				out = appendMoveablePanelFloat32(out, entry.Position.Y)
				out = appendMoveablePanelFloat32(out, entry.Position.Z)
			}
		}
	}

	{
		if value.MoveablePanelActiveState == nil {
			out = append(out, 0xc0)
		} else {
			out = appendMoveablePanelArrayHeader(out, int64(len(value.MoveablePanelActiveState)))
			for index, entry := range value.MoveablePanelActiveState {
				out = appendMoveablePanelArrayHeader(out, 2)
				if entry.PanelName == nil {
					out = append(out, 0xc0)
				} else {
					var err error
					out, err = appendMoveablePanelString(out, *entry.PanelName)
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
	return out, nil
}

// validateMoveablePanelSaveData 验证列表长度以及非空面板名称的 UTF-8 表示
// validateMoveablePanelSaveData validates list lengths and the UTF-8 representation of non-nil panel names
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
		if entry.PanelName != nil && !utf8.ValidString(*entry.PanelName) {
			return fmt.Errorf("MoveablePanelPosition[%d].PanelName is not valid UTF-8", index)
		}
	}

	for index, entry := range value.MoveablePanelActiveState {
		if entry.PanelName != nil && !utf8.ValidString(*entry.PanelName) {
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
		name, err := reader.readString(fmt.Sprintf("position entry %d name", index))
		if err != nil {
			return nil, err
		}
		position, err := reader.readVector3(fmt.Sprintf("position entry %d Vector3", index))
		if err != nil {
			return nil, err
		}
		result = append(result, MoveablePanelPositionEntry{PanelName: name, Position: position})
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
		name, err := reader.readString(fmt.Sprintf("active-state entry %d name", index))
		if err != nil {
			return nil, err
		}
		active, err := reader.readBool(fmt.Sprintf("active-state entry %d value", index))
		if err != nil {
			return nil, err
		}
		result = append(result, MoveablePanelActiveStateEntry{PanelName: name, Active: active})
	}
	return result, nil
}

// readString 读取可为 nil 的 MessagePack 字符串，并按游戏行为替换畸形 UTF-8
// readString reads a nullable MessagePack string and replaces malformed UTF-8 using game behavior
func (reader *moveablePanelMessagePackReader) readString(label string) (*string, error) {
	code, err := reader.readByte(label)
	if err != nil {
		return nil, err
	}
	if code == 0xc0 {
		return nil, nil
	}
	var length uint64
	switch {
	case code >= 0xa0 && code <= 0xbf:
		length = uint64(code & 0x1f)
	case code == 0xd9:
		value, err := reader.readByte(label + " str8 length")
		if err != nil {
			return nil, err
		}
		length = uint64(value)
	case code == 0xda:
		value, err := reader.readBytes(2, label+" str16 length")
		if err != nil {
			return nil, err
		}
		length = uint64(binary.BigEndian.Uint16(value))
	case code == 0xdb:
		value, err := reader.readBytes(4, label+" str32 length")
		if err != nil {
			return nil, err
		}
		length = uint64(binary.BigEndian.Uint32(value))
	default:
		return nil, fmt.Errorf("%s: expected MessagePack string, got code 0x%02x", label, code)
	}
	if length > uint64(reader.remaining()) {
		return nil, fmt.Errorf("%s: unexpected EOF (string length %d, have %d)", label, length, reader.remaining())
	}
	bytes, err := reader.readBytes(int64(length), label)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(bytes) {
		value := strings.ToValidUTF8(string(bytes), "\uFFFD")
		return &value, nil
	}
	value := string(bytes)
	return &value, nil
}

// readVector3 读取固定三槽的 Unity Vector3，任一分量为 nil 或宽度不符时返回错误
// readVector3 reads a fixed three-slot Unity Vector3 and rejects nil components or an incorrect width
func (reader *moveablePanelMessagePackReader) readVector3(label string) (Vector3, error) {
	count, err := reader.readArrayHeader(label)
	if err != nil {
		return Vector3{}, err
	}
	if count != 3 {
		return Vector3{}, fmt.Errorf("unsupported %s indexed-array width %d, expected 3", label, count)
	}
	x, err := reader.readSingle(label + ".x")
	if err != nil {
		return Vector3{}, err
	}
	y, err := reader.readSingle(label + ".y")
	if err != nil {
		return Vector3{}, err
	}
	z, err := reader.readSingle(label + ".z")
	if err != nil {
		return Vector3{}, err
	}
	return Vector3{X: x, Y: y, Z: z}, nil
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
