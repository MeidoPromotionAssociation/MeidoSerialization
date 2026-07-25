package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// .assets 与 Unity SerializedFile
// AssetBundle 内部的实际资源容器，包含类型树、对象元数据和资源数据
// 文件头使用 Big-Endian，后续字段的字节序由 Endianness 指定
// 结构如下
//
//	[Header]
//	  - MetadataSize: uint32（元数据块大小，不含 header）
//	  - FileSize: uint32（整个文件大小，v22+ 为 int64）
//	  - Version: uint32（序列化格式版本）
//	  - DataOffset: uint32（第一个资源数据的偏移，v22+ 为 int64）
//	  - Endianness: byte（0=Little-Endian, 1=Big-Endian）+ 3 padding
//	  - (v22+: MetadataSize uint32, FileSize int64, DataOffset int64, 8-byte field of unknown purpose)
//	[Metadata]（按 Endianness 编码）
//	  - UnityVersion: null-terminated string
//	  - TargetPlatform: uint32
//	  - TypeTreeEnabled: bool
//	  - TypeTreeTypes[]: 类型树定义
//	  - AssetInfos[]: 资源信息列表
//	  - ExternalFiles[]: 外部引用
//	  - RefTypes[]: 引用类型（v21+）
//	  - UserInformation: string
//
// .assets and Unity SerializedFile
// The actual resource container inside an AssetBundle, holding type trees, object metadata, and asset data
// The header is Big-Endian; Endianness selects the byte order of subsequent fields
// Its layout is:
//
//	[Header]
//	  - MetadataSize: uint32 (metadata size excluding the header)
//	  - FileSize: uint32, or int64 for version 22 and later
//	  - Version: uint32 serialized-file format version
//	  - DataOffset: uint32, or int64 for version 22 and later
//	  - Endianness: byte (0=Little-Endian, 1=Big-Endian) plus three padding bytes
//	[Metadata] (encoded according to Endianness)
//	  - UnityVersion, target platform, type-tree flag and definitions
//	  - AssetInfos, external references, version-21+ reference types, and user information

// AssetsFile 表示一个已解析的 Unity SerializedFile / AssetsFile represents one parsed Unity SerializedFile
type AssetsFile struct {
	Header    AssetsFileHeader        // 文件头 / File header
	Metadata  AssetsMetadata          // 包含类型树和资源列表的元数据 / Metadata including type trees and asset list
	Data      []byte                  // 用于按偏移读取资源的原始文件数据 / Raw file bytes used to read assets by offset
	readRange AssetsFileRangeResolver // 未整体载入文件时使用的范围读取器 / Range reader used when the complete file is not loaded
}

// AssetsFileRangeResolver 从 SerializedFile 的绝对偏移读取精确字节范围
// AssetsFileRangeResolver reads an exact byte range at an absolute SerializedFile offset
type AssetsFileRangeResolver func(offset int64, size int64) ([]byte, error)

// AssetsFileHeader 表示 Unity 序列化文件头 / AssetsFileHeader represents a Unity serialized file header
type AssetsFileHeader struct {
	MetadataSize       uint32  // 元数据块大小 / Metadata block size
	FileSize           int64   // 整个文件大小 / Total file size
	Version            uint32  // 序列化格式版本，当前支持 12 至 22 / Serialized file format version, currently supporting 12 through 22
	DataOffset         int64   // 资源数据区起始偏移 / Start offset of the asset data area
	Endianness         bool    // true 表示 Big-Endian，false 表示 Little-Endian / true means Big-Endian and false means Little-Endian
	LegacyMetadataSize uint32  // 固定头部中的 32 位元数据大小，v22 起通常为零并由扩展头替代 / 32-bit metadata size in the fixed header, normally zero and superseded by the extended header from v22
	LegacyFileSize     uint32  // 固定头部中的 32 位文件大小，v22 起通常为零并由扩展头替代 / 32-bit file size in the fixed header, normally zero and superseded by the extended header from v22
	LegacyDataOffset   uint32  // 固定头部中的 32 位数据偏移，v22 起通常为零并由扩展头替代 / 32-bit data offset in the fixed header, normally zero and superseded by the extended header from v22
	Reserved           [3]byte // Endianness 后由 Unity 读取并跳过的三个保留字节 / Three reserved bytes Unity reads and skips after Endianness
	LargeFilesUnknown  int64   // v22 扩展头末尾用途未知的 Int64 字段 / Int64 field of unknown purpose at the end of the v22 extended header
}

// AssetsMetadata 包含类型树、资源信息和 metadata 尾部 / AssetsMetadata contains type trees, asset metadata, and the metadata tail
type AssetsMetadata struct {
	UnityVersion    string                            // Unity 版本字符串 / Unity version string
	TargetPlatform  uint32                            // 目标平台 ID / Target platform ID
	TypeTreeEnabled bool                              // 是否包含类型树 / Whether type tree data is present
	TypeTreeTypes   []TypeTreeType                    // 类型树定义列表 / Type tree definition list
	BigIDEnabled    int32                             // v7-v13 的 64 位 PathID 开关；保留原始 int32 wire 值 / v7-v13 64-bit PathID switch; preserves the original int32 wire value
	AssetInfos      []AssetInfo                       // 资源信息列表 / Asset metadata list
	ScriptTypes     []LocalSerializedObjectIdentifier // 本地脚本对象引用 / Local script object identifiers
	ExternalFiles   []ExternalFile                    // 外部文件引用 / External file references
	RefTypes        []TypeTreeType                    // 引用类型定义 / Referenced serialized type definitions
	UserInformation string                            // metadata 尾部用户信息 / User-information string at the metadata tail
}

