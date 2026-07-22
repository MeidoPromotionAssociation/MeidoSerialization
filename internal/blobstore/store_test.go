package blobstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStorePutOpenDelete(t *testing.T) {
	directory := t.TempDir()
	store, err := New(Config{Directory: directory, MaxBlobBytes: 64, MaxTotalBytes: 128})
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Put(context.Background(), "sample.menu", bytes.NewBufferString("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "sample.menu" || meta.Size != 7 || len(meta.SHA256) != 64 {
		t.Fatalf("metadata = %+v", meta)
	}
	file, opened, err := store.Open(meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || string(data) != "payload" || opened != meta {
		t.Fatalf("opened=%+v data=%q err=%v", opened, data, err)
	}
	deleted, err := store.Delete(meta.ID)
	if err != nil || !deleted {
		t.Fatalf("Delete first call: deleted=%v err=%v", deleted, err)
	}
	deleted, err = store.Delete(meta.ID)
	if err != nil || deleted {
		t.Fatal("Delete did not report existence correctly")
	}
	if _, _, err := store.Open(meta.ID); !os.IsNotExist(err) {
		t.Fatalf("Open deleted blob error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreLimits(t *testing.T) {
	store, err := New(Config{MaxBlobBytes: 4, MaxTotalBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Put(context.Background(), "large", bytes.NewReader([]byte("12345"))); err == nil {
		t.Fatal("oversized blob was accepted")
	}
	if _, err := store.Put(context.Background(), "one", bytes.NewReader([]byte("1234"))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "two", bytes.NewReader([]byte("5678"))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "three", bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("total storage limit was not enforced")
	}
}

func TestStoreRejectsMalformedBlobIDs(t *testing.T) {
	store, err := New(Config{MaxBlobBytes: 8, MaxTotalBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, _, err := store.Open("not-an-id"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Open malformed ID = %v", err)
	}
	if _, err := store.Metadata("not-an-id"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Metadata malformed ID = %v", err)
	}
	if _, err := store.Delete("not-an-id"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Delete malformed ID = %v", err)
	}
}

func TestStoreDeleteDefersRemovalUntilOpenHandleCloses(t *testing.T) {
	store, err := New(Config{MaxBlobBytes: 32, MaxTotalBytes: 32})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	meta, err := store.Put(context.Background(), "held.bin", bytes.NewReader([]byte("held")))
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := store.Open(meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := store.Delete(meta.ID)
	if err != nil || !deleted {
		t.Fatalf("Delete open blob: deleted=%v err=%v", deleted, err)
	}
	if _, err := store.Metadata(meta.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Metadata after logical delete = %v", err)
	}
	data, err := io.ReadAll(file)
	if err != nil || string(data) != "held" {
		t.Fatalf("open handle read = %q, err=%v", data, err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.directory, meta.ID+".blob")); !os.IsNotExist(err) {
		t.Fatalf("deleted blob still exists after close: %v", err)
	}
}

func TestStoreObjectAndNameLimitsIncludeZeroByteBlobs(t *testing.T) {
	store, err := New(Config{MaxBlobBytes: 8, MaxTotalBytes: 8, MaxBlobs: 2, MaxNameBytes: 5})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Put(context.Background(), "123456", bytes.NewReader(nil)); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("long name error = %v", err)
	}
	if _, err := store.Put(context.Background(), "a", bytes.NewReader(nil)); err != nil {
		t.Fatalf("zero-byte blob: %v", err)
	}
	if _, err := store.Put(context.Background(), "b", bytes.NewReader(nil)); err != nil {
		t.Fatalf("second zero-byte blob: %v", err)
	}
	if _, err := store.Put(context.Background(), "c", bytes.NewReader(nil)); !errors.Is(err, ErrResourceExhausted) {
		t.Fatalf("object limit error = %v", err)
	}
}

func TestStoreConcurrentPutsReserveTotalBytes(t *testing.T) {
	store, err := New(Config{MaxBlobBytes: 8, MaxTotalBytes: 8, MaxBlobs: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var successes int
	var resourceErrors int
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, putErr := store.Put(context.Background(), string(rune('a'+index)), bytes.NewReader(bytes.Repeat([]byte{'x'}, 8)))
			mu.Lock()
			if putErr == nil {
				successes++
			} else if errors.Is(putErr, ErrResourceExhausted) {
				resourceErrors++
			}
			mu.Unlock()
		}(i)
	}
	close(start)
	wg.Wait()
	if successes > 1 || successes+resourceErrors != 2 {
		t.Fatalf("concurrent quota results: successes=%d resourceErrors=%d", successes, resourceErrors)
	}
}

func TestStoreCloseClosesOpenFilesAndIsIdempotent(t *testing.T) {
	directory := t.TempDir()
	store, err := New(Config{Directory: directory, MaxBlobBytes: 8, MaxTotalBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Put(context.Background(), "held", bytes.NewReader([]byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := store.Open(meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Read(make([]byte, 1)); err == nil {
		t.Fatalf("read from force-closed file = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, meta.ID+".blob")); !os.IsNotExist(err) {
		t.Fatalf("persistent blob remains after close: %v", err)
	}
}

func TestStoreJanitorExpiresBlobs(t *testing.T) {
	store, err := New(Config{MaxBlobBytes: 8, MaxTotalBytes: 8, TTL: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	meta, err := store.Put(context.Background(), "ttl", bytes.NewReader([]byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := store.Metadata(meta.ID); errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("janitor did not expire blob")
}

func TestStoreStartupCleanupPreservesUnrelatedFiles(t *testing.T) {
	directory := t.TempDir()
	staleBlob := filepath.Join(directory, strings.Repeat("a", 32)+".blob")
	staleUpload := filepath.Join(directory, ".upload-12345.tmp")
	unrelated := []string{
		filepath.Join(directory, "notes.blob"),
		filepath.Join(directory, ".upload-manual.tmp"),
		filepath.Join(directory, "keep.txt"),
	}
	for _, path := range append([]string{staleBlob, staleUpload}, unrelated...) {
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	store, err := New(Config{Directory: directory, MaxBlobBytes: 8, MaxTotalBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, path := range []string{staleBlob, staleUpload} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("managed stale file remains at %q: %v", path, err)
		}
	}
	for _, path := range unrelated {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("unrelated file %q was removed: %v", path, err)
		}
	}
}

func TestStoresCannotShareActiveDirectory(t *testing.T) {
	directory := t.TempDir()
	first, err := New(Config{Directory: directory, MaxBlobBytes: 16, MaxTotalBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	meta, err := first.Put(context.Background(), "active", bytes.NewBufferString("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if second, err := New(Config{Directory: directory, MaxBlobBytes: 16, MaxTotalBytes: 16}); err == nil {
		_ = second.Close()
		t.Fatal("a second store acquired an active blob directory")
	}
	file, _, err := first.Open(meta.ID)
	if err != nil {
		t.Fatalf("second New removed the first store's blob: %v", err)
	}
	data, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || string(data) != "payload" {
		t.Fatalf("first store blob = %q, err=%v", data, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(Config{Directory: directory, MaxBlobBytes: 16, MaxTotalBytes: 16})
	if err != nil {
		t.Fatalf("directory lock was not released: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreCloseWaitsForActivePutAndCleansTemporaryFile(t *testing.T) {
	directory := t.TempDir()
	store, err := New(Config{Directory: directory, MaxBlobBytes: 8, MaxTotalBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	reader := &gatedReader{started: make(chan struct{}), release: make(chan struct{})}
	putDone := make(chan error, 1)
	go func() {
		_, putErr := store.Put(context.Background(), "active", reader)
		putDone <- putErr
	}()
	<-reader.started
	closeDone := make(chan error, 1)
	go func() { closeDone <- store.Close() }()
	deadline := time.Now().Add(time.Second)
	for {
		store.mu.Lock()
		closed := store.closed
		store.mu.Unlock()
		if closed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Close did not mark the store closed")
		}
		time.Sleep(time.Millisecond)
	}
	close(reader.release)
	if err := <-putDone; err == nil {
		t.Fatal("active Put committed after Close began")
	}
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != blobStoreLockFileName {
			t.Fatalf("blob directory contains temporary file after Close: %v", entries)
		}
	}
}

type gatedReader struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	sent    bool
}

func (r *gatedReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.release
	if r.sent {
		return 0, io.EOF
	}
	r.sent = true
	return copy(p, []byte("x")), nil
}
