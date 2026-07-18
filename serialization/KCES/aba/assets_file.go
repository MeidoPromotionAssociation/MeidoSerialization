package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// .assets / Unity SerializedFile
// AssetBundle 内部的实际资源容器，包含类型树、对象元数据和资源数据。
// 文件头使用 Big-Endian；后续字段的字节序由 Endianness 指定。结构如下：
//
//	[Header]
//	  - MetadataSize: uint32（元数据块大小，不含 header）
//	  - FileSize: uint32（整个文件大小，v22+ 为 int64）
//	  - Version: uint32（序列化格式版本）
//	  - DataOffset: uint32（第一个资源数据的偏移，v22+ 为 int64）
//	  - Endianness: byte（0=Little-Endian, 1=Big-Endian）+ 3 padding
//	  - (v22+: MetadataSize uint32, FileSize int64, DataOffset int64, 8 unused)
//
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
// .assets / Unity SerializedFile
// The actual resource container inside an AssetBundle, holding type trees, object metadata, and asset data.
// The header is Big-Endian; Endianness selects the byte order of subsequent fields. Its layout is:
//
//	[Header]
//	  - MetadataSize: uint32 (metadata size excluding the header)
//	  - FileSize: uint32, or int64 for version 22 and later
//	  - Version: uint32 serialized-file format version
//	  - DataOffset: uint32, or int64 for version 22 and later
//	  - Endianness: byte (0=Little-Endian, 1=Big-Endian) plus three padding bytes
//
//	[Metadata] (encoded according to Endianness)
//	  - UnityVersion, target platform, type-tree flag and definitions
//	  - AssetInfos, external references, version-21+ reference types, and user information
type AssetsFile struct {
	Header   AssetsFileHeader // 文件头 / File header
	Metadata AssetsMetadata   // 元数据（类型树 + 资源列表）/ Metadata including type trees and asset list
	Data     []byte           // 原始文件数据（用于按偏移读取资源）/ Raw file bytes used to read assets by offset
}

// AssetsFileHeader 表示 Unity 序列化文件头 / AssetsFileHeader represents a Unity serialized file header
type AssetsFileHeader struct {
	MetadataSize uint32 // 元数据块大小 / Metadata block size
	FileSize     int64  // 整个文件大小 / Total file size
	Version      uint32 // 序列化格式版本（常见：17-22）/ Serialized file format version, commonly 17-22
	DataOffset   int64  // 资源数据区起始偏移 / Start offset of the asset data area
	Endianness   bool   // true=Big-Endian, false=Little-Endian / true=Big-Endian, false=Little-Endian
}

// AssetsMetadata 包含类型树和资源信息 / AssetsMetadata contains type trees and asset metadata
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
	TrailingData    []byte                            // UserInformation 之后的未解析字节 / Unparsed bytes following UserInformation
}

// TypeTreeType 表示一个类型的类型树定义 / TypeTreeType represents the type tree definition for one Unity type
type TypeTreeType struct {
	TypeId           int32          // 类型 ID（如 28=Texture2D, 49=TextAsset）/ Class ID such as 28=Texture2D and 49=TextAsset
	IsStrippedType   bool           // 是否被剥离 / Whether this type is stripped
	ScriptTypeIndex  int16          // 脚本类型索引，-1 表示无脚本 / Script type index, with -1 meaning no script
	ScriptIdHash     [16]byte       // 脚本 ID 哈希（v13+, MonoBehaviour）/ Script ID hash for v13+ MonoBehaviour
	TypeHash         [16]byte       // 类型哈希 / Type hash
	Nodes            []TypeTreeNode // 类型树节点列表（仅当 TypeTreeEnabled=true）/ Type tree node list, present only when TypeTreeEnabled is true
	StringBuffer     []byte         // 字符串缓冲区 / String buffer
	TypeDependencies []int32        // 普通类型依赖（v21+ 且启用 TypeTree）/ Ordinary type dependencies for v21+ with TypeTree enabled
	ClassName        string         // 引用类型类名 / Referenced type class name
	Namespace        string         // 引用类型命名空间 / Referenced type namespace
	AssemblyName     string         // 引用类型程序集名 / Referenced type assembly name
}

