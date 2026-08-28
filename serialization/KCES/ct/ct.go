package ct

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio"
	"github.com/ugorji/go/codec"
)

// .ct
//
// 概览
//
// .ct 是 KCES 使用的 Content Table 文件约定，外层采用 WfSystem.Serialization.VirtualDirectory 序列化格式
// VirtualDirectory 是应用层归档容器而不是操作系统文件系统，虽然是一个虚拟目录+虚拟文件的形式，但是 .ct 实际上只是一个索引
// 虚拟文件的内容也只是名称和 hash
// VirtualDirectory 本身是通用容器，能够保存子目录和任意不透明字节，catalog 只是 KCES 对这种容器最重要的一种使用约定
//
// 外层结构
//
// 文件头之后连续保存所有 VirtualFile 的原始字节，尾部保存描述目录树及各文件位置和大小的 MessagePack VirtualDirectory
// VirtualFile 没有独立文件头，其 position 是从整个 .ct 文件开头计算并包含八字节文件头的绝对偏移，其 size 使用游戏 C# int 对应的 Int32
// VirtualDirectory 只负责按名称定位不透明字节，不知道也不限制虚拟文件内部是 MessagePack、文本、Unity 数据还是其他格式
// 目录元数据可按 MessagePack LZ4 Block Array 规则压缩，尾部长度只表示压缩后目录元数据 M 的大小而不表示文件数据区 N 或整个 .ct 的大小
// 旧封装使用 0x8e 标记与 UInt32 目录长度，新封装使用 0xff 标记、UInt64 目录长度与固定尾部签名
// 新封装扩展的只是目录元数据长度，VirtualFile.size 在线格式中仍然是 Int32
//
//	[7 bytes]  FileSignature: bb c3 aa 9a a6 4d ad
//	[1 byte]   SerializeType: 8e = 旧 MessagePack 封装，ff = 扩展 MessagePack 封装
//	[N bytes]  各 VirtualFile 的连续原始数据
//	[M bytes]  MessagePack VirtualDirectory，可按 LZ4 Block Array 规则压缩
//	旧封装: [4 bytes] M 的值（Little-Endian UInt32）
//	新封装: [8 bytes] M 的值（Little-Endian UInt64）+ ed fb aa 55
//
// 标准 catalog 内容约定
//
// 标准 KCES catalog .ct 通常在根目录包含名为 "catalog" 的主虚拟文件和若干以扩展名命名的辅助虚拟文件
// "catalog" 的内容是独立使用 LZ4 Block Array MessagePack 编码的 AssetBundleCatalog 或 VirtualAssetCatalog
//
// AssetBundleCatalog.items 是按资源 hash 查询的主加载索引，每项保存 resourceIndex、name 和 hash，但不保存资源字节或 .ct 内偏移
// 游戏通过 Array.BinarySearch 查询 items，因此可用的 AssetBundleCatalog.items 必须按 hash 升序排列
//
// AssetBundleCatalog.extensionList 保存 ExtensionNameList 虚拟文件的名称，常见名称包括 ".tex"、".model" 和 ".menuassets"
// 每个扩展名虚拟文件的内容是独立使用 LZ4 Block Array MessagePack 编码的 ExtensionNameList
//
// ExtensionNameList.data 是按扩展名枚举资源的二级索引，每项再次保存资源 name 和 hash
//
// items 面向已知 hash 的单项定位，ExtensionNameList 面向已知扩展名的批量枚举，两者重复 name 和 hash 是有意保存的不同查询索引
// 游戏通常先从 ExtensionNameList 取得候选资源名，再根据名称 hash 通过 items 定位资源加载位置
// 对于 AssetBundleCatalog，resourceIndex 索引的是 catalog.resourceFileNames 而不是 .ct 的 VirtualFile 表
// resourceFileNames 通常指向配套 .aba 文件，实际纹理、模型和其他 Unity 资源位于 AssetBundle 中而不位于 .ct
// VirtualAssetCatalog 使用同一个 "catalog" 虚拟文件约定但具有不同的 catalog 条目布局，因此 .ct 格式本身并不保证存在配套 .aba
//
//
// 为什么重复保存 name 和 hash
//
// 这里重复的是少量索引元数据而不是实际资源内容，作用类似数据库同时保存主索引和物化二级索引
// 已知资源名时，游戏计算或使用其 hash 并在按 hash 排序的 items 中二分查找，以 O(log n) 复杂度取得包含 resourceIndex 的加载记录
// 只知道扩展名时，调用方尚不知道具体资源名，游戏直接读取对应 ExtensionNameList 并只遍历该组 k 个条目，而不必扫描全部 n 个 items
// 如果只保存 items，每次枚举一种扩展名都必须完整扫描按 hash 而非扩展名排列的 items，并重新解析文件名后缀和处理无扩展名规则
// 如果只保存 ExtensionNameList，精确加载时必须先确定并搜索分组，而且其中没有 resourceIndex，无法确定应从 resourceFileNames 的哪个资源源加载
// 两套索引因此分别解决资源发现和资源定位问题，使按类型批量发现与按 hash 单项加载都具有适合自身访问模式的数据布局
// 独立的扩展名虚拟文件还允许游戏按需读取一个分组，并在合并多个 catalog 时只枚举当前请求的扩展名
// 显式分组保存了生产者定义的分类语义，包括使用 "null" 表示无扩展名资源，而不要求消费者每次从 name 推导分类
// ExtensionNameList 必须重复 name 才能返回可加载的资源名，重复 hash 则使 Pack 保持自包含的 name/hash 固定线格式
// 在当前可见的 KCES 1.34.4 枚举调用中主要使用 Pack.name，没有足够源码证明 Pack.hash 还承担额外运行时查询职责
//
//
// 容易误解的地方
//
// 名为 ".tex" 的虚拟文件不是纹理，名为 ".model" 的虚拟文件也不是模型，它们通常只是相应扩展名的 ExtensionNameList 索引数据
// catalog.extensionList 中的值是同一 VirtualDirectory 内的虚拟文件名，不是普通资源扩展名集合的内联数据
// ExtensionNameList 的游戏字段拼写固定为 extention，查找分组时游戏使用 catalog.extensionList 和外层虚拟文件名而不是依赖该字段
// ExtensionNameList.Pack.hash 与 items 中的 hash 通常重复，当前游戏枚举路径主要消费 Pack.name，但固定两槽线格式仍要求保留 hash
// 无扩展名资源可以使用名为 "null" 的虚拟文件和分组，这里的 "null" 是普通字符串名称而不是 MessagePack nil
// "catalog" 和各 ExtensionNameList 虚拟文件的内容压缩与尾部 VirtualDirectory 元数据压缩是彼此独立的两层编码
// 解码外层 VirtualDirectory 只会得到虚拟文件原始字节，仍需分别解压并反序列化已知的 catalog 和 ExtensionNameList 内容
// 高层 JSON 编辑封套中的 extensionNameLists 是库将这些独立虚拟文件解码后汇总出的视图，不是 catalog 在线格式中的内联字段
// catalog.items 的数量是逻辑资源数量，VirtualDirectory.allFiles 的数量是 .ct 内部索引文件数量，两者通常不同
// extensionList、对应虚拟文件和 ExtensionNameList 内容必须保持一致，否则可能出现能够按 hash 加载却无法按扩展名枚举或能够枚举却无法加载的资源
//
//
// 游戏读取流程
//
//	打开 .ct -> 从尾部定位并解码 VirtualDirectory -> GetFile("catalog") -> 解码具体 catalog
//	按扩展名枚举 -> 在 catalog.extensionList 中匹配名称 -> GetFile(".tex") -> 解码 ExtensionNameList -> 取得资源名
//	加载资源 -> 计算或使用名称 hash -> 在 catalog.items 中定位 -> 使用 resourceIndex 选择 resourceFileNames -> 从对应资源源加载
//
//
//
// .ct
//
// Overview
//
// A .ct is the KCES Content Table file convention whose outer layer uses the serialized WfSystem.Serialization.VirtualDirectory format
// VirtualDirectory is an application-level archive container rather than an operating-system file system, with directories and files represented only by indexes and byte ranges inside one physical file
// It does not store permissions, timestamps, or similar file-system attributes and the game does not mount it as a Windows directory
// VirtualDirectory is a generic container capable of holding child directories and arbitrary opaque bytes, while a catalog is only its most important KCES content convention
//
// Outer layout
//
// Raw bytes for every VirtualFile are stored contiguously after the header and followed by a MessagePack VirtualDirectory describing the directory tree and every file position and size
// A VirtualFile has no individual file header, its position is an absolute offset counted from the beginning of the complete .ct including the eight-byte header, and its size uses the Int32 matching the game C# int
// VirtualDirectory only locates opaque bytes by name and neither knows nor restricts whether a virtual-file payload contains MessagePack, text, Unity data, or another format
// Directory metadata may use MessagePack LZ4 Block Array compression, and the trailing length describes only the compressed directory metadata M rather than file payload area N or the complete .ct size
// The legacy frame uses marker 0x8e and a UInt32 directory length while the extended frame uses marker 0xff, a UInt64 directory length, and a fixed footer signature
// The extended frame widens only the directory metadata length while VirtualFile.size remains an Int32 in the wire format
//
//	[7 bytes]  FileSignature: bb c3 aa 9a a6 4d ad
//	[1 byte]   SerializeType: 8e = legacy MessagePack frame, ff = extended MessagePack frame
//	[N bytes]  Contiguous raw data for every VirtualFile
//	[M bytes]  MessagePack VirtualDirectory, optionally compressed by the LZ4 Block Array rule
//	Legacy: [4 bytes] M encoded as a Little-Endian UInt32
//	Extended: [8 bytes] M encoded as a Little-Endian UInt64 + ed fb aa 55
//
// Standard catalog content convention
//
// A standard KCES catalog .ct usually contains a main virtual file named "catalog" and several extension-named helper virtual files at the root
// The "catalog" payload is an independently LZ4 Block Array MessagePack-encoded AssetBundleCatalog or VirtualAssetCatalog
// AssetBundleCatalog.items is the primary loading index queried by resource hash, with each item storing resourceIndex, name, and hash but no resource bytes or .ct offset
// The game queries items through Array.BinarySearch, so a usable AssetBundleCatalog.items array must be sorted by hash in ascending order
// AssetBundleCatalog.extensionList stores the names of ExtensionNameList virtual files, with common names including ".tex", ".model", and ".menuassets"
// Every extension-named virtual-file payload is an independently LZ4 Block Array MessagePack-encoded ExtensionNameList
// ExtensionNameList.data is the secondary index used to enumerate resources by extension, with every entry storing the resource name and hash again
// Items serve single-item lookup by a known hash while ExtensionNameList serves bulk enumeration by a known extension, so their duplicated names and hashes intentionally support different queries
// The game normally obtains candidate resource names from an ExtensionNameList and then locates each loading target through items using the name hash
// For AssetBundleCatalog, resourceIndex indexes catalog.resourceFileNames rather than the .ct VirtualFile table
// ResourceFileNames usually identifies a paired .aba while actual textures, models, and other Unity resources reside in the AssetBundle rather than the .ct
// VirtualAssetCatalog uses the same "catalog" virtual-file convention with a different catalog item layout, so the .ct format itself does not guarantee a paired .aba
//
// Why name and hash are stored twice
//
// Only small index metadata is duplicated rather than actual resource content, serving the same purpose as a database primary index plus a materialized secondary index
// Given a resource name, the game computes or uses its hash and binary-searches hash-sorted items to obtain the loading record containing resourceIndex in O(log n) time
// Given only an extension, the caller does not yet know any concrete resource name, so the game reads the matching ExtensionNameList and visits only the k entries in that group instead of scanning all n items
// Keeping only items would require every extension enumeration to scan the complete hash-ordered rather than extension-ordered array, parse name suffixes again, and handle extensionless rules
// Keeping only ExtensionNameList would require exact loading to identify and search a group first, and its entries have no resourceIndex with which to choose a resource source from resourceFileNames
// The two indexes therefore solve resource discovery and resource location separately, giving bulk discovery by type and single-item loading by hash a data layout suited to each access pattern
// Separate extension-named virtual files also let the game read one group on demand and enumerate only the requested extension while merging multiple catalogs
// Explicit grouping preserves producer-defined classification semantics including the "null" group for extensionless resources rather than requiring every consumer to derive a group from name
// ExtensionNameList must repeat name to return loadable resource names, while the repeated hash keeps Pack in its self-contained fixed name/hash wire layout
// Visible KCES 1.34.4 enumeration call sites primarily use Pack.name and do not provide enough evidence that Pack.hash has another runtime query responsibility
// A serializer must still preserve Pack.hash for compatibility with the game's fixed two-slot layout and with other versions or tools that may consume it, without inventing an unverified field meaning
// This design trades additional index size and synchronization cost for efficient access in both directions, so packing or editing must update items, extensionList, and ExtensionNameList together
//
// Counterintuitive details
//
// A virtual file named ".tex" is not a texture and one named ".model" is not a model, as these files normally contain only ExtensionNameList index data for the corresponding extension
// Values in catalog.extensionList are virtual-file names inside the same VirtualDirectory rather than inline data for a conventional set of resource extensions
// The game wire field in ExtensionNameList is spelled extention, while group lookup uses catalog.extensionList and the outer virtual-file name rather than relying on that field
// ExtensionNameList.Pack.hash normally duplicates the hash in items, and current game enumeration paths primarily consume Pack.name, but the fixed two-slot wire layout still requires the hash to be preserved
// Extensionless resources may use a virtual file and group named "null", where "null" is an ordinary string name rather than MessagePack nil
// Payload compression for "catalog" and each ExtensionNameList is independent from compression of the trailing VirtualDirectory metadata, forming two separate encoding layers
// Decoding the outer VirtualDirectory produces only raw virtual-file bytes, so known catalog and ExtensionNameList payloads still require their own decompression and deserialization
// The extensionNameLists property in the high-level JSON editing envelope is a view assembled by decoding these separate virtual files rather than an inline catalog wire field
// The number of catalog.items is the number of logical resources while the number of VirtualDirectory.allFiles is the number of internal index files, and these counts normally differ
// ExtensionList, its corresponding virtual files, and each ExtensionNameList payload must remain consistent or a resource may be loadable by hash but absent from extension enumeration, or enumerable but not loadable
//
// Extraction semantics
//
// Extracting a .ct to a disk directory only materializes the logical VirtualDirectory tree and opaque VirtualFile bytes
// Raw extraction of a standard catalog .ct normally produces compressed MessagePack index files such as "catalog", ".tex", and ".model" rather than the actual resources described by items
// Inspecting index semantics requires further catalog and ExtensionNameList decoding, while extracting resources described by an AssetBundleCatalog also requires the .aba identified by resourceFileNames
// VirtualDirectory can contain real child-directory structure, although official catalog .ct files normally use a flat layout containing only root-level virtual files
//
// Game read flow
//
//	Open .ct -> locate and decode VirtualDirectory from the footer -> GetFile("catalog") -> decode the concrete catalog
//	Enumerate by extension -> match a name in catalog.extensionList -> GetFile(".tex") -> decode ExtensionNameList -> obtain resource names
//	Load a resource -> compute or use the name hash -> locate it in catalog.items -> select resourceFileNames through resourceIndex -> load from the corresponding resource source

