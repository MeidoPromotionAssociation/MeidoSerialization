package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootSetConfinesReadsAndWrites(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "input.menu"), []byte("input"), 0644); err != nil {
		t.Fatal(err)
	}
	roots := NewRootSet()
	defer roots.Close()
	if err := roots.AddWritable("mods", directory); err != nil {
		t.Fatalf("Add: %v", err)
	}
	source, err := roots.Resolve("mods", "input.menu")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	reader, err := source.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(data) != "input" {
		t.Fatalf("read = %q, err=%v", data, err)
	}

	for _, unsafe := range []string{"../outside", `..\outside`, "/absolute", `\absolute`, `C:\absolute`, `C:relative`, `\\server\share`} {
		if _, err := roots.Resolve("mods", unsafe); err == nil || CodeOf(err) != CodeInvalidArgument {
			t.Fatalf("Resolve(%q) error = %v", unsafe, err)
		}
	}

	written, digest, err := roots.WriteFile(context.Background(), "mods", "nested/output.json", bytes.NewBufferString("first"), 64)
	if err != nil || written != 5 || len(digest) != 64 {
		t.Fatalf("WriteFile: written=%d digest=%q err=%v", written, digest, err)
	}
	written, _, err = roots.WriteFile(context.Background(), "mods", "nested/output.json", bytes.NewBufferString("replacement"), 64)
	if err != nil || written != int64(len("replacement")) {
		t.Fatalf("replace WriteFile: written=%d err=%v", written, err)
	}
	got, err := os.ReadFile(filepath.Join(directory, "nested", "output.json"))
	if err != nil || string(got) != "replacement" {
		t.Fatalf("rooted output = %q, err=%v", got, err)
	}
}

func TestRootSetWriteLimitDoesNotInstallPartialFile(t *testing.T) {
	directory := t.TempDir()
	roots := NewRootSet()
	defer roots.Close()
	if err := roots.AddWritable("mods", directory); err != nil {
		t.Fatal(err)
	}
	if _, _, err := roots.WriteFile(context.Background(), "mods", "too-large.bin", bytes.NewReader(make([]byte, 65)), 64); CodeOf(err) != CodeResourceExhausted {
		t.Fatalf("WriteFile error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "too-large.bin")); !os.IsNotExist(err) {
		t.Fatalf("partial destination exists: %v", err)
	}
}

func TestRootSetReadOnlyRootRejectsWritesAndPreservesDestination(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "existing.bin")
	if err := os.WriteFile(destination, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	roots := NewRootSet()
	defer roots.Close()
	if err := roots.Add("read", directory); err != nil {
		t.Fatal(err)
	}
	if _, _, err := roots.WriteFile(context.Background(), "read", "existing.bin", bytes.NewReader([]byte("replacement")), 64); CodeOf(err) != CodePermissionDenied {
		t.Fatalf("read-only WriteFile error = %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "original" {
		t.Fatalf("read-only destination = %q, err=%v", got, err)
	}
}

func TestRootSetWriteRejectsNonRegularDestination(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "target"), 0755); err != nil {
		t.Fatal(err)
	}
	roots := NewRootSet()
	defer roots.Close()
	if err := roots.AddWritable("write", directory); err != nil {
		t.Fatal(err)
	}
	if _, _, err := roots.WriteFile(context.Background(), "write", "target", bytes.NewReader([]byte("x")), 16); CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("directory destination error = %v", err)
	}
}

