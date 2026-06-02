package KCES

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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

	for _, name := range table.GetFileNames() {
		data, err := table.GetFileData(name)
		if err != nil {
			return fmt.Errorf("extract %q failed: %w", name, err)
		}

		outPath := filepath.Join(outDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("create directory for %q failed: %w", name, err)
		}
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("write %q failed: %w", name, err)
		}
	}
	return nil
}

// PackCt 将目录打包为 .ct 文件
func (s *CtService) PackCt(dirPath string, outPath string) error {
	table, err := ct.NewContentTableFromDir(dirPath)
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
