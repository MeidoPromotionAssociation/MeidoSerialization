package KCES

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestModelSamples(t *testing.T) {
	assertPartsSamplesForSuffixRoundTrip(t, ".model", DecodeModel, EncodeModel)
}

func TestModelSamplesReferenceNativeMMeshAssets(t *testing.T) {
	for _, path := range partsSamplePathsBySuffix(t, ".model") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			model, err := DecodeModel(readPartsSample(t, filepath.Base(path)))
			if err != nil {
				t.Fatalf("decode model: %v", err)
			}
			if model.MeshFileName == "" || !strings.HasSuffix(strings.ToLower(model.MeshFileName), ".mmesh") {
				t.Fatalf("meshfileName = %q, want a native .mmesh Unity Mesh reference", model.MeshFileName)
			}
		})
	}
}