// TypeTreeNode 表示类型树中的一个节点 / TypeTreeNode represents one node in a Unity type tree
type TypeTreeNode struct {
	Version     uint16 // 节点版本 / Node version
	Level       byte   // 层级深度 / Tree depth level
	TypeFlags   byte   // 类型标志（0x01=IsArray）/ Type flags, 0x01 means IsArray
	TypeStrOff  uint32 // 类型名在字符串缓冲区中的偏移 / Offset of the type name in the string buffer
	NameStrOff  uint32 // 字段名在字符串缓冲区中的偏移 / Offset of the field name in the string buffer
	ByteSize    int32  // 字段字节大小（-1 表示可变长度）/ Field byte size, -1 means variable length
	Index       int32  // 在父节点中的索引 / Index within the parent node
	MetaFlags   uint32 // 元标志（0x4000=AlignBytes）/ Metadata flags, 0x4000 means AlignBytes
	RefTypeHash uint64 // v19+ 引用类型哈希 / Referenced-type hash in v19+
}

// LocalSerializedObjectIdentifier 标识 metadata 中的本地脚本对象 / LocalSerializedObjectIdentifier identifies a local script object in metadata
type LocalSerializedObjectIdentifier struct {
	LocalSerializedFileIndex int32 // 本地序列化文件索引 / Local serialized-file index
	LocalIdentifierInFile    int64 // 文件内对象 ID / Local object identifier in the file
}

// AssetInfo 表示单个资源的元信息 / AssetInfo represents metadata for one asset object
type AssetInfo struct {
	PathId        int64  // 资源路径 ID（唯一标识）/ Asset PathID, unique within the file
	ByteOffset    int64  // 相对于 DataOffset 的偏移 / Offset relative to DataOffset
	ByteSize      uint32 // 资源数据大小 / Asset data size
	TypeIdOrIndex int32  // Serialized type 标识；v16+ 必须是 TypeTreeTypes 数组索引 / Serialized type identifier; v16+ must index TypeTreeTypes
	TypeId        int32  // 实际类型 ID（解析后填充）/ Actual class ID filled after parsing
}

// ExternalFile 表示外部文件引用 / ExternalFile represents an external file reference
type ExternalFile struct {
	AssetPath string   // 缓存资源虚拟路径 / Virtual cached-asset path
	Guid      [16]byte // GUID / GUID
	Type      int32    // 引用类型 / Reference type
	PathName  string   // 路径名 / Path name
}

