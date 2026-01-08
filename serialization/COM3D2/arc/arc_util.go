package arc

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// UTF8Hash computes a hash for the directory name using UTF-8 encoding and case-insensitive comparison.
func (d *Dir) UTF8Hash() uint64 { return NameHashUTF8(d.Name) }

// UTF16Hash computes a hash for the directory name using UTF-16LE encoding and case-insensitive comparison.
func (d *Dir) UTF16Hash() uint64 { return NameHashUTF16(d.Name) }

// UniqueID computes a unique identifier for the directory based on its full path using UTF-16LE encoding.
func (d *Dir) UniqueID() uint64 { return UniqueIDHash(d.FullName()) }

// UTF8Hash computes and returns a hash value for the file name using UTF-8 encoding and case-insensitive comparison.
func (f *File) UTF8Hash() uint64 { return NameHashUTF8(f.Name) }

// UTF16Hash computes and returns a hash value for the file name using UTF-16 encoding and case-insensitive comparison.
func (f *File) UTF16Hash() uint64 { return NameHashUTF16(f.Name) }

// UniqueID computes and returns a unique identifier for the file based on its full path encoded as UTF-16LE without lowercasing.
func (f *File) UniqueID() uint64 { return UniqueIDHash(f.FullName()) }

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

// FullName returns full path with OS separator
func (d *Dir) FullName() string {
	if d.Parent == nil {
		return d.Name
	}
	return d.Parent.FullName() + string(filepath.Separator) + d.Name
}

// Depth returns the depth in the tree
func (d *Dir) Depth() int {
	if d.Parent == nil {
		return 0
	}
	return d.Parent.Depth() + 1
}

// ensure maps exist
func (d *Dir) ensure() {
	if d.Dirs == nil {
		d.Dirs = map[string]*Dir{}
	}
	if d.Files == nil {
		d.Files = map[string]*File{}
	}
}

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

// Sorted subDirs by name
func (d *Dir) sortedDirs() []*Dir {
	d.ensure()
	out := make([]*Dir, 0, len(d.Dirs))
	for _, v := range d.Dirs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// Sorted files by name key
func (d *Dir) sortedFiles() []*File {
	d.ensure()
	out := make([]*File, 0, len(d.Files))
	for _, v := range d.Files {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// FullName returns full path including parent dirs
func (f *File) FullName() string {
	if f.Parent == nil {
		return f.Name
	}
	return f.Parent.FullName() + string(filepath.Separator) + f.Name
}

// SetData sets file data using a memory pointer
func (f *File) SetData(b []byte, compressed bool) {
	if compressed {
		f.Ptr = NewMemoryPointerCompressedAuto(b)
	} else {
		f.Ptr = NewMemoryPointer(b)
	}
}

// walkDirs returns all directories depth-first (excluding root if excludeRoot)
func walkDirs(root *Dir, list *[]*Dir, excludeRoot bool) {
	if !(excludeRoot) {
		*list = append(*list, root)
	}
	for _, d := range root.sortedDirs() {
		walkDirs(d, list, false)
	}
}

// AllDirs returns a list of all directories in the given Arc file system in depth-first order, excluding duplicates.
func AllDirs(fs *Arc) []*Dir {
	var out []*Dir
	walkDirs(fs.Root, &out, false)
	return out
}

// AllFiles returns a slice of all files in the provided Arc file system in a recursive traversal from the root directory.
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

// GetOrCreateDirByPath navigates through or creates directories along the specified path starting from the given parent directory.
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

// AddFileByPath creates a file node in the directory tree at the specified path relative to the given parent directory.
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

// FindFileByPath retrieves a file by its path relative to the given parent directory or returns nil if not found.
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

// DeleteFileByPath removes a file identified by its relative path from the specified parent directory and returns success status.
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

// RelativePath returns the path of the file relative to the Arc root.
func (f *File) RelativePath() string {
	var parts []string
	parts = append(parts, f.Name)
	cur := f.Parent
	for cur != nil && cur.Parent != nil {
		parts = append(parts, cur.Name)
		cur = cur.Parent
	}
	// reverse parts
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return filepath.Join(parts...)
}

// GetFileList retrieves a list of all files in the Arc file system with their relative paths.
func (arc *Arc) GetFileList() []string {
	files := AllFiles(arc)
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.RelativePath()
	}
	return out
}

// GetFile retrieves a file within the Arc file system by its relative path or returns nil if the file is not found.
func (arc *Arc) GetFile(path string) *File {
	return FindFileByPath(arc.Root, path)
}

// DeleteFile removes a file identified by its relative path within the Arc file system. Returns true if the file was deleted.
func (arc *Arc) DeleteFile(path string) bool {
	return DeleteFileByPath(arc.Root, path)
}

// CreateFile creates a new file at the specified path within the Arc file system and sets its data.
// creates a file node in the directory tree at the specified path relative to the given parent directory.
func (arc *Arc) CreateFile(path string, data []byte) *File {
	f := AddFileByPath(arc.Root, path)
	if f != nil {
		f.SetData(data, false)
	}
	return f
}

// CopyFile copies a file from the specified source path to the destination path within the Arc file system.
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
	// Match compression of source
	dstFile.SetData(data, srcFile.Ptr.Compressed())
	return nil
}

// globToRegex converts a shell-style glob pattern (e.g., "*", "?") into a corresponding regular expression.
func globToRegex(glob string) (*regexp.Regexp, error) {
	// very simple translation: * -> .*, ? -> .
	// escape regex meta
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