// FileSignature 是 .ct 文件的魔数签名（7 字节），用于验证文件格式
// 对应 C# VirtualDirectory.FileSignature = {0xbb, 0xc3, 0xaa, 0x9a, 0xa6, 0x4d, 0xad}
// FileSignature is the 7-byte magic signature used to validate .ct files
// It matches C# VirtualDirectory.FileSignature = {0xbb, 0xc3, 0xaa, 0x9a, 0xa6, 0x4d, 0xad}
var FileSignature = []byte{0xbb, 0xc3, 0xaa, 0x9a, 0xa6, 0x4d, 0xad}

const (
	// HeaderSize 是文件头大小，由 7 字节签名和 1 字节序列化类型组成
	// HeaderSize is the header width made up of a 7-byte signature and a 1-byte serialization type
	HeaderSize = 8
	// SerializeTypeMsgPack 表示带 UInt32 长度尾部的旧 MessagePack 封装
	// SerializeTypeMsgPack selects the legacy MessagePack frame with a trailing UInt32 length
	SerializeTypeMsgPack byte = 0x8e
	// SerializeTypeMsgPackExtended 表示带 UInt64 长度与固定签名尾部的扩展 MessagePack 封装
	// SerializeTypeMsgPackExtended selects the extended MessagePack frame with a trailing UInt64 length and fixed signature
	SerializeTypeMsgPackExtended byte = 0xff
	// legacyFooterSize 是旧封装末尾 MessagePack UInt32 长度的字节数
	// legacyFooterSize is the width of the trailing MessagePack UInt32 length in the legacy frame
	legacyFooterSize = 4
	// extendedFooterSize 是扩展封装 UInt64 长度和四字节签名的总宽度
	// extendedFooterSize is the combined width of the UInt64 length and four-byte signature in the extended frame
	extendedFooterSize = 12
	// ctVersion 是 VirtualDirectory 的固定版本号，对应 C# VirtualDirectory.FixVersion = 1000
	// ctVersion is the fixed VirtualDirectory version matching C# VirtualDirectory.FixVersion = 1000
	ctVersion = 1000
)

