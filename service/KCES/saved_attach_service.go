package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// SavedAttachService converts KCES/GP03 SAVED_ATTACH_DATA (.sad) files to and
// from the marker-based JSON representation exposed by serialization/KCES.
type SavedAttachService struct{}

type savedAttachEditingJSON struct {
	Format       *string                              `json:"format"`
	Signature    *string                              `json:"signature"`
	Version      *int32                               `json:"version"`
	Items        *[]serializationKCES.SavedAttachData `json:"items"`
	TrailingData []byte                               `json:"trailingData"`
}

func IsKCESSavedAttachFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), ".sad")
}

func IsKCESSavedAttachJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.EqualFold(filepath.Ext(base), ".sad")
}

func (s *SavedAttachService) ReadSavedAttachFile(path string) (*serializationKCES.SavedAttachFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read saved-attach file %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeSavedAttach(data)
	if err != nil {
		return nil, fmt.Errorf("decode saved-attach file %q: %w", path, err)
	}
	return value, nil
}

func (s *SavedAttachService) ConvertSavedAttachToJSON(inputPath, outputPath string) error {
	value, err := s.ReadSavedAttachFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal saved-attach JSON: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write saved-attach JSON %q: %w", outputPath, err)
	}
	return nil
}

func (s *SavedAttachService) ConvertJSONToSavedAttach(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read saved-attach JSON %q: %w", inputPath, err)
	}
	value, err := decodeSavedAttachEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse saved-attach JSON %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeSavedAttach(value)
	if err != nil {
		return fmt.Errorf("encode saved-attach JSON %q: %w", inputPath, err)
	}
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write saved-attach file %q: %w", outputPath, err)
	}
	return nil
}

func decodeSavedAttachEditingJSON(data []byte) (*serializationKCES.SavedAttachFile, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("saved-attach JSON is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var editing savedAttachEditingJSON
	if err := decoder.Decode(&editing); err != nil {
		return nil, err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON value")
		}
		return nil, fmt.Errorf("trailing content: %w", err)
	}
	if editing.Format == nil {
		return nil, fmt.Errorf("format is missing or null")
	}
	if editing.Signature == nil {
		return nil, fmt.Errorf("signature is missing or null")
	}
	if editing.Version == nil {
		return nil, fmt.Errorf("version is missing or null")
	}
	if *editing.Format != serializationKCES.KCESSavedAttachFormat {
		return nil, fmt.Errorf("unsupported saved-attach JSON format %q", *editing.Format)
	}
	if *editing.Signature != serializationKCES.SavedAttachSignature {
		return nil, fmt.Errorf("invalid saved-attach signature %q", *editing.Signature)
	}
	var items []serializationKCES.SavedAttachData
	if editing.Items != nil {
		items = *editing.Items
	}
	value := serializationKCES.SavedAttachFile{
		Format:       *editing.Format,
		Signature:    *editing.Signature,
		Version:      *editing.Version,
		Items:        items,
		TrailingData: append([]byte(nil), editing.TrailingData...),
	}
	if _, err := serializationKCES.EncodeSavedAttach(&value); err != nil {
		return nil, err
	}
	return &value, nil
}
