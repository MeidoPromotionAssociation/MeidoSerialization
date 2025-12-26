package arc

import (
	"path/filepath"
	"sort"
	"strings"
)

// Arc represents an ARC file system in memory
// It mirrors the structure used by the original C# ArcFileSystem.
type Arc struct {
	Name          string
	Root          *Dir
	KeepDupes     bool
	CompressGlobs []string
}

// New creates a new empty ARC file system
func New(name string) *Arc {
	if name == "" {
		name = "root"
	}
	fs := &Arc{
		Name:          name,
		CompressGlobs: []string{"*.ks", "*.menu", "*.tjs"},
	}
	root := &Dir{fs: fs, Name: "MeidoSerialization:" + string(filepath.Separator) + string(filepath.Separator) + name}
	fs.Root = root
	return fs
}

// Dir represents a directory node
type Dir struct {
	fs     *Arc
	Name   string
	Parent *Dir
	Dirs   map[string]*Dir
	Files  map[string]*File
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
	nd := &Dir{fs: d.fs, Name: name, Parent: d}
	d.Dirs[name] = nd
	return nd
}

// AddFile adds or replaces a file entry under this dir
func (d *Dir) AddFile(f *File) {
	d.ensure()
	key := f.Name
	if d.fs.KeepDupes {
		key = d.FullName() + string(filepath.Separator) + f.Name
	}
	d.Files[key] = f
	f.Parent = d
}

// Sorted subdirs by name
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

// File represents a file node with data pointer
type File struct {
	fs     *Arc
	Name   string
	Parent *Dir
	Ptr    FilePointer
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
