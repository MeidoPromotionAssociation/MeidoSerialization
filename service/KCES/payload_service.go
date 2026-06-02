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

// PayloadService 在 KCES MessagePack/LZ4 载荷文件和 JSON 之间转换 / PayloadService converts KCES MessagePack/LZ4 payload files to and from JSON
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

	var envelope serializationKCES.KCESPayloadEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("parse KCES payload json: %w", err)
	}
	if envelope.Extension == "" {
		envelope.Extension = serializationKCES.NormalizeKCESPayloadExtension(outputPath)
	}
	if envelope.Extension == "" {
		base := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
		envelope.Extension = serializationKCES.NormalizeKCESPayloadExtension(base)
	}

	encoded, err := serializationKCES.EncodeKCESPayload(&envelope)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}
