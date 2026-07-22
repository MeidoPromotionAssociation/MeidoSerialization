package arc

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// UTF8Hash 使用 UTF-8 编码和不区分大小写的比较方式计算目录名哈希
// UTF8Hash computes a hash for the directory name using UTF-8 encoding and case-insensitive comparison
func (d *Dir) UTF8Hash() uint64 { return NameHashUTF8(d.Name) }

// UTF16Hash 使用 UTF-16LE 编码和不区分大小写的比较方式计算目录名哈希
// UTF16Hash computes a hash for the directory name using UTF-16LE encoding and case-insensitive comparison
func (d *Dir) UTF16Hash() uint64 { return NameHashUTF16(d.Name) }

// UniqueID 使用 UTF-16LE 编码根据目录完整路径计算唯一标识
// UniqueID computes a unique identifier for the directory based on its full path using UTF-16LE encoding
func (d *Dir) UniqueID() uint64 { return UniqueIDHash(d.FullName()) }

// UTF8Hash 使用 UTF-8 编码和不区分大小写的比较方式计算并返回文件名哈希
// UTF8Hash computes and returns a hash value for the file name using UTF-8 encoding and case-insensitive comparison
func (f *File) UTF8Hash() uint64 { return NameHashUTF8(f.Name) }

// UTF16Hash 使用 UTF-16 编码和不区分大小写的比较方式计算并返回文件名哈希
// UTF16Hash computes and returns a hash value for the file name using UTF-16 encoding and case-insensitive comparison
func (f *File) UTF16Hash() uint64 { return NameHashUTF16(f.Name) }

// UniqueID 根据不转换为小写的 UTF-16LE 文件完整路径计算并返回唯一标识
// UniqueID computes and returns a unique identifier for the file based on its full path encoded as UTF-16LE without lowercasing
func (f *File) UniqueID() uint64 { return UniqueIDHash(f.FullName()) }

// NewArc 创建新的空 ARC 文件系统
// NewArc creates a new empty ARC file system
func NewArc(name string) *Arc {
	if name == "" {
		name = "root"
	}
	fs := &Arc{
		Name:          name,
		CompressGlobs: []string{"*.ks", "*.menu", "*.tjs"},
	}
	root := &Dir{Arc: fs, Name: "MeidoSerialization:" + string(filepath.Separator) + string(filepath.Separator) + name}
	fs.Root = root
	return fs
}

// FullName 返回使用操作系统分隔符连接的完整路径
// FullName returns full path with OS separator
func (d *Dir) FullName() string {
	if d.Parent == nil {
		return d.Name
	}
	return d.Parent.FullName() + string(filepath.Separator) + d.Name
}

// Depth 返回目录在树中的深度
// Depth returns the depth in the tree
func (d *Dir) Depth() int {
	if d.Parent == nil {
		return 0
	}
	return d.Parent.Depth() + 1
}

// ensure 确保目录映射和文件映射已初始化
// ensure ensures maps exist
func (d *Dir) ensure() {
	if d.Dirs == nil {
		d.Dirs = map[string]*Dir{}
	}
	if d.Files == nil {
		d.Files = map[string]*File{}
	}
}

// GetOrCreateDir 在当前目录下查找或创建指定名称的目录
// GetOrCreateDir finds or creates a directory by name under this dir
func (d *Dir) GetOrCreateDir(name string) *Dir {
	d.ensure()
	if x, ok := d.Dirs[name]; ok {
		return x
	}
	nd := &Dir{Arc: d.Arc, Name: name, Parent: d}
	d.Dirs[name] = nd
	return nd
}

// AddFile 在当前目录下添加或替换文件条目
// AddFile adds or replaces a file entry under this dir
func (d *Dir) AddFile(f *File) {
	d.ensure()
	key := f.Name
	if d.Arc.KeepDupes {
		key = d.FullName() + string(filepath.Separator) + f.Name
	}
	d.Files[key] = f
	f.Parent = d
}