// TypeTreeType 表示一个 Unity 类型的 TypeTree 定义 / TypeTreeType represents the TypeTree definition for one Unity type
type TypeTreeType struct {
	TypeId           int32          // 类型 ID，如 28=Texture2D、49=TextAsset / Class ID such as 28=Texture2D and 49=TextAsset
	IsStrippedType   bool           // 是否被剥离 / Whether this type is stripped
	ScriptTypeIndex  int16          // 脚本类型索引，-1 表示无脚本 / Script type index, with -1 meaning no script
	ScriptIdHash     [16]byte       // v13+ 脚本类型按条件保存的脚本 ID 哈希 / Conditionally stored script ID hash for script types in v13+
	TypeHash         [16]byte       // 类型哈希 / Type hash
	Nodes            []TypeTreeNode // 仅在 TypeTreeEnabled 为 true 时保存的类型树节点 / Type-tree nodes stored only when TypeTreeEnabled is true
	StringBuffer     []byte         // 字符串缓冲区 / String buffer
	TypeDependencies []int32        // v21+ 且启用 TypeTree 时的普通类型依赖 / Ordinary type dependencies for v21+ with TypeTree enabled
	ClassName        string         // 引用类型类名 / Referenced type class name
	Namespace        string         // 引用类型命名空间 / Referenced type namespace
	AssemblyName     string         // 引用类型程序集名 / Referenced type assembly name
}

// TypeTreeNode 表示 Unity TypeTree 中的一个节点 / TypeTreeNode represents one node in a Unity TypeTree
type TypeTreeNode struct {
	Version     uint16 // 节点版本 / Node version
	Level       byte   // 层级深度 / Tree depth level
	TypeFlags   byte   // 类型标志，0x01 表示 IsArray / Type flags, with 0x01 meaning IsArray
	TypeStrOff  uint32 // 类型名在字符串缓冲区中的偏移 / Offset of the type name in the string buffer
	NameStrOff  uint32 // 字段名在字符串缓冲区中的偏移 / Offset of the field name in the string buffer
	ByteSize    int32  // 字段字节大小，-1 表示可变长度 / Field byte size, with -1 meaning variable length
	Index       int32  // 在父节点中的索引 / Index within the parent node
	MetaFlags   uint32 // 元标志，0x4000 表示 AlignBytes / Metadata flags, with 0x4000 meaning AlignBytes
	RefTypeHash uint64 // v19+ 引用类型哈希 / Referenced-type hash in v19+
}

// LocalSerializedObjectIdentifier 标识 metadata 中的本地脚本对象 / LocalSerializedObjectIdentifier identifies a local script object in metadata
type LocalSerializedObjectIdentifier struct {
	LocalSerializedFileIndex int32 // 本地序列化文件索引 / Local serialized-file index
	LocalIdentifierInFile    int64 // 文件内对象 ID / Local object identifier in the file
}

// AssetInfo 表示单个序列化对象的元信息 / AssetInfo represents metadata for one serialized asset object
type AssetInfo struct {
	PathId          int64  // 文件内唯一标识资源的路径 ID / Asset PathID unique within the file
	ByteOffset      int64  // 相对于 DataOffset 的偏移 / Offset relative to DataOffset
	ByteSize        uint32 // 资源数据大小 / Asset data size
	TypeIdOrIndex   int32  // Serialized type 标识；v16+ 必须是 TypeTreeTypes 数组索引 / Serialized type identifier; v16+ must index TypeTreeTypes
	TypeId          int32  // 解析后填充的实际类型 ID / Actual class ID filled after parsing
	LegacyClassID   uint16 // v12 至 v15 对象项中显式保存的 class ID / Class ID stored explicitly in v12 through v15 object entries
	ScriptTypeIndex int16  // v12 至 v16 对象项中的脚本类型索引，-1 表示无脚本 / Script type index in v12 through v16 object entries, with -1 meaning no script
	Stripped        byte   // v15 至 v16 对象项中的 stripped 标记 / Stripped marker in v15 through v16 object entries
}

// ExternalFile 表示 SerializedFile 的外部文件引用 / ExternalFile represents an external file reference from a SerializedFile
type ExternalFile struct {
	AssetPath string   // 缓存资源虚拟路径 / Virtual cached-asset path
	Guid      [16]byte // 外部文件 GUID / External file GUID
	Type      int32    // 引用类型 / Reference type
	PathName  string   // 路径名 / Path name
}

// ReadAssetsFile 从字节数据中解析 Unity SerializedFile，并校验头部、metadata 边界和对象范围
// ReadAssetsFile parses a Unity SerializedFile from bytes and validates its header, metadata bounds, and object ranges
func ReadAssetsFile(data []byte) (*AssetsFile, error) {
	resolver := func(offset int64, size int64) ([]byte, error) {
		end, ok := addNonNegativeInt64(offset, size)
		if !ok || offset < 0 || end > int64(len(data)) {
			return nil, fmt.Errorf("serialized range [%d,%d) exceeds %d bytes", offset, end, len(data))
		}
		return data[offset:end], nil
	}
	return readAssetsFileFromRange(int64(len(data)), resolver, data)
}

// ReadAssetsFileRange 通过范围读取器解析 Unity SerializedFile，仅在访问对象时读取对象数据
// ReadAssetsFileRange parses a Unity SerializedFile through a range reader and reads object data only when accessed
func ReadAssetsFileRange(fileSize int64, resolver AssetsFileRangeResolver) (*AssetsFile, error) {
	return readAssetsFileFromRange(fileSize, resolver, nil)
}

