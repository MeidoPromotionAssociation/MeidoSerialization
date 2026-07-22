package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"github.com/pierrec/lz4/v4"
	"github.com/ulikunitz/xz/lzma"
)

// .aba
// KCES 资源包使用标准 Unity AssetBundle UnityFS 格式，Unity 5.3 及以上版本使用 UnityFS 签名
// 文件包含可压缩的块目录、资源数据块以及一个或多个 Unity SerializedFile
// 文件整体结构如下，所有头部字段均使用 Big-Endian
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
//	[BlockAndDirInfo]（位置由 Flags 决定，可能被 LZ4 或 LZMA 压缩）
//	  - Hash: 16 bytes
//	  - BlockCount: int32
//	  - BlockInfos[BlockCount]: DecompressedSize(uint32) + CompressedSize(uint32) + Flags(uint16)
//	  - DirectoryCount: int32
//	  - DirectoryInfos[DirectoryCount]: Offset(int64) + DecompressedSize(int64) + Flags(uint32) + Name(string)
//	[Data Blocks]（可能被 LZ4 分块压缩，每块最大 0x20000 bytes）
//	  - 包含一个或多个 AssetsFile（Unity 序列化文件）
//
//
// .aba
// KCES .aba files use the standard Unity AssetBundle UnityFS format; Unity 5.3 and later use the UnityFS signature
// A file contains an optionally compressed block directory, resource-data blocks, and one or more Unity SerializedFiles
// The overall layout is shown below, with all header fields encoded as Big-Endian
//	[Header]
//	  - Signature: "UnityFS" (null-terminated string)
//	  - Version: uint32 (usually file-format version 6 through 8)
//	  - GenerationVersion: null-terminated string such as "5.x.x"
//	  - EngineVersion: null-terminated string such as "2021.3.3f1"
//	  - FSHeader: TotalFileSize(int64), CompressedSize(uint32), DecompressedSize(uint32), Flags(uint32)
//	[BlockAndDirInfo] (location selected by Flags; optionally compressed with LZ4 or LZMA)
//	  - Hash: 16 bytes
//	  - BlockInfos: decompressed size, compressed size, and flags for each block
//	  - DirectoryInfos: offset, decompressed size, flags, and name for each entry
//	[Data Blocks] (optionally split into LZ4 blocks of at most 0x20000 bytes)
//	  - Contains one or more AssetsFiles (Unity serialized files)

