package aba

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAba(t *testing.T) {
	files := smallAbaTestFiles(t)

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			defer f.Close()

			abaFile, err := ReadAba(f)
			if err != nil {
				if isEncryptedError(err) {
					t.Skipf("skipping encrypted file: %v", err)
				}
				t.Fatalf("ReadAba failed: %v", err)
			}

			t.Logf("Signature: %s", abaFile.Header.Signature)
			t.Logf("Version: %d", abaFile.Header.Version)
			t.Logf("Engine: %s", abaFile.Header.EngineVersion)
			t.Logf("TotalFileSize: %d", abaFile.Header.FSHeader.TotalFileSize)
			t.Logf("Blocks: %d, Files: %d", len(abaFile.BlockInfo.BlockInfos), len(abaFile.BlockInfo.DirectoryInfos))

			for i, d := range abaFile.BlockInfo.DirectoryInfos {
				t.Logf("  [%d] %q offset=%d size=%d serialized=%v",
					i, d.Name, d.Offset, d.DecompressedSize, d.IsSerialized())
			}

			// 尝试读取每个文件的数据
			for i, d := range abaFile.BlockInfo.DirectoryInfos {
				if d.DecompressedSize > maxAbaReadSize {
					const probeSize int64 = 16
					for _, relativeOffset := range []int64{0, d.DecompressedSize - probeSize} {
						data, err := abaFile.GetFileDataRange(i, relativeOffset, probeSize)
						if err != nil {
							t.Errorf("read large file probe (%d, %q, offset=%d) failed: %v", i, d.Name, relativeOffset, err)
						} else if len(data) != int(probeSize) {
							t.Errorf("read large file probe (%d, %q, offset=%d): got %d bytes", i, d.Name, relativeOffset, len(data))
						}
					}
					continue
				}
				data, err := abaFile.GetFileData(i)
				if err != nil {
					t.Errorf("GetFileData(%d, %q) failed: %v", i, d.Name, err)
					continue
				}
				if int64(len(data)) != d.DecompressedSize {
					t.Errorf("GetFileData(%d, %q): got %d bytes, want %d",
						i, d.Name, len(data), d.DecompressedSize)
				}
			}
		})
	}
}

func TestReadAba_AllFiles(t *testing.T) {
	files := smallAbaTestFiles(t)

	success := 0
	skipped := 0
	for _, filePath := range files {
		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		abaFile, err := ReadAba(f)
		f.Close()
		if err == nil && len(abaFile.BlockInfo.DirectoryInfos) > 0 {
			success++
		} else if err != nil {
			if isEncryptedError(err) {
				skipped++
			} else {
				fmt.Printf("  FAIL %s: %v\n", filepath.Base(filePath), err)
			}
		}
	}

	fmt.Printf("Successfully parsed %d/%d .aba files (skipped %d encrypted)\n", success, len(files), skipped)
	if success == 0 && len(files) > 0 {
		t.Error("failed to parse any .aba files")
	}
	expected := len(files) - skipped
	if success < expected {
		t.Errorf("only %d/%d non-encrypted files parsed successfully", success, expected)
	}
}

func isEncryptedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "encrypted .aba file") || strings.Contains(msg, ".aba file is encrypted")
}
