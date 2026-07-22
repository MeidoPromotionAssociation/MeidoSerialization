package blobstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxBlobBytes  int64 = 4 << 30
	DefaultMaxTotalBytes int64 = 16 << 30
	DefaultMaxBlobs            = 4096
	DefaultMaxNameBytes        = 255
	DefaultTTL                 = 30 * time.Minute
)

var (
	// ErrResourceExhausted identifies a store quota failure to transport layers.
	ErrResourceExhausted = errors.New("blob store resource limit exceeded")
	ErrInvalidArgument   = errors.New("invalid blob argument")
)

type Config struct {
	Directory     string
	MaxBlobBytes  int64
	MaxTotalBytes int64
	MaxBlobs      int
	MaxNameBytes  int
	TTL           time.Duration
}

type Metadata struct {
	ID        string
	Name      string
	Size      int64
	SHA256    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type item struct {
	meta        Metadata
	openCount   int
	deleteAfter bool
}

type Store struct {
	mu            sync.Mutex
	directory     string
	ownedDir      bool
	directoryLock *directoryLock
	maxBlobBytes  int64
	maxTotalBytes int64
	maxBlobs      int
	maxNameBytes  int
	ttl           time.Duration
	totalBytes    int64
	inFlightBytes int64
	inFlightBlobs int
	items         map[string]*item
	openFiles     map[*File]struct{}
	closed        bool
	activePuts    sync.WaitGroup
	closeOnce     sync.Once
	closeDone     chan struct{}
	closeErr      error
	janitorStop   chan struct{}
	janitorDone   chan struct{}
}

var blobIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var uploadTempPattern = regexp.MustCompile(`^\.upload-[0-9]+\.tmp$`)

const blobStoreLockFileName = ".meido-blobstore.lock"

func New(config Config) (*Store, error) {
	if config.MaxBlobBytes <= 0 {
		config.MaxBlobBytes = DefaultMaxBlobBytes
	}
	if config.MaxTotalBytes <= 0 {
		config.MaxTotalBytes = DefaultMaxTotalBytes
	}
	if config.MaxTotalBytes < config.MaxBlobBytes {
		return nil, fmt.Errorf("maximum total blob size must be at least the maximum single blob size")
	}
	if config.MaxBlobs <= 0 {
		config.MaxBlobs = DefaultMaxBlobs
	}
	if config.MaxNameBytes <= 0 {
		config.MaxNameBytes = DefaultMaxNameBytes
	}
	if config.TTL <= 0 {
		config.TTL = DefaultTTL
	}
	directory := config.Directory
	owned := false
	var err error
	if directory == "" {
		directory, err = os.MkdirTemp("", "meido-blobs-")
		owned = true
	} else {
		directory, err = filepath.Abs(directory)
		if err == nil {
			err = os.MkdirAll(directory, 0700)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("create blob directory: %w", err)
	}
	var lock *directoryLock
	if !owned {
		lock, err = acquireDirectoryLock(filepath.Join(directory, blobStoreLockFileName))
		if err != nil {
			return nil, fmt.Errorf("lock blob directory %q: %w", directory, err)
		}
	}
	if err := removeStaleFiles(directory); err != nil {
		if lock != nil {
			_ = lock.Close()
		}
		if owned {
			_ = os.RemoveAll(directory)
		}
		return nil, fmt.Errorf("clean blob directory: %w", err)
	}
	s := &Store{
		directory: directory, ownedDir: owned, directoryLock: lock, maxBlobBytes: config.MaxBlobBytes,
		maxTotalBytes: config.MaxTotalBytes, maxBlobs: config.MaxBlobs,
		maxNameBytes: config.MaxNameBytes, ttl: config.TTL,
		items: make(map[string]*item), openFiles: make(map[*File]struct{}),
		closeDone: make(chan struct{}), janitorStop: make(chan struct{}),
		janitorDone: make(chan struct{}),
	}
	go s.runJanitor()
	return s, nil
}

func (s *Store) Limits() (maxBlobBytes, maxTotalBytes int64) {
	return s.maxBlobBytes, s.maxTotalBytes
}

func (s *Store) ObjectLimits() (maxBlobs, maxNameBytes int) {
	return s.maxBlobs, s.maxNameBytes
}

func (s *Store) Put(ctx context.Context, name string, reader io.Reader) (Metadata, error) {
	if reader == nil {
		return Metadata{}, fmt.Errorf("%w: blob reader is required", ErrInvalidArgument)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cleanName, err := s.normalizeName(name)
	if err != nil {
		return Metadata{}, err
	}
	if err := s.beginPut(); err != nil {
		return Metadata{}, err
	}
	defer s.activePuts.Done()
	putActive := true
	reserved := int64(0)
	defer func() {
		if putActive {
			s.abortPut(reserved)
		}
	}()

	temp, err := os.CreateTemp(s.directory, ".upload-*.tmp")
	if err != nil {
		return Metadata{}, fmt.Errorf("create blob temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	quota := &quotaWriter{store: s, writer: temp, maxBytes: s.maxBlobBytes}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(quota, hash), io.LimitReader(&contextReader{ctx: ctx, reader: reader}, limitWithSentinel(s.maxBlobBytes)))
	reserved = quota.reserved
	closeErr := temp.Close()
	if copyErr != nil {
		return Metadata{}, copyErr
	}
	if closeErr != nil {
		return Metadata{}, closeErr
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	if written > s.maxBlobBytes {
		return Metadata{}, fmt.Errorf("%w: blob size exceeds limit %d", ErrResourceExhausted, s.maxBlobBytes)
	}

	id, err := randomID()
	if err != nil {
		return Metadata{}, err
	}
	now := time.Now().UTC()
	meta := Metadata{ID: id, Name: cleanName, Size: written, SHA256: hex.EncodeToString(hash.Sum(nil)), CreatedAt: now, ExpiresAt: now.Add(s.ttl)}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Metadata{}, fmt.Errorf("blob store is closed")
	}
	s.cleanupExpiredLocked(now)
	if _, exists := s.items[id]; exists {
		return Metadata{}, fmt.Errorf("generated duplicate blob ID %q", id)
	}
	if err := os.Rename(tempPath, s.path(id)); err != nil {
		return Metadata{}, fmt.Errorf("commit blob: %w", err)
	}
	s.inFlightBytes -= reserved
	s.inFlightBlobs--
	putActive = false
	s.items[id] = &item{meta: meta}
	s.totalBytes += written
	return meta, nil
}

func (s *Store) Open(id string) (*File, Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, Metadata{}, fmt.Errorf("blob store is closed")
	}
	if !blobIDPattern.MatchString(id) {
		return nil, Metadata{}, fmt.Errorf("%w: invalid blob ID", ErrInvalidArgument)
	}
	s.cleanupExpiredLocked(time.Now().UTC())
	entry, ok := s.items[id]
	if !ok || entry.deleteAfter {
		return nil, Metadata{}, os.ErrNotExist
	}
	f, err := os.Open(s.path(id))
	if err != nil {
		return nil, Metadata{}, err
	}
	entry.openCount++
	result := &File{file: f, store: s, id: id}
	s.openFiles[result] = struct{}{}
	return result, entry.meta, nil
}

func (s *Store) Metadata(id string) (Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Metadata{}, fmt.Errorf("blob store is closed")
	}
	if !blobIDPattern.MatchString(id) {
		return Metadata{}, fmt.Errorf("%w: invalid blob ID", ErrInvalidArgument)
	}
	s.cleanupExpiredLocked(time.Now().UTC())
	entry, ok := s.items[id]
	if !ok || entry.deleteAfter {
		return Metadata{}, os.ErrNotExist
	}
	return entry.meta, nil
}

// Delete marks a blob unavailable immediately. Its quota is released only
// after all open handles close and the underlying file is removed successfully.
func (s *Store) Delete(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, fmt.Errorf("blob store is closed")
	}
	if !blobIDPattern.MatchString(id) {
		return false, fmt.Errorf("%w: invalid blob ID", ErrInvalidArgument)
	}
	s.cleanupExpiredLocked(time.Now().UTC())
	entry, ok := s.items[id]
	if !ok || entry.deleteAfter {
		return false, nil
	}
	if entry.openCount > 0 {
		entry.deleteAfter = true
		return true, nil
	}
	if err := s.removeItemLocked(id, entry); err != nil {
		return false, fmt.Errorf("delete blob: %w", err)
	}
	return true, nil
}

func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		directory := s.directory
		owned := s.ownedDir
		openFiles := make([]*File, 0, len(s.openFiles))
		for file := range s.openFiles {
			openFiles = append(openFiles, file)
		}
		s.mu.Unlock()

		close(s.janitorStop)
		<-s.janitorDone
		for _, file := range openFiles {
			_ = file.Close()
		}
		s.activePuts.Wait()

		s.mu.Lock()
		ids := make([]string, 0, len(s.items))
		for id := range s.items {
			ids = append(ids, id)
		}
		s.items = nil
		s.openFiles = nil
		s.totalBytes = 0
		s.inFlightBytes = 0
		s.inFlightBlobs = 0
		s.mu.Unlock()

		if owned {
			s.closeErr = os.RemoveAll(directory)
		} else {
			var result error
			for _, id := range ids {
				if err := os.Remove(filepath.Join(directory, id+".blob")); err != nil && !os.IsNotExist(err) {
					result = errors.Join(result, err)
				}
			}
			if s.directoryLock != nil {
				result = errors.Join(result, s.directoryLock.Close())
			}
			s.closeErr = result
		}
		close(s.closeDone)
	})
	<-s.closeDone
	return s.closeErr
}

