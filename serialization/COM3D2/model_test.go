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
