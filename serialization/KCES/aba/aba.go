package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"github.com/pierrec/lz4/v4"
	"github.com/ulikunitz/xz/lzma"
)

// .aba
// KCES 资源包使用标准 Unity AssetBundle UnityFS 格式。Unity 5.3 及以上版本使用 UnityFS 签名，
// 文件包含可压缩的块目录、资源数据块以及一个或多个 Unity SerializedFile。
// 文件整体结构如下，所有头部字段均使用 Big-Endian：
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
//
// .aba
// KCES .aba files use the standard Unity AssetBundle UnityFS format. Unity 5.3 and later use the UnityFS signature;
// a file contains an optionally compressed block directory, resource-data blocks, and one or more Unity SerializedFiles.
// The overall layout is shown below, with all header fields encoded as Big-Endian:
//
//	[Header]
//	  - Signature: "UnityFS" (null-terminated string)
//	  - Version: uint32 (usually file-format version 6 through 8)
//	  - GenerationVersion: null-terminated string such as "5.x.x"
//	  - EngineVersion: null-terminated string such as "2021.3.3f1"
//	  - FSHeader: TotalFileSize(int64), CompressedSize(uint32), DecompressedSize(uint32), Flags(uint32)
//
//	[BlockAndDirInfo] (location selected by Flags; optionally compressed with LZ4 or LZMA)
//	  - Hash: 16 bytes
//	  - BlockInfos: decompressed size, compressed size, and flags for each block
//	  - DirectoryInfos: offset, decompressed size, flags, and name for each entry
//
//	[Data Blocks] (optionally split into LZ4 blocks of at most 0x20000 bytes)
//	  - Contains one or more AssetsFiles (Unity serialized files)

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

	minSupportedAbaVersion = 6
	maxSupportedAbaVersion = 8

	// UnityFS block/directory metadata is small compared with the data stream.
	// The largest KCES sample is about 400 KiB while its decompressed .resS
	// entry exceeds 4 GiB. Keep metadata and per-block allocations bounded
	// without imposing a low limit on the logical data stream.
	maxBlockAndDirInfoSize = 64 << 20
	maxAbaBlockSize        = 256 << 20
	// parts_bv002.aba contains a real KCES SerializedFile directory of
	// 799,993,424 bytes. Keep a bounded single-allocation API while allowing
	// every observed SerializedFile to reach ReadAssetsFile; multi-gigabyte
	// .resS entries must still be consumed through smaller range reads.
	maxAbaReadSize = 1 << 30
)

// Aba 表示一个采用 Unity AssetBundle UnityFS 格式的 .aba 文件。
//
// Aba represents one .aba file using the Unity AssetBundle UnityFS format.
type Aba struct {
	Header     AbaHeader       // 文件头 / File header
	BlockInfo  BlockAndDirInfo // 压缩块和目录信息 / Compressed block and directory information
	DataReader io.ReadSeeker   // 压缩数据区 reader；各块按需解压 / Compressed data-area reader; blocks are decompressed on demand

	abaStart           int64
	abaSize            int64
	headerEnd          int64
	blockAndDirOffset  int64
	fileDataOffset     int64
	compressedDataSize int64
}

// AbaHeader 表示 UnityFS 文件头，所有字段使用 Big-Endian 编码。
//
// AbaHeader represents the UnityFS header with Big-Endian fields.
type AbaHeader struct {
	Signature         string   // 签名（"UnityFS"）/ Signature, usually "UnityFS"
	Version           uint32   // 文件格式版本（通常 6-8）/ File format version, usually 6-8
	GenerationVersion string   // 生成版本（如 "5.x.x"）/ Generation version such as "5.x.x"
	EngineVersion     string   // Unity 引擎版本（如 "2021.3.3f1"）/ Unity engine version such as "2021.3.3f1"
	FSHeader          FSHeader // 文件流头部 / File stream header
}