const (
	signatureUnityFS = "UnityFS" // signatureUnityFS 是 Unity 5.3 及以上 AssetBundle 的明文签名 / signatureUnityFS is the plaintext AssetBundle signature for Unity 5.3 and later
	signatureAbap    = "abap"    // signatureAbap 是需要 key 文件解密的 KCES 加密 AssetBundle 签名前缀 / signatureAbap is the KCES encrypted AssetBundle signature prefix requiring key-file decryption

	// 下列常量定义 FSHeader Flags 位
	// The following constants define FSHeader flag bits
	CompressionNone  = 0x00 // CompressionNone 表示不压缩 / CompressionNone denotes no compression
	CompressionLZMA  = 0x01 // CompressionLZMA 表示 LZMA 压缩 / CompressionLZMA denotes LZMA compression
	CompressionLZ4   = 0x02 // CompressionLZ4 表示 LZ4 压缩 / CompressionLZ4 denotes LZ4 compression
	CompressionLZ4HC = 0x03 // CompressionLZ4HC 表示高压缩率 LZ4HC 压缩 / / CompressionLZ4HC denotes higher-ratio LZ4HC compression
	CompressionMask  = 0x3f // CompressionMask 是低六位压缩类型掩码 / CompressionMask is the compression-type mask in the low six bits

	FlagHasDirectoryInfo            = 0x40  // FlagHasDirectoryInfo 表示块信息与目录信息组合保存 / FlagHasDirectoryInfo denotes combined block and directory information
	FlagBlockAndDirAtEnd            = 0x80  // FlagBlockAndDirAtEnd 表示 BlockAndDirInfo 位于文件末尾 / FlagBlockAndDirAtEnd denotes BlockAndDirInfo stored at file end
	FlagOldWebPluginCompat          = 0x100 // FlagOldWebPluginCompat 是旧版 Web 插件兼容位 / FlagOldWebPluginCompat is the old Web-plugin compatibility bit
	FlagBlockInfoNeedPaddingAtStart = 0x200 // FlagBlockInfoNeedPaddingAtStart 表示数据块起始需要 16 字节对齐 / FlagBlockInfoNeedPaddingAtStart denotes 16-byte alignment before data blocks

	// 下列常量定义 DirectoryInfo Flags 位
	// The following constants define DirectoryInfo flag bits
	DirFlagSerializedFile = 0x04 // DirFlagSerializedFile 表示条目是序列化 AssetsFile / DirFlagSerializedFile denotes a serialized AssetsFile entry

	minSupportedAbaVersion = 6 // 最低支持的 Aba 版本 / Minimum supported Aba version
	maxSupportedAbaVersion = 8 // 最高支持的 Aba 版本 / Maximum supported Aba version

	// UnityFS 块目录元数据相对数据流很小，最大的 KCES 样本约为 400 KiB，而其解压后的 .resS 条目超过 4 GiB
	// 因此限制元数据与单块分配，同时不对逻辑数据流施加较低上限
	// UnityFS block and directory metadata is small compared with the data stream; the largest KCES sample is about 400 KiB while its decompressed .resS entry exceeds 4 GiB
	// Metadata and per-block allocations are therefore bounded without imposing a low limit on the logical data stream
	maxBlockAndDirInfoSize = 64 << 20  // 64 MiB
	maxAbaBlockSize        = 256 << 20 // 256 MiB
	// parts_bv002.aba 包含一个 799,993,424 字节的真实 KCES SerializedFile 目录
	// 单次分配 API 保持有限，同时允许所有已观察到的 SerializedFile 传给 ReadAssetsFile；数 GiB 的 .resS 条目仍须通过较小范围读取
	// parts_bv002.aba contains a real KCES SerializedFile directory of 799,993,424 bytes
	// The single-allocation API remains bounded while allowing every observed SerializedFile to reach ReadAssetsFile; multi-gigabyte .resS entries must still use smaller range reads
	maxAbaReadSize = 1 << 30 // 1 GiB
)

// Aba 表示一个采用 Unity AssetBundle UnityFS 格式的 .aba 文件
// Aba represents one .aba file using the Unity AssetBundle UnityFS format
type Aba struct {
	Header     AbaHeader       // 文件头 / File header
	BlockInfo  BlockAndDirInfo // 压缩块和目录信息 / Compressed block and directory information
	DataReader io.ReadSeeker   // 压缩数据区 reader；各块按需解压 / Compressed data-area reader; blocks are decompressed on demand

	abaStart           int64 // .aba 在底层流中的起始绝对偏移 / Absolute start offset of the .aba in the backing stream
	abaSize            int64 // 从 abaStart 起计算的 .aba 总字节数 / Total .aba byte length measured from abaStart
	headerEnd          int64 // 相对于 abaStart 的对齐后头部末尾 / Aligned header end relative to abaStart
	blockAndDirOffset  int64 // 块目录信息相对于 abaStart 的偏移 / Block-and-directory info offset relative to abaStart
	fileDataOffset     int64 // 压缩数据块相对于 abaStart 的起始偏移 / Compressed data-block start offset relative to abaStart
	compressedDataSize int64 // 块表中全部压缩数据块的总字节数 / Total compressed byte count of all data blocks in the block table
}

// AbaHeader 表示 UnityFS 文件头，所有字段使用 Big-Endian 编码
// AbaHeader represents the UnityFS header with Big-Endian fields
type AbaHeader struct {
	Signature         string   // 签名，受支持的明文值为 "UnityFS" / Signature, with "UnityFS" as the supported plaintext value
	Version           uint32   // UnityFS 文件格式版本，当前支持 6 至 8 / UnityFS file-format version, currently supporting 6 through 8
	GenerationVersion string   // NUL 结尾的生成版本字符串，如 "5.x.x" / NUL-terminated generation-version string such as "5.x.x"
	EngineVersion     string   // NUL 结尾的 Unity 引擎版本字符串 / NUL-terminated Unity engine-version string
	FSHeader          FSHeader // 文件流头部 / File stream header
}

