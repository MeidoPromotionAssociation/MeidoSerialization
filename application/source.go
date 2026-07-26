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

// ReadSeekCloser 定义应用层输入需要的读取、定位和关闭能力 / ReadSeekCloser defines the read, seek, and close capabilities required for application input
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// Source 表示不限定底层存储方式的具名可定位制品 / Source represents a named seekable artifact without prescribing its storage
type Source interface {
	// Name 返回适合物化输入时使用的文件名
	// Name returns the filename to use when materializing the source
	Name() string
	// Size 返回源内容的精确字节数
	// Size returns the exact source content size in bytes
	Size() int64
	// Open 打开受上下文约束的独立可定位读取器
	// Open opens an independent seekable reader governed by a context
	Open(context.Context) (ReadSeekCloser, error)
}

// SourceAttachment 表示通过后缀关联到主要输入的伴随文件 / SourceAttachment represents a companion file associated with a primary source by suffix
type SourceAttachment struct {
	// Suffix 是追加到主要源文件名后的受管理后缀 / Suffix is the managed suffix appended to the primary source name
	Suffix string
	// Source 提供伴随文件的内容和元数据 / Source provides the companion file content and metadata
	Source Source
}

var artifactAttachmentSuffixes = []string{".meta.json", ".typetree.json"}

// ArtifactAttachmentSuffixes 返回作为单个应用制品管理的伴随文件后缀副本
// ArtifactAttachmentSuffixes returns a copy of companion suffixes managed as part of one application artifact
func ArtifactAttachmentSuffixes() []string {
	return append([]string(nil), artifactAttachmentSuffixes...)
}

// attachmentSource 定义能够公开伴随输入文件的源 / attachmentSource defines a source capable of exposing companion input files
type attachmentSource interface {
	// Attachments 返回与主要输入关联的伴随文件
	// Attachments returns companion files associated with the primary input
	Attachments() []SourceAttachment
}

// bundleSource 将主要输入源与一组受管理伴随文件组合 / bundleSource combines a primary input source with managed companion files
type bundleSource struct {
	// Source 是组合制品的主要输入 / Source is the primary input of the bundled artifact
	Source
	// attachments 保存已规范化且后缀唯一的伴随文件 / attachments stores companion files with normalized unique suffixes
	attachments []SourceAttachment
}

// Attachments 返回组合源所含伴随文件的浅拷贝
// Attachments returns a shallow copy of companion files contained by the bundled source
func (s *bundleSource) Attachments() []SourceAttachment {
	result := make([]SourceAttachment, len(s.attachments))
	copy(result, s.attachments)
	return result
}

// NewBundleSource 将受支持的伴随文件关联到主要源并拒绝重复后缀
// NewBundleSource associates supported companion files with a primary source and rejects duplicate suffixes
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

// sourceAttachments 返回源通过可选伴随文件接口公开的文件列表
// sourceAttachments returns files exposed by a source through the optional companion interface
func sourceAttachments(source Source) []SourceAttachment {
	provider, ok := source.(attachmentSource)
	if !ok {
		return nil
	}
	return provider.Attachments()
}

// normalizeAttachmentSuffix 校验并规范化受管理的伴随文件后缀
// normalizeAttachmentSuffix validates and normalizes a managed companion-file suffix
func normalizeAttachmentSuffix(value string) (string, error) {
	suffix := strings.ToLower(strings.TrimSpace(value))
	for _, supported := range artifactAttachmentSuffixes {
		if suffix == supported {
			return supported, nil
		}
	}
	return "", fmt.Errorf("unsupported artifact attachment suffix %q", value)
}

// bytesSource 保存不可变的内存输入及其安全文件名 / bytesSource stores immutable in-memory input and its safe filename
type bytesSource struct {
	// name 是物化内存内容时使用的文件名 / name is the filename used when materializing the in-memory content
	name string
	// data 是创建源时复制的不可变字节内容 / data is immutable byte content copied when the source is created
	data []byte
}

// NewBytesSource 创建复制输入内容的不可变内存源
// NewBytesSource creates an immutable in-memory source by copying the input content
func NewBytesSource(name string, data []byte) Source {
	return &bytesSource{name: cleanSourceName(name), data: append([]byte(nil), data...)}
}

// Name 返回内存源的安全文件名
// Name returns the safe filename of the in-memory source
func (s *bytesSource) Name() string { return s.name }

// Size 返回内存源的精确字节数
// Size returns the exact size in bytes of the in-memory source
func (s *bytesSource) Size() int64 { return int64(len(s.data)) }

