package ct

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierrec/lz4/v4"
)

func TestCompressLz4BlockArray_MessagePackCSharpWireFormat(t *testing.T) {
	data := bytes.Repeat([]byte("MessagePack-CSharp Lz4BlockArray compatibility\n"), 2000)
	encoded, err := CompressLz4BlockArray(data)
	if err != nil {
		t.Fatalf("CompressLz4BlockArray: %v", err)
	}

	pos := int64(0)
	arrayLen, err := readArrayHeader(encoded, &pos)
	if err != nil {
		t.Fatalf("read array header: %v", err)
	}
	wantBlocks := (int64(len(data)) + int64(blockSize) - 1) / int64(blockSize)
	if arrayLen != wantBlocks+1 {
		t.Fatalf("array length got %d, want %d", arrayLen, wantBlocks+1)
	}

	extType, sizePayload, err := readExtPayload(encoded, &pos)
	if err != nil {
		t.Fatalf("read size extension: %v", err)
	}
	if extType != 98 {
		t.Fatalf("size extension type got %d, want MessagePack-CSharp Lz4BlockArray type 98", extType)
	}
	sizes, err := decodeMsgpackIntList(sizePayload)
	if err != nil {
		t.Fatalf("decode size list: %v", err)
	}
	if int64(len(sizes)) != wantBlocks {
		t.Fatalf("size count got %d, want %d", len(sizes), wantBlocks)
	}

	inputOffset := int64(0)
	for i, size := range sizes {
		// MessagePackSerializer.WriteBin32Header always emits bin32 (0xc6),
		// even when a compressed block would fit in bin8/bin16.
		if pos >= int64(len(encoded)) {
			t.Fatalf("block[%d] marker at %d is past end %d", i, pos, len(encoded))
		}
		if encoded[pos] != 0xc6 {
			t.Fatalf("block[%d] marker at %d got 0x%02x, want bin32 0xc6", i, pos, encoded[pos])
		}
		compressed, err := ReadBin(encoded, &pos)
		if err != nil {
			t.Fatalf("read block[%d]: %v", i, err)
		}
		block := make([]byte, size)
		n, err := lz4.UncompressBlock(compressed, block)
		if err != nil {
			t.Fatalf("independent LZ4 decode block[%d]: %v", i, err)
		}
		if int64(n) != size {
			t.Fatalf("independent LZ4 decode block[%d] size got %d, want %d", i, n, size)
		}
		if !bytes.Equal(block, data[inputOffset:inputOffset+size]) {
			t.Fatalf("independent LZ4 decode block[%d] differs", i)
		}
		inputOffset += size
	}
	if pos != int64(len(encoded)) {
		t.Fatalf("wire parser left %d unread bytes", int64(len(encoded))-pos)
	}

	decoded, err := DecompressLz4BlockArray(encoded)
	if err != nil {
		t.Fatalf("DecompressLz4BlockArray: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatalf("round trip differs: got %d bytes, want %d", len(decoded), len(data))
	}
}

func TestDecodeMsgpackBoundsHostileCollectionsAndDepth(t *testing.T) {
	t.Run("huge declared array", func(t *testing.T) {
		// array32(2^31-1) followed by one element and EOF. The decoder must
		// report truncation without attempting the declared-size allocation.
		data := []byte{0xdd, 0x7f, 0xff, 0xff, 0xff, 0xc0}
		var out []interface{}
		if err := DecodeMsgpack(data, &out); err == nil {
			t.Fatal("hostile array32 unexpectedly decoded")
		}
	})

	t.Run("excessive nesting", func(t *testing.T) {
		data := bytes.Repeat([]byte{0x91}, 257)
		data = append(data, 0xc0)
		var out interface{}
		if err := DecodeMsgpack(data, &out); err == nil {
			t.Fatal("257-level MessagePack nesting unexpectedly decoded")
		}
	})

	t.Run("invalid UTF-8", func(t *testing.T) {
		var out string
		if err := DecodeMsgpack([]byte{0xa1, 0xff}, &out); err == nil {
			t.Fatal("invalid UTF-8 string unexpectedly decoded")
		}
	})
}

func TestDecodeMsgpackRejectsTopLevelTrailingBytes(t *testing.T) {
	data := []byte{0x92, 0x01, 0x02, 0xde, 0xad, 0xbe, 0xef}
	var out []int32
	if err := DecodeMsgpack(data, &out); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("DecodeMsgpack trailing-data error = %v", err)
	}
}

func TestDecompressLz4BlockArray_GameMessagePackCSharpSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "kces_parts", "cm3d2_megane002.materialassets")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read game sample: %v", err)
	}
	if len(data) < 4 || data[0] != 0x92 || data[3] != 98 {
		t.Fatalf("sample does not have expected array + ext(98) prefix: % x", data[:min(len(data), 8)])
	}
	decoded, err := DecompressLz4BlockArray(data)
	if err != nil {
		t.Fatalf("decompress game MessagePack-CSharp sample: %v", err)
	}
	var value interface{}
	if err := DecodeMsgpack(decoded, &value); err != nil {
		t.Fatalf("decode decompressed game MessagePack: %v", err)
	}
}