// extendedFooterMagic 是扩展 VirtualDirectory 封装固定使用的尾部签名
// extendedFooterMagic is the fixed trailing signature used by the extended VirtualDirectory frame
var extendedFooterMagic = [...]byte{0xed, 0xfb, 0xaa, 0x55}

// VirtualDirectoryFraming 标识 MessagePack VirtualDirectory 元数据采用的外层尾部封装 / VirtualDirectoryFraming identifies the outer footer frame used around MessagePack VirtualDirectory metadata
type VirtualDirectoryFraming uint8

const (
	// VirtualDirectoryFramingLegacy 使用 0x8e 标记和四字节长度尾部
	// VirtualDirectoryFramingLegacy uses marker 0x8e and a four-byte length footer
	VirtualDirectoryFramingLegacy VirtualDirectoryFraming = iota
	// VirtualDirectoryFramingExtended 使用 0xff 标记、八字节长度及固定签名尾部
	// VirtualDirectoryFramingExtended uses marker 0xff, an eight-byte length, and a fixed footer signature
	VirtualDirectoryFramingExtended
)

// ContentTable 表示解析后的 .ct 文件 VirtualDirectory 序列化结构
// .ct 文件是 KCES 游戏的资源目录容器，内部存储 catalog、ExtensionNameList 等虚拟文件
// 游戏通过 CatalogUtility.FromCatalog<T> 读取 .ct 中的 "catalog" 文件获取资源索引
// ContentTable represents a parsed .ct file in serialized VirtualDirectory form
// A .ct file is a KCES resource catalog container storing virtual files such as catalog and ExtensionNameList
// The game reads the "catalog" virtual file through CatalogUtility.FromCatalog<T> to obtain resource indexes
type ContentTable struct {
	Version     int32                               `json:"Version"`               // 保存的根 VirtualDirectory 版本 / Stored root VirtualDirectory version
	Framing     VirtualDirectoryFraming             `json:"Framing,omitempty"`     // MessagePack 目录的外层尾部封装 / Outer footer frame around the MessagePack directory
	Directories map[string]VirtualDirectoryMetadata `json:"Directories,omitempty"` // 子目录路径及其真实版本字段，包含空目录 / Child-directory paths and their real version fields, including empty directories
	Files       map[string]VirtualFile              `json:"Files"`                 // 虚拟文件表，键为文件名，值为位置和大小 / Virtual file table keyed by file name with position and size values
	Raw         []byte                              `json:"-"`                     // 完整文件原始字节，用于按偏移提取虚拟文件内容 / Raw bytes of the full file used to slice virtual file contents by offset
	dataEnd     int64                               `json:"-"`                     // 实际虚拟文件数据区末尾，零值表示使用 len(Raw) / End of the virtual-file payload area; zero means len(Raw)
}

// VirtualDirectoryMetadata 保存子 VirtualDirectory 的真实字段
// 路径在 ContentTable.Directories 中以规范化的斜杠分隔键保存，解码得到的每个目录都会有条目
// VirtualDirectoryMetadata stores the real fields of a child VirtualDirectory
// Paths use canonical slash-separated keys in ContentTable.Directories and every decoded directory has an entry
type VirtualDirectoryMetadata struct {
	Version int32 `json:"Version"` // 子 VirtualDirectory 的版本值 / Child VirtualDirectory version value
}

// VirtualFile 表示虚拟文件系统中的一个文件条目
// 对应 C# VirtualFile 的 MessagePack indexed array [Key(0)=position, Key(1)=size]
// VirtualFile represents one file entry in the virtual file system
// It matches the C# VirtualFile MessagePack indexed array [Key(0)=position, Key(1)=size]
type VirtualFile struct {
	Position int64 `json:"Position"` // 文件数据在 .ct 文件中的绝对字节偏移，从文件开头计算且包含 header / Absolute byte offset of file data inside the .ct file, counted from file start including header
	Size     int32 `json:"Size"`     // 文件数据的字节大小 / File data size in bytes
}

