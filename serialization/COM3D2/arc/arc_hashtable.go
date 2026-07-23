package arc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"unicode/utf16"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// makeCountedSliceForAppend 按线格式计数创建受限的初始切片容量，避免损坏计数触发巨额预分配
// makeCountedSliceForAppend creates a capped initial slice capacity from a wire count to prevent excessive preallocation from corrupt counts
func makeCountedSliceForAppend[T any](count int32) []T {
	if count <= 0 {
		return make([]T, 0)
	}
	const maxInitialCapacity = 1024
	capacity := count
	if capacity > maxInitialCapacity {
		capacity = maxInitialCapacity
	}
	return make([]T, 0, capacity)
}

// readHashTable 从二进制流读取哈希表，并返回构建的 hashTable 或错误
// readHashTable reads a hash table from a binary stream and returns a pointer to the constructed hashTable or an error
func readHashTable(reader *stream.BinaryReader) (*hashTable, error) {
	var ht hashTable
	header, err := reader.ReadInt64()
	if err != nil {
		return nil, fmt.Errorf("read header failed: %w", err)
	}
	ht.Header = header

	uid, err := reader.ReadUInt64()
	if err != nil {
		return nil, fmt.Errorf("read uid failed: %w", err)
	}
	ht.ID = uid

	dirCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read dir count failed: %w", err)
	}
	if dirCount < 0 {
		return nil, fmt.Errorf("directory count is negative: %d", dirCount)
	}
	ht.DirCount = dirCount

	fv, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read file count failed: %w", err)
	}
	if fv < 0 {
		return nil, fmt.Errorf("file count is negative: %d", fv)
	}
	ht.FileCount = fv

	depth, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read depth failed: %w", err)
	}
	if depth < 0 {
		return nil, fmt.Errorf("directory depth is negative: %d", depth)
	}
	ht.Depth = depth

	padding, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read padding failed: %w", err)
	}
	ht.Padding = padding

	ht.DirEntries = makeCountedSliceForAppend[fileEntryRec](ht.DirCount)
	for i := int32(0); i < ht.DirCount; i++ {
		var e fileEntryRec
		hash, err := reader.ReadUInt64()
		if err != nil {
			return nil, fmt.Errorf("read dir entry hash failed: %w", err)
		}
		offset, err := reader.ReadInt64()
		if err != nil {
			return nil, fmt.Errorf("read dir entry offset failed: %w", err)
		}
		e.Hash = hash
		e.Offset = offset
		ht.DirEntries = append(ht.DirEntries, e)
	}

	ht.FileEntries = makeCountedSliceForAppend[fileEntryRec](ht.FileCount)
	for i := int32(0); i < ht.FileCount; i++ {
		var e fileEntryRec
		hash, err := reader.ReadUInt64()
		if err != nil {
			return nil, fmt.Errorf("read file entry hash failed: %w", err)
		}
		offset, err := reader.ReadInt64()
		if err != nil {
			return nil, fmt.Errorf("read file entry offset failed: %w", err)
		}
		e.Hash = hash
		e.Offset = offset
		ht.FileEntries = append(ht.FileEntries, e)
	}
	ht.ParentsID = makeCountedSliceForAppend[uint64](ht.Depth)
	for i := int32(0); i < ht.Depth; i++ {
		parentHash, err := reader.ReadUInt64()
		if err != nil {
			return nil, fmt.Errorf("read parent hash failed: %w", err)
		}
		ht.ParentsID = append(ht.ParentsID, parentHash)
	}
	ht.SubDirEntries = makeCountedSliceForAppend[*hashTable](ht.DirCount)
	for i := int32(0); i < ht.DirCount; i++ {
		subDir, err := readHashTable(reader)
		if err != nil {
			return nil, fmt.Errorf("read subDir entry failed: %w", err)
		}
		ht.SubDirEntries = append(ht.SubDirEntries, subDir)
	}
	return &ht, nil
}