// FSHeader 表示紧跟在 AbaHeader 字符串字段之后的 UnityFS 文件流头部
// FSHeader represents UnityFS stream header fields following the AbaHeader strings
type FSHeader struct {
	TotalFileSize    int64  // 整个文件的总大小 / Total file size
	CompressedSize   uint32 // BlockAndDirInfo 压缩后的字节大小 / Compressed size of BlockAndDirInfo
	DecompressedSize uint32 // BlockAndDirInfo 解压后的字节大小 / Decompressed size of BlockAndDirInfo
	Flags            uint32 // 组合压缩类型与布局位的标志 / Flags combining compression type and layout bits
}

// BlockAndDirInfo 包含数据块列表和文件目录列表
// BlockAndDirInfo contains the data-block and directory lists
type BlockAndDirInfo struct {
	Hash           [16]byte        // 16 字节哈希 / 16-byte hash
	BlockInfos     []BlockInfo     // 数据压缩块信息列表 / Data block info list
	DirectoryInfos []DirectoryInfo // 文件目录条目列表 / File directory entry list
	TrailingData   []byte          // 已知目录表之后的未解析字节 / Unparsed bytes following the known directory table
}

// BlockInfo 描述一个数据压缩块的元信息
// BlockInfo describes one compressed data block
type BlockInfo struct {
	DecompressedSize uint32 // 解压后大小 / Decompressed size
	CompressedSize   uint32 // 压缩后大小，未压缩时与 DecompressedSize 相同 / Compressed size, equal to DecompressedSize when uncompressed
	Flags            uint16 // 标志位：低 6 位为压缩类型，bit6 表示是否为流式块 / Flags, low 6 bits are compression type and bit6 marks streamed blocks
}

// DirectoryInfo 描述 .aba 文件内的一个目录条目
// DirectoryInfo describes one directory entry inside an .aba file
type DirectoryInfo struct {
	Offset           int64  // 相对于数据区起始的偏移量 / Offset relative to the data area start
	DecompressedSize int64  // 解压后大小 / Decompressed size
	Flags            uint32 // 目录条目标志，0x04 表示序列化 AssetsFile / Directory-entry flags, with 0x04 denoting a serialized AssetsFile
	Name             string // 文件名 / File name
}

// GetCompressionType 返回数据块 Flags 低六位中的压缩类型
// GetCompressionType returns the compression type from the low six bits of data-block Flags
func (b *BlockInfo) GetCompressionType() byte {
	return byte(b.Flags & uint16(CompressionMask))
}

// IsSerialized 返回目录条目是否标记为序列化 AssetsFile
// IsSerialized reports whether the directory entry is marked as a serialized AssetsFile
func (d *DirectoryInfo) IsSerialized() bool {
	return d.Flags&DirFlagSerializedFile != 0
}

// GetCompressionType 返回 FSHeader Flags 低六位中的块目录元数据压缩类型
// GetCompressionType returns the block-directory metadata compression type from the low six bits of FSHeader Flags
func (h *FSHeader) GetCompressionType() byte {
	return byte(h.Flags & uint32(CompressionMask))
}

// ReadAba 从 reader 的当前位置读取并解析采用 Unity AssetBundle UnityFS 格式的 .aba 文件
// ReadAba reads and parses an .aba file in Unity AssetBundle UnityFS format from the reader's current position
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

	// 首先读取 UnityFS Header
	// First read the UnityFS Header
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

	// UnityFS version 7 及以上将头部末尾对齐到 16 字节
	// UnityFS version 7 and later align the end of the header to 16 bytes
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

	// 随后读取并解析 BlockAndDirInfo
	// Then read and parse BlockAndDirInfo
	if err := abaFile.readBlockAndDirInfo(r); err != nil {
		return nil, fmt.Errorf("read block and dir info failed: %w", err)
	}

	// 最后为压缩数据区创建支持随机访问的 section reader
	// Finally create a random-access section reader for the compressed data area
	readerAt, ok := r.(io.ReaderAt)
	if !ok {
		return nil, fmt.Errorf(".aba reader must implement io.ReaderAt")
	}
	abaFile.DataReader = io.NewSectionReader(readerAt, start+abaFile.fileDataOffset, abaFile.compressedDataSize)

	return abaFile, nil
}

// GetFileNames 按目录表顺序返回 .aba 中所有文件名称
// GetFileNames returns every file name in .aba directory-table order
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