// FSHeader 表示 UnityFS 的文件流头部信息，紧跟在 AbaHeader 字符串字段之后。
//
// FSHeader represents UnityFS stream header fields after the AbaHeader strings.
type FSHeader struct {
	TotalFileSize    int64  // 整个文件的总大小 / Total file size
	CompressedSize   uint32 // BlockAndDirInfo 压缩后的字节大小 / Compressed size of BlockAndDirInfo
	DecompressedSize uint32 // BlockAndDirInfo 解压后的字节大小 / Decompressed size of BlockAndDirInfo
	Flags            uint32 // 标志位（压缩类型 + 布局标志）/ Flags combining compression type and layout bits
}

// BlockAndDirInfo 包含压缩块列表和文件目录列表。
//
// BlockAndDirInfo contains the data-block and directory lists.
type BlockAndDirInfo struct {
	Hash           [16]byte        // 16 字节哈希 / 16-byte hash
	BlockInfos     []BlockInfo     // 数据压缩块信息列表 / Data block info list
	DirectoryInfos []DirectoryInfo // 文件目录条目列表 / File directory entry list
	TrailingData   []byte          // 已知目录表之后的未解析字节 / Unparsed bytes following the known directory table
}

// BlockInfo 描述一个数据压缩块的元信息。
//
// BlockInfo describes one compressed data block.
type BlockInfo struct {
	DecompressedSize uint32 // 解压后大小 / Decompressed size
	CompressedSize   uint32 // 压缩后大小（未压缩时与 DecompressedSize 相同）/ Compressed size, same as DecompressedSize when uncompressed
	Flags            uint16 // 标志位：低 6 位为压缩类型，bit6 表示是否为流式块 / Flags, low 6 bits are compression type and bit6 marks streamed blocks
}

// DirectoryInfo 描述 .aba 文件内的一个目录条目。
//
// DirectoryInfo describes one directory entry inside an .aba file.
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

