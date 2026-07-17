package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

type MaidColliderService struct{}

func IsKCESMaidColliderFile(path string) bool {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	return isMaidColliderBaseName(filepath.Base(path))
}

func IsKCESMaidColliderJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	if !isMaidColliderBaseName(filepath.Base(base)) {
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
	return header.Format == serializationKCES.MaidColliderFormat
}

func isMaidColliderBaseName(name string) bool {
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, ".bytes")
	return name == "maid_collider" || name == "maid_collider_touch"
}

func (s *MaidColliderService) ConvertMaidColliderToJSON(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	value, err := serializationKCES.DecodeMaidCollider(data)
	if err != nil {
		return fmt.Errorf("decode KCES maid collider %q: %w", inputPath, err)
	}
	jsonData, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES maid collider JSON: %w", err)
	}
	jsonData = append(jsonData, '\n')
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *MaidColliderService) ConvertJSONToMaidCollider(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	var value serializationKCES.MaidColliderFile
	if err := decodeStrictJSON(data, &value, "KCES maid collider JSON"); err != nil {
		return fmt.Errorf("parse KCES maid collider JSON: %w", err)
	}
	if value.Format != serializationKCES.MaidColliderFormat {
		return fmt.Errorf("unsupported KCES maid collider JSON format %q", value.Format)
	}
	encoded, err := serializationKCES.EncodeMaidCollider(&value)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}