// GetFileData 读取指定目录索引的完整解压文件数据
// GetFileData reads complete decompressed file data for a directory index
func (b *Aba) GetFileData(index int) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	if index < 0 || index >= len(b.BlockInfo.DirectoryInfos) {
		return nil, fmt.Errorf("file index %d out of range [0, %d)", index, len(b.BlockInfo.DirectoryInfos))
	}
	dir := &b.BlockInfo.DirectoryInfos[index]
	data, err := b.GetFileDataRange(index, 0, dir.DecompressedSize)
	if err != nil {
		return nil, fmt.Errorf("read file %q data failed: %w", dir.Name, err)
	}

	return data, nil
}

// GetFileDataByName 按目录条目名称读取完整解压文件数据
// GetFileDataByName reads complete decompressed file data by directory-entry name
func (b *Aba) GetFileDataByName(name string) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	for i, d := range b.BlockInfo.DirectoryInfos {
		if d.Name == name {
			return b.GetFileData(i)
		}
	}
	return nil, fmt.Errorf("file %q not found in .aba", name)
}

// GetFileDataRange 从目录索引指定的 .aba 条目读取字节范围，offset 和 size 相对于该目录条目
// GetFileDataRange reads a byte range from an .aba entry selected by directory index, with offset and size relative to that entry
func (b *Aba) GetFileDataRange(index int, offset int64, size int64) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	if index < 0 || index >= len(b.BlockInfo.DirectoryInfos) {
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

// GetFileDataRangeByName 按名称读取 .aba 条目的字节范围，offset 和 size 相对于解压后的文件条目而非整个 UnityFS 数据流
// GetFileDataRangeByName reads a byte range from a named .aba entry, with offset and size relative to the decompressed entry rather than the whole UnityFS data stream
func (b *Aba) GetFileDataRangeByName(name string, offset int64, size int64) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf(".aba is nil")
	}
	for index, d := range b.BlockInfo.DirectoryInfos {
		if d.Name != name {
			continue
		}
		return b.GetFileDataRange(index, offset, size)
	}
	return nil, fmt.Errorf("file %q not found in .aba", name)
}

// readHeader 按 Big-Endian 读取 UnityFS 头部字段
// readHeader reads UnityFS header fields in Big-Endian order
func (b *Aba) readHeader(r io.ReadSeeker) error {
	// 首先读取 NUL 结尾的 Signature，并为 Version、两个必需字符串终止符和 FSHeader 预留空间
	// First read the NUL-terminated Signature while reserving room for Version, two required string terminators, and FSHeader
	sig, err := b.readHeaderNullString(r, 4+1+1+20)
	if err != nil {
		return fmt.Errorf("read signature failed: %w", err)
	}
	b.Header.Signature = sig

	// Version 是 Big-Endian UInt32
	// Version is a Big-Endian UInt32
	if err := binary.Read(r, binary.BigEndian, &b.Header.Version); err != nil {
		return fmt.Errorf("read version failed: %w", err)
	}

	// GenerationVersion 是 NUL 结尾字符串，并为 EngineVersion 终止符和 FSHeader 预留空间
	// GenerationVersion is a NUL-terminated string read while reserving room for the EngineVersion terminator and FSHeader
	genVer, err := b.readHeaderNullString(r, 1+20)
	if err != nil {
		return fmt.Errorf("read generation version failed: %w", err)
	}
	b.Header.GenerationVersion = genVer

	// EngineVersion 是 NUL 结尾字符串
	// EngineVersion is a NUL-terminated string
	engVer, err := b.readHeaderNullString(r, 20)
	if err != nil {
		return fmt.Errorf("read engine version failed: %w", err)
	}
	b.Header.EngineVersion = engVer

	// FSHeader 是固定宽度的 Big-Endian 结构
	// FSHeader is a fixed-width Big-Endian structure
	if err := binary.Read(r, binary.BigEndian, &b.Header.FSHeader); err != nil {
		return fmt.Errorf("read fs header failed: %w", err)
	}

	return nil
}

