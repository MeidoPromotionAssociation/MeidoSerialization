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

// AbaWriteOptions 控制 ABA 文件的写入行为 / AbaWriteOptions controls ABA file writing
type AbaWriteOptions struct {
	EngineVersion     string // Unity 引擎版本，默认固定为 2022.3.35f1 / Unity engine version, fixed to 2022.3.35f1 by default
	GenerationVersion string // 生成版本（如 "5.x.x"），默认 "5.x.x" / Generation version such as "5.x.x", default "5.x.x"
	Version           uint32 // 文件格式版本，默认 8 / File format version, default 8
	Compress          bool   // 是否使用 LZ4 压缩数据块 / Whether to compress data blocks with LZ4
}

// AbaEntryWriteFunc 将一个 ABA 目录条目的完整逻辑字节流写入目标
// AbaEntryWriteFunc writes the complete logical byte stream of one ABA directory entry to a destination
type AbaEntryWriteFunc func(io.Writer) error

// AbaFileEntry 表示要写入 ABA 文件的一个目录条目 / AbaFileEntry represents one directory entry to write into an ABA file
type AbaFileEntry struct {
	Name         string            // 文件名，如 "CAB-xxx" / File name such as "CAB-xxx"
	Data         []byte            // 文件数据 / File data
	ReaderAt     io.ReaderAt       // 大型文件的范围读取源，与 Data 互斥 / Range-readable source for a large file, mutually exclusive with Data
	WriteTo      AbaEntryWriteFunc // 可重复调用的流式生成器，与 Data 和 ReaderAt 互斥 / Repeatable streaming generator, mutually exclusive with Data and ReaderAt
	Size         int64             // ReaderAt 或 WriteTo 提供的精确文件大小，使用 Data 时可为零 / Exact size supplied by ReaderAt or WriteTo, or zero when Data is used
	IsSerialized bool              // 是否为序列化 AssetsFile / Whether this entry is a serialized AssetsFile
}

