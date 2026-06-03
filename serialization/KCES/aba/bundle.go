package aba

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"github.com/pierrec/lz4/v4"
)

// .aba 文件是标准的 Unity AssetBundle UnityFS 格式 / .aba files use the standard Unity AssetBundle UnityFS format
// Unity 5.3+ 使用 UnityFS 签名，包含压缩的资源数据块 / Unity 5.3+ uses the UnityFS signature with compressed resource data blocks
//
// 文件整体结构（所有头部字段使用 Big-Endian）：
//
//	[Header]
//	  - Signature: "UnityFS" (null-terminated string)
//	  - Version: uint32（文件格式版本，通常 6-8）
//	  - GenerationVersion: string（如 "5.x.x", null-terminated）
//	  - EngineVersion: string（如 "2021.3.3f1", null-terminated）
//	  - FSHeader:
//	    - TotalFileSize: int64（整个文件的大小）
//	    - CompressedSize: uint32（BlockAndDirInfo 压缩后大小）
//	    - DecompressedSize: uint32（BlockAndDirInfo 解压后大小）
//	    - Flags: uint32（压缩和布局标志位）
//
//	[BlockAndDirInfo]（位置由 Flags 决定，可能被 LZ4 或 LZMA 压缩）
//	  - Hash: 16 bytes
//	  - BlockCount: int32
//	  - BlockInfos[BlockCount]: DecompressedSize(uint32) + CompressedSize(uint32) + Flags(uint16)
//	  - DirectoryCount: int32
//	  - DirectoryInfos[DirectoryCount]: Offset(int64) + DecompressedSize(int64) + Flags(uint32) + Name(string)
//
//	[Data Blocks]（可能被 LZ4 分块压缩，每块最大 0x20000 bytes）
//	  - 包含一个或多个 AssetsFile（Unity 序列化文件）

const (
	signatureUnityFS = "UnityFS" // Unity 5.3+ AssetBundle 签名 / Unity 5.3+ AssetBundle signature
	signatureAbap    = "abap"    // KCES 加密 AssetBundle 签名（需要 key 文件解密）/ KCES encrypted AssetBundle signature requiring key-file decryption

	// FSHeader Flags 位定义 / FSHeader flag bits
	CompressionNone  = 0x00 // 无压缩 / No compression
	CompressionLZMA  = 0x01 // LZMA 压缩 / LZMA compression
	CompressionLZ4   = 0x02 // LZ4 压缩 / LZ4 compression
	CompressionLZ4HC = 0x03 // LZ4HC 压缩（更高压缩率）/ LZ4HC compression with higher ratio
	CompressionMask  = 0x3f // 压缩类型掩码（低 6 位）/ Compression type mask in the low 6 bits

	FlagHasDirectoryInfo            = 0x40  // 包含目录信息（5.2+ 始终为 true）/ Contains directory info, always true in 5.2+
	FlagBlockAndDirAtEnd            = 0x80  // BlockAndDirInfo 位于文件末尾 / BlockAndDirInfo is stored at file end
	FlagOldWebPluginCompat          = 0x100 // 旧版 Web 插件兼容 / Old web plugin compatibility
	FlagBlockInfoNeedPaddingAtStart = 0x200 // 数据块起始需要 16 字节对齐 / Data blocks require 16-byte alignment at start

	// DirectoryInfo Flags / DirectoryInfo flags
	DirFlagSerializedFile = 0x04 // 该条目是 AssetsFile（序列化文件）/ Entry is an AssetsFile serialized file
)

// Bundle 表示一个 Unity AssetBundle UnityFS 文件 / Bundle represents one Unity AssetBundle UnityFS file
type Bundle struct {
	Header     BundleHeader    // 文件头 / File header
	BlockInfo  BlockAndDirInfo // 压缩块和目录信息 / Compressed block and directory information
	DataReader io.ReadSeeker   // 数据区 reader（已处理 LZ4 分块解压）/ Data-area reader after LZ4 block handling
}

