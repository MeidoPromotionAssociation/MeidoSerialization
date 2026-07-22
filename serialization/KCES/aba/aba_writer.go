package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"github.com/pierrec/lz4/v4"
)

// AbaWriteOptions 控制 .aba 文件的写入行为
// AbaWriteOptions controls .aba file writing
type AbaWriteOptions struct {
	EngineVersion     string // Unity 引擎版本（如 "2021.3.3f1"），默认 "2021.3.3f1" / Unity engine version such as "2021.3.3f1", default "2021.3.3f1"
	GenerationVersion string // 生成版本（如 "5.x.x"），默认 "5.x.x" / Generation version such as "5.x.x", default "5.x.x"
	Version           uint32 // 文件格式版本，默认 7 / File format version, default 7
	Compress          bool   // 是否使用 LZ4 压缩数据块 / Whether to compress data blocks with LZ4
}

// AbaFileEntry 表示要写入 .aba 文件的一个目录条目
// AbaFileEntry represents one directory entry to write into an .aba file
type AbaFileEntry struct {
	Name         string // 文件名，如 "CAB-xxx" / File name such as "CAB-xxx"
	Data         []byte // 文件数据 / File data
	IsSerialized bool   // 是否为序列化 AssetsFile / Whether this entry is a serialized AssetsFile
}

