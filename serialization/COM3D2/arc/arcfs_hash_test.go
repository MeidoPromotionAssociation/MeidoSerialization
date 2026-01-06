package arc

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func TestHashConsistency(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/*.arc")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		// 1. 读取 ARC 并提取 Name Table
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatal(err)
		}

		if len(data) < 28 {
			t.Logf("Skipping %s: file too small (%d bytes)", filePath, len(data))
			continue
		}

		// 检查 Header
		if !bytes.Equal(data[:len(arcHeader)], arcHeader) {
			if bytes.HasPrefix(data, encArcHeader) {
				t.Logf("Skipping %s: encrypted ARC (warp)", filePath)
			} else {
				t.Logf("Skipping %s: invalid ARC header", filePath)
			}
			continue
		}

		t.Logf("ARC size: %d", len(data))

		// 找到 metadata 偏移
		headerLen := 20
		metadataPosition := int64(binary.LittleEndian.Uint64(data[headerLen : headerLen+8]))
		baseOffset := int64(headerLen + 8)
		metadataOffset := baseOffset + metadataPosition
		t.Logf("metadataPosition: %d, metadataOffset: %d", metadataPosition, metadataOffset)

		// 简单的块遍历来找到 Name Table (Type 3)
		curr := metadataOffset
		var utf16NameData []byte
		for curr < int64(len(data)) {
			if curr+12 > int64(len(data)) {
				break
			}
			blockType := int32(binary.LittleEndian.Uint32(data[curr : curr+4]))
			blockSize := int64(binary.LittleEndian.Uint64(data[curr+4 : curr+12]))
			if blockType == 3 {
				// File block
				flag := binary.LittleEndian.Uint32(data[curr+12 : curr+16])
				encSize := binary.LittleEndian.Uint32(data[curr+24 : curr+28])
				payload := data[curr+28 : curr+28+int64(encSize)]
				if flag == 1 {
					dec, err := deflateDecompress(payload)
					if err != nil {
						t.Fatal(err)
					}
					utf16NameData = dec
				} else {
					utf16NameData = payload
				}
				break
			}
			curr += 12 + blockSize
		}

		if utf16NameData == nil {
			t.Fatal("Could not find Name Table in ARC")
		}

		// 解析 Name Table
		lut, err := readNameTable(stream.NewBinaryReader(bytes.NewReader(utf16NameData)))
		if err != nil {
			t.Fatal(err)
		}

		// 验证哈希
		count := 0
		for h, name := range lut {
			// 尝试 UTF16 哈希 (ARC 里面通常用 UTF16 哈希查找名称)
			// 我们尝试对名称进行各种形式的处理，看哪一个能匹配上

			// 1. 直接计算 (应该会包含 lower 逻辑)
			calc := NameHashUTF16(name)
			if calc == h {
				count++
				t.Logf("Hash match for name %q: %x", name, h)
				continue
			}

			// 2. 如果不匹配，尝试强制 lower (虽然 NameHashUTF16 内部应该已经做了)
			calc2 := NameHashUTF16(strings.ToLower(name))
			if calc2 == h {
				count++
				t.Logf("Hash match for name %q (lower): %x", name, h)
				continue
			}

			t.Errorf("Hash mismatch for name %q: expected %x, got %x (direct), %x (lower)", name, h, calc, calc2)
		}

		if count == 0 && len(lut) > 0 {
			t.Errorf("All hashes mismatched! Total items: %d", len(lut))
		} else {
			t.Logf("Successfully verified %d/%d hashes", count, len(lut))
		}
	}
}

func TestManualHash(t *testing.T) {
	h := NameHashUTF16("a")
	if h != 0x89be207bddf13e3 {
		t.Errorf("Hash mismatch for 'a': expected %x, got %x", 0x89be207bddf13e3, h)
	}
	t.Logf("Hash of 'a': %x", h)
}