func TestRootSetResolveIncludesRawUnityAttachments(t *testing.T) {
	directory := t.TempDir()
	primary := filepath.Join(directory, "hair.mmesh.bytes")
	if err := os.WriteFile(primary, []byte("raw"), 0644); err != nil {
		t.Fatal(err)
	}
	for suffix, data := range map[string]string{
		".meta.json":     `{"pathId":42}`,
		".typetree.json": `{"format":"kces-unity-typetree"}`,
	} {
		if err := os.WriteFile(primary+suffix, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	roots := NewRootSet()
	defer roots.Close()
	if err := roots.Add("mods", directory); err != nil {
		t.Fatal(err)
	}
	source, err := roots.Resolve("mods", "hair.mmesh.bytes")
	if err != nil {
		t.Fatal(err)
	}
	attachments := sourceAttachments(source)
	if len(attachments) != 2 || attachments[0].Suffix != ".meta.json" || attachments[1].Suffix != ".typetree.json" {
		t.Fatalf("resolved attachments = %+v", attachments)
	}
}

func TestRootSetWriteBundleInstallsAndPrunesSidecars(t *testing.T) {
	directory := t.TempDir()
	primary := filepath.Join(directory, "hair.mmesh.bytes")
	for path, data := range map[string]string{
		primary:                    "old raw",
		primary + ".meta.json":     "old meta",
		primary + ".typetree.json": "old tree",
	} {
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	roots := NewRootSet()
	defer roots.Close()
	if err := roots.AddWritable("work", directory); err != nil {
		t.Fatal(err)
	}
	rawSize := int64(len("new raw"))
	metaSize := int64(len("new meta"))
	metadata, err := roots.WriteBundle(context.Background(), "work", "hair.mmesh.bytes", []BundleFile{
		{Reader: bytes.NewBufferString("new raw"), ExpectedSize: &rawSize, ExpectedSHA256: bundleTestDigest("new raw")},
		{Suffix: ".meta.json", Reader: bytes.NewBufferString("new meta"), ExpectedSize: &metaSize, ExpectedSHA256: bundleTestDigest("new meta")},
	}, 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 2 {
		t.Fatalf("bundle metadata = %+v", metadata)
	}
	for path, want := range map[string]string{primary: "new raw", primary + ".meta.json": "new meta"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, err=%v", path, got, err)
		}
	}
	if _, err := os.Stat(primary + ".typetree.json"); !os.IsNotExist(err) {
		t.Fatalf("stale TypeTree sidecar remains: %v", err)
	}
}

func TestRootSetWriteBundleRejectsMetadataMismatchBeforeCommit(t *testing.T) {
	for _, mismatch := range []string{"size", "sha256"} {
		t.Run(mismatch, func(t *testing.T) {
			directory := t.TempDir()
			primary := filepath.Join(directory, "hair.mmesh.bytes")
			for path, data := range map[string]string{
				primary:                "old raw",
				primary + ".meta.json": "old meta",
			} {
				if err := os.WriteFile(path, []byte(data), 0644); err != nil {
					t.Fatal(err)
				}
			}
			roots := NewRootSet()
			defer roots.Close()
			if err := roots.AddWritable("work", directory); err != nil {
				t.Fatal(err)
			}

			expectedSize := int64(len("new raw"))
			expectedDigest := bundleTestDigest("new raw")
			if mismatch == "size" {
				expectedSize++
			} else {
				expectedDigest = strings.Repeat("0", 64)
			}
			_, err := roots.WriteBundle(context.Background(), "work", "hair.mmesh.bytes", []BundleFile{
				{Reader: bytes.NewBufferString("new raw"), ExpectedSize: &expectedSize, ExpectedSHA256: expectedDigest},
			}, 64)
			if CodeOf(err) != CodeInvalidArgument {
				t.Fatalf("metadata mismatch error = %v", err)
			}
			for path, want := range map[string]string{primary: "old raw", primary + ".meta.json": "old meta"} {
				got, readErr := os.ReadFile(path)
				if readErr != nil || string(got) != want {
					t.Fatalf("pre-commit mismatch changed %s to %q, err=%v", path, got, readErr)
				}
			}
			entries, readErr := os.ReadDir(directory)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".meido-write-") {
					t.Fatalf("metadata mismatch left staging file %q", entry.Name())
				}
			}
		})
	}
}

func bundleTestDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