// BundleHeader 表示 UnityFS 文件头，所有字段使用 Big-Endian 编码 / BundleHeader represents the UnityFS header with Big-Endian fields
type BundleHeader struct {
	Signature         string   // 签名（"UnityFS"）/ Signature, usually "UnityFS"
	Version           uint32   // 文件格式版本（通常 6-8）/ File format version, usually 6-8
	GenerationVersion string   // 生成版本（如 "5.x.x"）/ Generation version such as "5.x.x"
	EngineVersion     string   // Unity 引擎版本（如 "2021.3.3f1"）/ Unity engine version such as "2021.3.3f1"
	FSHeader          FSHeader // 文件流头部 / File stream header
}

// FSHeader 表示 UnityFS 的文件流头部信息，紧跟在 BundleHeader 字符串字段之后 / FSHeader represents UnityFS stream header fields after BundleHeader strings
type FSHeader struct {
	TotalFileSize    int64  // 整个文件的总大小 / Total file size
	CompressedSize   uint32 // BlockAndDirInfo 压缩后的字节大小 / Compressed size of BlockAndDirInfo
	DecompressedSize uint32 // BlockAndDirInfo 解压后的字节大小 / Decompressed size of BlockAndDirInfo
	Flags            uint32 // 标志位（压缩类型 + 布局标志）/ Flags combining compression type and layout bits
}

// BlockAndDirInfo 包含压缩块列表和文件目录列表 / BlockAndDirInfo contains data block and directory lists
type BlockAndDirInfo struct {
	Hash           [16]byte        // 16 字节哈希 / 16-byte hash
	BlockInfos     []BlockInfo     // 数据压缩块信息列表 / Data block info list
	DirectoryInfos []DirectoryInfo // 文件目录条目列表 / File directory entry list
}

// BlockInfo 描述一个数据压缩块的元信息 / BlockInfo describes one compressed data block
type BlockInfo struct {
	DecompressedSize uint32 // 解压后大小 / Decompressed size
	CompressedSize   uint32 // 压缩后大小（未压缩时与 DecompressedSize 相同）/ Compressed size, same as DecompressedSize when uncompressed
	Flags            uint16 // 标志位：低 6 位为压缩类型，bit6 表示是否为流式块 / Flags, low 6 bits are compression type and bit6 marks streamed blocks
}

// DirectoryInfo 描述 bundle 内的一个文件条目 / DirectoryInfo describes one file entry inside a bundle
type DirectoryInfo struct {
	Offset           int64  // 相对于数据区起始的偏移量 / Offset relative to the data area start
	DecompressedSize int64  // 解压后大小 / Decompressed size
	Flags            uint32 // 标志位（0x04 = 序列化文件/AssetsFile）/ Flags, 0x04 means serialized AssetsFile
	Name             string // 文件名 / File name
}

// GetCompressionType 返回块的压缩类型
func (b *BlockInfo) GetCompressionType() byte {
	return byte(b.Flags & uint16(CompressionMask))
}

// IsSerialized 返回该条目是否为 AssetsFile
func (d *DirectoryInfo) IsSerialized() bool {
	return d.Flags&DirFlagSerializedFile != 0
}

// GetCompressionType 返回 FSHeader 的压缩类型
func (h *FSHeader) GetCompressionType() byte {
	return byte(h.Flags & uint32(CompressionMask))
}

// ReadBundle 从 reader 中读取并解析 Unity AssetBundle 文件
func ReadBundle(r io.ReadSeeker) (*Bundle, error) {
	bundle := &Bundle{}

	// 1. 读取 Header
	if err := bundle.readHeader(r); err != nil {
		return nil, fmt.Errorf("read bundle header failed: %w", err)
	}

	if bundle.Header.Signature != signatureUnityFS {
		if len(bundle.Header.Signature) >= 4 && bundle.Header.Signature[:4] == signatureAbap {
			return nil, fmt.Errorf("this .aba file is encrypted (unsupported). Please install the original DLC and launch the game once to decrypt it")
		}
		return nil, fmt.Errorf("unsupported signature: %q (only UnityFS supported)", bundle.Header.Signature)
	}

	// 2. 对齐到 16 字节（version >= 7）
	if bundle.Header.Version >= 7 {
		pos, _ := r.Seek(0, io.SeekCurrent)
		aligned := (pos + 15) & ^int64(15)
		if aligned != pos {
			r.Seek(aligned, io.SeekStart)
		}
	}

	// 3. 读取 BlockAndDirInfo
	if err := bundle.readBlockAndDirInfo(r); err != nil {
		return nil, fmt.Errorf("read block and dir info failed: %w", err)
	}

	// 4. 设置数据区 reader
	dataOffset := bundle.getFileDataOffset()
	bundle.DataReader = io.NewSectionReader(r.(io.ReaderAt), dataOffset, bundle.Header.FSHeader.TotalFileSize-dataOffset)

	return bundle, nil
}

