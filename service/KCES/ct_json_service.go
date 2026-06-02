package KCES

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

const CtEnvelopeFormat = "kces-content-table"

// CtEnvelope 是 KCES .ct 文件的 JSON 可编辑封套 / CtEnvelope is the JSON-editable wrapper for KCES .ct files
// catalog 和 ExtensionNameList 会解码为类型化结构，其他虚拟文件以 base64 保留 / catalog and ExtensionNameList entries are decoded into typed structures while other virtual files are preserved as base64 payloads
type CtEnvelope struct {
	Format             string                           `json:"format"`                       // 封套格式标识，固定为 kces-content-table / Envelope format marker, fixed to kces-content-table
	Version            int                              `json:"version"`                      // VirtualDirectory 版本号 / VirtualDirectory version
	Catalog            *ct.AssetBundleCatalog           `json:"catalog,omitempty"`            // catalog 虚拟文件内容 / Decoded catalog virtual file content
	ExtensionNameLists map[string]*ct.ExtensionNameList `json:"extensionNameLists,omitempty"` // 按扩展名索引的 ExtensionNameList / ExtensionNameList values keyed by extension
	Files              []CtEnvelopeFile                 `json:"files,omitempty"`              // 未识别或非 catalog 虚拟文件 / Unrecognized or non-catalog virtual files
}

// CtEnvelopeFile 保留 .ct 包中的非 catalog 虚拟文件 / CtEnvelopeFile preserves a non-catalog virtual file from a .ct bundle
type CtEnvelopeFile struct {
	Name       string `json:"name"`       // 虚拟文件名 / Virtual file name
	DataBase64 string `json:"dataBase64"` // 原始文件数据的 base64 / Base64 of the raw file data
}

// IsKCESCtFile reports whether path is a KCES .ct file.
func IsKCESCtFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".ct") && !strings.HasSuffix(lower, ".ct.json")
}

// IsKCESCtJSONFile reports whether path is a JSON representation of a KCES .ct file.
func IsKCESCtJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".ct.json") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var header struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return false
	}
	return header.Format == CtEnvelopeFormat
}

// ConvertCtToJson converts a KCES .ct file into a JSON envelope.
func (s *CtService) ConvertCtToJson(inputPath string, outputPath string) error {
	envelope, err := s.ReadCtEnvelope(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES ct json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

// ConvertJsonToCt converts a JSON envelope back into a KCES .ct file.
func (s *CtService) ConvertJsonToCt(inputPath string, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}

	var envelope CtEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("parse KCES ct json: %w", err)
	}
	if envelope.Format != "" && envelope.Format != CtEnvelopeFormat {
		return fmt.Errorf("unsupported ct JSON format %q", envelope.Format)
	}

	table, err := buildContentTableFromCtEnvelope(&envelope)
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create %q: %w", outputPath, err)
	}
	defer f.Close()

	if err := ct.WriteContentTable(f, table); err != nil {
		return fmt.Errorf("write .ct file: %w", err)
	}
	return nil
}

// ReadCtEnvelope reads a .ct file and returns its JSON-editable envelope.
func (s *CtService) ReadCtEnvelope(path string) (*CtEnvelope, error) {
	table, err := s.ReadCt(path)
	if err != nil {
		return nil, err
	}
	return readCtEnvelopeFromTable(table)
}

