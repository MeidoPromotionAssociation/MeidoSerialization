package aba

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAba_RoundTrip(t *testing.T) {
	files := smallAbaTestFiles(t)

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}

			originalAba, err := ReadAba(f)
			if err != nil {
				f.Close()
				if isEncryptedError(err) {
					t.Skipf("skipping encrypted file: %v", err)
				}
				t.Fatalf("ReadAba failed: %v", err)
			}
			for _, dir := range originalAba.BlockInfo.DirectoryInfos {
				if dir.DecompressedSize > maxAbaReadSize {
					f.Close()
					t.Skipf(".aba contains %q (%d bytes), which cannot be represented by the in-memory AbaFileEntry API", dir.Name, dir.DecompressedSize)
				}
			}

			// 提取所有文件数据
			entries := make([]AbaFileEntry, len(originalAba.BlockInfo.DirectoryInfos))
			for i, dir := range originalAba.BlockInfo.DirectoryInfos {
				data, err := originalAba.GetFileData(int64(i))
				if err != nil {
					f.Close()
					t.Fatalf("GetFileData(%d) failed: %v", i, err)
				}
				entries[i] = AbaFileEntry{
					Name:         dir.Name,
					Data:         data,
					IsSerialized: dir.IsSerialized(),
				}
			}
			f.Close()

			// 写入新的 .aba 文件
			var buf bytes.Buffer
			opts := &AbaWriteOptions{
				EngineVersion:     originalAba.Header.EngineVersion,
				GenerationVersion: originalAba.Header.GenerationVersion,
				Version:           originalAba.Header.Version,
				Compress:          true,
			}
			if err := WriteAba(&buf, entries, opts); err != nil {
				t.Fatalf("WriteAba failed: %v", err)
			}

			// 重新读取并验证
			rewrittenData := buf.Bytes()
			rewrittenAba, err := ReadAba(bytes.NewReader(rewrittenData))
			if err != nil {
				t.Fatalf("re-read failed: %v", err)
			}

			if rewrittenAba.Header.Signature != originalAba.Header.Signature {
				t.Errorf("signature mismatch: got %q, want %q",
					rewrittenAba.Header.Signature, originalAba.Header.Signature)
			}
			if len(rewrittenAba.BlockInfo.DirectoryInfos) != len(originalAba.BlockInfo.DirectoryInfos) {
				t.Errorf("directory count mismatch: got %d, want %d",
					len(rewrittenAba.BlockInfo.DirectoryInfos), len(originalAba.BlockInfo.DirectoryInfos))
			}

			// 验证每个文件数据一致
			for i, entry := range entries {
				newData, err := rewrittenAba.GetFileData(int64(i))
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

func TestWriteAbaReaderAtEntryRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("streamed ABA entry"), 20000)
	var out bytes.Buffer
	if err := WriteAba(&out, []AbaFileEntry{{
		Name:         "CAB-streamed",
		ReaderAt:     bytes.NewReader(data),
		Size:         int64(len(data)),
		IsSerialized: true,
	}}, &AbaWriteOptions{Compress: true}); err != nil {
		t.Fatalf("WriteAba: %v", err)
	}
	bundle, err := ReadAba(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("ReadAba: %v", err)
	}
	got, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("streamed entry differs: got %d bytes, want %d", len(got), len(data))
	}
}

func TestWriteAbaGeneratedEntryRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("generated ABA entry"), 20000)
	var calls int64
	var out bytes.Buffer
	if err := WriteAba(&out, []AbaFileEntry{{
		Name: "CAB-generated-stream",
		WriteTo: func(destination io.Writer) error {
			calls++
			_, err := io.Copy(destination, bytes.NewReader(data))
			return err
		},
		Size:         int64(len(data)),
		IsSerialized: true,
	}}, &AbaWriteOptions{Compress: true}); err != nil {
		t.Fatalf("WriteAba: %v", err)
	}
	if calls != 2 {
		t.Fatalf("generator calls = %d, want 2", calls)
	}
	bundle, err := ReadAba(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("ReadAba: %v", err)
	}
	got, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("generated entry differs: got %d bytes, want %d", len(got), len(data))
	}
}

func TestWriteAbaUncompressedGeneratedEntryUsesSinglePass(t *testing.T) {
	data := bytes.Repeat([]byte("single-pass ABA entry"), 20000)
	var calls int64
	var out bytes.Buffer
	if err := WriteAba(&out, []AbaFileEntry{{
		Name: "CAB-single-pass-stream",
		WriteTo: func(destination io.Writer) error {
			calls++
			_, err := io.Copy(destination, bytes.NewReader(data))
			return err
		},
		Size:         int64(len(data)),
		IsSerialized: true,
	}}, &AbaWriteOptions{Compress: false}); err != nil {
		t.Fatalf("WriteAba: %v", err)
	}
	if calls != 1 {
		t.Fatalf("generator calls = %d, want 1", calls)
	}
	bundle, err := ReadAba(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("ReadAba: %v", err)
	}
	got, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("generated entry differs: got %d bytes, want %d", len(got), len(data))
	}
}

func TestWriteAbaRejectsIncorrectGeneratedEntrySize(t *testing.T) {
	err := WriteAba(io.Discard, []AbaFileEntry{{
		Name: "short",
		WriteTo: func(destination io.Writer) error {
			_, err := destination.Write([]byte("abc"))
			return err
		},
		Size: 4,
	}}, nil)
	if err == nil {
		t.Fatal("WriteAba accepted a short generated entry")
	}
}
