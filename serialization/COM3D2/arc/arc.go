package arc

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// .arc 文件主要由三个部分组成：文件头 (Header)、数据区 (Data Area) 和元数据区 (Metadata)
// 文件头后面是一个 metadata Offset (8 字节, int64)，这是一个相对偏移量，表示从文件偏移量 28（即 Header 结束处）开始，到元数据区起始位置的字节数
//
// Header 之后是数据区 (Data Area)，这里顺序存储了所有文件的二进制数据，每个文件条目包含一个小型头部：
// Compression Flag (4 字节, uint32)：1 表示数据经过 Deflate 压缩，0 表示原始数据
// Padding (4 字节, uint32)：通常为 0
// Decompressed Size (4 字节, uint32)：文件解压后的原始大小
// Compressed Size (4 字节, uint32)：文件在 ARC 中的实际存储大小
// Data：文件的实际数据
//
// 元数据区 (Footer) 位于文件末尾，由多个数据块 (Block) 组成，每个块包含：
// Block Type (4 字节, int32)：
// 0：UTF-16 哈希表，存储目录树结构并使用 UTF-16 编码生成哈希
// 1：UTF-8 哈希表，存储目录树结构并使用 UTF-8 编码生成哈希
// 3：名称表 (Name Table)，存储哈希值与原始文件或目录名的对应关系
// Block Size (8 字节, int64)：该块后续数据的长度
// Block Content：具体的块数据，如果是 Type 3，其内部也按照数据区格式存储，带有压缩标志和大小
//
// 哈希表 (Hash Table)
// 这是一个代表整个目录树的递归结构：
// Header (8 字节)：目录标识
// ID (8 字节, uint64)：当前目录名称的哈希值
// Counts (各 4 字节)：子目录数量 (DirCount) 和文件数量 (FileCount)
// Depth (4 字节)：当前目录在树中的深度，根目录为 0
// Dir Entries (每个 16 字节)：每个条目包含子目录的 Hash (uint64) 和在哈希块内的相对偏移量 Offset (int64)
// File Entries (每个 16 字节)：每个条目包含文件名的 Hash (uint64) 和在 ARC 文件数据区中的绝对偏移量 Offset (int64)
// Parent IDs：一系列父目录的哈希值
// Sub-Dir Data：递归包含所有子目录的哈希表结构
//
// 名称表 (Name Table)
// 解压后的名称表包含一系列条目：
// Hash (8 字节, uint64)：对应文件或目录的唯一哈希值
// Name Size (4 字节, int32)：名称的 UTF-16 字符数量
// Name (Size * 2 字节)：UTF-16LE 编码的原始名称字符串
// An .arc file mainly consists of three parts: the header, data area, and metadata area
// The metadata Offset (8 bytes, int64) after the header is relative and gives the byte count from file offset 28, where the header ends, to the start of metadata
//
// The data area follows the header and stores all file data sequentially, with each file entry containing a small header:
// Compression Flag (4 bytes, uint32): 1 means the data is Deflate-compressed and 0 means it is stored raw
// Padding (4 bytes, uint32): usually 0
// Decompressed Size (4 bytes, uint32): original file size after decompression
// Compressed Size (4 bytes, uint32): actual storage size of the file in the ARC
// Data: the file payload
//
// The metadata area (Footer) is located at the end of the file and consists of multiple blocks, each containing:
// Block Type (4 bytes, int32):
// 0: UTF-16 hash table storing the directory tree with hashes generated from UTF-16 text
// 1: UTF-8 hash table storing the directory tree with hashes generated from UTF-8 text
// 3: Name Table mapping hashes to original file or directory names
// Block Size (8 bytes, int64): length of the block content that follows
// Block Content: the block-specific data, with Type 3 itself stored in data-area form including a compression flag and sizes
//
// Hash Table
// This recursive structure represents the complete directory tree:
// Header (8 bytes): directory marker
// ID (8 bytes, uint64): hash of the current directory name
// Counts (4 bytes each): child-directory count (DirCount) and file count (FileCount)
// Depth (4 bytes): depth of the current directory in the tree, with the root at 0
// Dir Entries (16 bytes each): child-directory Hash (uint64) and relative Offset (int64) within the hash block
// File Entries (16 bytes each): file-name Hash (uint64) and absolute Offset (int64) within the ARC data area
// Parent IDs: hashes for the chain of parent directories
// Sub-Dir Data: recursively embedded hash-table structures for all child directories
//
// Name Table
// The decompressed name table contains a sequence of entries:
// Hash (8 bytes, uint64): unique hash corresponding to a file or directory
// Name Size (4 bytes, int32): number of UTF-16 code units in the name
// Name (Size * 2 bytes): original name encoded as UTF-16LE