// ReadAba 从 reader 中读取并解析采用 Unity AssetBundle UnityFS 格式的 .aba 文件。
//
// ReadAba reads and parses an .aba file using the Unity AssetBundle UnityFS format.
func ReadAba(r io.ReadSeeker) (*Aba, error) {
	if r == nil {
		return nil, fmt.Errorf(".aba reader is nil")
	}

	start, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("get .aba start offset: %w", err)
	}
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("get .aba stream size: %w", err)
	}
	if end < start {
		return nil, fmt.Errorf("invalid .aba stream range [%d, %d)", start, end)
	}
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return nil, fmt.Errorf("restore .aba start offset: %w", err)
	}

	abaFile := &Aba{
		abaStart: start,
		abaSize:  end - start,
	}

	// 1. 读取 Header
	if err := abaFile.readHeader(r); err != nil {
		return nil, fmt.Errorf("read .aba header failed: %w", err)
	}

	if abaFile.Header.Signature != signatureUnityFS {
		if len(abaFile.Header.Signature) >= 4 && abaFile.Header.Signature[:4] == signatureAbap {
			return nil, fmt.Errorf("this .aba file is encrypted (unsupported). Please install the original DLC and launch the game once to decrypt it")
		}
		return nil, fmt.Errorf("unsupported signature: %q (only UnityFS supported)", abaFile.Header.Signature)
	}
	if abaFile.Header.Version < minSupportedAbaVersion || abaFile.Header.Version > maxSupportedAbaVersion {
		return nil, fmt.Errorf("unsupported UnityFS version %d (supported: %d-%d)", abaFile.Header.Version, minSupportedAbaVersion, maxSupportedAbaVersion)
	}
	if abaFile.Header.FSHeader.TotalFileSize != abaFile.abaSize {
		return nil, fmt.Errorf("UnityFS total file size mismatch: header=%d, stream=%d", abaFile.Header.FSHeader.TotalFileSize, abaFile.abaSize)
	}
	if abaFile.Header.FSHeader.TotalFileSize <= 0 {
		return nil, fmt.Errorf("invalid UnityFS total file size %d", abaFile.Header.FSHeader.TotalFileSize)
	}
	if abaFile.Header.FSHeader.Flags&uint32(FlagHasDirectoryInfo) == 0 {
		return nil, fmt.Errorf("UnityFS .aba has no combined block/directory info")
	}
	if abaFile.Header.FSHeader.CompressedSize == 0 || abaFile.Header.FSHeader.DecompressedSize == 0 {
		return nil, fmt.Errorf("invalid block/directory info sizes compressed=%d decompressed=%d", abaFile.Header.FSHeader.CompressedSize, abaFile.Header.FSHeader.DecompressedSize)
	}
	if abaFile.Header.FSHeader.CompressedSize > maxBlockAndDirInfoSize || abaFile.Header.FSHeader.DecompressedSize > maxBlockAndDirInfoSize {
		return nil, fmt.Errorf("block/directory info too large: compressed=%d decompressed=%d (limit %d)", abaFile.Header.FSHeader.CompressedSize, abaFile.Header.FSHeader.DecompressedSize, maxBlockAndDirInfoSize)
	}

	// 2. 对齐到 16 字节（version >= 7）
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("get UnityFS header end: %w", err)
	}
	relativePos := pos - start
	if relativePos < 0 || relativePos > abaFile.abaSize {
		return nil, fmt.Errorf("invalid UnityFS header end %d", relativePos)
	}
	if abaFile.Header.Version >= 7 {
		aligned, ok := alignInt64(relativePos, 16)
		if !ok || aligned > abaFile.abaSize {
			return nil, fmt.Errorf("UnityFS aligned header end is out of bounds")
		}
		relativePos = aligned
		if _, err := r.Seek(start+relativePos, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek past UnityFS header padding: %w", err)
		}
	}
	abaFile.headerEnd = relativePos

	// 3. 读取 BlockAndDirInfo
	if err := abaFile.readBlockAndDirInfo(r); err != nil {
		return nil, fmt.Errorf("read block and dir info failed: %w", err)
	}

	// 4. 设置数据区 reader
	readerAt, ok := r.(io.ReaderAt)
	if !ok {
		return nil, fmt.Errorf(".aba reader must implement io.ReaderAt")
	}
	abaFile.DataReader = io.NewSectionReader(readerAt, start+abaFile.fileDataOffset, abaFile.compressedDataSize)

	return abaFile, nil
}

// GetFileNames 返回 .aba 中所有文件的名称列表。
func (b *Aba) GetFileNames() []string {
	if b == nil {
		return nil
	}
	names := make([]string, len(b.BlockInfo.DirectoryInfos))
	for i, d := range b.BlockInfo.DirectoryInfos {
		names[i] = d.Name
	}
	return names
}

// GetFileData 读取指定索引的文件数据（自动处理 LZ4 分块解压）
func (b *Aba) GetFileData(index int64) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	if index < 0 || index >= int64(len(b.BlockInfo.DirectoryInfos)) {
		return nil, fmt.Errorf("file index %d out of range [0, %d)", index, len(b.BlockInfo.DirectoryInfos))
	}
	dir := &b.BlockInfo.DirectoryInfos[index]
	data, err := b.GetFileDataRange(index, 0, dir.DecompressedSize)
	if err != nil {
		return nil, fmt.Errorf("read file %q data failed: %w", dir.Name, err)
	}

	return data, nil
}

// GetFileDataByName 按名称读取文件数据
func (b *Aba) GetFileDataByName(name string) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	for i, d := range b.BlockInfo.DirectoryInfos {
		if d.Name == name {
			return b.GetFileData(int64(i))
		}
	}
	return nil, fmt.Errorf("file %q not found in .aba", name)
}

