package arc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

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

// arcHeader represents the expected binary header signature for ARC files, used to validate file format integrity.
var arcHeader = []byte{
	0x77, 0x61, 0x72, 0x63, // warc
	0xFF, 0xAA, 0x45, 0xF1,
	0xE8, 0x03, 0x00, 0x00, // 1000
	0x04, 0x00, 0x00, 0x00, // 4
	0x02, 0x00, 0x00, 0x00, // 2
}

// encArcHeader represents the binary header signature for encrypted ARC files, used to identify unsupported formats.
var encArcHeader = []byte{
	0x77, 0x61, 0x72, 0x70, // warp
	0xE8, 0x03, 0x00, 0x00, // 1000
}

// dirHeader is a predefined byte array representing the header structure for directory hash table serialization.
var dirHeader = []byte{
	0x20, 0x00, 0x00, 0x00, // 32
	0x10, 0x00, 0x00, 0x00, // 16
}

// Arc represents an ARC file system in memory
type Arc struct {
	Name          string   // Name represents the name of the ARC file system or directory node.
	Root          *Dir     // Root represents the root directory of the ARC file system.
	KeepDupes     bool     // KeepDupes determines whether duplicate files are allowed based on their full path rather than just the name.
	CompressGlobs []string // CompressGlobs specifies glob patterns for file compression within the ARC file system.
}

// Dir represents a directory node
type Dir struct {
	Arc    *Arc             // Arc represents a pointer to the Arc file system associated with this directory node.
	Name   string           // Name represents the name of the directory.
	Parent *Dir             // Parent represents the parent directory of the current directory node in the filesystem hierarchy.
	Dirs   map[string]*Dir  // Dirs maps directory names to their corresponding directory nodes within the current directory.
	Files  map[string]*File // Files maps file names to their corresponding file nodes within the current directory.
}

// fileEntryRec represents metadata for a single file entry, including its unique hash and data offset within a structure.
type fileEntryRec struct {
	Hash   uint64 // Hash represents the unique identifier for a file entry in the fileEntryRec structure.
	Offset int64  // Offset represents the byte position of the file entry within the associated data structure.
}

// hashTable represents a hierarchical data structure used for storing directory and file metadata in a structured format.
type hashTable struct {
	Header        int64          // Header represents the primary identifier or descriptor for the hashTable structure.
	ID            uint64         // ID represents a unique identifier for the hashTable used to link or reference associated entries or sub-tables.
	DirCount      int32          // DirCount represents the total number of directories in the current hashTable instance.
	FileCount     int32          // FileCount represents the total number of files in the current hashTable instance.
	Depth         int32          // Depth represents the hierarchical level of the current hashTable within the overall directory structure.
	Padding       int32          // Padding is reserved for alignment or future use within the hashTable structure.
	DirEntries    []fileEntryRec // DirEntries holds a list of directory entry records, each containing metadata such as hash and offset information.
	FileEntries   []fileEntryRec // FileEntries holds a list of file entry records, each containing metadata such as hash and offset information.
	ParentsID     []uint64       // ParentsID represents a list of parent IDs in the hierarchy, used to trace the lineage of the current hashTable.
	SubDirEntries []*hashTable   // SubDirEntries represents a collection of hashTable pointers corresponding to the subdirectories of the current directory.
}

// File represents a file node with a data pointer
type File struct {
	Arc    *Arc        // Arc points to the ARC file system that this file belongs to.
	Name   string      // Name represents the name of the file or directory node.
	Parent *Dir        // Parent represents the parent directory of the current file node in the filesystem hierarchy.
	Ptr    FilePointer // Ptr represents a memory or compressed pointer to the file data.
}