// ReadAssetsFile 从字节数据中解析 Unity AssetsFile
func ReadAssetsFile(data []byte) (*AssetsFile, error) {
	af := &AssetsFile{Data: data}

	if len(data) < 20 {
		return nil, fmt.Errorf("data too short for assets file header: %d bytes", len(data))
	}

	// 1. 读取 header（始终 Big-Endian）
	headerReader := binaryio.NewEndianReader(data, binary.BigEndian)
	metadataSize, err := headerReader.ReadUInt32()
	if err != nil {
		return nil, fmt.Errorf("read metadata size failed: %w", err)
	}
	fileSize, err := headerReader.ReadUInt32()
	if err != nil {
		return nil, fmt.Errorf("read file size failed: %w", err)
	}
	version, err := headerReader.ReadUInt32()
	if err != nil {
		return nil, fmt.Errorf("read version failed: %w", err)
	}
	dataOffset, err := headerReader.ReadUInt32()
	if err != nil {
		return nil, fmt.Errorf("read data offset failed: %w", err)
	}
	endianness, err := headerReader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read endianness failed: %w", err)
	}
	var headerPadding [3]byte
	if err := headerReader.ReadFull(headerPadding[:]); err != nil {
		return nil, fmt.Errorf("read header padding failed: %w", err)
	}

	af.Header.MetadataSize = metadataSize
	af.Header.FileSize = int64(fileSize)
	af.Header.Version = version
	af.Header.DataOffset = int64(dataOffset)
	if endianness > 1 {
		return nil, fmt.Errorf("invalid serialized file endianness byte %d", endianness)
	}
	af.Header.Endianness = endianness == 1
	if af.Header.Version < 12 || af.Header.Version > 22 {
		return nil, fmt.Errorf("unsupported serialized file version %d (supported: 12-22)", af.Header.Version)
	}

	// v22+ 有扩展 header
	if af.Header.Version >= 22 {
		af.Header.MetadataSize, err = headerReader.ReadUInt32()
		if err != nil {
			return nil, fmt.Errorf("read extended metadata size failed: %w", err)
		}
		fileSize64, err := headerReader.ReadUInt64()
		if err != nil {
			return nil, fmt.Errorf("read extended file size failed: %w", err)
		}
		dataOffset64, err := headerReader.ReadUInt64()
		if err != nil {
			return nil, fmt.Errorf("read extended data offset failed: %w", err)
		}
		if _, err := headerReader.ReadUInt64(); err != nil {
			return nil, fmt.Errorf("read extended header unused field failed: %w", err)
		}
		if fileSize64 > math.MaxInt64 {
			return nil, fmt.Errorf("extended serialized file size %d exceeds int64", fileSize64)
		}
		if dataOffset64 > math.MaxInt64 {
			return nil, fmt.Errorf("extended serialized data offset %d exceeds int64", dataOffset64)
		}
		af.Header.FileSize = int64(fileSize64)
		af.Header.DataOffset = int64(dataOffset64)
	}
	if af.Header.FileSize != int64(len(data)) {
		return nil, fmt.Errorf("serialized file size %d does not match input length %d", af.Header.FileSize, len(data))
	}
	if af.Header.DataOffset < int64(headerReader.Pos()) || af.Header.DataOffset > af.Header.FileSize {
		return nil, fmt.Errorf("invalid serialized data offset %d for file size %d", af.Header.DataOffset, af.Header.FileSize)
	}
	metadataStart := int64(headerReader.Pos())
	metadataEnd := metadataStart + int64(af.Header.MetadataSize)
	if metadataEnd < metadataStart || metadataEnd > af.Header.FileSize {
		return nil, fmt.Errorf("serialized metadata range [%d, %d) exceeds declared file size %d", metadataStart, metadataEnd, af.Header.FileSize)
	}
	if metadataEnd > af.Header.DataOffset {
		return nil, fmt.Errorf("serialized metadata end %d exceeds data offset %d", metadataEnd, af.Header.DataOffset)
	}

	// 2. 确定字节序
	var order binary.ByteOrder
	if af.Header.Endianness {
		order = binary.BigEndian
	} else {
		order = binary.LittleEndian
	}

	// 3. 读取 Metadata
	// Give the metadata parser a bounded slice. It can therefore never consume
	// alignment padding or object bytes even if an inner count/string is forged.
	metadata := data[int(metadataStart):int(metadataEnd)]
	if err := af.readMetadata(metadata, 0, order); err != nil {
		return nil, fmt.Errorf("read metadata failed: %w", err)
	}
	if err := af.validateAssetInfos(); err != nil {
		return nil, fmt.Errorf("validate asset infos failed: %w", err)
	}

	return af, nil
}

// GetAssetData 读取指定资源的原始数据
func (af *AssetsFile) GetAssetData(info *AssetInfo) ([]byte, error) {
	if af == nil || info == nil {
		return nil, fmt.Errorf("nil assets file or asset info")
	}
	dataLen := int64(len(af.Data))
	if af.Header.FileSize != 0 {
		if af.Header.FileSize < 0 || af.Header.FileSize > dataLen {
			return nil, fmt.Errorf("declared file size %d is invalid for %d data bytes", af.Header.FileSize, len(af.Data))
		}
		dataLen = af.Header.FileSize
	}
	if af.Header.DataOffset < 0 || af.Header.DataOffset > dataLen || info.ByteOffset < 0 || info.ByteOffset > dataLen-af.Header.DataOffset {
		return nil, fmt.Errorf("asset data start out of bounds: dataOffset=%d byteOffset=%d in %d bytes", af.Header.DataOffset, info.ByteOffset, len(af.Data))
	}
	start := af.Header.DataOffset + info.ByteOffset
	size := int64(info.ByteSize)
	if size > dataLen-start {
		return nil, fmt.Errorf("asset data out of bounds: start=%d size=%d in %d bytes", start, size, len(af.Data))
	}
	end := start + size
	return af.Data[int(start):int(end)], nil
}

// GetAssetsByType 返回指定类型 ID 的所有资源
func (af *AssetsFile) GetAssetsByType(typeId int32) []AssetInfo {
	var result []AssetInfo
	for _, info := range af.Metadata.AssetInfos {
		if info.TypeId == typeId {
			result = append(result, info)
		}
	}
	return result
}

