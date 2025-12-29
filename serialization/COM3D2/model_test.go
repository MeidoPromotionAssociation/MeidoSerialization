package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func TestModel(t *testing.T) {
	filePath := "../../testdata/test.model"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	model, err := ReadModel(br)
	if err != nil {
		t.Fatalf("failed to read model: %v", err)
	}

	if model.Signature != "CM3D2_MESH" {
		t.Errorf("expected signature CM3D2_MESH, got %s", model.Signature)
	}

	// Test Dump
	var buf bytes.Buffer
	bw := stream.NewBinaryWriter(&buf)
	err = model.Dump(bw)
	if err != nil {
		t.Fatalf("failed to dump model: %v", err)
	}

	// Re-read from dumped buffer
	br2 := bufio.NewReader(&buf)
	model2, err := ReadModel(br2)
	if err != nil {
		t.Fatalf("failed to re-read dumped model: %v", err)
	}

	// Compare basic fields
	if model.Version != model2.Version {
		t.Errorf("version mismatch: %d != %d", model.Version, model2.Version)
	}
	if model.Name != model2.Name {
		t.Errorf("model name mismatch: %s != %s", model.Name, model2.Name)
	}
}

func TestReadModelMetadata(t *testing.T) {
	filePath := "../../testdata/test.model"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	model, err := ReadModel(bufio.NewReader(f))
	if err != nil {
		t.Fatalf("failed to read model: %v", err)
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		t.Fatalf("failed to seek: %v", err)
	}
	metadata, err := ReadModelMetadata(bufio.NewReader(f))
	if err != nil {
		t.Fatalf("failed to read model metadata: %v", err)
	}

	if model.Signature != metadata.Signature {
		t.Errorf("Signature mismatch: %s != %s", model.Signature, metadata.Signature)
	}
	if model.Version != metadata.Version {
		t.Errorf("Version mismatch: %d != %d", model.Version, metadata.Version)
	}
	if model.Name != metadata.Name {
		t.Errorf("Name mismatch: %s != %s", model.Name, metadata.Name)
	}
	if model.RootBoneName != metadata.RootBoneName {
		t.Errorf("RootBoneName mismatch: %s != %s", model.RootBoneName, metadata.RootBoneName)
	}
	if len(model.Materials) != len(metadata.Materials) {
		t.Errorf("Materials count mismatch: %d != %d", len(model.Materials), len(metadata.Materials))
	}
}
