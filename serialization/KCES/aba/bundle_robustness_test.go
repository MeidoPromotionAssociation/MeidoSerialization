package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ulikunitz/xz/lzma"
)

type testBundleImage struct {
	data           []byte
	fsHeaderOffset int
	infoOffset     int
}

func makeTestBundle(t *testing.T, blocks []BlockInfo, dirs []DirectoryInfo, data []byte) testBundleImage {
	t.Helper()
	info, err := serializeBlockAndDirInfo(blocks, dirs)
	if err != nil {
		t.Fatalf("serializeBlockAndDirInfo: %v", err)
	}
	return makeTestBundleWithInfo(t, info, uint32(len(info)), uint32(FlagHasDirectoryInfo), data)
}

func makeTestBundleWithInfo(t *testing.T, storedInfo []byte, decompressedInfoSize uint32, flags uint32, data []byte) testBundleImage {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("UnityFS\x00")
	if err := binary.Write(&buf, binary.BigEndian, uint32(7)); err != nil {
		t.Fatal(err)
	}
	buf.WriteString("5.x.x\x00")
	buf.WriteString("2021.3.3f1\x00")
	fsHeaderOffset := buf.Len()
	if err := binary.Write(&buf, binary.BigEndian, int64(0)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(storedInfo))); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&buf, binary.BigEndian, decompressedInfoSize); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&buf, binary.BigEndian, flags); err != nil {
		t.Fatal(err)
	}
	for buf.Len()%16 != 0 {
		buf.WriteByte(0)
	}
	infoOffset := buf.Len()
	buf.Write(storedInfo)
	buf.Write(data)
	out := buf.Bytes()
	binary.BigEndian.PutUint64(out[fsHeaderOffset:], uint64(len(out)))
	return testBundleImage{data: out, fsHeaderOffset: fsHeaderOffset, infoOffset: infoOffset}
}

func assertReadBundleErrorNoPanic(t *testing.T, data []byte) error {
	t.Helper()
	var err error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("ReadBundle panicked: %v", recovered)
			}
		}()
		_, err = ReadBundle(bytes.NewReader(data))
	}()
	if err == nil {
		t.Fatal("ReadBundle unexpectedly accepted malformed input")
	}
	return err
}

func relocateTestInfoToEnd(t *testing.T, image testBundleImage) []byte {
	t.Helper()
	infoSize := int(binary.BigEndian.Uint32(image.data[image.fsHeaderOffset+8:]))
	infoEnd := image.infoOffset + infoSize
	out := make([]byte, 0, len(image.data))
	out = append(out, image.data[:image.infoOffset]...)
	out = append(out, image.data[infoEnd:]...)
	out = append(out, image.data[image.infoOffset:infoEnd]...)
	flags := binary.BigEndian.Uint32(out[image.fsHeaderOffset+16:]) | uint32(FlagBlockAndDirAtEnd)
	binary.BigEndian.PutUint32(out[image.fsHeaderOffset+16:], flags)
	return out
}

func addTestDataStartPadding(t *testing.T, image testBundleImage) []byte {
	t.Helper()
	infoSize := int(binary.BigEndian.Uint32(image.data[image.fsHeaderOffset+8:]))
	dataOffset := image.infoOffset + infoSize
	padding := (16 - dataOffset%16) % 16
	out := make([]byte, 0, len(image.data)+padding)
	out = append(out, image.data[:dataOffset]...)
	out = append(out, make([]byte, padding)...)
	out = append(out, image.data[dataOffset:]...)
	binary.BigEndian.PutUint64(out[image.fsHeaderOffset:], uint64(len(out)))
	flags := binary.BigEndian.Uint32(out[image.fsHeaderOffset+16:]) | uint32(FlagBlockInfoNeedPaddingAtStart)
	binary.BigEndian.PutUint32(out[image.fsHeaderOffset+16:], flags)
	return out
}

