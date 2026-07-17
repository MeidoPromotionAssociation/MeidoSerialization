package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

func isExportCMPayloadExtension(extension string) bool {
	switch NormalizeKCESPayloadExtension(extension) {
	case ".dbconf", ".dbcol", ".dslcol":
		return true
	default:
		return false
	}
}

func decodeExportCMPayload(data []byte, extension string) (*KCESPayloadEnvelope, error) {
	ext := NormalizeKCESPayloadExtension(extension)
	storageVariant := PayloadStorageExportCMUnityJSON
	jsonData := data
	if ext == ".dslcol" {
		storageVariant = PayloadStorageExportCMDotNetStringJSON
		var err error
		jsonData, err = decodeExportCMDotNetString(data)
		if err != nil {
			return nil, fmt.Errorf("decode ExportCM BinaryWriter string: %w", err)
		}
	}
	if !utf8.Valid(jsonData) {
		return nil, fmt.Errorf("ExportCM JSON is not valid UTF-8")
	}

	kind := PayloadKindExportCMColliderJSON
	if ext == ".dbconf" {
		kind = PayloadKindExportCMDynamicBoneJSON
	}

	compact, err := compactExportCMJSON(jsonData)
	if err != nil {
		return nil, err
	}
	return &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESExportCM,
		Extension:      ext,
		LengthPrefixed: false,
		StorageVariant: storageVariant,
		Kind:           kind,
		Text:           string(jsonData),
		JSON:           compact,
	}, nil
}

func encodeExportCMPayload(env *KCESPayloadEnvelope, storageVariant string) ([]byte, error) {
	ext := NormalizeKCESPayloadExtension(env.Extension)
	if env.LengthPrefixed {
		return nil, fmt.Errorf("ExportCM storageVariant %q does not use the int32 lengthPrefixed wire", storageVariant)
	}

	expectedKind := PayloadKindExportCMColliderJSON
	expectedStorage := PayloadStorageExportCMUnityJSON
	switch ext {
	case ".dbconf":
		expectedKind = PayloadKindExportCMDynamicBoneJSON
	case ".dbcol":
	case ".dslcol":
		expectedStorage = PayloadStorageExportCMDotNetStringJSON
	default:
		return nil, fmt.Errorf("extension %q has no ExportCM JSON wire", ext)
	}
	if storageVariant != expectedStorage {
		return nil, fmt.Errorf("extension %q requires ExportCM storageVariant %q, got %q", ext, expectedStorage, storageVariant)
	}
	if env.Kind != expectedKind {
		return nil, fmt.Errorf("extension %q with storageVariant %q requires kind %q, got %q", ext, storageVariant, expectedKind, env.Kind)
	}

	jsonData, err := editableExportCMJSON(env)
	if err != nil {
		return nil, err
	}

	if storageVariant == PayloadStorageExportCMDotNetStringJSON {
		return encodeExportCMDotNetString(jsonData)
	}
	return append([]byte(nil), jsonData...), nil
}

func editableExportCMJSON(env *KCESPayloadEnvelope) ([]byte, error) {
	if len(env.JSON) != 0 {
		if !utf8.Valid(env.JSON) {
			return nil, fmt.Errorf("ExportCM envelope json is not valid UTF-8")
		}
		compactJSON, err := compactExportCMJSON(env.JSON)
		if err != nil {
			return nil, err
		}

		// Text is the exact sidecar string captured by the decoder, while JSON is
		// its editable parsed view. If that view is semantically unchanged, keep
		// the original whitespace and optional UTF-8 BOM byte-for-byte. Only an
		// actual JSON edit replaces the original text with compact JSON.
		if env.Text != "" && utf8.ValidString(env.Text) {
			compactText, textErr := compactExportCMJSON([]byte(env.Text))
			if textErr == nil && bytes.Equal(compactText, compactJSON) {
				return []byte(env.Text), nil
			}
		}
		return append([]byte(nil), compactJSON...), nil
	}
	if !utf8.ValidString(env.Text) {
		return nil, fmt.Errorf("ExportCM envelope text is not valid UTF-8")
	}
	if env.Text == "" {
		return nil, fmt.Errorf("ExportCM envelope json or text is required")
	}
	text := []byte(env.Text)
	if _, err := compactExportCMJSON(text); err != nil {
		return nil, err
	}
	return text, nil
}

func compactExportCMJSON(data []byte) (json.RawMessage, error) {
	data = trimExportCMUTF8BOM(data)
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return nil, fmt.Errorf("ExportCM JSON is invalid: %w", err)
	}
	return append(json.RawMessage(nil), compact.Bytes()...), nil
}

func decodeExportCMDotNetString(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)
	value, err := binaryio.ReadString(r)
	if err != nil {
		return nil, err
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("BinaryWriter string has %d trailing bytes", r.Len())
	}
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("BinaryWriter string is not valid UTF-8")
	}
	return []byte(value), nil
}

func encodeExportCMDotNetString(value []byte) ([]byte, error) {
	if !utf8.Valid(value) {
		return nil, fmt.Errorf("ExportCM BinaryWriter string is not valid UTF-8")
	}
	var out bytes.Buffer
	if err := binaryio.WriteString(&out, string(value)); err != nil {
		return nil, fmt.Errorf("write ExportCM BinaryWriter string: %w", err)
	}
	return out.Bytes(), nil
}

func trimExportCMUTF8BOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return data[3:]
	}
	return data
}
