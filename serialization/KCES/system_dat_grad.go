package KCES

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
)

// system.dat 内 EditData/GradSv{n} 虚拟文件的渐变点 MessagePack 布局
// 该载荷没有独立磁盘扩展名
// MessagePack layout for EditData/GradSv{n} gradation-point virtual files inside system.dat
// This payload has no standalone disk extension

// GradPointsData 表示 system.dat/EditData/GradSv 加索引文件中的类型化 Standard MessagePack 对象
// 它对应游戏 GradPointsData 类的固定 indexed array 字段，GradationPointPositionRates 与 EditMPN 是当前界面不读取但仍按真实类型建模的旧字段
// GradPointsData represents the typed Standard MessagePack object stored in a system.dat/EditData/GradSv file suffixed by its index
// It corresponds to the fixed indexed-array fields of the game GradPointsData class, while GradationPointPositionRates and EditMPN are legacy fields not read by the current UI but still modeled with their real types
type GradPointsData struct {
	GradPointParam              []map[int32]int32 `json:"gradPointParam"`              // Key 0 的各渐变点颜色参数字典，键布局与 PaletteColorSaveData 相同 / Per-point color-parameter maps at Key 0 using the same key layout as PaletteColorSaveData
	ControlPointPosValue        []float32         `json:"controlPointPosValue"`        // Key 1 的各渐变控制点中心位置 / Center positions of gradation control points at Key 1
	GradationPointPositionRates []float32         `json:"gradationPointPositionRates"` // Key 2 的旧式渐变点位置比例，当前游戏界面不读取 / Legacy gradation-point position rates at Key 2, not read by the current game UI
	EditMPN                     int32             `json:"editMpn"`                     // Key 3 的旧式编辑 MPN 整数，当前游戏界面不读取 / Legacy edited MPN integer at Key 3, not read by the current game UI
	PointRangeAfterRates        []float32         `json:"pointRangeAfterRates"`        // Key 4 的各控制点后侧范围值 / After-range values for control points at Key 4
	PointRangeBeforeRates       []float32         `json:"pointRangeBeforeRates"`       // Key 5 的各控制点前侧范围值 / Before-range values for control points at Key 5
	IsSave                      int32             `json:"isSave"`                      // Key 6 的保存状态，游戏以 1 表示已保存 / Save state at Key 6, with 1 indicating saved in the game
}

// NewGradPointsData 显式返回当前游戏字段初始化器为新对象创建的默认空列表
// NewGradPointsData explicitly returns the default empty lists created by the current game field initializers for a new object
func NewGradPointsData() *GradPointsData {
	return &GradPointsData{
		GradPointParam:              []map[int32]int32{},
		ControlPointPosValue:        []float32{},
		GradationPointPositionRates: []float32{},
		PointRangeAfterRates:        []float32{},
		PointRangeBeforeRates:       []float32{},
	}
}

