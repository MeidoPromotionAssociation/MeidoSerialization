package KCES

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
)

// system.dat 内 EditData/GradSv{n} 虚拟文件的渐变点 MessagePack 布局。
// 该载荷没有独立磁盘扩展名。
//
// MessagePack layout for EditData/GradSv{n} gradient-point virtual files inside system.dat.
// This payload has no standalone disk extension.

const (
	maxGradPointsMessagePackDepth = 256
)

// GradPointsData is the raw Standard MessagePack payload stored in
// system.dat/EditData/GradSv{index}. It matches the indexed-array fields in
// the game's GradPointsData class. GradaPointPosRates and EditMPN are legacy
// fields which the current UI does not read, but they remain part of the wire
// contract and are preserved.
type GradPointsData struct {
	MessagePackRootMetadata
	GradPointParam        []map[int32]int32 `json:"gradPointParam"`
	ControlPointPosValue  []float32         `json:"controlPointPosValue"`
	GradaPointPosRates    []float32         `json:"gradaPointPosRates"`
	EditMPN               int32             `json:"editMpn"`
	PointRangeAfterRates  []float32         `json:"pointRangeAfterRates"`
	PointRangeBeforeRates []float32         `json:"pointRangeBeforeRates"`
	IsSave                int32             `json:"isSave"`
	FieldCount            *int32            `json:"fieldCount,omitempty"`
	FutureSlots           [][]byte          `json:"futureSlots,omitempty"`
}

// NewGradPointsData explicitly returns the current game's field-initializer
// defaults for callers creating a new object.
func NewGradPointsData() *GradPointsData {
	return &GradPointsData{
		GradPointParam:        []map[int32]int32{},
		ControlPointPosValue:  []float32{},
		GradaPointPosRates:    []float32{},
		PointRangeAfterRates:  []float32{},
		PointRangeBeforeRates: []float32{},
	}
}

