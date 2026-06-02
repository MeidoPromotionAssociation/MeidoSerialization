package aba

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteBundle_RoundTrip(t *testing.T) {
	files := smallAbaTestFiles(t)

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}

			origBundle, err := ReadBundle(f)
			if err != nil {
				f.Close()
				if isEncryptedError(err) {
					t.Skipf("skipping encrypted file: %v", err)
				}
				t.Fatalf("ReadBundle failed: %v", err)
			}

			// 提取所有文件数据
			entries := make([]BundleFileEntry, len(origBundle.BlockInfo.DirectoryInfos))
			for i, dir := range origBundle.BlockInfo.DirectoryInfos {
				data, err := origBundle.GetFileData(i)
				if err != nil {
					f.Close()
					t.Fatalf("GetFileData(%d) failed: %v", i, err)
				}
				entries[i] = BundleFileEntry{
					Name:         dir.Name,
					Data:         data,
					IsSerialized: dir.IsSerialized(),
				}
			}
			f.Close()

			// 写入新 bundle
			var buf bytes.Buffer
			opts := &BundleWriteOptions{
				EngineVersion:     origBundle.Header.EngineVersion,
				GenerationVersion: origBundle.Header.GenerationVersion,
				Version:           origBundle.Header.Version,
				Compress:          true,
			}
			if err := WriteBundle(&buf, entries, opts); err != nil {
				t.Fatalf("WriteBundle failed: %v", err)
			}

			// 重新读取并验证
			rewrittenData := buf.Bytes()
			newBundle, err := ReadBundle(bytes.NewReader(rewrittenData))
			if err != nil {
				t.Fatalf("re-read failed: %v", err)
			}

			if newBundle.Header.Signature != origBundle.Header.Signature {
				t.Errorf("signature mismatch: got %q, want %q",
					newBundle.Header.Signature, origBundle.Header.Signature)
			}
			if len(newBundle.BlockInfo.DirectoryInfos) != len(origBundle.BlockInfo.DirectoryInfos) {
				t.Errorf("directory count mismatch: got %d, want %d",
					len(newBundle.BlockInfo.DirectoryInfos), len(origBundle.BlockInfo.DirectoryInfos))
			}

			// 验证每个文件数据一致
			for i, entry := range entries {
				newData, err := newBundle.GetFileData(i)
				if err != nil {
					t.Errorf("rewritten GetFileData(%d) failed: %v", i, err)
					continue
				}
				if !bytes.Equal(entry.Data, newData) {
					t.Errorf("data mismatch for %q: orig %d bytes, rewritten %d bytes",
						entry.Name, len(entry.Data), len(newData))
				}
			}
		})
	}
}