// DecodeGradPointsData 解码一个固定七槽且完整消费输入的未压缩 Standard MessagePack 值
// DecodeGradPointsData decodes one uncompressed Standard MessagePack value with the fixed seven-slot layout and consumes the complete input
func DecodeGradPointsData(data []byte) (*GradPointsData, error) {
	reader := gradPointsMessagePackReader{data: data}
	if len(data) > 0 && data[0] == 0xc0 {
		reader.pos = 1
		// 生成的类格式化器对 nil 根值返回 null
		// GradationColorBall 会在调用 GradPointParamToPartsColors 前检查结果，因此此 nil 根值是有意义的缺失状态
		// The generated class formatter returns null for a nil root
		// GradationColorBall checks the result before calling GradPointParamToPartsColors, so this nil root is a meaningful absence state
		if reader.pos != int64(len(data)) {
			return nil, fmt.Errorf("GradPointsData has %d trailing bytes", int64(len(data))-reader.pos)
		}
		return nil, nil
	}
	fieldCount, err := reader.readArrayLength("GradPointsData")
	if err != nil {
		return nil, err
	}

	if fieldCount != 7 {
		return nil, fmt.Errorf("unsupported GradPointsData indexed-array width %d, expected 7", fieldCount)
	}
	value := &GradPointsData{}
	for field := int64(0); field < 7; field++ {
		switch field {
		case 0:
			value.GradPointParam, err = reader.readColorMapList("gradPointParam")
		case 1:
			value.ControlPointPosValue, err = reader.readFloat32List("controlPointPosValue")
		case 2:
			value.GradationPointPositionRates, err = reader.readFloat32List("gradationPointPositionRates")
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
	if reader.pos != int64(len(data)) {
		return nil, fmt.Errorf("GradPointsData has %d trailing bytes", int64(len(data))-reader.pos)
	}
	if err := validateGradPointsData(value); err != nil {
		return nil, fmt.Errorf("validate decoded GradPointsData: %w", err)
	}
	return value, nil
}

// EncodeGradPointsData 将当前七个类型化 indexed 字段编码为 Standard MessagePack
// 字典键按数值升序写出以得到确定输出，调用者的切片与映射只读且不会被重排或修改
// EncodeGradPointsData encodes the seven current typed indexed fields as Standard MessagePack
// Dictionary keys are emitted in ascending numeric order for deterministic output while caller slices and maps are only read and never reordered or modified
func EncodeGradPointsData(value *GradPointsData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if err := validateGradPointsData(value); err != nil {
		return nil, fmt.Errorf("validate GradPointsData: %w", err)
	}

	out := make([]byte, 0, kcesInitialCollectionCapacity)
	out = appendGradPointsArrayHeader(out, 7)
	{
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
	{
		out = appendGradPointsFloat32List(out, value.ControlPointPosValue)
	}
	{
		out = appendGradPointsFloat32List(out, value.GradationPointPositionRates)
	}
	{
		out = appendGradPointsInt32(out, value.EditMPN)
	}
	{
		out = appendGradPointsFloat32List(out, value.PointRangeAfterRates)
	}
	{
		out = appendGradPointsFloat32List(out, value.PointRangeBeforeRates)
	}
	{
		out = appendGradPointsInt32(out, value.IsSave)
	}
	return out, nil
}

// validateGradPointsData 验证集合长度、颜色字典整数以及标量字段均可由目标 MessagePack 类型表示
// validateGradPointsData verifies that collection lengths, color-map integers, and scalar fields fit their target MessagePack types
func validateGradPointsData(value *GradPointsData) error {
	if value == nil {
		return nil
	}
	lists := []struct {
		name   string // 用于错误路径的列表名称 / List name used in error paths
		length int64  // 待验证的列表长度 / List length to validate
	}{
		{name: "gradPointParam", length: int64(len(value.GradPointParam))},
		{name: "controlPointPosValue", length: int64(len(value.ControlPointPosValue))},
		{name: "gradationPointPositionRates", length: int64(len(value.GradationPointPositionRates))},
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

// gradPointsMessagePackReader 提供 GradPointsData 所需的有界 MessagePack 数值和集合读取
// gradPointsMessagePackReader provides bounded MessagePack numeric and collection reads needed by GradPointsData
type gradPointsMessagePackReader struct {
	data []byte // 完整输入字节 / Complete input bytes
	pos  int64  // 下一个待读取字节的位置 / Position of the next byte to read
}

// tryReadNil 在当前位置为 MessagePack nil 时消费它并返回 true
// tryReadNil consumes MessagePack nil at the current position and reports whether it was present
func (r *gradPointsMessagePackReader) tryReadNil() bool {
	if r.pos < int64(len(r.data)) && r.data[r.pos] == 0xc0 {
		r.pos++
		return true
	}
	return false
}

// readByte 读取单个字节并在截断时附加上下文和偏移
// readByte reads one byte and annotates truncation errors with context and offset
func (r *gradPointsMessagePackReader) readByte(context string) (byte, error) {
	if r.pos >= int64(len(r.data)) {
		return 0, fmt.Errorf("%s is truncated at byte %d", context, r.pos)
	}
	value := r.data[r.pos]
	r.pos++
	return value, nil
}

// readBytes 读取指定长度的连续字节而不复制并推进当前位置
// readBytes reads a contiguous byte range without copying and advances the current position
func (r *gradPointsMessagePackReader) readBytes(length int64, context string) ([]byte, error) {
	if length < 0 || length > int64(len(r.data))-r.pos {
		return nil, fmt.Errorf("%s is truncated at byte %d: need %d bytes, have %d", context, r.pos, length, int64(len(r.data))-r.pos)
	}
	value := r.data[r.pos : r.pos+length]
	r.pos += length
	return value, nil
}

// readArrayLength 读取非 nil MessagePack 数组头并在分配前按剩余字节限制声明数量
// readArrayLength reads a non-nil MessagePack array header and bounds its declared count by remaining bytes before allocation
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
	// 每个 MessagePack 值至少占一个字节，可在分配前捕获声明数量巨大的集合
	// Every MessagePack value occupies at least one byte, catching a huge declared collection before allocation
	if count > uint64(int64(len(r.data))-r.pos) {
		return 0, fmt.Errorf("%s array is truncated: declares %d values with only %d bytes remaining", context, count, int64(len(r.data))-r.pos)
	}
	return int64(count), nil
}

// readMapLength 读取非 nil MessagePack 映射头并按键和值的最小字节数限制声明数量
// readMapLength reads a non-nil MessagePack map header and bounds its declared count using the minimum bytes for keys and values
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

// readColorMapList 读取可空列表以及其中可空且拒绝重复 Int32 键的颜色字典
// readColorMapList reads a nullable list whose color maps may be nil and reject duplicate Int32 keys
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

// readFloat32List 读取可空列表，并按 MessagePackReader.ReadSingle 规则转换每个数值
// readFloat32List reads a nullable list and converts each numeric value using MessagePackReader.ReadSingle rules
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

// readSingle 镜像 MessagePackReader.ReadSingle，而不只接受写入器的标准 0xca 标记
// MessagePack-CSharp 在此也接受 float64 和所有整数标记，并将数值转换为 System.Single
// readSingle mirrors MessagePackReader.ReadSingle instead of accepting only the writer canonical 0xca marker
// MessagePack-CSharp also accepts float64 and every integer marker here and converts the numeric value to System.Single
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

// readInt32 接受 MessagePack 整数编码并要求转换结果位于 System.Int32 范围
// readInt32 accepts MessagePack integer encodings and requires the converted result to fit System.Int32
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

// appendGradPointsFloat32List 写入可空的 Float32 列表并为每项使用标准 float32 标记
// appendGradPointsFloat32List writes a nullable Float32 list using the canonical float32 marker for every entry
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

// appendGradPointsArrayHeader 以可表示长度的最短 MessagePack 数组头追加元素数量
// appendGradPointsArrayHeader appends an element count using the shortest MessagePack array header that can represent it
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

// appendGradPointsMapHeader 以可表示长度的最短 MessagePack 映射头追加键值对数量
// appendGradPointsMapHeader appends a pair count using the shortest MessagePack map header that can represent it
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

// appendGradPointsInt32 以可表示给定 System.Int32 值的最短 MessagePack 整数形式追加值
// appendGradPointsInt32 appends a value using the shortest MessagePack integer form that can represent the given System.Int32
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
		binary.BigEndian.PutUint32(out[len(out)-4:], uint32(value))
		return out
	}
}
