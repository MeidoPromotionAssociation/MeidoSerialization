package ct

import (
	"encoding/binary"
	"fmt"

	"github.com/pierrec/lz4/v4"
	"github.com/ugorji/go/codec"
)

const (
	// MessagePack-CSharp 将扩展类型 98 保留给 Lz4BlockArray 头，将类型 99 保留给单个 Lz4Block
	// 对应游戏 MessagePackSerializer.ToLZ4BinaryCore 与 TryDecompress 实现
	// MessagePack-CSharp reserves extension type 98 for the Lz4BlockArray header and type 99 for a single Lz4Block
	// This corresponds to the game MessagePackSerializer.ToLZ4BinaryCore and TryDecompress implementations
	Lz4ArrayType = 98
	Lz4BlockType = 99

	blockSize = 65536

	// 本包旧版本曾写出类型号反向的私有布局，即数组、含总大小的 ext 99 以及多个块 ext 98
	// 为保持旧文件可读仍接受该布局，但绝不再写出
	// Older versions of this package emitted a private layout with reversed type numbers, consisting of an array, ext 99 carrying total size, and ext 98 blocks
	// The layout remains accepted for old-file readability but is never emitted
	legacyLz4ArrayType = 99
	legacyLz4BlockType = 98

	// 大小字段由输入控制，此限制防止微小畸形输入在 LZ4 校验前触发无界分配
	// Size fields are input-controlled, and this limit prevents a tiny malformed input from causing an unbounded allocation before LZ4 validation
	maxLz4DecompressedSize = 1 << 30

	messagePackInitialCollectionCapacity = 1024
)

// MessagePackRootMetadata 保留强类型根值之外的字节
// RootNil 仅用于 nil 根值后仍有尾部字节、因而不能只用 nil Go 指针表示的情况
// MessagePackRootMetadata preserves bytes outside a typed root value
// RootNil is used only when a nil root has trailing bytes and therefore cannot be represented by a nil Go pointer alone
type MessagePackRootMetadata struct {
	RootNil      bool   `json:"rootNil,omitempty"`      // 根 MessagePack 值是否显式为 nil / Whether the root MessagePack value was explicitly nil
	TrailingData []byte `json:"trailingData,omitempty"` // 完整根值之后逐字节保留的数据 / Data preserved byte-for-byte after the complete root value
}

// IndexedObjectMetadata 保留 MessagePack-CSharp int-key 对象的精确宽度和未知尾部 Key
// nil FieldCount 表示使用当前已知宽度，除非 FutureSlots 将其扩展
// IndexedObjectMetadata preserves the exact width and unknown trailing keys of a MessagePack-CSharp int-key object
// A nil FieldCount means the current known width unless FutureSlots extends it
type IndexedObjectMetadata struct {
	FieldCount  *int     `json:"fieldCount,omitempty"`  // 原始 indexed object 数组的槽位数 / Slot count of the original indexed object array
	FutureSlots [][]byte `json:"futureSlots,omitempty"` // 已知 Key 之后各未知槽位的完整原始 MessagePack 值 / Complete raw MessagePack values for unknown slots after known keys
}

// TypedIndexedObjectMetadata 在宽度和未来槽位元数据上增加普通强类型 Go 模型所需的 null 标注
// Go 标量、值字段和值元素切片自身无法区分 MessagePack nil 与零值，这些紧凑标注使共享 indexed object 适配器能恢复 nil 标记
// 因此编辑 JSON 无需重复保存每个已知字段的原始字节
// TypedIndexedObjectMetadata extends width and future-slot metadata with null annotations needed by ordinary typed Go models
// Go scalars, value fields, and slices of value elements cannot distinguish MessagePack nil from zero values by themselves, so these compact annotations let the shared indexed object adapter restore nil markers
// Editing JSON therefore does not need to duplicate raw bytes for every known field
type TypedIndexedObjectMetadata struct {
	FieldCount       *int             `json:"fieldCount,omitempty"`       // 原始 indexed object 数组的槽位数 / Slot count of the original indexed object array
	FutureSlots      [][]byte         `json:"futureSlots,omitempty"`      // 已知 Key 之后各未知槽位的完整原始 MessagePack 值 / Complete raw MessagePack values for unknown slots after known keys
	NilSlots         []int            `json:"nilSlots,omitempty"`         // 在线格式中显式为 nil 的已知标量或值槽位索引 / Known scalar or value slot indices explicitly nil on the wire
	NullElements     map[int][]bool   `json:"nullElements,omitempty"`     // 按槽位记录值切片中各元素的 nil 线状态 / Per-slot nil wire-state flags for elements of value slices
	NullMapValueKeys map[int][][]byte `json:"nullMapValueKeys,omitempty"` // 按槽位记录值映射中具有 nil 值的完整原始 Key / Per-slot complete raw keys whose values were nil in value maps
}

