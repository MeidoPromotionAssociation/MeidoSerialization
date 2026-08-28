package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

func TestReadMateRejectsWrongSignatureAndTrailingData(t *testing.T) {
	valid, err := os.ReadFile(filepath.Join("..", "..", "testdata", "test.mate"))
	if err != nil {
		t.Fatal(err)
	}
	if len(valid) < 1+len(MateSignature) || int(valid[0]) != len(MateSignature) {
		t.Fatalf("unexpected test fixture signature prefix")
	}

	wrongSignature := append([]byte(nil), valid...)
	copy(wrongSignature[1:1+len(MateSignature)], []byte("CM3D2_MATERIAX"))
	if _, err := ReadMate(bufio.NewReader(bytes.NewReader(wrongSignature))); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("wrong signature error = %v", err)
	}

	withTrailing := append(append([]byte(nil), valid...), 0xde, 0xad)
	if _, err := ReadMate(bufio.NewReader(bytes.NewReader(withTrailing))); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing data error = %v", err)
	}
}

func TestMateDumpRejectsStructurallyInvalidValues(t *testing.T) {
	for _, test := range []struct {
		name string
		mate *Mate
		want string
	}{
		{name: "wrong signature", mate: &Mate{Signature: "NOT_A_MATERIAL", Material: &Material{}}, want: "signature"},
		{name: "missing material", mate: &Mate{Signature: MateSignature}, want: "material"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := test.mate.Dump(&out); err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("Dump error = %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("invalid mate wrote %d bytes before validation", out.Len())
			}
		})
	}
}

func TestMateDumpRejectsTypedNilPropertyBeforeWriting(t *testing.T) {
	value := &Mate{
		Signature: MateSignature,
		Material:  &Material{Properties: []Property{(*FProperty)(nil)}},
	}
	var output bytes.Buffer
	if err := value.Dump(&output); err == nil {
		t.Fatal("Dump accepted a typed-nil material property")
	}
	if output.Len() != 0 {
		t.Fatalf("rejected mate wrote %d bytes", output.Len())
	}
}

func TestKeywordPropertyWriteRecomputesCount(t *testing.T) {
	value := &KeywordProperty{Count: -1, Keywords: []Keyword{{Key: "A", Value: true}}}
	var wire bytes.Buffer
	if err := value.Write(stream.NewBinaryWriter(&wire)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if value.Count != 1 {
		t.Fatalf("derived Count = %d, want 1", value.Count)
	}
}
