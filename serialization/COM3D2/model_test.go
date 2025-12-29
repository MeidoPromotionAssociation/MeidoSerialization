package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestModel(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.model")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
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

			// Test Dump
			var buf bytes.Buffer
			bw := bufio.NewWriter(&buf)
			err = model.Dump(bw)
			if err != nil {
				t.Fatalf("failed to dump model: %v", err)
			}
			bw.Flush()

			// Re-read from a dumped buffer
			br2 := bufio.NewReader(&buf)
			model2, err := ReadModel(br2)
			if err != nil {
				t.Fatalf("failed to re-read dumped model: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(model, model2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}

func TestReadModelMetadata(t *testing.T) {
	files, err := filepath.Glob("../../testdata/test*.model")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
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
		})
	}
}