// readAssetsFileFromRange 读取文件头和 metadata 并建立可延迟读取对象的 AssetsFile
// readAssetsFileFromRange reads the header and metadata and builds an AssetsFile that can load objects lazily
func readAssetsFileFromRange(fileSize int64, resolver AssetsFileRangeResolver, data []byte) (*AssetsFile, error) {
	if resolver == nil {
		return nil, fmt.Errorf("nil SerializedFile range resolver")
	}
	if fileSize < 20 {
		return nil, fmt.Errorf("data too short for assets file header: %d bytes", fileSize)
	}
	probeSize := fileSize
	if probeSize > 48 {
		probeSize = 48
	}
	headerData, err := readAssetsFileRangeExact(resolver, 0, probeSize)
	if err != nil {
		return nil, fmt.Errorf("read assets file header: %w", err)
	}
	header, headerSize, err := parseAssetsFileHeader(headerData)
	if err != nil {
		return nil, err
	}
	if header.FileSize != fileSize {
		return nil, fmt.Errorf("serialized file size %d does not match input length %d", header.FileSize, fileSize)
	}
	if header.DataOffset < headerSize || header.DataOffset > header.FileSize {
		return nil, fmt.Errorf("invalid serialized data offset %d for file size %d", header.DataOffset, header.FileSize)
	}
	metadataStart := headerSize
	metadataEnd, ok := addNonNegativeInt64(metadataStart, int64(header.MetadataSize))
	if !ok || metadataEnd > header.FileSize {
		return nil, fmt.Errorf("serialized metadata range [%d, %d) exceeds declared file size %d", metadataStart, metadataEnd, header.FileSize)
	}
	if metadataEnd > header.DataOffset {
		return nil, fmt.Errorf("serialized metadata end %d exceeds data offset %d", metadataEnd, header.DataOffset)
	}
	metadata, err := readAssetsFileRangeExact(resolver, metadataStart, int64(header.MetadataSize))
	if err != nil {
		return nil, fmt.Errorf("read serialized metadata: %w", err)
	}
	af := &AssetsFile{Header: header, Data: data, readRange: resolver}

	// 根据 Endianness 确定 metadata 和对象字段的字节序
	// Select metadata and object byte order from Endianness
	var order binary.ByteOrder
	if af.Header.Endianness {
		order = binary.BigEndian
	} else {
		order = binary.LittleEndian
	}

	// 在限定的 metadata 切片中解析，伪造的内部计数或字符串无法越过对齐填充和对象数据
	// Parse a bounded metadata slice so forged inner counts or strings cannot consume alignment padding or object bytes
	if err := af.readMetadata(metadata, 0, order); err != nil {
		return nil, fmt.Errorf("read metadata failed: %w", err)
	}
	if err := af.validateAssetInfos(); err != nil {
		return nil, fmt.Errorf("validate asset infos failed: %w", err)
	}
	return af, nil
}

// parseAssetsFileHeader 解析 Big-Endian 固定头部及 v22 扩展头部并返回实际头部长度
// parseAssetsFileHeader parses the Big-Endian fixed header and v22 extended header and returns the actual header length
func parseAssetsFileHeader(data []byte) (AssetsFileHeader, int64, error) {
	var header AssetsFileHeader
	if len(data) < 20 {
		return header, 0, fmt.Errorf("data too short for assets file header: %d bytes", len(data))
	}

	// 首先以 Big-Endian 读取固定头部
	// First read the fixed header in Big-Endian order
	headerReader := binaryio.NewEndianReader(data, binary.BigEndian)
	metadataSize, err := headerReader.ReadUInt32()
	if err != nil {
		return header, 0, fmt.Errorf("read metadata size failed: %w", err)
	}
	fileSize, err := headerReader.ReadUInt32()
	if err != nil {
		return header, 0, fmt.Errorf("read file size failed: %w", err)
	}
	version, err := headerReader.ReadUInt32()
	if err != nil {
		return header, 0, fmt.Errorf("read version failed: %w", err)
	}
	dataOffset, err := headerReader.ReadUInt32()
	if err != nil {
		return header, 0, fmt.Errorf("read data offset failed: %w", err)
	}
	endianness, err := headerReader.ReadByte()
	if err != nil {
		return header, 0, fmt.Errorf("read endianness failed: %w", err)
	}
	var headerPadding [3]byte
	if err := headerReader.ReadFull(headerPadding[:]); err != nil {
		return header, 0, fmt.Errorf("read header padding failed: %w", err)
	}

	header.MetadataSize = metadataSize
	header.FileSize = int64(fileSize)
	header.Version = version
	header.DataOffset = int64(dataOffset)
	header.LegacyMetadataSize = metadataSize
	header.LegacyFileSize = fileSize
	header.LegacyDataOffset = dataOffset
	header.Reserved = headerPadding
	if endianness > 1 {
		return header, 0, fmt.Errorf("invalid serialized file endianness byte %d", endianness)
	}
	header.Endianness = endianness == 1
	if header.Version < 12 || header.Version > 22 {
		return header, 0, fmt.Errorf("unsupported serialized file version %d (supported: 12-22)", header.Version)
	}

	// v22 及以上追加扩展头部字段
	// Version 22 and later append extended header fields
	if header.Version >= 22 {
		header.MetadataSize, err = headerReader.ReadUInt32()
		if err != nil {
			return header, 0, fmt.Errorf("read extended metadata size failed: %w", err)
		}
		fileSize64, err := headerReader.ReadInt64()
		if err != nil {
			return header, 0, fmt.Errorf("read extended file size failed: %w", err)
		}
		dataOffset64, err := headerReader.ReadInt64()
		if err != nil {
			return header, 0, fmt.Errorf("read extended data offset failed: %w", err)
		}
		header.LargeFilesUnknown, err = headerReader.ReadInt64()
		if err != nil {
			return header, 0, fmt.Errorf("read extended header unknown field failed: %w", err)
		}
		if fileSize64 < 0 {
			return header, 0, fmt.Errorf("extended serialized file size %d is negative", fileSize64)
		}
		if dataOffset64 < 0 {
			return header, 0, fmt.Errorf("extended serialized data offset %d is negative", dataOffset64)
		}
		header.FileSize = fileSize64
		header.DataOffset = dataOffset64
	}
	return header, headerReader.Pos(), nil
}

// readAssetsFileRangeExact 调用范围读取器并拒绝短读或超长结果
// readAssetsFileRangeExact calls a range reader and rejects short or oversized results
func readAssetsFileRangeExact(resolver AssetsFileRangeResolver, offset int64, size int64) ([]byte, error) {
	if offset < 0 || size < 0 {
		return nil, fmt.Errorf("negative serialized range offset=%d size=%d", offset, size)
	}
	data, err := resolver(offset, size)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != size {
		return nil, fmt.Errorf("serialized range offset=%d returned %d bytes, want %d", offset, len(data), size)
	}
	return data, nil
}