// ReadContentTable 从 reader 中读取并解析 .ct 文件
// 解析流程：验证签名、读取末尾长度、提取 MessagePack 数据、LZ4 解压并解码 VirtualDirectory
// ReadContentTable reads and parses a .ct file from reader
// The procedure validates the signature, reads the trailing length, extracts MessagePack data, decompresses LZ4, and decodes VirtualDirectory
func ReadContentTable(r io.Reader) (*ContentTable, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read .ct file failed: %w", err)
	}

	if len(data) < HeaderSize {
		return nil, fmt.Errorf("file too small: %d bytes", len(data))
	}

	for i, b := range FileSignature {
		if data[i] != b {
			return nil, fmt.Errorf("invalid file signature at byte %d: got 0x%02x, want 0x%02x", i, data[i], b)
		}
	}

	framing, msgpackStart, msgpackEnd, err := readVirtualDirectoryMetadataRange(data)
	if err != nil {
		return nil, err
	}
	msgpackData := data[msgpackStart:msgpackEnd]

	decompressed, err := msgpack.DecompressLz4BlockArray(msgpackData)
	if err != nil {
		return nil, fmt.Errorf("decompress VirtualDirectory failed: %w", err)
	}

	ct := &ContentTable{Framing: framing, Raw: data, dataEnd: msgpackStart}
	if err := ct.decodeVirtualDirectory(decompressed); err != nil {
		return nil, fmt.Errorf("decode VirtualDirectory failed: %w", err)
	}
	for name := range ct.Files {
		vf := ct.Files[name]
		if vf.Size > 0 && vf.Position < HeaderSize {
			return nil, fmt.Errorf("virtual file %q starts inside the .ct header at %d", name, vf.Position)
		}
		if _, err := ct.GetFileData(name); err != nil {
			return nil, err
		}
	}

	return ct, nil
}

// readVirtualDirectoryMetadataRange 验证序列化类型及对应尾部，并返回 MessagePack 元数据的半开区间
// readVirtualDirectoryMetadataRange validates the serialize type and matching footer and returns the half-open MessagePack metadata range
func readVirtualDirectoryMetadataRange(data []byte) (VirtualDirectoryFraming, int64, int64, error) {
	dataLength := int64(len(data))
	switch data[7] {
	case SerializeTypeMsgPack:
		if len(data) < HeaderSize+legacyFooterSize+1 {
			return 0, 0, 0, fmt.Errorf("legacy MessagePack VirtualDirectory file too small: %d bytes", len(data))
		}
		metadataEnd := dataLength - legacyFooterSize
		metadataSize := int64(binary.LittleEndian.Uint32(data[metadataEnd:]))
		if metadataSize <= 0 || metadataSize > metadataEnd-HeaderSize {
			return 0, 0, 0, fmt.Errorf("invalid legacy msgpack size: %d (file size: %d)", metadataSize, len(data))
		}
		return VirtualDirectoryFramingLegacy, metadataEnd - metadataSize, metadataEnd, nil
	case SerializeTypeMsgPackExtended:
		if len(data) < HeaderSize+extendedFooterSize+1 {
			return 0, 0, 0, fmt.Errorf("extended MessagePack VirtualDirectory file too small: %d bytes", len(data))
		}
		magicStart := len(data) - len(extendedFooterMagic)
		if !bytes.Equal(data[magicStart:], extendedFooterMagic[:]) {
			return 0, 0, 0, fmt.Errorf("invalid extended VirtualDirectory footer signature: got % x, want % x", data[magicStart:], extendedFooterMagic)
		}
		metadataEnd := dataLength - extendedFooterSize
		metadataSize := binary.LittleEndian.Uint64(data[metadataEnd : dataLength-int64(len(extendedFooterMagic))])
		maximumSize := uint64(metadataEnd - HeaderSize)
		if metadataSize == 0 || metadataSize > maximumSize {
			return 0, 0, 0, fmt.Errorf("invalid extended msgpack size: %d (file size: %d)", metadataSize, len(data))
		}
		return VirtualDirectoryFramingExtended, metadataEnd - int64(metadataSize), metadataEnd, nil
	default:
		return 0, 0, 0, fmt.Errorf("unsupported serialize type: 0x%02x (supported MessagePack markers are 0x%02x and 0x%02x)", data[7], SerializeTypeMsgPack, SerializeTypeMsgPackExtended)
	}
}

// WriteContentTable 将 ContentTable 序列化为 .ct 格式并写入 writer
// 写入顺序为签名、序列化类型、各文件原始数据、LZ4 压缩的 VirtualDirectory 和长度尾部
// WriteContentTable serializes ContentTable in .ct format and writes it to writer
// The write order is the signature, serialization type, raw file data, LZ4-compressed VirtualDirectory, and trailing length
func WriteContentTable(w io.Writer, ct *ContentTable) error {
	if w == nil {
		return fmt.Errorf("nil content table writer")
	}
	if ct == nil {
		return fmt.Errorf("nil content table")
	}
	serializeType := SerializeTypeMsgPack
	switch ct.Framing {
	case VirtualDirectoryFramingLegacy:
	case VirtualDirectoryFramingExtended:
		serializeType = SerializeTypeMsgPackExtended
	default:
		return fmt.Errorf("unsupported VirtualDirectory framing %d", ct.Framing)
	}
	type fileEntry struct {
		name string      // 虚拟文件名 / Virtual file name
		file VirtualFile // VirtualFile 线格式形状元数据 / VirtualFile wire-shape metadata
		data []byte      // 虚拟文件数据 / Virtual file data
	}
	names := make([]string, 0, len(ct.Files))
	for name := range ct.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]fileEntry, 0, len(names))
	canonicalNames := make(map[string]string, len(names))
	for _, name := range names {
		file := ct.Files[name]
		canonicalName, err := canonicalVirtualPath(name)
		if err != nil {
			return fmt.Errorf("invalid virtual file name %q: %w", name, err)
		}
		if previous, exists := canonicalNames[canonicalName]; exists {
			return fmt.Errorf("virtual file names %q and %q normalize to the same path %q", previous, name, canonicalName)
		}
		canonicalNames[canonicalName] = name
		fileData, err := ct.GetFileData(name)
		if err != nil {
			return fmt.Errorf("read file data %q for writing: %w", name, err)
		}
		entries = append(entries, fileEntry{name: canonicalName, file: file, data: fileData})
	}

	updatedFiles := make(map[string]VirtualFile, len(ct.Files))
	var offset int64 = HeaderSize
	for _, entry := range entries {
		fileSize, err := checkedVirtualFileSize(entry.name, int64(len(entry.data)))
		if err != nil {
			return err
		}
		updated := entry.file
		updated.Position = offset
		updated.Size = fileSize
		updatedFiles[entry.name] = updated
		offset += int64(len(entry.data))
	}

	rootMetadata := VirtualDirectoryMetadata{Version: ct.Version}
	dirArray, err := encodeVirtualDirectoryTree(rootMetadata, ct.Directories, updatedFiles)
	if err != nil {
		return err
	}

	msgpackData, err := msgpack.EncodeMsgpack(dirArray)
	if err != nil {
		return fmt.Errorf("msgpack encode VirtualDirectory failed: %w", err)
	}

	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		return fmt.Errorf("compress VirtualDirectory failed: %w", err)
	}
	footer, err := encodeVirtualDirectoryMetadataFooter(ct.Framing, uint64(len(compressed)))
	if err != nil {
		return err
	}

	// 所有模型和封装校验都在首次写入前完成，被拒绝的元数据形状不会留下部分写入的输出流
	// All model and framing validation is complete before the first write, so rejected metadata cannot leave a partially written output stream
	if err := binaryio.WriteBytes(w, FileSignature); err != nil {
		return fmt.Errorf("write file signature failed: %w", err)
	}
	if err := binaryio.WriteByte(w, serializeType); err != nil {
		return fmt.Errorf("write serialize type failed: %w", err)
	}
	for _, entry := range entries {
		if err := binaryio.WriteBytes(w, entry.data); err != nil {
			return fmt.Errorf("write file data %q failed: %w", entry.name, err)
		}
	}

	if err := binaryio.WriteBytes(w, compressed); err != nil {
		return fmt.Errorf("write msgpack data failed: %w", err)
	}

	if err := binaryio.WriteBytes(w, footer); err != nil {
		return fmt.Errorf("write msgpack size failed: %w", err)
	}

	return nil
}