// GetAssetInfoByPathID returns the asset metadata with the requested PathID.
func (af *AssetsFile) GetAssetInfoByPathID(pathID int64) *AssetInfo {
	for i := range af.Metadata.AssetInfos {
		if af.Metadata.AssetInfos[i].PathId == pathID {
			return &af.Metadata.AssetInfos[i]
		}
	}
	return nil
}

// readMetadata 读取元数据部分
func (af *AssetsFile) readMetadata(data []byte, pos int, order binary.ByteOrder) error {
	r := binaryio.NewEndianReaderAt(data, pos, order)

	// 1. UnityVersion (null-terminated)
	ver, err := r.ReadNullString()
	if err != nil {
		return fmt.Errorf("read unity version failed: %w", err)
	}
	af.Metadata.UnityVersion = ver

	// 2. TargetPlatform
	af.Metadata.TargetPlatform, err = r.ReadUInt32()
	if err != nil {
		return fmt.Errorf("read target platform failed: %w", err)
	}

	// 3. TypeTreeEnabled. Before format 13 the type tree is always present and
	// there is no enable byte in metadata.
	if af.Header.Version >= 13 {
		b, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read type tree enabled failed: %w", err)
		}
		af.Metadata.TypeTreeEnabled = b != 0
	} else {
		af.Metadata.TypeTreeEnabled = true
	}

	// 4. TypeTreeTypes
	typeCount, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read type count failed: %w", err)
	}
	if err := validateMetadataCount("type", typeCount, r.Remaining(), minimumSerializedTypeSize(af.Header.Version, af.Metadata.TypeTreeEnabled, false)); err != nil {
		return err
	}
	af.Metadata.TypeTreeTypes = makeABACountedSliceForAppend[TypeTreeType](int(typeCount))
	for i := 0; i < int(typeCount); i++ {
		var tt TypeTreeType
		if err := af.readTypeTreeType(r, &tt, false); err != nil {
			return fmt.Errorf("read type tree type[%d] failed: %w", i, err)
		}
		af.Metadata.TypeTreeTypes = append(af.Metadata.TypeTreeTypes, tt)
	}

	// 5. BigIDEnabled. Unity formats 7 through 13 store this int32 between
	// SerializedTypes and the object count. A non-zero value changes legacy
	// PathIDs from int32 to int64; format 14 made int64 PathIDs unconditional.
	if af.Header.Version >= 7 && af.Header.Version < 14 {
		af.Metadata.BigIDEnabled, err = r.ReadInt32()
		if err != nil {
			return fmt.Errorf("read big ID enabled failed: %w", err)
		}
	}

	// 6. AssetInfos
	assetCount, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read asset count failed: %w", err)
	}
	if err := validateMetadataCount("asset", assetCount, r.Remaining(), minimumAssetInfoSize(af.Header.Version, af.Metadata.BigIDEnabled != 0)); err != nil {
		return err
	}
	af.Metadata.AssetInfos = makeABACountedSliceForAppend[AssetInfo](int(assetCount))
	for i := 0; i < int(assetCount); i++ {
		var info AssetInfo
		if err := af.readAssetInfo(r, &info); err != nil {
			return fmt.Errorf("read asset info[%d] failed: %w", i, err)
		}
		af.Metadata.AssetInfos = append(af.Metadata.AssetInfos, info)
	}

	// A LocalSerializedObjectIdentifier (script type) array precedes external
	// references in supported Unity formats. Parse the standards-compliant tail
	// first. Older versions of this Go writer accidentally omitted the empty
	// script-count field; retain a narrowly-scoped fallback only for its exact
	// zero-count shape so existing generated files remain readable.
	tailPos := r.Pos()
	tail, err := af.readMetadataTail(data, tailPos, order, true)
	const legacyGoWriterTailSize = 4 + 4 + 1 // external count + ref count + empty UserInformation
	if err != nil && af.Header.Version >= 17 && len(data)-tailPos == legacyGoWriterTailSize &&
		order.Uint32(data[tailPos:tailPos+4]) == 0 && data[len(data)-1] == 0 {
		legacyTail, legacyErr := af.readMetadataTail(data, tailPos, order, false)
		if legacyErr == nil {
			tail = legacyTail
			err = nil
		}
	}
	if err != nil {
		return err
	}
	af.Metadata.ScriptTypes = tail.ScriptTypes
	af.Metadata.ExternalFiles = tail.ExternalFiles
	af.Metadata.RefTypes = tail.RefTypes
	af.Metadata.UserInformation = tail.UserInformation
	af.Metadata.TrailingData = tail.TrailingData
	return nil
}