// GetAssetData 按 DataOffset、ByteOffset 和 ByteSize 提取指定资源的原始数据
// GetAssetData extracts raw data for an asset using DataOffset, ByteOffset, and ByteSize
func (af *AssetsFile) GetAssetData(info *AssetInfo) ([]byte, error) {
	if af == nil || info == nil {
		return nil, fmt.Errorf("nil assets file or asset info")
	}
	dataLen := af.Header.FileSize
	if dataLen == 0 {
		dataLen = int64(len(af.Data))
	}
	if af.Data != nil {
		if dataLen < 0 || dataLen > int64(len(af.Data)) {
			return nil, fmt.Errorf("declared file size %d is invalid for %d data bytes", af.Header.FileSize, len(af.Data))
		}
	} else if af.readRange == nil {
		return nil, fmt.Errorf("assets file has neither complete data nor a range resolver")
	}
	if af.Header.DataOffset < 0 || af.Header.DataOffset > dataLen || info.ByteOffset < 0 || info.ByteOffset > dataLen-af.Header.DataOffset {
		return nil, fmt.Errorf("asset data start out of bounds: dataOffset=%d byteOffset=%d in %d bytes", af.Header.DataOffset, info.ByteOffset, dataLen)
	}
	start := af.Header.DataOffset + info.ByteOffset
	size := int64(info.ByteSize)
	if size > dataLen-start {
		return nil, fmt.Errorf("asset data out of bounds: start=%d size=%d in %d bytes", start, size, dataLen)
	}
	end := start + size
	if af.Data != nil {
		return af.Data[start:end], nil
	}
	data, err := readAssetsFileRangeExact(af.readRange, start, size)
	if err != nil {
		return nil, fmt.Errorf("read asset PathID %d range [%d,%d): %w", info.PathId, start, end, err)
	}
	return data, nil
}

// GetAssetsByType 返回指定 Unity class ID 的所有资源元信息
// GetAssetsByType returns metadata for all assets with the specified Unity class ID
func (af *AssetsFile) GetAssetsByType(typeId int32) []AssetInfo {
	var result []AssetInfo
	for _, info := range af.Metadata.AssetInfos {
		if info.TypeId == typeId {
			result = append(result, info)
		}
	}
	return result
}

// GetAssetInfoByPathID 按 PathID 查找资源元信息
// GetAssetInfoByPathID finds asset metadata by PathID
func (af *AssetsFile) GetAssetInfoByPathID(pathID int64) *AssetInfo {
	for i := range af.Metadata.AssetInfos {
		if af.Metadata.AssetInfos[i].PathId == pathID {
			return &af.Metadata.AssetInfos[i]
		}
	}
	return nil
}

// readMetadata 按 SerializedFile 版本读取 metadata 主体和尾部表
// readMetadata reads the metadata body and tail tables according to the SerializedFile version
func (af *AssetsFile) readMetadata(data []byte, pos int64, order binary.ByteOrder) error {
	if pos < 0 || pos > int64(len(data)) {
		return fmt.Errorf("metadata position %d is outside %d bytes", pos, len(data))
	}
	r := binaryio.NewEndianReaderAt(data, pos, order)

	// 首先读取 NUL 结尾的 UnityVersion
	// First read the NUL-terminated UnityVersion
	ver, err := r.ReadNullString()
	if err != nil {
		return fmt.Errorf("read unity version failed: %w", err)
	}
	af.Metadata.UnityVersion = ver

	// 随后读取 TargetPlatform
	// Then read TargetPlatform
	af.Metadata.TargetPlatform, err = r.ReadUInt32()
	if err != nil {
		return fmt.Errorf("read target platform failed: %w", err)
	}

	// v13 以前 TypeTree 始终存在，metadata 中没有 TypeTreeEnabled 字节
	// Before format 13 the TypeTree is always present and metadata has no TypeTreeEnabled byte
	if af.Header.Version >= 13 {
		b, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read type tree enabled failed: %w", err)
		}
		af.Metadata.TypeTreeEnabled = b != 0
	} else {
		af.Metadata.TypeTreeEnabled = true
	}

	// 读取 TypeTreeTypes 数组
	// Read the TypeTreeTypes array
	typeCount, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read type count failed: %w", err)
	}
	if err := validateMetadataCount("type", typeCount, r.Remaining(), minimumSerializedTypeSize(af.Header.Version, af.Metadata.TypeTreeEnabled, false)); err != nil {
		return err
	}
	af.Metadata.TypeTreeTypes = makeABACountedSliceForAppend[TypeTreeType](int64(typeCount))
	for i := int64(0); i < int64(typeCount); i++ {
		var tt TypeTreeType
		if err := af.readTypeTreeType(r, &tt, false); err != nil {
			return fmt.Errorf("read type tree type[%d] failed: %w", i, err)
		}
		af.Metadata.TypeTreeTypes = append(af.Metadata.TypeTreeTypes, tt)
	}

	// Unity 格式 7 至 13 在 SerializedTypes 与对象数量之间保存 BigIDEnabled
	// 非零值使旧 PathID 从 Int32 变为 Int64，格式 14 起 Int64 PathID 固定存在
	// Unity formats 7 through 13 store BigIDEnabled between SerializedTypes and the object count
	// A non-zero value changes legacy PathIDs from Int32 to Int64; format 14 makes Int64 PathIDs unconditional
	if af.Header.Version >= 7 && af.Header.Version < 14 {
		af.Metadata.BigIDEnabled, err = r.ReadInt32()
		if err != nil {
			return fmt.Errorf("read big ID enabled failed: %w", err)
		}
	}

	// 读取 AssetInfos 数组
	// Read the AssetInfos array
	assetCount, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read asset count failed: %w", err)
	}
	if err := validateMetadataCount("asset", assetCount, r.Remaining(), minimumAssetInfoSize(af.Header.Version, af.Metadata.BigIDEnabled != 0)); err != nil {
		return err
	}
	af.Metadata.AssetInfos = makeABACountedSliceForAppend[AssetInfo](int64(assetCount))
	for i := int64(0); i < int64(assetCount); i++ {
		var info AssetInfo
		if err := af.readAssetInfo(r, &info); err != nil {
			return fmt.Errorf("read asset info[%d] failed: %w", i, err)
		}
		af.Metadata.AssetInfos = append(af.Metadata.AssetInfos, info)
	}

	// 支持的 Unity 格式在外部引用前包含 LocalSerializedObjectIdentifier 脚本类型数组
	// Supported Unity formats place a LocalSerializedObjectIdentifier script-type array before external references
	tail, err := af.readMetadataTail(data, r.Pos(), order)
	if err != nil {
		return err
	}
	af.Metadata.ScriptTypes = tail.ScriptTypes
	af.Metadata.ExternalFiles = tail.ExternalFiles
	af.Metadata.RefTypes = tail.RefTypes
	af.Metadata.UserInformation = tail.UserInformation
	return nil
}

