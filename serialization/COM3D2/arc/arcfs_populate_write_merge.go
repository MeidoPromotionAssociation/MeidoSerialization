package arc

import (
	"bytes"
	"fmt"
	"io"
	_ "math"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// populate builds the Arc from UTF16 hashtable and name lut
func populate(fs *Arc, t *hashTable, nameLUT map[uint64]string, arcPath string, baseOffset int64) error {
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
			f.fs = fs
			f.Ptr = NewArcPointer(arcPath, baseOffset+fe.Offset)
		}
		// dirs
		for _, de := range tab.DirEntries {
			name, ok := nameLUT[de.Hash]
			if !ok {
				return fmt.Errorf("missing name for dir hash %x", de.Hash)
			}
			d := GetOrCreateDirByPath(parent, name)
			d.fs = fs
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
	return walk(t, fs.Root)
}

// Dump writes the Arc to an ARC file on disk
func (fs *Arc) Dump(path string) error {
	tmpDir := filepath.Dir(path)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		/* ignore on Windows perms */
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// header + placeholder footer offset
	if _, err := f.Write(arcHeader); err != nil {
		return err
	}
	if err := binaryio.WriteInt64(f, 0); err != nil {
		return err
	}
	baseOff, _ := f.Seek(0, io.SeekCurrent)

	// file table write
	fileOffsets := map[uint64]int64{}
	files := AllFiles(fs)
	// compile compress globs into regex
	var pats []*regexp.Regexp
	for _, g := range fs.CompressGlobs {
		if g == "" {
			continue
		}
		pats = append(pats, globToRegex(g))
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
			return err
		}
		raw := data
		enc := data
		if compress && !fl.Ptr.Compressed() {
			enc, err = deflateCompress(data)
			if err != nil {
				return err
			}
		}
		fileOffsets[fl.UniqueID()] = mustPos(f) - baseOff
		// header
		flag := uint32(0)
		if compress {
			flag = 1
		}
		if err := binaryio.WriteUInt32(f, flag); err != nil {
			return err
		}
		if err := binaryio.WriteUInt32(f, 0); err != nil {
			return err
		}
		if err := binaryio.WriteUInt32(f, uint32(len(raw))); err != nil {
			return err
		}
		if err := binaryio.WriteUInt32(f, uint32(len(enc))); err != nil {
			return err
		}
		if _, err := f.Write(enc); err != nil {
			return err
		}
		_ = i // progress
	}

	// write footer
	footerPos := mustPos(f)
	// patch footer offset
	if _, err := f.Seek(int64(len(arcHeader)), io.SeekStart); err != nil {
		return err
	}
	if err := binaryio.WriteInt64(f, footerPos-baseOff); err != nil {
		return err
	}
	if _, err := f.Seek(footerPos, io.SeekStart); err != nil {
		return err
	}

	// build uuid->hash
	uuidToHash16 := map[uint64]uint64{}
	uuidToHash8 := map[uint64]uint64{}
	for _, d := range AllDirs(fs) {
		uuidToHash16[d.UniqueID()] = d.UTF16Hash()
		uuidToHash8[d.UniqueID()] = d.UTF8Hash()
	}
	for _, fl := range AllFiles(fs) {
		uuidToHash16[fl.UniqueID()] = fl.UTF16Hash()
		uuidToHash8[fl.UniqueID()] = fl.UTF8Hash()
	}
	uuidToHash16[fs.Root.UniqueID()] = fs.Root.UTF16Hash()
	uuidToHash8[fs.Root.UniqueID()] = fs.Root.UTF8Hash()

	// calculate directory offsets for both tables
	dirOff16 := fs.calculateDirOffsets(uuidToHash16)
	dirOff8 := fs.calculateDirOffsets(uuidToHash8)

	// Footer block 0 (UTF16)
	var buf bytes.Buffer
	if err := fs.writeHashTable(&buf, dirOff16, uuidToHash16, fileOffsets, fs.Root); err != nil {
		return err
	}
	if err := binaryio.WriteInt32(f, 0); err != nil {
		return err
	}
	if err := binaryio.WriteInt64(f, int64(buf.Len())); err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		return err
	}
	buf.Reset()

	// Footer block 1 (UTF8)
	if err := fs.writeHashTable(&buf, dirOff8, uuidToHash8, fileOffsets, fs.Root); err != nil {
		return err
	}
	if err := binaryio.WriteInt32(f, 1); err != nil {
		return err
	}
	if err := binaryio.WriteInt64(f, int64(buf.Len())); err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		return err
	}
	buf.Reset()

	// Footer block 3 (UTF16 name table, compressed)
	if err := fs.writeNameTable(&buf, true); err != nil {
		return err
	}
	nameRaw := buf.Bytes()
	nameEnc, err := deflateCompress(nameRaw)
	if err != nil {
		return err
	}
	if err := binaryio.WriteInt32(f, 3); err != nil {
		return err
	}
	if err := binaryio.WriteInt64(f, int64(len(nameEnc)+16)); err != nil {
		return err
	}
	if err := binaryio.WriteUInt32(f, 1); err != nil {
		return err
	}
	if err := binaryio.WriteUInt32(f, 0); err != nil {
		return err
	}
	if err := binaryio.WriteUInt32(f, uint32(len(nameRaw))); err != nil {
		return err
	}
	if err := binaryio.WriteUInt32(f, uint32(len(nameEnc))); err != nil {
		return err
	}
	if _, err := f.Write(nameEnc); err != nil {
		return err
	}

	return nil
}

