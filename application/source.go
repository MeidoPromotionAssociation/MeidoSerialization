package application

import (
	"bytes"
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
	"sort"
	"strings"
	"sync"
)

// ReadSeekCloser is the common input primitive used by the application layer.
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// Source represents a named, seekable artifact without prescribing its storage.
type Source interface {
	Name() string
	Size() int64
	Open(context.Context) (ReadSeekCloser, error)
}

// SourceAttachment is a companion file whose name is derived by appending
// Suffix to the primary source name. KCES raw Unity objects use these files to
// carry AssetBundle metadata and a TypeTree view.
type SourceAttachment struct {
	Suffix string
	Source Source
}

var artifactAttachmentSuffixes = []string{".meta.json", ".typetree.json"}

// ArtifactAttachmentSuffixes returns the sidecar suffixes managed as part of
// one application artifact.
func ArtifactAttachmentSuffixes() []string {
	return append([]string(nil), artifactAttachmentSuffixes...)
}

type attachmentSource interface {
	Attachments() []SourceAttachment
}

type bundleSource struct {
	Source
	attachments []SourceAttachment
}

func (s *bundleSource) Attachments() []SourceAttachment {
	result := make([]SourceAttachment, len(s.attachments))
	copy(result, s.attachments)
	return result
}

// NewBundleSource associates supported companion files with a primary source.
// Existing companions exposed by primary are retained, and duplicate suffixes
// are rejected.
func NewBundleSource(primary Source, attachments []SourceAttachment) (Source, error) {
	if primary == nil {
		return nil, opError("create source bundle", CodeInvalidArgument, fmt.Errorf("primary source is required"))
	}
	all := sourceAttachments(primary)
	all = append(all, attachments...)
	if len(all) == 0 {
		return primary, nil
	}
	seen := make(map[string]struct{}, len(all))
	for i := range all {
		if all[i].Source == nil {
			return nil, opError("create source bundle", CodeInvalidArgument, fmt.Errorf("attachment %d has no source", i))
		}
		suffix, err := normalizeAttachmentSuffix(all[i].Suffix)
		if err != nil {
			return nil, opError("create source bundle", CodeInvalidArgument, err)
		}
		if _, exists := seen[suffix]; exists {
			return nil, opError("create source bundle", CodeInvalidArgument, fmt.Errorf("duplicate attachment suffix %q", suffix))
		}
		seen[suffix] = struct{}{}
		all[i].Suffix = suffix
	}
	return &bundleSource{Source: primary, attachments: all}, nil
}

func sourceAttachments(source Source) []SourceAttachment {
	provider, ok := source.(attachmentSource)
	if !ok {
		return nil
	}
	return provider.Attachments()
}

func normalizeAttachmentSuffix(value string) (string, error) {
	suffix := strings.ToLower(strings.TrimSpace(value))
	for _, supported := range artifactAttachmentSuffixes {
		if suffix == supported {
			return supported, nil
		}
	}
	return "", fmt.Errorf("unsupported artifact attachment suffix %q", value)
}

type bytesSource struct {
	name string
	data []byte
}

// NewBytesSource creates an immutable in-memory source.
func NewBytesSource(name string, data []byte) Source {
	return &bytesSource{name: cleanSourceName(name), data: append([]byte(nil), data...)}
}

func (s *bytesSource) Name() string { return s.name }
func (s *bytesSource) Size() int64  { return int64(len(s.data)) }
func (s *bytesSource) Open(ctx context.Context) (ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &readSeekNopCloser{Reader: bytes.NewReader(s.data)}, nil
}

type readSeekNopCloser struct{ *bytes.Reader }

func (r *readSeekNopCloser) Close() error { return nil }

type fileSource struct {
	name string
	path string
	size int64
}

// NewFileSource creates a source for a regular local file. RPC callers should
// normally use RootSet.Resolve instead, so they cannot choose arbitrary paths.
func NewFileSource(path string) (Source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, opError("open source", CodeNotFound, err)
	}
	if !info.Mode().IsRegular() {
		return nil, opError("open source", CodeInvalidArgument, fmt.Errorf("%q is not a regular file", path))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, opError("resolve source", CodeInvalidArgument, err)
	}
	primary := &fileSource{name: filepath.Base(abs), path: abs, size: info.Size()}
	attachments, err := discoverFileAttachments(abs)
	if err != nil {
		return nil, err
	}
	return NewBundleSource(primary, attachments)
}