func TestReadBundleSupportsInfoAtEndAndDataPaddingLayouts(t *testing.T) {
	image := makeTestBundle(t,
		[]BlockInfo{{DecompressedSize: 4, CompressedSize: 4, Flags: 0x40}},
		[]DirectoryInfo{{Offset: 0, DecompressedSize: 4, Name: "file"}},
		[]byte("data"),
	)
	for name, data := range map[string][]byte{
		"info at end":       relocateTestInfoToEnd(t, image),
		"padded data start": addTestDataStartPadding(t, image),
	} {
		t.Run(name, func(t *testing.T) {
			bundle, err := ReadBundle(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("ReadBundle: %v", err)
			}
			got, err := bundle.GetFileData(0)
			if err != nil {
				t.Fatalf("GetFileData: %v", err)
			}
			if string(got) != "data" {
				t.Fatalf("data=%q", got)
			}
		})
	}
}

func TestReadBundleRejectsEveryTruncatedPrefixWithoutPanic(t *testing.T) {
	image := makeTestBundle(t,
		[]BlockInfo{{DecompressedSize: 4, CompressedSize: 4, Flags: 0x40}},
		[]DirectoryInfo{{Offset: 0, DecompressedSize: 4, Name: "CAB-test", Flags: DirFlagSerializedFile}},
		[]byte("data"),
	)
	for length := 0; length < len(image.data); length++ {
		t.Run("prefix_"+strconv.Itoa(length), func(t *testing.T) {
			assertReadBundleErrorNoPanic(t, image.data[:length])
		})
	}
}

func TestReadBundleRejectsHeaderAndMetadataResourceAttacks(t *testing.T) {
	valid := makeTestBundle(t,
		[]BlockInfo{{DecompressedSize: 1, CompressedSize: 1, Flags: 0x40}},
		[]DirectoryInfo{{Offset: 0, DecompressedSize: 1, Name: "a"}},
		[]byte{1},
	)

	t.Run("trailing file bytes", func(t *testing.T) {
		assertReadBundleErrorNoPanic(t, append(append([]byte(nil), valid.data...), 0))
	})
	t.Run("oversized metadata declaration", func(t *testing.T) {
		data := append([]byte(nil), valid.data...)
		binary.BigEndian.PutUint32(data[valid.fsHeaderOffset+12:], maxBlockAndDirInfoSize+1)
		assertReadBundleErrorNoPanic(t, data)
	})
	t.Run("corrupt LZMA metadata", func(t *testing.T) {
		data := append([]byte(nil), valid.data...)
		binary.BigEndian.PutUint32(data[valid.fsHeaderOffset+16:], uint32(FlagHasDirectoryInfo|CompressionLZMA))
		err := assertReadBundleErrorNoPanic(t, data)
		if !strings.Contains(err.Error(), "LZMA") {
			t.Fatalf("error does not identify LZMA: %v", err)
		}
	})
	t.Run("corrupt LZ4 metadata", func(t *testing.T) {
		data := append([]byte(nil), valid.data...)
		binary.BigEndian.PutUint32(data[valid.fsHeaderOffset+16:], uint32(FlagHasDirectoryInfo|CompressionLZ4))
		assertReadBundleErrorNoPanic(t, data)
	})
	t.Run("uncompressed metadata size mismatch", func(t *testing.T) {
		data := append([]byte(nil), valid.data...)
		declared := binary.BigEndian.Uint32(data[valid.fsHeaderOffset+12:])
		binary.BigEndian.PutUint32(data[valid.fsHeaderOffset+12:], declared+1)
		assertReadBundleErrorNoPanic(t, data)
	})
	t.Run("unterminated signature", func(t *testing.T) {
		assertReadBundleErrorNoPanic(t, bytes.Repeat([]byte{'x'}, 4098))
	})

	t.Run("impossible block count", func(t *testing.T) {
		var info bytes.Buffer
		info.Write(make([]byte, 16))
		binary.Write(&info, binary.BigEndian, int32((1<<20)+1))
		binary.Write(&info, binary.BigEndian, int32(0))
		image := makeTestBundleWithInfo(t, info.Bytes(), uint32(info.Len()), uint32(FlagHasDirectoryInfo), nil)
		assertReadBundleErrorNoPanic(t, image.data)
	})
	t.Run("impossible directory count", func(t *testing.T) {
		var info bytes.Buffer
		info.Write(make([]byte, 16))
		binary.Write(&info, binary.BigEndian, int32(0))
		binary.Write(&info, binary.BigEndian, int32((1<<20)+1))
		image := makeTestBundleWithInfo(t, info.Bytes(), uint32(info.Len()), uint32(FlagHasDirectoryInfo), nil)
		assertReadBundleErrorNoPanic(t, image.data)
	})
}