// GetFileDataRange reads a byte range from an .aba entry selected by directory
// index. The offset and size are relative to that directory entry.
func (b *Aba) GetFileDataRange(index int64, offset int64, size int64) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	if index < 0 || index >= int64(len(b.BlockInfo.DirectoryInfos)) {
		return nil, fmt.Errorf("file index %d out of range [0, %d)", index, len(b.BlockInfo.DirectoryInfos))
	}
	dir := &b.BlockInfo.DirectoryInfos[index]
	if offset < 0 || size < 0 {
		return nil, fmt.Errorf("invalid file range offset=%d size=%d", offset, size)
	}
	end, ok := addNonNegativeInt64(offset, size)
	if !ok || end > dir.DecompressedSize {
		return nil, fmt.Errorf("file %q range offset=%d size=%d out of bounds %d", dir.Name, offset, size, dir.DecompressedSize)
	}
	absoluteOffset, ok := addNonNegativeInt64(dir.Offset, offset)
	if !ok {
		return nil, fmt.Errorf("file %q absolute range offset overflows", dir.Name)
	}
	return b.readDataRange(absoluteOffset, size)
}

// GetFileDataRangeByName reads a byte range from an .aba entry by name.
// The offset and size are relative to the decompressed file entry, not the
// whole UnityFS data stream.
func (b *Aba) GetFileDataRangeByName(name string, offset int64, size int64) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	for index, d := range b.BlockInfo.DirectoryInfos {
		if d.Name != name {
			continue
		}
		return b.GetFileDataRange(int64(index), offset, size)
	}
	return nil, fmt.Errorf("file %q not found in .aba", name)
}