// ReadArc parses an ARC file and returns an in-memory Arc representation
func ReadArc(rs io.ReadSeeker) (*Arc, error) {
	reader := stream.NewBinaryReader(rs)

	// 1. check header
	header, err := reader.ReadBytes(len(arcHeader))
	if err != nil { // 20 byte
		return nil, err
	}
	if !bytes.Equal(header, arcHeader) {
		if bytes.HasPrefix(header, encArcHeader) {
			return nil, fmt.Errorf("this .arc file is encrypted (unsupported). Please install the original DLC and launch the game once to decrypt it")
		}
		return nil, fmt.Errorf("invalid ARC header, this may not be a .arc file")
	}

	// 2. get metadata Offset
	metadataOffset, err := reader.ReadInt64() // 8 byte
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata offset: %w", err)
	}

	// 3. jump to metadataPosition
	headerEndPosition, _ := reader.Seek(0, io.SeekCurrent)                                 // baseOffset =  20 + 8
	if _, err := reader.Seek(headerEndPosition+metadataOffset, io.SeekStart); err != nil { // metadataPosition = headerEndPosition + metadataOffset
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
		case 0: // utf16 hash
			if utf16HashData, err = reader.ReadBytes(int(blockSize)); err != nil {
				return nil, fmt.Errorf("failed to read utf16 hash data: %w", err)
			}
		case 1: // utf8 hash
			if utf8HashData, err = reader.ReadBytes(int(blockSize)); err != nil {
				return nil, fmt.Errorf("failed to read utf8 hash data: %w", err)
			}
		case 3: // name table as file block
			// read inline file header
			compressedFlag, err := reader.ReadUInt32()
			if err != nil {
				return nil, fmt.Errorf("failed to read compressed flag: %w", err)
			}
			_, err = reader.ReadUInt32() // padding
			if err != nil {
				return nil, fmt.Errorf("failed to read padding: %w", err)
			}
			_, err = reader.ReadUInt32() // Decompressed Size
			if err != nil {
				return nil, fmt.Errorf("failed to read decompressed size: %w", err)
			}
			compressedSize, err := reader.ReadUInt32() // Compressed Size
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

	// parse tables
	_, err = readHashTable(stream.NewBinaryReader(bytes.NewReader(utf8HashData))) // utf-8 hashtable, in CM3D2.Tookit only use this to check if it same to utf-16 hashtable
	if err != nil {
		return nil, fmt.Errorf("failed to read utf8 hash table: %w", err)
	}
	utf16HT, err := readHashTable(stream.NewBinaryReader(bytes.NewReader(utf16HashData))) // utf-16 hashtable
	if err != nil {
		return nil, fmt.Errorf("failed to read utf16 hash table: %w", err)
	}
	nameLUT, err := readNameTable(stream.NewBinaryReader(bytes.NewReader(utf16NameData))) // name table
	if err != nil {
		return nil, fmt.Errorf("failed to read utf16 name table: %w", err)
	}

	// Setup Arc
	arc := NewArc("")
	// Set name from root ID
	if rootName, ok := nameLUT[utf16HT.ID]; ok {
		// extract after the last separator if present
		base := rootName
		if i := lastIndexOfSep(rootName); i >= 0 && i+1 < len(rootName) {
			base = rootName[i+1:]
		}
		arc = NewArc(base)
	}

	// populateArc structure using UTF16 table
	if err := populateArc(arc, utf16HT, nameLUT, reader, headerEndPosition); err != nil {
		return nil, fmt.Errorf("failed to populateArc arc structure: %w", err)
	}

	return arc, nil
}