func TestReadBundleRejectsInvalidBlockAndDirectoryLayout(t *testing.T) {
	tests := []struct {
		name   string
		blocks []BlockInfo
		dirs   []DirectoryInfo
		data   []byte
	}{
		{
			name:   "uncompressed block size mismatch",
			blocks: []BlockInfo{{DecompressedSize: 2, CompressedSize: 1, Flags: 0x40}},
			data:   []byte{1},
		},
		{
			name:   "directory outside decompressed stream",
			blocks: []BlockInfo{{DecompressedSize: 4, CompressedSize: 4, Flags: 0x40}},
			dirs:   []DirectoryInfo{{Offset: 3, DecompressedSize: 2, Name: "outside"}},
			data:   []byte("data"),
		},
		{
			name:   "directory offset overflow",
			blocks: []BlockInfo{{DecompressedSize: 4, CompressedSize: 4, Flags: 0x40}},
			dirs:   []DirectoryInfo{{Offset: int64(^uint64(0) >> 1), DecompressedSize: 1, Name: "overflow"}},
			data:   []byte("data"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			image := makeTestBundle(t, test.blocks, test.dirs, test.data)
			assertReadBundleErrorNoPanic(t, image.data)
		})
	}

	t.Run("negative directory offset", func(t *testing.T) {
		image := makeTestBundle(t,
			[]BlockInfo{{DecompressedSize: 1, CompressedSize: 1, Flags: 0x40}},
			[]DirectoryInfo{{Offset: 0, DecompressedSize: 1, Name: "negative"}},
			[]byte{1},
		)
		const directoryOffsetWithinInfo = 16 + 4 + 10 + 4
		binary.BigEndian.PutUint64(image.data[image.infoOffset+directoryOffsetWithinInfo:], ^uint64(0))
		assertReadBundleErrorNoPanic(t, image.data)
	})
	t.Run("negative directory size", func(t *testing.T) {
		image := makeTestBundle(t,
			[]BlockInfo{{DecompressedSize: 1, CompressedSize: 1, Flags: 0x40}},
			[]DirectoryInfo{{Offset: 0, DecompressedSize: 1, Name: "negative"}},
			[]byte{1},
		)
		const directorySizeWithinInfo = 16 + 4 + 10 + 4 + 8
		binary.BigEndian.PutUint64(image.data[image.infoOffset+directorySizeWithinInfo:], ^uint64(0))
		assertReadBundleErrorNoPanic(t, image.data)
	})
	t.Run("oversized individual block", func(t *testing.T) {
		image := makeTestBundle(t,
			[]BlockInfo{{DecompressedSize: 1, CompressedSize: 1, Flags: 0x40}}, nil,
			[]byte{1},
		)
		const blockSizeWithinInfo = 16 + 4
		binary.BigEndian.PutUint32(image.data[image.infoOffset+blockSizeWithinInfo:], maxBundleBlockSize+1)
		binary.BigEndian.PutUint32(image.data[image.infoOffset+blockSizeWithinInfo+4:], maxBundleBlockSize+1)
		assertReadBundleErrorNoPanic(t, image.data)
	})
	t.Run("duplicate and overlapping directories remain readable", func(t *testing.T) {
		image := makeTestBundle(t,
			[]BlockInfo{{DecompressedSize: 8, CompressedSize: 8, Flags: 0x40}},
			[]DirectoryInfo{
				{Offset: 0, DecompressedSize: 6, Name: "same"},
				{Offset: 4, DecompressedSize: 4, Name: "same"},
			},
			[]byte("12345678"),
		)
		bundle, err := ReadBundle(bytes.NewReader(image.data))
		if err != nil {
			t.Fatalf("ReadBundle: %v", err)
		}
		second, err := bundle.GetFileData(1)
		if err != nil {
			t.Fatalf("GetFileData(1): %v", err)
		}
		if string(second) != "5678" {
			t.Fatalf("second aliased range = %q, want %q", second, "5678")
		}
	})
	t.Run("trailing metadata bytes are preserved", func(t *testing.T) {
		info, err := serializeBlockAndDirInfo(
			[]BlockInfo{{DecompressedSize: 1, CompressedSize: 1, Flags: 0x40}}, nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		info = append(info, 0)
		image := makeTestBundleWithInfo(t, info, uint32(len(info)), uint32(FlagHasDirectoryInfo), []byte{1})
		bundle, err := ReadBundle(bytes.NewReader(image.data))
		if err != nil {
			t.Fatalf("ReadBundle: %v", err)
		}
		if !bytes.Equal(bundle.BlockInfo.TrailingData, []byte{0}) {
			t.Fatalf("TrailingData = % x, want 00", bundle.BlockInfo.TrailingData)
		}
	})

	t.Run("unaccounted compressed tail", func(t *testing.T) {
		image := makeTestBundle(t,
			[]BlockInfo{{DecompressedSize: 1, CompressedSize: 1, Flags: 0x40}}, nil,
			[]byte{1, 2},
		)
		assertReadBundleErrorNoPanic(t, image.data)
	})
}

func TestBundleCorruptLZ4DataReturnsErrorWithoutPanic(t *testing.T) {
	image := makeTestBundle(t,
		[]BlockInfo{{DecompressedSize: 8, CompressedSize: 4, Flags: 0x40 | CompressionLZ4}},
		[]DirectoryInfo{{Offset: 0, DecompressedSize: 8, Name: "bad"}},
		[]byte{0xff, 0xff, 0xff, 0xff},
	)
	bundle, err := ReadBundle(bytes.NewReader(image.data))
	if err != nil {
		t.Fatalf("ReadBundle metadata: %v", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("GetFileData panicked: %v", recovered)
		}
	}()
	if _, err := bundle.GetFileData(0); err == nil {
		t.Fatal("corrupt LZ4 data unexpectedly decoded")
	}
}

func TestBundleReadsLZMACompressedMetadataAndDataBlocks(t *testing.T) {
	rawData := []byte("UnityFS LZMA payload")
	compressedData := compressUnityLZMAForTest(t, rawData)
	blocks := []BlockInfo{{
		DecompressedSize: uint32(len(rawData)),
		CompressedSize:   uint32(len(compressedData)),
		Flags:            0x40 | CompressionLZMA,
	}}
	dirs := []DirectoryInfo{{Offset: 0, DecompressedSize: int64(len(rawData)), Name: "lzma.bin"}}
	info, err := serializeBlockAndDirInfo(blocks, dirs)
	if err != nil {
		t.Fatalf("serializeBlockAndDirInfo: %v", err)
	}
	compressedInfo := compressUnityLZMAForTest(t, info)
	image := makeTestBundleWithInfo(t, compressedInfo, uint32(len(info)), uint32(FlagHasDirectoryInfo|CompressionLZMA), compressedData)

	bundle, err := ReadBundle(bytes.NewReader(image.data))
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}
	got, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	if !bytes.Equal(got, rawData) {
		t.Fatalf("LZMA data = %q, want %q", got, rawData)
	}
}

func compressUnityLZMAForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	var standard bytes.Buffer
	w, err := lzma.NewWriter(&standard)
	if err != nil {
		t.Fatalf("lzma.NewWriter: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("LZMA write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("LZMA close: %v", err)
	}
	encoded := standard.Bytes()
	if len(encoded) < 13 {
		t.Fatalf("standard LZMA stream is too short: %d", len(encoded))
	}
	unity := make([]byte, 0, len(encoded)-8)
	unity = append(unity, encoded[:5]...)
	unity = append(unity, encoded[13:]...)
	return unity
}

func TestBundleLargeLogicalEntryRequiresRangeReads(t *testing.T) {
	blocks := []BlockInfo{
		{DecompressedSize: maxBundleBlockSize, CompressedSize: maxBundleBlockSize, Flags: 0x40},
		{DecompressedSize: maxBundleBlockSize, CompressedSize: maxBundleBlockSize, Flags: 0x40},
		{DecompressedSize: 1, CompressedSize: 1, Flags: 0x40},
	}
	bundle := &Bundle{
		BlockInfo: BlockAndDirInfo{
			BlockInfos: blocks,
			DirectoryInfos: []DirectoryInfo{{
				Name:             "large.resS",
				DecompressedSize: maxBundleReadSize + 1,
			}},
		},
	}
	if _, err := bundle.GetFileData(0); err == nil || !strings.Contains(err.Error(), "in-memory read limit") {
		t.Fatalf("GetFileData should reject a huge allocation, got %v", err)
	}
	if _, err := bundle.GetFileDataRangeByName("large.resS", 0, maxBundleReadSize+1); err == nil {
		t.Fatal("oversized range read unexpectedly succeeded")
	}
}