func readCtEnvelopeFromTable(table *ct.ContentTable) (*CtEnvelope, error) {
	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}

	envelope := &CtEnvelope{
		Format:             CtEnvelopeFormat,
		Version:            table.Version,
		Catalog:            catalog,
		ExtensionNameLists: make(map[string]*ct.ExtensionNameList, len(catalog.ExtensionList)),
	}

	rawFiles := make(map[string][]byte)
	consumed := map[string]struct{}{
		"catalog": {},
	}

	for _, ext := range catalog.ExtensionList {
		if ext == "" {
			continue
		}
		if _, seen := consumed[ext]; seen {
			continue
		}
		consumed[ext] = struct{}{}

		enl, err := ct.DecodeExtensionNameListFromCt(table, ext)
		if err != nil {
			data, rawErr := table.GetFileData(ext)
			if rawErr != nil {
				return nil, fmt.Errorf("decode ExtensionNameList %q: %w", ext, err)
			}
			rawFiles[ext] = append([]byte(nil), data...)
			continue
		}
		envelope.ExtensionNameLists[ext] = enl
	}

	for _, name := range table.GetFileNames() {
		if name == "catalog" {
			continue
		}
		if _, ok := consumed[name]; ok {
			continue
		}
		data, err := table.GetFileData(name)
		if err != nil {
			return nil, fmt.Errorf("read virtual file %q: %w", name, err)
		}
		rawFiles[name] = append([]byte(nil), data...)
	}

	if len(envelope.ExtensionNameLists) == 0 {
		envelope.ExtensionNameLists = nil
	}
	if len(rawFiles) > 0 {
		names := make([]string, 0, len(rawFiles))
		for name := range rawFiles {
			names = append(names, name)
		}
		sort.Strings(names)
		envelope.Files = make([]CtEnvelopeFile, 0, len(names))
		for _, name := range names {
			envelope.Files = append(envelope.Files, CtEnvelopeFile{
				Name:       name,
				DataBase64: base64.StdEncoding.EncodeToString(rawFiles[name]),
			})
		}
	}

	return envelope, nil
}

func buildContentTableFromCtEnvelope(envelope *CtEnvelope) (*ct.ContentTable, error) {
	if envelope == nil {
		return nil, fmt.Errorf("ct envelope is nil")
	}
	if envelope.Catalog == nil {
		return nil, fmt.Errorf("ct envelope missing catalog")
	}

	table := &ct.ContentTable{
		Version: envelope.Version,
	}
	if table.Version == 0 {
		table.Version = 1000
	}

	rawFiles := make(map[string][]byte, len(envelope.Files))
	for _, file := range envelope.Files {
		if file.Name == "" {
			return nil, fmt.Errorf("ct envelope file name is required")
		}
		if _, exists := rawFiles[file.Name]; exists {
			return nil, fmt.Errorf("duplicate ct envelope file %q", file.Name)
		}
		data, err := base64.StdEncoding.DecodeString(file.DataBase64)
		if err != nil {
			return nil, fmt.Errorf("decode %q dataBase64: %w", file.Name, err)
		}
		rawFiles[file.Name] = data
	}

	catalogData, err := ct.EncodeCatalog(envelope.Catalog)
	if err != nil {
		return nil, fmt.Errorf("encode catalog: %w", err)
	}
	compressedCatalog, err := ct.CompressLz4BlockArray(catalogData)
	if err != nil {
		return nil, fmt.Errorf("compress catalog: %w", err)
	}
	table.AddFile("catalog", compressedCatalog)
	delete(rawFiles, "catalog")

	seenExt := map[string]struct{}{}
	for _, ext := range envelope.Catalog.ExtensionList {
		if ext == "" {
			continue
		}
		if _, ok := seenExt[ext]; ok {
			continue
		}
		seenExt[ext] = struct{}{}

		if enl, ok := envelope.ExtensionNameLists[ext]; ok && enl != nil {
			if enl.Extension == "" {
				enl.Extension = ext
			}
			data, err := ct.EncodeExtensionNameList(enl)
			if err != nil {
				return nil, fmt.Errorf("encode ExtensionNameList %q: %w", ext, err)
			}
			compressed, err := ct.CompressLz4BlockArray(data)
			if err != nil {
				return nil, fmt.Errorf("compress ExtensionNameList %q: %w", ext, err)
			}
			table.AddFile(ext, compressed)
			delete(rawFiles, ext)
			continue
		}

		data, ok := rawFiles[ext]
		if !ok {
			return nil, fmt.Errorf("missing ExtensionNameList %q", ext)
		}
		table.AddFile(ext, data)
		delete(rawFiles, ext)
	}

	for ext := range envelope.ExtensionNameLists {
		if ext == "" {
			continue
		}
		if _, ok := seenExt[ext]; !ok {
			return nil, fmt.Errorf("extensionNameLists contains %q not listed in catalog.extensionList", ext)
		}
	}

	if len(rawFiles) > 0 {
		names := make([]string, 0, len(rawFiles))
		for name := range rawFiles {
			if name == "" {
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			table.AddFile(name, rawFiles[name])
		}
	}

	return table, nil
}