func (s *fileSource) Name() string { return s.name }
func (s *fileSource) Size() int64  { return s.size }
func (s *fileSource) Open(ctx context.Context) (ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.Open(s.path)
}

func discoverFileAttachments(path string) ([]SourceAttachment, error) {
	var result []SourceAttachment
	for _, suffix := range artifactAttachmentSuffixes {
		attachmentPath := path + suffix
		info, err := os.Stat(attachmentPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, opError("inspect source attachment", CodeInvalidArgument, err)
		}
		if !info.Mode().IsRegular() {
			return nil, opError("inspect source attachment", CodeInvalidArgument, fmt.Errorf("%q is not a regular file", attachmentPath))
		}
		result = append(result, SourceAttachment{
			Suffix: suffix,
			Source: &fileSource{name: filepath.Base(attachmentPath), path: attachmentPath, size: info.Size()},
		})
	}
	return result, nil
}

var rootIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// RootSet maps public root IDs to server-configured directories. A caller only
// supplies a portable relative path; absolute paths and traversal are rejected.
type RootSet struct {
	mu      sync.RWMutex
	writeMu sync.Mutex
	roots   map[string]rootEntry
}

type rootEntry struct {
	root     *os.Root
	writable bool
}

func NewRootSet() *RootSet { return &RootSet{roots: make(map[string]rootEntry)} }

func (r *RootSet) Add(id, directory string) error {
	return r.add(id, directory, false)
}

func (r *RootSet) AddWritable(id, directory string) error {
	return r.add(id, directory, true)
}