func (s *Store) beginPut() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("blob store is closed")
	}
	s.cleanupExpiredLocked(time.Now().UTC())
	if len(s.items)+s.inFlightBlobs >= s.maxBlobs {
		return fmt.Errorf("%w: blob count limit %d", ErrResourceExhausted, s.maxBlobs)
	}
	s.inFlightBlobs++
	s.activePuts.Add(1)
	return nil
}

func (s *Store) reserveBytes(n int64) error {
	if n <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("blob store is closed")
	}
	if n > s.maxTotalBytes-s.totalBytes-s.inFlightBytes {
		return fmt.Errorf("%w: total blob storage limit %d would be exceeded", ErrResourceExhausted, s.maxTotalBytes)
	}
	s.inFlightBytes += n
	return nil
}

func (s *Store) releaseBytes(n int64) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	s.inFlightBytes -= n
	if s.inFlightBytes < 0 {
		s.inFlightBytes = 0
	}
	s.mu.Unlock()
}

func (s *Store) abortPut(reserved int64) {
	s.mu.Lock()
	s.inFlightBytes -= reserved
	if s.inFlightBytes < 0 {
		s.inFlightBytes = 0
	}
	if s.inFlightBlobs > 0 {
		s.inFlightBlobs--
	}
	s.mu.Unlock()
}

