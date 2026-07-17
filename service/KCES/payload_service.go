package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// IsKCESPayloadFile reports whether path is a KCES physics/collider payload.
func IsKCESPayloadFile(path string) bool {
	ext := serializationKCES.NormalizeKCESPayloadExtension(path)
	if ext == "" {
		return false
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	return true
}

// IsKCESPayloadJSONFile reports whether path is a JSON representation of a
// KCES physics/collider payload.
func IsKCESPayloadJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return serializationKCES.NormalizeKCESPayloadExtension(base) != ""
}

// PayloadService converts native KCES MessagePack/LZ4 payloads and ExportCM
// JSON sidecars to and from their explicitly tagged editing envelopes.
type PayloadService struct{}

func (s *PayloadService) ConvertPayloadToJson(inputPath string, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}

	envelope, err := serializationKCES.DecodeKCESPayload(data, inputPath)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES payload json: %w", err)
	}
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *PayloadService) ConvertJsonToPayload(inputPath string, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}

	base := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
	expectedExtension := serializationKCES.NormalizeKCESPayloadExtension(base)
	if expectedExtension == "" {
		expectedExtension = serializationKCES.NormalizeKCESPayloadExtension(outputPath)
	}
	envelope, err := decodeKCESPayloadEditingJSON(data, expectedExtension)
	if err != nil {
		return fmt.Errorf("parse KCES payload json: %w", err)
	}

	encoded, err := serializationKCES.EncodeKCESPayload(envelope)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func decodeKCESPayloadEditingJSON(data []byte, expectedExtension string) (*serializationKCES.KCESPayloadEnvelope, error) {
	var envelope serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(data, &envelope, "KCES payload JSON"); err != nil {
		return nil, err
	}
	if envelope.Format != serializationKCES.PayloadFormatKCESMessagePack && envelope.Format != serializationKCES.PayloadFormatKCESExportCM {
		return nil, fmt.Errorf("unsupported KCES payload JSON format %q", envelope.Format)
	}
	expected := serializationKCES.NormalizeKCESPayloadExtension(expectedExtension)
	actual := serializationKCES.NormalizeKCESPayloadExtension(envelope.Extension)
	if actual == "" {
		actual = expected
		envelope.Extension = expected
	}
	if actual == "" {
		return nil, fmt.Errorf("unsupported or missing KCES payload extension %q", envelope.Extension)
	}
	if expected != "" && actual != expected {
		return nil, fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, expected)
	}
	envelope.Extension = actual
	if _, err := serializationKCES.EncodeKCESPayload(&envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}