// Open 返回读取内存源内容的新可定位读取器
// Open returns a new seekable reader for the in-memory source content
func (s *bytesSource) Open(ctx context.Context) (ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &readSeekNopCloser{Reader: bytes.NewReader(s.data)}, nil
}

// readSeekNopCloser 为内存读取器补充无需操作的关闭方法 / readSeekNopCloser adds a no-op close method to an in-memory reader
type readSeekNopCloser struct {
	// Reader 提供实际的内存读取和定位能力 / Reader provides the underlying in-memory read and seek capabilities
	*bytes.Reader
}

// Close 完成无需释放资源的内存读取器关闭操作
// Close completes closing an in-memory reader that owns no releasable resources
func (r *readSeekNopCloser) Close() error { return nil }

// fileSource 保存常规本地文件的稳定名称、绝对路径和已观察大小 / fileSource stores a regular local file's stable name, absolute path, and observed size
type fileSource struct {
	// name 是不包含目录的本地文件名 / name is the local filename without directory components
	name string
	// path 是本地文件的绝对路径 / path is the absolute path of the local file
	path string
	// size 是创建源时观察到的精确文件字节数 / size is the exact file size in bytes observed when the source was created
	size int64
}

// NewFileSource 为常规本地文件及其受管理伴随文件创建输入源
// NewFileSource creates an input source for a regular local file and its managed companions
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

// Name 返回本地文件源的基本文件名
// Name returns the base filename of the local file source
func (s *fileSource) Name() string { return s.name }

// Size 返回创建本地文件源时记录的精确字节数
// Size returns the exact byte size recorded when the local file source was created
func (s *fileSource) Size() int64 { return s.size }

// Open 打开本地文件源并在操作前检查上下文
// Open opens the local file source after checking the context
func (s *fileSource) Open(ctx context.Context) (ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.Open(s.path)
}

// discoverFileAttachments 查找主要本地文件旁存在的受管理常规伴随文件
// discoverFileAttachments finds managed regular companion files beside a primary local file
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

// RootSet 将公开根标识符映射到服务端配置的受限目录 / RootSet maps public root identifiers to server-configured confined directories
type RootSet struct {
	// mu 保护根目录映射及关闭状态 / mu protects the root map and closed state
	mu sync.RWMutex
	// writeMu 串行化多文件提交以保持写入和回滚一致性 / writeMu serializes multi-file commits to preserve write and rollback consistency
	writeMu sync.Mutex
	// roots 保存已打开根目录句柄及其写权限 / roots stores open root handles and their write permissions
	roots map[string]rootEntry
}

// rootEntry 保存一个受限目录句柄及其写权限 / rootEntry stores one confined directory handle and its write permission
type rootEntry struct {
	// root 是限制文件访问范围的目录句柄 / root is the directory handle that confines file access
	root *os.Root
	// writable 表示是否允许在该根目录下安装输出 / writable reports whether outputs may be installed beneath the root
	writable bool
}

// NewRootSet 创建空的开放根目录集合
// NewRootSet creates an empty open root set
func NewRootSet() *RootSet { return &RootSet{roots: make(map[string]rootEntry)} }

// Add 注册只读受限根目录
// Add registers a read-only confined root directory
func (r *RootSet) Add(id, directory string) error {
	return r.add(id, directory, false)
}

// AddWritable 注册可读取和写入的受限根目录
// AddWritable registers a confined root directory that permits reads and writes
func (r *RootSet) AddWritable(id, directory string) error {
	return r.add(id, directory, true)
}

// add 校验并打开根目录后以指定写权限注册
// add validates and opens a root directory before registering it with the requested write permission
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

// IDs 返回按字典序排列的全部已注册根标识符
// IDs returns all registered root identifiers in lexical order
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

// WritableIDs 返回按字典序排列的可写根标识符
// WritableIDs returns writable root identifiers in lexical order
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

// Resolve 在指定受限根目录下解析常规文件及其受管理伴随文件
// Resolve resolves a regular file and its managed companions beneath a confined root
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

// ValidateWrite 在不创建目录或文件的情况下检查根权限、路径限制和现有目标
// ValidateWrite checks root permissions, path confinement, and an existing destination without creating directories or files
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

// WriteFile 在同目录临时路径完整写入常规文件后将其原子安装到配置根目录
// WriteFile fully writes a regular file to a same-directory temporary path before atomically installing it beneath a configured root
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

