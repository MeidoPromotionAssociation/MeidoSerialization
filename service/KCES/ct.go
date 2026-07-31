package KCES

import (
	"bytes"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// CtService 提供 .ct 文件 VirtualDirectory 的读取、列出、提取、生成和写入操作 / CtService provides read, list, extraction, generation, and write operations for .ct VirtualDirectory files
type CtService struct{}

// ReadCt 读取 .ct 文件并返回 ContentTable
// ReadCt reads a .ct file and returns its ContentTable
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
// ListCt lists every virtual file name stored in a .ct file
func (s *CtService) ListCt(path string) ([]string, error) {
	table, err := s.ReadCt(path)
	if err != nil {
		return nil, err
	}
	return table.GetFileNames(), nil
}

// GenerateCtFromAba 读取 .aba 的 AssetBundle 容器并生成使用默认 Parts/Plugin 元数据的配套 .ct 文件，outPath 为空时输出到 .aba 同目录同基名
// GenerateCtFromAba reads the AssetBundle container of a .aba file and generates its companion .ct file with default Parts/Plugin metadata, writing next to the .aba with the same base name when outPath is empty
func (s *CtService) GenerateCtFromAba(abaPath string, outPath string) error {
	name := strings.TrimSuffix(filepath.Base(abaPath), filepath.Ext(abaPath))
	if err := validateModOutputName(name); err != nil {
		return fmt.Errorf("invalid catalog name %q derived from %q: %w", name, abaPath, err)
	}
	entries, err := collectAbaCatalogEntries(abaPath)
	if err != nil {
		return err
	}
	table, err := buildKcesModContentTable(name, "", ct.CatalogTypeParts, ct.PackageTypePlugin, 0, entries)
	if err != nil {
		return fmt.Errorf("build content table for %q: %w", abaPath, err)
	}
	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(abaPath), name+".ct")
	}
	return s.WriteCtFile(outPath, table)
}

// collectAbaCatalogEntries 遍历 .aba 中全部 SerializedFile 并为每个 m_Container 条目按游戏规则收集 catalog 条目
// collectAbaCatalogEntries walks every SerializedFile in a .aba and collects one catalog entry per m_Container entry using the game's rules
func collectAbaCatalogEntries(abaPath string) ([]catalogEntry, error) {
	abaService := &AbaService{}
	abaFile, f, err := abaService.ReadAba(abaPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []catalogEntry
	seenNames := make(map[uint64]string)
	for directoryIndex, dir := range abaFile.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := abaFile.GetFileData(int64(directoryIndex))
		if err != nil {
			return nil, fmt.Errorf("read serialized .aba entry %q at directory index %d: %w", dir.Name, directoryIndex, err)
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return nil, fmt.Errorf("parse serialized .aba entry %q at directory index %d: %w", dir.Name, directoryIndex, err)
		}
		containerNames, err := af.GetAssetBundleContainerMap()
		if err != nil {
			return nil, fmt.Errorf("read AssetBundle container map from %q: %w", dir.Name, err)
		}
		for _, entry := range af.GetAssetEntries() {
			if entry.TypeId == aba.ClassIDAssetBundle {
				continue
			}
			containerPath := containerNames[entry.PathId]
			if containerPath == "" {
				continue
			}
			name := catalogNameFromContainerPath(containerPath)
			if name == "" {
				return nil, fmt.Errorf("m_Container path %q for PathID %d in %q yields an empty catalog name", containerPath, entry.PathId, dir.Name)
			}
			// 同名不同类型的对象共用一个 catalog 条目，游戏在 LoadAsset 时按类型区分
			// Objects that share one name across types share one catalog entry, and the game distinguishes them by type in LoadAsset
			nameHash := ct.HashStringIgnoreCase(name)
			if previous, exists := seenNames[nameHash]; exists && strings.EqualFold(previous, name) {
				continue
			}
			if _, exists := seenNames[nameHash]; !exists {
				seenNames[nameHash] = name
			}
			ext := strings.ToLower(filepath.Ext(name))
			// 官方 system.ct 将无后缀资源的 ExtensionNameList 分组命名为 null
			// Official system.ct names the ExtensionNameList group for extensionless resources null
			if ext == "" {
				ext = "null"
			}
			entries = append(entries, catalogEntry{name: name, ext: ext})
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no cataloged assets found in %q", abaPath)
	}
	return entries, nil
}

// catalogNameFromContainerPath 从 m_Container 加载名还原 catalog 资源名称，Unity 资产路径按游戏规则取 basename 并去掉一层资产扩展名，本库打包的平坦短名保持原样
// catalogNameFromContainerPath restores the catalog resource name from an m_Container load name, taking the basename of a Unity asset path and stripping one asset-extension layer per the game's rules while keeping flat short names from this library's packer unchanged
func catalogNameFromContainerPath(containerPath string) string {
	normalized := strings.ReplaceAll(containerPath, "\\", "/")
	base := pathpkg.Base(normalized)
	if base == "." || base == "/" {
		return ""
	}
	if strings.ContainsRune(normalized, '/') {
		return strings.TrimSuffix(base, pathpkg.Ext(base))
	}
	return base
}

// ExtractFile 从 .ct 中提取单个虚拟文件并写入目标 writer
// ExtractFile extracts one virtual file from a .ct file and writes it to the destination writer
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

// WriteCtFile 将内容表结构直接编码并写入 .ct 或 VirtualDirectory 文件
// WriteCtFile directly encodes a content table value and writes it to a .ct or VirtualDirectory file
func (s *CtService) WriteCtFile(path string, value *ct.ContentTable) error {
	var encoded bytes.Buffer
	if err := ct.WriteContentTable(&encoded, value); err != nil {
		return fmt.Errorf("encode KCES content table: %w", err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0644); err != nil {
		return fmt.Errorf("write .ct file %q: %w", path, err)
	}
	return nil
}
