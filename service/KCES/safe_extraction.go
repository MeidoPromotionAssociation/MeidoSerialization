package KCES

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// extractionRoot 使用 os.Root 将归档提取限制在一个目录内并抵御并发替换的链接或重解析点 / extractionRoot uses os.Root to confine archive extraction to one directory and resist concurrently replaced links or reparse points
type extractionRoot struct {
	absPath string
	root    *os.Root
}

// normalizeExtractionPath 将归档路径转换为可移植的相对文件系统路径，并在所有平台识别两种分隔符
// normalizeExtractionPath converts an archive path into a portable relative filesystem path while recognizing both separators on every platform
func normalizeExtractionPath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty extraction path")
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("extraction path is not valid UTF-8")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("extraction path contains NUL")
	}

	portable := strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(portable, "/") || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("absolute extraction path %q is not allowed", name)
	}

	parts := strings.Split(portable, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("unsafe extraction path component %q in %q", part, name)
		}
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", fmt.Errorf("Windows-ambiguous extraction path component %q", part)
		}
		for _, r := range part {
			if r < 0x20 || strings.ContainsRune(`<>:"|?*`, r) {
				return "", fmt.Errorf("invalid extraction path character %q in %q", r, name)
			}
		}
		if isWindowsReservedName(part) {
			return "", fmt.Errorf("Windows reserved extraction path component %q", part)
		}
	}

	rel := filepath.Join(parts...)
	if rel == "." || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("extraction path %q is not relative", name)
	}
	return rel, nil
}

// isWindowsReservedName 判断文件名首段是否为 Windows 保留设备名
// isWindowsReservedName reports whether the leading file-name segment is a Windows reserved device name
func isWindowsReservedName(name string) bool {
	base := name
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	base = strings.ToUpper(base)
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(base) == 4 {
		prefix := base[:3]
		return (prefix == "COM" || prefix == "LPT") && base[3] >= '1' && base[3] <= '9'
	}
	return false
}

// openExtractionRoot 创建或打开真实输出目录并确认打开前后的文件身份一致
// openExtractionRoot creates or opens a real output directory and verifies that its file identity remains unchanged while opening
func openExtractionRoot(path string) (*extractionRoot, error) {
	if path == "" {
		return nil, fmt.Errorf("empty output directory")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory %q: %w", path, err)
	}
	absPath = filepath.Clean(absPath)

	info, err := os.Lstat(absPath)
	switch {
	case err == nil:
		if isLinkOrReparse(info) {
			return nil, fmt.Errorf("output directory %q is a symlink or reparse point", absPath)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("output path %q is not a directory", absPath)
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return nil, fmt.Errorf("create output directory %q: %w", absPath, err)
		}
		info, err = os.Lstat(absPath)
		if err != nil {
			return nil, fmt.Errorf("inspect output directory %q: %w", absPath, err)
		}
		if isLinkOrReparse(info) || !info.IsDir() {
			return nil, fmt.Errorf("created output path %q is not a real directory", absPath)
		}
	case err != nil:
		return nil, fmt.Errorf("inspect output directory %q: %w", absPath, err)
	}

	root, err := os.OpenRoot(absPath)
	if err != nil {
		return nil, fmt.Errorf("open output root %q: %w", absPath, err)
	}
	rootInfo, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("inspect opened output root %q: %w", absPath, err)
	}
	if !os.SameFile(info, rootInfo) {
		root.Close()
		return nil, fmt.Errorf("output directory %q changed while it was being opened", absPath)
	}
	return &extractionRoot{absPath: absPath, root: root}, nil
}

// Close 关闭安全提取根目录
// Close closes the safe extraction root
func (r *extractionRoot) Close() error {
	if r == nil || r.root == nil {
		return nil
	}
	return r.root.Close()
}

// resolve 校验归档名称并解析根目录内的相对路径和绝对路径
// resolve validates an archive name and resolves its relative and absolute paths inside the root
func (r *extractionRoot) resolve(name string) (rel string, abs string, err error) {
	if r == nil || r.root == nil {
		return "", "", fmt.Errorf("nil or closed extraction root")
	}
	rel, err = normalizeExtractionPath(name)
	if err != nil {
		return "", "", err
	}
	abs = filepath.Clean(filepath.Join(r.absPath, rel))
	check, err := filepath.Rel(r.absPath, abs)
	if err != nil {
		return "", "", fmt.Errorf("check extraction path %q: %w", name, err)
	}
	if check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) || filepath.IsAbs(check) {
		return "", "", fmt.Errorf("extraction path %q escapes output root %q", name, r.absPath)
	}
	return rel, abs, nil
}