// encodeVirtualDirectoryMetadataFooter 为所选封装创建经过范围检查的元数据长度尾部
// encodeVirtualDirectoryMetadataFooter creates a range-checked metadata-length footer for the selected frame
func encodeVirtualDirectoryMetadataFooter(framing VirtualDirectoryFraming, metadataSize uint64) ([]byte, error) {
	if metadataSize == 0 {
		return nil, fmt.Errorf("compressed VirtualDirectory must not be empty")
	}
	switch framing {
	case VirtualDirectoryFramingLegacy:
		if metadataSize > math.MaxUint32 {
			return nil, fmt.Errorf("compressed VirtualDirectory is too large for legacy framing: %d bytes", metadataSize)
		}
		footer := make([]byte, legacyFooterSize)
		binary.LittleEndian.PutUint32(footer, uint32(metadataSize))
		return footer, nil
	case VirtualDirectoryFramingExtended:
		footer := make([]byte, extendedFooterSize)
		binary.LittleEndian.PutUint64(footer, metadataSize)
		copy(footer[extendedFooterSize-len(extendedFooterMagic):], extendedFooterMagic[:])
		return footer, nil
	default:
		return nil, fmt.Errorf("unsupported VirtualDirectory framing %d", framing)
	}
}

// NewContentTableFromDir 从磁盘目录创建 ContentTable，将目录中所有文件作为虚拟文件
// 文件路径相对于 dirPath，并使用正斜杠分隔
// NewContentTableFromDir creates a ContentTable from a disk directory and treats every file as a virtual file
// File paths are relative to dirPath and use slash separators
func NewContentTableFromDir(dirPath string) (*ContentTable, error) {
	ct := &ContentTable{
		Version: ctVersion,
		Framing: VirtualDirectoryFramingLegacy,
		Files:   make(map[string]VirtualFile),
	}

	var rawBuf []byte
	var offset int64 = HeaderSize

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("get relative path failed: %w", err)
		}
		relPath = filepath.ToSlash(relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file %q failed: %w", relPath, err)
		}
		fileSize, err := checkedVirtualFileSize(relPath, int64(len(data)))
		if err != nil {
			return err
		}

		ct.Files[relPath] = VirtualFile{Position: offset, Size: fileSize}
		rawBuf = append(rawBuf, data...)
		offset += int64(len(data))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory failed: %w", err)
	}

	ct.Raw = make([]byte, HeaderSize+len(rawBuf))
	copy(ct.Raw[:7], FileSignature)
	ct.Raw[7] = SerializeTypeMsgPack
	copy(ct.Raw[HeaderSize:], rawBuf)

	return ct, nil
}

// GetFileData 根据虚拟文件名提取原始字节数据
// 函数使用 Files 表中的 Position 和 Size 在 Raw 中切片返回
// GetFileData extracts raw bytes for a virtual file name
// It slices Raw using Position and Size from the Files table
func (ct *ContentTable) GetFileData(name string) ([]byte, error) {
	if ct == nil {
		return nil, fmt.Errorf("nil content table")
	}
	vf, ok := ct.Files[name]
	if !ok {
		return nil, fmt.Errorf("file %q not found in content table", name)
	}

	dataLen := int64(len(ct.Raw))
	if ct.dataEnd > 0 && ct.dataEnd < dataLen {
		dataLen = ct.dataEnd
	}
	if vf.Position < 0 || vf.Position > dataLen || vf.Size < 0 || int64(vf.Size) > dataLen-vf.Position {
		return nil, fmt.Errorf("file %q out of bounds: position=%d size=%d in data of %d bytes", name, vf.Position, vf.Size, len(ct.Raw))
	}
	start := vf.Position
	end := start + int64(vf.Size)
	return ct.Raw[start:end], nil
}