func (r *RootSet) add(id, directory string, writable bool) error {
	if !rootIDPattern.MatchString(id) {
		return opError("add root", CodeInvalidArgument, fmt.Errorf("invalid root ID %q", id))
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return opError("add root", CodeInvalidArgument, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return opError("add root", CodeNotFound, err)
	}
	if !info.IsDir() {
		return opError("add root", CodeInvalidArgument, fmt.Errorf("%q is not a directory", directory))
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return opError("add root", CodeInvalidArgument, fmt.Errorf("open root %q: %w", directory, err))
	}
	r.mu.Lock()
	if r.roots == nil {
		r.mu.Unlock()
		_ = root.Close()
		return opError("add root", CodeInternal, fmt.Errorf("root set is closed"))
	}
	if _, exists := r.roots[id]; exists {
		r.mu.Unlock()
		_ = root.Close()
		return opError("add root", CodeInvalidArgument, fmt.Errorf("duplicate root ID %q", id))
	}
	r.roots[id] = rootEntry{root: root, writable: writable}
	r.mu.Unlock()
	return nil
}

func (r *RootSet) IDs() []string {
	r.mu.RLock()
	ids := make([]string, 0, len(r.roots))
	for id := range r.roots {
		ids = append(ids, id)
	}
	r.mu.RUnlock()
	sort.Strings(ids)
	return ids
}

func (r *RootSet) WritableIDs() []string {
	r.mu.RLock()
	ids := make([]string, 0, len(r.roots))
	for id, entry := range r.roots {
		if entry.writable {
			ids = append(ids, id)
		}
	}
	r.mu.RUnlock()
	sort.Strings(ids)
	return ids
}

func (r *RootSet) Resolve(id, relativePath string) (Source, error) {
	entry, ok := r.root(id)
	if !ok {
		return nil, opError("resolve file", CodeNotFound, fmt.Errorf("unknown root ID %q", id))
	}
	rel, err := normalizeRelativePath(relativePath)
	if err != nil {
		return nil, opError("resolve file", CodeInvalidArgument, err)
	}
	info, err := entry.root.Stat(rel)
	if err != nil {
		return nil, opError("resolve file", CodeNotFound, err)
	}
	if !info.Mode().IsRegular() {
		return nil, opError("resolve file", CodeInvalidArgument, fmt.Errorf("%q is not a regular file", relativePath))
	}
	primary := &rootSource{name: filepath.Base(rel), root: entry.root, rel: rel, size: info.Size()}
	var attachments []SourceAttachment
	for _, suffix := range artifactAttachmentSuffixes {
		attachmentRel := rel + suffix
		attachmentInfo, attachmentErr := entry.root.Stat(attachmentRel)
		if os.IsNotExist(attachmentErr) {
			continue
		}
		if attachmentErr != nil {
			return nil, opError("resolve file attachment", CodeInvalidArgument, attachmentErr)
		}
		if !attachmentInfo.Mode().IsRegular() {
			return nil, opError("resolve file attachment", CodeInvalidArgument, fmt.Errorf("%q is not a regular file", filepath.ToSlash(attachmentRel)))
		}
		attachments = append(attachments, SourceAttachment{
			Suffix: suffix,
			Source: &rootSource{name: filepath.Base(attachmentRel), root: entry.root, rel: attachmentRel, size: attachmentInfo.Size()},
		})
	}
	return NewBundleSource(primary, attachments)
}

// ValidateWrite checks root permissions, path confinement, and any existing
// destination without creating directories or files.
func (r *RootSet) ValidateWrite(id, relativePath string) error {
	entry, ok := r.root(id)
	if !ok {
		return opError("validate rooted output", CodeNotFound, fmt.Errorf("unknown root ID %q", id))
	}
	if !entry.writable {
		return opError("validate rooted output", CodePermissionDenied, fmt.Errorf("root ID %q is read-only", id))
	}
	rel, err := normalizeRelativePath(relativePath)
	if err != nil {
		return opError("validate rooted output", CodeInvalidArgument, err)
	}
	if info, err := entry.root.Lstat(rel); err == nil {
		if !info.Mode().IsRegular() {
			return opError("validate rooted output", CodeInvalidArgument, fmt.Errorf("output %q is not a regular file", relativePath))
		}
	} else if !os.IsNotExist(err) {
		return opError("validate rooted output", CodeInvalidArgument, fmt.Errorf("inspect output: %w", err))
	}
	return nil
}

// WriteFile builds a regular file in a same-directory temporary path before
// installing it beneath a configured root, so a partial file is never exposed.
// The returned digest covers exactly the bytes installed at relativePath.
func (r *RootSet) WriteFile(ctx context.Context, id, relativePath string, reader io.Reader, maxBytes int64) (int64, string, error) {
	if reader == nil || maxBytes <= 0 {
		return 0, "", opError("write rooted file", CodeInvalidArgument, fmt.Errorf("reader and a positive size limit are required"))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, "", opError("write rooted file", CodeCanceled, err)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if err := r.ValidateWrite(id, relativePath); err != nil {
		return 0, "", err
	}
	entry, ok := r.root(id)
	if !ok {
		return 0, "", opError("write rooted file", CodeNotFound, fmt.Errorf("unknown root ID %q", id))
	}
	root := entry.root
	rel, err := normalizeRelativePath(relativePath)
	if err != nil {
		return 0, "", opError("write rooted file", CodeInvalidArgument, err)
	}
	parent := filepath.Dir(rel)
	if parent != "." {
		if err := root.MkdirAll(parent, 0755); err != nil {
			return 0, "", opError("write rooted file", CodeInvalidArgument, fmt.Errorf("create output directory: %w", err))
		}
	}
	tempRel, err := rootedTempName(parent)
	if err != nil {
		return 0, "", opError("write rooted file", CodeInternal, err)
	}
	temp, err := root.OpenFile(tempRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return 0, "", opError("write rooted file", CodeInternal, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = temp.Close()
			_ = root.Remove(tempRel)
		}
	}()
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(&contextReader{ctx: ctx, reader: reader}, limitWithSentinel(maxBytes)))
	if copyErr != nil {
		return 0, "", opError("write rooted file", CodeInternal, copyErr)
	}
	if written > maxBytes {
		return 0, "", opError("write rooted file", CodeResourceExhausted, fmt.Errorf("output exceeds limit %d", maxBytes))
	}
	if err := ctx.Err(); err != nil {
		return 0, "", opError("write rooted file", CodeCanceled, err)
	}
	if err := temp.Sync(); err != nil {
		return 0, "", opError("write rooted file", CodeInternal, fmt.Errorf("sync output: %w", err))
	}
	if err := temp.Close(); err != nil {
		return 0, "", opError("write rooted file", CodeInternal, err)
	}
	if err := ctx.Err(); err != nil {
		return 0, "", opError("write rooted file", CodeCanceled, err)
	}
	if info, err := root.Lstat(rel); err == nil {
		if !info.Mode().IsRegular() {
			return 0, "", opError("write rooted file", CodeInvalidArgument, fmt.Errorf("output %q is not a regular file", relativePath))
		}
	} else if !os.IsNotExist(err) {
		return 0, "", opError("write rooted file", CodeInvalidArgument, fmt.Errorf("inspect output: %w", err))
	}
	if err := root.Rename(tempRel, rel); err != nil {
		return 0, "", opError("write rooted file", CodeInternal, fmt.Errorf("commit output: %w", err))
	}
	committed = true
	return written, hex.EncodeToString(hash.Sum(nil)), nil
}

// BundleFile is one file in a rooted artifact. The primary file uses an empty
// suffix; companion files use one of ArtifactAttachmentSuffixes.
type BundleFile struct {
	Suffix         string
	Reader         io.Reader
	ExpectedSize   *int64
	ExpectedSHA256 string
}

type BundleFileMetadata struct {
	Suffix string
	Size   int64
	SHA256 string
}

// WriteBundle stages and verifies every file before installing any of them.
// Managed sidecars omitted from files are removed, so an older sidecar cannot
// be accidentally paired with a new primary file. Existing destinations are
// backed up and restored if any pre-commit rename fails. Once every staged file
// is installed, backup removal is best-effort and cannot turn the committed
// installation into a reported failure.
func (r *RootSet) WriteBundle(ctx context.Context, id, relativePath string, files []BundleFile, maxBytes int64) ([]BundleFileMetadata, error) {
	if len(files) == 0 || maxBytes <= 0 {
		return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("at least one file and a positive size limit are required"))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, opError("write rooted bundle", CodeCanceled, err)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	entry, ok := r.root(id)
	if !ok {
		return nil, opError("write rooted bundle", CodeNotFound, fmt.Errorf("unknown root ID %q", id))
	}
	if !entry.writable {
		return nil, opError("write rooted bundle", CodePermissionDenied, fmt.Errorf("root ID %q is read-only", id))
	}
	rel, err := normalizeRelativePath(relativePath)
	if err != nil {
		return nil, opError("write rooted bundle", CodeInvalidArgument, err)
	}

	type preparedFile struct {
		suffix          string
		target          string
		temp            string
		reader          io.Reader
		expectedSize    int64
		hasExpectedSize bool
		expectedDigest  string
		size            int64
		digest          string
		staged          bool
	}
	prepared := make([]preparedFile, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	hasPrimary := false
	for i, file := range files {
		if file.Reader == nil {
			return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("file %d has no reader", i))
		}
		suffix := file.Suffix
		if suffix == "" {
			if hasPrimary {
				return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("bundle contains multiple primary files"))
			}
			hasPrimary = true
		} else {
			suffix, err = normalizeAttachmentSuffix(suffix)
			if err != nil {
				return nil, opError("write rooted bundle", CodeInvalidArgument, err)
			}
		}
		if _, duplicate := seen[suffix]; duplicate {
			return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("duplicate bundle suffix %q", suffix))
		}
		if file.ExpectedSize != nil && *file.ExpectedSize < 0 {
			return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("bundle suffix %q has negative expected size", suffix))
		}
		expectedDigest := strings.ToLower(strings.TrimSpace(file.ExpectedSHA256))
		if expectedDigest != "" {
			decoded, decodeErr := hex.DecodeString(expectedDigest)
			if decodeErr != nil || len(decoded) != sha256.Size {
				return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("bundle suffix %q has invalid expected SHA-256", suffix))
			}
		}
		seen[suffix] = struct{}{}
		preparedFile := preparedFile{suffix: suffix, target: rel + suffix, reader: file.Reader, expectedDigest: expectedDigest}
		if file.ExpectedSize != nil {
			preparedFile.expectedSize = *file.ExpectedSize
			preparedFile.hasExpectedSize = true
		}
		prepared = append(prepared, preparedFile)
	}
	if !hasPrimary {
		return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("bundle primary file is required"))
	}

	managedTargets := make([]string, 0, len(artifactAttachmentSuffixes)+1)
	for _, suffix := range artifactAttachmentSuffixes {
		managedTargets = append(managedTargets, rel+suffix)
	}
	managedTargets = append(managedTargets, rel)
	for _, target := range managedTargets {
		if info, statErr := entry.root.Lstat(target); statErr == nil {
			if !info.Mode().IsRegular() {
				return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("output %q is not a regular file", filepath.ToSlash(target)))
			}
		} else if !os.IsNotExist(statErr) {
			return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("inspect output: %w", statErr))
		}
	}

	parent := filepath.Dir(rel)
	if parent != "." {
		if err := entry.root.MkdirAll(parent, 0755); err != nil {
			return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("create output directory: %w", err))
		}
	}
	cleanupStaged := func() {
		for i := range prepared {
			if prepared[i].staged {
				_ = entry.root.Remove(prepared[i].temp)
			}
		}
	}
	defer cleanupStaged()

	var total int64
	for i := range prepared {
		tempRel, tempErr := rootedTempName(parent)
		if tempErr != nil {
			return nil, opError("write rooted bundle", CodeInternal, tempErr)
		}
		output, openErr := entry.root.OpenFile(tempRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if openErr != nil {
			return nil, opError("write rooted bundle", CodeInternal, openErr)
		}
		hash := sha256.New()
		remaining := maxBytes - total
		written, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(&contextReader{ctx: ctx, reader: prepared[i].reader}, limitWithSentinel(remaining)))
		syncErr := output.Sync()
		closeErr := output.Close()
		if copyErr != nil {
			_ = entry.root.Remove(tempRel)
			return nil, opError("write rooted bundle", CodeInternal, copyErr)
		}
		if written > remaining {
			_ = entry.root.Remove(tempRel)
			return nil, opError("write rooted bundle", CodeResourceExhausted, fmt.Errorf("bundle exceeds limit %d", maxBytes))
		}
		if syncErr != nil {
			_ = entry.root.Remove(tempRel)
			return nil, opError("write rooted bundle", CodeInternal, fmt.Errorf("sync output: %w", syncErr))
		}
		if closeErr != nil {
			_ = entry.root.Remove(tempRel)
			return nil, opError("write rooted bundle", CodeInternal, closeErr)
		}
		prepared[i].temp = tempRel
		prepared[i].size = written
		prepared[i].digest = hex.EncodeToString(hash.Sum(nil))
		prepared[i].staged = true
		if prepared[i].hasExpectedSize && prepared[i].size != prepared[i].expectedSize {
			return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("bundle suffix %q size changed while staging: got %d, expected %d", prepared[i].suffix, prepared[i].size, prepared[i].expectedSize))
		}
		if prepared[i].expectedDigest != "" && prepared[i].digest != prepared[i].expectedDigest {
			return nil, opError("write rooted bundle", CodeInvalidArgument, fmt.Errorf("bundle suffix %q SHA-256 changed while staging", prepared[i].suffix))
		}
		total += written
	}
	if err := ctx.Err(); err != nil {
		return nil, opError("write rooted bundle", CodeCanceled, err)
	}

	backups := make(map[string]string)
	installed := make([]string, 0, len(prepared))
	rollback := func(cause error) error {
		var rollbackErr error
		for i := len(installed) - 1; i >= 0; i-- {
			if removeErr := entry.root.Remove(installed[i]); removeErr != nil && !os.IsNotExist(removeErr) {
				rollbackErr = errors.Join(rollbackErr, removeErr)
			}
		}
		for i := len(managedTargets) - 1; i >= 0; i-- {
			target := managedTargets[i]
			if backup := backups[target]; backup != "" {
				if restoreErr := entry.root.Rename(backup, target); restoreErr != nil {
					rollbackErr = errors.Join(rollbackErr, restoreErr)
				}
			}
		}
		return errors.Join(cause, rollbackErr)
	}
	for _, target := range managedTargets {
		if _, statErr := entry.root.Lstat(target); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			return nil, opError("write rooted bundle", CodeInternal, rollback(statErr))
		}
		backup, backupErr := rootedTempName(filepath.Dir(target))
		if backupErr != nil {
			return nil, opError("write rooted bundle", CodeInternal, rollback(backupErr))
		}
		if renameErr := entry.root.Rename(target, backup); renameErr != nil {
			return nil, opError("write rooted bundle", CodeInternal, rollback(renameErr))
		}
		backups[target] = backup
	}
	sort.SliceStable(prepared, func(i, j int) bool {
		return prepared[i].suffix != "" && prepared[j].suffix == ""
	})
	for i := range prepared {
		if err := ctx.Err(); err != nil {
			return nil, opError("write rooted bundle", CodeCanceled, rollback(err))
		}
		if renameErr := entry.root.Rename(prepared[i].temp, prepared[i].target); renameErr != nil {
			return nil, opError("write rooted bundle", CodeInternal, rollback(renameErr))
		}
		prepared[i].staged = false
		installed = append(installed, prepared[i].target)
	}
	// All new files are installed at this point. Backup cleanup happens after
	// the commit boundary; a removal failure leaves a recoverable temporary
	// backup but must not report that the installed bundle was rolled back.
	for _, backup := range backups {
		_ = entry.root.Remove(backup)
	}

	metadata := make([]BundleFileMetadata, 0, len(prepared))
	for _, file := range prepared {
		metadata = append(metadata, BundleFileMetadata{Suffix: file.suffix, Size: file.size, SHA256: file.digest})
	}
	return metadata, nil
}