// GetFileNames 返回 bundle 中所有文件的名称列表
func (b *Bundle) GetFileNames() []string {
	names := make([]string, len(b.BlockInfo.DirectoryInfos))
	for i, d := range b.BlockInfo.DirectoryInfos {
		names[i] = d.Name
	}
	return names
}

// GetFileData 读取指定索引的文件数据（自动处理 LZ4 分块解压）
func (b *Bundle) GetFileData(index int) ([]byte, error) {
	if index < 0 || index >= len(b.BlockInfo.DirectoryInfos) {
		return nil, fmt.Errorf("file index %d out of range [0, %d)", index, len(b.BlockInfo.DirectoryInfos))
	}

	dir := b.BlockInfo.DirectoryInfos[index]

	data, err := b.readDataRange(dir.Offset, dir.DecompressedSize)
	if err != nil {
		return nil, fmt.Errorf("read file %q data failed: %w", dir.Name, err)
	}

	return data, nil
}

// GetFileDataByName 按名称读取文件数据
func (b *Bundle) GetFileDataByName(name string) ([]byte, error) {
	for i, d := range b.BlockInfo.DirectoryInfos {
		if d.Name == name {
			return b.GetFileData(i)
		}
	}
	return nil, fmt.Errorf("file %q not found in bundle", name)
}

// GetFileDataRangeByName reads a byte range from a bundle file by name.
// The offset and size are relative to the decompressed file entry, not the
// whole UnityFS data stream.
func (b *Bundle) GetFileDataRangeByName(name string, offset int64, size int64) ([]byte, error) {
	for _, d := range b.BlockInfo.DirectoryInfos {
		if d.Name != name {
			continue
		}
		if offset < 0 || size < 0 {
			return nil, fmt.Errorf("invalid file range offset=%d size=%d", offset, size)
		}
		if offset+size < offset || offset+size > d.DecompressedSize {
			return nil, fmt.Errorf("file %q range [%d, %d) out of bounds %d", name, offset, offset+size, d.DecompressedSize)
		}
		return b.readDataRange(d.Offset+offset, size)
	}
	return nil, fmt.Errorf("file %q not found in bundle", name)
}

// readHeader 读取 UnityFS 文件头
func (b *Bundle) readHeader(r io.ReadSeeker) error {
	// 1. Signature (null-terminated string)
	sig, err := binaryio.ReadNullString(r)
	if err != nil {
		return fmt.Errorf("read signature failed: %w", err)
	}
	b.Header.Signature = sig

	// 2. Version (uint32 big-endian)
	if err := binary.Read(r, binary.BigEndian, &b.Header.Version); err != nil {
		return fmt.Errorf("read version failed: %w", err)
	}

	// 3. GenerationVersion (null-terminated string)
	genVer, err := binaryio.ReadNullString(r)
	if err != nil {
		return fmt.Errorf("read generation version failed: %w", err)
	}
	b.Header.GenerationVersion = genVer

	// 4. EngineVersion (null-terminated string)
	engVer, err := binaryio.ReadNullString(r)
	if err != nil {
		return fmt.Errorf("read engine version failed: %w", err)
	}
	b.Header.EngineVersion = engVer

	// 5. FSHeader
	if err := binary.Read(r, binary.BigEndian, &b.Header.FSHeader); err != nil {
		return fmt.Errorf("read fs header failed: %w", err)
	}

	return nil
}