// GetFileNames 返回按字典序排列的所有虚拟文件名
// GetFileNames returns all virtual file names in lexicographic order
func (ct *ContentTable) GetFileNames() []string {
	if ct == nil {
		return nil
	}
	names := make([]string, 0, len(ct.Files))
	for name := range ct.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetVirtualDirectoryMetadata 返回子目录真实字段的深拷贝
// GetVirtualDirectoryMetadata returns a deep copy of the real child-directory fields
func (ct *ContentTable) GetVirtualDirectoryMetadata() map[string]VirtualDirectoryMetadata {
	if ct == nil || len(ct.Directories) == 0 {
		return nil
	}
	result := make(map[string]VirtualDirectoryMetadata, len(ct.Directories))
	for path, metadata := range ct.Directories {
		result[path] = metadata
	}
	return result
}

// AddFile 向 ContentTable 追加一个虚拟文件
// 数据会追加到 Raw 的载荷区末尾，并自动更新 Position 和 Size
// AddFile appends a virtual file to ContentTable
// Data is appended to the end of the Raw payload area and Position and Size are updated automatically
func (ct *ContentTable) AddFile(name string, data []byte) error {
	if ct == nil {
		return fmt.Errorf("nil content table")
	}
	fileSize, err := checkedVirtualFileSize(name, int64(len(data)))
	if err != nil {
		return err
	}
	if ct.Files == nil {
		ct.Files = make(map[string]VirtualFile)
	}
	// ReadContentTable 返回的表会在 Raw 中保留序列化目录与尾部，dataEnd 标记虚拟文件数据终点
	// 追加前先分离该尾部，否则新位置会落入元数据或其后并被 GetFileData 拒绝；载荷前缀不变，因此已有文件偏移仍然有效
	// A table returned by ReadContentTable retains the serialized directory and footer in Raw, with dataEnd marking the end of virtual-file data
	// The tail is detached before appending so the new position does not point into or beyond metadata; existing offsets remain valid because the payload prefix is unchanged
	if ct.dataEnd > 0 && ct.dataEnd <= int64(len(ct.Raw)) {
		ct.Raw = append([]byte(nil), ct.Raw[:ct.dataEnd]...)
		ct.dataEnd = 0
	}
	position := int64(len(ct.Raw))
	ct.Raw = append(ct.Raw, data...)
	ct.Files[name] = VirtualFile{Position: position, Size: fileSize}
	return nil
}

// checkedVirtualFileSize 将内存或文件系统长度安全窄化为游戏 VirtualFile 使用的 C# Int32。
// checkedVirtualFileSize safely narrows an in-memory or filesystem length to the C# Int32 used by the game's VirtualFile.
func checkedVirtualFileSize(name string, length int64) (int32, error) {
	if length < 0 || length > math.MaxInt32 {
		return 0, fmt.Errorf("virtual file %q size %d is outside the C# Int32 range", name, length)
	}
	return int32(length), nil
}

// DecodeMsgpackFile 提取虚拟文件并解码 MessagePack，同时自动处理 Lz4BlockArray 压缩
// 适用于读取 catalog、ExtensionNameList 等 MessagePack 序列化文件
// DecodeMsgpackFile extracts a virtual file and decodes MessagePack while handling Lz4BlockArray compression automatically
// It is suitable for MessagePack files such as catalog and ExtensionNameList
func (ct *ContentTable) DecodeMsgpackFile(name string, out interface{}) error {
	raw, err := ct.GetFileData(name)
	if err != nil {
		return err
	}

	decoded, err := msgpack.DecompressLz4BlockArray(raw)
	if err != nil {
		return fmt.Errorf("decompress content table file %q: %w", name, err)
	}

	return msgpack.DecodeMsgpack(decoded, out)
}

// decodeVirtualDirectory 解码一个固定三槽 MessagePack VirtualDirectory
// decodeVirtualDirectory decodes one fixed three-slot MessagePack VirtualDirectory
func (ct *ContentTable) decodeVirtualDirectory(data []byte) error {
	root, err := decodeRawMsgpackArray(data, "VirtualDirectory root")
	if err != nil {
		return err
	}
	if len(root) != 3 {
		return fmt.Errorf("unsupported VirtualDirectory root indexed-array width %d, expected 3", len(root))
	}
	ct.Version = 0
	ct.Directories = make(map[string]VirtualDirectoryMetadata)
	ct.Files = make(map[string]VirtualFile)
	return ct.extractDirectoryFilesRaw(root, "")
}

// extractDirectoryFilesRaw 从一个 VirtualDirectory indexed array 递归提取目录和文件
// extractDirectoryFilesRaw recursively extracts directories and files from one VirtualDirectory indexed array
func (ct *ContentTable) extractDirectoryFilesRaw(arr []codec.Raw, prefix string) error {
	if len(arr) != 3 {
		return fmt.Errorf("unsupported VirtualDirectory %q indexed-array width %d, expected 3", prefix, len(arr))
	}
	var rawVersion interface{}
	if err := decodeSingleRawMsgpackValue(arr[0], &rawVersion, fmt.Sprintf("VirtualDirectory %q version", prefix)); err != nil {
		return err
	}
	version, ok := toInt32(rawVersion)
	if !ok {
		return fmt.Errorf("VirtualDirectory %q version must be an Int32 MessagePack integer, got %T", prefix, rawVersion)
	}
	metadata := VirtualDirectoryMetadata{Version: version}
	directories, directoriesNil, err := decodeVirtualDirectoryRawMap(arr[1], fmt.Sprintf("VirtualDirectory %q allDirectorys", prefix))
	if err != nil {
		return err
	}
	if directoriesNil {
		return fmt.Errorf("VirtualDirectory %q allDirectorys must not be nil", prefix)
	}
	files, filesNil, err := decodeVirtualDirectoryRawMap(arr[2], fmt.Sprintf("VirtualDirectory %q allFiles", prefix))
	if err != nil {
		return err
	}
	if filesNil {
		return fmt.Errorf("VirtualDirectory %q allFiles must not be nil", prefix)
	}

	if prefix == "" {
		ct.Version = metadata.Version
	} else {
		if _, exists := ct.Directories[prefix]; exists {
			return fmt.Errorf("duplicate VirtualDirectory path %q", prefix)
		}
		ct.Directories[prefix] = metadata
	}

	for name, child := range directories {
		component, err := canonicalVirtualComponent(name)
		if err != nil {
			return fmt.Errorf("VirtualDirectory %q child %q: %w", prefix, name, err)
		}
		childPrefix := component
		if prefix != "" {
			childPrefix = prefix + "/" + component
		}
		childArray, err := decodeRawMsgpackArray(child, fmt.Sprintf("VirtualDirectory %q", childPrefix))
		if err != nil {
			return err
		}
		if err := ct.extractDirectoryFilesRaw(childArray, childPrefix); err != nil {
			return err
		}
	}

	for name, rawFile := range files {
		component, err := canonicalVirtualComponent(name)
		if err != nil {
			return fmt.Errorf("VirtualDirectory %q file %q: %w", prefix, name, err)
		}
		fullName := component
		if prefix != "" {
			fullName = prefix + "/" + component
		}
		file, err := decodeVirtualFileRaw(rawFile)
		if err != nil {
			return fmt.Errorf("VirtualDirectory file %q: %w", fullName, err)
		}
		if _, exists := ct.Files[fullName]; exists {
			return fmt.Errorf("duplicate VirtualDirectory file path %q", fullName)
		}
		ct.Files[fullName] = file
	}
	return nil
}

// decodeRawMsgpackArray 将单个 MessagePack 数组解码为保留原始编码的槽位
// decodeRawMsgpackArray decodes one MessagePack array into slots that retain their raw encoding
func decodeRawMsgpackArray(data []byte, label string) ([]codec.Raw, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%s: empty MessagePack value", label)
	}
	pos := int64(0)
	count, err := msgpack.ReadArrayHeaderStrict(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if count > int64(len(data))-pos {
		return nil, fmt.Errorf("%s array length %d exceeds the capacity of %d remaining bytes", label, count, int64(len(data))-pos)
	}
	var values []codec.Raw
	if err := decodeSingleRawMsgpackValue(data, &values, label); err != nil {
		return nil, err
	}
	if int64(len(values)) != count {
		return nil, fmt.Errorf("%s decoded %d fields, header declares %d", label, len(values), count)
	}
	return values, nil
}

// decodeVirtualDirectoryRawMap 解码 allDirectorys 或 allFiles 映射，并单独报告其是否为 nil
// decodeVirtualDirectoryRawMap decodes an allDirectorys or allFiles map and separately reports whether it was nil
func decodeVirtualDirectoryRawMap(data []byte, label string) (map[string]codec.Raw, bool, error) {
	// ugorji 将 codec.Raw 中解码得到的 MessagePack nil 表示为 nil 切片，而非单字节 0xc0
	// 调用方会另行跟踪槽位是否存在，因此两种形式在此都明确表示一个存在但值为 nil 的映射
	// ugorji represents a decoded MessagePack nil held in codec.Raw as a nil slice rather than the one-byte 0xc0 span
	// The caller tracks slot presence separately, so both forms unambiguously mean a present nil map here
	if len(data) == 0 || (len(data) == 1 && data[0] == 0xc0) {
		return nil, true, nil
	}
	position := int64(0)
	count, err := msgpack.ReadMapHeader(data, &position)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", label, err)
	}
	headerSize := position
	if count > (int64(len(data))-headerSize)/2 {
		return nil, false, fmt.Errorf("%s map length %d exceeds the capacity of %d remaining bytes", label, count, int64(len(data))-headerSize)
	}
	var generic map[interface{}]codec.Raw
	if err := decodeSingleRawMsgpackValue(data, &generic, label); err != nil {
		return nil, false, err
	}
	if int64(len(generic)) != count {
		return nil, false, fmt.Errorf("%s contains duplicate string keys that the Go map model cannot preserve", label)
	}
	values := make(map[string]codec.Raw, len(generic))
	for key, value := range generic {
		stringKey, ok := key.(string)
		if !ok {
			return nil, false, fmt.Errorf("%s map key must be string, got %T", label, key)
		}
		values[stringKey] = value
	}
	return values, false, nil
}