// readNameTable 从二进制流读取名称表并返回哈希到字符串的映射
// 到达流末尾或发生错误时停止读取
// readNameTable reads a table of names from a binary stream and returns a map of hashes to strings
// It stops reading when the end of the stream is reached or an error occurs
func readNameTable(reader *stream.BinaryReader) (map[uint64]string, error) {
	lut := make(map[uint64]string)
	for {
		nameHash, err := reader.ReadUInt64()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read name hash failed: %w", err)
		}

		nameSize, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read name size failed: %w", err)
		}
		if nameSize < 0 {
			return nil, fmt.Errorf("invalid name size")
		}

		utf16leString, err := reader.ReadBytes(int64(nameSize) * 2)
		if err != nil {
			return nil, fmt.Errorf("read name bytes failed: %w", err)
		}

		// 将 UTF-16LE 转换为字符串
		// Convert UTF-16LE to a string
		name := utf16leToString(utf16leString)
		if _, exists := lut[nameHash]; !exists {
			lut[nameHash] = name
		}
	}
	return lut, nil
}

// writeHashTable 将当前目录及其子目录的哈希表写入 BinaryWriter
// writeHashTable writes the hash table for the current directory and its subdirectories to the provided BinaryWriter
func (arc *Arc) writeHashTable(bw *stream.BinaryWriter, dirOffsets map[uint64]int64, uuidToHash map[uint64]uint64, fileOffsets map[uint64]int64, cur *Dir) error {
	dirCount, err := checkedArcInt32Count("directory count", int64(len(cur.Dirs)))
	if err != nil {
		return err
	}
	fileCount, err := checkedArcInt32Count("file count", int64(len(cur.Files)))
	if err != nil {
		return err
	}
	depth, err := checkedArcInt32Count("directory depth", cur.Depth())
	if err != nil {
		return err
	}
	if err := bw.WriteBytes(dirHeader); err != nil {
		return fmt.Errorf("write dir header failed: %w", err)
	}

	if err := bw.WriteUInt64(uuidToHash[cur.UniqueID()]); err != nil {
		return fmt.Errorf("write dir hash failed: %w", err)
	}

	if err := bw.WriteInt32(dirCount); err != nil {
		return fmt.Errorf("write dir count failed: %w", err)
	}

	if err := bw.WriteInt32(fileCount); err != nil {
		return fmt.Errorf("write file count failed: %w", err)
	}

	if err := bw.WriteInt32(depth); err != nil {
		return fmt.Errorf("write depth failed: %w", err)
	}

	if err := bw.WriteUInt32(0); err != nil {
		return fmt.Errorf("write padding failed: %w", err)
	}

	// 目录条目按 dirOffsets 排序
	// Directory entries ordered by dirOffsets
	dirs := cur.sortedDirs()
	sort.Slice(dirs, func(i, j int) bool { return dirOffsets[dirs[i].UniqueID()] < dirOffsets[dirs[j].UniqueID()] })
	for _, d := range dirs {
		if err := bw.WriteUInt64(uuidToHash[d.UniqueID()]); err != nil {
			return fmt.Errorf("write dir entry hash failed: %w", err)
		}
		if err := bw.WriteInt64(dirOffsets[d.UniqueID()]); err != nil {
			return fmt.Errorf("write dir entry offset failed: %w", err)
		}
	}

	// 文件条目按 uuidToHash 升序排序
	// File entries ordered by uuidToHash ascending
	files := cur.sortedFiles()
	sort.Slice(files, func(i, j int) bool { return uuidToHash[files[i].UniqueID()] < uuidToHash[files[j].UniqueID()] })
	for _, f := range files {
		if err := bw.WriteUInt64(uuidToHash[f.UniqueID()]); err != nil {
			return fmt.Errorf("write file entry hash failed: %w", err)
		}
		if err := bw.WriteInt64(fileOffsets[f.UniqueID()]); err != nil {
			return fmt.Errorf("write file entry offset failed: %w", err)
		}
	}

	// 父哈希从父目录到根目录收集后反向写入
	// 收集父目录
	// Parent hashes from parent up to root reversed
	// Collect parents
	var parents []uint64
	p := cur.Parent
	for p != nil {
		parents = append(parents, uuidToHash[p.UniqueID()])
		p = p.Parent
	}
	// 反向写入
	// Write reversed
	for i := int64(len(parents)) - 1; i >= 0; i-- {
		if err := bw.WriteUInt64(parents[i]); err != nil {
			return fmt.Errorf("write parent hash failed: %w", err)
		}
	}

	// 子表
	// Subtables
	for _, d := range dirs {
		if err := arc.writeHashTable(bw, dirOffsets, uuidToHash, fileOffsets, d); err != nil {
			return fmt.Errorf("write subDir entry failed: %w", err)
		}
	}
	return nil
}

