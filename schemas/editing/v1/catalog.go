// Package editingv1 公开当前编辑 JSON 契约中签入仓库的 Draft 2020-12 模式
// Package editingv1 exposes checked-in Draft 2020-12 schemas for the current editing JSON contract
package editingv1

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

const (
	// Version 是当前编辑模式目录版本 / Version is the current editing-schema catalog version
	Version = "1.0.0"
	// Dialect 是全部编辑模式使用的 JSON Schema 方言 / Dialect is the JSON Schema dialect used by every editing schema
	Dialect = "https://json-schema.org/draft/2020-12/schema"
	// MediaType 是编辑模式文档的媒体类型 / MediaType is the media type of editing schema documents
	MediaType = "application/schema+json"
)

//go:generate go run ../../../internal/schemagen/cmd -out .

//go:embed *.schema.json
var schemaFiles embed.FS

// Document 表示一个已嵌入且通过元数据校验的编辑 JSON 模式 / Document represents one embedded editing JSON schema with validated metadata
type Document struct {
	// FormatID 是模式对应的稳定格式标识符 / FormatID is the stable format identifier associated with the schema
	FormatID string
	// Version 是模式目录中声明的契约版本 / Version is the contract version declared by the schema catalog
	Version string
	// ID 是模式文档的规范标识符 / ID is the canonical identifier of the schema document
	ID string
	// Dialect 是模式文档声明的 JSON Schema 方言 / Dialect is the JSON Schema dialect declared by the document
	Dialect string
	// MediaType 是模式文档的媒体类型 / MediaType is the media type of the schema document
	MediaType string
	// SHA256 是嵌入模式字节的十六进制 SHA-256 摘要 / SHA256 is the hexadecimal SHA-256 digest of the embedded schema bytes
	SHA256 string
	// NativeSuffixes 是模式声明的原生文件后缀 / NativeSuffixes contains native file suffixes declared by the schema
	NativeSuffixes []string
	// JSON 是嵌入模式文档的独立字节副本 / JSON is an independent byte copy of the embedded schema document
	JSON []byte
}

// Lookup 按规范化格式标识符读取并校验嵌入的编辑模式
// Lookup reads and validates an embedded editing schema by normalized format identifier
func Lookup(formatID string) (Document, bool, error) {
	id := strings.ToLower(strings.TrimSpace(formatID))
	if id == "" || strings.ContainsAny(id, `/\\`) {
		return Document{}, false, nil
	}
	data, err := schemaFiles.ReadFile(id + ".schema.json")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Document{}, false, nil
		}
		return Document{}, false, err
	}
	var header struct {
		ID             string   `json:"$id"`
		Schema         string   `json:"$schema"`
		FormatID       string   `json:"x-meido-format-id"`
		Version        string   `json:"x-meido-schema-version"`
		NativeSuffixes []string `json:"x-meido-native-suffixes"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Document{}, false, fmt.Errorf("decode embedded schema %s: %w", id, err)
	}
	if header.FormatID != id || header.Version != Version || header.Schema != Dialect || header.ID == "" {
		return Document{}, false, fmt.Errorf("embedded schema %s has inconsistent metadata", id)
	}
	digest := sha256.Sum256(data)
	return Document{
		FormatID: id, Version: header.Version, ID: header.ID, Dialect: header.Schema,
		MediaType: MediaType, SHA256: fmt.Sprintf("%x", digest[:]),
		NativeSuffixes: append([]string(nil), header.NativeSuffixes...), JSON: append([]byte(nil), data...),
	}, true, nil
}

// Formats 返回按字典序排列的全部嵌入编辑模式格式标识符
// Formats returns all embedded editing-schema format identifiers in lexical order
func Formats() ([]string, error) {
	paths, err := fs.Glob(schemaFiles, "*.schema.json")
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		result = append(result, strings.TrimSuffix(path, ".schema.json"))
	}
	sort.Strings(result)
	return result, nil
}