// decodeSingleRawMsgpackValue 解码恰好一个原始 MessagePack 值，并拒绝尾随字节
// decodeSingleRawMsgpackValue decodes exactly one raw MessagePack value and rejects trailing bytes
func decodeSingleRawMsgpackValue(data []byte, out interface{}, label string) error {
	if len(data) == 0 {
		data = []byte{0xc0}
	}
	if err := msgpack.DecodeMsgpack(data, out); err != nil {
		return fmt.Errorf("%s MessagePack decode failed: %w", label, err)
	}
	return nil
}

// decodeVirtualFileRaw 解码固定两槽 VirtualFile indexed array
// decodeVirtualFileRaw decodes the fixed two-slot VirtualFile indexed array
func decodeVirtualFileRaw(data []byte) (VirtualFile, error) {
	fields, err := decodeRawMsgpackArray(data, "VirtualFile")
	if err != nil {
		return VirtualFile{}, err
	}
	if len(fields) != 2 {
		return VirtualFile{}, fmt.Errorf("unsupported VirtualFile indexed-array width %d, expected 2", len(fields))
	}
	file := VirtualFile{}
	var position interface{}
	if err := decodeSingleRawMsgpackValue(fields[0], &position, "VirtualFile.position"); err != nil {
		return VirtualFile{}, err
	}
	var ok bool
	file.Position, ok = toInt64(position)
	if !ok {
		return VirtualFile{}, fmt.Errorf("position/size: position must be an Int64-compatible MessagePack integer, got %T", position)
	}
	var size interface{}
	if err := decodeSingleRawMsgpackValue(fields[1], &size, "VirtualFile.size"); err != nil {
		return VirtualFile{}, err
	}
	file.Size, ok = toInt32(size)
	if !ok {
		if isIntegerValue(size) {
			return VirtualFile{}, fmt.Errorf("position/size: size is outside the Int32 range")
		}
		return VirtualFile{}, fmt.Errorf("position/size: size must be an Int32 MessagePack integer, got %T", size)
	}
	return file, nil
}

// decodeVirtualFile 将 MessagePack 解码后的 indexed array [position, size] 转为 VirtualFile
// decodeVirtualFile converts a decoded MessagePack indexed array [position, size] to VirtualFile
func decodeVirtualFile(val interface{}) (VirtualFile, error) {
	arr, ok := val.([]interface{})
	if !ok || len(arr) != 2 {
		return VirtualFile{}, fmt.Errorf("VirtualFile: expected array(2), got %T", val)
	}

	pos, ok1 := toInt64(arr[0])
	size, ok2 := toInt32(arr[1])
	if !ok1 {
		return VirtualFile{}, fmt.Errorf("VirtualFile position/size: position expected Int64 integer, got %T", arr[0])
	}
	if !ok2 {
		if isIntegerValue(arr[1]) {
			return VirtualFile{}, fmt.Errorf("VirtualFile position/size: size is outside the Int32 range")
		}
		return VirtualFile{}, fmt.Errorf("VirtualFile position/size: size expected Int32 integer, got %T", arr[1])
	}

	return VirtualFile{Position: pos, Size: size}, nil
}

// virtualDirNode 是将扁平路径重建为 VirtualDirectory 层级时使用的内部节点
// virtualDirNode is an internal node used to rebuild the VirtualDirectory hierarchy from flattened paths
type virtualDirNode struct {
	dirs     map[string]*virtualDirNode // 按当前层名称索引的子目录 / Child directories keyed by their names at this level
	files    map[string]VirtualFile     // 当前目录中的虚拟文件 / Virtual files in the current directory
	metadata *VirtualDirectoryMetadata  // 当前目录显式保留的线格式元数据 / Explicit wire metadata retained for the current directory
}

// encodeVirtualDirectoryTree 从扁平目录元数据和文件路径构建可编码的 VirtualDirectory 树
// encodeVirtualDirectoryTree builds an encodable VirtualDirectory tree from flattened directory metadata and file paths
func encodeVirtualDirectoryTree(rootMetadata VirtualDirectoryMetadata, metadata map[string]VirtualDirectoryMetadata, files map[string]VirtualFile) ([]interface{}, error) {
	root := &virtualDirNode{
		dirs:     map[string]*virtualDirNode{},
		files:    map[string]VirtualFile{},
		metadata: &rootMetadata,
	}
	canonicalMetadata := make(map[string]string, len(metadata))
	for path, value := range metadata {
		canonicalPath, err := canonicalVirtualPath(path)
		if err != nil {
			return nil, fmt.Errorf("invalid VirtualDirectory metadata path %q: %w", path, err)
		}
		if previous, exists := canonicalMetadata[canonicalPath]; exists {
			return nil, fmt.Errorf("VirtualDirectory metadata paths %q and %q normalize to %q", previous, path, canonicalPath)
		}
		canonicalMetadata[canonicalPath] = path
		node := ensureVirtualDirNode(root, splitVirtualPath(canonicalPath))
		copyValue := value
		node.metadata = &copyValue
	}
	for name, vf := range files {
		parts := splitVirtualPath(name)
		if len(parts) == 0 {
			return nil, fmt.Errorf("virtual file path %q has no components", name)
		}
		node := ensureVirtualDirNode(root, parts[:len(parts)-1])
		node.files[parts[len(parts)-1]] = vf
	}
	encoded, err := encodeVirtualDirNode(root, rootMetadata, "")
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

// ensureVirtualDirNode 确保路径各层节点存在并返回末级目录
// ensureVirtualDirNode ensures every node on a path exists and returns the final directory
func ensureVirtualDirNode(root *virtualDirNode, parts []string) *virtualDirNode {
	node := root
	for _, part := range parts {
		child := node.dirs[part]
		if child == nil {
			child = &virtualDirNode{dirs: map[string]*virtualDirNode{}, files: map[string]VirtualFile{}}
			node.dirs[part] = child
		}
		node = child
	}
	return node
}

// encodeVirtualDirNode 按游戏当前固定布局递归编码一个 VirtualDirectory 节点
// encodeVirtualDirNode recursively encodes a VirtualDirectory node using the game's current fixed layout
func encodeVirtualDirNode(node *virtualDirNode, inherited VirtualDirectoryMetadata, path string) ([]interface{}, error) {
	metadata := inherited
	if node.metadata != nil {
		metadata = *node.metadata
	}
	label := path
	if label == "" {
		label = "<root>"
	}
	dirs := make(map[string]interface{}, len(node.dirs))
	for name, child := range node.dirs {
		childPath := name
		if path != "" {
			childPath = path + "/" + name
		}
		childDefault := VirtualDirectoryMetadata{Version: metadata.Version}
		encoded, err := encodeVirtualDirNode(child, childDefault, childPath)
		if err != nil {
			return nil, err
		}
		dirs[name] = encoded
	}
	files, err := encodeVirtualFilesMap(node.files, label)
	if err != nil {
		return nil, err
	}

	return []interface{}{metadata.Version, dirs, files}, nil
}

// encodeVirtualFilesMap 编码一个 VirtualDirectory 的 allFiles 映射
// encodeVirtualFilesMap encodes the allFiles map of one VirtualDirectory
func encodeVirtualFilesMap(files map[string]VirtualFile, directoryLabel string) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(files))
	for name, file := range files {
		encoded, err := encodeVirtualFile(file, fmt.Sprintf("VirtualDirectory %q file %q", directoryLabel, name))
		if err != nil {
			return nil, err
		}
		result[name] = encoded
	}
	return result, nil
}

