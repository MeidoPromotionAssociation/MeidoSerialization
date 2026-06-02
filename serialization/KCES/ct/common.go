package ct

import (
	"encoding/binary"
	"fmt"

	"github.com/pierrec/lz4/v4"
	"github.com/ugorji/go/codec"
)

const (
	Lz4BlockType = 98 // MessagePack ext type: LZ4 压缩块
	Lz4ArrayType = 99 // MessagePack ext type: Lz4BlockArray 头（包含解压后总大小）
	blockSize    = 65536
)

// DecompressLz4BlockArray 处理 MessagePack-CSharp 的 Lz4Block / Lz4BlockArray 格式。
// 支持三种变体：
//
//  1. Lz4BlockArray（标准多块）：
//     array(N+1) [ext(99, totalUncompressedSize), ext(98, block1), ..., ext(98, blockN)]
//
//  2. Lz4Block（单块）：
//     array(2) [ext(98, uncompressedSize), bin(compressedData)]
//
//  3. multi-block Lz4Block（KCES catalog 使用的变体）：
//     array(N+1) [ext(98, [size1,size2,...] as packed msgpack ints), bin(block1), ..., bin(blockN)]
func DecompressLz4BlockArray(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	b := data[0]
	isArray := (b >= 0x90 && b <= 0x9f) || b == 0xdc || b == 0xdd
	if !isArray {
		return data, nil
	}

	pos := 0
	arrayLen := ReadArrayHeader(data, &pos)
	if arrayLen < 2 {
		return data, nil
	}

	if pos >= len(data) {
		return data, nil
	}

	savedPos := pos
	if uncompressedSize, err := ReadExtHeader(data, &pos, Lz4ArrayType); err == nil {
		result := make([]byte, 0, uncompressedSize)
		for i := 1; i < arrayLen && pos < len(data); i++ {
			block, err := readAndDecompressBlock(data, &pos)
			if err != nil {
				return nil, fmt.Errorf("decompress block[%d]: %w", i, err)
			}
			result = append(result, block...)
		}
		return result, nil
	}

	pos = savedPos
	if arrayLen == 2 {
		uncompressedSize, err := ReadExtHeader(data, &pos, Lz4BlockType)
		if err == nil {
			compressed, err := ReadBin(data, &pos)
			if err != nil {
				return nil, fmt.Errorf("read Lz4Block bin payload: %w", err)
			}
			dst := make([]byte, uncompressedSize)
			n, err := lz4.UncompressBlock(compressed, dst)
			if err != nil {
				return nil, fmt.Errorf("Lz4Block decompress: %w", err)
			}
			return dst[:n], nil
		}
	}

	// 尝试 multi-block Lz4Block 格式：ext(98) 头 + N 个 bin 块
	pos = savedPos
	sizes, err := readExtPayloadAsIntList(data, &pos, Lz4BlockType)
	if err != nil {
		return data, nil
	}
	expectedBlocks := arrayLen - 1
	if len(sizes) != expectedBlocks {
		return nil, fmt.Errorf("multi-block Lz4Block: payload sizes=%d but arrayLen-1=%d", len(sizes), expectedBlocks)
	}

	totalSize := 0
	for _, s := range sizes {
		totalSize += s
	}
	result := make([]byte, 0, totalSize)
	for i, uncompressedSize := range sizes {
		compressed, err := ReadBin(data, &pos)
		if err != nil {
			return nil, fmt.Errorf("read multi-block bin[%d]: %w", i, err)
		}
		dst := make([]byte, uncompressedSize)
		n, err := lz4.UncompressBlock(compressed, dst)
		if err != nil {
			return nil, fmt.Errorf("multi-block decompress[%d]: %w", i, err)
		}
		result = append(result, dst[:n]...)
	}
	return result, nil
}