type assetsMetadataTail struct {
	ScriptTypes     []LocalSerializedObjectIdentifier
	ExternalFiles   []ExternalFile
	RefTypes        []TypeTreeType
	UserInformation string
	TrailingData    []byte
}

func (af *AssetsFile) readMetadataTail(data []byte, pos int, order binary.ByteOrder, hasScriptTypeCount bool) (assetsMetadataTail, error) {
	var tail assetsMetadataTail
	r := binaryio.NewEndianReaderAt(data, pos, order)
	if hasScriptTypeCount {
		scriptCount, err := r.ReadInt32()
		if err != nil {
			return tail, fmt.Errorf("read script type count failed: %w", err)
		}
		entrySize := 8
		if af.Header.Version >= 14 {
			entrySize = 12
		}
		if err := validateMetadataCount("script type", scriptCount, r.Remaining(), entrySize); err != nil {
			return tail, err
		}
		tail.ScriptTypes = makeABACountedSliceForAppend[LocalSerializedObjectIdentifier](int(scriptCount))
		for i := 0; i < int(scriptCount); i++ {
			identifier, err := af.readLocalSerializedObjectIdentifier(r)
			if err != nil {
				return tail, fmt.Errorf("read script type[%d]: %w", i, err)
			}
			tail.ScriptTypes = append(tail.ScriptTypes, identifier)
		}
	}

	extCount, err := r.ReadInt32()
	if err != nil {
		return tail, fmt.Errorf("read external count failed: %w", err)
	}
	if err := validateMetadataCount("external file", extCount, r.Remaining(), 22); err != nil {
		return tail, err
	}
	externals := makeABACountedSliceForAppend[ExternalFile](int(extCount))
	for i := 0; i < int(extCount); i++ {
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
		tail.RefTypes = makeABACountedSliceForAppend[TypeTreeType](int(refTypeCount))
		for i := 0; i < int(refTypeCount); i++ {
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
		tail.TrailingData, err = r.ReadBytes(r.Remaining())
		if err != nil {
			return tail, fmt.Errorf("read metadata trailing data: %w", err)
		}
	}
	return tail, nil
}

// readTypeTreeType 读取单个类型树类型定义
func (af *AssetsFile) readTypeTreeType(r *binaryio.EndianReader, tt *TypeTreeType, isRefType bool) error {
	var err error
	v := af.Header.Version
	tt.ScriptTypeIndex = -1

	// TypeId
	typeId, err := r.ReadInt32()
	if err != nil {
		return err
	}
	tt.TypeId = typeId

	// IsStrippedType was added in format 16.
	if v >= 16 {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		tt.IsStrippedType = b != 0
	}

	// ScriptTypeIndex (v17+)
	if v >= 17 {
		idx, err := r.ReadInt16()
		if err != nil {
			return err
		}
		tt.ScriptTypeIndex = idx
	}

	// ScriptIdHash (v13+). Before v16 script types use a negative class ID;
	// from v16 onward MonoBehaviour is identified by class ID 114.
	if v >= 13 {
		if (isRefType && tt.ScriptTypeIndex >= 0) || (v < 16 && typeId < 0) || (v >= 16 && typeId == 114) {
			if err := r.ReadFull(tt.ScriptIdHash[:]); err != nil {
				return err
			}
		}
	}

	// TypeHash
	if v >= 13 {
		if err := r.ReadFull(tt.TypeHash[:]); err != nil {
			return err
		}
	}

	// TypeTree nodes (仅当 TypeTreeEnabled)
	if af.Metadata.TypeTreeEnabled {
		if v >= 12 {
			// Blob format: nodeCount + stringBufferSize + nodes + stringBuffer
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
			nodeSize := 24
			if v >= 19 {
				nodeSize = 32
			}
			if strBufSize < 0 {
				return fmt.Errorf("negative type tree string buffer size %d", strBufSize)
			}
			nodeBytes := int64(nodeCount) * int64(nodeSize)
			required := nodeBytes + int64(strBufSize)
			if nodeBytes < 0 || required < nodeBytes || required > int64(r.Remaining()) {
				return fmt.Errorf("type tree nodes/string buffer require %d bytes but only %d metadata bytes remain", required, r.Remaining())
			}

			tt.Nodes = makeABACountedSliceForAppend[TypeTreeNode](int(nodeCount))
			for i := 0; i < int(nodeCount); i++ {
				var node TypeTreeNode
				if err := af.readTypeTreeNodeBlob(r, &node); err != nil {
					return fmt.Errorf("read node[%d]: %w", i, err)
				}
				tt.Nodes = append(tt.Nodes, node)
			}

			if int64(strBufSize) > int64(r.Remaining()) {
				return fmt.Errorf("type tree string buffer size %d exceeds remaining metadata %d", strBufSize, r.Remaining())
			}
			tt.StringBuffer = make([]byte, int(strBufSize))
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
			if depCount < 0 || int64(depCount) > int64(r.Remaining())/4 {
				return fmt.Errorf("invalid type dependency count %d with %d bytes remaining", depCount, r.Remaining())
			}
			tt.TypeDependencies = makeABACountedSliceForAppend[int32](int(depCount))
			for i := 0; i < int(depCount); i++ {
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

// readTypeTreeNodeBlob 读取 blob 格式的类型树节点
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
	// v19+ 有额外的 8 字节（RefTypeHash）
	if af.Header.Version >= 19 {
		node.RefTypeHash, err = r.ReadUInt64()
		return err
	}
	return nil
}

// readAssetInfo 读取单个资源信息
func (af *AssetsFile) readAssetInfo(r *binaryio.EndianReader, info *AssetInfo) error {
	v := af.Header.Version

	// Format 14 introduced both unconditional int64 PathIDs and the 4-byte
	// alignment before each object-table entry. Legacy BigID entries are int64
	// too, but remain unaligned on the wire.
	if v >= 14 {
		if err := alignMetadata4(r, "asset info"); err != nil {
			return err
		}
	}

	// PathId
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

	// ByteOffset
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

	// ByteSize
	size, err := r.ReadUInt32()
	if err != nil {
		return err
	}
	info.ByteSize = size

	// TypeIdOrIndex
	typeIdx, err := r.ReadInt32()
	if err != nil {
		return err
	}
	info.TypeIdOrIndex = typeIdx

	// Resolve the actual class ID. Before v16 the int32 field is a serialized
	// type identifier and the following int16 stores the class ID explicitly.
	if v >= 16 {
		if typeIdx < 0 || int64(typeIdx) >= int64(len(af.Metadata.TypeTreeTypes)) {
			return fmt.Errorf("type tree index %d out of range [0, %d)", typeIdx, len(af.Metadata.TypeTreeTypes))
		}
		info.TypeId = af.Metadata.TypeTreeTypes[int(typeIdx)].TypeId
	} else {
		classID, err := r.ReadInt16()
		if err != nil {
			return fmt.Errorf("read legacy asset class id: %w", err)
		}
		info.TypeId = int32(classID)
	}

	// ScriptTypeIndex (v16-)
	if v <= 16 {
		if err := skipMetadataBytes(r, 2, "asset script type index"); err != nil {
			return err
		}
	}

	// Stripped (v15-v16)
	if v >= 15 && v <= 16 {
		if err := skipMetadataBytes(r, 1, "asset stripped flag"); err != nil {
			return err
		}
	}

	return nil
}

func validateMetadataCount(name string, count int32, remaining int, minimumEntrySize int) error {
	if count < 0 {
		return fmt.Errorf("negative %s count %d", name, count)
	}
	if minimumEntrySize <= 0 {
		minimumEntrySize = 1
	}
	if int64(count) > int64(remaining)/int64(minimumEntrySize) {
		return fmt.Errorf("%s count %d cannot fit in %d remaining metadata bytes (minimum entry size %d)", name, count, remaining, minimumEntrySize)
	}
	return nil
}

func minimumSerializedTypeSize(version uint32, typeTreeEnabled bool, isRefType bool) int {
	size := 4 // class ID
	if version >= 16 {
		size++ // IsStrippedType
	}
	if version >= 17 {
		size += 2 // ScriptTypeIndex
	}
	if version >= 13 {
		size += 16 // TypeHash; conditional ScriptIdHash is deliberately omitted
	}
	if typeTreeEnabled {
		size += 8 // node count + string buffer size; both may be zero
	}
	if typeTreeEnabled && version >= 21 {
		if isRefType {
			size += 3 // class, namespace, and assembly NUL terminators
		} else {
			size += 4 // dependency count; entries may be empty
		}
	}
	return size
}

func minimumAssetInfoSize(version uint32, bigIDEnabled bool) int {
	size := 4 // PathID before v14
	if version >= 14 || bigIDEnabled {
		size = 8
	}
	if version >= 22 {
		size += 8 // ByteOffset
	} else {
		size += 4
	}
	size += 8 // ByteSize + TypeIdOrIndex
	if version < 16 {
		size += 2 // class ID
	}
	if version <= 16 {
		size += 2 // ScriptTypeIndex
	}
	if version >= 15 && version <= 16 {
		size++ // stripped flag
	}
	return size
}

func skipMetadataBytes(r *binaryio.EndianReader, count int, what string) error {
	if count < 0 || count > r.Remaining() {
		return fmt.Errorf("%s requires %d bytes but only %d metadata bytes remain", what, count, r.Remaining())
	}
	r.Skip(count)
	return nil
}

func alignMetadata4(r *binaryio.EndianReader, what string) error {
	padding := (4 - r.Pos()%4) % 4
	if err := skipMetadataBytes(r, padding, what+" alignment"); err != nil {
		return err
	}
	return nil
}

func validateTypeTreeStringOffsets(tt *TypeTreeType) error {
	for i := range tt.Nodes {
		if err := validateTypeTreeStringOffset("type", i, tt.Nodes[i].TypeStrOff, tt.StringBuffer); err != nil {
			return err
		}
		if err := validateTypeTreeStringOffset("name", i, tt.Nodes[i].NameStrOff, tt.StringBuffer); err != nil {
			return err
		}
	}
	return nil
}

func validateTypeTreeStringOffset(kind string, nodeIndex int, offset uint32, stringBuffer []byte) error {
	// High-bit offsets address Unity's built-in common string table, which is
	// external to the per-type StringBuffer. Keep those values opaque so new
	// Unity common strings remain forward-compatible.
	if offset&0x80000000 != 0 {
		return nil
	}
	if uint64(offset) >= uint64(len(stringBuffer)) {
		return fmt.Errorf("type tree node[%d] %s string offset %d is outside local string buffer size %d", nodeIndex, kind, offset, len(stringBuffer))
	}
	if bytes.IndexByte(stringBuffer[int(offset):], 0) < 0 {
		return fmt.Errorf("type tree node[%d] %s string at local offset %d is not null-terminated", nodeIndex, kind, offset)
	}
	return nil
}

func (af *AssetsFile) validateAssetInfos() error {
	if af.Header.DataOffset < 0 || af.Header.DataOffset > af.Header.FileSize {
		return fmt.Errorf("data offset %d is outside file size %d", af.Header.DataOffset, af.Header.FileSize)
	}
	dataSize := af.Header.FileSize - af.Header.DataOffset
	seenPathIDs := make(map[int64]int, len(af.Metadata.AssetInfos))
	type objectRange struct {
		start  int64
		end    int64
		pathID int64
		index  int
	}
	ranges := make([]objectRange, 0, len(af.Metadata.AssetInfos))
	for i := range af.Metadata.AssetInfos {
		info := &af.Metadata.AssetInfos[i]
		if previous, ok := seenPathIDs[info.PathId]; ok {
			return fmt.Errorf("duplicate PathID %d in asset infos %d and %d", info.PathId, previous, i)
		}
		seenPathIDs[info.PathId] = i
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
				index:  i,
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

// readExternalFile 读取外部文件引用
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

func (af *AssetsFile) readExternalFile(r *binaryio.EndianReader, ext *ExternalFile) error {
	// Empty string (v6+)
	if af.Header.Version >= 6 {
		assetPath, err := r.ReadNullString()
		if err != nil {
			return err
		}
		ext.AssetPath = assetPath
	}

	// GUID
	if err := r.ReadFull(ext.Guid[:]); err != nil {
		return err
	}

	// Type
	t, err := r.ReadInt32()
	if err != nil {
		return err
	}
	ext.Type = t

	// PathName
	path, err := r.ReadNullString()
	if err != nil {
		return err
	}
	ext.PathName = path

	return nil
}