// readBlockAndDirInfo 定位、读取、解压并解析 BlockAndDirInfo
// readBlockAndDirInfo locates, reads, decompresses, and parses BlockAndDirInfo
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

	// BlockAndDirInfo 默认紧随头部，也可由 FlagBlockAndDirAtEnd 指定在文件末尾
	// BlockAndDirInfo follows the header by default or is placed at file end by FlagBlockAndDirAtEnd
	infoOffset := b.headerEnd
	if flags&uint32(FlagBlockAndDirAtEnd) != 0 {
		// 末尾布局使用文件大小减去压缩元数据大小得到起始位置
		// The end layout derives the start from file size minus compressed metadata size
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

	// 读取 BlockAndDirInfo 的压缩或原始字节
	// Read the compressed or raw BlockAndDirInfo bytes
	compressedData := make([]byte, int(compressedSize))
	if _, err := io.ReadFull(r, compressedData); err != nil {
		return fmt.Errorf("read compressed block info failed: %w", err)
	}

	// 按 FSHeader 低六位指定的算法解压元数据
	// Decompress metadata using the algorithm selected by the low six FSHeader bits
	var infoData []byte
	switch compType {
	case CompressionNone:
		infoData = compressedData
	case CompressionLZ4, CompressionLZ4HC:
		infoData = make([]byte, int(decompressedSize))
		n, err := lz4.UncompressBlock(compressedData, infoData)
		if err != nil {
			return fmt.Errorf("LZ4 decompress block info failed: %w", err)
		}
		if n != len(infoData) {
			return fmt.Errorf("LZ4 block/directory info size mismatch: got %d, want %d", n, len(infoData))
		}
	case CompressionLZMA:
		decoded, err := decompressUnityLZMA(compressedData, int(decompressedSize))
		if err != nil {
			return fmt.Errorf("LZMA decompress block/directory info failed: %w", err)
		}
		infoData = decoded
	}

	// 解析块表与目录表并校验数据布局
	// Parse the block and directory tables and validate the data layout
	if err := b.parseBlockAndDirInfo(infoData); err != nil {
		return err
	}
	return b.validateDataLayout()
}

// parseBlockAndDirInfo 从解压后的 Big-Endian 字节中解析 BlockAndDirInfo
// parseBlockAndDirInfo parses BlockAndDirInfo from decompressed Big-Endian bytes
func (b *Aba) parseBlockAndDirInfo(data []byte) error {
	if len(data) < 24 {
		return fmt.Errorf("block/directory info is too short: %d bytes", len(data))
	}
	r := binaryio.NewEndianReader(data, binary.BigEndian)
	var parsed BlockAndDirInfo

	// 元数据首先保存 16 字节 Hash
	// Metadata begins with a 16-byte Hash
	if err := r.ReadFull(parsed.Hash[:]); err != nil {
		return fmt.Errorf("read hash: %w", err)
	}

	// Hash 后保存 BlockCount 和固定宽度的 BlockInfos
	// Hash is followed by BlockCount and fixed-width BlockInfos
	blockCountRaw, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read block count: %w", err)
	}
	if blockCountRaw < 0 {
		return fmt.Errorf("negative block count %d", blockCountRaw)
	}
	blockCount := int(blockCountRaw)
	if r.Remaining() < 4 || blockCount > (r.Remaining()-4)/10 {
		return fmt.Errorf("block count %d cannot fit in remaining metadata (%d bytes)", blockCount, r.Remaining())
	}

	parsed.BlockInfos = makeABACountedSliceForAppend[BlockInfo](blockCount)
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

	// 块表后保存 DirectoryCount 和变长名称的 DirectoryInfos
	// The block table is followed by DirectoryCount and DirectoryInfos with variable-length names
	dirCountRaw, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read directory count: %w", err)
	}
	if dirCountRaw < 0 {
		return fmt.Errorf("negative directory count %d", dirCountRaw)
	}
	dirCount := int(dirCountRaw)
	const minimumDirectoryInfoSize = 8 + 8 + 4 + 1
	if dirCount > r.Remaining()/minimumDirectoryInfoSize {
		return fmt.Errorf("directory count %d cannot fit in remaining metadata (%d bytes)", dirCount, r.Remaining())
	}

	parsed.DirectoryInfos = makeABACountedSliceForAppend[DirectoryInfo](dirCount)
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

		// 每个目录条目以 NUL 结尾的 Name 收尾
		// Each directory entry ends with a NUL-terminated Name
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