func mustPos(f *os.File) int64 {
	p, _ := f.Seek(0, io.SeekCurrent)
	return p
}

func (fs *Arc) calculateDirOffsets(uuidToHash map[uint64]uint64) map[uint64]int64 {
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
	rec(fs.Root)
	return dict
}

func (fs *Arc) writeHashTable(w io.Writer, dirOffsets map[uint64]int64, uuidToHash map[uint64]uint64, fileOffsets map[uint64]int64, cur *Dir) error {
	if _, err := w.Write(dirHeader); err != nil {
		return err
	}
	if err := binaryio.WriteUInt64(w, uuidToHash[cur.UniqueID()]); err != nil {
		return err
	}
	if err := binaryio.WriteUInt32(w, uint32(len(cur.Dirs))); err != nil {
		return err
	}
	if err := binaryio.WriteUInt32(w, uint32(len(cur.Files))); err != nil {
		return err
	}
	if err := binaryio.WriteUInt32(w, uint32(cur.Depth())); err != nil {
		return err
	}
	if err := binaryio.WriteUInt32(w, 0); err != nil {
		return err
	}

	// Directory entries ordered by dirOffsets
	dirs := cur.sortedDirs()
	sort.Slice(dirs, func(i, j int) bool { return dirOffsets[dirs[i].UniqueID()] < dirOffsets[dirs[j].UniqueID()] })
	for _, d := range dirs {
		if err := binaryio.WriteUInt64(w, uuidToHash[d.UniqueID()]); err != nil {
			return err
		}
		if err := binaryio.WriteInt64(w, dirOffsets[d.UniqueID()]); err != nil {
			return err
		}
	}

	// File entries ordered by uuidToHash ascending
	files := cur.sortedFiles()
	sort.Slice(files, func(i, j int) bool { return uuidToHash[files[i].UniqueID()] < uuidToHash[files[j].UniqueID()] })
	for _, f := range files {
		if err := binaryio.WriteUInt64(w, uuidToHash[f.UniqueID()]); err != nil {
			return err
		}
		if err := binaryio.WriteInt64(w, fileOffsets[f.UniqueID()]); err != nil {
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
		if err := binaryio.WriteUInt64(w, parents[i]); err != nil {
			return err
		}
	}

	// Subtables
	for _, d := range dirs {
		if err := fs.writeHashTable(w, dirOffsets, uuidToHash, fileOffsets, d); err != nil {
			return err
		}
	}
	return nil
}

func (fs *Arc) writeNameTable(w io.Writer, utf16 bool) error {
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
	for _, f := range AllFiles(fs) {
		add(f.Name)
	}
	for _, d := range AllDirs(fs) {
		add(d.Name)
	}
	add(fs.Root.Name)

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

		if err := binaryio.WriteUInt64(w, h); err != nil {
			return err
		}
		if err := binaryio.WriteInt32(w, sz); err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return nil
}

// MergeFrom merges src into this Arc. If keepDupes is true, use full path as key for files; otherwise last segment.
func (fs *Arc) MergeFrom(src *Arc, keepDupes bool) {
	fs.KeepDupes = keepDupes
	var walk func(d *Dir, into *Dir)
	walk = func(d *Dir, into *Dir) {
		for _, sub := range d.sortedDirs() {
			nd := into.GetOrCreateDir(sub.Name)
			walk(sub, nd)
		}
		for _, fl := range d.sortedFiles() {
			// if duplicate and not keep dupes, replace
			nf := &File{fs: fs, Name: fl.Name}
			data, _ := fl.Ptr.Data()
			if fl.Ptr.Compressed() {
				nf.Ptr = NewMemoryPointerCompressed(data, fl.Ptr.RawSize())
			} else {
				nf.Ptr = NewMemoryPointer(data)
			}
			into.AddFile(nf)
		}
	}
	walk(src.Root, fs.Root)
}

// Helpers
func globToRegex(glob string) *regexp.Regexp {
	// very simple translation: * -> .*, ? -> .
	// escape regex meta
	rx := "^"
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			rx += ".*"
		case '?':
			rx += "."
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			rx += "\\" + string(c)
		default:
			rx += string(c)
		}
	}
	rx += "$"
	r, _ := regexp.Compile(rx)
	return r
}