// DecodeGradPointsData decodes one uncompressed Standard MessagePack value.
// Missing slots retain wire-model zero values and unknown future slots are
// consumed and retained verbatim. The root boundary is located by the shared
// MessagePack library and any remaining bytes are retained in TrailingData.
func DecodeGradPointsData(data []byte) (*GradPointsData, error) {
	reader := gradPointsMessagePackReader{data: data}
	if len(data) > 0 && data[0] == 0xc0 {
		reader.pos = 1
		trailing, err := messagePackRootTrailingAfterParsed(data, reader.pos, "GradPointsData")
		if err != nil {
			return nil, err
		}
		// The generated class formatter returns null. GradationColorBall checks
		// the result before it calls GradPointParamToPartsColors, so this root
		// nil is a safe and meaningful absence value.
		if len(trailing) != 0 {
			return &GradPointsData{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
		}
		return nil, nil
	}
	fieldCount, err := reader.readArrayLength("GradPointsData")
	if err != nil {
		return nil, err
	}

	value := &GradPointsData{}
	if fieldCount != 7 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount > 7 {
		value.FutureSlots = makeKCESCountedSliceForAppend[[]byte](uint64(fieldCount - 7))
	}
	knownFields := min(fieldCount, 7)
	for field := int64(0); field < knownFields; field++ {
		switch field {
		case 0:
			value.GradPointParam, err = reader.readColorMapList("gradPointParam")
		case 1:
			value.ControlPointPosValue, err = reader.readFloat32List("controlPointPosValue")
		case 2:
			value.GradaPointPosRates, err = reader.readFloat32List("gradaPointPosRates")
		case 3:
			value.EditMPN, err = reader.readInt32("editMpn")
		case 4:
			value.PointRangeAfterRates, err = reader.readFloat32List("pointRangeAfterRates")
		case 5:
			value.PointRangeBeforeRates, err = reader.readFloat32List("pointRangeBeforeRates")
		case 6:
			value.IsSave, err = reader.readInt32("isSave")
		}
		if err != nil {
			return nil, err
		}
	}
	for field := int64(7); field < fieldCount; field++ {
		start := reader.pos
		if err := reader.skipValue(0); err != nil {
			return nil, fmt.Errorf("GradPointsData future field[%d]: %w", field, err)
		}
		value.FutureSlots = append(value.FutureSlots, append([]byte(nil), reader.data[start:reader.pos]...))
	}
	trailing, err := messagePackRootTrailingAfterParsed(data, reader.pos, "GradPointsData")
	if err != nil {
		return nil, err
	}
	if err := validateGradPointsData(value); err != nil {
		return nil, fmt.Errorf("validate decoded GradPointsData: %w", err)
	}
	value.TrailingData = trailing
	return value, nil
}

// EncodeGradPointsData encodes the seven current indexed fields as raw
// Standard MessagePack. Dictionary keys are emitted in ascending numeric
// order so output is deterministic. The caller's slices and maps are only
// read and are never reordered or otherwise modified.
func EncodeGradPointsData(value *GradPointsData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if out, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.GradPointParam != nil || value.ControlPointPosValue != nil || value.GradaPointPosRates != nil ||
			value.EditMPN != 0 || value.PointRangeAfterRates != nil || value.PointRangeBeforeRates != nil ||
			value.IsSave != 0 || value.FieldCount != nil || len(value.FutureSlots) != 0,
		"GradPointsData",
	); handled {
		return out, err
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 7, value.FutureSlots, "GradPointsData")
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.GradPointParam != nil {
		return nil, fmt.Errorf("GradPointsData fieldCount %d would discard gradPointParam", fieldCount)
	}
	if fieldCount < 2 && value.ControlPointPosValue != nil {
		return nil, fmt.Errorf("GradPointsData fieldCount %d would discard controlPointPosValue", fieldCount)
	}
	if fieldCount < 3 && value.GradaPointPosRates != nil {
		return nil, fmt.Errorf("GradPointsData fieldCount %d would discard gradaPointPosRates", fieldCount)
	}
	if fieldCount < 4 && value.EditMPN != 0 {
		return nil, fmt.Errorf("GradPointsData fieldCount %d would discard editMpn=%d", fieldCount, value.EditMPN)
	}
	if fieldCount < 5 && value.PointRangeAfterRates != nil {
		return nil, fmt.Errorf("GradPointsData fieldCount %d would discard pointRangeAfterRates", fieldCount)
	}
	if fieldCount < 6 && value.PointRangeBeforeRates != nil {
		return nil, fmt.Errorf("GradPointsData fieldCount %d would discard pointRangeBeforeRates", fieldCount)
	}
	if fieldCount < 7 && value.IsSave != 0 {
		return nil, fmt.Errorf("GradPointsData fieldCount %d would discard isSave=%d", fieldCount, value.IsSave)
	}
	if err := validateGradPointsData(value); err != nil {
		return nil, fmt.Errorf("validate GradPointsData: %w", err)
	}

	// Preserve the stored indexed-object width; newly constructed values use
	// all seven known fields.
	out := make([]byte, 0, kcesInitialCollectionCapacity)
	out = appendGradPointsArrayHeader(out, fieldCount)
	if fieldCount >= 1 {
		if value.GradPointParam == nil {
			out = append(out, 0xc0)
		} else {
			out = appendGradPointsArrayHeader(out, int64(len(value.GradPointParam)))
			for _, color := range value.GradPointParam {
				if color == nil {
					out = append(out, 0xc0)
					continue
				}
				out = appendGradPointsMapHeader(out, int64(len(color)))
				keys := make([]int32, 0, len(color))
				for key := range color {
					keys = append(keys, key)
				}
				slices.Sort(keys)
				for _, key := range keys {
					out = appendGradPointsInt32(out, key)
					out = appendGradPointsInt32(out, color[key])
				}
			}
		}
	}
	if fieldCount >= 2 {
		out = appendGradPointsFloat32List(out, value.ControlPointPosValue)
	}
	if fieldCount >= 3 {
		out = appendGradPointsFloat32List(out, value.GradaPointPosRates)
	}
	if fieldCount >= 4 {
		out = appendGradPointsInt32(out, value.EditMPN)
	}
	if fieldCount >= 5 {
		out = appendGradPointsFloat32List(out, value.PointRangeAfterRates)
	}
	if fieldCount >= 6 {
		out = appendGradPointsFloat32List(out, value.PointRangeBeforeRates)
	}
	if fieldCount >= 7 {
		out = appendGradPointsInt32(out, value.IsSave)
	}
	for _, slot := range value.FutureSlots {
		out = append(out, slot...)
	}
	return appendMessagePackRootTrailing(out, value.MessagePackRootMetadata), nil
}