func TestWriteBundleValidationAndBoundedBlocks(t *testing.T) {
	if err := WriteBundle(nil, []BundleFileEntry{{Name: "x"}}, nil); err == nil {
		t.Fatal("nil writer unexpectedly accepted")
	}
	if err := WriteBundle(io.Discard, []BundleFileEntry{{Name: "x"}}, &BundleWriteOptions{Version: 9}); err == nil {
		t.Fatal("unsupported version unexpectedly accepted")
	}
	if err := WriteBundle(io.Discard, []BundleFileEntry{{Name: "a\x00b"}}, nil); err == nil {
		t.Fatal("NUL in entry name unexpectedly accepted")
	}
	opts := &BundleWriteOptions{}
	data := bytes.Repeat([]byte("0123456789abcdef"), 20000)
	var output bytes.Buffer
	if err := WriteBundle(&output, []BundleFileEntry{{Name: "large", Data: data}}, opts); err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}
	if opts.Version != 0 || opts.EngineVersion != "" || opts.GenerationVersion != "" {
		t.Fatalf("WriteBundle mutated caller options: %+v", opts)
	}
	bundle, err := ReadBundle(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}
	if len(bundle.BlockInfo.BlockInfos) < 2 {
		t.Fatalf("large uncompressed output used %d block(s), want multiple bounded blocks", len(bundle.BlockInfo.BlockInfos))
	}
	got, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("round-trip data mismatch")
	}
}

func TestBundleLongHeaderAndEntryNamesRoundTrip(t *testing.T) {
	longGeneration := strings.Repeat("g", 5000)
	longEngine := strings.Repeat("e", 5000)
	longName := strings.Repeat("n", 5000)
	data := []byte("payload")

	var output bytes.Buffer
	err := WriteBundle(&output, []BundleFileEntry{{Name: longName, Data: data}}, &BundleWriteOptions{
		GenerationVersion: longGeneration,
		EngineVersion:     longEngine,
	})
	if err != nil {
		t.Fatalf("WriteBundle with long strings: %v", err)
	}
	bundle, err := ReadBundle(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatalf("ReadBundle with long strings: %v", err)
	}
	if bundle.Header.GenerationVersion != longGeneration || bundle.Header.EngineVersion != longEngine {
		t.Fatal("long bundle header strings did not round-trip")
	}
	if len(bundle.BlockInfo.DirectoryInfos) != 1 || bundle.BlockInfo.DirectoryInfos[0].Name != longName {
		t.Fatal("long directory name did not round-trip")
	}
	got, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("entry data = %q, want %q", got, data)
	}
}

func TestBundleEmptyAndDuplicateEntryNamesRoundTripByIndex(t *testing.T) {
	entries := []BundleFileEntry{
		{Name: "", Data: []byte("empty")},
		{Name: "same", Data: []byte("first")},
		{Name: "same", Data: []byte("second")},
	}
	var output bytes.Buffer
	if err := WriteBundle(&output, entries, nil); err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}
	bundle, err := ReadBundle(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}
	if len(bundle.BlockInfo.DirectoryInfos) != len(entries) {
		t.Fatalf("directory count = %d, want %d", len(bundle.BlockInfo.DirectoryInfos), len(entries))
	}
	for i, entry := range entries {
		if bundle.BlockInfo.DirectoryInfos[i].Name != entry.Name {
			t.Fatalf("directory[%d] name = %q, want %q", i, bundle.BlockInfo.DirectoryInfos[i].Name, entry.Name)
		}
		data, err := bundle.GetFileData(i)
		if err != nil {
			t.Fatalf("GetFileData(%d): %v", i, err)
		}
		if !bytes.Equal(data, entry.Data) {
			t.Fatalf("GetFileData(%d) = %q, want %q", i, data, entry.Data)
		}
	}
	rangeData, err := bundle.GetFileDataRange(2, 1, 4)
	if err != nil {
		t.Fatalf("GetFileDataRange for second duplicate: %v", err)
	}
	if !bytes.Equal(rangeData, []byte("econ")) {
		t.Fatalf("second duplicate range = %q, want %q", rangeData, "econ")
	}
}