// populateArc builds the Arc from UTF16 hashtable and name lut
func populateArc(arc *Arc, t *hashTable, nameLUT map[uint64]string, reader *stream.BinaryReader, baseOffset int64) error {
	// recursively traverse
	var walk func(tab *hashTable, parent *Dir) error
	walk = func(tab *hashTable, parent *Dir) error {
		// files
		for _, fe := range tab.FileEntries {
			name, ok := nameLUT[fe.Hash]
			if !ok {
				return fmt.Errorf("missing name for file hash %x", fe.Hash)
			}
			f := AddFileByPath(parent, name)
			f.Arc = arc
			f.Ptr = NewArcPointer(reader, baseOffset+fe.Offset)
		}
		// dirs
		for _, de := range tab.DirEntries {
			name, ok := nameLUT[de.Hash]
			if !ok {
				return fmt.Errorf("missing name for dir hash %x", de.Hash)
			}
			d := GetOrCreateDirByPath(parent, name)
			d.Arc = arc
			// find matching subtable by ID
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

// Dump writes the Arc to an ARC file on disk
func (arc *Arc) Dump(path string) error {
	tmpDir := filepath.Dir(path)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		/* ignore on Windows perms */
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create ARC file: %w", err)
	}
	defer f.Close()

	writer := stream.NewBinaryWriter(f)

	// header + placeholder metadata offset
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

	// file table write
	fileOffsets := map[uint64]int64{}
	files := AllFiles(arc)
	// compile compress globs into regex
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
		// decide compression
		compress := false
		for _, p := range pats {
			if p.MatchString(fl.Name) {
				compress = true
				break
			}
		}
		data, err := fl.Ptr.Data()
		if err != nil {
			return fmt.Errorf("failed to read file data: %w", err)
		}
		raw := data
		enc := data
		if compress && !fl.Ptr.Compressed() {
			enc, err = deflateCompress(data)
			if err != nil {
				return fmt.Errorf("failed to compress file data: %w", err)
			}
		}
		pos, err := writer.Tell()
		if err != nil {
			return fmt.Errorf("failed to get file position: %w", err)
		}
		fileOffsets[fl.UniqueID()] = pos - baseOff
		// header
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
		if err := writer.WriteUInt32(uint32(len(raw))); err != nil {
			return fmt.Errorf("failed to write file header: %w", err)
		}
		if err := writer.WriteUInt32(uint32(len(enc))); err != nil {
			return fmt.Errorf("failed to write file header: %w", err)
		}
		if err := writer.WriteBytes(enc); err != nil {
			return fmt.Errorf("failed to write file data: %w", err)
		}
		_ = i // progress
	}

	// write metadata
	metadataPos, err := writer.Tell()
	if err != nil {
		return fmt.Errorf("failed to get metadata position: %w", err)
	}
	// patch metadata offset
	if _, err := writer.Seek(int64(len(arcHeader)), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to metadata offset: %w", err)
	}
	if err := writer.WriteInt64(metadataPos - baseOff); err != nil {
		return fmt.Errorf("failed to write metadata offset: %w", err)
	}
	if _, err := writer.Seek(metadataPos, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to metadata position: %w", err)
	}

	// build uuid->hash
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

	// calculate directory offsets for both tables
	dirOff16 := arc.calculateDirOffsets(uuidToHash16)
	dirOff8 := arc.calculateDirOffsets(uuidToHash8)

	// metadata block 0 (UTF16)
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

	// metadata block 1 (UTF8)
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

	// metadata block 3 (UTF16 name table, compressed)
	if err := arc.writeNameTable(bufWriter, true); err != nil {
		return fmt.Errorf("failed to write UTF16 name table: %w", err)
	}
	nameRaw := buf.Bytes()
	nameEnc, err := deflateCompress(nameRaw)
	if err != nil {
		return fmt.Errorf("failed to compress UTF16 name table: %w", err)
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
	if err := writer.WriteUInt32(uint32(len(nameRaw))); err != nil {
		return fmt.Errorf("failed to write UTF16 name table raw size: %w", err)
	}
	if err := writer.WriteUInt32(uint32(len(nameEnc))); err != nil {
		return fmt.Errorf("failed to write UTF16 name table compressed size: %w", err)
	}
	if err := writer.WriteBytes(nameEnc); err != nil {
		return fmt.Errorf("failed to write UTF16 name table compressed data: %w", err)
	}

	return nil
}

// calculateDirOffsets computes offset mapping for directories in an ARC file system based on their structure and depth.
func (arc *Arc) calculateDirOffsets(uuidToHash map[uint64]uint64) map[uint64]int64 {
	dict := map[uint64]int64{}
	var offset int64 = 0
	var rec func(d *Dir)
	rec = func(d *Dir) {
		// accumulate parent deltas
		var delta int64 = 0
		p := d.Parent
		for p != nil {
			delta += dict[p.UniqueID()]
			p = p.Parent
		}
		dict[d.UniqueID()] = offset - delta
		offset += 32 // header
		// 16 per entry (dir or file)
		cnt := len(d.Dirs) + len(d.Files)
		offset += int64(16 * cnt)
		// 8 per parent hash
		offset += int64(8 * d.Depth())
		// children ordered by hash, then by offset when writing
		// we follow order by uuidToHash
		children := d.sortedDirs()
		// stable sort by hash
		sort.Slice(children, func(i, j int) bool { return uuidToHash[children[i].UniqueID()] < uuidToHash[children[j].UniqueID()] })
		for _, sub := range children {
			rec(sub)
		}
	}
	rec(arc.Root)
	return dict
}

// MergeFrom merges src into this Arc. If keepDupes is true, use full path as key for files; otherwise last segment.
func (arc *Arc) MergeFrom(src *Arc, keepDupes bool) {
	arc.KeepDupes = keepDupes
	var walk func(d *Dir, into *Dir)
	walk = func(d *Dir, into *Dir) {
		for _, sub := range d.sortedDirs() {
			nd := into.GetOrCreateDir(sub.Name)
			walk(sub, nd)
		}
		for _, fl := range d.sortedFiles() {
			// if duplicate and not keep dupes, replace
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

// Pack loads all files from dirPath into an Arc structure and dumps it to arcPath
func Pack(dirPath string, arcPath string) error {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to getting absolute path for %q: %w", dirPath, err)
	}

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

		// Calculate relative path for use within the ARC
		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return fmt.Errorf("failed to calculating relative path for %q: %w", path, err)
		}

		// Read file data
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to reading file %q: %w", path, err)
		}

		// Add to Arc
		f := AddFileByPath(fs.Root, rel)
		f.Arc = fs
		f.Ptr = NewMemoryPointer(data)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory %q: %w", absDir, err)
	}

	// Dump to arcPath
	return fs.Dump(arcPath)
}

// Unpack extracts the entire Arc file system to the specified directory.
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

// Extract saves the file to the specified path.
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