// DecompressLz4BlockArray 处理 MessagePack-CSharp 的 Lz4Block 与 Lz4BlockArray 格式
// 标准多块布局是 array N+1、含各块 MessagePack 解压大小的 ext 98 头和后续 bin 块
// 单块布局是 ext 99，其载荷为一个 MessagePack 解压大小及压缩数据
// 还会只读兼容本包旧版写出的 ext 类型号反向私有布局，但不会再写出该布局
// DecompressLz4BlockArray handles MessagePack-CSharp Lz4Block and Lz4BlockArray forms
// The standard multi-block layout is array N+1 with an ext 98 header containing MessagePack decompressed sizes followed by bin blocks
// The single-block layout is ext 99 whose payload contains one MessagePack decompressed size and compressed data
// It also read-only accepts the old private layout emitted by this package with reversed extension type numbers but never emits that layout
func DecompressLz4BlockArray(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// MessagePackCompression.Lz4Block 是顶层 ext 99，载荷为一个表示解压大小的 MessagePack 整数和后续 LZ4 字节
	// MessagePackCompression.Lz4Block is top-level ext 99 whose payload contains one MessagePack integer for decompressed size followed by LZ4 bytes
	if isExtMarker(data[0]) {
		pos := 0
		extType, payload, err := readExtPayload(data, &pos)
		if err != nil {
			return nil, err
		}
		if extType != int8(Lz4BlockType) {
			return data, nil
		}
		if pos != len(data) {
			return nil, fmt.Errorf("Lz4Block has %d trailing bytes", len(data)-pos)
		}
		return decompressSingleLz4Block(payload)
	}

	if !isArrayMarker(data[0]) {
		return data, nil
	}

	pos := 0
	arrayLen, err := readArrayHeader(data, &pos)
	if err != nil {
		return nil, err
	}
	if arrayLen < 2 || pos >= len(data) || !isExtMarker(data[pos]) {
		// 普通 MessagePack 数组未压缩，必须原样通过
		// Ordinary MessagePack arrays are uncompressed and must pass through unchanged
		return data, nil
	}
	extType, headerPayload, err := readExtPayload(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("read Lz4BlockArray header: %w", err)
	}
	switch extType {
	case int8(Lz4ArrayType):
		return decompressStandardLz4BlockArray(data, pos, arrayLen, headerPayload)
	case int8(legacyLz4ArrayType):
		return decompressLegacyLz4BlockArray(data, pos, arrayLen, headerPayload)
	default:
		return data, nil
	}
}

// decompressSingleLz4Block 解码 ext 99 载荷中的 MessagePack 解压大小并解压单块
// decompressSingleLz4Block decodes the MessagePack decompressed size in an ext 99 payload and decompresses the single block
func decompressSingleLz4Block(payload []byte) ([]byte, error) {
	uncompressedSize, consumed, err := decodeMsgpackInt(payload)
	if err != nil {
		return nil, fmt.Errorf("decode Lz4Block size: %w", err)
	}
	if err := validateLz4Size(uncompressedSize); err != nil {
		return nil, err
	}
	compressed := payload[consumed:]
	return decompressLz4Bytes(compressed, uncompressedSize, false)
}

// decompressStandardLz4BlockArray 解压标准 ext 98 大小头加 bin 块的多块布局
// decompressStandardLz4BlockArray decompresses the standard multi-block layout with an ext 98 size header and bin blocks
func decompressStandardLz4BlockArray(data []byte, pos, arrayLen int, headerPayload []byte) ([]byte, error) {
	sizes, err := decodeMsgpackIntList(headerPayload)
	if err != nil {
		return nil, fmt.Errorf("decode Lz4BlockArray sizes: %w", err)
	}
	expectedBlocks := arrayLen - 1
	if len(sizes) != expectedBlocks {
		return nil, fmt.Errorf("Lz4BlockArray sizes=%d but arrayLen-1=%d", len(sizes), expectedBlocks)
	}

	totalSize, err := sumLz4Sizes(sizes)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 0, totalSize)
	for i, uncompressedSize := range sizes {
		compressed, err := ReadBin(data, &pos)
		if err != nil {
			return nil, fmt.Errorf("read Lz4BlockArray bin[%d]: %w", i, err)
		}
		block, err := decompressLz4Bytes(compressed, uncompressedSize, false)
		if err != nil {
			return nil, fmt.Errorf("decompress Lz4BlockArray block[%d]: %w", i, err)
		}
		result = append(result, block...)
	}
	if pos != len(data) {
		return nil, fmt.Errorf("Lz4BlockArray has %d trailing bytes", len(data)-pos)
	}
	return result, nil
}