// assetsMetadataTail 保存 metadata 尾部各已知表 / assetsMetadataTail stores the known metadata tail tables
type assetsMetadataTail struct {
	ScriptTypes     []LocalSerializedObjectIdentifier // 本地脚本对象标识数组 / Local script-object identifiers
	ExternalFiles   []ExternalFile                    // 外部文件引用数组 / External file references
	RefTypes        []TypeTreeType                    // v20+ 引用类型数组 / Referenced type array for v20+
	UserInformation string                            // 尾部 NUL 结尾用户信息 / NUL-terminated user information at the tail
}

// readMetadataTail 读取脚本类型、外部文件、引用类型和 UserInformation 尾部
// readMetadataTail reads the script types, external files, reference types, and UserInformation tail
func (af *AssetsFile) readMetadataTail(data []byte, pos int64, order binary.ByteOrder) (assetsMetadataTail, error) {
	var tail assetsMetadataTail
	if pos < 0 || pos > int64(len(data)) {
		return tail, fmt.Errorf("metadata tail position %d is outside %d bytes", pos, len(data))
	}
	r := binaryio.NewEndianReaderAt(data, pos, order)
	scriptCount, err := r.ReadInt32()
	if err != nil {
		return tail, fmt.Errorf("read script type count failed: %w", err)
	}
	entrySize := int64(8)
	if af.Header.Version >= 14 {
		entrySize = 12
	}
	if err := validateMetadataCount("script type", scriptCount, r.Remaining(), entrySize); err != nil {
		return tail, err
	}
	tail.ScriptTypes = makeABACountedSliceForAppend[LocalSerializedObjectIdentifier](int64(scriptCount))
	for i := int64(0); i < int64(scriptCount); i++ {
		identifier, err := af.readLocalSerializedObjectIdentifier(r)
		if err != nil {
			return tail, fmt.Errorf("read script type[%d]: %w", i, err)
		}
		tail.ScriptTypes = append(tail.ScriptTypes, identifier)
	}

	extCount, err := r.ReadInt32()
	if err != nil {
		return tail, fmt.Errorf("read external count failed: %w", err)
	}
	if err := validateMetadataCount("external file", extCount, r.Remaining(), 22); err != nil {
		return tail, err
	}
	externals := makeABACountedSliceForAppend[ExternalFile](int64(extCount))
	for i := int64(0); i < int64(extCount); i++ {
		var external ExternalFile
		if err := af.readExternalFile(r, &external); err != nil {
			return tail, fmt.Errorf("read external file[%d] failed: %w", i, err)
		}
		externals = append(externals, external)
	}

	tail.ExternalFiles = externals

	if af.Header.Version >= 20 {
		refTypeCount, err := r.ReadInt32()
		if err != nil {
			return tail, fmt.Errorf("read reference type count failed: %w", err)
		}
		if r.Remaining() < 1 {
			return tail, fmt.Errorf("metadata has no room for UserInformation after %d reference types", refTypeCount)
		}
		minimumSize := minimumSerializedTypeSize(af.Header.Version, af.Metadata.TypeTreeEnabled, true)
		if err := validateMetadataCount("reference type", refTypeCount, r.Remaining()-1, minimumSize); err != nil {
			return tail, err
		}
		tail.RefTypes = makeABACountedSliceForAppend[TypeTreeType](int64(refTypeCount))
		for i := int64(0); i < int64(refTypeCount); i++ {
			var refType TypeTreeType
			if err := af.readTypeTreeType(r, &refType, true); err != nil {
				return tail, fmt.Errorf("read reference type[%d]: %w", i, err)
			}
			tail.RefTypes = append(tail.RefTypes, refType)
		}
	}

	tail.UserInformation, err = r.ReadNullString()
	if err != nil {
		return tail, fmt.Errorf("read UserInformation: %w", err)
	}
	if r.Remaining() != 0 {
		return tail, fmt.Errorf("metadata has %d bytes of trailing data after UserInformation", r.Remaining())
	}
	return tail, nil
}

