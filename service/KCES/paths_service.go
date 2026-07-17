package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

type PathsService struct{}

func IsKCESPathsFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Base(path), "paths.dat")
}

func IsKCESPathsJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), "paths.dat.json") {
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
	return header.Format == serializationKCES.KCESPathsFormat
}

func (s *PathsService) ConvertPathsToJSON(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read paths.dat %q: %w", inputPath, err)
	}
	value, err := serializationKCES.DecodeKCESPaths(data)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal paths.dat JSON: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write paths.dat JSON %q: %w", outputPath, err)
	}
	return nil
}

func (s *PathsService) ConvertJSONToPaths(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read paths.dat JSON %q: %w", inputPath, err)
	}
	var value serializationKCES.KCESPathsFile
	if err := decodeStrictJSON(data, &value, "KCES paths.dat JSON"); err != nil {
		return fmt.Errorf("parse paths.dat JSON: %w", err)
	}
	if value.Format != serializationKCES.KCESPathsFormat {
		return fmt.Errorf("unsupported paths.dat JSON format %q", value.Format)
	}
	if value.Signature != serializationKCES.KCESPathsSignature {
		return fmt.Errorf("invalid paths.dat JSON signature %q", value.Signature)
	}
	encoded, err := serializationKCES.EncodeKCESPaths(&value)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write paths.dat %q: %w", outputPath, err)
	}
	return nil
}
