package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio"
)

func TestReadAssetsFileSupportsMetadataVersions12Through22(t *testing.T) {
	for version := uint32(12); version <= 22; version++ {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			metadata := buildAssetsMetadataForTest(t, version, []metadataAssetForTest{{pathID: 1, offset: 0, size: 1}}, nil)
			data := buildSerializedFileForMetadataTest(t, version, metadata, []byte{0}, 0)
			af, err := ReadAssetsFile(data)
			if err != nil {
				t.Fatalf("ReadAssetsFile v%d: %v", version, err)
			}
			if af.Header.Version != version || len(af.Metadata.TypeTreeTypes) != 1 || len(af.Metadata.AssetInfos) != 1 || af.Metadata.AssetInfos[0].TypeId != ClassIDTextAsset {
				t.Fatalf("unexpected v%d parse result: header=%+v types=%d assets=%d", version, af.Header, len(af.Metadata.TypeTreeTypes), len(af.Metadata.AssetInfos))
			}
		})
	}
}

func TestReadAssetsFileSupportsLegacyBigIDEnabled(t *testing.T) {
	const pathID = int64(math.MaxInt32) + 0x12345
	for _, version := range []uint32{12, 13} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			metadata := buildAssetsMetadataForTestWithBigID(t, version, 1, []metadataAssetForTest{{pathID: pathID, offset: 0, size: 1}}, nil)
			data := buildSerializedFileForMetadataTest(t, version, metadata, []byte{0}, 0)
			af, err := ReadAssetsFile(data)
			if err != nil {
				t.Fatalf("ReadAssetsFile v%d with bigIDEnabled: %v", version, err)
			}
			if af.Metadata.BigIDEnabled != 1 {
				t.Fatalf("BigIDEnabled = %d, want 1", af.Metadata.BigIDEnabled)
			}
			if len(af.Metadata.AssetInfos) != 1 || af.Metadata.AssetInfos[0].PathId != pathID {
				t.Fatalf("legacy 64-bit PathID got %+v, want %d", af.Metadata.AssetInfos, pathID)
			}
		})
	}
}

func TestReadAssetsFilePreservesFixedHeaderFields(t *testing.T) {
	metadata := buildAssetsMetadataForTest(t, 22, nil, nil)
	data := buildSerializedFileForMetadataTest(t, 22, metadata, nil, 0)
	binary.BigEndian.PutUint32(data[0:4], 0x10203040)
	binary.BigEndian.PutUint32(data[4:8], 0x50607080)
	binary.BigEndian.PutUint32(data[12:16], 0x90a0b0c0)
	copy(data[17:20], []byte{1, 2, 3})
	binary.BigEndian.PutUint64(data[40:48], uint64(0xfedcba9876543210))

	af, err := ReadAssetsFile(data)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	header := af.Header
	if header.LegacyMetadataSize != 0x10203040 || header.LegacyFileSize != 0x50607080 || header.LegacyDataOffset != 0x90a0b0c0 {
		t.Fatalf("legacy header fields were not preserved: %+v", header)
	}
	if header.Reserved != [3]byte{1, 2, 3} || header.LargeFilesUnknown != int64(-81985529216486896) {
		t.Fatalf("reserved or unknown header fields were not preserved: %+v", header)
	}
}