// sortedDirs 返回按名称排序的子目录
// sortedDirs returns subdirectories sorted by name
func (d *Dir) sortedDirs() []*Dir {
	d.ensure()
	out := make([]*Dir, 0, len(d.Dirs))
	for _, v := range d.Dirs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// sortedFiles 返回按名称键排序的文件
// sortedFiles returns files sorted by name key
func (d *Dir) sortedFiles() []*File {
	d.ensure()
	out := make([]*File, 0, len(d.Files))
	for _, v := range d.Files {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// FullName 返回包含父目录的完整路径
// FullName returns full path including parent dirs
func (f *File) FullName() string {
	if f.Parent == nil {
		return f.Name
	}
	return f.Parent.FullName() + string(filepath.Separator) + f.Name
}

// SetData 使用内存指针设置文件数据
// SetData sets file data using a memory pointer
func (f *File) SetData(b []byte, compressed bool) {
	if compressed {
		f.Ptr = NewMemoryPointerCompressedAuto(b)
	} else {
		f.Ptr = NewMemoryPointer(b)
	}
}

// walkDirs 以深度优先顺序返回所有目录，并可排除根目录
// walkDirs returns all directories depth-first and excludes root when requested
func walkDirs(root *Dir, list *[]*Dir, excludeRoot bool) {
	if !(excludeRoot) {
		*list = append(*list, root)
	}
	for _, d := range root.sortedDirs() {
		walkDirs(d, list, false)
	}
}

// AllDirs 返回给定 ARC 文件系统中按深度优先顺序排列且不重复的所有目录
// AllDirs returns a list of all directories in the given Arc file system in depth-first order, excluding duplicates
func AllDirs(fs *Arc) []*Dir {
	var out []*Dir
	walkDirs(fs.Root, &out, false)
	return out
}

// AllFiles 返回从根目录递归遍历给定 ARC 文件系统得到的所有文件
// AllFiles returns a slice of all files in the provided Arc file system in a recursive traversal from the root directory
func AllFiles(fs *Arc) []*File {
	var out []*File
	var walk func(*Dir)
	walk = func(d *Dir) {
		for _, f := range d.sortedFiles() {
			out = append(out, f)
		}
		for _, sub := range d.sortedDirs() {
			walk(sub)
		}
	}
	walk(fs.Root)
	return out
}

// pathSplit 同时按操作系统分隔符和正斜杠拆分路径
// pathSplit splits by both OS separator and '/'
func pathSplit(p string) []string {
	p = strings.ReplaceAll(p, "\\", string(filepath.Separator))
	p = strings.ReplaceAll(p, "/", string(filepath.Separator))
	parts := strings.Split(p, string(filepath.Separator))
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s == "" || s == "." {
			continue
		}
		if s == ".." {
			out = append(out, s)
			continue
		}
		out = append(out, s)
	}
	return out
}

// GetOrCreateDirByPath 从给定父目录开始沿指定路径查找或创建目录
// GetOrCreateDirByPath navigates through or creates directories along the specified path starting from the given parent directory
func GetOrCreateDirByPath(parent *Dir, path string) *Dir {
	cur := parent
	for _, seg := range pathSplit(path) {
		switch seg {
		case "..":
			if cur.Parent != nil {
				cur = cur.Parent
			}
		case ".":
			continue
		default:
			cur = cur.GetOrCreateDir(seg)
		}
	}
	return cur
}

// AddFileByPath 在给定父目录的指定相对路径中创建文件节点
// AddFileByPath creates a file node in the directory tree at the specified path relative to the given parent directory
func AddFileByPath(parent *Dir, path string) *File {
	parts := pathSplit(path)
	if len(parts) == 0 {
		return nil
	}
	dir := parent
	if len(parts) > 1 {
		dir = GetOrCreateDirByPath(parent, strings.Join(parts[:len(parts)-1], string(filepath.Separator)))
	}
	f := &File{Arc: parent.Arc, Name: parts[len(parts)-1]}
	dir.AddFile(f)
	return f
}

// FindFileByPath 按相对于给定父目录的路径获取文件，未找到时返回 nil
// FindFileByPath retrieves a file by its path relative to the given parent directory or returns nil if not found
func FindFileByPath(parent *Dir, path string) *File {
	parts := pathSplit(path)
	if len(parts) == 0 {
		return nil
	}
	cur := parent
	for i := 0; i < len(parts)-1; i++ {
		seg := parts[i]
		switch seg {
		case "..":
			if cur.Parent != nil {
				cur = cur.Parent
			}
		case ".":
			continue
		default:
			if next, ok := cur.Dirs[seg]; ok {
				cur = next
			} else {
				return nil
			}
		}
	}

	fileName := parts[len(parts)-1]
	key := fileName
	if cur.Arc.KeepDupes {
		key = cur.FullName() + string(filepath.Separator) + fileName
	}

	if f, ok := cur.Files[key]; ok {
		return f
	}
	return nil
}

// DeleteFileByPath 从给定父目录删除相对路径所标识的文件并返回成功状态
// DeleteFileByPath removes a file identified by its relative path from the specified parent directory and returns success status
func DeleteFileByPath(parent *Dir, path string) bool {
	f := FindFileByPath(parent, path)
	if f == nil {
		return false
	}
	dir := f.Parent
	key := f.Name
	if dir.Arc.KeepDupes {
		key = dir.FullName() + string(filepath.Separator) + f.Name
	}
	delete(dir.Files, key)
	return true
}

// RelativePath 返回文件相对于 ARC 根目录的路径
// RelativePath returns the path of the file relative to the Arc root
func (f *File) RelativePath() string {
	var parts []string
	parts = append(parts, f.Name)
	cur := f.Parent
	for cur != nil && cur.Parent != nil {
		parts = append(parts, cur.Name)
		cur = cur.Parent
	}
	// 反转路径段
	// Reverse the path parts
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return filepath.Join(parts...)
}

// GetFileList 获取 ARC 文件系统中所有文件及其相对路径的列表
// GetFileList retrieves a list of all files in the Arc file system with their relative paths
func (arc *Arc) GetFileList() []string {
	files := AllFiles(arc)
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.RelativePath()
	}
	return out
}

// GetFile 按相对路径获取 ARC 文件系统中的文件，未找到时返回 nil
// GetFile retrieves a file within the Arc file system by its relative path or returns nil if the file is not found
func (arc *Arc) GetFile(path string) *File {
	return FindFileByPath(arc.Root, path)
}

// DeleteFile 删除 ARC 文件系统中相对路径所标识的文件，成功删除时返回 true
// DeleteFile removes a file identified by its relative path within the Arc file system and returns true if the file was deleted
func (arc *Arc) DeleteFile(path string) bool {
	return DeleteFileByPath(arc.Root, path)
}

// CreateFile 在 ARC 文件系统中的指定路径创建新文件并设置其数据
// 此函数在目录树中按相对于给定父目录的指定路径创建文件节点
// CreateFile creates a new file at the specified path within the Arc file system and sets its data
// This creates a file node in the directory tree at the specified path relative to the given parent directory
func (arc *Arc) CreateFile(path string, data []byte) *File {
	f := AddFileByPath(arc.Root, path)
	if f != nil {
		f.SetData(data, false)
	}
	return f
}

// CopyFile 在 ARC 文件系统中将文件从指定源路径复制到目标路径
// CopyFile copies a file from the specified source path to the destination path within the Arc file system
func (arc *Arc) CopyFile(srcPath string, dstPath string) error {
	srcFile := arc.GetFile(srcPath)
	if srcFile == nil {
		return fmt.Errorf("source file not found: %s", srcPath)
	}

	data, err := srcFile.Ptr.Data()
	if err != nil {
		return fmt.Errorf("failed to read source file data: %w", err)
	}

	dstFile := arc.CreateFile(dstPath, data)
	if dstFile == nil {
		return fmt.Errorf("failed to create destination file: %s", dstPath)
	}
	// 匹配源文件的压缩状态
	// Match compression of source
	dstFile.SetData(data, srcFile.Ptr.Compressed())
	return nil
}

// globToRegex 将 shell 风格的通配模式转换为对应的正则表达式
// globToRegex converts a shell-style glob pattern such as "*" or "?" into a corresponding regular expression
func globToRegex(glob string) (*regexp.Regexp, error) {
	// 仅执行简单转换，将星号替换为任意字符串、问号替换为任意字符，并转义正则元字符
	// Perform a simple translation of * to .*, ? to ., and escape regular-expression metacharacters
	rx := "^"
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			rx += ".*"
		case '?':
			rx += "."
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			rx += "\\" + string(c)
		default:
			rx += string(c)
		}
	}
	rx += "$"
	r, err := regexp.Compile(rx)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %s", glob)
	}
	return r, nil
}