// decompressLegacyLz4BlockArray 只读解压本包旧版类型号反向的私有多块布局
// decompressLegacyLz4BlockArray read-only decompresses the old private multi-block layout from this package with reversed type numbers
func decompressLegacyLz4BlockArray(data []byte, pos, arrayLen int, headerPayload []byte) ([]byte, error) {
	totalSize, err := decodeFixedWidthInt(headerPayload)
	if err != nil {
		return nil, fmt.Errorf("decode legacy Lz4BlockArray size: %w", err)
	}
	if err := validateLz4Size(totalSize); err != nil {
		return nil, err
	}
	expectedBlocks := 0
	if totalSize > 0 {
		expectedBlocks = (totalSize + blockSize - 1) / blockSize
	}
	if arrayLen-1 != expectedBlocks {
		return nil, fmt.Errorf("legacy Lz4BlockArray blocks=%d but size %d requires %d", arrayLen-1, totalSize, expectedBlocks)
	}

	result := make([]byte, 0, totalSize)
	for i := 0; i < expectedBlocks; i++ {
		extType, blockPayload, err := readExtPayload(data, &pos)
		if err != nil {
			return nil, fmt.Errorf("read legacy Lz4BlockArray block[%d]: %w", i, err)
		}
		if extType != int8(legacyLz4BlockType) {
			return nil, fmt.Errorf("legacy block[%d]: expected ext type %d, got %d", i, legacyLz4BlockType, extType)
		}
		expectedSize := min(blockSize, totalSize-len(result))
		block, err := decompressLz4Bytes(blockPayload, expectedSize, true)
		if err != nil {
			return nil, fmt.Errorf("decompress legacy block[%d]: %w", i, err)
		}
		result = append(result, block...)
	}
	if pos != len(data) {
		return nil, fmt.Errorf("legacy Lz4BlockArray has %d trailing bytes", len(data)-pos)
	}
	return result, nil
}

// decompressLz4Bytes 将一个 LZ4 块解压到精确目标大小，并可为旧布局接受等长原始块
// decompressLz4Bytes decompresses one LZ4 block to an exact target size and may accept equal-length raw blocks for the legacy layout
func decompressLz4Bytes(compressed []byte, uncompressedSize int, allowRaw bool) ([]byte, error) {
	if err := validateLz4Size(uncompressedSize); err != nil {
		return nil, err
	}
	if uncompressedSize == 0 {
		if len(compressed) != 0 {
			return nil, fmt.Errorf("zero-size block has %d payload bytes", len(compressed))
		}
		return []byte{}, nil
	}

	dst := make([]byte, uncompressedSize)
	n, err := lz4.UncompressBlock(compressed, dst)
	if err == nil && n == uncompressedSize {
		return dst, nil
	}
	if allowRaw && len(compressed) == uncompressedSize {
		return append([]byte(nil), compressed...), nil
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("decompressed size got %d, want %d", n, uncompressedSize)
}

// validateLz4Size 验证单个或累计 LZ4 解压大小为非负且不超过安全上限
// validateLz4Size verifies that a single or accumulated LZ4 decompressed size is non-negative and within the safety limit
func validateLz4Size(size int) error {
	if size < 0 {
		return fmt.Errorf("negative LZ4 decompressed size %d", size)
	}
	if size > maxLz4DecompressedSize {
		return fmt.Errorf("LZ4 decompressed size %d exceeds limit %d", size, maxLz4DecompressedSize)
	}
	return nil
}

// sumLz4Sizes 验证各块大小并以防溢出方式计算总解压大小
// sumLz4Sizes validates block sizes and computes total decompressed size with overflow protection
func sumLz4Sizes(sizes []int) (int, error) {
	total := 0
	for i, size := range sizes {
		if err := validateLz4Size(size); err != nil {
			return 0, fmt.Errorf("block[%d]: %w", i, err)
		}
		if size > maxLz4DecompressedSize-total {
			return 0, fmt.Errorf("LZ4 total decompressed size exceeds limit %d", maxLz4DecompressedSize)
		}
		total += size
	}
	return total, nil
}

// readExtPayloadAsIntList 读取指定类型的 ext 并将载荷解析为连续 MessagePack 整数列表
// 用于 ext 98 多块头，其中每个整数表示后续对应 bin 块的解压大小
// readExtPayloadAsIntList reads an extension of the expected type and parses its payload as consecutive MessagePack integers
// It is used for the ext 98 multi-block header where each integer gives the decompressed size of the corresponding following bin block
func readExtPayloadAsIntList(data []byte, pos *int, expectedType int8) ([]int, error) {
	extType, payload, err := readExtPayload(data, pos)
	if err != nil {
		return nil, err
	}
	if extType != expectedType {
		return nil, fmt.Errorf("expected ext type %d, got %d", expectedType, extType)
	}
	return decodeMsgpackIntList(payload)
}

// decodeMsgpackIntList 解码载荷中首尾相接的所有 MessagePack 整数
// decodeMsgpackIntList decodes every consecutive MessagePack integer in a payload
func decodeMsgpackIntList(payload []byte) ([]int, error) {
	sizes := make([]int, 0, min(len(payload), 64))
	for pp := 0; pp < len(payload); {
		n, consumed, err := decodeMsgpackInt(payload[pp:])
		if err != nil {
			return nil, fmt.Errorf("decode size[%d] at offset %d: %w", len(sizes), pp, err)
		}
		if consumed <= 0 {
			return nil, fmt.Errorf("decode size[%d] consumed %d bytes", len(sizes), consumed)
		}
		sizes = append(sizes, n)
		pp += consumed
	}
	return sizes, nil
}

// decodeMsgpackInt 从字节序列开头解码一个 MessagePack 整数并返回值及消费字节数
// decodeMsgpackInt decodes one MessagePack integer from the start of a byte sequence and returns its value and consumed byte count
func decodeMsgpackInt(data []byte) (int, int, error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("empty data")
	}
	b := data[0]
	switch {
	case b <= 0x7f:
		return int(b), 1, nil
	case b >= 0xe0:
		return int(int8(b)), 1, nil
	case b == 0xcc:
		if len(data) < 2 {
			return 0, 0, fmt.Errorf("truncated uint8")
		}
		return int(data[1]), 2, nil
	case b == 0xcd:
		if len(data) < 3 {
			return 0, 0, fmt.Errorf("truncated uint16")
		}
		return int(binary.BigEndian.Uint16(data[1:])), 3, nil
	case b == 0xce:
		if len(data) < 5 {
			return 0, 0, fmt.Errorf("truncated uint32")
		}
		return uint64ToInt(uint64(binary.BigEndian.Uint32(data[1:])), 5)
	case b == 0xcf:
		if len(data) < 9 {
			return 0, 0, fmt.Errorf("truncated uint64")
		}
		return uint64ToInt(binary.BigEndian.Uint64(data[1:]), 9)
	case b == 0xd0:
		if len(data) < 2 {
			return 0, 0, fmt.Errorf("truncated int8")
		}
		return int(int8(data[1])), 2, nil
	case b == 0xd1:
		if len(data) < 3 {
			return 0, 0, fmt.Errorf("truncated int16")
		}
		return int(int16(binary.BigEndian.Uint16(data[1:]))), 3, nil
	case b == 0xd2:
		if len(data) < 5 {
			return 0, 0, fmt.Errorf("truncated int32")
		}
		return int(int32(binary.BigEndian.Uint32(data[1:]))), 5, nil
	case b == 0xd3:
		if len(data) < 9 {
			return 0, 0, fmt.Errorf("truncated int64")
		}
		value := int64(binary.BigEndian.Uint64(data[1:]))
		if int64(int(value)) != value {
			return 0, 0, fmt.Errorf("int64 %d overflows int", value)
		}
		return int(value), 9, nil
	}
	return 0, 0, fmt.Errorf("not an int: 0x%02x", b)
}