func validateGradPointsData(value *GradPointsData) error {
	if value == nil {
		return nil
	}
	lists := []struct {
		name   string
		length int64
	}{
		{name: "gradPointParam", length: int64(len(value.GradPointParam))},
		{name: "controlPointPosValue", length: int64(len(value.ControlPointPosValue))},
		{name: "gradaPointPosRates", length: int64(len(value.GradaPointPosRates))},
		{name: "pointRangeAfterRates", length: int64(len(value.PointRangeAfterRates))},
		{name: "pointRangeBeforeRates", length: int64(len(value.PointRangeBeforeRates))},
	}
	for _, list := range lists {
		if uint64(list.length) > math.MaxUint32 {
			return fmt.Errorf("%s length %d exceeds the MessagePack array32 limit", list.name, list.length)
		}
	}

	for index, color := range value.GradPointParam {
		if color == nil {
			continue
		}
		if uint64(len(color)) > math.MaxUint32 {
			return fmt.Errorf("gradPointParam[%d] map length %d exceeds the MessagePack map32 limit", index, len(color))
		}
	}

	return nil
}

type gradPointsMessagePackReader struct {
	data []byte
	pos  int64
}

func (r *gradPointsMessagePackReader) tryReadNil() bool {
	if r.pos < int64(len(r.data)) && r.data[r.pos] == 0xc0 {
		r.pos++
		return true
	}
	return false
}

func (r *gradPointsMessagePackReader) readByte(context string) (byte, error) {
	if r.pos >= int64(len(r.data)) {
		return 0, fmt.Errorf("%s is truncated at byte %d", context, r.pos)
	}
	value := r.data[r.pos]
	r.pos++
	return value, nil
}

func (r *gradPointsMessagePackReader) readBytes(length int64, context string) ([]byte, error) {
	if length < 0 || length > int64(len(r.data))-r.pos {
		return nil, fmt.Errorf("%s is truncated at byte %d: need %d bytes, have %d", context, r.pos, length, int64(len(r.data))-r.pos)
	}
	value := r.data[r.pos : r.pos+length]
	r.pos += length
	return value, nil
}

func (r *gradPointsMessagePackReader) readArrayLength(context string) (int64, error) {
	code, err := r.readByte(context + " array header")
	if err != nil {
		return 0, err
	}
	var count uint64
	switch {
	case code >= 0x90 && code <= 0x9f:
		count = uint64(code & 0x0f)
	case code == 0xdc:
		bytes, err := r.readBytes(2, context+" array16 header")
		if err != nil {
			return 0, err
		}
		count = uint64(binary.BigEndian.Uint16(bytes))
	case code == 0xdd:
		bytes, err := r.readBytes(4, context+" array32 header")
		if err != nil {
			return 0, err
		}
		count = uint64(binary.BigEndian.Uint32(bytes))
	case code == 0xc0:
		return 0, fmt.Errorf("%s array must not be nil", context)
	default:
		return 0, fmt.Errorf("%s must be a MessagePack array, got marker 0x%02x", context, code)
	}
	// Every MessagePack value consumes at least one byte. This catches a huge
	// declared collection before allocating.
	if count > uint64(int64(len(r.data))-r.pos) {
		return 0, fmt.Errorf("%s array is truncated: declares %d values with only %d bytes remaining", context, count, int64(len(r.data))-r.pos)
	}
	return int64(count), nil
}

func (r *gradPointsMessagePackReader) readMapLength(context string) (int64, error) {
	code, err := r.readByte(context + " map header")
	if err != nil {
		return 0, err
	}
	var count uint64
	switch {
	case code >= 0x80 && code <= 0x8f:
		count = uint64(code & 0x0f)
	case code == 0xde:
		bytes, err := r.readBytes(2, context+" map16 header")
		if err != nil {
			return 0, err
		}
		count = uint64(binary.BigEndian.Uint16(bytes))
	case code == 0xdf:
		bytes, err := r.readBytes(4, context+" map32 header")
		if err != nil {
			return 0, err
		}
		count = uint64(binary.BigEndian.Uint32(bytes))
	case code == 0xc0:
		return 0, fmt.Errorf("%s map must not be nil", context)
	default:
		return 0, fmt.Errorf("%s must be a MessagePack map, got marker 0x%02x", context, code)
	}
	if count*2 > uint64(int64(len(r.data))-r.pos) {
		return 0, fmt.Errorf("%s map is truncated: declares %d entries with only %d bytes remaining", context, count, int64(len(r.data))-r.pos)
	}
	return int64(count), nil
}