// encodeVirtualFile 按固定两槽布局编码 VirtualFile
// encodeVirtualFile encodes a VirtualFile using the fixed two-slot layout
func encodeVirtualFile(file VirtualFile, label string) ([]interface{}, error) {
	if file.Position < 0 {
		return nil, fmt.Errorf("%s position must be non-negative", label)
	}
	if file.Size < 0 {
		return nil, fmt.Errorf("%s size must be non-negative", label)
	}
	return []interface{}{file.Position, file.Size}, nil
}

// splitVirtualPath 将虚拟路径拆分为非空且非点号的组成部分
// splitVirtualPath splits a virtual path into non-empty, non-dot components
func splitVirtualPath(name string) []string {
	cleaned := strings.ReplaceAll(filepath.ToSlash(name), "\\", "/")
	raw := strings.Split(cleaned, "/")
	parts := raw[:0]
	for _, part := range raw {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

// joinVirtualPath 连接并规范化两个供内部使用的虚拟路径片段
// joinVirtualPath joins and normalizes two virtual-path fragments for internal use
func joinVirtualPath(prefix string, name string) string {
	parts := append(splitVirtualPath(prefix), splitVirtualPath(name)...)
	return strings.Join(parts, "/")
}

// joinVirtualPathChecked 校验并连接两个可序列化的虚拟路径
// joinVirtualPathChecked validates and joins two serializable virtual paths
func joinVirtualPathChecked(prefix string, name string) (string, error) {
	canonicalName, err := canonicalVirtualPath(name)
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return canonicalName, nil
	}
	canonicalPrefix, err := canonicalVirtualPath(prefix)
	if err != nil {
		return "", err
	}
	return canonicalPrefix + "/" + canonicalName, nil
}

// canonicalVirtualComponent 校验 allDirectorys 或 allFiles 中的一个字典键
// 键内分隔符无法由扁平 ContentTable 路径模型表示且会在重新编码时改变目录层级，因此会作为不可表示的线格式形状被拒绝而不是规范化
// canonicalVirtualComponent validates one dictionary key from allDirectorys or allFiles
// A separator inside a key cannot be represented by the flattened ContentTable path model without changing directory depth on re-encode, so it is rejected as an unrepresentable wire shape instead of normalized
func canonicalVirtualComponent(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is empty")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("name contains NUL")
	}
	if strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("name contains a path separator")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("unsafe name %q", name)
	}
	return name, nil
}

// canonicalVirtualPath 校验虚拟路径并统一使用正斜杠分隔
// canonicalVirtualPath validates a virtual path and normalizes its separators to slashes
func canonicalVirtualPath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("path is empty")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("path contains NUL")
	}
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("absolute or volume-qualified path is not allowed")
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("absolute path is not allowed")
	}
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("path contains unsafe segment %q", part)
		}
	}
	return strings.Join(parts, "/"), nil
}

// strictStringMap 将通用 MessagePack 映射转换为仅允许字符串键的映射
// strictStringMap converts a generic MessagePack map to a map that permits only string keys
func strictStringMap(v interface{}) (map[string]interface{}, error) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, nil
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(m))
		for k, val := range m {
			ks, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("map key must be string, got %T", k)
			}
			result[ks] = val
		}
		return result, nil
	}
	return nil, fmt.Errorf("expected map, got %T", v)
}

const (
	csharpInt32Min int64 = -1 << 31
	csharpInt32Max int64 = 1<<31 - 1
)

// checkedInt32Count 将内部 Int64 集合长度安全窄化为 C# 集合 API 使用的 Int32。
// checkedInt32Count safely narrows an internal Int64 collection length to the Int32 used by C# collection APIs.
func checkedInt32Count(count int64, label string) (int32, error) {
	if count < 0 || count > csharpInt32Max {
		return 0, fmt.Errorf("%s %d is outside the C# Int32 range [0,%d]", label, count, csharpInt32Max)
	}
	return int32(count), nil
}

// toInt32 将 MessagePack 整数转换为 C# int 使用的 wire 宽度，并始终执行 Int32 范围校验。
// toInt32 converts a MessagePack integer to the wire width used by C# int and always enforces the Int32 range.
func toInt32(v interface{}) (int32, bool) {
	switch n := v.(type) {
	case int64:
		if n < csharpInt32Min || n > csharpInt32Max {
			return 0, false
		}
		return int32(n), true
	case uint64:
		if n > uint64(csharpInt32Max) {
			return 0, false
		}
		return int32(n), true
	}
	return 0, false
}

// isIntegerValue 判断解码值是否属于支持的 MessagePack 整数表示
// isIntegerValue reports whether a decoded value uses one of the supported MessagePack integer representations
func isIntegerValue(value interface{}) bool {
	switch value.(type) {
	case int64, uint64:
		return true
	default:
		return false
	}
}

// toInt64 将支持的 MessagePack 整数转换为有符号 Int64
// toInt64 converts a supported MessagePack integer to signed Int64
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case uint64:
		if n > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(n), true
	}
	return 0, false
}

// lenOf 返回通用数组的长度，非数组值返回 -1
// lenOf returns the length of a generic array or -1 for a non-array value
func lenOf(v interface{}) int64 {
	if arr, ok := v.([]interface{}); ok {
		return int64(len(arr))
	}
	return -1
}
