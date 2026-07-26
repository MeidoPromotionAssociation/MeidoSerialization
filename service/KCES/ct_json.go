package KCES

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/strictjson"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

const CtEnvelopeFormat = "kces-content-table"

// CtEnvelope 是将 catalog 和 ExtensionNameList 解码为类型化结构并以 base64 保留其他虚拟文件的 KCES .ct 编辑封套 / CtEnvelope is the KCES .ct editing envelope that decodes catalog and ExtensionNameList entries into typed structures while preserving other virtual files as base64 payloads
type CtEnvelope struct {
	Format             string                                 `json:"format"`                       // 封套格式标识，固定为 kces-content-table / Envelope format marker, fixed to kces-content-table
	Version            int32                                  `json:"version"`                      // VirtualDirectory 版本号 / VirtualDirectory version
	Directories        map[string]ct.VirtualDirectoryMetadata `json:"directories,omitempty"`        // 子目录路径和真实版本字段，包含空目录 / Child-directory paths and real version fields, including empty directories
	Catalog            ct.AssetBundleCatalog                  `json:"catalog"`                      // 必需的 catalog 虚拟文件内容 / Required decoded catalog virtual file content
	ExtensionNameLists map[string]*ct.ExtensionNameList       `json:"extensionNameLists,omitempty"` // 按扩展名索引的 ExtensionNameList / ExtensionNameList values keyed by extension
	Files              []CtEnvelopeFile                       `json:"files,omitempty"`              // 未识别或非 catalog 虚拟文件 / Unrecognized or non-catalog virtual files
}

// CtEnvelopeFile 保留 .ct 包中的非 catalog 虚拟文件 / CtEnvelopeFile preserves a non-catalog virtual file from a .ct bundle
type CtEnvelopeFile struct {
	Name       string `json:"name"`       // 虚拟文件名 / Virtual file name
	DataBase64 string `json:"dataBase64"` // 原始文件数据的 base64 / Base64 of the raw file data
}

// UnmarshalJSON 严格解码 .ct 编辑封套并拒绝缺失根字段或空 ExtensionNameLists 键
// UnmarshalJSON strictly decodes the .ct editing envelope and rejects missing root fields or empty ExtensionNameLists keys
func (value *CtEnvelope) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil KCES content-table JSON target")
	}
	type plainCtEnvelope CtEnvelope
	var decoded plainCtEnvelope
	if err := strictjson.Decode(data, &decoded); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "ct", "format", "version", "catalog"); err != nil {
		return err
	}
	for name := range decoded.ExtensionNameLists {
		if name == "" {
			return fmt.Errorf("ct envelope extensionNameLists contains an empty key")
		}
	}
	*value = CtEnvelope(decoded)
	return nil
}

// UnmarshalJSON 严格解码 .ct 独立虚拟文件并要求名称与 dataBase64 字段显式出现
// UnmarshalJSON strictly decodes a standalone .ct virtual file and requires name and dataBase64 to be explicitly present
func (value *CtEnvelopeFile) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil KCES content-table file JSON target")
	}
	type plainCtEnvelopeFile CtEnvelopeFile
	var decoded plainCtEnvelopeFile
	if err := strictjson.Decode(data, &decoded); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "ct.files[]", "name", "dataBase64"); err != nil {
		return err
	}
	*value = CtEnvelopeFile(decoded)
	return nil
}

// IsKCESCtFile 判断路径是否为原生 KCES .ct 文件
// IsKCESCtFile reports whether a path names a native KCES .ct file
func IsKCESCtFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".ct") && !strings.HasSuffix(lower, ".ct.json")
}

// IsKCESCtJSONFile 判断路径是否为带正确格式标记的 KCES .ct 编辑 JSON
// IsKCESCtJSONFile reports whether a path names KCES .ct editing JSON with the correct format marker
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
	if err := json.Unmarshal(trimJSONUTF8BOM(data), &header); err != nil {
		return false
	}
	return header.Format == CtEnvelopeFormat
}

// ConvertCtToJson 将 KCES .ct 文件转换为可编辑 JSON 封套
// ConvertCtToJson converts a KCES .ct file into an editable JSON envelope
func (s *CtService) ConvertCtToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	envelope, err := s.ReadCtEnvelope(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES ct json: %w", err)
	}
	data = append(data, '\n')
	return writeCtConversionOutput(ctx, outputPath, data, maxOutputBytes)
}

// ConvertJsonToCt 将可编辑 JSON 封套转换回 KCES .ct 文件
// ConvertJsonToCt converts an editable JSON envelope back into a KCES .ct file
func (s *CtService) ConvertJsonToCt(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}

	var envelope CtEnvelope
	if err := decodeStrictJSON(data, &envelope, "KCES content-table JSON"); err != nil {
		return fmt.Errorf("parse KCES ct json: %w", err)
	}
	if envelope.Format != CtEnvelopeFormat {
		return fmt.Errorf("unsupported ct JSON format %q", envelope.Format)
	}

	table, err := buildContentTableFromCtEnvelope(&envelope)
	if err != nil {
		return err
	}
	var encoded bytes.Buffer
	if err := ct.WriteContentTable(&encoded, table); err != nil {
		return fmt.Errorf("encode .ct file: %w", err)
	}
	return writeCtConversionOutput(ctx, outputPath, encoded.Bytes(), maxOutputBytes)
}

// ReadCtEnvelope 读取 .ct 文件并返回可编辑 JSON 封套
// ReadCtEnvelope reads a .ct file and returns its editable JSON envelope
func (s *CtService) ReadCtEnvelope(path string) (*CtEnvelope, error) {
	table, err := s.ReadCt(path)
	if err != nil {
		return nil, err
	}
	return readCtEnvelopeFromTable(table)
}