// WriteAba 将文件条目列表写入采用 Unity AssetBundle UnityFS 格式的 .aba 文件
// 输出依次包含 Header、BlockAndDirInfo、16 字节对齐填充和数据块，数据块按 0x20000 字节分块并可选使用 LZ4 压缩
// WriteAba writes file entries as an .aba file using the Unity AssetBundle UnityFS format
// Output contains Header, BlockAndDirInfo, 16-byte alignment padding, and data blocks in order; data is split into 0x20000-byte blocks with optional LZ4 compression
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
		options.EngineVersion = "2022.3.35f1"
	}
	if options.GenerationVersion == "" {
		options.GenerationVersion = "5.x.x"
	}
	if options.Version == 0 {
		options.Version = 8
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
		entrySize, err := abaFileEntrySize(entry)
		if err != nil {
			return fmt.Errorf("entry %d %q: %w", i, entry.Name, err)
		}
		dirInfos[i] = DirectoryInfo{
			Offset:           totalDataSize,
			DecompressedSize: entrySize,
			Name:             entry.Name,
		}
		if entry.IsSerialized {
			dirInfos[i].Flags = DirFlagSerializedFile
		}
		var ok bool
		totalDataSize, ok = addNonNegativeInt64(totalDataSize, entrySize)
		if !ok {
			return fmt.Errorf("entry data size sum overflows at entry %d", i)
		}
	}

	var blockInfos []BlockInfo
	var compressedDataSize int64
	var err error
	if options.Compress {
		// 第一遍编码压缩数据块时只记录块表和压缩后总大小，最终写出时重新编码，以免为整个 ABA 保留第二份大切片
		// The first compressed data-block pass records only the block table and total compressed size, and final output re-encodes the blocks to avoid retaining a second full ABA-sized slice
		err = forEachAbaDataBlock(entries, func(index int64, block []byte) error {
			if _, err := int32WireLength("data block count", uint64(index)+1); err != nil {
				return fmt.Errorf("data block count exceeds Int32 wire range")
			}
			info, encoded, err := encodeAbaDataBlock(block, true)
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
	} else {
		blockInfos, err = uncompressedAbaBlockInfos(totalDataSize)
		if err != nil {
			return err
		}
		compressedDataSize = totalDataSize
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
	headerSize := int64(len(signatureUnityFS)) + 1 +
		4 +
		int64(len(options.GenerationVersion)) + 1 +
		int64(len(options.EngineVersion)) + 1 +
		20

	// UnityFS version 7 及以上将头部对齐到 16 字节
	// UnityFS version 7 and later align the header to 16 bytes
	alignedHeaderSize := headerSize
	if options.Version >= 7 {
		alignedHeaderSize = binaryio.AlignOffset(headerSize, 16)
	}

	totalFileSize, ok := addNonNegativeInt64(alignedHeaderSize, int64(n))
	if !ok {
		return fmt.Errorf(".aba header and metadata size overflow")
	}
	// 官方 KCES 文件设置 FlagBlockInfoNeedPaddingAtStart 并把数据块起点对齐到 16 字节，这里保持同样布局
	// Official KCES files set FlagBlockInfoNeedPaddingAtStart and align the data-block start to 16 bytes, and this writer keeps the same layout
	dataStart, ok := alignInt64(totalFileSize, 16)
	if !ok {
		return fmt.Errorf(".aba data-block start offset overflow")
	}
	dataPadding := dataStart - totalFileSize
	totalFileSize, ok = addNonNegativeInt64(dataStart, compressedDataSize)
	if !ok {
		return fmt.Errorf("total .aba file size overflow")
	}

	// 当前写入布局将 BlockAndDirInfo 放在数据块之前，并设置组合目录与数据块起始对齐标志
	// The emitted layout places BlockAndDirInfo before data blocks and sets the combined-directory and data-block alignment flags
	flags := uint32(FlagHasDirectoryInfo) | uint32(FlagBlockInfoNeedPaddingAtStart) | uint32(infoCompression)

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

	// 最后顺序输出 Header、BlockAndDirInfo、数据块起始对齐填充和第二遍编码的数据块
	// Finally output Header, BlockAndDirInfo, data-block start alignment padding, and the second-pass encoded data blocks in order
	if err := writeAbaBytes(w, buf.Bytes()); err != nil {
		return fmt.Errorf("write UnityFS header: %w", err)
	}
	if err := writeAbaBytes(w, blockAndDirCompressed); err != nil {
		return fmt.Errorf("write block and dir info: %w", err)
	}
	if dataPadding > 0 {
		var paddingBytes [16]byte
		if err := writeAbaBytes(w, paddingBytes[:dataPadding]); err != nil {
			return fmt.Errorf("write data-block start padding: %w", err)
		}
	}
	var blockIndex int64
	err = forEachAbaDataBlock(entries, func(index int64, block []byte) error {
		info, encoded, err := encodeAbaDataBlock(block, options.Compress)
		if err != nil {
			return err
		}
		if index >= int64(len(blockInfos)) || info != blockInfos[index] {
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
	if blockIndex != int64(len(blockInfos)) {
		return fmt.Errorf("wrote %d data blocks, expected %d", blockIndex, len(blockInfos))
	}
	return nil
}

// uncompressedAbaBlockInfos 根据逻辑数据总大小构建无需预读正文的未压缩 UnityFS 块表
// uncompressedAbaBlockInfos builds an uncompressed UnityFS block table from the total logical data size without pre-reading payloads
func uncompressedAbaBlockInfos(totalDataSize int64) ([]BlockInfo, error) {
	if totalDataSize < 0 {
		return nil, fmt.Errorf("negative ABA data size %d", totalDataSize)
	}
	const blockSize int64 = 0x20000
	blockCount := totalDataSize / blockSize
	if totalDataSize%blockSize != 0 {
		blockCount++
	}
	if blockCount == 0 {
		blockCount = 1
	}
	if _, err := int32WireLength("data block count", uint64(blockCount)); err != nil {
		return nil, fmt.Errorf("data block count exceeds Int32 wire range")
	}

	blocks := make([]BlockInfo, 0, blockCount)
	remaining := totalDataSize
	for blockIndex := int64(0); blockIndex < blockCount; blockIndex++ {
		dataSize := remaining
		if dataSize > blockSize {
			dataSize = blockSize
		}
		blocks = append(blocks, BlockInfo{
			DecompressedSize: uint32(dataSize),
			CompressedSize:   uint32(dataSize),
			Flags:            uint16(CompressionNone),
		})
		remaining -= dataSize
	}
	return blocks, nil
}

// validateAbaHeaderString 拒绝无法由 NUL 结尾头部字符串表示的内嵌 NUL
// validateAbaHeaderString rejects embedded NUL bytes that cannot be represented by NUL-terminated header strings
func validateAbaHeaderString(field, value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%s contains a NUL byte", field)
	}
	return nil
}

// abaFileEntrySize 返回内存或 ReaderAt 条目的精确逻辑大小并验证数据源组合
// abaFileEntrySize returns the exact logical size of an in-memory or ReaderAt entry and validates its source combination
func abaFileEntrySize(entry AbaFileEntry) (int64, error) {
	if entry.ReaderAt != nil || entry.WriteTo != nil {
		if entry.Data != nil {
			return 0, fmt.Errorf("Data cannot be combined with ReaderAt or WriteTo")
		}
		if entry.ReaderAt != nil && entry.WriteTo != nil {
			return 0, fmt.Errorf("ReaderAt and WriteTo cannot both be set")
		}
		if entry.Size < 0 {
			return 0, fmt.Errorf("streamed size %d is negative", entry.Size)
		}
		return entry.Size, nil
	}
	dataSize := int64(len(entry.Data))
	if entry.Size != 0 && entry.Size != dataSize {
		return 0, fmt.Errorf("declared size %d does not match %d Data bytes", entry.Size, dataSize)
	}
	return dataSize, nil
}

// forEachAbaDataBlock 将拼接后的条目流按有界 UnityFS 块传给回调
// 暂存区会复用，每个 block 切片只在对应 fn 调用返回前有效
// forEachAbaDataBlock passes the concatenated entry stream to a callback in bounded UnityFS blocks
// Scratch storage is reused, and each block slice remains valid only until its fn call returns
func forEachAbaDataBlock(entries []AbaFileEntry, fn func(index int64, block []byte) error) error {
	if fn == nil {
		return fmt.Errorf("data block callback is nil")
	}
	const blockSize = 0x20000
	blocks := &abaDataBlockWriter{callback: fn, scratch: make([]byte, blockSize)}
	for entryIndex := range entries {
		entry := entries[entryIndex]
		size, err := abaFileEntrySize(entry)
		if err != nil {
			return fmt.Errorf("entry %d %q: %w", entryIndex, entry.Name, err)
		}
		exact := &abaExactWriter{writer: blocks, remaining: size}
		switch {
		case entry.WriteTo != nil:
			if err := entry.WriteTo(exact); err != nil {
				return fmt.Errorf("generate entry %d %q: %w", entryIndex, entry.Name, err)
			}
		case entry.ReaderAt != nil:
			if err := writeAbaReaderAtEntry(exact, entry.ReaderAt, size); err != nil {
				return fmt.Errorf("read entry %d %q: %w", entryIndex, entry.Name, err)
			}
		default:
			if err := writeAbaBytes(exact, entry.Data); err != nil {
				return fmt.Errorf("write entry %d %q Data: %w", entryIndex, entry.Name, err)
			}
		}
		if exact.remaining != 0 {
			return fmt.Errorf("entry %d %q wrote %d fewer bytes than declared", entryIndex, entry.Name, exact.remaining)
		}
	}
	return blocks.finish()
}

// writeAbaReaderAtEntry 顺序读取 ReaderAt 并将精确大小转发到 UnityFS 块写入器
// writeAbaReaderAtEntry reads a ReaderAt sequentially and forwards its exact size to the UnityFS block writer
func writeAbaReaderAtEntry(out io.Writer, source io.ReaderAt, size int64) error {
	if out == nil || source == nil {
		return fmt.Errorf("nil ReaderAt source or destination")
	}
	const chunkSize int64 = 0x20000
	buffer := make([]byte, chunkSize)
	for offset := int64(0); offset < size; {
		readSize := size - offset
		if readSize > int64(len(buffer)) {
			readSize = int64(len(buffer))
		}
		n, err := source.ReadAt(buffer[:readSize], offset)
		if n < 0 || int64(n) > readSize {
			return fmt.Errorf("ReaderAt returned invalid byte count %d for %d-byte range", n, readSize)
		}
		if err != nil && !(err == io.EOF && int64(n) == readSize) {
			return err
		}
		if int64(n) != readSize {
			return io.ErrUnexpectedEOF
		}
		if err := writeAbaBytes(out, buffer[:readSize]); err != nil {
			return err
		}
		offset += readSize
	}
	return nil
}

// abaDataBlockWriter 将连续条目流切分为固定大小的 UnityFS 数据块 / abaDataBlockWriter splits a continuous entry stream into fixed-size UnityFS data blocks
type abaDataBlockWriter struct {
	callback   func(index int64, block []byte) error // 数据块回调 / Data-block callback
	scratch    []byte                                // 当前数据块缓冲区 / Current data-block buffer
	filled     int64                                 // 当前块已填充字节数 / Bytes filled in the current block
	blockIndex int64                                 // 下一个数据块索引 / Next data-block index
	produced   bool                                  // 是否已产生非空块 / Whether a non-empty block was produced
}

// Write 将条目字节追加到固定大小数据块并在块满时调用回调
// Write appends entry bytes to fixed-size data blocks and invokes the callback whenever a block fills
func (w *abaDataBlockWriter) Write(data []byte) (int, error) {
	if w == nil || w.callback == nil || len(w.scratch) == 0 {
		return 0, fmt.Errorf("invalid ABA data-block writer")
	}
	written := int64(0)
	for written < int64(len(data)) {
		space := int64(len(w.scratch)) - w.filled
		copySize := int64(len(data)) - written
		if copySize > space {
			copySize = space
		}
		copy(w.scratch[w.filled:w.filled+copySize], data[written:written+copySize])
		w.filled += copySize
		written += copySize
		if w.filled == int64(len(w.scratch)) {
			if err := w.flush(); err != nil {
				return int(written), err
			}
		}
	}
	return int(written), nil
}

// flush 提交当前非空数据块并重置缓冲区
// flush submits the current non-empty block and resets the buffer
func (w *abaDataBlockWriter) flush() error {
	if w.filled == 0 {
		return nil
	}
	if err := w.callback(w.blockIndex, w.scratch[:w.filled]); err != nil {
		return err
	}
	w.produced = true
	w.blockIndex++
	w.filled = 0
	return nil
}

// finish 提交最后一个数据块，并为完全空的条目流产生一个空块
// finish submits the final data block and emits one empty block for a completely empty entry stream
func (w *abaDataBlockWriter) finish() error {
	if err := w.flush(); err != nil {
		return err
	}
	if !w.produced {
		return w.callback(0, w.scratch[:0])
	}
	return nil
}

// abaExactWriter 限制单个 ABA 条目生成器写入的剩余字节数 / abaExactWriter limits the remaining bytes written by one ABA entry generator
type abaExactWriter struct {
	writer    io.Writer // 下游块写入器 / Downstream block writer
	remaining int64     // 条目尚须写入的字节数 / Entry bytes still required
}

// Write 转发条目字节并拒绝超过声明大小的写入
// Write forwards entry bytes and rejects writes beyond the declared size
func (w *abaExactWriter) Write(data []byte) (int, error) {
	if w == nil || w.writer == nil {
		return 0, fmt.Errorf("nil ABA exact-size writer")
	}
	if int64(len(data)) > w.remaining {
		return 0, fmt.Errorf("entry attempted to write %d bytes with only %d remaining", len(data), w.remaining)
	}
	n, err := w.writer.Write(data)
	if n < 0 || n > len(data) {
		return 0, fmt.Errorf("writer returned invalid byte count %d for %d-byte buffer", n, len(data))
	}
	w.remaining -= int64(n)
	if err == nil && n == 0 && len(data) != 0 {
		return 0, io.ErrShortWrite
	}
	return n, err
}

// encodeAbaDataBlock 按选项尝试 LZ4 压缩一个数据块，并在无收益时保留原始字节
// 块 Flags 只保存压缩类型，官方 KCES 文件的数据块不设置额外标志位
// encodeAbaDataBlock attempts LZ4 compression for one data block and retains raw bytes when compression has no benefit
// Block Flags store only the compression type because official KCES data blocks set no extra flag bits
func encodeAbaDataBlock(block []byte, compress bool) (BlockInfo, []byte, error) {
	if len(block) > maxAbaBlockSize || uint64(len(block)) > uint64(^uint32(0)) {
		return BlockInfo{}, nil, fmt.Errorf("data block is too large: %d bytes", len(block))
	}
	rawInfo := BlockInfo{
		DecompressedSize: uint32(len(block)),
		CompressedSize:   uint32(len(block)),
		Flags:            uint16(CompressionNone),
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
		Flags:            uint16(CompressionLZ4),
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
