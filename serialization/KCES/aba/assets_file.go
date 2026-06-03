package aba

import (
	"encoding/binary"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// AssetsFile 表示 Unity 序列化文件 .assets / AssetsFile represents a Unity serialized .assets file
// 这是 AssetBundle 内部的实际资源容器，包含类型树和资源数据 / This is the actual resource container inside an AssetBundle, containing type trees and asset data
//
// 文件结构（header 使用 Big-Endian，之后按 Endianness 字段决定）：
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
	UnityVersion    string         // Unity 版本字符串 / Unity version string
	TargetPlatform  uint32         // 目标平台 ID / Target platform ID
	TypeTreeEnabled bool           // 是否包含类型树 / Whether type tree data is present
	TypeTreeTypes   []TypeTreeType // 类型树定义列表 / Type tree definition list
	AssetInfos      []AssetInfo    // 资源信息列表 / Asset metadata list
	ExternalFiles   []ExternalFile // 外部文件引用 / External file references
}

// TypeTreeType 表示一个类型的类型树定义 / TypeTreeType represents the type tree definition for one Unity type
type TypeTreeType struct {
	TypeId          int32          // 类型 ID（如 28=Texture2D, 49=TextAsset）/ Class ID such as 28=Texture2D and 49=TextAsset
	IsStrippedType  bool           // 是否被剥离 / Whether this type is stripped
	ScriptTypeIndex uint16         // 脚本类型索引（MonoBehaviour 使用）/ Script type index used by MonoBehaviour
	ScriptIdHash    [16]byte       // 脚本 ID 哈希（v13+, MonoBehaviour）/ Script ID hash for v13+ MonoBehaviour
	TypeHash        [16]byte       // 类型哈希 / Type hash
	Nodes           []TypeTreeNode // 类型树节点列表（仅当 TypeTreeEnabled=true）/ Type tree node list, present only when TypeTreeEnabled is true
	StringBuffer    []byte         // 字符串缓冲区 / String buffer
}

// TypeTreeNode 表示类型树中的一个节点 / TypeTreeNode represents one node in a Unity type tree
type TypeTreeNode struct {
	Version    uint16 // 节点版本 / Node version
	Level      byte   // 层级深度 / Tree depth level
	TypeFlags  byte   // 类型标志（0x01=IsArray）/ Type flags, 0x01 means IsArray
	TypeStrOff uint32 // 类型名在字符串缓冲区中的偏移 / Offset of the type name in the string buffer
	NameStrOff uint32 // 字段名在字符串缓冲区中的偏移 / Offset of the field name in the string buffer
	ByteSize   int32  // 字段字节大小（-1 表示可变长度）/ Field byte size, -1 means variable length
	Index      int32  // 在父节点中的索引 / Index within the parent node
	MetaFlags  uint32 // 元标志（0x4000=AlignBytes）/ Metadata flags, 0x4000 means AlignBytes
}

// AssetInfo 表示单个资源的元信息 / AssetInfo represents metadata for one asset object
type AssetInfo struct {
	PathId        int64  // 资源路径 ID（唯一标识）/ Asset PathID, unique within the file
	ByteOffset    int64  // 相对于 DataOffset 的偏移 / Offset relative to DataOffset
	ByteSize      uint32 // 资源数据大小 / Asset data size
	TypeIdOrIndex int32  // v16+: TypeTreeTypes 数组索引; v15-: 类型 ID / v16+: TypeTreeTypes index; v15-: class ID
	TypeId        int32  // 实际类型 ID（解析后填充）/ Actual class ID filled after parsing
}

// ExternalFile 表示外部文件引用 / ExternalFile represents an external file reference
type ExternalFile struct {
	Guid     [16]byte // GUID / GUID
	Type     int32    // 引用类型 / Reference type
	PathName string   // 路径名 / Path name
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
	headerReader.Skip(3) // padding

	af.Header.MetadataSize = metadataSize
	af.Header.FileSize = int64(fileSize)
	af.Header.Version = version
	af.Header.DataOffset = int64(dataOffset)
	af.Header.Endianness = endianness != 0

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
		af.Header.FileSize = int64(fileSize64)
		af.Header.DataOffset = int64(dataOffset64)
	}

	// 2. 确定字节序
	var order binary.ByteOrder
	if af.Header.Endianness {
		order = binary.BigEndian
	} else {
		order = binary.LittleEndian
	}

	// 3. 读取 Metadata
	if err := af.readMetadata(data, headerReader.Pos(), order); err != nil {
		return nil, fmt.Errorf("read metadata failed: %w", err)
	}

	return af, nil
}

