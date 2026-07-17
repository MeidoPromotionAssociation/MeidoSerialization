package KCES

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

func TestCtServiceUnpackRejectsTraversalName(t *testing.T) {
	baseDir := t.TempDir()
	outDir := filepath.Join(baseDir, "out")
	escapePath := filepath.Join(baseDir, "escaped.txt")
	ctPath := filepath.Join(baseDir, "malicious.ct")
	writeMaliciousCT(t, ctPath, "../escaped.txt", []byte("escaped"))

	err := (&CtService{}).UnpackCt(ctPath, outDir)
	if err == nil {
		t.Fatal("UnpackCt accepted a parent-directory traversal name")
	}
	if _, statErr := os.Stat(escapePath); !os.IsNotExist(statErr) {
		t.Fatalf("UnpackCt wrote outside output root: stat err=%v", statErr)
	}
}

func TestAbaServiceUnpackRejectsTraversalName(t *testing.T) {
	baseDir := t.TempDir()
	outDir := filepath.Join(baseDir, "out")
	escapePath := filepath.Join(baseDir, "escaped.bin")
	abaPath := filepath.Join(baseDir, "malicious.aba")

	var bundle bytes.Buffer
	err := aba.WriteBundle(&bundle, []aba.BundleFileEntry{{
		Name: "../escaped.bin",
		Data: []byte("escaped"),
	}}, &aba.BundleWriteOptions{Compress: false})
	if err != nil {
		t.Fatalf("create malicious ABA: %v", err)
	}
	if err := os.WriteFile(abaPath, bundle.Bytes(), 0644); err != nil {
		t.Fatalf("write malicious ABA: %v", err)
	}

	err = (&AbaService{}).UnpackAba(abaPath, outDir)
	if err == nil {
		t.Fatal("UnpackAba accepted a parent-directory traversal name")
	}
	if _, statErr := os.Stat(escapePath); !os.IsNotExist(statErr) {
		t.Fatalf("UnpackAba wrote outside output root: stat err=%v", statErr)
	}
}

func TestNormalizeExtractionPathRejectsUnsafeNames(t *testing.T) {
	tests := []string{
		"",
		".",
		"..",
		"../escape",
		`..\escape`,
		"safe/../../escape",
		"/absolute/path",
		`\rooted`,
		`C:\absolute\path`,
		`C:drive-relative`,
		`\\server\share\file`,
		`\\?\C:\extended\path`,
		`//server/share/file`,
		"safe//file",
		"safe/./file",
		"file:stream",
		"safe/NUL.txt",
		"safe/name. ",
		"safe/with\x00nul",
	}
	for _, name := range tests {
		t.Run(strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			if _, err := normalizeExtractionPath(name); err == nil {
				t.Fatalf("normalizeExtractionPath(%q) unexpectedly succeeded", name)
			}
		})
	}
}

func TestNormalizeExtractionPathAcceptsPortableRelativeNames(t *testing.T) {
	tests := []string{
		"catalog",
		"nested/file.bin",
		`nested\windows-style.bin`,
		"日本語/资源.bin",
	}
	for _, name := range tests {
		rel, err := normalizeExtractionPath(name)
		if err != nil {
			t.Fatalf("normalizeExtractionPath(%q): %v", name, err)
		}
		if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
			t.Fatalf("normalized path is not relative: %q", rel)
		}
	}
}

func TestExtractionRootRejectsOutputSymlink(t *testing.T) {
	baseDir := t.TempDir()
	outsideDir := filepath.Join(baseDir, "outside")
	outDir := filepath.Join(baseDir, "out")
	if err := os.MkdirAll(filepath.Join(outDir, "nested"), 0755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	linkPath := filepath.Join(outDir, "nested", "link")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating symlinks requires Windows Developer Mode or privilege: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}

	root, err := openExtractionRoot(outDir)
	if err != nil {
		t.Fatalf("open extraction root: %v", err)
	}
	defer root.Close()
	if err := root.WriteFile("nested/link/escaped.txt", []byte("escaped"), 0644); err == nil {
		t.Fatal("WriteFile followed an output-directory symlink")
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "escaped.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("WriteFile escaped through symlink: stat err=%v", statErr)
	}
}

