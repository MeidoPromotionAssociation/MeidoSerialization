package KCES

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

// system.dat 的共通 EditData MessagePack 实现，包含预设面板名称、调色板颜色及共享读取器。
// 这些对象没有独立磁盘扩展名，只作为 VirtualDirectory 内部虚拟文件存在。
//
// Shared EditData MessagePack implementation for system.dat, including preset-panel names, palette colors, and the shared reader.
// These objects have no standalone disk extension and exist only as virtual files inside VirtualDirectory.

// PresetPanelNameSaveData is the raw MessagePack payload stored at
// EditData/PresetPanelNameSaveData::SceneEdit::savedata in system.dat.
//
// The C# object is an indexed MessagePack object with one field:
//
//	[Key(0)] List<string> BoxNameList
//
// A list element is a *string because MessagePack-CSharp's string formatter
// accepts nil. PresetPanel.LoadBoxName deliberately treats a nil entry as a
// missing name and falls back to "BOX<n>".
type PresetPanelNameSaveData struct {
	MessagePackRootMetadata
	BoxNameList []*string `json:"boxNameList"`
	FieldCount  *int32    `json:"fieldCount,omitempty"`
	FutureSlots [][]byte  `json:"futureSlots,omitempty"`
}

// PaletteColorSaveData is the raw MessagePack payload stored at
// EditData/PaletteColorSave{index} in system.dat.
//
// Its indexed MessagePack layout is [color, index, isSave]. The game currently
// defines color keys 0 through 8, but the field itself is a public
// Dictionary<int,int>. Unknown keys are therefore preserved for forward
// compatibility without imposing the consumer's expected key set.
type PaletteColorSaveData struct {
	MessagePackRootMetadata
	Color       map[int32]int32 `json:"color"`
	Index       int32           `json:"index"`
	IsSave      int32           `json:"isSave"`
	FieldCount  *int32          `json:"fieldCount,omitempty"`
	FutureSlots [][]byte        `json:"futureSlots,omitempty"`
}

const simpleEditDataMaxDepth = 256

// DecodePresetPanelNameSaveData decodes one uncompressed, unprefixed
// MessagePack value. The codec library finds the root boundary; bytes after it
// are retained in TrailingData. Future indexed object fields inside the root
// array are consumed like the game's formatter and retained verbatim in
// FutureSlots for lossless re-encoding.
func DecodePresetPanelNameSaveData(data []byte) (*PresetPanelNameSaveData, error) {
	r := simpleEditDataReader{data: data}
	if r.tryReadNil() {
		trailing, err := messagePackRootTrailingAfterParsed(data, r.pos, "PresetPanelNameSaveData")
		if err != nil {
			return nil, err
		}
		// DynamicObjectTypeBuilder returns null for a nil class. PresetPanel
		// handles that result as absent data and creates BOX1..BOX10 defaults.
		if len(trailing) != 0 {
			return &PresetPanelNameSaveData{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
		}
		return nil, nil
	}

	fieldCount, err := r.readArrayLength("PresetPanelNameSaveData")
	if err != nil {
		return nil, err
	}
	if err := r.requirePossibleValues(fieldCount, "PresetPanelNameSaveData fields"); err != nil {
		return nil, err
	}

	value := &PresetPanelNameSaveData{}
	if fieldCount != 1 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount >= 1 {
		value.BoxNameList, err = r.readNullableStringList("PresetPanelNameSaveData.boxNameList")
		if err != nil {
			return nil, err
		}
	}
	for key := int64(1); key < fieldCount; key++ {
		start := r.pos
		if err := r.skipValue(0); err != nil {
			return nil, fmt.Errorf("skip PresetPanelNameSaveData future Key(%d): %w", key, err)
		}
		value.FutureSlots = append(value.FutureSlots, append([]byte(nil), r.data[start:r.pos]...))
	}
	trailing, err := messagePackRootTrailingAfterParsed(data, r.pos, "PresetPanelNameSaveData")
	if err != nil {
		return nil, err
	}
	value.TrailingData = trailing
	return value, nil
}

// EncodePresetPanelNameSaveData emits the indexed-object width represented by
// FieldCount and FutureSlots. A nil object is encoded as MessagePack nil,
// matching the C# class formatter. The caller and its slice are never modified.
func EncodePresetPanelNameSaveData(value *PresetPanelNameSaveData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if out, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.BoxNameList != nil || value.FieldCount != nil || len(value.FutureSlots) != 0,
		"PresetPanelNameSaveData",
	); handled {
		return out, err
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 1, value.FutureSlots, "PresetPanelNameSaveData")
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.BoxNameList != nil {
		return nil, fmt.Errorf("PresetPanelNameSaveData fieldCount %d would discard boxNameList", fieldCount)
	}
	if uint64(len(value.BoxNameList)) > math.MaxUint32 {
		return nil, fmt.Errorf("PresetPanelNameSaveData.boxNameList has %d entries, exceeding the MessagePack array32 limit", len(value.BoxNameList))
	}
	for i, name := range value.BoxNameList {
		if name != nil && !utf8.ValidString(*name) {
			return nil, fmt.Errorf("PresetPanelNameSaveData.boxNameList[%d] is not valid UTF-8", i)
		}
		if name != nil && uint64(len(*name)) > math.MaxUint32 {
			return nil, fmt.Errorf("PresetPanelNameSaveData.boxNameList[%d] is too large for MessagePack str32", i)
		}
	}

	out := simpleEditDataAppendArrayHeader(nil, fieldCount)
	if fieldCount >= 1 {
		if value.BoxNameList == nil {
			out = append(out, 0xc0)
		} else {
			out = simpleEditDataAppendArrayHeader(out, int64(len(value.BoxNameList)))
			for _, name := range value.BoxNameList {
				if name == nil {
					out = append(out, 0xc0)
				} else {
					out = simpleEditDataAppendString(out, *name)
				}
			}
		}
	}
	for _, slot := range value.FutureSlots {
		out = append(out, slot...)
	}
	return appendMessagePackRootTrailing(out, value.MessagePackRootMetadata), nil
}