// WriteAba 将文件条目列表写入采用 Unity AssetBundle UnityFS 格式的 .aba 文件
// 输出依次包含 Header、BlockAndDirInfo 和数据块，数据块按 0x20000 字节分块并可选使用 LZ4 压缩
// WriteAba writes file entries as an .aba file using the Unity AssetBundle UnityFS format
// Output contains Header, BlockAndDirInfo, and data blocks in order; data is split into 0x20000-byte blocks with optional LZ4 compression
func WriteAba(w io.Writer, entries []AbaFileEntry, opts *AbaWriteOptions) error {
	if w == nil {
		return fmt.Errorf(".aba writer is nil")
	}
	if _, err := int32WireLength("entry count", uint64(len(entries))); err != nil {
		return err
	}

	var options AbaWriteOptions
	if opts != nil {
		options = *opts
	}
	if options.EngineVersion == "" {
		options.EngineVersion = "2021.3.3f1"
	}
	if options.GenerationVersion == "" {
		options.GenerationVersion = "5.x.x"
	}
	if options.Version == 0 {
		options.Version = 7
	}
	if options.Version < minSupportedAbaVersion || options.Version > maxSupportedAbaVersion {
		return fmt.Errorf("unsupported UnityFS version %d (supported: %d-%d)", options.Version, minSupportedAbaVersion, maxSupportedAbaVersion)
	}
	if err := validateAbaHeaderString("generation version", options.GenerationVersion); err != nil {
		return err
	}
	if err := validateAbaHeaderString("engine version", options.EngineVersion); err != nil {
		return err
	}

	// 首先构建 DirectoryInfos，只计算逻辑偏移而不复制全部输入数据
	// First build DirectoryInfos, computing logical offsets without copying all input data
	dirInfos := make([]DirectoryInfo, len(entries))
	var totalDataSize int64
	for i, entry := range entries {
		if strings.IndexByte(entry.Name, 0) >= 0 {
			return fmt.Errorf("entry %d name contains a NUL byte", i)
		}
		dirInfos[i] = DirectoryInfo{
			Offset:           totalDataSize,
			DecompressedSize: int64(len(entry.Data)),
			Name:             entry.Name,
		}
		if entry.IsSerialized {
			dirInfos[i].Flags = DirFlagSerializedFile
		}
		var ok bool
		totalDataSize, ok = addNonNegativeInt64(totalDataSize, int64(len(entry.Data)))
		if !ok {
			return fmt.Errorf("entry data size sum overflows at entry %d", i)
		}
	}

	// 第一遍编码数据块时只记录块表和压缩后总大小，最终写出时重新编码，以免为整个 .aba 保留第二份大切片
	// The first data-block encoding pass records only the block table and total compressed size; blocks are encoded again during final output to avoid retaining a second large slice for the entire .aba
	var blockInfos []BlockInfo
	var compressedDataSize int64
	err := forEachAbaDataBlock(entries, func(index int, block []byte) error {
		if _, err := int32WireLength("data block count", uint64(index)+1); err != nil {
			return fmt.Errorf("data block count exceeds Int32 wire range")
		}
		info, encoded, err := encodeAbaDataBlock(block, options.Compress)
		if err != nil {
			return fmt.Errorf("encode data block %d: %w", index, err)
		}
		var ok bool
		compressedDataSize, ok = addNonNegativeInt64(compressedDataSize, int64(len(encoded)))
		if !ok {
			return fmt.Errorf("compressed data size sum overflows at block %d", index)
		}
		blockInfos = append(blockInfos, info)
		return nil
	})
	if err != nil {
		return err
	}

	// 将块表与目录表序列化为 BlockAndDirInfo
	// Serialize the block and directory tables as BlockAndDirInfo
	blockAndDirBytes, err := serializeBlockAndDirInfo(blockInfos, dirInfos)
	if err != nil {
		return fmt.Errorf("serialize block and dir info: %w", err)
	}
	if len(blockAndDirBytes) > maxBlockAndDirInfoSize {
		return fmt.Errorf("block/directory info size %d exceeds limit %d", len(blockAndDirBytes), maxBlockAndDirInfoSize)
	}

	// 尝试使用 LZ4 压缩 BlockAndDirInfo
	// Attempt to compress BlockAndDirInfo with LZ4
	blockAndDirCompressed := make([]byte, lz4.CompressBlockBound(len(blockAndDirBytes)))
	n, err := lz4.CompressBlock(blockAndDirBytes, blockAndDirCompressed, nil)
	if err != nil {
		return fmt.Errorf("compress block/directory info: %w", err)
	}
	infoCompression := byte(CompressionNone)
	if n == 0 || n >= len(blockAndDirBytes) {
		// 压缩没有减小数据时保留原始元数据
		// Keep raw metadata when compression does not reduce its size
		blockAndDirCompressed = blockAndDirBytes
		n = len(blockAndDirBytes)
	} else {
		blockAndDirCompressed = blockAndDirCompressed[:n]
		infoCompression = CompressionLZ4
	}

	// 头部大小包括三个 NUL 结尾字符串、UInt32 Version 和固定 20 字节 FSHeader
	// Header size includes three NUL-terminated strings, UInt32 Version, and the fixed 20-byte FSHeader
	headerSize := len(signatureUnityFS) + 1 +
		4 +
		len(options.GenerationVersion) + 1 +
		len(options.EngineVersion) + 1 +
		20

	// UnityFS version 7 及以上将头部对齐到 16 字节
	// UnityFS version 7 and later align the header to 16 bytes
	alignedHeaderSize := headerSize
	if options.Version >= 7 {
		alignedHeaderSize = binaryio.AlignOffset(headerSize, 16)
	}

	totalFileSize, ok := addNonNegativeInt64(int64(alignedHeaderSize), int64(n))
	if !ok {
		return fmt.Errorf(".aba header and metadata size overflow")
	}
	totalFileSize, ok = addNonNegativeInt64(totalFileSize, compressedDataSize)
	if !ok {
		return fmt.Errorf("total .aba file size overflow")
	}

	// 当前写入布局将 BlockAndDirInfo 放在数据块之前并设置组合目录标志
	// The emitted layout places BlockAndDirInfo before data blocks and sets the combined-directory flag
	flags := uint32(FlagHasDirectoryInfo) | uint32(infoCompression)

	// 在内存缓冲区中构造 Big-Endian Header
	// Build the Big-Endian Header in a memory buffer
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.BigEndian)

	// Signature 以 NUL 结尾
	// Signature is NUL-terminated
	if err := bw.WriteNullString(signatureUnityFS); err != nil {
		return fmt.Errorf("write UnityFS signature: %w", err)
	}

	// Version 使用 Big-Endian UInt32
	// Version uses Big-Endian UInt32
	if err := bw.WriteUInt32(options.Version); err != nil {
		return fmt.Errorf("write UnityFS version: %w", err)
	}

	// GenerationVersion 以 NUL 结尾
	// GenerationVersion is NUL-terminated
	if err := bw.WriteNullString(options.GenerationVersion); err != nil {
		return fmt.Errorf("write UnityFS generation version: %w", err)
	}

	// EngineVersion 以 NUL 结尾
	// EngineVersion is NUL-terminated
	if err := bw.WriteNullString(options.EngineVersion); err != nil {
		return fmt.Errorf("write UnityFS engine version: %w", err)
	}

	// FSHeader 保存总大小、元数据大小和 flags
	// FSHeader stores total size, metadata sizes, and flags
	if err := bw.WriteInt64(totalFileSize); err != nil {
		return fmt.Errorf("write UnityFS total file size: %w", err)
	}
	compressedInfoSize, err := uint32WireLength("compressed block/directory info size", uint64(n))
	if err != nil {
		return err
	}
	decompressedInfoSize, err := uint32WireLength("decompressed block/directory info size", uint64(len(blockAndDirBytes)))
	if err != nil {
		return err
	}
	if err := bw.WriteUInt32(compressedInfoSize); err != nil {
		return fmt.Errorf("write UnityFS block info compressed size: %w", err)
	}
	if err := bw.WriteUInt32(decompressedInfoSize); err != nil {
		return fmt.Errorf("write UnityFS block info decompressed size: %w", err)
	}
	if err := bw.WriteUInt32(flags); err != nil {
		return fmt.Errorf("write UnityFS flags: %w", err)
	}

	// UnityFS version 7 及以上写入零字节补齐 16 字节边界
	// UnityFS version 7 and later write zero padding to the 16-byte boundary
	if options.Version >= 7 {
		if err := bw.WriteZeroes(alignedHeaderSize - bw.Len()); err != nil {
			return fmt.Errorf("write UnityFS header padding: %w", err)
		}
	}

	// 最后顺序输出 Header、BlockAndDirInfo 和第二遍编码的数据块
	// Finally output Header, BlockAndDirInfo, and the second-pass encoded data blocks in order
	if err := writeAbaBytes(w, buf.Bytes()); err != nil {
		return fmt.Errorf("write UnityFS header: %w", err)
	}
	if err := writeAbaBytes(w, blockAndDirCompressed); err != nil {
		return fmt.Errorf("write block and dir info: %w", err)
	}
	blockIndex := 0
	err = forEachAbaDataBlock(entries, func(index int, block []byte) error {
		info, encoded, err := encodeAbaDataBlock(block, options.Compress)
		if err != nil {
			return err
		}
		if index >= len(blockInfos) || info != blockInfos[index] {
			return fmt.Errorf("data block %d changed between encoding passes", index)
		}
		if err := writeAbaBytes(w, encoded); err != nil {
			return err
		}
		blockIndex++
		return nil
	})
	if err != nil {
		return fmt.Errorf("write data blocks: %w", err)
	}
	if blockIndex != len(blockInfos) {
		return fmt.Errorf("wrote %d data blocks, expected %d", blockIndex, len(blockInfos))
	}
	return nil
}