func TestExtractionRootRejectsLinkedDestination(t *testing.T) {
	baseDir := t.TempDir()
	outDir := filepath.Join(baseDir, "out")
	outsidePath := filepath.Join(baseDir, "outside.txt")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	if err := os.WriteFile(outsidePath, []byte("original"), 0644); err != nil {
		t.Fatalf("create outside file: %v", err)
	}
	linkPath := filepath.Join(outDir, "target.txt")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating symlinks requires Windows Developer Mode or privilege: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}

	root, err := openExtractionRoot(outDir)
	if err != nil {
		t.Fatalf("open extraction root: %v", err)
	}
	defer root.Close()
	if err := root.WriteFile("target.txt", []byte("replacement"), 0644); err == nil {
		t.Fatal("WriteFile followed a symlink destination")
	}
	data, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("outside file was modified through symlink: %q", data)
	}
}

func TestExtractionRootRejectsSymlinkRoot(t *testing.T) {
	baseDir := t.TempDir()
	realDir := filepath.Join(baseDir, "real")
	linkDir := filepath.Join(baseDir, "link")
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatalf("create real directory: %v", err)
	}
	if err := os.Symlink(realDir, linkDir); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating symlinks requires Windows Developer Mode or privilege: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}
	if root, err := openExtractionRoot(linkDir); err == nil {
		root.Close()
		t.Fatal("openExtractionRoot accepted a symlink output root")
	}
}

func TestExtractionRootAtomicallyReplacesRegularFile(t *testing.T) {
	outDir := t.TempDir()
	root, err := openExtractionRoot(outDir)
	if err != nil {
		t.Fatalf("open extraction root: %v", err)
	}
	defer root.Close()
	if err := root.WriteFile("nested/file.bin", []byte("first"), 0644); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}
	if err := root.WriteFile("nested/file.bin", []byte("second"), 0644); err != nil {
		t.Fatalf("replacement WriteFile: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "nested", "file.bin"))
	if err != nil {
		t.Fatalf("read replaced file: %v", err)
	}
	if string(data) != "second" {
		t.Fatalf("replaced file got %q", data)
	}
}

func TestExtractionRootWriteFileStreamIsAtomic(t *testing.T) {
	outDir := t.TempDir()
	root, err := openExtractionRoot(outDir)
	if err != nil {
		t.Fatalf("open extraction root: %v", err)
	}
	defer root.Close()

	if err := root.WriteFile("nested/file.bin", []byte("original"), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}
	wantErr := errors.New("injected streaming failure")
	err = root.WriteFileStream("nested/file.bin", 0644, func(f *os.File) error {
		if _, err := f.Write([]byte("partial")); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteFileStream error = %v, want injected error", err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "nested", "file.bin"))
	if err != nil {
		t.Fatalf("read target after failed stream: %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("failed stream replaced target with %q", data)
	}

	err = root.WriteFileStream("nested/file.bin", 0644, func(f *os.File) error {
		if _, err := f.Write([]byte("streamed ")); err != nil {
			return err
		}
		_, err := f.Write([]byte("replacement"))
		return err
	})
	if err != nil {
		t.Fatalf("successful WriteFileStream: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(outDir, "nested", "file.bin"))
	if err != nil {
		t.Fatalf("read streamed target: %v", err)
	}
	if string(data) != "streamed replacement" {
		t.Fatalf("streamed target got %q", data)
	}
}

func writeMaliciousCT(t *testing.T, path, name string, data []byte) {
	t.Helper()
	virtualDirectory := []interface{}{
		int64(1000),
		map[string]interface{}{},
		map[string]interface{}{
			name: []interface{}{int64(ct.HeaderSize), int64(len(data))},
		},
	}
	h := &codec.MsgpackHandle{}
	var msgpackData []byte
	if err := codec.NewEncoderBytes(&msgpackData, h).Encode(virtualDirectory); err != nil {
		t.Fatalf("encode malicious CT directory: %v", err)
	}
	compressed, err := ct.CompressLz4BlockArray(msgpackData)
	if err != nil {
		t.Fatalf("compress malicious CT directory: %v", err)
	}

	var file bytes.Buffer
	file.Write(ct.FileSignature)
	file.WriteByte(ct.SerializeTypeMsgPack)
	file.Write(data)
	file.Write(compressed)
	var footer [4]byte
	binary.LittleEndian.PutUint32(footer[:], uint32(len(compressed)))
	file.Write(footer[:])
	if err := os.WriteFile(path, file.Bytes(), 0644); err != nil {
		t.Fatalf("write malicious CT: %v", err)
	}
}