// uint64ToInt 验证无符号值可由主机 int 表示并保留调用者提供的消费字节数
// uint64ToInt verifies that an unsigned value fits the host int and preserves the caller-supplied consumed byte count
func uint64ToInt(value uint64, consumed int) (int, int, error) {
	maxInt := uint64(^uint(0) >> 1)
	if value > maxInt {
		return 0, 0, fmt.Errorf("uint64 %d overflows int", value)
	}
	return int(value), consumed, nil
}

// CompressLz4BlockArray 将数据按 65536 字节分块并压缩为 MessagePack-CSharp 标准 Lz4BlockArray 布局
// CompressLz4BlockArray splits data into 65536-byte blocks and compresses it into the standard MessagePack-CSharp Lz4BlockArray layout
func CompressLz4BlockArray(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	if err := validateLz4Size(len(data)); err != nil {
		return nil, err
	}

	numBlocks := (len(data) + blockSize - 1) / blockSize
	arrayLen := 1 + numBlocks

	// MessagePack-CSharp Lz4BlockArray 由 array N+1、含每块独立 MessagePack 解压大小的 ext 98 和后续 bin32 块组成
	// 扩展载荷不是单个总大小整数
	// MessagePack-CSharp Lz4BlockArray consists of array N+1, ext 98 carrying one independently encoded MessagePack decompressed size per block, and following bin32 blocks
	// The extension payload is not one total-size integer
	sizePayload := make([]byte, 0, numBlocks*3)
	for offset := 0; offset < len(data); offset += blockSize {
		end := min(offset+blockSize, len(data))
		sizePayload = appendMsgpackUint(sizePayload, uint32(end-offset))
	}

	var out []byte
	out = WriteArrayHeader(out, arrayLen)
	out = WriteExt(out, Lz4ArrayType, sizePayload)

	for offset := 0; offset < len(data); offset += blockSize {
		end := min(offset+blockSize, len(data))
		block := data[offset:end]
		compressed, err := compressLz4Block(block)
		if err != nil {
			return nil, fmt.Errorf("compress Lz4BlockArray block[%d]: %w", offset/blockSize, err)
		}
		out = writeBin32(out, compressed)
	}

	return out, nil
}

// compressLz4Block 压缩单块并将不可压缩结果交给兼容收尾逻辑
// compressLz4Block compresses one block and delegates incompressible output to compatibility finishing logic
func compressLz4Block(block []byte) ([]byte, error) {
	dst := make([]byte, lz4.CompressBlockBound(len(block)))
	n, err := lz4.CompressBlock(block, dst, nil)
	if err != nil {
		return nil, err
	}
	return finishLz4Block(block, dst, n), nil
}

