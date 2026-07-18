package KCES

import (
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// system.dat 内 color_preset/.../preset_orderlist 虚拟文件的 MessagePack 布局。
// 该载荷没有独立磁盘扩展名。
//
// MessagePack layout for color_preset/.../preset_orderlist virtual files inside system.dat.
// This payload has no standalone disk extension.

const (
	// ColorPresetOrderListVersion is
	// ColorPresetProvider.PresetOrderList.FixVersion in KCES 1.34.4.
	ColorPresetOrderListVersion = 1000

	// MessagePackSerializerOptions.CompressionMinLength defaults to 64 in the
	// MessagePack-CSharp version shipped by KCES. Lz4BlockArray options write a
	// shorter serialized value through unchanged and only wrap values at or
	// above this boundary.
	colorPresetOrderListCompressionMinLength = 64
)

// ColorPresetOrderList is the payload stored in a VirtualFile named
// "preset_orderlist" below system.dat/EditData/color_preset/... . It mirrors
// MaidEdit.ColorPresetProvider.PresetOrderList:
//
//	[Key(0)] int version
//	[Key(1)] List<string> idOrderList
//
// IDOrderList and its elements are nullable because MessagePack-CSharp's
// standard List<string> formatter represents both with MessagePack nil. The
// game reports empty, nil, and duplicate IDs while loading but does not reject
// the serialized object, so this lossless payload layer preserves them.
type ColorPresetOrderList struct {
	MessagePackRootMetadata
	Version     int       `json:"version"`
	IDOrderList []*string `json:"idOrderList"`
	FieldCount  *int      `json:"fieldCount,omitempty"`
	FutureSlots [][]byte  `json:"futureSlots,omitempty"`
}

// NewColorPresetOrderList returns the state produced by the C# constructors.
// AMessagePackSerializationVersionControlIntKey initializes version to
// FixVersion, while the auto-property idOrderList has no initializer and is
// therefore nil.
func NewColorPresetOrderList() *ColorPresetOrderList {
	return &ColorPresetOrderList{Version: ColorPresetOrderListVersion}
}

// DecodeColorPresetOrderList decodes the StandardLz4BlockArray bytes stored
// directly in the preset_orderlist VirtualFile. There is no BinaryWriter
// length prefix: SetFileData receives MessagePackSerializer.Serialize's bytes
// directly, and LoadPresetOrder passes the complete VirtualFile bytes back to
// MessagePackSerializer.Deserialize.
//
// Indexed-object compatibility matches MessagePack-CSharp while preserving the
// original wire shape: missing fields remain absent and unknown future fields
// are consumed and retained verbatim. The empty Migrate implementation is not
// invoked and a legacy version is not rewritten after decoding. A nil root and
// nil idOrderList are both retained as wire values.
func DecodeColorPresetOrderList(data []byte) (*ColorPresetOrderList, error) {
	raw, err := ct.DecompressLz4BlockArray(data)
	if err != nil {
		return nil, fmt.Errorf("decompress ColorPresetOrderList Lz4BlockArray: %w", err)
	}

	r := simpleEditDataReader{data: raw}
	if r.tryReadNil() {
		trailing, err := messagePackRootTrailingAfterParsed(raw, r.pos, "ColorPresetOrderList")
		if err != nil {
			return nil, err
		}
		if len(trailing) != 0 {
			return &ColorPresetOrderList{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
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

	value := &ColorPresetOrderList{}
	if fieldCount != 2 {
		storedFieldCount := fieldCount
		value.FieldCount = &storedFieldCount
	}
	if fieldCount > 2 {
		value.FutureSlots = makeKCESCountedSliceForAppend[[]byte](uint64(fieldCount - 2))
	}
	if fieldCount >= 1 {
		value.Version, err = r.readInt32("ColorPresetOrderList.version")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 2 {
		value.IDOrderList, err = readColorPresetOrderStringList(&r)
		if err != nil {
			return nil, err
		}
	}
	for field := 2; field < fieldCount; field++ {
		start := r.pos
		if err := r.skipValue(0); err != nil {
			return nil, fmt.Errorf("skip ColorPresetOrderList future Key(%d): %w", field, err)
		}
		value.FutureSlots = append(value.FutureSlots, append([]byte(nil), r.data[start:r.pos]...))
	}
	trailing, err := messagePackRootTrailingAfterParsed(raw, r.pos, "ColorPresetOrderList")
	if err != nil {
		return nil, err
	}
	value.TrailingData = trailing
	return value, nil
}

// EncodeColorPresetOrderList emits the indexed-object width represented by
// FieldCount and FutureSlots using StandardLz4BlockArray, and preserves the
// caller's version verbatim.
func EncodeColorPresetOrderList(value *ColorPresetOrderList) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if raw, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.Version != 0 || value.IDOrderList != nil || value.FieldCount != nil || len(value.FutureSlots) != 0,
		"ColorPresetOrderList",
	); handled {
		if err != nil {
			return nil, err
		}
		return encodeColorPresetOrderListRaw(raw)
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 2, value.FutureSlots, "ColorPresetOrderList")
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.Version != 0 {
		return nil, fmt.Errorf("ColorPresetOrderList fieldCount %d would discard version=%d", fieldCount, value.Version)
	}
	if fieldCount < 2 && value.IDOrderList != nil {
		return nil, fmt.Errorf("ColorPresetOrderList fieldCount %d would discard idOrderList", fieldCount)
	}
	if err := requireInt32("ColorPresetOrderList.version", value.Version); err != nil {
		return nil, err
	}
	if uint64(len(value.IDOrderList)) > math.MaxUint32 {
		return nil, fmt.Errorf("ColorPresetOrderList.idOrderList length %d exceeds the MessagePack array32 limit", len(value.IDOrderList))
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

	raw := simpleEditDataAppendArrayHeader(nil, fieldCount)
	if fieldCount >= 1 {
		raw = simpleEditDataAppendInt32(raw, value.Version)
	}
	if fieldCount >= 2 {
		if value.IDOrderList == nil {
			raw = append(raw, 0xc0)
		} else {
			raw = simpleEditDataAppendArrayHeader(raw, len(value.IDOrderList))
			for _, id := range value.IDOrderList {
				if id == nil {
					raw = append(raw, 0xc0)
				} else {
					raw = simpleEditDataAppendString(raw, *id)
				}
			}
		}
	}
	for _, slot := range value.FutureSlots {
		raw = append(raw, slot...)
	}
	raw = appendMessagePackRootTrailing(raw, value.MessagePackRootMetadata)
	return encodeColorPresetOrderListRaw(raw)
}

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
	for index := 0; index < count; index++ {
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
