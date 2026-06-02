package aba

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBundle(t *testing.T) {
	files := smallAbaTestFiles(t)

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			defer f.Close()

			bundle, err := ReadBundle(f)
			if err != nil {
				if isEncryptedError(err) {
					t.Skipf("skipping encrypted file: %v", err)
				}
				t.Fatalf("ReadBundle failed: %v", err)
			}

			t.Logf("Signature: %s", bundle.Header.Signature)
			t.Logf("Version: %d", bundle.Header.Version)
			t.Logf("Engine: %s", bundle.Header.EngineVersion)
			t.Logf("TotalFileSize: %d", bundle.Header.FSHeader.TotalFileSize)
			t.Logf("Blocks: %d, Files: %d", len(bundle.BlockInfo.BlockInfos), len(bundle.BlockInfo.DirectoryInfos))

			for i, d := range bundle.BlockInfo.DirectoryInfos {
				t.Logf("  [%d] %q offset=%d size=%d serialized=%v",
					i, d.Name, d.Offset, d.DecompressedSize, d.IsSerialized())
			}

			// 尝试读取每个文件的数据
			for i, d := range bundle.BlockInfo.DirectoryInfos {
				data, err := bundle.GetFileData(i)
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

func TestReadBundle_AllFiles(t *testing.T) {
	files := smallAbaTestFiles(t)

	success := 0
	skipped := 0
	for _, filePath := range files {
		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		bundle, err := ReadBundle(f)
		f.Close()
		if err == nil && len(bundle.BlockInfo.DirectoryInfos) > 0 {
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