// DecodePaletteColorSaveData decodes one uncompressed, unprefixed MessagePack
// value. Short indexed arrays retain the C# parameterless constructor's zero
// defaults for absent fields. Nullable dictionaries and unknown/missing keys
// are retained without applying SaveDataToLayerFreeColor behavior.
func DecodePaletteColorSaveData(data []byte) (*PaletteColorSaveData, error) {
	r := simpleEditDataReader{data: data}
	if r.tryReadNil() {
		trailing, err := messagePackRootTrailingAfterParsed(data, r.pos, "PaletteColorSaveData")
		if err != nil {
			return nil, err
		}
		if len(trailing) != 0 {
			return &PaletteColorSaveData{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
		}
		return nil, nil
	}

	fieldCount, err := r.readArrayLength("PaletteColorSaveData")
	if err != nil {
		return nil, err
	}
	if err := r.requirePossibleValues(fieldCount, "PaletteColorSaveData fields"); err != nil {
		return nil, err
	}

	value := &PaletteColorSaveData{}
	if fieldCount != 3 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount >= 1 {
		colors, err := r.readInt32Map("PaletteColorSaveData.color")
		if err != nil {
			return nil, err
		}
		value.Color = colors
	}
	if fieldCount >= 2 {
		value.Index, err = r.readInt32("PaletteColorSaveData.index")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 3 {
		value.IsSave, err = r.readInt32("PaletteColorSaveData.isSave")
		if err != nil {
			return nil, err
		}
	}
	for key := int64(3); key < fieldCount; key++ {
		start := r.pos
		if err := r.skipValue(0); err != nil {
			return nil, fmt.Errorf("skip PaletteColorSaveData future Key(%d): %w", key, err)
		}
		value.FutureSlots = append(value.FutureSlots, append([]byte(nil), r.data[start:r.pos]...))
	}
	trailing, err := messagePackRootTrailingAfterParsed(data, r.pos, "PaletteColorSaveData")
	if err != nil {
		return nil, err
	}
	value.TrailingData = trailing
	return value, nil
}

// EncodePaletteColorSaveData emits the indexed-object width represented by
// FieldCount and FutureSlots. Dictionary keys are sorted numerically for
// deterministic output; the source map is only read and is never reordered or
// mutated.
func EncodePaletteColorSaveData(value *PaletteColorSaveData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if out, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.Color != nil || value.Index != 0 || value.IsSave != 0 || value.FieldCount != nil || len(value.FutureSlots) != 0,
		"PaletteColorSaveData",
	); handled {
		return out, err
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 3, value.FutureSlots, "PaletteColorSaveData")
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.Color != nil {
		return nil, fmt.Errorf("PaletteColorSaveData fieldCount %d would discard color", fieldCount)
	}
	if fieldCount < 2 && value.Index != 0 {
		return nil, fmt.Errorf("PaletteColorSaveData fieldCount %d would discard index=%d", fieldCount, value.Index)
	}
	if fieldCount < 3 && value.IsSave != 0 {
		return nil, fmt.Errorf("PaletteColorSaveData fieldCount %d would discard isSave=%d", fieldCount, value.IsSave)
	}

	if uint64(len(value.Color)) > math.MaxUint32 {
		return nil, fmt.Errorf("PaletteColorSaveData.color has %d entries, exceeding the MessagePack map32 limit", len(value.Color))
	}

	keys := make([]int32, 0, len(value.Color))
	for key := range value.Color {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	out := simpleEditDataAppendArrayHeader(nil, fieldCount)
	if fieldCount >= 1 {
		if value.Color == nil {
			out = append(out, 0xc0)
		} else {
			out = simpleEditDataAppendMapHeader(out, int64(len(keys)))
			for _, key := range keys {
				out = simpleEditDataAppendInt32(out, key)
				out = simpleEditDataAppendInt32(out, value.Color[key])
			}
		}
	}
	if fieldCount >= 2 {
		out = simpleEditDataAppendInt32(out, value.Index)
	}
	if fieldCount >= 3 {
		out = simpleEditDataAppendInt32(out, value.IsSave)
	}
	for _, slot := range value.FutureSlots {
		out = append(out, slot...)
	}
	return appendMessagePackRootTrailing(out, value.MessagePackRootMetadata), nil
}

func resolveIndexedFieldCount(stored *int32, known int64, futureSlots [][]byte, path string) (int64, error) {
	if int64(len(futureSlots)) > math.MaxInt32 {
		return 0, fmt.Errorf("%s futureSlots has %d values, exceeding the C# Int32 array-header limit", path, len(futureSlots))
	}
	fieldCount := known + int64(len(futureSlots))
	if fieldCount < 0 || fieldCount > math.MaxInt32 {
		return 0, fmt.Errorf("%s field count %d is outside the C# Int32 array-header range", path, fieldCount)
	}
	if stored != nil {
		fieldCount = int64(*stored)
	}
	if fieldCount < 0 {
		return 0, fmt.Errorf("%s fieldCount %d is outside the C# Int32 array-header range", path, fieldCount)
	}
	wantFuture := int64(0)
	if fieldCount > known {
		wantFuture = fieldCount - known
	}
	if int64(len(futureSlots)) != wantFuture {
		return 0, fmt.Errorf("%s fieldCount %d requires %d futureSlots, got %d", path, fieldCount, wantFuture, len(futureSlots))
	}
	for index, slot := range futureSlots {
		if err := validateRawMessagePackValue(slot, fmt.Sprintf("%s.futureSlots[%d]", path, index)); err != nil {
			return 0, err
		}
	}
	return fieldCount, nil
}

func validateRawMessagePackValue(data []byte, path string) error {
	if len(data) == 0 {
		return fmt.Errorf("%s is empty; one complete MessagePack value is required", path)
	}
	r := simpleEditDataReader{data: data}
	if err := r.skipValue(0); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := r.requireEOF(path); err != nil {
		return err
	}
	return nil
}

type simpleEditDataReader struct {
	data []byte
	pos  int64
}

func (r *simpleEditDataReader) remaining() int64 {
	return int64(len(r.data)) - r.pos
}

func (r *simpleEditDataReader) tryReadNil() bool {
	if r.remaining() > 0 && r.data[r.pos] == 0xc0 {
		r.pos++
		return true
	}
	return false
}

func (r *simpleEditDataReader) requireEOF(name string) error {
	if remaining := r.remaining(); remaining != 0 {
		return fmt.Errorf("%s has %d trailing bytes", name, remaining)
	}
	return nil
}

func (r *simpleEditDataReader) requirePossibleValues(count int64, path string) error {
	// Every MessagePack value occupies at least one byte. This both catches a
	// truncated array32 bomb before looping and bounds work by the input size.
	if count > r.remaining() {
		return fmt.Errorf("%s count %d exceeds the %d remaining bytes", path, count, r.remaining())
	}
	return nil
}

func (r *simpleEditDataReader) readByte(path string) (byte, error) {
	if r.remaining() < 1 {
		return 0, fmt.Errorf("read %s: unexpected EOF", path)
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *simpleEditDataReader) readBytes(length int64, path string) ([]byte, error) {
	if length < 0 || length > r.remaining() {
		return nil, fmt.Errorf("read %s: need %d bytes, only %d remain", path, length, r.remaining())
	}
	value := r.data[r.pos : r.pos+length]
	r.pos += length
	return value, nil
}

func (r *simpleEditDataReader) readArrayLength(path string) (int64, error) {
	marker, err := r.readByte(path + " array header")
	if err != nil {
		return 0, err
	}
	switch {
	case marker >= 0x90 && marker <= 0x9f:
		return int64(marker & 0x0f), nil
	case marker == 0xdc:
		value, err := r.readBytes(2, path+" array16 length")
		if err != nil {
			return 0, err
		}
		return int64(binary.BigEndian.Uint16(value)), nil
	case marker == 0xdd:
		value, err := r.readBytes(4, path+" array32 length")
		if err != nil {
			return 0, err
		}
		return int64(binary.BigEndian.Uint32(value)), nil
	default:
		return 0, fmt.Errorf("%s must be a MessagePack array, got marker 0x%02x", path, marker)
	}
}

func (r *simpleEditDataReader) readMapLength(path string) (int64, error) {
	marker, err := r.readByte(path + " map header")
	if err != nil {
		return 0, err
	}
	switch {
	case marker >= 0x80 && marker <= 0x8f:
		return int64(marker & 0x0f), nil
	case marker == 0xde:
		value, err := r.readBytes(2, path+" map16 length")
		if err != nil {
			return 0, err
		}
		return int64(binary.BigEndian.Uint16(value)), nil
	case marker == 0xdf:
		value, err := r.readBytes(4, path+" map32 length")
		if err != nil {
			return 0, err
		}
		return int64(binary.BigEndian.Uint32(value)), nil
	default:
		return 0, fmt.Errorf("%s must be a MessagePack map, got marker 0x%02x", path, marker)
	}
}

func (r *simpleEditDataReader) readNullableStringList(path string) ([]*string, error) {
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
	for i := int64(0); i < count; i++ {
		if r.tryReadNil() {
			result = append(result, nil)
			continue
		}
		text, err := r.readString(fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		textCopy := text
		result = append(result, &textCopy)
	}
	return result, nil
}

func (r *simpleEditDataReader) readString(path string) (string, error) {
	marker, err := r.readByte(path + " string header")
	if err != nil {
		return "", err
	}
	var length int64
	switch {
	case marker >= 0xa0 && marker <= 0xbf:
		length = int64(marker & 0x1f)
	case marker == 0xd9:
		value, err := r.readByte(path + " str8 length")
		if err != nil {
			return "", err
		}
		length = int64(value)
	case marker == 0xda:
		value, err := r.readBytes(2, path+" str16 length")
		if err != nil {
			return "", err
		}
		length = int64(binary.BigEndian.Uint16(value))
	case marker == 0xdb:
		value, err := r.readBytes(4, path+" str32 length")
		if err != nil {
			return "", err
		}
		length = int64(binary.BigEndian.Uint32(value))
	default:
		return "", fmt.Errorf("%s must be a MessagePack string or nil, got marker 0x%02x", path, marker)
	}
	payload, err := r.readBytes(length, path+" UTF-8 payload")
	if err != nil {
		return "", err
	}
	if !utf8.Valid(payload) {
		// MessagePackReader.ReadString uses new UTF8Encoding(false), whose
		// default DecoderReplacementFallback converts malformed input to U+FFFD.
		// Normalize the same way instead of rejecting bytes the game can read.
		return strings.ToValidUTF8(string(payload), "\uFFFD"), nil
	}
	return string(payload), nil
}

func (r *simpleEditDataReader) readInt32Map(path string) (map[int32]int32, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := r.readMapLength(path)
	if err != nil {
		return nil, err
	}
	if count > r.remaining()/2 {
		return nil, fmt.Errorf("%s count %d exceeds the capacity of %d remaining bytes", path, count, r.remaining())
	}
	result := makeKCESCountedMap[int32, int32](uint64(count))
	for i := int64(0); i < count; i++ {
		key, err := r.readInt32(fmt.Sprintf("%s key %d", path, i))
		if err != nil {
			return nil, err
		}
		if _, duplicate := result[key]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate key %d", path, key)
		}
		value, err := r.readInt32(fmt.Sprintf("%s[%d]", path, key))
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

func (r *simpleEditDataReader) readInt32(path string) (int32, error) {
	marker, err := r.readByte(path)
	if err != nil {
		return 0, err
	}
	var signed int64
	switch {
	case marker <= 0x7f:
		return int32(marker), nil
	case marker >= 0xe0:
		return int32(int8(marker)), nil
	}

	switch marker {
	case 0xcc:
		value, err := r.readByte(path + " uint8")
		return int32(value), err
	case 0xcd:
		value, err := r.readBytes(2, path+" uint16")
		if err != nil {
			return 0, err
		}
		return int32(binary.BigEndian.Uint16(value)), nil
	case 0xce:
		value, err := r.readBytes(4, path+" uint32")
		if err != nil {
			return 0, err
		}
		unsigned := uint64(binary.BigEndian.Uint32(value))
		if unsigned > uint64(gameInt32Max) {
			return 0, fmt.Errorf("%s=%d is outside the Int32 range [%d,%d]", path, unsigned, gameInt32Min, gameInt32Max)
		}
		return int32(unsigned), nil
	case 0xcf:
		value, err := r.readBytes(8, path+" uint64")
		if err != nil {
			return 0, err
		}
		unsigned := binary.BigEndian.Uint64(value)
		if unsigned > uint64(gameInt32Max) {
			return 0, fmt.Errorf("%s=%d is outside the Int32 range [%d,%d]", path, unsigned, gameInt32Min, gameInt32Max)
		}
		return int32(unsigned), nil
	case 0xd0:
		value, err := r.readByte(path + " int8")
		if err != nil {
			return 0, err
		}
		signed = int64(int8(value))
	case 0xd1:
		value, err := r.readBytes(2, path+" int16")
		if err != nil {
			return 0, err
		}
		signed = int64(int16(binary.BigEndian.Uint16(value)))
	case 0xd2:
		value, err := r.readBytes(4, path+" int32")
		if err != nil {
			return 0, err
		}
		signed = int64(int32(binary.BigEndian.Uint32(value)))
	case 0xd3:
		value, err := r.readBytes(8, path+" int64")
		if err != nil {
			return 0, err
		}
		signed = int64(binary.BigEndian.Uint64(value))
	default:
		return 0, fmt.Errorf("%s must be a MessagePack Int32-compatible integer, got marker 0x%02x", path, marker)
	}
	if signed < gameInt32Min || signed > gameInt32Max {
		return 0, fmt.Errorf("%s=%d is outside the Int32 range [%d,%d]", path, signed, gameInt32Min, gameInt32Max)
	}
	return int32(signed), nil
}

// skipValue mirrors MessagePackReader.Skip while locating the end of a future
// indexed-object field. The complete byte range is then retained by the caller;
// it may contain any valid MessagePack value, including nested arrays, maps,
// binary, and extensions.
func (r *simpleEditDataReader) skipValue(depth int64) error {
	if depth >= simpleEditDataMaxDepth {
		return fmt.Errorf("MessagePack nesting exceeds %d", simpleEditDataMaxDepth)
	}
	marker, err := r.readByte("MessagePack value")
	if err != nil {
		return err
	}

	switch {
	case marker <= 0x7f || marker >= 0xe0:
		return nil
	case marker >= 0xa0 && marker <= 0xbf:
		_, err = r.readBytes(int64(marker&0x1f), "fixstr payload")
		return err
	case marker >= 0x90 && marker <= 0x9f:
		return r.skipValues(int64(marker&0x0f), false, depth)
	case marker >= 0x80 && marker <= 0x8f:
		return r.skipValues(int64(marker&0x0f), true, depth)
	}

	var payloadLength int64
	switch marker {
	case 0xc0, 0xc2, 0xc3:
		return nil
	case 0xc1:
		return fmt.Errorf("reserved MessagePack marker 0xc1")
	case 0xc4, 0xd9:
		length, readErr := r.readByte("8-bit payload length")
		if readErr != nil {
			return readErr
		}
		payloadLength = int64(length)
	case 0xc5, 0xda:
		length, readErr := r.readBytes(2, "16-bit payload length")
		if readErr != nil {
			return readErr
		}
		payloadLength = int64(binary.BigEndian.Uint16(length))
	case 0xc6, 0xdb:
		length, readErr := r.readBytes(4, "32-bit payload length")
		if readErr != nil {
			return readErr
		}
		payloadLength = int64(binary.BigEndian.Uint32(length))
	case 0xc7:
		length, readErr := r.readByte("ext8 length")
		if readErr != nil {
			return readErr
		}
		payloadLength = int64(length) + 1 // type code
	case 0xc8:
		length, readErr := r.readBytes(2, "ext16 length")
		if readErr != nil {
			return readErr
		}
		payloadLength = int64(binary.BigEndian.Uint16(length)) + 1
	case 0xc9:
		length, readErr := r.readBytes(4, "ext32 length")
		if readErr != nil {
			return readErr
		}
		extLength := int64(binary.BigEndian.Uint32(length))
		payloadLength = extLength + 1
	case 0xca, 0xce, 0xd2:
		payloadLength = 4
	case 0xcb, 0xcf, 0xd3:
		payloadLength = 8
	case 0xcc, 0xd0:
		payloadLength = 1
	case 0xcd, 0xd1:
		payloadLength = 2
	case 0xd4:
		payloadLength = 2 // type code + one payload byte
	case 0xd5:
		payloadLength = 3
	case 0xd6:
		payloadLength = 5
	case 0xd7:
		payloadLength = 9
	case 0xd8:
		payloadLength = 17
	case 0xdc, 0xdd:
		var count int64
		if marker == 0xdc {
			length, readErr := r.readBytes(2, "array16 length")
			if readErr != nil {
				return readErr
			}
			count = int64(binary.BigEndian.Uint16(length))
		} else {
			length, readErr := r.readBytes(4, "array32 length")
			if readErr != nil {
				return readErr
			}
			count = int64(binary.BigEndian.Uint32(length))
		}
		return r.skipValues(count, false, depth)
	case 0xde, 0xdf:
		var count int64
		if marker == 0xde {
			length, readErr := r.readBytes(2, "map16 length")
			if readErr != nil {
				return readErr
			}
			count = int64(binary.BigEndian.Uint16(length))
		} else {
			length, readErr := r.readBytes(4, "map32 length")
			if readErr != nil {
				return readErr
			}
			count = int64(binary.BigEndian.Uint32(length))
		}
		return r.skipValues(count, true, depth)
	default:
		return fmt.Errorf("unsupported MessagePack marker 0x%02x", marker)
	}
	_, err = r.readBytes(payloadLength, "MessagePack payload")
	return err
}

func (r *simpleEditDataReader) skipValues(count int64, isMap bool, depth int64) error {
	values := count
	if isMap {
		if count > r.remaining()/2 {
			return fmt.Errorf("MessagePack map count %d exceeds %d remaining bytes", count, r.remaining())
		}
		values *= 2
	} else if count > r.remaining() {
		return fmt.Errorf("MessagePack array count %d exceeds %d remaining bytes", count, r.remaining())
	}
	for i := int64(0); i < values; i++ {
		if err := r.skipValue(depth + 1); err != nil {
			return fmt.Errorf("element %d: %w", i, err)
		}
	}
	return nil
}

func simpleEditDataAppendArrayHeader(dst []byte, length int64) []byte {
	switch {
	case length <= 15:
		return append(dst, 0x90|byte(length))
	case length <= math.MaxUint16:
		return append(dst, 0xdc, byte(length>>8), byte(length))
	default:
		dst = append(dst, 0xdd, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(length))
		return dst
	}
}

func simpleEditDataAppendMapHeader(dst []byte, length int64) []byte {
	switch {
	case length <= 15:
		return append(dst, 0x80|byte(length))
	case length <= math.MaxUint16:
		return append(dst, 0xde, byte(length>>8), byte(length))
	default:
		dst = append(dst, 0xdf, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(length))
		return dst
	}
}

func simpleEditDataAppendString(dst []byte, value string) []byte {
	length := len(value)
	switch {
	case length <= 31:
		dst = append(dst, 0xa0|byte(length))
	case length <= math.MaxUint8:
		dst = append(dst, 0xd9, byte(length))
	case length <= math.MaxUint16:
		dst = append(dst, 0xda, byte(length>>8), byte(length))
	default:
		dst = append(dst, 0xdb, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(length))
	}
	return append(dst, value...)
}

func simpleEditDataAppendInt32(dst []byte, value int32) []byte {
	switch {
	case value >= 0 && value <= 0x7f:
		return append(dst, byte(value))
	case value >= 0 && value <= math.MaxUint8:
		return append(dst, 0xcc, byte(value))
	case value >= 0 && value <= math.MaxUint16:
		return append(dst, 0xcd, byte(value>>8), byte(value))
	case value >= 0:
		dst = append(dst, 0xce, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(value))
		return dst
	case value >= -32:
		return append(dst, byte(int8(value)))
	case value >= math.MinInt8:
		return append(dst, 0xd0, byte(int8(value)))
	case value >= math.MinInt16:
		return append(dst, 0xd1, byte(int16(value)>>8), byte(int16(value)))
	default:
		dst = append(dst, 0xd2, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(int32(value)))
		return dst
	}
}