// readHeader 读取 UnityFS 文件头
func (b *Aba) readHeader(r io.ReadSeeker) error {
	// 1. Signature (null-terminated string)
	// Reserve version, two mandatory string terminators, and FSHeader.
	sig, err := b.readHeaderNullString(r, 4+1+1+20)
	if err != nil {
		return fmt.Errorf("read signature failed: %w", err)
	}
	b.Header.Signature = sig

	// 2. Version (uint32 big-endian)
	if err := binary.Read(r, binary.BigEndian, &b.Header.Version); err != nil {
		return fmt.Errorf("read version failed: %w", err)
	}

	// 3. GenerationVersion (null-terminated string)
	// Reserve the engine-version terminator and FSHeader.
	genVer, err := b.readHeaderNullString(r, 1+20)
	if err != nil {
		return fmt.Errorf("read generation version failed: %w", err)
	}
	b.Header.GenerationVersion = genVer

	// 4. EngineVersion (null-terminated string)
	engVer, err := b.readHeaderNullString(r, 20)
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
func (b *Aba) readBlockAndDirInfo(r io.ReadSeeker) error {
	flags := b.Header.FSHeader.Flags
	compressedSize := int64(b.Header.FSHeader.CompressedSize)
	decompressedSize := int64(b.Header.FSHeader.DecompressedSize)
	compType := b.Header.FSHeader.GetCompressionType()
	if compType != CompressionNone && compType != CompressionLZMA && compType != CompressionLZ4 && compType != CompressionLZ4HC {
		return fmt.Errorf("unknown block/directory info compression type: %d", compType)
	}
	if compType == CompressionNone && compressedSize != decompressedSize {
		return fmt.Errorf("uncompressed block/directory info size mismatch: compressed=%d decompressed=%d", compressedSize, decompressedSize)
	}

	// 确定 BlockAndDirInfo 的位置
	infoOffset := b.headerEnd
	if flags&uint32(FlagBlockAndDirAtEnd) != 0 {
		// 位于文件末尾
		infoOffset = b.abaSize - compressedSize
	}
	infoEnd, ok := addNonNegativeInt64(infoOffset, compressedSize)
	if !ok || infoOffset < b.headerEnd || infoEnd > b.abaSize {
		return fmt.Errorf("block/directory info range [%d, %d) is outside .aba size %d", infoOffset, infoEnd, b.abaSize)
	}
	if _, err := r.Seek(b.abaStart+infoOffset, io.SeekStart); err != nil {
		return fmt.Errorf("seek to block/directory info: %w", err)
	}
	b.blockAndDirOffset = infoOffset

	// 读取压缩数据
	compressedData := make([]byte, int64(compressedSize))
	if _, err := io.ReadFull(r, compressedData); err != nil {
		return fmt.Errorf("read compressed block info failed: %w", err)
	}

	// 解压
	var infoData []byte
	switch compType {
	case CompressionNone:
		infoData = compressedData
	case CompressionLZ4, CompressionLZ4HC:
		infoData = make([]byte, int64(decompressedSize))
		n, err := lz4.UncompressBlock(compressedData, infoData)
		if err != nil {
			return fmt.Errorf("LZ4 decompress block info failed: %w", err)
		}
		if n != len(infoData) {
			return fmt.Errorf("LZ4 block/directory info size mismatch: got %d, want %d", n, len(infoData))
		}
	case CompressionLZMA:
		decoded, err := decompressUnityLZMA(compressedData, int64(decompressedSize))
		if err != nil {
			return fmt.Errorf("LZMA decompress block/directory info failed: %w", err)
		}
		infoData = decoded
	}

	// 解析 BlockAndDirInfo
	if err := b.parseBlockAndDirInfo(infoData); err != nil {
		return err
	}
	return b.validateDataLayout()
}

// parseBlockAndDirInfo 从解压后的字节中解析 BlockAndDirInfo
func (b *Aba) parseBlockAndDirInfo(data []byte) error {
	if len(data) < 24 {
		return fmt.Errorf("block/directory info is too short: %d bytes", len(data))
	}
	r := binaryio.NewEndianReader(data, binary.BigEndian)
	var parsed BlockAndDirInfo

	// 1. Hash (16 bytes)
	if err := r.ReadFull(parsed.Hash[:]); err != nil {
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
	blockCount := int64(blockCountRaw)
	if r.Remaining() < 4 || blockCount > int64((r.Remaining()-4)/10) {
		return fmt.Errorf("block count %d cannot fit in remaining metadata (%d bytes)", blockCount, r.Remaining())
	}

	parsed.BlockInfos = makeABACountedSliceForAppend[BlockInfo](blockCount)
	for i := int64(0); i < blockCount; i++ {
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
		block := BlockInfo{
			DecompressedSize: decompressedSize,
			CompressedSize:   compressedSize,
			Flags:            flags,
		}
		if decompressedSize > maxAbaBlockSize || compressedSize > maxAbaBlockSize {
			return fmt.Errorf("block info %d is too large: compressed=%d decompressed=%d (per-block limit %d)", i, compressedSize, decompressedSize, maxAbaBlockSize)
		}
		switch compType := block.GetCompressionType(); compType {
		case CompressionNone:
			if compressedSize != decompressedSize {
				return fmt.Errorf("uncompressed block info %d size mismatch: compressed=%d decompressed=%d", i, compressedSize, decompressedSize)
			}
		case CompressionLZMA, CompressionLZ4, CompressionLZ4HC:
			if compressedSize == 0 || decompressedSize == 0 {
				return fmt.Errorf("compressed block info %d has zero size", i)
			}
		default:
			return fmt.Errorf("data block %d has unknown compression type %d", i, compType)
		}
		parsed.BlockInfos = append(parsed.BlockInfos, block)
	}

	// 3. DirectoryCount + DirectoryInfos
	dirCountRaw, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read directory count: %w", err)
	}
	if dirCountRaw < 0 {
		return fmt.Errorf("negative directory count %d", dirCountRaw)
	}
	dirCount := int64(dirCountRaw)
	const minimumDirectoryInfoSize = 8 + 8 + 4 + 1
	if dirCount > int64(r.Remaining()/minimumDirectoryInfoSize) {
		return fmt.Errorf("directory count %d cannot fit in remaining metadata (%d bytes)", dirCount, r.Remaining())
	}

	parsed.DirectoryInfos = makeABACountedSliceForAppend[DirectoryInfo](dirCount)
	for i := int64(0); i < dirCount; i++ {
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
		if offset < 0 || decompSize < 0 {
			return fmt.Errorf("directory info %d has invalid range offset=%d size=%d", i, offset, decompSize)
		}

		parsed.DirectoryInfos = append(parsed.DirectoryInfos, DirectoryInfo{
			Offset:           offset,
			DecompressedSize: decompSize,
			Flags:            flags,
			Name:             name,
		})
	}
	if r.Remaining() != 0 {
		parsed.TrailingData, err = r.ReadBytes(r.Remaining())
		if err != nil {
			return fmt.Errorf("read block/directory trailing data: %w", err)
		}
	}

	var totalDecompressed int64
	for i, block := range parsed.BlockInfos {
		var ok bool
		totalDecompressed, ok = addNonNegativeInt64(totalDecompressed, int64(block.DecompressedSize))
		if !ok {
			return fmt.Errorf("decompressed block size sum overflows at block %d", i)
		}
	}
	for i, dir := range parsed.DirectoryInfos {
		end, ok := addNonNegativeInt64(dir.Offset, dir.DecompressedSize)
		if !ok || end > totalDecompressed {
			return fmt.Errorf("directory info %d range offset=%d size=%d exceeds decompressed data size %d", i, dir.Offset, dir.DecompressedSize, totalDecompressed)
		}
	}

	b.BlockInfo = parsed
	return nil
}

// readDataRange returns a slice from the decompressed UnityFS data stream.
// It only reads and decompresses blocks that overlap the requested range, so
// extracting many files does not retain or repeatedly allocate a full .aba
// decompression buffer.
func (b *Aba) readDataRange(offset int64, size int64) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	if offset < 0 || size < 0 {
		return nil, fmt.Errorf("invalid range offset=%d size=%d", offset, size)
	}
	if size > maxAbaReadSize {
		return nil, fmt.Errorf("requested range size %d exceeds in-memory read limit %d; request smaller ranges with GetFileDataRangeByName", size, maxAbaReadSize)
	}

	totalSize, err := sumDecompressedBlockSizes(b.BlockInfo.BlockInfos)
	if err != nil {
		return nil, err
	}
	end, ok := addNonNegativeInt64(offset, size)
	if !ok || end > totalSize {
		return nil, fmt.Errorf("range offset=%d size=%d out of decompressed data bounds %d", offset, size, totalSize)
	}
	if size == 0 {
		return []byte{}, nil
	}

	readerAt, ok := b.DataReader.(io.ReaderAt)
	if !ok {
		return nil, fmt.Errorf(".aba data reader does not support random access")
	}

	result := make([]byte, int64(size))
	written := 0
	var compressedOffset int64
	var decompressedOffset int64
	for blockIndex, block := range b.BlockInfo.BlockInfos {
		blockStart := decompressedOffset
		blockEnd, ok := addNonNegativeInt64(blockStart, int64(block.DecompressedSize))
		if !ok {
			return nil, fmt.Errorf("decompressed offset overflows at block[%d]", blockIndex)
		}
		overlaps := offset < blockEnd && end > blockStart

		if overlaps {
			compressed := make([]byte, int64(block.CompressedSize))
			if _, err := readerAt.ReadAt(compressed, compressedOffset); err != nil {
				return nil, fmt.Errorf("read block[%d] data: %w", blockIndex, err)
			}

			blockData, err := decompressDataBlock(block, compressed)
			if err != nil {
				return nil, fmt.Errorf("decompress block[%d]: %w", blockIndex, err)
			}
			if int64(len(blockData)) != int64(block.DecompressedSize) {
				return nil, fmt.Errorf("block[%d] decompressed size mismatch: got %d, want %d", blockIndex, len(blockData), block.DecompressedSize)
			}

			copyStart := maxInt64(offset, blockStart) - blockStart
			copyEnd := minInt64(end, blockEnd) - blockStart
			written += copy(result[written:], blockData[int64(copyStart):int64(copyEnd)])
		}

		compressedOffset, ok = addNonNegativeInt64(compressedOffset, int64(block.CompressedSize))
		if !ok {
			return nil, fmt.Errorf("compressed offset overflows at block[%d]", blockIndex)
		}
		decompressedOffset = blockEnd
	}

	if int64(written) != size {
		return nil, fmt.Errorf("range read size mismatch: got %d, want %d", written, size)
	}
	return result, nil
}