func TestReadAssetsFilePreservesLegacyObjectFields(t *testing.T) {
	for _, version := range []uint32{12, 15, 16} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			scriptTypeIndex := int16(23)
			asset := metadataAssetForTest{pathID: 1, offset: 0, size: 1, scriptTypeIndex: &scriptTypeIndex, stripped: 1}
			metadata := buildAssetsMetadataForTest(t, version, []metadataAssetForTest{asset}, nil)
			data := buildSerializedFileForMetadataTest(t, version, metadata, []byte{0}, 0)
			af, err := ReadAssetsFile(data)
			if err != nil {
				t.Fatalf("ReadAssetsFile: %v", err)
			}
			info := af.Metadata.AssetInfos[0]
			if info.ScriptTypeIndex != scriptTypeIndex {
				t.Fatalf("ScriptTypeIndex = %d, want %d", info.ScriptTypeIndex, scriptTypeIndex)
			}
			if version < 16 && info.LegacyClassID != uint16(ClassIDTextAsset) {
				t.Fatalf("LegacyClassID = %d, want %d", info.LegacyClassID, ClassIDTextAsset)
			}
			wantStripped := byte(0)
			if version >= 15 {
				wantStripped = 1
			}
			if info.Stripped != wantStripped {
				t.Fatalf("Stripped = %d, want %d", info.Stripped, wantStripped)
			}
		})
	}
}

func TestReadAssetsFileMetadataCannotConsumePaddingOrObjectData(t *testing.T) {
	fullMetadata := buildAssetsMetadataForTest(t, 22, nil, nil)
	// Remove script count, external count, ref type count, and UserInformation.
	// Zero padding/object bytes would have completed all four fields when the
	// metadata reader was incorrectly backed by the complete SerializedFile.
	metadata := fullMetadata[:len(fullMetadata)-13]
	data := buildSerializedFileForMetadataTest(t, 22, metadata, make([]byte, 32), 32)

	assertAssetsErrorWithoutPanic(t, func() error {
		_, err := ReadAssetsFile(data)
		return err
	}, "script type count")
}

func TestMetadataAlignmentAndSkipAreBoundsChecked(t *testing.T) {
	r := binaryio.NewEndianReaderAt(make([]byte, 3), 2, binary.LittleEndian)
	if err := alignMetadata4(r, "test asset"); err == nil || !strings.Contains(err.Error(), "alignment") {
		t.Fatalf("alignMetadata4 error = %v, want alignment bounds error", err)
	}
	if r.Pos() != 2 {
		t.Fatalf("failed alignment changed reader position to %d", r.Pos())
	}
	if err := skipMetadataBytes(r, 2, "test skip"); err == nil || !strings.Contains(err.Error(), "only 1") {
		t.Fatalf("skipMetadataBytes error = %v, want remaining-byte error", err)
	}
	if r.Pos() != 2 {
		t.Fatalf("failed skip changed reader position to %d", r.Pos())
	}
}

func TestReadAssetsFileValidatesHeaderAndMetadataRanges(t *testing.T) {
	metadata := buildAssetsMetadataForTest(t, 22, nil, nil)
	valid := buildSerializedFileForMetadataTest(t, 22, metadata, []byte{1, 2, 3, 4}, 0)

	tests := []struct {
		name   string
		mutate func([]byte) []byte
		want   string
	}{
		{
			name: "declared file smaller than actual",
			mutate: func(data []byte) []byte {
				return append(data, 0)
			},
			want: "does not match input length",
		},
		{
			name: "declared file larger than actual",
			mutate: func(data []byte) []byte {
				binary.BigEndian.PutUint64(data[24:32], uint64(len(data)+1))
				return data
			},
			want: "does not match input length",
		},
		{
			name: "uint64 file size overflow",
			mutate: func(data []byte) []byte {
				binary.BigEndian.PutUint64(data[24:32], math.MaxUint64)
				return data
			},
			want: "file size",
		},
		{
			name: "uint64 data offset overflow",
			mutate: func(data []byte) []byte {
				binary.BigEndian.PutUint64(data[32:40], math.MaxUint64)
				return data
			},
			want: "data offset",
		},
		{
			name: "metadata crosses data offset",
			mutate: func(data []byte) []byte {
				dataOffset := binary.BigEndian.Uint64(data[32:40])
				binary.BigEndian.PutUint32(data[20:24], uint32(dataOffset-48+1))
				return data
			},
			want: "metadata end",
		},
		{
			name: "metadata crosses file end",
			mutate: func(data []byte) []byte {
				binary.BigEndian.PutUint32(data[20:24], uint32(len(data)))
				return data
			},
			want: "metadata range",
		},
		{
			name: "data offset before header",
			mutate: func(data []byte) []byte {
				binary.BigEndian.PutUint64(data[32:40], 20)
				return data
			},
			want: "data offset",
		},
		{
			name: "invalid endian byte",
			mutate: func(data []byte) []byte {
				data[16] = 2
				return data
			},
			want: "endianness",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := append([]byte(nil), valid...)
			data = tc.mutate(data)
			assertAssetsErrorWithoutPanic(t, func() error {
				_, err := ReadAssetsFile(data)
				return err
			}, tc.want)
		})
	}
}

