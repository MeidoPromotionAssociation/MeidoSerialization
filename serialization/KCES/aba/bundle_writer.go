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

// BundleWriteOptions 控制 AssetBundle 写入行为 / BundleWriteOptions controls AssetBundle write behavior
type BundleWriteOptions struct {
	EngineVersion     string // Unity 引擎版本（如 "2021.3.3f1"），默认 "2021.3.3f1" / Unity engine version such as "2021.3.3f1", default "2021.3.3f1"
	GenerationVersion string // 生成版本（如 "5.x.x"），默认 "5.x.x" / Generation version such as "5.x.x", default "5.x.x"
	Version           uint32 // 文件格式版本，默认 7 / File format version, default 7
	Compress          bool   // 是否使用 LZ4 压缩数据块 / Whether to compress data blocks with LZ4
}

// BundleFileEntry 表示要写入 bundle 的一个文件条目 / BundleFileEntry represents one file entry to write into a bundle
type BundleFileEntry struct {
	Name         string // 文件名（如 "CAB-xxx"）/ File name such as "CAB-xxx"
	Data         []byte // 文件数据 / File data
	IsSerialized bool   // 是否为 AssetsFile（序列化文件）/ Whether this entry is an AssetsFile serialized file
}

// WriteBundle 将文件条目列表写入为 UnityFS 格式的 AssetBundle
//
// 写入格式：
//
//	[Header] UnityFS signature + version + engine version + FSHeader
//	[BlockAndDirInfo] LZ4 压缩的块信息和目录信息
//	[Data Blocks] 文件数据（可选 LZ4 压缩，每块 0x20000 字节）
func WriteBundle(w io.Writer, entries []BundleFileEntry, opts *BundleWriteOptions) error {
	if w == nil {
		return fmt.Errorf("bundle writer is nil")
	}
	if _, err := int32WireLength("entry count", uint64(len(entries))); err != nil {
		return err
	}

	var options BundleWriteOptions
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
	if options.Version < minSupportedBundleVersion || options.Version > maxSupportedBundleVersion {
		return fmt.Errorf("unsupported UnityFS version %d (supported: %d-%d)", options.Version, minSupportedBundleVersion, maxSupportedBundleVersion)
	}
	if err := validateBundleHeaderString("generation version", options.GenerationVersion); err != nil {
		return err
	}
	if err := validateBundleHeaderString("engine version", options.EngineVersion); err != nil {
		return err
	}

	// 1. 构建 DirectoryInfos。只计算逻辑偏移，不复制全部输入数据。
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

	// 2. 第一遍编码数据块，只记录块表和压缩后总大小。数据在最终
	// 写出时重新编码，避免为整个 bundle 保留第二份大切片。
	var blockInfos []BlockInfo
	var compressedDataSize int64
	err := forEachBundleDataBlock(entries, func(index int, block []byte) error {
		if _, err := int32WireLength("data block count", uint64(index)+1); err != nil {
			return fmt.Errorf("data block count exceeds Int32 wire range")
		}
		info, encoded, err := encodeBundleDataBlock(block, options.Compress)
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

	// 3. 序列化 BlockAndDirInfo
	blockAndDirBytes, err := serializeBlockAndDirInfo(blockInfos, dirInfos)
	if err != nil {
		return fmt.Errorf("serialize block and dir info: %w", err)
	}
	if len(blockAndDirBytes) > maxBlockAndDirInfoSize {
		return fmt.Errorf("block/directory info size %d exceeds limit %d", len(blockAndDirBytes), maxBlockAndDirInfoSize)
	}

	// 4. LZ4 压缩 BlockAndDirInfo
	blockAndDirCompressed := make([]byte, lz4.CompressBlockBound(len(blockAndDirBytes)))
	n, err := lz4.CompressBlock(blockAndDirBytes, blockAndDirCompressed, nil)
	if err != nil {
		return fmt.Errorf("compress block/directory info: %w", err)
	}
	infoCompression := byte(CompressionNone)
	if n == 0 || n >= len(blockAndDirBytes) {
		// 压缩无收益
		blockAndDirCompressed = blockAndDirBytes
		n = len(blockAndDirBytes)
	} else {
		blockAndDirCompressed = blockAndDirCompressed[:n]
		infoCompression = CompressionLZ4
	}

	// 5. 计算 header 大小和总文件大小
	headerSize := len(signatureUnityFS) + 1 + // signature + null
		4 + // version
		len(options.GenerationVersion) + 1 + // gen version + null
		len(options.EngineVersion) + 1 + // engine version + null
		20 // FSHeader (8+4+4+4)

	// version >= 7 需要 16 字节对齐
	alignedHeaderSize := headerSize
	if options.Version >= 7 {
		alignedHeaderSize = binaryio.AlignOffset(headerSize, 16)
	}

	totalFileSize, ok := addNonNegativeInt64(int64(alignedHeaderSize), int64(n))
	if !ok {
		return fmt.Errorf("bundle header and metadata size overflow")
	}
	totalFileSize, ok = addNonNegativeInt64(totalFileSize, compressedDataSize)
	if !ok {
		return fmt.Errorf("total bundle file size overflow")
	}

	// 6. 确定 flags
	flags := uint32(FlagHasDirectoryInfo) | uint32(infoCompression)

	// 7. 写入 Header
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.BigEndian)

	// Signature (null-terminated)
	if err := bw.WriteNullString(signatureUnityFS); err != nil {
		return fmt.Errorf("write UnityFS signature: %w", err)
	}

	// Version
	if err := bw.WriteUInt32(options.Version); err != nil {
		return fmt.Errorf("write UnityFS version: %w", err)
	}

	// GenerationVersion (null-terminated)
	if err := bw.WriteNullString(options.GenerationVersion); err != nil {
		return fmt.Errorf("write UnityFS generation version: %w", err)
	}

	// EngineVersion (null-terminated)
	if err := bw.WriteNullString(options.EngineVersion); err != nil {
		return fmt.Errorf("write UnityFS engine version: %w", err)
	}

	// FSHeader
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

	// Align to 16 bytes (version >= 7)
	if options.Version >= 7 {
		if err := bw.WriteZeroes(alignedHeaderSize - bw.Len()); err != nil {
			return fmt.Errorf("write UnityFS header padding: %w", err)
		}
	}

	// 8. 顺序输出 header、BlockAndDirInfo 和数据块。
	if err := writeBundleBytes(w, buf.Bytes()); err != nil {
		return fmt.Errorf("write UnityFS header: %w", err)
	}
	if err := writeBundleBytes(w, blockAndDirCompressed); err != nil {
		return fmt.Errorf("write block and dir info: %w", err)
	}
	blockIndex := 0
	err = forEachBundleDataBlock(entries, func(index int, block []byte) error {
		info, encoded, err := encodeBundleDataBlock(block, options.Compress)
		if err != nil {
			return err
		}
		if index >= len(blockInfos) || info != blockInfos[index] {
			return fmt.Errorf("data block %d changed between encoding passes", index)
		}
		if err := writeBundleBytes(w, encoded); err != nil {
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

func validateBundleHeaderString(field, value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%s contains a NUL byte", field)
	}
	return nil
}

// forEachBundleDataBlock exposes the concatenated entry stream in bounded
// UnityFS blocks. The scratch storage is reused and is valid only until fn
// returns.
func forEachBundleDataBlock(entries []BundleFileEntry, fn func(index int, block []byte) error) error {
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

func encodeBundleDataBlock(block []byte, compress bool) (BlockInfo, []byte, error) {
	if len(block) > maxBundleBlockSize || uint64(len(block)) > uint64(^uint32(0)) {
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

func writeBundleBytes(w io.Writer, data []byte) error {
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

// serializeBlockAndDirInfo 将 BlockInfos 和 DirectoryInfos 序列化为二进制格式
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

	// Hash (16 bytes, all zeros)
	if err := bw.WriteZeroes(16); err != nil {
		return nil, fmt.Errorf("write hash: %w", err)
	}

	// BlockCount + BlockInfos
	if err := bw.WriteInt32(blockCount); err != nil {
		return nil, fmt.Errorf("write block count: %w", err)
	}
	for i, b := range blocks {
		if b.DecompressedSize > maxBundleBlockSize || b.CompressedSize > maxBundleBlockSize {
			return nil, fmt.Errorf("block[%d] exceeds per-block size limit %d", i, maxBundleBlockSize)
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

	// DirectoryCount + DirectoryInfos
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
