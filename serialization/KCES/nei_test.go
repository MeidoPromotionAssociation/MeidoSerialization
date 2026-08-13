package KCES

import (
	"bytes"
	"testing"
)

func TestNewNeiUsesKCESCellEncoding(t *testing.T) {
	table := NewNei(1, 1, [][]string{{"身体"}})
	if table.TextEncoding != NeiTextEncodingUTF8 {
		t.Fatalf("TextEncoding = %q, want %q", table.TextEncoding, NeiTextEncodingUTF8)
	}

	var encoded bytes.Buffer
	if err := table.Dump(&encoded); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	// KCES reads cells as UTF-8, so the encrypted payload must carry UTF-8 bytes.
	decoded, err := ReadNei(bytes.NewReader(encoded.Bytes()), nil)
	if err != nil {
		t.Fatalf("ReadNei: %v", err)
	}
	if decoded.TextEncoding != NeiTextEncodingUTF8 {
		t.Errorf("round-trip TextEncoding = %q, want %q", decoded.TextEncoding, NeiTextEncodingUTF8)
	}
	if decoded.Data[0][0] != "身体" {
		t.Errorf("round-trip value = %q, want 身体", decoded.Data[0][0])
	}
}