func TestReadAssetsFileRejectsForgedMetadataTailCounts(t *testing.T) {
	metadata := buildAssetsMetadataForTest(t, 22, nil, nil)
	// The standard empty v22 tail is scriptCount, externalCount,
	// refTypeCount, UserInformation: 4+4+4+1 bytes.
	tests := []struct {
		name   string
		offset int
		value  uint32
	}{
		{name: "negative script count", offset: len(metadata) - 13, value: math.MaxUint32},
		{name: "negative external count", offset: len(metadata) - 9, value: math.MaxUint32},
		{name: "negative reference count", offset: len(metadata) - 5, value: math.MaxUint32},
		{name: "forged reference count", offset: len(metadata) - 5, value: 1 << 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			malformed := append([]byte(nil), metadata...)
			binary.LittleEndian.PutUint32(malformed[tc.offset:tc.offset+4], tc.value)
			data := buildSerializedFileForMetadataTest(t, 22, malformed, nil, 0)
			assertAssetsErrorWithoutPanic(t, func() error {
				_, err := ReadAssetsFile(data)
				return err
			}, "count")
		})
	}
}

func TestReadAssetsFileParsesScriptTypesBeforeExternalFiles(t *testing.T) {
	metadata := buildAssetsMetadataForTest(t, 22, nil, nil)
	metadata = append([]byte(nil), metadata[:len(metadata)-13]...)
	var tail bytes.Buffer
	writeLEForAssetsTest(t, &tail, int32(2))
	for i := 0; i < 2; i++ {
		writeLEForAssetsTest(t, &tail, int32(i))
		for (len(metadata)+tail.Len())%4 != 0 {
			tail.WriteByte(0)
		}
		writeLEForAssetsTest(t, &tail, int64(100+i))
	}
	writeLEForAssetsTest(t, &tail, int32(1))
	tail.WriteByte(0)
	tail.Write(make([]byte, 16))
	writeLEForAssetsTest(t, &tail, int32(0))
	tail.WriteString("library/unity default resources\x00")
	writeLEForAssetsTest(t, &tail, int32(0))
	tail.WriteByte(0)
	metadata = append(metadata, tail.Bytes()...)

	data := buildSerializedFileForMetadataTest(t, 22, metadata, nil, 0)
	af, err := ReadAssetsFile(data)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	if len(af.Metadata.ScriptTypes) != 2 || af.Metadata.ScriptTypes[0].LocalIdentifierInFile != 100 || af.Metadata.ScriptTypes[1].LocalIdentifierInFile != 101 {
		t.Fatalf("script types parsed incorrectly: %+v", af.Metadata.ScriptTypes)
	}
	if len(af.Metadata.ExternalFiles) != 1 || af.Metadata.ExternalFiles[0].PathName != "library/unity default resources" {
		t.Fatalf("external files parsed at wrong tail offset: %+v (metadata=%+v)", af.Metadata.ExternalFiles, af.Metadata)
	}
}

