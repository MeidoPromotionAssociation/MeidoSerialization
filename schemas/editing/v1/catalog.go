// Package editingv1 exposes the checked-in Draft 2020-12 schemas for the
// current editing-JSON contract.
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
	Version   = "1.0.0"
	Dialect   = "https://json-schema.org/draft/2020-12/schema"
	MediaType = "application/schema+json"
)

//go:generate go run ../../../internal/schemagen/cmd -out .

//go:embed *.schema.json
var schemaFiles embed.FS

type Document struct {
	FormatID       string
	Version        string
	ID             string
	Dialect        string
	MediaType      string
	SHA256         string
	NativeSuffixes []string
	JSON           []byte
}

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
