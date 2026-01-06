package arc

// .arc 文件主要由三个部分组成：文件头 (Header)、数据区 (Data Area) 和 元数据区 (metadata)。
// 文件头后面是一个 metadata Offset (8 字节, int64)：这是一个相对偏移量。它表示从文件偏移量 28（即 Header 结束处）开始，到元数据区（metadata）起始位置的字节数。
//
// Header 之后是数据区 (Data Area)，这里顺序存储了所有文件的二进制数据。每个文件条目包含一个小型头部：
// Compression Flag (4 字节, uint32)：1 表示数据经过 Deflate 压缩，0 表示原始数据。
// Padding (4 字节, uint32)：通常为 0。
// Decompressed Size (4 字节, uint32)：文件解压后的原始大小。
// Compressed Size (4 字节, uint32)：文件在 ARC 中的实际存储大小。
// Data：文件的实际数据。
//
// 元数据区 (Footer)位于文件末尾，由多个“数据块 (Block)”组成。每个块包含：
// Block Type (4 字节, int32)：
// 0: UTF-16 哈希表。存储目录树结构，使用 UTF-16 编码生成的哈希。
// 1: UTF-8 哈希表。存储目录树结构，使用 UTF-8 编码生成的哈希。
// 3: 名称表 (Name Table)。存储哈希值与原始文件/目录名的对应关系。
// Block Size (8 字节, int64)：该块后续数据的长度。
// Block Content：具体的块数据。如果是 Type 3，其内部也是按照“数据区”的格式（带压缩标志和大小）存储的。
//
// 哈希表 (Hash Table)
// 这是一个递归结构，代表了整个目录树：
// Header (8 字节)：目录标识。
// ID (8 字节, uint64)：当前目录名称的哈希值。
// Counts (各 4 字节)：子目录数量 (DirCount)、文件数量 (FileCount)。
// Depth (4 字节)：当前目录在树中的深度（根目录为 0）。
// Dir Entries (每个 16 字节)：每个条目包含子目录的 Hash (uint64) 和在哈希块内的相对偏移量 Offset (int64)。
// File Entries (每个 16 字节)：每个条目包含文件名的 Hash (uint64) 和在 ARC 文件数据区中的绝对偏移量 Offset (int64)。
// Parent IDs：一系列父目录的哈希值。
// Sub-Dir Data：递归地包含所有子目录的哈希表结构。
//
// 名称表 (Name Table)
// 解压后的名称表包含一系列条目：
// Hash (8 字节, uint64)：对应文件或目录的唯一哈希值。
// Name Size (4 字节, int32)：名称的字符数（UTF-16）。
// Name (Size * 2 字节)：UTF-16LE 编码的原始名称字符串。

var arcHeader = []byte{
	0x77, 0x61, 0x72, 0x63, // warc
	0xFF, 0xAA, 0x45, 0xF1,
	0xE8, 0x03, 0x00, 0x00, // 1000
	0x04, 0x00, 0x00, 0x00, // 4
	0x02, 0x00, 0x00, 0x00, // 2
}

var encArcHeader = []byte{
	0x77, 0x61, 0x72, 0x70, // warp
	0xE8, 0x03, 0x00, 0x00, // 1000
}

var dirHeader = []byte{
	0x20, 0x00, 0x00, 0x00, // 32
	0x10, 0x00, 0x00, 0x00, // 16
}

// Arc represents an ARC file system in memory
type Arc struct {
	Name          string
	Root          *Dir
	KeepDupes     bool
	CompressGlobs []string
}

// Dir represents a directory node
type Dir struct {
	arc    *Arc
	Name   string
	Parent *Dir
	Dirs   map[string]*Dir
	Files  map[string]*File
}

type fileEntryRec struct {
	Hash   uint64
	Offset int64
}

type hashTable struct {
	Header        int64
	ID            uint64
	DirCount      int32
	FileCount     int32
	Depth         int32
	Padding       int32
	DirEntries    []fileEntryRec
	FileEntries   []fileEntryRec
	ParentsID     []uint64
	SubDirEntries []*hashTable
}

// File represents a file node with data pointer
type File struct {
	arc    *Arc
	Name   string
	Parent *Dir
	Ptr    FilePointer
}