func (r *RootSet) root(id string) (rootEntry, bool) {
	r.mu.RLock()
	root, ok := r.roots[id]
	r.mu.RUnlock()
	return root, ok
}

// Close releases the directory handles retained by the root set. Servers must
// stop accepting requests before calling Close.
func (r *RootSet) Close() error {
	r.mu.Lock()
	roots := r.roots
	r.roots = nil
	r.mu.Unlock()
	var result error
	for _, entry := range roots {
		result = errors.Join(result, entry.root.Close())
	}
	return result
}

func rootedTempName(parent string) (string, error) {
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	name := ".meido-write-" + hex.EncodeToString(random[:]) + ".tmp"
	if parent == "." {
		return name, nil
	}
	return filepath.Join(parent, name), nil
}

type rootSource struct {
	name string
	root *os.Root
	rel  string
	size int64
}

func (s *rootSource) Name() string { return s.name }
func (s *rootSource) Size() int64  { return s.size }
func (s *rootSource) Open(ctx context.Context) (ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.root.Open(s.rel)
}

// normalizeRelativePath 将外部路径规范化为当前平台的受限相对路径并拒绝遍历与卷限定路径
// normalizeRelativePath normalizes an external path to a confined relative path for the current platform and rejects traversal and volume-qualified paths
func normalizeRelativePath(name string) (string, error) {
	if name == "" || strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("relative path is empty or contains NUL")
	}
	portable := strings.ReplaceAll(name, `\`, "/")
	hasWindowsDrive := len(portable) >= 2 && portable[1] == ':' &&
		((portable[0] >= 'A' && portable[0] <= 'Z') || (portable[0] >= 'a' && portable[0] <= 'z'))
	if strings.HasPrefix(portable, "/") || hasWindowsDrive || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("absolute path %q is not allowed", name)
	}
	parts := strings.Split(portable, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("unsafe path component %q in %q", part, name)
		}
	}
	rel := filepath.Join(parts...)
	if rel == "." || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("path %q is not relative", name)
	}
	return rel, nil
}

func cleanSourceName(name string) string {
	if strings.IndexByte(name, 0) >= 0 {
		return "input.bin"
	}
	name = strings.ReplaceAll(name, `\`, "/")
	name = filepath.Base(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "input.bin"
	}
	return name
}

func limitWithSentinel(limit int64) int64 {
	if limit == math.MaxInt64 {
		return limit
	}
	return limit + 1
}