func TestReadAssetsFileReadsFullMetadataTypeAndTailFields(t *testing.T) {
	const version = uint32(22)
	var metadata bytes.Buffer
	metadata.WriteString("2021.3.3f1\x00")
	writeLEForAssetsTest(t, &metadata, uint32(5))
	metadata.WriteByte(1) // TypeTreeEnabled

	writeLEForAssetsTest(t, &metadata, int32(1))
	writeSerializedTypeForMetadataTest(t, &metadata, ClassIDTextAsset, 0x1122334455667788)
	writeLEForAssetsTest(t, &metadata, int32(2))
	writeLEForAssetsTest(t, &metadata, int32(7))
	writeLEForAssetsTest(t, &metadata, int32(9))

	writeLEForAssetsTest(t, &metadata, int32(0)) // AssetInfos
	writeLEForAssetsTest(t, &metadata, int32(1)) // ScriptTypes
	writeLEForAssetsTest(t, &metadata, int32(3))
	for metadata.Len()%4 != 0 {
		metadata.WriteByte(0)
	}
	writeLEForAssetsTest(t, &metadata, int64(1234))

	writeLEForAssetsTest(t, &metadata, int32(1)) // ExternalFiles
	metadata.WriteString("virtual/cache/path\x00")
	var guid [16]byte
	for i := range guid {
		guid[i] = byte(i + 1)
	}
	metadata.Write(guid[:])
	writeLEForAssetsTest(t, &metadata, int32(3))
	metadata.WriteString("archive:/shared.assets\x00")

	writeLEForAssetsTest(t, &metadata, int32(1)) // RefTypes
	writeSerializedTypeForMetadataTest(t, &metadata, 1, 0x8877665544332211)
	metadata.WriteString("ReferencedClass\x00")
	metadata.WriteString("Example.Namespace\x00")
	metadata.WriteString("Example.Assembly\x00")
	metadata.WriteString("user information\x00")

	data := buildSerializedFileForMetadataTest(t, version, metadata.Bytes(), nil, 0)
	af, err := ReadAssetsFile(data)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	if got := af.Metadata.TypeTreeTypes[0].TypeDependencies; len(got) != 2 || got[0] != 7 || got[1] != 9 {
		t.Fatalf("TypeDependencies = %v, want [7 9]", got)
	}
	if got := af.Metadata.TypeTreeTypes[0].Nodes[0].RefTypeHash; got != 0x1122334455667788 {
		t.Fatalf("ordinary RefTypeHash = %#x", got)
	}
	if len(af.Metadata.ScriptTypes) != 1 || af.Metadata.ScriptTypes[0].LocalSerializedFileIndex != 3 || af.Metadata.ScriptTypes[0].LocalIdentifierInFile != 1234 {
		t.Fatalf("ScriptTypes = %+v", af.Metadata.ScriptTypes)
	}
	if len(af.Metadata.ExternalFiles) != 1 || af.Metadata.ExternalFiles[0].AssetPath != "virtual/cache/path" || af.Metadata.ExternalFiles[0].PathName != "archive:/shared.assets" || af.Metadata.ExternalFiles[0].Guid != guid {
		t.Fatalf("ExternalFiles = %+v", af.Metadata.ExternalFiles)
	}
	if len(af.Metadata.RefTypes) != 1 {
		t.Fatalf("RefTypes count = %d", len(af.Metadata.RefTypes))
	}
	refType := af.Metadata.RefTypes[0]
	if refType.ClassName != "ReferencedClass" || refType.Namespace != "Example.Namespace" || refType.AssemblyName != "Example.Assembly" || refType.Nodes[0].RefTypeHash != 0x8877665544332211 {
		t.Fatalf("RefType = %+v", refType)
	}
	if af.Metadata.UserInformation != "user information" {
		t.Fatalf("UserInformation=%q", af.Metadata.UserInformation)
	}

	metadataWithTrailing := append(append([]byte(nil), metadata.Bytes()...), 0xaa, 0xbb, 0xcc)
	dataWithTrailing := buildSerializedFileForMetadataTest(t, version, metadataWithTrailing, nil, 0)
	if _, err := ReadAssetsFile(dataWithTrailing); err == nil {
		t.Fatal("ReadAssetsFile accepted bytes after UserInformation")
	}
}

