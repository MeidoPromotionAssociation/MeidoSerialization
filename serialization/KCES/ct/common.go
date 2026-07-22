package ct

import (
	"encoding/binary"
	"fmt"

	"github.com/pierrec/lz4/v4"
	"github.com/ugorji/go/codec"
)

const (
	// MessagePack-CSharp reserves extension type 98 for the Lz4BlockArray
	// header and type 99 for a single Lz4Block. See the game's
	// MessagePackSerializer.ToLZ4BinaryCore/TryDecompress implementation.
	Lz4ArrayType = 98
	Lz4BlockType = 99

	blockSize = 65536

	// Older versions of this package emitted a private, reversed layout:
	// array + ext(99, total size) + ext(98, block) ... . Keep accepting it so
	// files produced by those versions remain readable, but never emit it.
	legacyLz4ArrayType = 99
	legacyLz4BlockType = 98

	// Size fields are attacker controlled. A limit prevents a tiny malformed
	// input from causing an unbounded allocation before LZ4 validates it.
	maxLz4DecompressedSize = 1 << 30

	messagePackInitialCollectionCapacity = 1024
)

// MessagePackRootMetadata preserves bytes that are outside a typed root value.
// RootNil is used only when a nil root has trailing bytes and therefore cannot
// be represented by a nil Go pointer alone.
type MessagePackRootMetadata struct {
	RootNil      bool   `json:"rootNil,omitempty"`
	TrailingData []byte `json:"trailingData,omitempty"`
}

// IndexedObjectMetadata preserves the exact width and unknown trailing keys of
// a MessagePack-CSharp int-key object. A nil FieldCount means the current known
// width unless FutureSlots extends it.
type IndexedObjectMetadata struct {
	FieldCount  *int     `json:"fieldCount,omitempty"`
	FutureSlots [][]byte `json:"futureSlots,omitempty"`
}

// TypedIndexedObjectMetadata extends the width/future-slot metadata used by
// hand-written catalog codecs with null annotations needed by ordinary typed
// Go models. Go scalar/value fields and slices of value elements cannot by
// themselves distinguish MessagePack nil from their zero value. These compact
// annotations let the shared indexed-object adapter restore those nil markers
// without duplicating every known field's raw bytes in editing JSON.
type TypedIndexedObjectMetadata struct {
	FieldCount       *int             `json:"fieldCount,omitempty"`
	FutureSlots      [][]byte         `json:"futureSlots,omitempty"`
	NilSlots         []int            `json:"nilSlots,omitempty"`
	NullElements     map[int][]bool   `json:"nullElements,omitempty"`
	NullMapValueKeys map[int][][]byte `json:"nullMapValueKeys,omitempty"`
}

