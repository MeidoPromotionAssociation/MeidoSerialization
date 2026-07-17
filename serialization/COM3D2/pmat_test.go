package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPmat(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.pmat")
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

			pmat, err := ReadPMat(f)
			if err != nil {
				t.Fatalf("failed to read pmat: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = pmat.Dump(&buf, false)
			if err != nil {
				t.Fatalf("failed to dump pmat: %v", err)
			}
			dumped := append([]byte(nil), buf.Bytes()...)

			// Re-read from dumped buffer
			pmat2, err := ReadPMat(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped pmat: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(pmat, pmat2) {
				t.Errorf("data mismatch after dump and re-read")
			}
			original, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(dumped, original) {
				t.Errorf("byte round-trip mismatch:\n got %x\nwant %x", dumped, original)
			}
		})
	}
}

func TestPMatAcceptsMissingShaderButAlwaysWritesIt(t *testing.T) {
	original, err := os.ReadFile("../../testdata/test.pmat")
	if err != nil {
		t.Fatal(err)
	}
	// The sample's final field is a one-byte length followed by the shader.
	// Remove it to model an old/incomplete producer.
	shaderBytes := len("Toon/Lighted_CM3D") + 1
	short := original[:len(original)-shaderBytes]
	decoded, err := ReadPMat(bytes.NewReader(short))
	if err != nil {
		t.Fatalf("ReadPMat rejected missing optional shader: %v", err)
	}
	if decoded.Shader != "" {
		t.Fatalf("missing shader decoded as %q", decoded.Shader)
	}
	var output bytes.Buffer
	if err := decoded.Dump(&output, false); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if output.Len() != len(short)+1 || output.Bytes()[output.Len()-1] != 0 {
		t.Fatalf("Dump did not append the mandatory empty shader field: %x", output.Bytes())
	}
}

func TestPMatDumpValidatesNilAndStoresRecalculatedHash(t *testing.T) {
	var nilValue *PMat
	var wire bytes.Buffer
	if err := nilValue.Dump(&wire, false); err == nil || wire.Len() != 0 {
		t.Fatalf("nil Dump error=%v bytes=%d", err, wire.Len())
	}

	value := &PMat{Signature: PMatSignature, Version: 1000, MaterialName: "material", RenderQueue: 2000, Shader: "shader"}
	if err := value.Dump(&wire, true); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	decoded, err := ReadPMat(bytes.NewReader(wire.Bytes()))
	if err != nil {
		t.Fatalf("ReadPMat: %v", err)
	}
	if decoded.Hash != value.Hash {
		t.Fatalf("recalculated hash was not stored on the source: source=%d wire=%d", value.Hash, decoded.Hash)
	}
}