// readCtEnvelopeFromTable 将 ContentTable 中已知虚拟文件展开为类型化编辑封套
// readCtEnvelopeFromTable expands known virtual files from a ContentTable into a typed editing envelope
func readCtEnvelopeFromTable(table *ct.ContentTable) (*CtEnvelope, error) {
	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	if catalog == nil {
		return nil, fmt.Errorf("catalog root must not be null")
	}

	envelope := &CtEnvelope{
		Format:      CtEnvelopeFormat,
		Version:     table.Version,
		Directories: table.GetVirtualDirectoryMetadata(),
		Catalog:     *catalog,
	}
	envelope.ExtensionNameLists = make(map[string]*ct.ExtensionNameList, len(catalog.ExtensionList))

	rawFiles := make(map[string][]byte)
	consumed := map[string]struct{}{
		"catalog": {},
	}

	for index, extension := range catalog.ExtensionList {
		if extension == nil || *extension == "" {
			return nil, fmt.Errorf("catalog.extensionList[%d] must name an ExtensionNameList virtual file", index)
		}
		ext := *extension
		if _, seen := consumed[ext]; seen {
			continue
		}
		consumed[ext] = struct{}{}

		enl, err := ct.DecodeExtensionNameListFromCt(table, ext)
		if err != nil {
			return nil, fmt.Errorf("decode ExtensionNameList %q: %w", ext, err)
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

// buildContentTableFromCtEnvelope 校验编辑封套并重建完整 ContentTable
// buildContentTableFromCtEnvelope validates an editing envelope and rebuilds the complete ContentTable
func buildContentTableFromCtEnvelope(envelope *CtEnvelope) (*ct.ContentTable, error) {
	if envelope == nil {
		return nil, fmt.Errorf("ct envelope is nil")
	}
	table := &ct.ContentTable{
		Version:     envelope.Version,
		Directories: envelope.Directories,
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

	catalogData, err := ct.EncodeCatalog(&envelope.Catalog)
	if err != nil {
		return nil, fmt.Errorf("encode catalog: %w", err)
	}
	compressedCatalog, err := ct.CompressLz4BlockArray(catalogData)
	if err != nil {
		return nil, fmt.Errorf("compress catalog: %w", err)
	}
	if err := table.AddFile("catalog", compressedCatalog); err != nil {
		return nil, err
	}
	if _, exists := rawFiles["catalog"]; exists {
		return nil, fmt.Errorf(`files cannot contain the known virtual file "catalog"`)
	}

	seenExt := map[string]struct{}{}
	for index, extension := range envelope.Catalog.ExtensionList {
		if extension == nil || *extension == "" {
			return nil, fmt.Errorf("catalog.extensionList[%d] must name an ExtensionNameList virtual file", index)
		}
		ext := *extension
		if _, ok := seenExt[ext]; ok {
			continue
		}
		seenExt[ext] = struct{}{}

		enl, ok := envelope.ExtensionNameLists[ext]
		if !ok {
			return nil, fmt.Errorf("missing typed ExtensionNameList %q", ext)
		}
		if _, exists := rawFiles[ext]; exists {
			return nil, fmt.Errorf("files cannot contain catalog-declared ExtensionNameList %q", ext)
		}
		data, err := ct.EncodeExtensionNameList(enl)
		if err != nil {
			return nil, fmt.Errorf("encode ExtensionNameList %q: %w", ext, err)
		}
		compressed, err := ct.CompressLz4BlockArray(data)
		if err != nil {
			return nil, fmt.Errorf("compress ExtensionNameList %q: %w", ext, err)
		}
		if err := table.AddFile(ext, compressed); err != nil {
			return nil, err
		}
	}

	for ext := range envelope.ExtensionNameLists {
		if ext == "" {
			return nil, fmt.Errorf("extensionNameLists contains an empty key")
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
			if err := table.AddFile(name, rawFiles[name]); err != nil {
				return nil, err
			}
		}
	}
	return table, nil
}

// WriteCtEnvelopeFile 将可编辑内容表封套直接构建并写入 .ct 或 VirtualDirectory 文件
// WriteCtEnvelopeFile directly builds an editable content table envelope and writes it to a .ct or VirtualDirectory file
func (s *CtService) WriteCtEnvelopeFile(path string, value *CtEnvelope) error {
	if value == nil {
		return fmt.Errorf("encode KCES content table envelope: nil envelope")
	}
	if value.Format != CtEnvelopeFormat {
		return fmt.Errorf("unsupported ct envelope format %q", value.Format)
	}
	table, err := buildContentTableFromCtEnvelope(value)
	if err != nil {
		return fmt.Errorf("encode KCES content table envelope: %w", err)
	}
	return s.WriteCtFile(path, table)
}

// writeCtConversionOutput 在上下文有效且大小不超限时写入 .ct 转换结果
// writeCtConversionOutput writes .ct conversion output while the context is active and the size remains within the limit
func writeCtConversionOutput(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive .ct conversion output limit is required")
	}
	dataSize := int64(len(data))
	if dataSize > maxOutputBytes {
		return fmt.Errorf("%w: .ct conversion output needs %d bytes but the limit is %d", ErrConversionOutputLimitExceeded, dataSize, maxOutputBytes)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write .ct conversion output %q: %w", path, err)
	}
	if ctx != nil {
		return ctx.Err()
	}
	return nil
}