func writeSerializedTypeForMetadataTest(t *testing.T, metadata *bytes.Buffer, classID int32, refTypeHash uint64) {
	t.Helper()
	writeLEForAssetsTest(t, metadata, classID)
	metadata.WriteByte(0)
	writeLEForAssetsTest(t, metadata, int16(-1))
	metadata.Write(make([]byte, 16)) // TypeHash
	stringsBuffer := []byte("int\x00value\x00")
	writeLEForAssetsTest(t, metadata, int32(1))
	writeLEForAssetsTest(t, metadata, int32(len(stringsBuffer)))
	writeLEForAssetsTest(t, metadata, uint16(1))
	metadata.WriteByte(0)
	metadata.WriteByte(0)
	writeLEForAssetsTest(t, metadata, uint32(0))
	writeLEForAssetsTest(t, metadata, uint32(4))
	writeLEForAssetsTest(t, metadata, int32(4))
	writeLEForAssetsTest(t, metadata, int32(0))
	writeLEForAssetsTest(t, metadata, uint32(0))
	writeLEForAssetsTest(t, metadata, refTypeHash)
	metadata.Write(stringsBuffer)
}

func TestReadAssetsFileRejectsForgedTypeTreeCounts(t *testing.T) {
	stringBuffer := []byte("int\x00name\x00")
	metadata := buildTypeTreeOffsetMetadataForTest(t, 0, 4, stringBuffer)
	countsOffset := len("2021.3.3f1\x00") + 4 + 1 + 4 + 4 + 1 + 2 + 16
	dependencyOffset := countsOffset + 8 + 32 + len(stringBuffer)
	tests := []struct {
		name   string
		offset int
		value  uint32
		want   string
	}{
		{name: "negative node count", offset: countsOffset, value: math.MaxUint32, want: "negative type tree node count"},
		{name: "negative string buffer size", offset: countsOffset + 4, value: math.MaxUint32, want: "negative type tree string buffer size"},
		{name: "forged string buffer size", offset: countsOffset + 4, value: math.MaxInt32, want: "require"},
		{name: "negative dependency count", offset: dependencyOffset, value: math.MaxUint32, want: "dependency count"},
		{name: "forged dependency count", offset: dependencyOffset, value: 1 << 30, want: "dependency count"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			malformed := append([]byte(nil), metadata...)
			binary.LittleEndian.PutUint32(malformed[tc.offset:tc.offset+4], tc.value)
			data := buildSerializedFileForMetadataTest(t, 22, malformed, nil, 0)
			assertAssetsErrorWithoutPanic(t, func() error {
				_, err := ReadAssetsFile(data)
				return err
			}, tc.want)
		})
	}
}