// finishLz4Block 返回压缩结果，或为不可压缩块构造 MessagePack-CSharp 可解码的纯字面量 LZ4 块
// finishLz4Block returns compressed output or constructs a literal-only LZ4 block that MessagePack-CSharp can decode for incompressible input
func finishLz4Block(block, compressed []byte, compressedSize int) []byte {
	if compressedSize > 0 {
		return compressed[:compressedSize]
	}

	// pierrec/lz4 对不可压缩块返回 n 等于零并要求框架调用者标记原始块
	// MessagePack-CSharp 此布局没有原始标记且总会调用 LZ4Codec.Decode，因此必须编码有效纯字面量 LZ4 块而不能逐字节复制
	// pierrec/lz4 returns n equal to zero for an incompressible block and expects its framing caller to mark the block as raw
	// MessagePack-CSharp has no raw marker in this layout and always calls LZ4Codec.Decode, so a valid literal-only LZ4 block must be encoded instead of copying bytes verbatim
	return encodeLz4LiteralBlock(block)
}

// encodeLz4LiteralBlock 将整块编码为没有匹配序列的有效 LZ4 字面量块
// encodeLz4LiteralBlock encodes an entire block as valid LZ4 literals with no match sequence
func encodeLz4LiteralBlock(block []byte) []byte {
	literalLength := len(block)
	extraLengthBytes := 0
	if literalLength >= 15 {
		extraLengthBytes = (literalLength-15)/255 + 1
	}
	out := make([]byte, 0, 1+extraLengthBytes+literalLength)
	out = append(out, byte(min(literalLength, 15)<<4))
	if literalLength >= 15 {
		remaining := literalLength - 15
		for remaining >= 255 {
			out = append(out, 255)
			remaining -= 255
		}
		out = append(out, byte(remaining))
	}
	out = append(out, block...)
	return out
}

// appendMsgpackUint 以可表示 uint32 值的最短 MessagePack 无符号整数形式追加值
// appendMsgpackUint appends a uint32 value using the shortest MessagePack unsigned integer form that can represent it
func appendMsgpackUint(dst []byte, value uint32) []byte {
	switch {
	case value <= 0x7f:
		return append(dst, byte(value))
	case value <= 0xff:
		return append(dst, 0xcc, byte(value))
	case value <= 0xffff:
		return append(dst, 0xcd, byte(value>>8), byte(value))
	default:
		var encoded [5]byte
		encoded[0] = 0xce
		binary.BigEndian.PutUint32(encoded[1:], value)
		return append(dst, encoded[:]...)
	}
}

// writeBin32 追加固定使用 bin32 头的二进制载荷
// writeBin32 appends a binary payload using an explicit bin32 header
func writeBin32(dst, payload []byte) []byte {
	var header [5]byte
	header[0] = 0xc6
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	dst = append(dst, header[:]...)
	return append(dst, payload...)
}

// ReadArrayHeader 容错读取 MessagePack 数组头，失败时不推进位置并返回零
// ReadArrayHeader tolerantly reads a MessagePack array header and returns zero without advancing the position on failure
func ReadArrayHeader(data []byte, pos *int) int {
	if pos == nil {
		return 0
	}
	current := *pos
	n, err := readArrayHeader(data, &current)
	if err != nil {
		return 0
	}
	*pos = current
	return n
}

// readArrayHeader 严格读取 fixarray、array16 或 array32 头并推进位置
// readArrayHeader strictly reads a fixarray, array16, or array32 header and advances the position
func readArrayHeader(data []byte, pos *int) (int, error) {
	if pos == nil || *pos < 0 || *pos >= len(data) {
		return 0, fmt.Errorf("read array header: unexpected EOF")
	}
	b := data[*pos]
	switch {
	case b >= 0x90 && b <= 0x9f:
		*pos += 1
		return int(b & 0x0f), nil
	case b == 0xdc:
		if len(data)-*pos < 3 {
			return 0, fmt.Errorf("read array16 header: unexpected EOF")
		}
		*pos += 1
		n := int(binary.BigEndian.Uint16(data[*pos:]))
		*pos += 2
		return n, nil
	case b == 0xdd:
		if len(data)-*pos < 5 {
			return 0, fmt.Errorf("read array32 header: unexpected EOF")
		}
		*pos += 1
		n64 := uint64(binary.BigEndian.Uint32(data[*pos:]))
		*pos += 4
		n, _, err := uint64ToInt(n64, 0)
		if err != nil {
			return 0, err
		}
		return n, nil
	}
	return 0, fmt.Errorf("expected array, got 0x%02x", b)
}

// ReadExtHeader 读取指定 ext 类型并将载荷解码为整数
// 载荷可为一、二、四或八字节固定宽度大端整数，也可为 MessagePack-CSharp LZ4 格式使用的 MessagePack 整数
// ReadExtHeader reads an extension of the expected type and decodes its payload as an integer
// The payload may be a one-byte, two-byte, four-byte, or eight-byte fixed-width big-endian integer or a MessagePack integer used by MessagePack-CSharp LZ4 framing
func ReadExtHeader(data []byte, pos *int, expectedType int8) (int, error) {
	extType, payload, err := readExtPayload(data, pos)
	if err != nil {
		return 0, err
	}
	if extType != expectedType {
		return 0, fmt.Errorf("expected ext type %d, got %d", expectedType, extType)
	}
	size, err := decodeExtPayloadAsInt(payload)
	if err != nil {
		return 0, fmt.Errorf("decode ext payload: %w", err)
	}
	return size, nil
}

