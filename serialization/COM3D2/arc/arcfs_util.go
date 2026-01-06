package arc

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (d *Dir) UTF8Hash() uint64  { return NameHashUTF8(d.Name) }
func (d *Dir) UTF16Hash() uint64 { return NameHashUTF16(d.Name) }
func (d *Dir) UniqueID() uint64  { return UniqueIDHash(d.FullName()) }

func (f *File) UTF8Hash() uint64  { return NameHashUTF8(f.Name) }
func (f *File) UTF16Hash() uint64 { return NameHashUTF16(f.Name) }
func (f *File) UniqueID() uint64  { return UniqueIDHash(f.FullName()) }

// walkDirs returns all directories depth-first (excluding root if excludeRoot)
func walkDirs(root *Dir, list *[]*Dir, excludeRoot bool) {
	if !(excludeRoot) {
		*list = append(*list, root)
	}
	for _, d := range root.sortedDirs() {
		walkDirs(d, list, false)
	}
}

func AllDirs(fs *Arc) []*Dir { out := []*Dir{}; walkDirs(fs.Root, &out, false); return out }

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

func AddFileByPath(parent *Dir, path string) *File {
	parts := pathSplit(path)
	if len(parts) == 0 {
		return nil
	}
	dir := parent
	if len(parts) > 1 {
		dir = GetOrCreateDirByPath(parent, strings.Join(parts[:len(parts)-1], string(filepath.Separator)))
	}
	f := &File{arc: parent.arc, Name: parts[len(parts)-1]}
	dir.AddFile(f)
	return f
}

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
	if cur.arc.KeepDupes {
		key = cur.FullName() + string(filepath.Separator) + fileName
	}

	if f, ok := cur.Files[key]; ok {
		return f
	}
	return nil
}

func DeleteFileByPath(parent *Dir, path string) bool {
	f := FindFileByPath(parent, path)
	if f == nil {
		return false
	}
	dir := f.Parent
	key := f.Name
	if dir.arc.KeepDupes {
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

func (arc *Arc) GetFileList() []string {
	files := AllFiles(arc)
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.RelativePath()
	}
	return out
}

func (arc *Arc) GetFile(path string) *File {
	return FindFileByPath(arc.Root, path)
}

func (arc *Arc) DeleteFile(path string) bool {
	return DeleteFileByPath(arc.Root, path)
}

func (arc *Arc) CreateFile(path string, data []byte) *File {
	f := AddFileByPath(arc.Root, path)
	if f != nil {
		f.SetData(data, false)
	}
	return f
}

func (arc *Arc) CopyFile(srcPath, dstPath string) error {
	srcFile := arc.GetFile(srcPath)
	if srcFile == nil {
		return fmt.Errorf("source file not found: %s", srcPath)
	}

	data, err := srcFile.Ptr.Data()
	if err != nil {
		return err
	}

	dstFile := arc.CreateFile(dstPath, data)
	if dstFile == nil {
		return fmt.Errorf("failed to create destination file: %s", dstPath)
	}
	// Match compression of source
	dstFile.SetData(data, srcFile.Ptr.Compressed())
	return nil
}