// GetAssetData 读取指定资源的原始数据
func (af *AssetsFile) GetAssetData(info *AssetInfo) ([]byte, error) {
	start := af.Header.DataOffset + info.ByteOffset
	end := start + int64(info.ByteSize)
	if start < 0 || end > int64(len(af.Data)) {
		return nil, fmt.Errorf("asset data out of bounds: [%d, %d) in %d bytes", start, end, len(af.Data))
	}
	return af.Data[start:end], nil
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

	// 3. TypeTreeEnabled
	b, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("read type tree enabled failed: %w", err)
	}
	af.Metadata.TypeTreeEnabled = b != 0

	// 4. TypeTreeTypes
	typeCount, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read type count failed: %w", err)
	}
	af.Metadata.TypeTreeTypes = make([]TypeTreeType, typeCount)
	for i := range af.Metadata.TypeTreeTypes {
		if err := af.readTypeTreeType(r, &af.Metadata.TypeTreeTypes[i]); err != nil {
			return fmt.Errorf("read type tree type[%d] failed: %w", i, err)
		}
	}

	// 5. AssetInfos
	assetCount, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read asset count failed: %w", err)
	}
	af.Metadata.AssetInfos = make([]AssetInfo, assetCount)
	for i := range af.Metadata.AssetInfos {
		if err := af.readAssetInfo(r, &af.Metadata.AssetInfos[i]); err != nil {
			return fmt.Errorf("read asset info[%d] failed: %w", i, err)
		}
	}

	// 6. ExternalFiles
	extCount, err := r.ReadInt32()
	if err != nil {
		return fmt.Errorf("read external count failed: %w", err)
	}
	af.Metadata.ExternalFiles = make([]ExternalFile, extCount)
	for i := range af.Metadata.ExternalFiles {
		if err := af.readExternalFile(r, &af.Metadata.ExternalFiles[i]); err != nil {
			return fmt.Errorf("read external file[%d] failed: %w", i, err)
		}
	}

	return nil
}

// readTypeTreeType 读取单个类型树类型定义
func (af *AssetsFile) readTypeTreeType(r *binaryio.EndianReader, tt *TypeTreeType) error {
	var err error
	v := af.Header.Version

	// TypeId
	typeId, err := r.ReadInt32()
	if err != nil {
		return err
	}
	tt.TypeId = typeId

	// IsStrippedType (v13+)
	if v >= 13 {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		tt.IsStrippedType = b != 0
	}

	// ScriptTypeIndex (v17+)
	if v >= 17 {
		idx, err := r.ReadUInt16()
		if err != nil {
			return err
		}
		tt.ScriptTypeIndex = idx
	}

	// ScriptIdHash (v13+, 仅 MonoBehaviour typeId=114 或 typeId<0)
	if v >= 13 {
		if (v < 17 && typeId < 0) || (v >= 17 && typeId == 114) {
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

			tt.Nodes = make([]TypeTreeNode, nodeCount)
			for i := range tt.Nodes {
				if err := af.readTypeTreeNodeBlob(r, &tt.Nodes[i]); err != nil {
					return fmt.Errorf("read node[%d]: %w", i, err)
				}
			}

			tt.StringBuffer = make([]byte, strBufSize)
			if err := r.ReadFull(tt.StringBuffer); err != nil {
				return err
			}
		}
	}

	// TypeDependencies (v21+)
	if v >= 21 {
		depCount, err := r.ReadInt32()
		if err != nil {
			return err
		}
		r.Skip(int(depCount) * 4) // int32 array
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
		r.Skip(8)
	}
	return nil
}

// readAssetInfo 读取单个资源信息
func (af *AssetsFile) readAssetInfo(r *binaryio.EndianReader, info *AssetInfo) error {
	v := af.Header.Version

	// Align before each entry
	r.Align4()

	// PathId
	if v >= 14 {
		pid, err := r.ReadInt64()
		if err != nil {
			return err
		}
		info.PathId = pid
	} else {
		pid, err := r.ReadUInt32()
		if err != nil {
			return err
		}
		info.PathId = int64(pid)
	}

	// ByteOffset
	if v >= 22 {
		off, err := r.ReadInt64()
		if err != nil {
			return err
		}
		info.ByteOffset = off
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

	// 解析实际 TypeId
	if v >= 16 {
		if int(typeIdx) < len(af.Metadata.TypeTreeTypes) {
			info.TypeId = af.Metadata.TypeTreeTypes[typeIdx].TypeId
		}
	} else {
		info.TypeId = typeIdx
	}

	// OldTypeId (v15-)
	if v <= 15 {
		r.Skip(2) // uint16
	}

	// ScriptTypeIndex (v16-)
	if v <= 16 {
		r.Skip(2) // uint16
	}

	// Stripped (v15-v16)
	if v >= 15 && v <= 16 {
		r.Skip(1) // byte
	}

	return nil
}

// readExternalFile 读取外部文件引用
func (af *AssetsFile) readExternalFile(r *binaryio.EndianReader, ext *ExternalFile) error {
	// Empty string (v6+)
	if af.Header.Version >= 6 {
		if _, err := r.ReadNullString(); err != nil {
			return err
		}
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
