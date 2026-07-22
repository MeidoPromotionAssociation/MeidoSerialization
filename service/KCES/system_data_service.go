package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// SystemDataService converts the VirtualDirectory-based system.dat file while
// exposing known EditData payloads as typed JSON and preserving unknown files
// as base64-encoded []byte map values.
type SystemDataService struct{}

func IsKCESSystemDataFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Base(path), "system.dat")
}

func IsKCESSystemDataJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), "system.dat.json") {
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
	return header.Format == serializationKCES.KCESSystemDataFormat
}

func (s *SystemDataService) ReadSystemDataFile(path string) (*serializationKCES.KCESSystemData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read KCES system.dat %q: %w", path, err)
	}
	value, err := serializationKCES.DecodeKCESSystemData(data)
	if err != nil {
		return nil, fmt.Errorf("parse KCES system.dat %q: %w", path, err)
	}
	return value, nil
}

func (s *SystemDataService) ConvertSystemDataToJSON(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	input, err := readConversionFile(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("read KCES system.dat %q: %w", inputPath, err)
	}
	value, err := serializationKCES.DecodeKCESSystemData(input)
	if err != nil {
		return fmt.Errorf("parse KCES system.dat %q: %w", inputPath, err)
	}
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES system.dat JSON: %w", err)
	}
	data = append(data, '\n')
	if err := writeConversionFile(ctx, outputPath, data, maxOutputBytes); err != nil {
		return fmt.Errorf("write KCES system.dat JSON %q: %w", outputPath, err)
	}
	return nil
}

func (s *SystemDataService) ConvertJSONToSystemData(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	data, err := readConversionFile(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("read KCES system.dat JSON %q: %w", inputPath, err)
	}
	value, err := decodeKCESSystemDataEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse KCES system.dat JSON %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeKCESSystemData(value)
	if err != nil {
		return fmt.Errorf("encode KCES system.dat JSON %q: %w", inputPath, err)
	}
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionFile(ctx, outputPath, encoded, maxOutputBytes); err != nil {
		return fmt.Errorf("write KCES system.dat %q: %w", outputPath, err)
	}
	return nil
}

func decodeKCESSystemDataEditingJSON(data []byte) (*serializationKCES.KCESSystemData, error) {
	var value serializationKCES.KCESSystemData
	if err := decodeStrictJSON(data, &value, "KCES system.dat JSON"); err != nil {
		return nil, err
	}
	if value.Format != serializationKCES.KCESSystemDataFormat {
		return nil, fmt.Errorf("unsupported KCES system.dat JSON format %q", value.Format)
	}
	if _, err := serializationKCES.EncodeKCESSystemData(&value); err != nil {
		return nil, err
	}
	return &value, nil
}