// ensureParent 逐级创建并校验目标文件的真实父目录
// ensureParent creates and validates each real parent directory of a destination file
func (r *extractionRoot) ensureParent(rel string) error {
	parent := filepath.Dir(rel)
	if parent == "." {
		return nil
	}
	var current string
	for _, part := range strings.Split(parent, string(filepath.Separator)) {
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}

		info, err := r.root.Lstat(current)
		if os.IsNotExist(err) {
			if err := r.root.Mkdir(current, 0755); err != nil && !os.IsExist(err) {
				return fmt.Errorf("create extraction directory %q: %w", current, err)
			}
			info, err = r.root.Lstat(current)
		}
		if err != nil {
			return fmt.Errorf("inspect extraction directory %q: %w", current, err)
		}
		if isLinkOrReparse(info) {
			return fmt.Errorf("refusing symlink or reparse point in output path %q", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("output path component %q is not a directory", current)
		}
	}
	return nil
}

// rejectLinkedTarget 拒绝现有链接、重解析点或非普通文件目标
// rejectLinkedTarget rejects an existing link, reparse point, or non-regular destination
func (r *extractionRoot) rejectLinkedTarget(rel string) error {
	info, err := r.root.Lstat(rel)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect output file %q: %w", rel, err)
	}
	if isLinkOrReparse(info) {
		return fmt.Errorf("refusing symlink or reparse point output file %q", rel)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("output file %q is not a regular file", rel)
	}
	return nil
}

// newTempFile 在目标父目录中创建不可预测且排他打开的临时文件
// newTempFile creates an unpredictable exclusively opened temporary file in the destination parent directory
func (r *extractionRoot) newTempFile(rel string, perm fs.FileMode) (*os.File, string, error) {
	parent := filepath.Dir(rel)
	for attempt := 0; attempt < 20; attempt++ {
		var random [12]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", fmt.Errorf("generate temporary output name: %w", err)
		}
		name := ".meido-extract-" + hex.EncodeToString(random[:]) + ".tmp"
		tempRel := name
		if parent != "." {
			tempRel = filepath.Join(parent, name)
		}
		f, err := r.root.OpenFile(tempRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return nil, "", fmt.Errorf("create temporary output for %q: %w", rel, err)
		}
		return f, tempRel, nil
	}
	return nil, "", fmt.Errorf("could not allocate a temporary output for %q", rel)
}

// commitTempFile 校验临时文件和目标后以根目录内重命名原子提交输出
// commitTempFile validates the temporary file and destination before atomically committing the output through a root-relative rename
func (r *extractionRoot) commitTempFile(tempRel, rel string) error {
	info, err := r.root.Lstat(tempRel)
	if err != nil {
		return fmt.Errorf("inspect temporary output %q: %w", tempRel, err)
	}
	if isLinkOrReparse(info) || !info.Mode().IsRegular() {
		return fmt.Errorf("temporary output %q is not a regular file", tempRel)
	}
	if err := r.rejectLinkedTarget(rel); err != nil {
		return err
	}
	if err := r.root.Rename(tempRel, rel); err != nil {
		return fmt.Errorf("commit output file %q: %w", rel, err)
	}
	return nil
}

// WriteFile 在安全提取根目录内原子写入完整内存数据
// WriteFile atomically writes complete in-memory data inside the safe extraction root
func (r *extractionRoot) WriteFile(name string, data []byte, perm fs.FileMode) (err error) {
	rel, _, err := r.resolve(name)
	if err != nil {
		return err
	}
	if err := r.ensureParent(rel); err != nil {
		return err
	}
	if err := r.rejectLinkedTarget(rel); err != nil {
		return err
	}

	f, tempRel, err := r.newTempFile(rel, perm)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = r.root.Remove(tempRel)
		}
	}()
	n, err := f.Write(data)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("write temporary output for %q: %w", rel, err)
	}
	if n != len(data) {
		_ = f.Close()
		return fmt.Errorf("short write to temporary output for %q: wrote %d of %d bytes", rel, n, len(data))
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temporary output for %q: %w", rel, err)
	}
	if err := r.commitTempFile(tempRel, rel); err != nil {
		return err
	}
	committed = true
	return nil
}

// WriteFileStream 在安全提取根目录内原子提交由回调增量写入的文件，避免将大型 UnityFS 数据完整载入内存
// WriteFileStream atomically commits a file written incrementally by a callback inside the safe extraction root without materializing large UnityFS data in memory
func (r *extractionRoot) WriteFileStream(name string, perm fs.FileMode, write func(*os.File) error) (err error) {
	if write == nil {
		return fmt.Errorf("nil streaming output writer")
	}
	rel, _, err := r.resolve(name)
	if err != nil {
		return err
	}
	if err := r.ensureParent(rel); err != nil {
		return err
	}
	if err := r.rejectLinkedTarget(rel); err != nil {
		return err
	}

	f, tempRel, err := r.newTempFile(rel, perm)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = f.Close()
			_ = r.root.Remove(tempRel)
		}
	}()
	if err := write(f); err != nil {
		return fmt.Errorf("write temporary output for %q: %w", rel, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temporary output for %q: %w", rel, err)
	}
	if err := r.commitTempFile(tempRel, rel); err != nil {
		return err
	}
	committed = true
	return nil
}