// arcHeader 是普通 ARC 文件的固定头部签名
// arcHeader represents the expected binary header signature for ARC files, used to validate file format integrity
var arcHeader = []byte{
	// ASCII 签名 warc
	// ASCII signature warc
	0x77, 0x61, 0x72, 0x63,
	0xFF, 0xAA, 0x45, 0xF1,
	// 数值 1000
	// Value 1000
	0xE8, 0x03, 0x00, 0x00,
	// 数值 4
	// Value 4
	0x04, 0x00, 0x00, 0x00,
	// 数值 2
	// Value 2
	0x02, 0x00, 0x00, 0x00,
}

// encArcHeader 是加密 ARC 文件的固定头部签名
// encArcHeader represents the binary header signature for encrypted ARC files, used to identify unsupported formats
var encArcHeader = []byte{
	// ASCII 签名 warp
	// ASCII signature warp
	0x77, 0x61, 0x72, 0x70,
	// 数值 1000
	// Value 1000
	0xE8, 0x03, 0x00, 0x00,
}

// dirHeader 是目录哈希表序列化使用的固定头部
// dirHeader is a predefined byte array representing the header structure for directory hash table serialization
var dirHeader = []byte{
	// 数值 32
	// Value 32
	0x20, 0x00, 0x00, 0x00,
	// 数值 16
	// Value 16
	0x10, 0x00, 0x00, 0x00,
}

// Arc 表示内存中的 ARC 文件系统
// Arc represents an ARC file system in memory
type Arc struct {
	Name          string   // ARC 文件系统或目录节点的名称 / Name represents the name of the ARC file system or directory node
	Root          *Dir     // ARC 文件系统的根目录 / Root represents the root directory of the ARC file system
	KeepDupes     bool     // 是否根据完整路径而不只是名称允许重复文件 / KeepDupes determines whether duplicate files are allowed based on their full path rather than just the name
	CompressGlobs []string // ARC 文件系统内用于压缩文件的通配模式 / CompressGlobs specifies glob patterns for file compression within the ARC file system
}

// Dir 表示目录节点
// Dir represents a directory node
type Dir struct {
	Arc    *Arc             // 与此目录节点关联的 ARC 文件系统 / Arc represents a pointer to the Arc file system associated with this directory node
	Name   string           // 目录名称 / Name represents the name of the directory
	Parent *Dir             // 当前目录节点在文件系统层级中的父目录 / Parent represents the parent directory of the current directory node in the filesystem hierarchy
	Dirs   map[string]*Dir  // 当前目录内从目录名到相应目录节点的映射 / Dirs maps directory names to their corresponding directory nodes within the current directory
	Files  map[string]*File // 当前目录内从文件名到相应文件节点的映射 / Files maps file names to their corresponding file nodes within the current directory
}

// fileEntryRec 表示单个文件条目的哈希和数据偏移元数据
// fileEntryRec represents metadata for a single file entry, including its unique hash and data offset within a structure
type fileEntryRec struct {
	Hash   uint64 // fileEntryRec 结构中文件条目的唯一标识 / Hash represents the unique identifier for a file entry in the fileEntryRec structure
	Offset int64  // 文件条目在关联数据结构中的字节位置 / Offset represents the byte position of the file entry within the associated data structure
}