// DecompressLz4BlockArray 处理 MessagePack-CSharp 的 Lz4Block / Lz4BlockArray 格式。
// 支持三种 wire 格式：
//
//  1. Lz4BlockArray（标准多块）：
//     array(N+1) [ext(98, packed MessagePack size1..sizeN), bin32(block1), ..., bin32(blockN)]
//
//  2. Lz4Block（单块）：
//     ext(99, MessagePack uncompressedSize + compressedData)
//
//  3. 旧版 Go 私有布局（仅为向后兼容而读取，不再写出）：
//     array(N+1) [ext(99, fixed-width totalSize), ext(98, block1), ..., ext(98, blockN)]
func DecompressLz4BlockArray(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// MessagePackCompression.Lz4Block is a top-level ext(99) whose payload is
	// one MessagePack integer (the uncompressed size) followed by LZ4 bytes.
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
		// Ordinary MessagePack arrays are not compressed and must pass through.
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

func validateLz4Size(size int) error {
	if size < 0 {
		return fmt.Errorf("negative LZ4 decompressed size %d", size)
	}
	if size > maxLz4DecompressedSize {
		return fmt.Errorf("LZ4 decompressed size %d exceeds limit %d", size, maxLz4DecompressedSize)
	}
	return nil
}

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

// readExtPayloadAsIntList 读取一个 ext 头并将其 payload 解析为多个 MessagePack 整数列表。
// 用于 multi-block Lz4Block 格式：ext(98) 的 payload 包含 N 个 MessagePack 整数，
// 分别表示后续 N 个 bin 块各自的 uncompressed size。
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

// decodeMsgpackInt 从字节序列开头解码一个 MessagePack 整数，
// 返回值与消费的字节数。
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

func uint64ToInt(value uint64, consumed int) (int, int, error) {
	maxInt := uint64(^uint(0) >> 1)
	if value > maxInt {
		return 0, 0, fmt.Errorf("uint64 %d overflows int", value)
	}
	return int(value), consumed, nil
}

// CompressLz4BlockArray 将数据压缩为 MessagePack-CSharp 的 Lz4BlockArray 格式
func CompressLz4BlockArray(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	if err := validateLz4Size(len(data)); err != nil {
		return nil, err
	}

	numBlocks := (len(data) + blockSize - 1) / blockSize
	arrayLen := 1 + numBlocks

	// MessagePack-CSharp Lz4BlockArray wire layout:
	//   array(N+1), ext(98, msgpack(uncompressedSize[0..N])), bin32(block[0])...
	// The extension payload contains one independently encoded MessagePack
	// integer per block, not a single total-size integer.
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

func compressLz4Block(block []byte) ([]byte, error) {
	dst := make([]byte, lz4.CompressBlockBound(len(block)))
	n, err := lz4.CompressBlock(block, dst, nil)
	if err != nil {
		return nil, err
	}
	return finishLz4Block(block, dst, n), nil
}

func finishLz4Block(block, compressed []byte, compressedSize int) []byte {
	if compressedSize > 0 {
		return compressed[:compressedSize]
	}

	// pierrec/lz4 returns n == 0 for an incompressible block and expects its
	// framing caller to mark the block as raw. MessagePack-CSharp has no raw
	// marker here and always invokes LZ4Codec.Decode, so encode a valid
	// literal-only LZ4 block instead of copying the bytes verbatim.
	return encodeLz4LiteralBlock(block)
}

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

func writeBin32(dst, payload []byte) []byte {
	var header [5]byte
	header[0] = 0xc6
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	dst = append(dst, header[:]...)
	return append(dst, payload...)
}

// ReadArrayHeader 读取 msgpack array header
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

// ReadExtHeader 读取 ext 类型并返回其 int payload。
// payload 可能是固定字节数的大端整数（1/2/4/8 字节），
// 也可能是 MessagePack 编码的整数（MessagePack-CSharp 的 Lz4 压缩格式使用此方式）。
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

// ReadBin 读取 msgpack bin 类型的字节数据
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

func isArrayMarker(b byte) bool {
	return (b >= 0x90 && b <= 0x9f) || b == 0xdc || b == 0xdd
}

func isExtMarker(b byte) bool {
	switch b {
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xc7, 0xc8, 0xc9:
		return true
	default:
		return false
	}
}

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

// WriteArrayHeader 写入 msgpack array header
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

// WriteExt 写入 msgpack ext 类型
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

func newMsgpackHandle() *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{}
	// KCES uses the current MessagePack specification. In particular, byte[]
	// is Binary rather than the legacy Raw/String carrier. WriteExt also lets
	// the decoder distinguish String from Binary when decoding interface values.
	h.WriteExt = true
	// Raw is decode-safe and lets SplitFirstMsgpackValue ask the library to
	// traverse exactly one value without interpreting maps, extension payloads,
	// numeric markers, or future fields. Encoding codec.Raw remains opt-in on
	// the separate encoder handle because blindly writing raw bytes is unsafe.
	h.Raw = true
	h.RawToString = false
	// Bound attacker-controlled initial collection allocations and recursion.
	// MaxInitLen is not a wire-length limit: larger legitimate collections grow
	// incrementally, while a tiny array32/map32 bomb cannot request a multi-GiB
	// allocation before the decoder discovers EOF.
	h.MaxInitLen = messagePackInitialCollectionCapacity
	h.MaxDepth = 256
	h.ValidateUnicode = true
	return h
}

func newMsgpackEncoderHandle(allowRaw bool) *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{}
	h.WriteExt = true
	h.Raw = allowRaw
	h.MaxDepth = 256
	return h
}