func decompressDataBlock(block BlockInfo, compressed []byte) ([]byte, error) {
	if block.CompressedSize > maxAbaBlockSize || block.DecompressedSize > maxAbaBlockSize {
		return nil, fmt.Errorf("block is too large: compressed=%d decompressed=%d", block.CompressedSize, block.DecompressedSize)
	}
	if int64(len(compressed)) != int64(block.CompressedSize) {
		return nil, fmt.Errorf("compressed input size mismatch: got %d, want %d", len(compressed), block.CompressedSize)
	}
	switch compType := block.GetCompressionType(); compType {
	case CompressionNone:
		if block.CompressedSize != block.DecompressedSize {
			return nil, fmt.Errorf("uncompressed block size mismatch: compressed=%d decompressed=%d", block.CompressedSize, block.DecompressedSize)
		}
		return compressed, nil
	case CompressionLZ4, CompressionLZ4HC:
		if block.CompressedSize == 0 || block.DecompressedSize == 0 {
			return nil, fmt.Errorf("compressed block has zero size")
		}
		dst := make([]byte, int64(block.DecompressedSize))
		n, err := lz4.UncompressBlock(compressed, dst)
		if err != nil {
			return nil, err
		}
		if n != len(dst) {
			return nil, fmt.Errorf("LZ4 decompressed size mismatch: got %d, want %d", n, len(dst))
		}
		return dst, nil
	case CompressionLZMA:
		data, err := decompressUnityLZMA(compressed, int64(block.DecompressedSize))
		if err != nil {
			return nil, fmt.Errorf("LZMA decompress: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unknown block compression type: %d", compType)
	}
}

// UnityFS stores the standard five-byte LZMA properties header but omits the
// classic format's eight-byte uncompressed-size field because that size is
// already present in the block table. Insert the known size and delegate the
// actual codec work to the xz/lzma implementation.
func decompressUnityLZMA(compressed []byte, decompressedSize int64) ([]byte, error) {
	if decompressedSize < 0 {
		return nil, fmt.Errorf("negative decompressed size %d", decompressedSize)
	}
	if len(compressed) < 5 {
		return nil, fmt.Errorf("compressed stream is shorter than the 5-byte properties header")
	}
	var header [13]byte
	copy(header[:5], compressed[:5])
	binary.LittleEndian.PutUint64(header[5:], uint64(decompressedSize))
	stream := io.MultiReader(bytes.NewReader(header[:]), bytes.NewReader(compressed[5:]))
	reader, err := (lzma.ReaderConfig{DictCap: math.MaxInt32}).NewReader(stream)
	if err != nil {
		return nil, err
	}
	result := make([]byte, decompressedSize)
	if _, err := io.ReadFull(reader, result); err != nil {
		return nil, err
	}
	extra, err := io.ReadAll(io.LimitReader(reader, 1))
	if err != nil {
		return nil, err
	}
	if len(extra) != 0 {
		return nil, fmt.Errorf("decompressed stream exceeds declared size %d", decompressedSize)
	}
	return result, nil
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

func (b *Aba) validateDataLayout() error {
	dataOffset := b.headerEnd
	if b.Header.FSHeader.Flags&uint32(FlagBlockAndDirAtEnd) == 0 {
		var ok bool
		dataOffset, ok = addNonNegativeInt64(b.blockAndDirOffset, int64(b.Header.FSHeader.CompressedSize))
		if !ok {
			return fmt.Errorf("data offset overflows after block/directory info")
		}
	}
	if b.Header.FSHeader.Flags&uint32(FlagBlockInfoNeedPaddingAtStart) != 0 {
		aligned, ok := alignInt64(dataOffset, 16)
		if !ok {
			return fmt.Errorf("aligned data offset overflows")
		}
		dataOffset = aligned
	}
	dataEnd := b.abaSize
	if b.Header.FSHeader.Flags&uint32(FlagBlockAndDirAtEnd) != 0 {
		dataEnd = b.blockAndDirOffset
	}
	if dataOffset < b.headerEnd || dataOffset > dataEnd {
		return fmt.Errorf("invalid compressed data range [%d, %d)", dataOffset, dataEnd)
	}

	var compressedSize int64
	for i, block := range b.BlockInfo.BlockInfos {
		var ok bool
		compressedSize, ok = addNonNegativeInt64(compressedSize, int64(block.CompressedSize))
		if !ok {
			return fmt.Errorf("compressed block size sum overflows at block %d", i)
		}
	}
	available := dataEnd - dataOffset
	if compressedSize != available {
		return fmt.Errorf("compressed data size mismatch: block table=%d, file range=%d", compressedSize, available)
	}
	b.fileDataOffset = dataOffset
	b.compressedDataSize = compressedSize
	return nil
}

func sumDecompressedBlockSizes(blocks []BlockInfo) (int64, error) {
	var total int64
	for i, block := range blocks {
		var ok bool
		total, ok = addNonNegativeInt64(total, int64(block.DecompressedSize))
		if !ok {
			return 0, fmt.Errorf("decompressed block size sum overflows at block %d", i)
		}
	}
	return total, nil
}

func addNonNegativeInt64(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || a > int64(^uint64(0)>>1)-b {
		return 0, false
	}
	return a + b, true
}

func alignInt64(value, alignment int64) (int64, bool) {
	if value < 0 || alignment <= 0 {
		return 0, false
	}
	remainder := value % alignment
	if remainder == 0 {
		return value, true
	}
	return addNonNegativeInt64(value, alignment-remainder)
}

func (b *Aba) readHeaderNullString(r io.ReadSeeker, reservedBytes int64) (string, error) {
	if b == nil || r == nil {
		return "", fmt.Errorf("nil .aba header reader")
	}
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}
	relativePos := pos - b.abaStart
	if relativePos < 0 || relativePos > b.abaSize || reservedBytes < 0 || reservedBytes > b.abaSize-relativePos {
		return "", fmt.Errorf(".aba has no room for the remaining header fields")
	}
	return readNullStringWithin(r, b.abaSize-relativePos-reservedBytes)
}

func readNullStringWithin(r io.ReadSeeker, maxBytes int64) (string, error) {
	if r == nil {
		return "", fmt.Errorf("reader is nil")
	}
	if maxBytes <= 0 {
		return "", fmt.Errorf("no room for a null-terminated string")
	}
	start, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}

	var scratch [4096]byte
	var scanned int64
	for scanned < maxBytes {
		chunkSize := int64(len(scratch))
		if remaining := maxBytes - scanned; chunkSize > remaining {
			chunkSize = remaining
		}
		n, readErr := io.ReadFull(r, scratch[:chunkSize])
		if terminator := bytes.IndexByte(scratch[:n], 0); terminator >= 0 {
			length := scanned + int64(terminator)
			if strconv.IntSize == 32 && length > math.MaxInt32 {
				return "", fmt.Errorf("null-terminated string length %d exceeds platform int capacity", length)
			}
			if _, err := r.Seek(start, io.SeekStart); err != nil {
				return "", err
			}
			data := make([]byte, length)
			if _, err := io.ReadFull(r, data); err != nil {
				return "", err
			}
			var nul [1]byte
			if _, err := io.ReadFull(r, nul[:]); err != nil {
				return "", err
			}
			return string(data), nil
		}
		scanned += int64(n)
		if readErr != nil {
			return "", readErr
		}
	}
	return "", fmt.Errorf("null-terminated string is not terminated within %d available bytes", maxBytes)
}