func (r *gradPointsMessagePackReader) readColorMapList(context string) ([]map[int32]int32, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := r.readArrayLength(context)
	if err != nil {
		return nil, err
	}
	values := makeKCESCountedSliceForAppend[map[int32]int32](uint64(count))
	for index := int64(0); index < count; index++ {
		entryContext := fmt.Sprintf("%s[%d]", context, index)
		if r.tryReadNil() {
			values = append(values, nil)
			continue
		}
		entryCount, err := r.readMapLength(entryContext)
		if err != nil {
			return nil, err
		}
		entry := makeKCESCountedMap[int32, int32](uint64(entryCount))
		for pair := int64(0); pair < entryCount; pair++ {
			key, err := r.readInt32(fmt.Sprintf("%s key[%d]", entryContext, pair))
			if err != nil {
				return nil, err
			}
			if _, exists := entry[key]; exists {
				return nil, fmt.Errorf("%s contains duplicate Int32 key %d", entryContext, key)
			}
			mapValue, err := r.readInt32(fmt.Sprintf("%s[%d]", entryContext, key))
			if err != nil {
				return nil, err
			}
			entry[key] = mapValue
		}
		values = append(values, entry)
	}
	return values, nil
}

func (r *gradPointsMessagePackReader) readFloat32List(context string) ([]float32, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := r.readArrayLength(context)
	if err != nil {
		return nil, err
	}
	values := makeKCESCountedSliceForAppend[float32](uint64(count))
	for index := int64(0); index < count; index++ {
		value, err := r.readSingle(fmt.Sprintf("%s[%d]", context, index))
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

// readSingle mirrors MessagePackReader.ReadSingle rather than accepting only
// the writer's canonical 0xca marker. MessagePack-CSharp also accepts float64
// and every integer marker here, converting the numeric value to System.Single.
func (r *gradPointsMessagePackReader) readSingle(context string) (float32, error) {
	code, err := r.readByte(context + " Single")
	if err != nil {
		return 0, err
	}
	if code <= 0x7f {
		return float32(code), nil
	}
	if code >= 0xe0 {
		return float32(int8(code)), nil
	}

	switch code {
	case 0xca:
		bytes, err := r.readBytes(4, context+" float32")
		if err != nil {
			return 0, err
		}
		return math.Float32frombits(binary.BigEndian.Uint32(bytes)), nil
	case 0xcb:
		bytes, err := r.readBytes(8, context+" float64")
		if err != nil {
			return 0, err
		}
		return float32(math.Float64frombits(binary.BigEndian.Uint64(bytes))), nil
	case 0xcc:
		bytes, err := r.readBytes(1, context+" uint8")
		if err != nil {
			return 0, err
		}
		return float32(bytes[0]), nil
	case 0xcd:
		bytes, err := r.readBytes(2, context+" uint16")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint16(bytes)), nil
	case 0xce:
		bytes, err := r.readBytes(4, context+" uint32")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint32(bytes)), nil
	case 0xcf:
		bytes, err := r.readBytes(8, context+" uint64")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint64(bytes)), nil
	case 0xd0:
		bytes, err := r.readBytes(1, context+" int8")
		if err != nil {
			return 0, err
		}
		return float32(int8(bytes[0])), nil
	case 0xd1:
		bytes, err := r.readBytes(2, context+" int16")
		if err != nil {
			return 0, err
		}
		return float32(int16(binary.BigEndian.Uint16(bytes))), nil
	case 0xd2:
		bytes, err := r.readBytes(4, context+" int32")
		if err != nil {
			return 0, err
		}
		return float32(int32(binary.BigEndian.Uint32(bytes))), nil
	case 0xd3:
		bytes, err := r.readBytes(8, context+" int64")
		if err != nil {
			return 0, err
		}
		return float32(int64(binary.BigEndian.Uint64(bytes))), nil
	default:
		return 0, fmt.Errorf("%s must be a MessagePack number accepted by ReadSingle, got marker 0x%02x", context, code)
	}
}