// writeNameTable 将名称、哈希和 UTF-16LE 字节大小写入名称表
// writeNameTable writes the name table, including names, hashes, and their UTF-16LE encoded byte size, to the provided BinaryWriter
func (arc *Arc) writeNameTable(bw *stream.BinaryWriter, utf16 bool) error {
	// 收集文件、目录和根目录名称，按名称去重并保留顺序以保证确定性
	// Gather files, dirs, and root, distinct by name, preserving order for determinism
	var names []string
	seen := make(map[string]bool)
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}

	// 遵循 C# 顺序：FileCount、DirCount、Root
	// Follow C# order: FileCount, then DirCount, then Root
	for _, f := range AllFiles(arc) {
		add(f.Name)
	}
	for _, d := range AllDirs(arc) {
		add(d.Name)
	}
	add(arc.Root.Name)

	// 写出名称与哈希对
	// Write pairs
	for _, n := range names {
		var h uint64
		// C# 中的 Bytes 和 Size 始终使用 UTF-16LE 字节与字符数量
		// 只有 Hash 取决于 utf16 参数
		// In C#, Bytes and Size are always UTF-16LE and character count
		// Only the Hash depends on the utf16 parameter
		if utf16 {
			h = NameHashUTF16(n)
		} else {
			h = NameHashUTF8(n)
		}
		b := utf16le(n)
		sz, err := checkedArcInt32Count(fmt.Sprintf("name %q UTF-16 code-unit count", n), int64(len(b))/2)
		if err != nil {
			return err
		}

		if err := bw.WriteUInt64(h); err != nil {
			return fmt.Errorf("write name hash failed: %w", err)
		}
		if err := bw.WriteInt32(sz); err != nil {
			return fmt.Errorf("write name size failed: %w", err)
		}
		if err := bw.WriteBytes(b); err != nil {
			return fmt.Errorf("write name bytes failed: %w", err)
		}
	}
	return nil
}

// lastIndexOfSep 返回给定字符串中最后一个路径分隔符的索引
// 未找到分隔符时返回 -1
// lastIndexOfSep returns the index of the last occurrence of the file path separator in the given string
// If the separator is not found, it returns -1
func lastIndexOfSep(s string) int64 {
	sep := string(filepath.Separator)
	return int64(bytes.LastIndex([]byte(s), []byte(sep)))
}

// utf16leToString 将 UTF-16LE 字节切片转换为 UTF-8 字符串，长度为奇数时截断末尾字节
// utf16leToString converts a UTF-16LE encoded byte slice into a UTF-8 string, truncating one trailing byte if the length is odd
func utf16leToString(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	// 将字节对解码为 rune
	// Decode pairs into runes
	u16 := make([]uint16, len(b)/2)
	for i := 0; i < len(u16); i++ {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	// 转换为 rune
	// Convert to runes
	runes := utf16.Decode(u16)
	return string(runes)
}