// validateAbaHeaderString 拒绝无法由 NUL 结尾头部字符串表示的内嵌 NUL
// validateAbaHeaderString rejects embedded NUL bytes that cannot be represented by NUL-terminated header strings
func validateAbaHeaderString(field, value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%s contains a NUL byte", field)
	}
	return nil
}

// forEachAbaDataBlock 将拼接后的条目流按有界 UnityFS 块传给回调
// 暂存区会复用，每个 block 切片只在对应 fn 调用返回前有效
// forEachAbaDataBlock passes the concatenated entry stream to a callback in bounded UnityFS blocks
// Scratch storage is reused, and each block slice remains valid only until its fn call returns
func forEachAbaDataBlock(entries []AbaFileEntry, fn func(index int, block []byte) error) error {
	if fn == nil {
		return fmt.Errorf("data block callback is nil")
	}
	const blockSize = 0x20000
	scratch := make([]byte, blockSize)
	entryIndex := 0
	entryOffset := 0
	blockIndex := 0
	producedData := false

	for {
		filled := 0
		for filled < len(scratch) && entryIndex < len(entries) {
			entry := entries[entryIndex].Data
			if entryOffset >= len(entry) {
				entryIndex++
				entryOffset = 0
				continue
			}
			n := copy(scratch[filled:], entry[entryOffset:])
			filled += n
			entryOffset += n
		}
		if filled == 0 {
			if !producedData {
				if err := fn(0, scratch[:0]); err != nil {
					return err
				}
			}
			return nil
		}
		producedData = true
		if err := fn(blockIndex, scratch[:filled]); err != nil {
			return err
		}
		blockIndex++
	}
}