func (r *gradPointsMessagePackReader) readInt32(context string) (int32, error) {
	code, err := r.readByte(context + " Int32")
	if err != nil {
		return 0, err
	}
	if code <= 0x7f {
		return int32(code), nil
	}
	if code >= 0xe0 {
		return int32(int8(code)), nil
	}

	var signed int64
	var unsigned uint64
	var isUnsigned bool
	switch code {
	case 0xcc:
		bytes, err := r.readBytes(1, context+" uint8")
		if err != nil {
			return 0, err
		}
		unsigned, isUnsigned = uint64(bytes[0]), true
	case 0xcd:
		bytes, err := r.readBytes(2, context+" uint16")
		if err != nil {
			return 0, err
		}
		unsigned, isUnsigned = uint64(binary.BigEndian.Uint16(bytes)), true
	case 0xce:
		bytes, err := r.readBytes(4, context+" uint32")
		if err != nil {
			return 0, err
		}
		unsigned, isUnsigned = uint64(binary.BigEndian.Uint32(bytes)), true
	case 0xcf:
		bytes, err := r.readBytes(8, context+" uint64")
		if err != nil {
			return 0, err
		}
		unsigned, isUnsigned = binary.BigEndian.Uint64(bytes), true
	case 0xd0:
		bytes, err := r.readBytes(1, context+" int8")
		if err != nil {
			return 0, err
		}
		signed = int64(int8(bytes[0]))
	case 0xd1:
		bytes, err := r.readBytes(2, context+" int16")
		if err != nil {
			return 0, err
		}
		signed = int64(int16(binary.BigEndian.Uint16(bytes)))
	case 0xd2:
		bytes, err := r.readBytes(4, context+" int32")
		if err != nil {
			return 0, err
		}
		signed = int64(int32(binary.BigEndian.Uint32(bytes)))
	case 0xd3:
		bytes, err := r.readBytes(8, context+" int64")
		if err != nil {
			return 0, err
		}
		signed = int64(binary.BigEndian.Uint64(bytes))
	default:
		return 0, fmt.Errorf("%s must be a MessagePack Int32, got marker 0x%02x", context, code)
	}
	if isUnsigned {
		if unsigned > uint64(math.MaxInt32) {
			return 0, fmt.Errorf("%s unsigned value %d is outside the Int32 range [%d,%d]", context, unsigned, int64(math.MinInt32), int64(math.MaxInt32))
		}
		return int32(unsigned), nil
	}
	if signed < math.MinInt32 || signed > math.MaxInt32 {
		return 0, fmt.Errorf("%s signed value %d is outside the Int32 range [%d,%d]", context, signed, int64(math.MinInt32), int64(math.MaxInt32))
	}
	return int32(signed), nil
}

func (r *gradPointsMessagePackReader) skipValue(depth int64) error {
	if depth >= maxGradPointsMessagePackDepth {
		return fmt.Errorf("MessagePack nesting exceeds safety limit %d", maxGradPointsMessagePackDepth)
	}
	code, err := r.readByte("MessagePack value")
	if err != nil {
		return err
	}
	switch {
	case code <= 0x7f || code >= 0xe0:
		return nil
	case code >= 0xa0 && code <= 0xbf:
		_, err = r.readBytes(int64(code&0x1f), "fixstr payload")
		return err
	case code >= 0x90 && code <= 0x9f:
		return r.skipValues(int64(code&0x0f), depth+1)
	case code >= 0x80 && code <= 0x8f:
		return r.skipValues(int64(code&0x0f)*2, depth+1)
	}

	switch code {
	case 0xc0, 0xc2, 0xc3:
		return nil
	case 0xc1:
		return fmt.Errorf("reserved MessagePack marker 0xc1")
	case 0xc4, 0xd9:
		length, err := r.readUnsignedLength(1, "8-bit length")
		if err != nil {
			return err
		}
		_, err = r.readBytes(length, "MessagePack payload")
		return err
	case 0xc5, 0xda:
		length, err := r.readUnsignedLength(2, "16-bit length")
		if err != nil {
			return err
		}
		_, err = r.readBytes(length, "MessagePack payload")
		return err
	case 0xc6, 0xdb:
		length, err := r.readUnsignedLength(4, "32-bit length")
		if err != nil {
			return err
		}
		_, err = r.readBytes(length, "MessagePack payload")
		return err
	case 0xc7, 0xc8, 0xc9:
		width := map[byte]int64{0xc7: 1, 0xc8: 2, 0xc9: 4}[code]
		length, err := r.readUnsignedLength(width, "extension length")
		if err != nil {
			return err
		}
		_, err = r.readBytes(length+1, "extension type and payload")
		return err
	case 0xca, 0xce, 0xd2:
		_, err = r.readBytes(4, "four-byte MessagePack scalar")
		return err
	case 0xcb, 0xcf, 0xd3:
		_, err = r.readBytes(8, "eight-byte MessagePack scalar")
		return err
	case 0xcc, 0xd0:
		_, err = r.readBytes(1, "one-byte MessagePack scalar")
		return err
	case 0xcd, 0xd1:
		_, err = r.readBytes(2, "two-byte MessagePack scalar")
		return err
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8:
		length := map[byte]int64{0xd4: 1, 0xd5: 2, 0xd6: 4, 0xd7: 8, 0xd8: 16}[code]
		_, err = r.readBytes(length+1, "fixed extension type and payload")
		return err
	case 0xdc, 0xdd:
		width := map[byte]int64{0xdc: 2, 0xdd: 4}[code]
		count, err := r.readUnsignedLength(width, "array length")
		if err != nil {
			return err
		}
		return r.skipValues(count, depth+1)
	case 0xde, 0xdf:
		width := map[byte]int64{0xde: 2, 0xdf: 4}[code]
		count, err := r.readUnsignedLength(width, "map length")
		if err != nil {
			return err
		}
		return r.skipValues(count*2, depth+1)
	default:
		return fmt.Errorf("unsupported MessagePack marker 0x%02x", code)
	}
}