// readTypeTreeType 按格式版本读取一个 SerializedType 或引用类型定义
// readTypeTreeType reads one SerializedType or referenced-type definition according to the format version
func (af *AssetsFile) readTypeTreeType(r *binaryio.EndianReader, tt *TypeTreeType, isRefType bool) error {
	var err error
	v := af.Header.Version
	tt.ScriptTypeIndex = -1

	// 类型定义首先保存 TypeId
	// The type definition begins with TypeId
	typeId, err := r.ReadInt32()
	if err != nil {
		return err
	}
	tt.TypeId = typeId

	// IsStrippedType 自格式 16 起存在
	// IsStrippedType was added in format 16
	if v >= 16 {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		tt.IsStrippedType = b != 0
	}

	// ScriptTypeIndex 自格式 17 起存在
	// ScriptTypeIndex is present in format 17 and later
	if v >= 17 {
		idx, err := r.ReadInt16()
		if err != nil {
			return err
		}
		tt.ScriptTypeIndex = idx
	}

	// ScriptIdHash 自格式 13 起按脚本类型条件存在，v16 以前脚本类型使用负 class ID，v16 起 MonoBehaviour 使用 class ID 114
	// ScriptIdHash is conditionally present for script types from format 13; before v16 script types use a negative class ID, while v16 and later identify MonoBehaviour with class ID 114
	if v >= 13 {
		if (isRefType && tt.ScriptTypeIndex >= 0) || (v < 16 && typeId < 0) || (v >= 16 && typeId == 114) {
			if err := r.ReadFull(tt.ScriptIdHash[:]); err != nil {
				return err
			}
		}
	}

	// TypeHash 自格式 13 起固定存在
	// TypeHash is always present from format 13
	if v >= 13 {
		if err := r.ReadFull(tt.TypeHash[:]); err != nil {
			return err
		}
	}

	// 仅在 TypeTreeEnabled 时读取 TypeTree 节点
	// Read TypeTree nodes only when TypeTreeEnabled
	if af.Metadata.TypeTreeEnabled {
		if v >= 12 {
			// v12+ blob 依次保存 nodeCount、stringBufferSize、nodes 和 stringBuffer
			// The v12+ blob stores nodeCount, stringBufferSize, nodes, and stringBuffer in order
			nodeCount, err := r.ReadInt32()
			if err != nil {
				return err
			}
			strBufSize, err := r.ReadInt32()
			if err != nil {
				return err
			}

			if nodeCount < 0 {
				return fmt.Errorf("negative type tree node count %d", nodeCount)
			}
			nodeSize := int64(24)
			if v >= 19 {
				nodeSize = 32
			}
			if strBufSize < 0 {
				return fmt.Errorf("negative type tree string buffer size %d", strBufSize)
			}
			nodeBytes := int64(nodeCount) * nodeSize
			required := nodeBytes + int64(strBufSize)
			if nodeBytes < 0 || required < nodeBytes || required > r.Remaining() {
				return fmt.Errorf("type tree nodes/string buffer require %d bytes but only %d metadata bytes remain", required, r.Remaining())
			}

			tt.Nodes = makeABACountedSliceForAppend[TypeTreeNode](int64(nodeCount))
			for i := int64(0); i < int64(nodeCount); i++ {
				var node TypeTreeNode
				if err := af.readTypeTreeNodeBlob(r, &node); err != nil {
					return fmt.Errorf("read node[%d]: %w", i, err)
				}
				tt.Nodes = append(tt.Nodes, node)
			}

			if int64(strBufSize) > r.Remaining() {
				return fmt.Errorf("type tree string buffer size %d exceeds remaining metadata %d", strBufSize, r.Remaining())
			}
			tt.StringBuffer = make([]byte, int64(strBufSize))
			if err := r.ReadFull(tt.StringBuffer); err != nil {
				return err
			}
			if err := validateTypeTreeStringOffsets(tt); err != nil {
				return err
			}
		}
	}

	if af.Metadata.TypeTreeEnabled && v >= 21 {
		if isRefType {
			if tt.ClassName, err = r.ReadNullString(); err != nil {
				return fmt.Errorf("read reference type class name: %w", err)
			}
			if tt.Namespace, err = r.ReadNullString(); err != nil {
				return fmt.Errorf("read reference type namespace: %w", err)
			}
			if tt.AssemblyName, err = r.ReadNullString(); err != nil {
				return fmt.Errorf("read reference type assembly name: %w", err)
			}
		} else {
			depCount, err := r.ReadInt32()
			if err != nil {
				return err
			}
			if depCount < 0 || int64(depCount) > r.Remaining()/4 {
				return fmt.Errorf("invalid type dependency count %d with %d bytes remaining", depCount, r.Remaining())
			}
			tt.TypeDependencies = makeABACountedSliceForAppend[int32](int64(depCount))
			for i := int64(0); i < int64(depCount); i++ {
				dependency, err := r.ReadInt32()
				if err != nil {
					return fmt.Errorf("read type dependency[%d]: %w", i, err)
				}
				tt.TypeDependencies = append(tt.TypeDependencies, dependency)
			}
		}
	}

	return nil
}

// readTypeTreeNodeBlob 读取固定宽度 blob 格式的 TypeTree 节点
// readTypeTreeNodeBlob reads one fixed-width blob-format TypeTree node
func (af *AssetsFile) readTypeTreeNodeBlob(r *binaryio.EndianReader, node *TypeTreeNode) error {
	var err error
	node.Version, err = r.ReadUInt16()
	if err != nil {
		return err
	}
	node.Level, err = r.ReadByte()
	if err != nil {
		return err
	}
	node.TypeFlags, err = r.ReadByte()
	if err != nil {
		return err
	}
	node.TypeStrOff, err = r.ReadUInt32()
	if err != nil {
		return err
	}
	node.NameStrOff, err = r.ReadUInt32()
	if err != nil {
		return err
	}
	node.ByteSize, err = r.ReadInt32()
	if err != nil {
		return err
	}
	node.Index, err = r.ReadInt32()
	if err != nil {
		return err
	}
	node.MetaFlags, err = r.ReadUInt32()
	if err != nil {
		return err
	}
	// v19 及以上追加八字节 RefTypeHash
	// Version 19 and later append an eight-byte RefTypeHash
	if af.Header.Version >= 19 {
		node.RefTypeHash, err = r.ReadUInt64()
		return err
	}
	return nil
}