func TestBundleEmptyRoundTrip(t *testing.T) {
	for _, version := range []uint32{6, 7, 8} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			var output bytes.Buffer
			if err := WriteBundle(&output, nil, &BundleWriteOptions{Version: version}); err != nil {
				t.Fatalf("WriteBundle: %v", err)
			}
			bundle, err := ReadBundle(bytes.NewReader(output.Bytes()))
			if err != nil {
				t.Fatalf("ReadBundle: %v", err)
			}
			if len(bundle.BlockInfo.DirectoryInfos) != 0 {
				t.Fatalf("empty bundle gained directory entries: %+v", bundle.BlockInfo)
			}
			for index, block := range bundle.BlockInfo.BlockInfos {
				if block.DecompressedSize != 0 || block.CompressedSize != 0 {
					t.Fatalf("empty bundle block[%d] carries data: %+v", index, block)
				}
			}
			if bundle.Header.Version != version || bundle.Header.FSHeader.TotalFileSize != int64(output.Len()) {
				t.Fatalf("empty bundle header = %+v, size = %d", bundle.Header, output.Len())
			}
		})
	}
}

func TestBundleZeroBlockAndDirectoryCountsAreReadable(t *testing.T) {
	image := makeTestBundle(t, nil, nil, nil)
	bundle, err := ReadBundle(bytes.NewReader(image.data))
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}
	if len(bundle.BlockInfo.BlockInfos) != 0 || len(bundle.BlockInfo.DirectoryInfos) != 0 {
		t.Fatalf("zero metadata counts changed: %+v", bundle.BlockInfo)
	}
}

type zeroProgressWriter struct{}

func (zeroProgressWriter) Write([]byte) (int, error) { return 0, nil }

func TestWriteBundleRejectsZeroProgressWriter(t *testing.T) {
	err := WriteBundle(zeroProgressWriter{}, []BundleFileEntry{{Name: "x", Data: []byte("x")}}, nil)
	if err == nil || !strings.Contains(err.Error(), io.ErrShortWrite.Error()) {
		t.Fatalf("expected io.ErrShortWrite, got %v", err)
	}
}

func TestBundleSampleMetadataBounds(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/aba/*.aba")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no .aba test files found")
	}

	var versions = make(map[uint32]int)
	var maxBlocks, maxDirs int
	var maxEntry int64
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		bundle, err := ReadBundle(f)
		if err != nil {
			f.Close()
			if isEncryptedError(err) {
				continue
			}
			t.Fatalf("ReadBundle(%s): %v", filepath.Base(path), err)
		}
		versions[bundle.Header.Version]++
		if len(bundle.BlockInfo.BlockInfos) > maxBlocks {
			maxBlocks = len(bundle.BlockInfo.BlockInfos)
		}
		if len(bundle.BlockInfo.DirectoryInfos) > maxDirs {
			maxDirs = len(bundle.BlockInfo.DirectoryInfos)
		}
		for _, dir := range bundle.BlockInfo.DirectoryInfos {
			if dir.DecompressedSize > maxEntry {
				maxEntry = dir.DecompressedSize
			}
			if dir.DecompressedSize == 0 {
				continue
			}
			probeSize := minInt64(dir.DecompressedSize, 16)
			if _, err := bundle.GetFileDataRangeByName(dir.Name, 0, probeSize); err != nil {
				t.Fatalf("read first bytes of %s/%s: %v", filepath.Base(path), dir.Name, err)
			}
			if _, err := bundle.GetFileDataRangeByName(dir.Name, dir.DecompressedSize-probeSize, probeSize); err != nil {
				t.Fatalf("read last bytes of %s/%s: %v", filepath.Base(path), dir.Name, err)
			}
		}
		f.Close()
	}
	t.Logf("versions=%v maxBlocks=%d maxDirs=%d maxEntry=%d", versions, maxBlocks, maxDirs, maxEntry)
}