// ReadBin 读取 MessagePack bin8、bin16 或 bin32 载荷并推进位置
// ReadBin reads a MessagePack bin8, bin16, or bin32 payload and advances the position
func ReadBin(data []byte, pos *int) ([]byte, error) {
	if pos == nil || *pos < 0 || *pos >= len(data) {
		return nil, fmt.Errorf("unexpected EOF")
	}
	b := data[*pos]
	var headerSize int
	var size64 uint64
	switch b {
	case 0xc4:
		headerSize = 2
		if len(data)-*pos < headerSize {
			return nil, fmt.Errorf("read bin8 header: unexpected EOF")
		}
		size64 = uint64(data[*pos+1])
	case 0xc5:
		headerSize = 3
		if len(data)-*pos < headerSize {
			return nil, fmt.Errorf("read bin16 header: unexpected EOF")
		}
		size64 = uint64(binary.BigEndian.Uint16(data[*pos+1:]))
	case 0xc6:
		headerSize = 5
		if len(data)-*pos < headerSize {
			return nil, fmt.Errorf("read bin32 header: unexpected EOF")
		}
		size64 = uint64(binary.BigEndian.Uint32(data[*pos+1:]))
	default:
		return nil, fmt.Errorf("expected bin, got 0x%02x", b)
	}
	if size64 > uint64(len(data)-*pos-headerSize) {
		return nil, fmt.Errorf("bin payload size %d exceeds remaining %d bytes", size64, len(data)-*pos-headerSize)
	}
	size := int(size64)
	*pos += headerSize
	out := data[*pos : *pos+size]
	*pos += size
	return out, nil
}

// isArrayMarker 判断字节是否为任一 MessagePack 数组头标记
// isArrayMarker reports whether a byte is any MessagePack array-header marker
func isArrayMarker(b byte) bool {
	return (b >= 0x90 && b <= 0x9f) || b == 0xdc || b == 0xdd
}

// isExtMarker 判断字节是否为任一 MessagePack 扩展头标记
// isExtMarker reports whether a byte is any MessagePack extension-header marker
func isExtMarker(b byte) bool {
	switch b {
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xc7, 0xc8, 0xc9:
		return true
	default:
		return false
	}
}

// readExtPayload 读取 fixext 或 ext8、ext16、ext32 的类型号与载荷并推进位置
// readExtPayload reads the type number and payload of fixext or ext8, ext16, and ext32 and advances the position
func readExtPayload(data []byte, pos *int) (int8, []byte, error) {
	if pos == nil || *pos < 0 || *pos >= len(data) {
		return 0, nil, fmt.Errorf("unexpected EOF")
	}
	start := *pos
	marker := data[start]
	headerSize := 0
	var payloadSize64 uint64
	var extType int8

	switch marker {
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8:
		headerSize = 2
		if len(data)-start < headerSize {
			return 0, nil, fmt.Errorf("read fixext header: unexpected EOF")
		}
		payloadSizes := map[byte]uint64{0xd4: 1, 0xd5: 2, 0xd6: 4, 0xd7: 8, 0xd8: 16}
		payloadSize64 = payloadSizes[marker]
		extType = int8(data[start+1])
	case 0xc7:
		headerSize = 3
		if len(data)-start < headerSize {
			return 0, nil, fmt.Errorf("read ext8 header: unexpected EOF")
		}
		payloadSize64 = uint64(data[start+1])
		extType = int8(data[start+2])
	case 0xc8:
		headerSize = 4
		if len(data)-start < headerSize {
			return 0, nil, fmt.Errorf("read ext16 header: unexpected EOF")
		}
		payloadSize64 = uint64(binary.BigEndian.Uint16(data[start+1:]))
		extType = int8(data[start+3])
	case 0xc9:
		headerSize = 6
		if len(data)-start < headerSize {
			return 0, nil, fmt.Errorf("read ext32 header: unexpected EOF")
		}
		payloadSize64 = uint64(binary.BigEndian.Uint32(data[start+1:]))
		extType = int8(data[start+5])
	default:
		return 0, nil, fmt.Errorf("expected ext, got 0x%02x", marker)
	}

	remaining := len(data) - start - headerSize
	if payloadSize64 > uint64(remaining) {
		return 0, nil, fmt.Errorf("ext payload size %d exceeds remaining %d bytes", payloadSize64, remaining)
	}
	payloadSize := int(payloadSize64)
	payloadStart := start + headerSize
	*pos = payloadStart + payloadSize
	return extType, data[payloadStart:*pos], nil
}

// WriteArrayHeader 以可表示长度的最短 MessagePack 数组头追加元素数量
// WriteArrayHeader appends an element count using the shortest MessagePack array header that can represent it
func WriteArrayHeader(buf []byte, length int) []byte {
	switch {
	case length <= 15:
		return append(buf, byte(0x90|length))
	case length <= 0xffff:
		b := make([]byte, 3)
		b[0] = 0xdc
		binary.BigEndian.PutUint16(b[1:], uint16(length))
		return append(buf, b...)
	default:
		b := make([]byte, 5)
		b[0] = 0xdd
		binary.BigEndian.PutUint32(b[1:], uint32(length))
		return append(buf, b...)
	}
}