func (s *Store) normalizeName(name string) (string, error) {
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("%w: blob name contains NUL", ErrInvalidArgument)
	}
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if name == "." || name == "" || name == string(filepath.Separator) {
		name = "blob.bin"
	}
	if len([]byte(name)) > s.maxNameBytes {
		return "", fmt.Errorf("%w: blob name exceeds %d bytes", ErrInvalidArgument, s.maxNameBytes)
	}
	return name, nil
}

func (s *Store) cleanupExpiredLocked(now time.Time) {
	for id, entry := range s.items {
		if !entry.deleteAfter && now.Before(entry.meta.ExpiresAt) {
			continue
		}
		entry.deleteAfter = true
		if entry.openCount == 0 {
			_ = s.removeItemLocked(id, entry)
		}
	}
}

func (s *Store) removeItemLocked(id string, entry *item) error {
	if err := os.Remove(s.path(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(s.items, id)
	s.totalBytes -= entry.meta.Size
	if s.totalBytes < 0 {
		s.totalBytes = 0
	}
	return nil
}

func (s *Store) releaseOpen(id string, file *File) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.openFiles, file)
	entry, ok := s.items[id]
	if !ok {
		return
	}
	if entry.openCount > 0 {
		entry.openCount--
	}
	if entry.openCount == 0 && entry.deleteAfter {
		_ = s.removeItemLocked(id, entry)
	}
}

func (s *Store) runJanitor() {
	defer close(s.janitorDone)
	interval := s.ttl / 2
	if interval > time.Minute {
		interval = time.Minute
	}
	if interval < time.Millisecond {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.janitorStop:
			return
		case now := <-ticker.C:
			s.mu.Lock()
			if !s.closed {
				s.cleanupExpiredLocked(now.UTC())
			}
			s.mu.Unlock()
		}
	}
}

func removeStaleFiles(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		blobID := strings.TrimSuffix(name, ".blob")
		if !(strings.HasSuffix(name, ".blob") && blobIDPattern.MatchString(blobID)) && !uploadTempPattern.MatchString(name) {
			continue
		}
		if err := os.Remove(filepath.Join(directory, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Store) path(id string) string { return filepath.Join(s.directory, id+".blob") }

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate blob ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

type File struct {
	file  *os.File
	store *Store
	id    string
	once  sync.Once
	err   error
}

func (f *File) Read(p []byte) (int, error) {
	if f == nil || f.file == nil {
		return 0, os.ErrInvalid
	}
	return f.file.Read(p)
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f == nil || f.file == nil {
		return 0, os.ErrInvalid
	}
	return f.file.Seek(offset, whence)
}

func (f *File) Close() error {
	if f == nil || f.file == nil {
		return nil
	}
	f.once.Do(func() {
		f.err = f.file.Close()
		f.store.releaseOpen(f.id, f)
	})
	return f.err
}

type quotaWriter struct {
	store    *Store
	writer   io.Writer
	maxBytes int64
	reserved int64
}

func (w *quotaWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.maxBytes-w.reserved {
		return 0, fmt.Errorf("%w: blob size exceeds limit %d", ErrResourceExhausted, w.maxBytes)
	}
	n := int64(len(p))
	if err := w.store.reserveBytes(n); err != nil {
		return 0, err
	}
	written, err := w.writer.Write(p)
	if written < 0 || written > len(p) {
		written = 0
		err = fmt.Errorf("invalid temporary file writer count")
	}
	if int64(written) < n {
		w.store.releaseBytes(n - int64(written))
	}
	w.reserved += int64(written)
	if err != nil {
		return written, err
	}
	if written != len(p) {
		return written, io.ErrShortWrite
	}
	return written, nil
}

func limitWithSentinel(limit int64) int64 {
	if limit == math.MaxInt64 {
		return limit
	}
	return limit + 1
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