// encodeAbaDataBlock 按选项尝试 LZ4 压缩一个数据块，并在无收益时保留原始字节
// encodeAbaDataBlock attempts LZ4 compression for one data block and retains raw bytes when compression has no benefit
func encodeAbaDataBlock(block []byte, compress bool) (BlockInfo, []byte, error) {
	if len(block) > maxAbaBlockSize || uint64(len(block)) > uint64(^uint32(0)) {
		return BlockInfo{}, nil, fmt.Errorf("data block is too large: %d bytes", len(block))
	}
	rawInfo := BlockInfo{
		DecompressedSize: uint32(len(block)),
		CompressedSize:   uint32(len(block)),
		Flags:            0x40,
	}
	if !compress || len(block) == 0 {
		return rawInfo, block, nil
	}

	dst := make([]byte, lz4.CompressBlockBound(len(block)))
	n, err := lz4.CompressBlock(block, dst, nil)
	if err != nil {
		return BlockInfo{}, nil, err
	}
	if n == 0 || n >= len(block) {
		return rawInfo, block, nil
	}
	return BlockInfo{
		DecompressedSize: uint32(len(block)),
		CompressedSize:   uint32(n),
		Flags:            0x40 | uint16(CompressionLZ4),
	}, dst[:n], nil
}

// writeAbaBytes 持续写入直到全部字节完成，并拒绝无进展或无效写入计数
// writeAbaBytes keeps writing until all bytes are complete and rejects no-progress or invalid write counts
func writeAbaBytes(w io.Writer, data []byte) error {
	for len(data) != 0 {
		n, err := w.Write(data)
		if n < 0 || n > len(data) {
			return fmt.Errorf("writer returned invalid byte count %d for %d-byte buffer", n, len(data))
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

// serializeBlockAndDirInfo 将 BlockInfos 和 DirectoryInfos 序列化为 UnityFS Big-Endian 二进制格式
// serializeBlockAndDirInfo serializes BlockInfos and DirectoryInfos in UnityFS Big-Endian binary form
func serializeBlockAndDirInfo(blocks []BlockInfo, dirs []DirectoryInfo) ([]byte, error) {
	blockCount, err := int32WireLength("block count", uint64(len(blocks)))
	if err != nil {
		return nil, err
	}
	directoryCount, err := int32WireLength("directory count", uint64(len(dirs)))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.BigEndian)

	// 当前写入器将 16 字节 Hash 写为全零
	// The current writer emits the 16-byte Hash as all zeros
	if err := bw.WriteZeroes(16); err != nil {
		return nil, fmt.Errorf("write hash: %w", err)
	}

	// Hash 后写入 BlockCount 和固定宽度 BlockInfos
	// Write BlockCount and fixed-width BlockInfos after Hash
	if err := bw.WriteInt32(blockCount); err != nil {
		return nil, fmt.Errorf("write block count: %w", err)
	}
	for i, b := range blocks {
		if b.DecompressedSize > maxAbaBlockSize || b.CompressedSize > maxAbaBlockSize {
			return nil, fmt.Errorf("block[%d] exceeds per-block size limit %d", i, maxAbaBlockSize)
		}
		if err := bw.WriteUInt32(b.DecompressedSize); err != nil {
			return nil, fmt.Errorf("write block[%d] decompressed size: %w", i, err)
		}
		if err := bw.WriteUInt32(b.CompressedSize); err != nil {
			return nil, fmt.Errorf("write block[%d] compressed size: %w", i, err)
		}
		if err := bw.WriteUInt16(b.Flags); err != nil {
			return nil, fmt.Errorf("write block[%d] flags: %w", i, err)
		}
	}

	// 块表后写入 DirectoryCount 和含 NUL 结尾名称的 DirectoryInfos
	// Write DirectoryCount and DirectoryInfos with NUL-terminated names after the block table
	if err := bw.WriteInt32(directoryCount); err != nil {
		return nil, fmt.Errorf("write directory count: %w", err)
	}
	for i, d := range dirs {
		if d.Offset < 0 || d.DecompressedSize < 0 {
			return nil, fmt.Errorf("directory[%d] has invalid range offset=%d size=%d", i, d.Offset, d.DecompressedSize)
		}
		if strings.IndexByte(d.Name, 0) >= 0 {
			return nil, fmt.Errorf("directory[%d] has an invalid name", i)
		}
		if err := bw.WriteInt64(d.Offset); err != nil {
			return nil, fmt.Errorf("write directory[%d] offset: %w", i, err)
		}
		if err := bw.WriteInt64(d.DecompressedSize); err != nil {
			return nil, fmt.Errorf("write directory[%d] decompressed size: %w", i, err)
		}
		if err := bw.WriteUInt32(d.Flags); err != nil {
			return nil, fmt.Errorf("write directory[%d] flags: %w", i, err)
		}
		if err := bw.WriteNullString(d.Name); err != nil {
			return nil, fmt.Errorf("write directory[%d] name: %w", i, err)
		}
	}

	return buf.Bytes(), nil
}