// BundleFile 描述受限根目录制品中待安装的单个主要文件或伴随文件 / BundleFile describes one primary or companion file to install as part of a rooted artifact
type BundleFile struct {
	// Suffix 为空时表示主要文件，否则必须是受管理伴随文件后缀 / Suffix identifies the primary file when empty or a managed companion suffix otherwise
	Suffix string
	// Reader 提供待安装文件的内容 / Reader supplies the content of the file to install
	Reader io.Reader
	// ExpectedSize 是提交前可选校验的精确字节数 / ExpectedSize is an optional exact byte size verified before commit
	ExpectedSize *int64
	// ExpectedSHA256 是提交前可选校验的十六进制 SHA-256 摘要 / ExpectedSHA256 is an optional hexadecimal SHA-256 digest verified before commit
	ExpectedSHA256 string
}

// BundleFileMetadata 描述成功安装的主要文件或伴随文件 / BundleFileMetadata describes a successfully installed primary or companion file
type BundleFileMetadata struct {
	// Suffix 标识主要文件或受管理伴随文件 / Suffix identifies the primary file or managed companion file
	Suffix string
	// Size 是已安装文件的精确字节数 / Size is the exact installed file size in bytes
	Size int64
	// SHA256 是已安装文件内容的十六进制 SHA-256 摘要 / SHA256 is the hexadecimal SHA-256 digest of the installed file content
	SHA256 string
}

// WriteBundle 在安装任何文件前暂存并校验完整制品集合且在提交失败时恢复原文件
// WriteBundle stages and verifies the complete artifact set before installation and restores original files if commit fails
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

	// preparedFile 保存单个制品文件从请求校验到提交完成的暂存状态 / preparedFile stores staging state for one artifact file from request validation through commit
	type preparedFile struct {
		// suffix 标识主要文件或受管理伴随文件 / suffix identifies the primary file or managed companion file
		suffix string
		// target 是受限根目录下的最终相对路径 / target is the final relative path beneath the confined root
		target string
		// temp 是受限根目录下的暂存相对路径 / temp is the staging relative path beneath the confined root
		temp string
		// reader 提供请求中的文件内容 / reader supplies file content from the request
		reader io.Reader
		// expectedSize 是请求声明的精确文件字节数 / expectedSize is the exact file size declared by the request
		expectedSize int64
		// hasExpectedSize 表示是否需要校验请求声明的文件大小 / hasExpectedSize reports whether the requested file size must be verified
		hasExpectedSize bool
		// expectedDigest 是规范化后的请求 SHA-256 摘要 / expectedDigest is the normalized SHA-256 digest supplied by the request
		expectedDigest string
		// size 是暂存期间实际写入的精确字节数 / size is the exact number of bytes written during staging
		size int64
		// digest 是暂存内容计算得到的 SHA-256 摘要 / digest is the SHA-256 digest computed from staged content
		digest string
		// staged 表示临时文件仍需在延迟清理时删除 / staged reports whether deferred cleanup must still remove the temporary file
		staged bool
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

// root 在并发读取保护下查找已注册根目录条目
// root looks up a registered root entry under concurrent read protection
func (r *RootSet) root(id string) (rootEntry, bool) {
	r.mu.RLock()
	root, ok := r.roots[id]
	r.mu.RUnlock()
	return root, ok
}

// Close 释放根目录集合保留的目录句柄并阻止后续注册
// Close releases directory handles retained by the root set and prevents further registration
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

// rootedTempName 为指定相对父目录生成随机隐藏临时文件名
// rootedTempName generates a random hidden temporary filename beneath a relative parent directory
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

// rootSource 表示通过受限根目录句柄访问的常规文件 / rootSource represents a regular file accessed through a confined root handle
type rootSource struct {
	// name 是不包含目录的源文件名 / name is the source filename without directory components
	name string
	// root 是限制文件访问范围的共享目录句柄 / root is the shared directory handle that confines file access
	root *os.Root
	// rel 是相对于受限根目录的已规范化路径 / rel is the normalized path relative to the confined root
	rel string
	// size 是解析源时观察到的精确文件字节数 / size is the exact file size in bytes observed when resolving the source
	size int64
}

// Name 返回受限根目录源的基本文件名
// Name returns the base filename of the confined root source
func (s *rootSource) Name() string { return s.name }

// Size 返回解析受限根目录源时记录的精确字节数
// Size returns the exact byte size recorded when the confined root source was resolved
func (s *rootSource) Size() int64 { return s.size }

// Open 通过受限根目录句柄打开源并在操作前检查上下文
// Open opens the source through its confined root handle after checking the context
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

// cleanSourceName 将不可信源名称限制为安全的基本文件名
// cleanSourceName confines an untrusted source name to a safe base filename
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

// limitWithSentinel 返回比限制多一个字节的读取上限并避免精确整数溢出
// limitWithSentinel returns a read bound one byte beyond the limit while avoiding exact integer overflow
func limitWithSentinel(limit int64) int64 {
	if limit == math.MaxInt64 {
		return limit
	}
	return limit + 1
}
