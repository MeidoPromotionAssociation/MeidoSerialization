package application

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	serializationCRC3D3 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/CRC3D3"
)

// TestEngineConvertsCRC3D3Neimp 验证默认注册表能通过应用层接口检测、转换并校验 .neimp
// TestEngineConvertsCRC3D3Neimp verifies that the default registry detects, converts, and validates .neimp through the application interface
func TestEngineConvertsCRC3D3Neimp(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(EngineOptions{})

	registered, found := DefaultRegistry().Lookup("crc3d3.neimp")
	if !found {
		t.Fatalf("crc3d3.neimp is not registered")
	}
	if !registered.Capability.Detect || !registered.Capability.Convert || !registered.Capability.Validate || registered.Capability.Archive {
		t.Fatalf("crc3d3.neimp capability = %+v", registered.Capability)
	}
	if registered.SchemaVersion == "" || registered.SchemaID == "" || registered.SchemaSHA256 == "" {
		t.Fatalf("crc3d3.neimp has no published schema metadata: %+v", registered)
	}
	if registered.GuideVersion == "" || registered.GuideVerification != "serialization_verified" {
		t.Fatalf("crc3d3.neimp has no reviewed guide: %+v", registered)
	}

	native, err := os.ReadFile("../testdata/CRC3D3/csv.aba_unpacked/TextAsset/cast_data.neimp")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}

	detection, err := engine.Detect(ctx, NewBytesSource("cast_data.neimp", native))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if detection.FormatID != "crc3d3.neimp" || detection.Representation != RepresentationNative {
		t.Fatalf("detection = %+v", detection)
	}

	artifact, editingJSON, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource("cast_data.neimp", native),
		To:     RepresentationEditingJSON,
	})
	if err != nil {
		t.Fatalf("Convert to editing JSON: %v", err)
	}
	if artifact.Name != "cast_data.neimp.json" || artifact.FormatID != "crc3d3.neimp" || !json.Valid(editingJSON) {
		t.Fatalf("editing artifact = %+v, JSON valid=%v", artifact, json.Valid(editingJSON))
	}
	if _, err := engine.Validate(ctx, NewBytesSource(artifact.Name, editingJSON), ""); err != nil {
		t.Fatalf("Validate editing JSON: %v", err)
	}

	backArtifact, back, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource(artifact.Name, editingJSON),
		To:     RepresentationNative,
	})
	if err != nil {
		t.Fatalf("Convert to native: %v", err)
	}
	if backArtifact.Name != "cast_data.neimp" {
		t.Fatalf("native output name = %q", backArtifact.Name)
	}

	original, err := serializationCRC3D3.DecodeNeimp(native)
	if err != nil {
		t.Fatalf("decode original: %v", err)
	}
	restored, err := serializationCRC3D3.DecodeNeimp(back)
	if err != nil {
		t.Fatalf("decode restored: %v", err)
	}
	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("value tree changed through the application round trip")
	}
}