// readAssetInfo 按格式版本读取一个对象表条目并解析其实际 class ID
// readAssetInfo reads one object-table entry according to the format version and resolves its actual class ID
func (af *AssetsFile) readAssetInfo(r *binaryio.EndianReader, info *AssetInfo) error {
	v := af.Header.Version
	info.ScriptTypeIndex = -1

	// 格式 14 同时引入固定 Int64 PathID 和每个对象表条目前的四字节对齐
	// 旧 BigID 条目也使用 Int64，但在线格式中不对齐
	// Format 14 introduced unconditional Int64 PathIDs and four-byte alignment before each object-table entry
	// Legacy BigID entries also use Int64 but remain unaligned on the wire
	if v >= 14 {
		if err := alignMetadata4(r, "asset info"); err != nil {
			return err
		}
	}

	// PathId 在 v14+ 或旧 BigID 布局中为 Int64，否则为 Int32
	// PathId is Int64 in v14+ or legacy BigID layouts and Int32 otherwise
	if v >= 14 || af.Metadata.BigIDEnabled != 0 {
		pid, err := r.ReadInt64()
		if err != nil {
			return err
		}
		info.PathId = pid
	} else {
		pid, err := r.ReadInt32()
		if err != nil {
			return err
		}
		info.PathId = int64(pid)
	}

	// ByteOffset 在 v22+ 为 UInt64，否则为 UInt32
	// ByteOffset is UInt64 in v22+ and UInt32 otherwise
	if v >= 22 {
		off, err := r.ReadUInt64()
		if err != nil {
			return err
		}
		if off > math.MaxInt64 {
			return fmt.Errorf("asset byte offset %d exceeds int64", off)
		}
		info.ByteOffset = int64(off)
	} else {
		off, err := r.ReadUInt32()
		if err != nil {
			return err
		}
		info.ByteOffset = int64(off)
	}

	// ByteSize 固定为 UInt32
	// ByteSize is always UInt32
	size, err := r.ReadUInt32()
	if err != nil {
		return err
	}
	info.ByteSize = size

	// 随后读取 TypeIdOrIndex
	// Then read TypeIdOrIndex
	typeIdx, err := r.ReadInt32()
	if err != nil {
		return err
	}
	info.TypeIdOrIndex = typeIdx

	// v16 起 TypeIdOrIndex 索引 TypeTreeTypes，v16 以前该 Int32 是 serialized type 标识，后续 Int16 显式保存 class ID
	// From v16 TypeIdOrIndex indexes TypeTreeTypes; before v16 the Int32 is a serialized-type identifier followed by an explicit Int16 class ID
	if v >= 16 {
		if typeIdx < 0 || int64(typeIdx) >= int64(len(af.Metadata.TypeTreeTypes)) {
			return fmt.Errorf("type tree index %d out of range [0, %d)", typeIdx, len(af.Metadata.TypeTreeTypes))
		}
		info.TypeId = af.Metadata.TypeTreeTypes[int64(typeIdx)].TypeId
	} else {
		classID, err := r.ReadUInt16()
		if err != nil {
			return fmt.Errorf("read legacy asset class id: %w", err)
		}
		info.LegacyClassID = classID
		info.TypeId = int32(classID)
	}

	// v12 至 v16 在对象项中显式保存 Int16 ScriptTypeIndex
	// Versions 12 through 16 store an explicit Int16 ScriptTypeIndex in each object entry
	if v <= 16 {
		info.ScriptTypeIndex, err = r.ReadInt16()
		if err != nil {
			return fmt.Errorf("read asset script type index: %w", err)
		}
	}

	// v15 至 v16 在对象项末尾保存 Stripped 字节
	// Versions 15 through 16 store a Stripped byte at the end of each object entry
	if v >= 15 && v <= 16 {
		info.Stripped, err = r.ReadByte()
		if err != nil {
			return fmt.Errorf("read asset stripped flag: %w", err)
		}
	}

	return nil
}

// validateMetadataCount 使用剩余字节与最小条目宽度校验 metadata 数组计数
// validateMetadataCount validates a metadata array count against remaining bytes and minimum entry width
func validateMetadataCount(name string, count int32, remaining int64, minimumEntrySize int64) error {
	if count < 0 {
		return fmt.Errorf("negative %s count %d", name, count)
	}
	if minimumEntrySize <= 0 {
		minimumEntrySize = 1
	}
	if int64(count) > remaining/minimumEntrySize {
		return fmt.Errorf("%s count %d cannot fit in %d remaining metadata bytes (minimum entry size %d)", name, count, remaining, minimumEntrySize)
	}
	return nil
}

// minimumSerializedTypeSize 返回 SerializedType 可使用的保守最小宽度
// 条件存在的 ScriptIdHash 被有意省略，使该值保持下界
// minimumSerializedTypeSize returns a conservative minimum width for SerializedType
// The conditionally present ScriptIdHash is deliberately omitted so the result remains a lower bound
func minimumSerializedTypeSize(version uint32, typeTreeEnabled bool, isRefType bool) int64 {
	size := int64(4)
	if version >= 16 {
		size++
	}
	if version >= 17 {
		size += 2
	}
	if version >= 13 {
		size += 16
	}
	if typeTreeEnabled {
		size += 8
	}
	if typeTreeEnabled && version >= 21 {
		if isRefType {
			size += 3
		} else {
			size += 4
		}
	}
	return size
}

// minimumAssetInfoSize 返回指定版本对象表条目的最小线格式宽度
// minimumAssetInfoSize returns the minimum wire width of an object-table entry for a format version
func minimumAssetInfoSize(version uint32, bigIDEnabled bool) int64 {
	size := int64(4)
	if version >= 14 || bigIDEnabled {
		size = 8
	}
	if version >= 22 {
		size += 8
	} else {
		size += 4
	}
	size += 8
	if version < 16 {
		size += 2
	}
	if version <= 16 {
		size += 2
	}
	if version >= 15 && version <= 16 {
		size++
	}
	return size
}

// skipMetadataBytes 在边界校验后跳过指定数量的 metadata 字节
// skipMetadataBytes skips a number of metadata bytes after bounds validation
func skipMetadataBytes(r *binaryio.EndianReader, count int64, what string) error {
	if count < 0 || count > r.Remaining() {
		return fmt.Errorf("%s requires %d bytes but only %d metadata bytes remain", what, count, r.Remaining())
	}
	r.Skip(count)
	return nil
}

// alignMetadata4 将 metadata reader 推进到四字节边界
// alignMetadata4 advances the metadata reader to a four-byte boundary
func alignMetadata4(r *binaryio.EndianReader, what string) error {
	padding := (4 - r.Pos()%4) % 4
	if err := skipMetadataBytes(r, padding, what+" alignment"); err != nil {
		return err
	}
	return nil
}

// validateTypeTreeStringOffsets 校验所有本地 TypeTree 类型名和字段名偏移
// validateTypeTreeStringOffsets validates every local TypeTree type-name and field-name offset
func validateTypeTreeStringOffsets(tt *TypeTreeType) error {
	for i := range tt.Nodes {
		if err := validateTypeTreeStringOffset("type", int64(i), tt.Nodes[i].TypeStrOff, tt.StringBuffer); err != nil {
			return err
		}
		if err := validateTypeTreeStringOffset("name", int64(i), tt.Nodes[i].NameStrOff, tt.StringBuffer); err != nil {
			return err
		}
	}
	return nil
}

