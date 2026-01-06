package arc

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

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
	_, err = readHashTable(stream.NewBinaryReader(bytes.NewReader(utf8HashData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read utf8 hash table: %w", err)
	}
	utf16HT, err := readHashTable(stream.NewBinaryReader(bytes.NewReader(utf16HashData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read utf16 hash table: %w", err)
	}
	nameLUT, err := readNameTable(stream.NewBinaryReader(bytes.NewReader(utf16NameData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read utf16 name table: %w", err)
	}

	// Setup Arc
	arc := NewArc("")
	// Set name from root ID
	if rootName, ok := nameLUT[utf16HT.ID]; ok {
		// extract after last separator if present
		base := rootName
		if i := lastIndexOfSep(rootName); i >= 0 && i+1 < len(rootName) {
			base = rootName[i+1:]
		}
		arc = NewArc(base)
	}

	// populate structure using UTF16 table
	if err := populate(arc, utf16HT, nameLUT, reader, headerEndPosition); err != nil {
		return nil, fmt.Errorf("failed to populate arc structure: %w", err)
	}

	// optional consistency check using utf8 table/name LUT omitted for brevity

	return arc, nil
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

// populate builds the Arc from UTF16 hashtable and name lut
func populate(arc *Arc, t *hashTable, nameLUT map[uint64]string, reader *stream.BinaryReader, baseOffset int64) error {
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
			f.arc = arc
			f.Ptr = NewArcPointer(reader, baseOffset+fe.Offset)
		}
		// dirs
		for _, de := range tab.DirEntries {
			name, ok := nameLUT[de.Hash]
			if !ok {
				return fmt.Errorf("missing name for dir hash %x", de.Hash)
			}
			d := GetOrCreateDirByPath(parent, name)
			d.arc = arc
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
