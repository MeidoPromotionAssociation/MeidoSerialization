package arc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf16"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

type fileEntryRec struct {
	Hash   uint64
	Offset int64
}

type hashTable struct {
	Header        int64
	ID            uint64
	Dirs          int32
	Files         int32
	Depth         int32
	Junk          int32
	DirEntries    []fileEntryRec
	FileEntries   []fileEntryRec
	Parents       []uint64
	SubDirEntries []*hashTable
}

func readHashTable(r io.Reader) (*hashTable, error) {
	br := &countedReader{r: r}
	var ht hashTable
	v, err := binaryio.ReadInt64(br)
	if err != nil {
		return nil, err
	}
	ht.Header = v
	uid, err := binaryio.ReadUInt64(br)
	if err != nil {
		return nil, err
	}
	ht.ID = uid
	dv, err := binaryio.ReadInt32(br)
	if err != nil {
		return nil, err
	}
	ht.Dirs = dv
	fv, err := binaryio.ReadInt32(br)
	if err != nil {
		return nil, err
	}
	ht.Files = fv
	dep, err := binaryio.ReadInt32(br)
	if err != nil {
		return nil, err
	}
	ht.Depth = dep
	junk, err := binaryio.ReadInt32(br)
	if err != nil {
		return nil, err
	}
	ht.Junk = junk
	ht.DirEntries = make([]fileEntryRec, ht.Dirs)
	for i := 0; i < int(ht.Dirs); i++ {
		var e fileEntryRec
		h, err := binaryio.ReadUInt64(br)
		if err != nil {
			return nil, err
		}
		off, err := binaryio.ReadInt64(br)
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
		h, err := binaryio.ReadUInt64(br)
		if err != nil {
			return nil, err
		}
		off, err := binaryio.ReadInt64(br)
		if err != nil {
			return nil, err
		}
		e.Hash = h
		e.Offset = off
		ht.FileEntries[i] = e
	}
	ht.Parents = make([]uint64, ht.Depth)
	for i := 0; i < int(ht.Depth); i++ {
		val, err := binaryio.ReadUInt64(br)
		if err != nil {
			return nil, err
		}
		ht.Parents[i] = val
	}
	ht.SubDirEntries = make([]*hashTable, ht.Dirs)
	for i := 0; i < int(ht.Dirs); i++ {
		sub, err := readHashTable(br)
		if err != nil {
			return nil, err
		}
		ht.SubDirEntries[i] = sub
	}
	return &ht, nil
}

type countedReader struct {
	r io.Reader
	n int64
}

func (c *countedReader) Read(p []byte) (int, error) {
	n, e := c.r.Read(p)
	c.n += int64(n)
	return n, e
}

// Detect checks if a file seems to be an ARC by verifying magic header
func Detect(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	hdr := make([]byte, len(arcHeader))
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false, err
	}
	return bytes.Equal(hdr, arcHeader), nil
}

// ReadArc parses an ARC file and returns an in-memory Arc representation
func ReadArc(path string) (*Arc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// header
	hdr := make([]byte, len(arcHeader))
	if _, err := io.ReadFull(f, hdr); err != nil {
		return nil, err
	}
	if !bytes.Equal(hdr, arcHeader) {
		return nil, fmt.Errorf("invalid ARC header")
	}

	// footer position
	footerRel, err := binaryio.ReadInt64(f)
	if err != nil {
		return nil, err
	}
	baseOff, _ := f.Seek(0, io.SeekCurrent)
	if _, err := f.Seek(baseOff+footerRel, io.SeekStart); err != nil {
		return nil, err
	}

	var utf8HashData, utf16HashData, utf16NameData []byte

	for utf8HashData == nil || utf16HashData == nil || utf16NameData == nil {
		blockType, err := binaryio.ReadInt32(f)
		if err != nil {
			return nil, err
		}
		blockSize, err := binaryio.ReadInt64(f)
		if err != nil {
			return nil, err
		}
		switch blockType {
		case 0: // utf16 hash
			buf := make([]byte, blockSize)
			if _, err := io.ReadFull(f, buf); err != nil {
				return nil, err
			}
			utf16HashData = buf
		case 1: // utf8 hash
			buf := make([]byte, blockSize)
			if _, err := io.ReadFull(f, buf); err != nil {
				return nil, err
			}
			utf8HashData = buf
		case 3: // name table as file block
			// read inline file header
			flag, err := binaryio.ReadUInt32(f)
			if err != nil {
				return nil, err
			}
			_, err = binaryio.ReadUInt32(f) // junk
			if err != nil {
				return nil, err
			}
			_, err = binaryio.ReadUInt32(f)
			if err != nil {
				return nil, err
			}
			enc, err := binaryio.ReadUInt32(f)
			if err != nil {
				return nil, err
			}
			data := make([]byte, enc)
			if _, err := io.ReadFull(f, data); err != nil {
				return nil, err
			}
			if flag == 1 {
				dec, err := deflateDecompress(data)
				if err != nil {
					return nil, err
				}
				utf16NameData = dec
			} else {
				utf16NameData = data
			}
		default:
			return nil, fmt.Errorf("unknown footer block type %d", blockType)
		}
	}

	// parse tables
	_, err = readHashTable(bytes.NewReader(utf8HashData))
	if err != nil {
		return nil, err
	}
	utf16HT, err := readHashTable(bytes.NewReader(utf16HashData))
	if err != nil {
		return nil, err
	}
	nameLUT, err := readNameTable(bytes.NewReader(utf16NameData))
	if err != nil {
		return nil, err
	}

	// Setup Arc
	fs := New("")
	// Set name from root ID
	if rootName, ok := nameLUT[utf16HT.ID]; ok {
		// extract after last separator if present
		base := rootName
		if i := lastIndexOfSep(rootName); i >= 0 && i+1 < len(rootName) {
			base = rootName[i+1:]
		}
		fs = New(base)
	}

	// populate structure using UTF16 table
	if err := populate(fs, utf16HT, nameLUT, path, baseOff); err != nil {
		return nil, err
	}

	// optional consistency check using utf8 table/name LUT omitted for brevity

	return fs, nil
}

func lastIndexOfSep(s string) int {
	sep := string(filepath.Separator)
	return bytes.LastIndex([]byte(s), []byte(sep))
}

func readNameTable(r io.Reader) (map[uint64]string, error) {
	br := &countedReader{r: r}
	lut := make(map[uint64]string)
	for {
		h, err := binaryio.ReadUInt64(br)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		sz, err := binaryio.ReadInt32(br)
		if err != nil {
			return nil, err
		}
		if sz < 0 {
			return nil, fmt.Errorf("invalid name size")
		}
		buf := make([]byte, int(sz)*2)
		if _, err := io.ReadFull(br, buf); err != nil {
			return nil, err
		}
		// UTF-16LE to string
		name := utf16leToString(buf)
		if _, exists := lut[h]; !exists {
			lut[h] = name
		}
	}
	return lut, nil
}

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
	// go stdlib utf16.Decode
	runes := make([]rune, 0, len(u16))
	// minimal implementation inline to avoid import cycle; we can reuse utf16.Decode but we already import in hash.go
	runes = utf16Decode(u16)
	return string(runes)
}

// minimal wrapper on utf16.Decode to avoid an extra import here
func utf16Decode(u16 []uint16) []rune {
	return utf16.Decode(u16)
}
