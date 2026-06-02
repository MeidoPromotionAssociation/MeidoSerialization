package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

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
	if len(entries) == 0 {
		return fmt.Errorf("no entries to write")
	}

	if opts == nil {
		opts = &BundleWriteOptions{}
	}
	if opts.EngineVersion == "" {
		opts.EngineVersion = "2021.3.3f1"
	}
	if opts.GenerationVersion == "" {
		opts.GenerationVersion = "5.x.x"
	}
	if opts.Version == 0 {
		opts.Version = 7
	}

	// 1. 拼接所有文件数据，构建 DirectoryInfos
	var allData []byte
	dirInfos := make([]DirectoryInfo, len(entries))
	for i, entry := range entries {
		dirInfos[i] = DirectoryInfo{
			Offset:           int64(len(allData)),
			DecompressedSize: int64(len(entry.Data)),
			Name:             entry.Name,
		}
		if entry.IsSerialized {
			dirInfos[i].Flags = DirFlagSerializedFile
		}
		allData = append(allData, entry.Data...)
	}

	// 2. 构建数据块（可选 LZ4 压缩）
	const blockSize = 0x20000 // 128KB per block
	var blockInfos []BlockInfo
	var compressedData []byte

	if opts.Compress {
		for offset := 0; offset < len(allData); offset += blockSize {
			end := offset + blockSize
			if end > len(allData) {
				end = len(allData)
			}
			block := allData[offset:end]

			dst := make([]byte, lz4.CompressBlockBound(len(block)))
			n, err := lz4.CompressBlock(block, dst, nil)
			if err != nil || n == 0 || n >= len(block) {
				// 压缩无收益，存储原始数据
				compressedData = append(compressedData, block...)
				blockInfos = append(blockInfos, BlockInfo{
					DecompressedSize: uint32(len(block)),
					CompressedSize:   uint32(len(block)),
					Flags:            0x40, // not compressed, streamed
				})
			} else {
				compressedData = append(compressedData, dst[:n]...)
				blockInfos = append(blockInfos, BlockInfo{
					DecompressedSize: uint32(len(block)),
					CompressedSize:   uint32(n),
					Flags:            0x43, // LZ4HC + streamed
				})
			}
		}
	} else {
		// 不压缩：单个块
		compressedData = allData
		blockInfos = []BlockInfo{{
			DecompressedSize: uint32(len(allData)),
			CompressedSize:   uint32(len(allData)),
			Flags:            0x40, // not compressed, streamed
		}}
	}

	// 3. 序列化 BlockAndDirInfo
	blockAndDirBytes := serializeBlockAndDirInfo(blockInfos, dirInfos)

	// 4. LZ4 压缩 BlockAndDirInfo
	blockAndDirCompressed := make([]byte, lz4.CompressBlockBound(len(blockAndDirBytes)))
	n, err := lz4.CompressBlock(blockAndDirBytes, blockAndDirCompressed, nil)
	if err != nil || n == 0 || n >= len(blockAndDirBytes) {
		// 压缩无收益
		blockAndDirCompressed = blockAndDirBytes
		n = len(blockAndDirBytes)
	} else {
		blockAndDirCompressed = blockAndDirCompressed[:n]
	}

	// 5. 计算 header 大小和总文件大小
	headerSize := len(signatureUnityFS) + 1 + // signature + null
		4 + // version
		len(opts.GenerationVersion) + 1 + // gen version + null
		len(opts.EngineVersion) + 1 + // engine version + null
		20 // FSHeader (8+4+4+4)

	// version >= 7 需要 16 字节对齐
	alignedHeaderSize := headerSize
	if opts.Version >= 7 {
		alignedHeaderSize = (headerSize + 15) & ^15
	}

	totalFileSize := int64(alignedHeaderSize) + int64(n) + int64(len(compressedData))

	// 6. 确定 flags
	var flags uint32 = FlagHasDirectoryInfo
	if n < len(blockAndDirBytes) {
		flags |= CompressionLZ4HC // BlockAndDirInfo 使用 LZ4 压缩
	}

	// 7. 写入 Header
	var buf bytes.Buffer

	// Signature (null-terminated)
	buf.WriteString(signatureUnityFS)
	buf.WriteByte(0)

	// Version
	binary.Write(&buf, binary.BigEndian, opts.Version)

	// GenerationVersion (null-terminated)
	buf.WriteString(opts.GenerationVersion)
	buf.WriteByte(0)

	// EngineVersion (null-terminated)
	buf.WriteString(opts.EngineVersion)
	buf.WriteByte(0)

	// FSHeader
	binary.Write(&buf, binary.BigEndian, totalFileSize)
	binary.Write(&buf, binary.BigEndian, uint32(n))
	binary.Write(&buf, binary.BigEndian, uint32(len(blockAndDirBytes)))
	binary.Write(&buf, binary.BigEndian, flags)

	// Align to 16 bytes (version >= 7)
	if opts.Version >= 7 {
		for buf.Len() < alignedHeaderSize {
			buf.WriteByte(0)
		}
	}

	// 8. 写入 BlockAndDirInfo
	buf.Write(blockAndDirCompressed)

	// 9. 写入数据块
	buf.Write(compressedData)

	// 10. 输出
	_, err = w.Write(buf.Bytes())
	return err
}

// serializeBlockAndDirInfo 将 BlockInfos 和 DirectoryInfos 序列化为二进制格式
func serializeBlockAndDirInfo(blocks []BlockInfo, dirs []DirectoryInfo) []byte {
	var buf bytes.Buffer

	// Hash (16 bytes, all zeros)
	buf.Write(make([]byte, 16))

	// BlockCount + BlockInfos
	binary.Write(&buf, binary.BigEndian, int32(len(blocks)))
	for _, b := range blocks {
		binary.Write(&buf, binary.BigEndian, b.DecompressedSize)
		binary.Write(&buf, binary.BigEndian, b.CompressedSize)
		binary.Write(&buf, binary.BigEndian, b.Flags)
	}

	// DirectoryCount + DirectoryInfos
	binary.Write(&buf, binary.BigEndian, int32(len(dirs)))
	for _, d := range dirs {
		binary.Write(&buf, binary.BigEndian, d.Offset)
		binary.Write(&buf, binary.BigEndian, d.DecompressedSize)
		binary.Write(&buf, binary.BigEndian, d.Flags)
		buf.WriteString(d.Name)
		buf.WriteByte(0)
	}

	return buf.Bytes()
}