// readBlockAndDirInfo 读取并解压 BlockAndDirInfo
func (b *Bundle) readBlockAndDirInfo(r io.ReadSeeker) error {
	flags := b.Header.FSHeader.Flags

	// 确定 BlockAndDirInfo 的位置
	if flags&uint32(FlagBlockAndDirAtEnd) != 0 {
		// 位于文件末尾
		offset := b.Header.FSHeader.TotalFileSize - int64(b.Header.FSHeader.CompressedSize)
		r.Seek(offset, io.SeekStart)
	}
	// 否则紧跟在 header 之后（当前位置）

	compressedSize := int(b.Header.FSHeader.CompressedSize)
	decompressedSize := int(b.Header.FSHeader.DecompressedSize)

	// 读取压缩数据
	compressedData := make([]byte, compressedSize)
	if _, err := io.ReadFull(r, compressedData); err != nil {
		return fmt.Errorf("read compressed block info failed: %w", err)
	}

	// 解压
	var infoData []byte
	compType := b.Header.FSHeader.GetCompressionType()
	switch compType {
	case CompressionNone:
		infoData = compressedData
	case CompressionLZ4, CompressionLZ4HC:
		infoData = make([]byte, decompressedSize)
		n, err := lz4.UncompressBlock(compressedData, infoData)
		if err != nil {
			return fmt.Errorf("LZ4 decompress block info failed: %w", err)
		}
		infoData = infoData[:n]
	case CompressionLZMA:
		return fmt.Errorf("LZMA compression not yet supported")
	default:
		return fmt.Errorf("unknown compression type: %d", compType)
	}

	// 解析 BlockAndDirInfo
	return b.parseBlockAndDirInfo(infoData)
}

// parseBlockAndDirInfo 从解压后的字节中解析 BlockAndDirInfo
func (b *Bundle) parseBlockAndDirInfo(data []byte) error {
	r := binaryio.NewEndianReader(data, binary.BigEndian)

	// 1. Hash (16 bytes)
	if err := r.ReadFull(b.BlockInfo.Hash[:]); err != nil {
		return fmt.Errorf("read hash: %w", err)
	}

	// 2. BlockCount + BlockInfos
	blockCountRaw, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read block count: %w", err)
	}
	if blockCountRaw < 0 {
		return fmt.Errorf("negative block count %d", blockCountRaw)
	}
	blockCount := int(blockCountRaw)

	b.BlockInfo.BlockInfos = make([]BlockInfo, blockCount)
	for i := 0; i < blockCount; i++ {
		decompressedSize, err := r.ReadUInt32()
		if err != nil {
			return fmt.Errorf("read block info %d decompressed size: %w", i, err)
		}
		compressedSize, err := r.ReadUInt32()
		if err != nil {
			return fmt.Errorf("read block info %d compressed size: %w", i, err)
		}
		flags, err := r.ReadUInt16()
		if err != nil {
			return fmt.Errorf("read block info %d flags: %w", i, err)
		}
		b.BlockInfo.BlockInfos[i] = BlockInfo{
			DecompressedSize: decompressedSize,
			CompressedSize:   compressedSize,
			Flags:            flags,
		}
	}

	// 3. DirectoryCount + DirectoryInfos
	dirCountRaw, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read directory count: %w", err)
	}
	if dirCountRaw < 0 {
		return fmt.Errorf("negative directory count %d", dirCountRaw)
	}
	dirCount := int(dirCountRaw)

	b.BlockInfo.DirectoryInfos = make([]DirectoryInfo, dirCount)
	for i := 0; i < dirCount; i++ {
		offset, err := r.ReadInt64()
		if err != nil {
			return fmt.Errorf("read directory info %d offset: %w", i, err)
		}
		decompSize, err := r.ReadInt64()
		if err != nil {
			return fmt.Errorf("read directory info %d decompressed size: %w", i, err)
		}
		flags, err := r.ReadUInt32()
		if err != nil {
			return fmt.Errorf("read directory info %d flags: %w", i, err)
		}

		// Name (null-terminated)
		name, err := r.ReadNullString()
		if err != nil {
			return fmt.Errorf("read directory info %d name: %w", i, err)
		}

		b.BlockInfo.DirectoryInfos[i] = DirectoryInfo{
			Offset:           offset,
			DecompressedSize: decompSize,
			Flags:            flags,
			Name:             name,
		}
	}

	return nil
}