func TestCompressLz4BlockArray_IncompressibleBlock(t *testing.T) {
	rng := rand.New(rand.NewSource(0x4b434553))
	data := make([]byte, blockSize+257)
	if _, err := rng.Read(data); err != nil {
		t.Fatalf("generate deterministic data: %v", err)
	}

	probe := make([]byte, lz4.CompressBlockBound(blockSize))
	n, err := lz4.CompressBlock(data[:blockSize], probe, nil)
	if err != nil {
		t.Fatalf("probe CompressBlock: %v", err)
	}
	if n > 0 && n < blockSize {
		t.Fatalf("test precondition failed: first block compressed to %d bytes, want an incompressible result", n)
	}
	// Some pierrec/lz4 versions report incompressibility as n == 0, while the
	// current version emits a valid literal-only block larger than the input.
	// Exercise the n == 0 production fallback explicitly as well.
	fallback := finishLz4Block(data[:blockSize], nil, 0)
	fallbackDecoded := make([]byte, blockSize)
	fallbackN, err := lz4.UncompressBlock(fallback, fallbackDecoded)
	if err != nil || fallbackN != blockSize || !bytes.Equal(fallbackDecoded, data[:blockSize]) {
		t.Fatalf("n == 0 literal fallback failed: n=%d err=%v", fallbackN, err)
	}

	encoded, err := CompressLz4BlockArray(data)
	if err != nil {
		t.Fatalf("CompressLz4BlockArray: %v", err)
	}
	decoded, err := DecompressLz4BlockArray(encoded)
	if err != nil {
		t.Fatalf("DecompressLz4BlockArray: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatalf("incompressible round trip differs")
	}

	pos := int64(0)
	if _, err := readArrayHeader(encoded, &pos); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readExtPayload(encoded, &pos); err != nil {
		t.Fatal(err)
	}
	firstBlock, err := ReadBin(encoded, &pos)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstBlock) <= blockSize {
		t.Fatalf("literal-only LZ4 block got %d bytes, expected overhead beyond %d raw bytes", len(firstBlock), blockSize)
	}
}

func TestDecompressLz4Block(t *testing.T) {
	data := bytes.Repeat([]byte("single-Lz4Block"), 50)
	compressed, err := compressLz4Block(data)
	if err != nil {
		t.Fatal(err)
	}
	payload := appendMsgpackUint(nil, uint32(len(data)))
	payload = append(payload, compressed...)
	encoded := WriteExt(nil, Lz4BlockType, payload)

	decoded, err := DecompressLz4BlockArray(encoded)
	if err != nil {
		t.Fatalf("decompress Lz4Block: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatalf("single Lz4Block decoded data differs")
	}
}

func TestDecompressLz4BlockArray_TruncationNeverPanics(t *testing.T) {
	data := bytes.Repeat([]byte("truncate-me"), 30)
	standard, err := CompressLz4BlockArray(data)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := compressLz4Block(data)
	if err != nil {
		t.Fatal(err)
	}
	payload := appendMsgpackUint(nil, uint32(len(data)))
	single := WriteExt(nil, Lz4BlockType, append(payload, compressed...))

	for name, encoded := range map[string][]byte{
		"standard": standard,
		"single":   single,
	} {
		t.Run(name, func(t *testing.T) {
			for cut := 0; cut < len(encoded); cut++ {
				assertDecompressDoesNotPanic(t, encoded[:cut], cut)
			}
		})
	}
}

func TestDecompressLz4BlockArray_RejectsOversizedAllocations(t *testing.T) {
	hugeSize := uint32(maxLz4DecompressedSize + 1)
	hugeSizePayload := appendMsgpackUint(nil, hugeSize)
	standard := WriteArrayHeader(nil, 2)
	standard = WriteExt(standard, Lz4ArrayType, hugeSizePayload)
	standard = writeBin32(standard, nil)

	singlePayload := appendMsgpackUint(nil, hugeSize)
	single := WriteExt(nil, Lz4BlockType, singlePayload)

	tooManyBlocks := []byte{0xdd, 0x00, 0x10, 0x00, 0x02, 0xd4, byte(Lz4ArrayType), 0x00}

	for name, encoded := range map[string][]byte{
		"standard-size": standard,
		"single-size":   single,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := DecompressLz4BlockArray(encoded)
			if err == nil {
				t.Fatal("expected allocation-limit error")
			}
			if !strings.Contains(err.Error(), "exceeds limit") {
				t.Fatalf("error %q does not report limit", err)
			}
		})
	}

	// The array32 count is not itself an allocation request. Its one-entry size
	// table proves the payload is structurally incomplete before any block list
	// or decompressed output is allocated.
	if _, err := DecompressLz4BlockArray(tooManyBlocks); err == nil || !strings.Contains(err.Error(), "arrayLen") {
		t.Fatalf("large truncated block array error = %v", err)
	}
}

func TestDecompressLz4BlockArray_RejectsTrailingBytes(t *testing.T) {
	encoded, err := CompressLz4BlockArray(bytes.Repeat([]byte("tail"), 100))
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, 0xde, 0xad, 0xbe, 0xef)
	if _, err := DecompressLz4BlockArray(encoded); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("got error %v, want trailing-byte rejection", err)
	}
}

func TestDecompressLz4BlockArray_OrdinaryArrayPassesThrough(t *testing.T) {
	data := []byte{0x92, 0x01, 0xa3, 'r', 'a', 'w'}
	decoded, err := DecompressLz4BlockArray(data)
	if err != nil {
		t.Fatalf("ordinary array: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatalf("ordinary MessagePack array changed")
	}
}

func assertDecompressDoesNotPanic(t *testing.T, data []byte, cut int) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("panic at prefix length %d: %v", cut, recovered)
		}
	}()
	_, _ = DecompressLz4BlockArray(data)
}