// hashTable 表示用于以结构化形式保存目录和文件元数据的层级数据结构
// hashTable represents a hierarchical data structure used for storing directory and file metadata in a structured format
type hashTable struct {
	Header        int64          // hashTable 结构的主要标识或描述值 / Header represents the primary identifier or descriptor for the hashTable structure
	ID            uint64         // 用于链接或引用关联条目或子表的唯一标识 / ID represents a unique identifier for the hashTable used to link or reference associated entries or sub-tables
	DirCount      int32          // 当前 hashTable 实例中的目录总数 / DirCount represents the total number of directories in the current hashTable instance
	FileCount     int32          // 当前 hashTable 实例中的文件总数 / FileCount represents the total number of files in the current hashTable instance
	Depth         int32          // 当前 hashTable 在整体目录结构中的层级 / Depth represents the hierarchical level of the current hashTable within the overall directory structure
	Padding       int32          // hashTable 结构中为对齐或未来用途保留的值 / Padding is reserved for alignment or future use within the hashTable structure
	DirEntries    []fileEntryRec // 目录条目记录列表，每项包含哈希和偏移等元数据 / DirEntries holds a list of directory entry records, each containing metadata such as hash and offset information
	FileEntries   []fileEntryRec // 文件条目记录列表，每项包含哈希和偏移等元数据 / FileEntries holds a list of file entry records, each containing metadata such as hash and offset information
	ParentsID     []uint64       // 用于追踪当前 hashTable 层级沿革的父级 ID 列表 / ParentsID represents a list of parent IDs in the hierarchy, used to trace the lineage of the current hashTable
	SubDirEntries []*hashTable   // 与当前目录的子目录相对应的 hashTable 指针集合 / SubDirEntries represents a collection of hashTable pointers corresponding to the subdirectories of the current directory
}

// File 表示带有数据指针的文件节点
// File represents a file node with a data pointer
type File struct {
	Arc    *Arc        // 此文件所属的 ARC 文件系统 / Arc points to the ARC file system that this file belongs to
	Name   string      // 文件或目录节点的名称 / Name represents the name of the file or directory node
	Parent *Dir        // 当前文件节点在文件系统层级中的父目录 / Parent represents the parent directory of the current file node in the filesystem hierarchy
	Ptr    FilePointer // 指向文件数据的内存或压缩指针 / Ptr represents a memory or compressed pointer to the file data
}

// ReadArcBytes 从内存中解析 ARC 载荷
// 返回的 Arc 延迟读取内存缓冲区，不依赖打开的文件句柄
// ReadArcBytes parses an ARC payload from memory
// The returned Arc lazily reads from the in-memory buffer and does not depend on an open file handle
func ReadArcBytes(data []byte) (*Arc, error) {
	return ReadArc(bytes.NewReader(data))
}

// ReadArcFile 将 ARC 文件读入内存后解析
// 当源文件关闭后仍需使用返回的 Arc 时使用此函数
// ReadArcFile loads an ARC file into memory before parsing it
// Use this when the returned Arc must remain usable after the source file is closed
func ReadArcFile(path string) (*Arc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read .arc file: %w", err)
	}
	return ReadArcBytes(data)
}