// DecodeMsgpackWithConsumed decodes the first MessagePack value and reports
// exactly how many input bytes that value consumed. MessagePack-CSharp's
// Deserialize<T> API returns after the selected formatter finishes and does
// not require EOF, so callers that rewrite a game file can retain data after
// the root value instead of silently discarding it.
func DecodeMsgpackWithConsumed(data []byte, out interface{}) (int, error) {
	h := newMsgpackHandle()
	dec := codec.NewDecoderBytes(data, h)
	if err := dec.Decode(out); err != nil {
		return dec.NumBytesRead(), err
	}
	return dec.NumBytesRead(), nil
}

// DecodeMsgpack 解码 msgpack 数据到目标对象。与游戏的 Deserialize<T> 一样，
// 只读取第一个值；需要保留根值之后字节的调用方应使用
// DecodeMsgpackWithConsumed。
func DecodeMsgpack(data []byte, out interface{}) error {
	_, err := DecodeMsgpackWithConsumed(data, out)
	return err
}

// SplitFirstMsgpackValue uses the codec library to locate one complete root
// value while returning both sides byte-for-byte. It deliberately decodes into
// codec.Raw so duplicate map keys, extension values, non-canonical numeric
// markers, and unknown fields are not converted into a lossy Go value merely
// to discover the root boundary.
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

// EncodeMsgpack 将对象编码为 msgpack 数据
func EncodeMsgpack(v interface{}) ([]byte, error) {
	h := newMsgpackEncoderHandle(false)
	var out []byte
	enc := codec.NewEncoderBytes(&out, h)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out, nil
}

// EncodeIndexedMsgpack encodes a model whose codec.Selfer implementation uses
// EncodeIndexedObjectSelf. Raw mode is enabled solely so that the helper can
// inject future slots after validating that each byte slice is exactly one
// complete MessagePack value. Requiring codec.Selfer prevents arbitrary slices
// or maps containing codec.Raw from entering this validation boundary.
// Ordinary callers should continue to use EncodeMsgpack, where codec.Raw is
// deliberately not active.
func EncodeIndexedMsgpack(v codec.Selfer) ([]byte, error) {
	h := newMsgpackEncoderHandle(true)
	var out []byte
	enc := codec.NewEncoderBytes(&out, h)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out, nil
}

// encodeMsgpackAllowRaw is reserved for models that first validate every
// codec.Raw slot as one complete MessagePack value. Keeping it separate from
// EncodeMsgpack prevents arbitrary raw-byte injection in ordinary encoders.
func encodeMsgpackAllowRaw(v interface{}) ([]byte, error) {
	h := newMsgpackEncoderHandle(true)
	var out []byte
	enc := codec.NewEncoderBytes(&out, h)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out, nil
}

// decodeExtPayloadAsInt 将 ext payload 字节解码为整数。
// 支持两种格式：
//  1. 固定字节数大端整数（1/2/4/8 字节，fixext 格式常用）
//  2. MessagePack 编码的整数（ext8/ext16 格式，MessagePack-CSharp Lz4 压缩使用）
func decodeExtPayloadAsInt(payload []byte) (int, error) {
	if value, consumed, err := decodeMsgpackInt(payload); err == nil && consumed == len(payload) {
		return value, nil
	}
	return decodeFixedWidthInt(payload)
}

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
