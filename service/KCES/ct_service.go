package KCES

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// CtService 提供 .ct 文件 VirtualDirectory 的读取、写入、列出和提取操作 / CtService provides read, write, list, and extraction operations for .ct VirtualDirectory files
type CtService struct{}

// ReadCt 读取 .ct 文件并返回 ContentTable
func (s *CtService) ReadCt(path string) (*ct.ContentTable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .ct file failed: %w", err)
	}
	defer f.Close()

	table, err := ct.ReadContentTable(f)
	if err != nil {
		return nil, fmt.Errorf("parse .ct file failed: %w", err)
	}
	return table, nil
}

// ListCt 列出 .ct 文件中的所有文件名
func (s *CtService) ListCt(path string) ([]string, error) {
	table, err := s.ReadCt(path)
	if err != nil {
		return nil, err
	}
	return table.GetFileNames(), nil
}

// UnpackCt 将 .ct 文件解压到指定目录
func (s *CtService) UnpackCt(ctPath string, outDir string) error {
	table, err := s.ReadCt(ctPath)
	if err != nil {
		return err
	}

	if outDir == "" {
		outDir = ctPath + "_unpacked"
	}

	type extractFile struct {
		archiveName string
		relPath     string
		data        []byte
	}
	names := table.GetFileNames()
	sort.Strings(names)
	files := make([]extractFile, 0, len(names))
	seenPaths := make(map[string]string, len(names))
	for _, name := range names {
		relPath, err := virtualDirectoryNameToExtractionPath(name)
		if err != nil {
			return fmt.Errorf("unsafe virtual file name %q: %w", name, err)
		}
		// Windows output paths are case-insensitive. Reject portable collisions
		// instead of letting map iteration order decide which entry wins.
		pathKey := strings.ToLower(relPath)
		if previous, ok := seenPaths[pathKey]; ok {
			return fmt.Errorf("virtual file names %q and %q map to the same output path", previous, name)
		}
		seenPaths[pathKey] = name

		data, err := table.GetFileData(name)
		if err != nil {
			return fmt.Errorf("extract %q failed: %w", name, err)
		}
		files = append(files, extractFile{archiveName: name, relPath: relPath, data: data})
	}

	root, err := openExtractionRoot(outDir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range files {
		if err := root.WriteFile(file.relPath, file.data, 0644); err != nil {
			return fmt.Errorf("write virtual file %q failed: %w", file.archiveName, err)
		}
	}
	return nil
}

// PackCt 将目录打包为 .ct 文件
func (s *CtService) PackCt(dirPath string, outPath string) error {
	table, err := newContentTableFromGameWindowsDirectory(dirPath)
	if err != nil {
		return fmt.Errorf("create content table from directory failed: %w", err)
	}

	if outPath == "" {
		outPath = dirPath + ".ct"
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file failed: %w", err)
	}
	defer f.Close()

	if err := ct.WriteContentTable(f, table); err != nil {
		return fmt.Errorf("write .ct file failed: %w", err)
	}
	return nil
}

func newContentTableFromGameWindowsDirectory(dirPath string) (*ct.ContentTable, error) {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("resolve source directory: %w", err)
	}
	rootInfo, err := os.Lstat(absDir)
	if err != nil {
		return nil, fmt.Errorf("inspect source directory: %w", err)
	}
	if !rootInfo.IsDir() || isLinkOrReparse(rootInfo) {
		return nil, fmt.Errorf("source path must be a real directory, not a symlink or reparse point")
	}
	root, err := os.OpenRoot(absDir)
	if err != nil {
		return nil, fmt.Errorf("open source directory: %w", err)
	}
	defer root.Close()

	type sourceFile struct {
		diskPath    string
		virtualName string
	}
	var sources []sourceFile
	seenVirtualNames := make(map[string]string)
	err = filepath.WalkDir(absDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absDir {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if isLinkOrReparse(info) {
			return fmt.Errorf("source entry %q is a symlink or reparse point", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("source entry %q is not a regular file", path)
		}
		rel, relErr := filepath.Rel(absDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		virtualName, nameErr := extractionPathToVirtualDirectoryName(rel)
		if nameErr != nil {
			return fmt.Errorf("decode game VirtualDirectory file name %q: %w", rel, nameErr)
		}
		key := strings.ToLower(virtualName)
		if previous, exists := seenVirtualNames[key]; exists {
			return fmt.Errorf("source files %q and %q map to the same case-insensitive VirtualDirectory path %q", previous, rel, virtualName)
		}
		seenVirtualNames[key] = rel
		sources = append(sources, sourceFile{diskPath: rel, virtualName: virtualName})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk source directory: %w", err)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].virtualName < sources[j].virtualName })

	table := &ct.ContentTable{
		Version: 1000,
		Files:   make(map[string]ct.VirtualFile),
		Raw:     make([]byte, ct.HeaderSize),
	}
	copy(table.Raw[:7], ct.FileSignature)
	table.Raw[7] = ct.SerializeTypeMsgPack
	for _, source := range sources {
		data, readErr := readPackRootRegularFile(root, source.diskPath)
		if readErr != nil {
			return nil, fmt.Errorf("read source file %q: %w", source.diskPath, readErr)
		}
		if err := table.AddFile(source.virtualName, data); err != nil {
			return nil, err
		}
	}
	return table, nil
}

// ExtractFile 从 .ct 中提取单个文件
func (s *CtService) ExtractFile(ctPath string, fileName string, w io.Writer) error {
	table, err := s.ReadCt(ctPath)
	if err != nil {
		return err
	}

	data, err := table.GetFileData(fileName)
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}