// WriteExt 根据载荷大小选择 fixext 或 ext8、ext16、ext32 并追加扩展值
// WriteExt selects fixext or ext8, ext16, and ext32 according to payload size and appends the extension value
func WriteExt(buf []byte, extType int8, payload []byte) []byte {
	size := len(payload)
	switch size {
	case 1:
		buf = append(buf, 0xd4, byte(extType))
	case 2:
		buf = append(buf, 0xd5, byte(extType))
	case 4:
		buf = append(buf, 0xd6, byte(extType))
	case 8:
		buf = append(buf, 0xd7, byte(extType))
	case 16:
		buf = append(buf, 0xd8, byte(extType))
	default:
		if size <= 0xff {
			buf = append(buf, 0xc7, byte(size), byte(extType))
		} else if size <= 0xffff {
			b := make([]byte, 4)
			b[0] = 0xc8
			binary.BigEndian.PutUint16(b[1:], uint16(size))
			b[3] = byte(extType)
			buf = append(buf, b...)
		} else {
			b := make([]byte, 6)
			b[0] = 0xc9
			binary.BigEndian.PutUint32(b[1:], uint32(size))
			b[5] = byte(extType)
			buf = append(buf, b...)
		}
	}
	return append(buf, payload...)
}

// newMsgpackHandle 创建用于安全解码当前 KCES MessagePack 规范及定位原始根值的句柄
// newMsgpackHandle creates a handle for safely decoding the current KCES MessagePack specification and locating raw root values
func newMsgpackHandle() *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{}
	// KCES 使用当前 MessagePack 规范，byte[] 是 Binary 而不是旧式 Raw 或 String 载体
	// WriteExt 也使解码器在读取 interface 值时能够区分 String 与 Binary
	// KCES uses the current MessagePack specification where byte[] is Binary rather than the legacy Raw or String carrier
	// WriteExt also lets the decoder distinguish String from Binary when decoding interface values
	h.WriteExt = true
	// Raw 在解码时安全，使 SplitFirstMsgpackValue 能遍历恰好一个值而不解释映射、扩展载荷、数值标记或未来字段
	// 编码 codec.Raw 仍须在独立编码句柄上显式启用，因为盲目写入原始字节不安全
	// Raw is decode-safe and lets SplitFirstMsgpackValue traverse exactly one value without interpreting maps, extension payloads, numeric markers, or future fields
	// Encoding codec.Raw remains opt-in on a separate encoder handle because blindly writing raw bytes is unsafe
	h.Raw = true
	h.RawToString = false
	// 限制输入控制的初始集合分配与递归深度
	// MaxInitLen 不是线长度上限，较大合法集合会逐步增长，而微小 array32 或 map32 攻击载荷不能在发现 EOF 前请求巨量分配
	// Input-controlled initial collection allocations and recursion depth are bounded
	// MaxInitLen is not a wire-length limit because larger legitimate collections grow incrementally while tiny array32 or map32 attack payloads cannot request huge allocations before EOF is discovered
	h.MaxInitLen = messagePackInitialCollectionCapacity
	h.MaxDepth = 256
	h.ValidateUnicode = true
	return h
}

// newMsgpackEncoderHandle 创建当前规范编码句柄，并只在调用者完成原始槽位校验时启用 Raw
// newMsgpackEncoderHandle creates a current-specification encoder handle and enables Raw only when the caller has validated raw slots
func newMsgpackEncoderHandle(allowRaw bool) *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{}
	h.WriteExt = true
	h.Raw = allowRaw
	h.MaxDepth = 256
	return h
}

// DecodeMsgpackWithConsumed 解码第一个 MessagePack 值并返回该值精确消费的输入字节数
// MessagePack-CSharp Deserialize<T> 在所选格式化器完成后返回且不要求 EOF，因此重写游戏文件的调用者可保留根值之后的数据
// DecodeMsgpackWithConsumed decodes the first MessagePack value and reports exactly how many input bytes it consumed
// MessagePack-CSharp Deserialize<T> returns after the selected formatter finishes and does not require EOF, allowing callers rewriting game files to retain data after the root value
func DecodeMsgpackWithConsumed(data []byte, out interface{}) (int, error) {
	h := newMsgpackHandle()
	dec := codec.NewDecoderBytes(data, h)
	if err := dec.Decode(out); err != nil {
		return dec.NumBytesRead(), err
	}
	return dec.NumBytesRead(), nil
}

// DecodeMsgpack 将第一个 MessagePack 值解码到目标对象，与游戏 Deserialize<T> 一样不要求根后 EOF
// 需要保留根值之后字节的调用者应使用 DecodeMsgpackWithConsumed
// DecodeMsgpack decodes the first MessagePack value into a target and like the game Deserialize<T> does not require EOF after the root
// Callers that need to retain bytes after the root should use DecodeMsgpackWithConsumed
func DecodeMsgpack(data []byte, out interface{}) error {
	_, err := DecodeMsgpackWithConsumed(data, out)
	return err
}