// readDataRange returns a slice from the decompressed UnityFS data stream.
// It only reads and decompresses blocks that overlap the requested range, so
// extracting many files does not retain or repeatedly allocate a full bundle
// decompression buffer.
func (b *Bundle) readDataRange(offset int64, size int64) ([]byte, error) {
	if offset < 0 || size < 0 {
		return nil, fmt.Errorf("invalid range offset=%d size=%d", offset, size)
	}
	if size == 0 {
		return []byte{}, nil
	}

	var totalSize int64
	for _, block := range b.BlockInfo.BlockInfos {
		totalSize += int64(block.DecompressedSize)
	}
	end := offset + size
	if end < offset || end > totalSize {
		return nil, fmt.Errorf("range [%d, %d) out of decompressed data bounds %d", offset, end, totalSize)
	}

	readerAt, ok := b.DataReader.(io.ReaderAt)
	if !ok {
		return nil, fmt.Errorf("bundle data reader does not support random access")
	}

	result := make([]byte, 0, size)
	var compressedOffset int64
	var decompressedOffset int64
	for blockIndex, block := range b.BlockInfo.BlockInfos {
		blockStart := decompressedOffset
		blockEnd := blockStart + int64(block.DecompressedSize)
		overlaps := offset < blockEnd && end > blockStart

		if overlaps {
			compressed := make([]byte, block.CompressedSize)
			if _, err := readerAt.ReadAt(compressed, compressedOffset); err != nil {
				return nil, fmt.Errorf("read block[%d] data: %w", blockIndex, err)
			}

			blockData, err := decompressDataBlock(block, compressed)
			if err != nil {
				return nil, fmt.Errorf("decompress block[%d]: %w", blockIndex, err)
			}
			if int64(len(blockData)) < int64(block.DecompressedSize) {
				return nil, fmt.Errorf("block[%d] decompressed too short: got %d, want %d", blockIndex, len(blockData), block.DecompressedSize)
			}

			copyStart := maxInt64(offset, blockStart) - blockStart
			copyEnd := minInt64(end, blockEnd) - blockStart
			result = append(result, blockData[copyStart:copyEnd]...)
		}

		compressedOffset += int64(block.CompressedSize)
		decompressedOffset = blockEnd
	}

	if int64(len(result)) != size {
		return nil, fmt.Errorf("range read size mismatch: got %d, want %d", len(result), size)
	}
	return result, nil
}

func decompressDataBlock(block BlockInfo, compressed []byte) ([]byte, error) {
	switch compType := block.GetCompressionType(); compType {
	case CompressionNone:
		return compressed, nil
	case CompressionLZ4, CompressionLZ4HC:
		dst := make([]byte, block.DecompressedSize)
		n, err := lz4.UncompressBlock(compressed, dst)
		if err != nil {
			return nil, err
		}
		return dst[:n], nil
	case CompressionLZMA:
		return nil, fmt.Errorf("LZMA compression not yet supported")
	default:
		return nil, fmt.Errorf("unknown block compression type: %d", compType)
	}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// getFileDataOffset 计算数据区在文件中的起始偏移
func (b *Bundle) getFileDataOffset() int64 {
	flags := b.Header.FSHeader.Flags

	// 基础偏移 = signature + version + genVersion + engVersion + fsHeader
	// signature: len + 1 (null), version: 4, genVersion: len + 1, engVersion: len + 1
	// fsHeader: 8 + 4 + 4 + 4 = 20
	offset := int64(len(b.Header.Signature) + 1 + 4 +
		len(b.Header.GenerationVersion) + 1 +
		len(b.Header.EngineVersion) + 1 + 20)

	if b.Header.Version >= 7 {
		offset = (offset + 15) & ^int64(15) // align to 16
	}

	if flags&uint32(FlagBlockAndDirAtEnd) == 0 {
		// BlockAndDirInfo 在 header 之后
		offset += int64(b.Header.FSHeader.CompressedSize)
	}

	if flags&uint32(FlagBlockInfoNeedPaddingAtStart) != 0 {
		offset = (offset + 15) & ^int64(15) // align to 16
	}

	return offset
}