// ReadArc 从 rs 解析 ARC 元数据
// 文件载荷仍由 rs 延迟支持，因此后续 Data 或 Extract 调用时调用方必须保持 rs 可读，需要脱离源流时使用 ReadArcFile 或 ReadArcBytes
// ReadArc parses ARC metadata from rs
// File payloads remain lazily backed by rs, so callers must keep rs readable for later Data or Extract calls
// Use ReadArcFile or ReadArcBytes when a detached Arc is required
func ReadArc(rs io.ReadSeeker) (*Arc, error) {
	reader := stream.NewBinaryReader(rs)

	// 1. 检查头部
	// 1. Check the header
	// 固定头部为 20 字节
	// The fixed header is 20 bytes
	header, err := reader.ReadBytes(len(arcHeader))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(header, arcHeader) {
		if bytes.HasPrefix(header, encArcHeader) {
			return nil, fmt.Errorf("this .arc file is encrypted (unsupported). Please install the original DLC and launch the game once to decrypt it")
		}
		return nil, fmt.Errorf("invalid ARC header, this may not be a .arc file")
	}

	// 2. 获取元数据偏移
	// 2. Get the metadata offset
	// 元数据偏移占 8 字节
	// The metadata offset occupies 8 bytes
	metadataOffset, err := reader.ReadInt64()
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata offset: %w", err)
	}

	// 3. 跳转到元数据位置
	// 3. Jump to the metadata position
	// 基准偏移为 20 + 8
	// The base offset is 20 + 8
	headerEndPosition, _ := reader.Seek(0, io.SeekCurrent)
	// 元数据位置等于头部结束位置加元数据相对偏移
	// The metadata position equals the header-end position plus the relative metadata offset
	if _, err := reader.Seek(headerEndPosition+metadataOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("invalid metadata offset, file may broken: %w", err)
	}

	var utf8HashData, utf16HashData, utf16NameData []byte

	for utf8HashData == nil || utf16HashData == nil || utf16NameData == nil {

		blockType, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("failed to read block type: %w", err)
		}
		blockSize, err := reader.ReadInt64()
		if err != nil {
			return nil, fmt.Errorf("failed to read block size: %w", err)
		}

		switch blockType {
		// UTF-16 哈希表
		// UTF-16 hash table
		case 0:
			if utf16HashData, err = reader.ReadBytes(int(blockSize)); err != nil {
				return nil, fmt.Errorf("failed to read utf16 hash data: %w", err)
			}
		// UTF-8 哈希表
		// UTF-8 hash table
		case 1:
			if utf8HashData, err = reader.ReadBytes(int(blockSize)); err != nil {
				return nil, fmt.Errorf("failed to read utf8 hash data: %w", err)
			}
		// 作为文件块存储的名称表
		// Name table stored as a file block
		case 3:
			// 读取内嵌文件头
			// Read the inline file header
			compressedFlag, err := reader.ReadUInt32()
			if err != nil {
				return nil, fmt.Errorf("failed to read compressed flag: %w", err)
			}
			// 跳过保留值
			// Skip the padding value
			_, err = reader.ReadUInt32()
			if err != nil {
				return nil, fmt.Errorf("failed to read padding: %w", err)
			}
			// 跳过解压大小
			// Skip the decompressed size
			_, err = reader.ReadUInt32()
			if err != nil {
				return nil, fmt.Errorf("failed to read decompressed size: %w", err)
			}
			// 读取压缩大小
			// Read the compressed size
			compressedSize, err := reader.ReadUInt32()
			if err != nil {
				return nil, fmt.Errorf("failed to read compressed size: %w", err)
			}

			data, err := reader.ReadBytes(int(compressedSize))
			if err != nil {
				return nil, fmt.Errorf("failed to read compressed data: %w", err)
			}

			if compressedFlag == 1 {
				dec, err := deflateDecompress(data)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress data: %w", err)
				}
				utf16NameData = dec
			} else {
				utf16NameData = data
			}
		default:
			return nil, fmt.Errorf("unknown metadata block type %d", blockType)
		}
	}

	// 解析哈希表和名称表
	// Parse the hash tables and name table
	// UTF-8 哈希表在 CM3D2.Tookit 中仅用于检查是否与 UTF-16 哈希表一致
	// The UTF-8 hash table is used only to check equality with the UTF-16 table in CM3D2.Tookit
	_, err = readHashTable(stream.NewBinaryReader(bytes.NewReader(utf8HashData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read utf8 hash table: %w", err)
	}
	// 读取 UTF-16 哈希表
	// Read the UTF-16 hash table
	utf16HT, err := readHashTable(stream.NewBinaryReader(bytes.NewReader(utf16HashData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read utf16 hash table: %w", err)
	}
	// 读取名称表
	// Read the name table
	nameLUT, err := readNameTable(stream.NewBinaryReader(bytes.NewReader(utf16NameData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read utf16 name table: %w", err)
	}

	// 初始化 Arc
	// Setup Arc
	arc := NewArc("")
	// 根据根节点 ID 设置名称
	// Set name from root ID
	if rootName, ok := nameLUT[utf16HT.ID]; ok {
		// 如果存在分隔符，提取最后一个分隔符之后的部分
		// Extract after the last separator if present
		base := rootName
		if i := lastIndexOfSep(rootName); i >= 0 && i+1 < len(rootName) {
			base = rootName[i+1:]
		}
		arc = NewArc(base)
	}

	// 使用 UTF-16 哈希表填充 Arc 结构
	// Populate Arc structure using UTF16 table
	if err := populateArc(arc, utf16HT, nameLUT, reader, headerEndPosition); err != nil {
		return nil, fmt.Errorf("failed to populateArc arc structure: %w", err)
	}

	return arc, nil
}

// populateArc 根据 UTF-16 哈希表和名称查找表建立 Arc
// populateArc builds the Arc from UTF16 hashtable and name lut
func populateArc(arc *Arc, t *hashTable, nameLUT map[uint64]string, reader *stream.BinaryReader, baseOffset int64) error {
	// 递归遍历
	// Recursively traverse
	var walk func(tab *hashTable, parent *Dir) error
	walk = func(tab *hashTable, parent *Dir) error {
		// 文件
		// Files
		for _, fe := range tab.FileEntries {
			name, ok := nameLUT[fe.Hash]
			if !ok {
				return fmt.Errorf("missing name for file hash %x", fe.Hash)
			}
			f := AddFileByPath(parent, name)
			f.Arc = arc
			f.Ptr = NewArcPointer(reader, baseOffset+fe.Offset)
		}
		// 目录
		// Dirs
		for _, de := range tab.DirEntries {
			name, ok := nameLUT[de.Hash]
			if !ok {
				return fmt.Errorf("missing name for dir hash %x", de.Hash)
			}
			d := GetOrCreateDirByPath(parent, name)
			d.Arc = arc
			// 根据 ID 查找匹配的子表
			// Find matching subtable by ID
			var sub *hashTable
			for _, st := range tab.SubDirEntries {
				if st.ID == de.Hash {
					sub = st
					break
				}
			}
			if sub == nil {
				return fmt.Errorf("subtable not found for dir %x", de.Hash)
			}
			if err := walk(sub, d); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(t, arc.Root)
}

// Dump 将 Arc 写入磁盘上的 ARC 文件
// Dump writes the Arc to an ARC file on disk
func (arc *Arc) Dump(path string) error {
	tmpDir := filepath.Dir(path)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		// Windows 下忽略权限问题
		// Ignore on Windows perms
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create ARC file: %w", err)
	}
	defer f.Close()

	writer := stream.NewBinaryWriter(f)

	// 写入头部和元数据偏移占位值
	// Header + placeholder metadata offset
	if err := writer.WriteBytes(arcHeader); err != nil {
		return fmt.Errorf("failed to write ARC header: %w", err)
	}
	if err := writer.WriteInt64(0); err != nil {
		return fmt.Errorf("failed to write placeholder metadata offset: %w", err)
	}
	baseOff, err := writer.Tell()
	if err != nil {
		return fmt.Errorf("failed to get base offset: %w", err)
	}

	// 写入文件表
	// File table write
	fileOffsets := map[uint64]int64{}
	files := AllFiles(arc)
	// 将压缩通配模式编译为正则表达式
	// Compile compress globs into regex
	var pats []*regexp.Regexp
	for _, g := range arc.CompressGlobs {
		if g == "" {
			continue
		}
		regex, err := globToRegex(g)
		if err != nil {
			return fmt.Errorf("invalid glob pattern: %w", err)
		}
		pats = append(pats, regex)
	}

	for i, fl := range files {
		// 默认保留已有的压缩载荷
		// 压缩通配模式还会选择未压缩文件或新文件进行压缩
		// Preserve an existing compressed payload by default
		// Compression globs additionally select uncompressed/new files for compression
		wasCompressed := fl.Ptr.Compressed()
		compress := wasCompressed
		for _, p := range pats {
			if p.MatchString(fl.Name) {
				compress = true
				break
			}
		}
		stored, err := fl.Ptr.Data()
		if err != nil {
			return fmt.Errorf("failed to read file data: %w", err)
		}
		raw := stored
		enc := stored
		if wasCompressed {
			raw, err = deflateDecompress(stored)
			if err != nil {
				return fmt.Errorf("failed to decompress existing file %q: %w", fl.RelativePath(), err)
			}
		} else if compress {
			enc, err = deflateCompress(raw)
			if err != nil {
				return fmt.Errorf("failed to compress file data: %w", err)
			}
		}
		rawSize, err := checkedArcUint32Length(fmt.Sprintf("file %q raw size", fl.RelativePath()), len(raw))
		if err != nil {
			return err
		}
		storedSize, err := checkedArcUint32Length(fmt.Sprintf("file %q stored size", fl.RelativePath()), len(enc))
		if err != nil {
			return err
		}
		pos, err := writer.Tell()
		if err != nil {
			return fmt.Errorf("failed to get file position: %w", err)
		}
		fileOffsets[fl.UniqueID()] = pos - baseOff
		// 写入文件头
		// Header
		flag := uint32(0)
		if compress {
			flag = 1
		}
		if err := writer.WriteUInt32(flag); err != nil {
			return fmt.Errorf("failed to write file header: %w", err)
		}
		if err := writer.WriteUInt32(0); err != nil {
			return fmt.Errorf("failed to write file header: %w", err)
		}
		if err := writer.WriteUInt32(rawSize); err != nil {
			return fmt.Errorf("failed to write file header: %w", err)
		}
		if err := writer.WriteUInt32(storedSize); err != nil {
			return fmt.Errorf("failed to write file header: %w", err)
		}
		if err := writer.WriteBytes(enc); err != nil {
			return fmt.Errorf("failed to write file data: %w", err)
		}
		// 进度索引
		// Progress
		_ = i
	}

	// 写入元数据
	// Write metadata
	metadataPos, err := writer.Tell()
	if err != nil {
		return fmt.Errorf("failed to get metadata position: %w", err)
	}
	// 回填元数据偏移
	// Patch metadata offset
	if _, err := writer.Seek(int64(len(arcHeader)), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to metadata offset: %w", err)
	}
	if err := writer.WriteInt64(metadataPos - baseOff); err != nil {
		return fmt.Errorf("failed to write metadata offset: %w", err)
	}
	if _, err := writer.Seek(metadataPos, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to metadata position: %w", err)
	}

	// 建立唯一 ID 到哈希的映射
	// Build uuid->hash mappings
	uuidToHash16 := map[uint64]uint64{}
	uuidToHash8 := map[uint64]uint64{}
	for _, d := range AllDirs(arc) {
		uuidToHash16[d.UniqueID()] = d.UTF16Hash()
		uuidToHash8[d.UniqueID()] = d.UTF8Hash()
	}
	for _, fl := range AllFiles(arc) {
		uuidToHash16[fl.UniqueID()] = fl.UTF16Hash()
		uuidToHash8[fl.UniqueID()] = fl.UTF8Hash()
	}
	uuidToHash16[arc.Root.UniqueID()] = arc.Root.UTF16Hash()
	uuidToHash8[arc.Root.UniqueID()] = arc.Root.UTF8Hash()

	// 计算两个哈希表的目录偏移
	// Calculate directory offsets for both tables
	dirOff16 := arc.calculateDirOffsets(uuidToHash16)
	dirOff8 := arc.calculateDirOffsets(uuidToHash8)

	// 元数据块 0，即 UTF-16 哈希表
	// Metadata block 0 (UTF16)
	var buf bytes.Buffer
	bufWriter := stream.NewBinaryWriter(&buf)
	if err := arc.writeHashTable(bufWriter, dirOff16, uuidToHash16, fileOffsets, arc.Root); err != nil {
		return fmt.Errorf("failed to write UTF16 hash table: %w", err)
	}
	if err := writer.WriteInt32(0); err != nil {
		return fmt.Errorf("failed to write metadata block count: %w", err)
	}
	if err := writer.WriteInt64(int64(buf.Len())); err != nil {
		return fmt.Errorf("failed to write metadata block size: %w", err)
	}
	if err := writer.WriteBytes(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write metadata block: %w", err)
	}
	buf.Reset()

	// 元数据块 1，即 UTF-8 哈希表
	// Metadata block 1 (UTF8)
	if err := arc.writeHashTable(bufWriter, dirOff8, uuidToHash8, fileOffsets, arc.Root); err != nil {
		return fmt.Errorf("failed to write UTF8 hash table: %w", err)
	}
	if err := writer.WriteInt32(1); err != nil {
		return fmt.Errorf("failed to write metadata block count: %w", err)
	}
	if err := writer.WriteInt64(int64(buf.Len())); err != nil {
		return fmt.Errorf("failed to write metadata block size: %w", err)
	}
	if err := writer.WriteBytes(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write metadata block: %w", err)
	}
	buf.Reset()

	// 元数据块 3，即压缩的 UTF-16 名称表
	// Metadata block 3 (UTF16 name table, compressed)
	if err := arc.writeNameTable(bufWriter, true); err != nil {
		return fmt.Errorf("failed to write UTF16 name table: %w", err)
	}
	nameRaw := buf.Bytes()
	nameEnc, err := deflateCompress(nameRaw)
	if err != nil {
		return fmt.Errorf("failed to compress UTF16 name table: %w", err)
	}
	nameRawSize, err := checkedArcUint32Length("UTF16 name table raw size", len(nameRaw))
	if err != nil {
		return err
	}
	nameStoredSize, err := checkedArcUint32Length("UTF16 name table compressed size", len(nameEnc))
	if err != nil {
		return err
	}
	if err := writer.WriteInt32(3); err != nil {
		return fmt.Errorf("failed to write metadata block count: %w", err)
	}
	if err := writer.WriteInt64(int64(len(nameEnc) + 16)); err != nil {
		return fmt.Errorf("failed to write metadata block size: %w", err)
	}
	if err := writer.WriteUInt32(1); err != nil {
		return fmt.Errorf("failed to write metadata block type: %w", err)
	}
	if err := writer.WriteUInt32(0); err != nil {
		return fmt.Errorf("failed to write metadata block flags: %w", err)
	}
	if err := writer.WriteUInt32(nameRawSize); err != nil {
		return fmt.Errorf("failed to write UTF16 name table raw size: %w", err)
	}
	if err := writer.WriteUInt32(nameStoredSize); err != nil {
		return fmt.Errorf("failed to write UTF16 name table compressed size: %w", err)
	}
	if err := writer.WriteBytes(nameEnc); err != nil {
		return fmt.Errorf("failed to write UTF16 name table compressed data: %w", err)
	}

	return nil
}

// checkedArcUint32Length 将字节长度转换为 ARC 线格式的 UInt32 并拒绝溢出
// checkedArcUint32Length converts a byte length to the ARC wire-format UInt32 and rejects overflow
func checkedArcUint32Length(path string, length int) (uint32, error) {
	if uint64(length) > math.MaxUint32 {
		return 0, fmt.Errorf("%s %d exceeds UInt32", path, length)
	}
	return uint32(length), nil
}

// checkedArcInt32Count 将集合数量转换为 ARC 线格式的 Int32 并拒绝溢出
// checkedArcInt32Count converts a collection count to the ARC wire-format Int32 and rejects overflow
func checkedArcInt32Count(path string, count int) (int32, error) {
	if uint64(count) > math.MaxInt32 {
		return 0, fmt.Errorf("%s %d exceeds Int32", path, count)
	}
	return int32(count), nil
}

// calculateDirOffsets 根据目录结构和深度计算 ARC 文件系统中的目录偏移映射
// calculateDirOffsets computes offset mapping for directories in an ARC file system based on their structure and depth
func (arc *Arc) calculateDirOffsets(uuidToHash map[uint64]uint64) map[uint64]int64 {
	dict := map[uint64]int64{}
	var offset int64 = 0
	var rec func(d *Dir)
	rec = func(d *Dir) {
		// 累加父目录偏移差值
		// Accumulate parent deltas
		var delta int64 = 0
		p := d.Parent
		for p != nil {
			delta += dict[p.UniqueID()]
			p = p.Parent
		}
		dict[d.UniqueID()] = offset - delta
		// 目录头占 32 字节
		// Header occupies 32 bytes
		offset += 32
		// 每个目录或文件条目占 16 字节
		// 16 bytes per entry (dir or file)
		cnt := len(d.Dirs) + len(d.Files)
		offset += int64(16 * cnt)
		// 每个父目录哈希占 8 字节
		// 8 bytes per parent hash
		offset += int64(8 * d.Depth())
		// 写入时子目录先按哈希再按偏移排序
		// 此处遵循 uuidToHash 的顺序
		// Children are ordered by hash, then by offset when writing
		// We follow order by uuidToHash
		children := d.sortedDirs()
		// 按哈希稳定排序
		// Stable sort by hash
		sort.Slice(children, func(i, j int) bool { return uuidToHash[children[i].UniqueID()] < uuidToHash[children[j].UniqueID()] })
		for _, sub := range children {
			rec(sub)
		}
	}
	rec(arc.Root)
	return dict
}

// MergeFrom 将 src 合并到当前 Arc，keepDupes 为 true 时使用完整路径作为文件键，否则使用最后一段名称
// MergeFrom merges src into this Arc; if keepDupes is true, use full path as key for files; otherwise last segment
func (arc *Arc) MergeFrom(src *Arc, keepDupes bool) {
	arc.KeepDupes = keepDupes
	var walk func(d *Dir, into *Dir)
	walk = func(d *Dir, into *Dir) {
		for _, sub := range d.sortedDirs() {
			nd := into.GetOrCreateDir(sub.Name)
			walk(sub, nd)
		}
		for _, fl := range d.sortedFiles() {
			// 如果存在重复项且不保留重复项，则替换文件
			// If duplicate and not keep dupes, replace
			nf := &File{Arc: arc, Name: fl.Name}
			data, _ := fl.Ptr.Data()
			if fl.Ptr.Compressed() {
				nf.Ptr = NewMemoryPointerCompressed(data, fl.Ptr.RawSize())
			} else {
				nf.Ptr = NewMemoryPointer(data)
			}
			into.AddFile(nf)
		}
	}
	walk(src.Root, arc.Root)
}

// Pack 将 dirPath 中的全部文件载入 Arc 结构并写到 arcPath
// Pack loads all files from dirPath into an Arc structure and dumps it to arcPath
func Pack(dirPath string, arcPath string) error {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to getting absolute path for %q: %w", dirPath, err)
	}

	// 创建新的 Arc
	// 使用目录名称作为 Arc 名称
	// Create a new Arc
	// Use the directory name as the Arc name
	name := filepath.Base(absDir)
	fs := NewArc(name)

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walking %q: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}

		// 计算 ARC 内使用的相对路径
		// Calculate relative path for use within the ARC
		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return fmt.Errorf("failed to calculating relative path for %q: %w", path, err)
		}

		// 读取文件数据
		// Read file data
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to reading file %q: %w", path, err)
		}

		// 加入 Arc
		// Add to Arc
		f := AddFileByPath(fs.Root, rel)
		f.Arc = fs
		f.Ptr = NewMemoryPointer(data)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory %q: %w", absDir, err)
	}

	// 写到 arcPath
	// Dump to arcPath
	return fs.Dump(arcPath)
}

// Unpack 将整个 Arc 文件系统解压到指定目录
// Unpack extracts the entire Arc file system to the specified directory
func (arc *Arc) Unpack(outDir string) error {
	for _, f := range AllFiles(arc) {
		relPath := f.RelativePath()
		targetPath := filepath.Join(outDir, relPath)
		if err := f.Extract(targetPath); err != nil {
			return fmt.Errorf("failed to extract %s: %w", relPath, err)
		}
	}
	return nil
}

// Extract 将文件保存到指定路径
// Extract saves the file to the specified path
func (f *File) Extract(outPath string) error {
	data, err := f.Ptr.Data()
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", f.RelativePath(), err)
	}

	if f.Ptr.Compressed() {
		data, err = deflateDecompress(data)
		if err != nil {
			return fmt.Errorf("failed to decompress %s: %w", f.RelativePath(), err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", outPath, err)
	}

	return os.WriteFile(outPath, data, 0644)
}
