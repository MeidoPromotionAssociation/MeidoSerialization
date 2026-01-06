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
		return err
	}
	if err := writer.WriteInt32(0); err != nil {
		return err
	}
	if err := writer.WriteInt64(int64(buf.Len())); err != nil {
		return err
	}
	if err := writer.WriteBytes(buf.Bytes()); err != nil {
		return err
	}
	buf.Reset()

	// metadata block 1 (UTF8)
	if err := arc.writeHashTable(bufWriter, dirOff8, uuidToHash8, fileOffsets, arc.Root); err != nil {
		return err
	}
	if err := writer.WriteInt32(1); err != nil {
		return err
	}
	if err := writer.WriteInt64(int64(buf.Len())); err != nil {
		return err
	}
	if err := writer.WriteBytes(buf.Bytes()); err != nil {
		return err
	}
	buf.Reset()

	// metadata block 3 (UTF16 name table, compressed)
	if err := arc.writeNameTable(bufWriter, true); err != nil {
		return err
	}
	nameRaw := buf.Bytes()
	nameEnc, err := deflateCompress(nameRaw)
	if err != nil {
		return err
	}
	if err := writer.WriteInt32(3); err != nil {
		return err
	}
	if err := writer.WriteInt64(int64(len(nameEnc) + 16)); err != nil {
		return err
	}
	if err := writer.WriteUInt32(1); err != nil {
		return err
	}
	if err := writer.WriteUInt32(0); err != nil {
		return err
	}
	if err := writer.WriteUInt32(uint32(len(nameRaw))); err != nil {
		return err
	}
	if err := writer.WriteUInt32(uint32(len(nameEnc))); err != nil {
		return err
	}
	if err := writer.WriteBytes(nameEnc); err != nil {
		return err
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
			nf := &File{arc: arc, Name: fl.Name}
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

// globToRegex converts a shell-style glob pattern (e.g., "*", "?") into a corresponding regular expression.
func globToRegex(glob string) (*regexp.Regexp, error) {
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
	r, err := regexp.Compile(rx)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %s", glob)
	}
	return r, nil
}