// readExtPayloadAsIntList 读取一个 ext 头并将其 payload 解析为多个 MessagePack 整数列表。
// 用于 multi-block Lz4Block 格式：ext(98) 的 payload 包含 N 个 MessagePack 整数，
// 分别表示后续 N 个 bin 块各自的 uncompressed size。
func readExtPayloadAsIntList(data []byte, pos *int, expectedType int8) ([]int, error) {
	if *pos >= len(data) {
		return nil, fmt.Errorf("unexpected EOF")
	}

	b := data[*pos]
	var extSize int
	var extType int8

	switch b {
	case 0xd4:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 1
	case 0xd5:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 2
	case 0xd6:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 4
	case 0xd7:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 8
	case 0xd8:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 16
	case 0xc7:
		*pos++
		extSize = int(data[*pos])
		*pos++
		extType = int8(data[*pos])
		*pos++
	case 0xc8:
		*pos++
		extSize = int(binary.BigEndian.Uint16(data[*pos:]))
		*pos += 2
		extType = int8(data[*pos])
		*pos++
	default:
		return nil, fmt.Errorf("expected ext, got 0x%02x", b)
	}

	if extType != expectedType {
		return nil, fmt.Errorf("expected ext type %d, got %d", expectedType, extType)
	}

	payload := data[*pos : *pos+extSize]
	*pos += extSize

	var sizes []int
	pp := 0
	for pp < len(payload) {
		n, consumed, err := decodeMsgpackInt(payload[pp:])
		if err != nil {
			return nil, fmt.Errorf("decode size[%d] at offset %d: %w", len(sizes), pp, err)
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
		return int(binary.BigEndian.Uint32(data[1:])), 5, nil
	case b == 0xcf:
		if len(data) < 9 {
			return 0, 0, fmt.Errorf("truncated uint64")
		}
		return int(binary.BigEndian.Uint64(data[1:])), 9, nil
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
		return int(int64(binary.BigEndian.Uint64(data[1:]))), 9, nil
	}
	return 0, 0, fmt.Errorf("not an int: 0x%02x", b)
}

// CompressLz4BlockArray 将数据压缩为 MessagePack-CSharp 的 Lz4BlockArray 格式
func CompressLz4BlockArray(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	numBlocks := (len(data) + blockSize - 1) / blockSize
	arrayLen := 1 + numBlocks

	var out []byte
	out = WriteArrayHeader(out, arrayLen)

	sizePayload := make([]byte, 4)
	binary.BigEndian.PutUint32(sizePayload, uint32(len(data)))
	out = WriteExt(out, Lz4ArrayType, sizePayload)

	for offset := 0; offset < len(data); offset += blockSize {
		end := offset + blockSize
		if end > len(data) {
			end = len(data)
		}
		block := data[offset:end]

		dst := make([]byte, lz4.CompressBlockBound(len(block)))
		n, err := lz4.CompressBlock(block, dst, nil)
		if err != nil || n == 0 || n >= len(block) {
			out = WriteExt(out, Lz4BlockType, block)
		} else {
			out = WriteExt(out, Lz4BlockType, dst[:n])
		}
	}

	return out, nil
}

// ReadArrayHeader 读取 msgpack array header
func ReadArrayHeader(data []byte, pos *int) int {
	b := data[*pos]
	switch {
	case b >= 0x90 && b <= 0x9f:
		*pos++
		return int(b & 0x0f)
	case b == 0xdc:
		*pos++
		n := int(binary.BigEndian.Uint16(data[*pos:]))
		*pos += 2
		return n
	case b == 0xdd:
		*pos++
		n := int(binary.BigEndian.Uint32(data[*pos:]))
		*pos += 4
		return n
	}
	return 0
}

// ReadExtHeader 读取 ext 类型并返回其 int payload。
// payload 可能是固定字节数的大端整数（1/2/4/8 字节），
// 也可能是 MessagePack 编码的整数（MessagePack-CSharp 的 Lz4 压缩格式使用此方式）。
func ReadExtHeader(data []byte, pos *int, expectedType int8) (int, error) {
	if *pos >= len(data) {
		return 0, fmt.Errorf("unexpected EOF")
	}

	b := data[*pos]
	var extSize int
	var extType int8

	switch b {
	case 0xd4:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 1
	case 0xd5:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 2
	case 0xd6:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 4
	case 0xd7:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 8
	case 0xd8:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 16
	case 0xc7:
		*pos++
		extSize = int(data[*pos])
		*pos++
		extType = int8(data[*pos])
		*pos++
	case 0xc8:
		*pos++
		extSize = int(binary.BigEndian.Uint16(data[*pos:]))
		*pos += 2
		extType = int8(data[*pos])
		*pos++
	default:
		return 0, fmt.Errorf("expected ext, got 0x%02x", b)
	}

	if extType != expectedType {
		return 0, fmt.Errorf("expected ext type %d, got %d", expectedType, extType)
	}

	// payload 解码为整数。
	// fixext 格式（1/2/4/8 字节）直接按大端解释；
	// ext8/ext16 格式的 payload 可能是 MessagePack 编码的整数（MessagePack-CSharp 使用此方式）。
	size, err := decodeExtPayloadAsInt(data[*pos : *pos+extSize])
	*pos += extSize
	if err != nil {
		return 0, fmt.Errorf("decode ext payload: %w", err)
	}
	return size, nil
}

// ReadBin 读取 msgpack bin 类型的字节数据
func ReadBin(data []byte, pos *int) ([]byte, error) {
	if *pos >= len(data) {
		return nil, fmt.Errorf("unexpected EOF")
	}
	b := data[*pos]
	var size int
	switch b {
	case 0xc4:
		*pos++
		size = int(data[*pos])
		*pos++
	case 0xc5:
		*pos++
		size = int(binary.BigEndian.Uint16(data[*pos:]))
		*pos += 2
	case 0xc6:
		*pos++
		size = int(binary.BigEndian.Uint32(data[*pos:]))
		*pos += 4
	default:
		return nil, fmt.Errorf("expected bin, got 0x%02x", b)
	}
	out := data[*pos : *pos+size]
	*pos += size
	return out, nil
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

// DecodeMsgpack 解码 msgpack 数据到目标对象
func DecodeMsgpack(data []byte, out interface{}) error {
	h := &codec.MsgpackHandle{}
	h.RawToString = true
	dec := codec.NewDecoderBytes(data, h)
	return dec.Decode(out)
}

// EncodeMsgpack 将对象编码为 msgpack 数据
func EncodeMsgpack(v interface{}) ([]byte, error) {
	h := &codec.MsgpackHandle{}
	var out []byte
	enc := codec.NewEncoderBytes(&out, h)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out, nil
}

func readAndDecompressBlock(data []byte, pos *int) ([]byte, error) {
	if *pos >= len(data) {
		return nil, fmt.Errorf("unexpected EOF")
	}

	b := data[*pos]
	var extSize int
	var extType int8

	switch b {
	case 0xd4:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 1
	case 0xd5:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 2
	case 0xd6:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 4
	case 0xd7:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 8
	case 0xd8:
		*pos++
		extType = int8(data[*pos])
		*pos++
		extSize = 16
	case 0xc7:
		*pos++
		extSize = int(data[*pos])
		*pos++
		extType = int8(data[*pos])
		*pos++
	case 0xc8:
		*pos++
		extSize = int(binary.BigEndian.Uint16(data[*pos:]))
		*pos += 2
		extType = int8(data[*pos])
		*pos++
	case 0xc9:
		*pos++
		extSize = int(binary.BigEndian.Uint32(data[*pos:]))
		*pos += 4
		extType = int8(data[*pos])
		*pos++
	default:
		return nil, fmt.Errorf("expected ext, got 0x%02x at pos %d", b, *pos)
	}

	if extType != Lz4BlockType {
		*pos += extSize
		return nil, fmt.Errorf("expected ext type %d, got %d", Lz4BlockType, extType)
	}

	compressed := data[*pos : *pos+extSize]
	*pos += extSize

	dst := make([]byte, extSize*255)
	n, err := lz4.UncompressBlock(compressed, dst)
	if err != nil {
		return compressed, nil
	}

	return dst[:n], nil
}

// decodeExtPayloadAsInt 将 ext payload 字节解码为整数。
// 支持两种格式：
//  1. 固定字节数大端整数（1/2/4/8 字节，fixext 格式常用）
//  2. MessagePack 编码的整数（ext8/ext16 格式，MessagePack-CSharp Lz4 压缩使用）
func decodeExtPayloadAsInt(payload []byte) (int, error) {
	switch len(payload) {
	case 1:
		return int(payload[0]), nil
	case 2:
		return int(binary.BigEndian.Uint16(payload)), nil
	case 4:
		return int(binary.BigEndian.Uint32(payload)), nil
	case 8:
		return int(binary.BigEndian.Uint64(payload)), nil
	}

	// 尝试作为 MessagePack 编码的整数解码
	if len(payload) == 0 {
		return 0, fmt.Errorf("empty ext payload")
	}
	b := payload[0]
	switch {
	case b <= 0x7f:
		return int(b), nil
	case b == 0xcc && len(payload) >= 2:
		return int(payload[1]), nil
	case b == 0xcd && len(payload) >= 3:
		return int(binary.BigEndian.Uint16(payload[1:])), nil
	case b == 0xce && len(payload) >= 5:
		return int(binary.BigEndian.Uint32(payload[1:])), nil
	case b == 0xcf && len(payload) >= 9:
		return int(binary.BigEndian.Uint64(payload[1:])), nil
	}
	return 0, fmt.Errorf("unsupported ext payload: %d bytes, first=0x%02x", len(payload), b)
}