// SplitFirstMsgpackValue 使用编解码库定位一个完整根值，并逐字节返回根值与尾部两侧
// 它有意解码为 codec.Raw，避免仅为寻找根边界就把重复映射键、扩展值、非标准数值标记和未知字段转换成有损 Go 值
// SplitFirstMsgpackValue uses the codec library to locate one complete root value and returns both root and trailing sides byte-for-byte
// It deliberately decodes into codec.Raw so duplicate map keys, extension values, non-canonical numeric markers, and unknown fields are not converted into a lossy Go value merely to find the root boundary
func SplitFirstMsgpackValue(data []byte) (root, trailing []byte, err error) {
	var raw codec.Raw
	consumed, err := DecodeMsgpackWithConsumed(data, &raw)
	if err != nil {
		return nil, nil, err
	}
	if consumed < 0 || consumed > len(data) {
		return nil, nil, fmt.Errorf("MessagePack decoder consumed invalid byte count %d of %d", consumed, len(data))
	}
	return append([]byte(nil), data[:consumed]...), append([]byte(nil), data[consumed:]...), nil
}

// EncodeMsgpack 使用不允许注入 codec.Raw 的当前 KCES 句柄编码普通 MessagePack 对象
// EncodeMsgpack encodes an ordinary MessagePack object with the current KCES handle that does not allow codec.Raw injection
func EncodeMsgpack(v interface{}) ([]byte, error) {
	h := newMsgpackEncoderHandle(false)
	var out []byte
	enc := codec.NewEncoderBytes(&out, h)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out, nil
}

// EncodeIndexedMsgpack 编码通过 codec.Selfer 和 EncodeIndexedObjectSelf 实现的 indexed object 模型
// Raw 模式仅用于辅助函数在确认每个字节切片恰含一个完整 MessagePack 值后注入未来槽位
// 要求 codec.Selfer 可阻止含任意 codec.Raw 的切片或映射进入此校验边界，普通调用者应继续使用禁用 Raw 的 EncodeMsgpack
// EncodeIndexedMsgpack encodes an indexed object model implemented through codec.Selfer and EncodeIndexedObjectSelf
// Raw mode exists only so the helper can inject future slots after verifying that every byte slice contains exactly one complete MessagePack value
// Requiring codec.Selfer prevents arbitrary slices or maps containing codec.Raw from entering this validation boundary, and ordinary callers should keep using EncodeMsgpack with Raw disabled
func EncodeIndexedMsgpack(v codec.Selfer) ([]byte, error) {
	h := newMsgpackEncoderHandle(true)
	var out []byte
	enc := codec.NewEncoderBytes(&out, h)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out, nil
}

// encodeMsgpackAllowRaw 仅供先将每个 codec.Raw 槽位验证为单个完整 MessagePack 值的模型使用
// 与 EncodeMsgpack 分离可防止普通编码器注入任意原始字节
// encodeMsgpackAllowRaw is reserved for models that first validate every codec.Raw slot as one complete MessagePack value
// Keeping it separate from EncodeMsgpack prevents arbitrary raw-byte injection in ordinary encoders
func encodeMsgpackAllowRaw(v interface{}) ([]byte, error) {
	h := newMsgpackEncoderHandle(true)
	var out []byte
	enc := codec.NewEncoderBytes(&out, h)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out, nil
}

// decodeExtPayloadAsInt 将 ext 载荷解码为整数
// 优先接受完整 MessagePack 整数，也接受 fixext 常用的一、二、四或八字节固定宽度大端整数
// decodeExtPayloadAsInt decodes an extension payload as an integer
// It first accepts a complete MessagePack integer and also accepts one-byte, two-byte, four-byte, or eight-byte fixed-width big-endian integers commonly used by fixext
func decodeExtPayloadAsInt(payload []byte) (int, error) {
	if value, consumed, err := decodeMsgpackInt(payload); err == nil && consumed == len(payload) {
		return value, nil
	}
	return decodeFixedWidthInt(payload)
}

// decodeFixedWidthInt 将一、二、四或八字节大端无符号载荷转换为主机 int
// decodeFixedWidthInt converts a one-byte, two-byte, four-byte, or eight-byte big-endian unsigned payload to host int
func decodeFixedWidthInt(payload []byte) (int, error) {
	switch len(payload) {
	case 1:
		return int(payload[0]), nil
	case 2:
		return int(binary.BigEndian.Uint16(payload)), nil
	case 4:
		value, _, err := uint64ToInt(uint64(binary.BigEndian.Uint32(payload)), 0)
		return value, err
	case 8:
		value, _, err := uint64ToInt(binary.BigEndian.Uint64(payload), 0)
		return value, err
	}
	if len(payload) == 0 {
		return 0, fmt.Errorf("empty ext payload")
	}
	return 0, fmt.Errorf("unsupported fixed-width integer payload: %d bytes", len(payload))
}