// readDataRange 从解压后的 UnityFS 数据流返回一个范围
// 仅读取和解压与请求范围重叠的块，因此提取多个文件时不会保留或反复分配完整 .aba 解压缓冲区
// readDataRange returns a range from the decompressed UnityFS data stream
// It reads and decompresses only overlapping blocks, so extracting many files does not retain or repeatedly allocate a full .aba decompression buffer
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

	result := make([]byte, int(size))
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
			compressed := make([]byte, int(block.CompressedSize))
			if _, err := readerAt.ReadAt(compressed, compressedOffset); err != nil {
				return nil, fmt.Errorf("read block[%d] data: %w", blockIndex, err)
			}

			blockData, err := decompressDataBlock(block, compressed)
			if err != nil {
				return nil, fmt.Errorf("decompress block[%d]: %w", blockIndex, err)
			}
			if len(blockData) != int(block.DecompressedSize) {
				return nil, fmt.Errorf("block[%d] decompressed size mismatch: got %d, want %d", blockIndex, len(blockData), block.DecompressedSize)
			}

			copyStart := maxInt64(offset, blockStart) - blockStart
			copyEnd := minInt64(end, blockEnd) - blockStart
			written += copy(result[written:], blockData[int(copyStart):int(copyEnd)])
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

// decompressDataBlock 按数据块 Flags 解压一个压缩块并校验输出长度
// decompressDataBlock decompresses one data block according to its Flags and validates the output length
func decompressDataBlock(block BlockInfo, compressed []byte) ([]byte, error) {
	if block.CompressedSize > maxAbaBlockSize || block.DecompressedSize > maxAbaBlockSize {
		return nil, fmt.Errorf("block is too large: compressed=%d decompressed=%d", block.CompressedSize, block.DecompressedSize)
	}
	if len(compressed) != int(block.CompressedSize) {
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
		dst := make([]byte, int(block.DecompressedSize))
		n, err := lz4.UncompressBlock(compressed, dst)
		if err != nil {
			return nil, err
		}
		if n != len(dst) {
			return nil, fmt.Errorf("LZ4 decompressed size mismatch: got %d, want %d", n, len(dst))
		}
		return dst, nil
	case CompressionLZMA:
		data, err := decompressUnityLZMA(compressed, int(block.DecompressedSize))
		if err != nil {
			return nil, fmt.Errorf("LZMA decompress: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unknown block compression type: %d", compType)
	}
}

// UnityFS 保存标准五字节 LZMA 属性头，但省略传统格式的八字节解压大小，因为该大小已在块表中给出
// 本函数插入已知大小，并将实际解码交给 xz/lzma 实现
// UnityFS stores the standard five-byte LZMA properties header but omits the classic eight-byte uncompressed-size field because that size is already present in the block table
// This function inserts the known size and delegates codec work to the xz/lzma implementation
func decompressUnityLZMA(compressed []byte, decompressedSize int) ([]byte, error) {
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

// minInt64 返回两个 Int64 中较小者
// minInt64 returns the smaller of two Int64 values
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// maxInt64 返回两个 Int64 中较大者
// maxInt64 returns the larger of two Int64 values
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// validateDataLayout 根据头部 flags 和块表计算并校验压缩数据区
// validateDataLayout computes and validates the compressed data area from header flags and the block table
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

// sumDecompressedBlockSizes 安全累加所有数据块的解压大小
// sumDecompressedBlockSizes safely sums decompressed sizes for all data blocks
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

// addNonNegativeInt64 加法仅接受非负值并检测 Int64 溢出
// addNonNegativeInt64 adds non-negative values and detects Int64 overflow
func addNonNegativeInt64(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || a > int64(^uint64(0)>>1)-b {
		return 0, false
	}
	return a + b, true
}

// alignInt64 将非负值向上对齐到指定正数边界
// alignInt64 rounds a non-negative value up to a positive alignment boundary
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

// readHeaderNullString 在保留后续头部字段空间的前提下读取一个 NUL 结尾字符串
// readHeaderNullString reads a NUL-terminated string while reserving space for remaining header fields
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

// readNullStringWithin 在给定最大字节数内读取 NUL 结尾字符串
// readNullStringWithin reads a NUL-terminated string within a given maximum byte count
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
		n, readErr := io.ReadFull(r, scratch[:int(chunkSize)])
		if terminator := bytes.IndexByte(scratch[:n], 0); terminator >= 0 {
			length := scanned + int64(terminator)
			if length > int64(int(^uint(0)>>1)) {
				return "", fmt.Errorf("null-terminated string length %d exceeds platform int capacity", length)
			}
			if _, err := r.Seek(start, io.SeekStart); err != nil {
				return "", err
			}
			data := make([]byte, int(length))
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