func TestReadAssetsFileValidatesTypeTreeStringOffsets(t *testing.T) {
	tests := []struct {
		name       string
		typeOffset uint32
		nameOffset uint32
		strings    []byte
		wantError  string
	}{
		{
			name: "valid local strings", typeOffset: 0, nameOffset: 4,
			strings: []byte("int\x00name\x00"),
		},
		{
			name: "valid common strings", typeOffset: 0x80000000, nameOffset: 0x80000035,
			strings: nil,
		},
		{
			name: "type offset outside local buffer", typeOffset: 99, nameOffset: 0x80000000,
			strings: []byte("int\x00"), wantError: "outside local string buffer",
		},
		{
			name: "name offset outside local buffer", typeOffset: 0x80000000, nameOffset: 4,
			strings: []byte("int\x00"), wantError: "outside local string buffer",
		},
		{
			name: "local string lacks terminator", typeOffset: 0, nameOffset: 0x80000000,
			strings: []byte("unterminated"), wantError: "not null-terminated",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metadata := buildTypeTreeOffsetMetadataForTest(t, tc.typeOffset, tc.nameOffset, tc.strings)
			data := buildSerializedFileForMetadataTest(t, 22, metadata, nil, 0)
			_, err := ReadAssetsFile(data)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("ReadAssetsFile: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("ReadAssetsFile error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestReadAssetsFileValidatesAssetIdentityAndRanges(t *testing.T) {
	tests := []struct {
		name      string
		assets    []metadataAssetForTest
		dataSize  int
		wantError string
	}{
		{
			name: "valid adjacent ranges",
			assets: []metadataAssetForTest{
				{pathID: 1, offset: 0, size: 4},
				{pathID: 2, offset: 4, size: 4},
			},
			dataSize: 8,
		},
		{
			name: "duplicate path id",
			assets: []metadataAssetForTest{
				{pathID: 7, offset: 0, size: 2},
				{pathID: 7, offset: 2, size: 2},
			},
			dataSize: 4, wantError: "duplicate PathID",
		},
		{
			name: "overlapping ranges",
			assets: []metadataAssetForTest{
				{pathID: 1, offset: 0, size: 5},
				{pathID: 2, offset: 4, size: 4},
			},
			dataSize: 8, wantError: "ranges overlap",
		},
		{
			name: "range exceeds file",
			assets: []metadataAssetForTest{
				{pathID: 1, offset: 7, size: 2},
			},
			dataSize: 8, wantError: "exceeds data section",
		},
		{
			name: "offset exceeds int64",
			assets: []metadataAssetForTest{
				{pathID: 1, offset: math.MaxUint64, size: 0},
			},
			dataSize: 0, wantError: "exceeds int64",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metadata := buildAssetsMetadataForTest(t, 22, tc.assets, nil)
			data := buildSerializedFileForMetadataTest(t, 22, metadata, make([]byte, tc.dataSize), 0)
			_, err := ReadAssetsFile(data)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("ReadAssetsFile: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("ReadAssetsFile error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestReadAssetsFileAllReadableKCESSamples(t *testing.T) {
	if os.Getenv(abaHeavyTestEnv) == "" {
		t.Skipf("set %s=1 to parse every readable KCES ABA sample", abaHeavyTestEnv)
	}
	paths, err := filepath.Glob("../../../testdata/KCES/*.aba")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no KCES ABA samples")
	}
	parsed := 0
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("open %s: %v", filepath.Base(path), err)
			continue
		}
		abaFile, err := ReadAba(f)
		if err != nil {
			f.Close()
			if isEncryptedError(err) {
				continue
			}
			t.Errorf("ReadAba(%s): %v", filepath.Base(path), err)
			continue
		}
		for _, entry := range abaFile.BlockInfo.DirectoryInfos {
			if !entry.IsSerialized() {
				continue
			}
			data, err := readWholeAbaEntryForAssetsTest(abaFile, entry)
			if err != nil {
				t.Errorf("read %s/%s: %v", filepath.Base(path), entry.Name, err)
				continue
			}
			af, err := ReadAssetsFile(data)
			if err != nil {
				t.Errorf("ReadAssetsFile(%s/%s): %v", filepath.Base(path), entry.Name, err)
				continue
			}
			if af.Header.Version != 22 {
				t.Errorf("%s/%s version = %d, want KCES-observed v22", filepath.Base(path), entry.Name, af.Header.Version)
			}
			parsed++
		}
		f.Close()
	}
	const observedReadableKCESSamples = 27
	if parsed < observedReadableKCESSamples {
		t.Fatalf("parsed %d SerializedFiles, want at least the %d currently observed readable KCES samples", parsed, observedReadableKCESSamples)
	}
	t.Logf("parsed %d SerializedFiles from all readable KCES ABA samples", parsed)
}

type metadataAssetForTest struct {
	pathID          int64
	offset          uint64
	size            uint32
	scriptTypeIndex *int16
	stripped        byte
}

func buildAssetsMetadataForTest(t *testing.T, version uint32, assets []metadataAssetForTest, mutateType func(*bytes.Buffer)) []byte {
	return buildAssetsMetadataForTestWithBigID(t, version, 0, assets, mutateType)
}

func buildAssetsMetadataForTestWithBigID(t *testing.T, version uint32, bigIDEnabled int32, assets []metadataAssetForTest, mutateType func(*bytes.Buffer)) []byte {
	t.Helper()
	var metadata bytes.Buffer
	metadata.WriteString("2021.3.3f1\x00")
	writeLEForAssetsTest(t, &metadata, uint32(5))
	typeTreeEnabled := version < 13
	if version >= 13 {
		metadata.WriteByte(0)
	}
	writeLEForAssetsTest(t, &metadata, int32(1))
	writeLEForAssetsTest(t, &metadata, int32(ClassIDTextAsset))
	if version >= 16 {
		metadata.WriteByte(0)
	}
	if version >= 17 {
		writeLEForAssetsTest(t, &metadata, uint16(0xffff))
	}
	if version >= 13 {
		metadata.Write(make([]byte, 16))
	}
	if typeTreeEnabled {
		writeTypeTreeBlobForAssetsTest(t, &metadata, version, 0, 4, []byte("int\x00name\x00"))
	}
	if mutateType != nil {
		mutateType(&metadata)
	}
	if typeTreeEnabled && version >= 21 {
		writeLEForAssetsTest(t, &metadata, int32(0))
	}
	if version >= 7 && version < 14 {
		writeLEForAssetsTest(t, &metadata, bigIDEnabled)
	}

	writeLEForAssetsTest(t, &metadata, int32(len(assets)))
	for _, asset := range assets {
		if version >= 14 {
			for metadata.Len()%4 != 0 {
				metadata.WriteByte(0)
			}
		}
		if version >= 14 || bigIDEnabled != 0 {
			writeLEForAssetsTest(t, &metadata, asset.pathID)
		} else {
			writeLEForAssetsTest(t, &metadata, int32(asset.pathID))
		}
		if version >= 22 {
			writeLEForAssetsTest(t, &metadata, asset.offset)
		} else {
			writeLEForAssetsTest(t, &metadata, uint32(asset.offset))
		}
		writeLEForAssetsTest(t, &metadata, asset.size)
		writeLEForAssetsTest(t, &metadata, int32(0))
		if version < 16 {
			writeLEForAssetsTest(t, &metadata, int16(ClassIDTextAsset))
		}
		if version <= 16 {
			scriptTypeIndex := int16(-1)
			if asset.scriptTypeIndex != nil {
				scriptTypeIndex = *asset.scriptTypeIndex
			}
			writeLEForAssetsTest(t, &metadata, scriptTypeIndex)
		}
		if version >= 15 && version <= 16 {
			metadata.WriteByte(asset.stripped)
		}
	}

	writeLEForAssetsTest(t, &metadata, int32(0)) // script types
	writeLEForAssetsTest(t, &metadata, int32(0)) // external files
	if version >= 20 {
		writeLEForAssetsTest(t, &metadata, int32(0)) // reference types
	}
	metadata.WriteByte(0) // UserInformation
	return metadata.Bytes()
}

func buildTypeTreeOffsetMetadataForTest(t *testing.T, typeOffset, nameOffset uint32, stringBuffer []byte) []byte {
	t.Helper()
	var metadata bytes.Buffer
	metadata.WriteString("2021.3.3f1\x00")
	writeLEForAssetsTest(t, &metadata, uint32(5))
	metadata.WriteByte(1)
	writeLEForAssetsTest(t, &metadata, int32(1))
	writeLEForAssetsTest(t, &metadata, int32(ClassIDTextAsset))
	metadata.WriteByte(0)
	writeLEForAssetsTest(t, &metadata, uint16(0xffff))
	metadata.Write(make([]byte, 16))
	writeTypeTreeBlobForAssetsTest(t, &metadata, 22, typeOffset, nameOffset, stringBuffer)
	writeLEForAssetsTest(t, &metadata, int32(0)) // dependencies
	writeLEForAssetsTest(t, &metadata, int32(0)) // assets
	writeLEForAssetsTest(t, &metadata, int32(0)) // script types
	writeLEForAssetsTest(t, &metadata, int32(0)) // externals
	writeLEForAssetsTest(t, &metadata, int32(0)) // ref types
	metadata.WriteByte(0)
	return metadata.Bytes()
}

func writeTypeTreeBlobForAssetsTest(t *testing.T, metadata *bytes.Buffer, version uint32, typeOffset, nameOffset uint32, stringBuffer []byte) {
	t.Helper()
	writeLEForAssetsTest(t, metadata, int32(1))
	writeLEForAssetsTest(t, metadata, int32(len(stringBuffer)))
	writeLEForAssetsTest(t, metadata, uint16(1))
	metadata.WriteByte(0)
	metadata.WriteByte(0)
	writeLEForAssetsTest(t, metadata, typeOffset)
	writeLEForAssetsTest(t, metadata, nameOffset)
	writeLEForAssetsTest(t, metadata, int32(-1))
	writeLEForAssetsTest(t, metadata, int32(0))
	writeLEForAssetsTest(t, metadata, uint32(0))
	if version >= 19 {
		metadata.Write(make([]byte, 8))
	}
	metadata.Write(stringBuffer)
}

func buildSerializedFileForMetadataTest(t *testing.T, version uint32, metadata, objectData []byte, extraPadding int) []byte {
	t.Helper()
	headerSize := 20
	if version >= 22 {
		headerSize = 48
	}
	dataOffset := headerSize + len(metadata) + extraPadding
	if rem := dataOffset % 16; rem != 0 {
		dataOffset += 16 - rem
	}
	fileSize := dataOffset + len(objectData)
	data := make([]byte, fileSize)
	binary.BigEndian.PutUint32(data[0:4], uint32(len(metadata)))
	binary.BigEndian.PutUint32(data[4:8], uint32(fileSize))
	binary.BigEndian.PutUint32(data[8:12], version)
	binary.BigEndian.PutUint32(data[12:16], uint32(dataOffset))
	data[16] = 0
	if version >= 22 {
		binary.BigEndian.PutUint32(data[20:24], uint32(len(metadata)))
		binary.BigEndian.PutUint64(data[24:32], uint64(fileSize))
		binary.BigEndian.PutUint64(data[32:40], uint64(dataOffset))
	}
	copy(data[headerSize:], metadata)
	copy(data[dataOffset:], objectData)
	return data
}

func writeLEForAssetsTest(t *testing.T, dst *bytes.Buffer, value any) {
	t.Helper()
	if err := binary.Write(dst, binary.LittleEndian, value); err != nil {
		t.Fatal(err)
	}
}

func readWholeAbaEntryForAssetsTest(abaFile *Aba, entry DirectoryInfo) ([]byte, error) {
	if entry.DecompressedSize < 0 || uint64(entry.DecompressedSize) > uint64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("entry size %d cannot fit in memory", entry.DecompressedSize)
	}
	data := make([]byte, 0, int(entry.DecompressedSize))
	const chunkSize = int64(64 << 20)
	for offset := int64(0); offset < entry.DecompressedSize; {
		size := entry.DecompressedSize - offset
		if size > chunkSize {
			size = chunkSize
		}
		chunk, err := abaFile.GetFileDataRangeByName(entry.Name, offset, size)
		if err != nil {
			return nil, err
		}
		data = append(data, chunk...)
		offset += size
	}
	return data, nil
}