// validateTypeTreeStringOffset 校验本地字符串偏移存在 NUL 终止符，并保留公共字符串表偏移
// validateTypeTreeStringOffset validates NUL termination for local string offsets and preserves common-table offsets
func validateTypeTreeStringOffset(kind string, nodeIndex int64, offset uint32, stringBuffer []byte) error {
	// 最高位偏移指向每类型 StringBuffer 之外的 Unity 内置公共字符串表，因此保持不透明以兼容未来新增公共字符串
	// High-bit offsets address Unity's built-in common string table outside the per-type StringBuffer, so they remain opaque for forward compatibility with new common strings
	if offset&0x80000000 != 0 {
		return nil
	}
	if uint64(offset) >= uint64(len(stringBuffer)) {
		return fmt.Errorf("type tree node[%d] %s string offset %d is outside local string buffer size %d", nodeIndex, kind, offset, len(stringBuffer))
	}
	if bytes.IndexByte(stringBuffer[int64(offset):], 0) < 0 {
		return fmt.Errorf("type tree node[%d] %s string at local offset %d is not null-terminated", nodeIndex, kind, offset)
	}
	return nil
}

// validateAssetInfos 校验对象 PathID 唯一、数据范围有效且互不重叠
// validateAssetInfos validates unique object PathIDs and valid non-overlapping data ranges
func (af *AssetsFile) validateAssetInfos() error {
	if af.Header.DataOffset < 0 || af.Header.DataOffset > af.Header.FileSize {
		return fmt.Errorf("data offset %d is outside file size %d", af.Header.DataOffset, af.Header.FileSize)
	}
	dataSize := af.Header.FileSize - af.Header.DataOffset
	seenPathIDs := make(map[int64]int64, len(af.Metadata.AssetInfos))
	type objectRange struct {
		start  int64 // 相对于数据区的范围起点 / Range start relative to the data section
		end    int64 // 相对于数据区的范围终点 / Range end relative to the data section
		pathID int64 // 诊断信息中的对象 PathID / Object PathID used in diagnostics
		index  int64 // AssetInfos 中的对象索引 / Object index in AssetInfos
	}
	ranges := make([]objectRange, 0, len(af.Metadata.AssetInfos))
	for i := range af.Metadata.AssetInfos {
		info := &af.Metadata.AssetInfos[i]
		if previous, ok := seenPathIDs[info.PathId]; ok {
			return fmt.Errorf("duplicate PathID %d in asset infos %d and %d", info.PathId, previous, i)
		}
		seenPathIDs[info.PathId] = int64(i)
		if info.ByteOffset < 0 || info.ByteOffset > dataSize {
			return fmt.Errorf("asset info[%d] PathID %d byte offset %d is outside data section size %d", i, info.PathId, info.ByteOffset, dataSize)
		}
		size := int64(info.ByteSize)
		if size > dataSize-info.ByteOffset {
			return fmt.Errorf("asset info[%d] PathID %d range offset=%d size=%d exceeds data section size %d", i, info.PathId, info.ByteOffset, info.ByteSize, dataSize)
		}
		if size != 0 {
			ranges = append(ranges, objectRange{
				start:  info.ByteOffset,
				end:    info.ByteOffset + size,
				pathID: info.PathId,
				index:  int64(i),
			})
		}
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].start != ranges[j].start {
			return ranges[i].start < ranges[j].start
		}
		return ranges[i].end < ranges[j].end
	})
	for i := 1; i < len(ranges); i++ {
		previous, current := ranges[i-1], ranges[i]
		if current.start < previous.end {
			return fmt.Errorf("asset data ranges overlap: asset info[%d] PathID %d [%d,%d) and asset info[%d] PathID %d [%d,%d)",
				previous.index, previous.pathID, previous.start, previous.end,
				current.index, current.pathID, current.start, current.end)
		}
	}
	return nil
}

// readLocalSerializedObjectIdentifier 读取一个本地脚本对象标识，并按版本处理 PathID 宽度与对齐
// readLocalSerializedObjectIdentifier reads one local script-object identifier with versioned PathID width and alignment
func (af *AssetsFile) readLocalSerializedObjectIdentifier(r *binaryio.EndianReader) (LocalSerializedObjectIdentifier, error) {
	var identifier LocalSerializedObjectIdentifier
	index, err := r.ReadInt32()
	if err != nil {
		return identifier, err
	}
	identifier.LocalSerializedFileIndex = index
	if af.Header.Version >= 14 {
		if err := alignMetadata4(r, "local serialized object identifier"); err != nil {
			return identifier, err
		}
		identifier.LocalIdentifierInFile, err = r.ReadInt64()
		return identifier, err
	}
	localID, err := r.ReadInt32()
	identifier.LocalIdentifierInFile = int64(localID)
	return identifier, err
}

// readExternalFile 读取一个外部文件引用记录
// readExternalFile reads one external-file reference record
func (af *AssetsFile) readExternalFile(r *binaryio.EndianReader, ext *ExternalFile) error {
	// v6 及以上首先保存通常为空的 AssetPath 字符串
	// Version 6 and later begin with the usually empty AssetPath string
	if af.Header.Version >= 6 {
		assetPath, err := r.ReadNullString()
		if err != nil {
			return err
		}
		ext.AssetPath = assetPath
	}

	// 随后保存 16 字节 GUID
	// Then comes the 16-byte GUID
	if err := r.ReadFull(ext.Guid[:]); err != nil {
		return err
	}

	// GUID 后保存 Int32 Type
	// GUID is followed by Int32 Type
	t, err := r.ReadInt32()
	if err != nil {
		return err
	}
	ext.Type = t

	// 记录以 NUL 结尾的 PathName 收尾
	// The record ends with NUL-terminated PathName
	path, err := r.ReadNullString()
	if err != nil {
		return err
	}
	ext.PathName = path

	return nil
}
