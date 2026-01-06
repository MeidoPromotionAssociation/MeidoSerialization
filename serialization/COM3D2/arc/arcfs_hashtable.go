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

// readHashTable reads a hash table from a binary stream and returns a pointer to the constructed hashTable or an error.
func readHashTable(reader *stream.BinaryReader) (*hashTable, error) {
	var ht hashTable
	v, err := reader.ReadInt64()
	if err != nil {
		return nil, err
	}
	ht.Header = v
	uid, err := reader.ReadUInt64()
	if err != nil {
		return nil, err
	}
	ht.ID = uid
	dv, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	ht.Dirs = dv
	fv, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	ht.Files = fv
	dep, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	ht.Depth = dep
	padding, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	ht.Padding = padding
	ht.DirEntries = make([]fileEntryRec, ht.Dirs)
	for i := 0; i < int(ht.Dirs); i++ {
		var e fileEntryRec
		h, err := reader.ReadUInt64()
		if err != nil {
			return nil, err
		}
		off, err := reader.ReadInt64()
		if err != nil {
			return nil, err
		}
		e.Hash = h
		e.Offset = off
		ht.DirEntries[i] = e
	}
	ht.FileEntries = make([]fileEntryRec, ht.Files)
	for i := 0; i < int(ht.Files); i++ {
		var e fileEntryRec
		h, err := reader.ReadUInt64()
		if err != nil {
			return nil, err
		}
		off, err := reader.ReadInt64()
		if err != nil {
			return nil, err
		}
		e.Hash = h
		e.Offset = off
		ht.FileEntries[i] = e
	}
	ht.Parents = make([]uint64, ht.Depth)
	for i := 0; i < int(ht.Depth); i++ {
		val, err := reader.ReadUInt64()
		if err != nil {
			return nil, err
		}
		ht.Parents[i] = val
	}
	ht.SubDirEntries = make([]*hashTable, ht.Dirs)
	for i := 0; i < int(ht.Dirs); i++ {
		sub, err := readHashTable(reader)
		if err != nil {
			return nil, err
		}
		ht.SubDirEntries[i] = sub
	}
	return &ht, nil
}

// readNameTable reads a table of names from a binary stream and returns a map of hashes to strings.
// It stops reading when the end of the stream is reached or an error occurs.
func readNameTable(reader *stream.BinaryReader) (map[uint64]string, error) {
	lut := make(map[uint64]string)
	for {
		h, err := reader.ReadUInt64()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		sz, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		if sz < 0 {
			return nil, fmt.Errorf("invalid name size")
		}
		utf16leString, err := reader.ReadBytes(int(sz) * 2)
		if err != nil {
			return nil, err
		}
		// UTF-16LE to string
		name := utf16leToString(utf16leString)
		if _, exists := lut[h]; !exists {
			lut[h] = name
		}
	}
	return lut, nil
}

// writeHashTable writes the hash table for the current directory and its subdirectories to the provided BinaryWriter.
func (arc *Arc) writeHashTable(bw *stream.BinaryWriter, dirOffsets map[uint64]int64, uuidToHash map[uint64]uint64, fileOffsets map[uint64]int64, cur *Dir) error {
	if err := bw.WriteBytes(dirHeader); err != nil {
		return err
	}
	if err := bw.WriteUInt64(uuidToHash[cur.UniqueID()]); err != nil {
		return err
	}
	if err := bw.WriteUInt32(uint32(len(cur.Dirs))); err != nil {
		return err
	}
	if err := bw.WriteUInt32(uint32(len(cur.Files))); err != nil {
		return err
	}
	if err := bw.WriteUInt32(uint32(cur.Depth())); err != nil {
		return err
	}
	if err := bw.WriteUInt32(0); err != nil {
		return err
	}

	// Directory entries ordered by dirOffsets
	dirs := cur.sortedDirs()
	sort.Slice(dirs, func(i, j int) bool { return dirOffsets[dirs[i].UniqueID()] < dirOffsets[dirs[j].UniqueID()] })
	for _, d := range dirs {
		if err := bw.WriteUInt64(uuidToHash[d.UniqueID()]); err != nil {
			return err
		}
		if err := bw.WriteInt64(dirOffsets[d.UniqueID()]); err != nil {
			return err
		}
	}

	// File entries ordered by uuidToHash ascending
	files := cur.sortedFiles()
	sort.Slice(files, func(i, j int) bool { return uuidToHash[files[i].UniqueID()] < uuidToHash[files[j].UniqueID()] })
	for _, f := range files {
		if err := bw.WriteUInt64(uuidToHash[f.UniqueID()]); err != nil {
			return err
		}
		if err := bw.WriteInt64(fileOffsets[f.UniqueID()]); err != nil {
			return err
		}
	}

	// Parent hashes from parent up to root reversed
	// collect parents
	var parents []uint64
	p := cur.Parent
	for p != nil {
		parents = append(parents, uuidToHash[p.UniqueID()])
		p = p.Parent
	}
	// write reversed
	for i := len(parents) - 1; i >= 0; i-- {
		if err := bw.WriteUInt64(parents[i]); err != nil {
			return err
		}
	}

	// Subtables
	for _, d := range dirs {
		if err := arc.writeHashTable(bw, dirOffsets, uuidToHash, fileOffsets, d); err != nil {
			return err
		}
	}
	return nil
}

// writeNameTable writes the name table, including names, hashes, and their UTF-16LE encoded byte size, to the provided BinaryWriter.
func (arc *Arc) writeNameTable(bw *stream.BinaryWriter, utf16 bool) error {
	// gather files, dirs, and root, distinct by name, preserving order for determinism
	var names []string
	seen := make(map[string]bool)
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}

	// Follow C# order: Files, then Dirs, then Root
	for _, f := range AllFiles(arc) {
		add(f.Name)
	}
	for _, d := range AllDirs(arc) {
		add(d.Name)
	}
	add(arc.Root.Name)

	// write pairs
	for _, n := range names {
		var h uint64
		// In C#, Bytes and Size are always UTF-16LE and character count
		// only the Hash depends on the utf16 parameter
		if utf16 {
			h = NameHashUTF16(n)
		} else {
			h = NameHashUTF8(n)
		}
		b := utf16le(n)
		sz := int32(len(b) / 2)

		if err := bw.WriteUInt64(h); err != nil {
			return err
		}
		if err := bw.WriteInt32(sz); err != nil {
			return err
		}
		if err := bw.WriteBytes(b); err != nil {
			return err
		}
	}
	return nil
}

// lastIndexOfSep returns the index of the last occurrence of the file path separator in the given string.
// If the separator is not found, it returns -1.
func lastIndexOfSep(s string) int {
	sep := string(filepath.Separator)
	return bytes.LastIndex([]byte(s), []byte(sep))
}

// utf16leToString converts a UTF-16LE encoded byte slice into a string (utf-8), truncating one trailing byte if the length is odd.
func utf16leToString(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	// decode pairs into runes
	u16 := make([]uint16, len(b)/2)
	for i := 0; i < len(u16); i++ {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	// convert to runes
	runes := utf16.Decode(u16)
	return string(runes)
}