func (r *gradPointsMessagePackReader) skipValues(count, depth int64) error {
	if count > int64(len(r.data))-r.pos {
		return fmt.Errorf("MessagePack collection is truncated: declares %d values with only %d bytes remaining", count, int64(len(r.data))-r.pos)
	}
	for index := int64(0); index < count; index++ {
		if err := r.skipValue(depth); err != nil {
			return fmt.Errorf("value[%d]: %w", index, err)
		}
	}
	return nil
}

func (r *gradPointsMessagePackReader) readUnsignedLength(width int64, context string) (int64, error) {
	bytes, err := r.readBytes(width, context)
	if err != nil {
		return 0, err
	}
	var value uint64
	switch width {
	case 1:
		value = uint64(bytes[0])
	case 2:
		value = uint64(binary.BigEndian.Uint16(bytes))
	case 4:
		value = uint64(binary.BigEndian.Uint32(bytes))
	default:
		return 0, fmt.Errorf("internal invalid MessagePack length width %d", width)
	}
	return int64(value), nil
}

func appendGradPointsFloat32List(out []byte, values []float32) []byte {
	if values == nil {
		return append(out, 0xc0)
	}
	out = appendGradPointsArrayHeader(out, int64(len(values)))
	for _, value := range values {
		out = append(out, 0xca)
		var bytes [4]byte
		binary.BigEndian.PutUint32(bytes[:], math.Float32bits(value))
		out = append(out, bytes[:]...)
	}
	return out
}

func appendGradPointsArrayHeader(out []byte, length int64) []byte {
	switch {
	case length <= 15:
		return append(out, 0x90|byte(length))
	case length <= math.MaxUint16:
		out = append(out, 0xdc, 0, 0)
		binary.BigEndian.PutUint16(out[len(out)-2:], uint16(length))
		return out
	default:
		out = append(out, 0xdd, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(out[len(out)-4:], uint32(length))
		return out
	}
}

func appendGradPointsMapHeader(out []byte, length int64) []byte {
	switch {
	case length <= 15:
		return append(out, 0x80|byte(length))
	case length <= math.MaxUint16:
		out = append(out, 0xde, 0, 0)
		binary.BigEndian.PutUint16(out[len(out)-2:], uint16(length))
		return out
	default:
		out = append(out, 0xdf, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(out[len(out)-4:], uint32(length))
		return out
	}
}

func appendGradPointsInt32(out []byte, value int32) []byte {
	switch {
	case value >= 0 && value <= 0x7f:
		return append(out, byte(value))
	case value >= -32 && value < 0:
		return append(out, byte(int8(value)))
	case value >= 0 && value <= math.MaxUint8:
		return append(out, 0xcc, byte(value))
	case value >= 0 && value <= math.MaxUint16:
		out = append(out, 0xcd, 0, 0)
		binary.BigEndian.PutUint16(out[len(out)-2:], uint16(value))
		return out
	case value >= 0:
		out = append(out, 0xce, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(out[len(out)-4:], uint32(value))
		return out
	case value >= math.MinInt8:
		return append(out, 0xd0, byte(int8(value)))
	case value >= math.MinInt16:
		out = append(out, 0xd1, 0, 0)
		binary.BigEndian.PutUint16(out[len(out)-2:], uint16(int16(value)))
		return out
	default:
		out = append(out, 0xd2, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(out[len(out)-4:], uint32(int32(value)))
		return out
	}
}
