package ct

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestExtendedVirtualDirectoryFramingRoundTrip(t *testing.T) {
	table := &ContentTable{
		Version: ctVersion,
		Framing: VirtualDirectoryFramingExtended,
		Directories: map[string]VirtualDirectoryMetadata{
			"EditData":              {Version: 1000},
			"EditData/color_preset": {Version: 1000},
		},
		Raw:   make([]byte, HeaderSize),
		Files: make(map[string]VirtualFile),
	}
	if err := table.AddFile("EditData/state", []byte("extended-frame-payload")); err != nil {
		t.Fatal(err)
	}

	var wire bytes.Buffer
	if err := WriteContentTable(&wire, table); err != nil {
		t.Fatalf("WriteContentTable: %v", err)
	}
	data := wire.Bytes()
	if data[7] != SerializeTypeMsgPackExtended {
		t.Fatalf("serialize marker = 0x%02x, want 0x%02x", data[7], SerializeTypeMsgPackExtended)
	}
	if !bytes.Equal(data[len(data)-len(extendedFooterMagic):], extendedFooterMagic[:]) {
		t.Fatalf("footer signature = %x", data[len(data)-len(extendedFooterMagic):])
	}
	metadataSize := binary.LittleEndian.Uint64(data[len(data)-extendedFooterSize : len(data)-len(extendedFooterMagic)])
	if metadataSize == 0 || metadataSize > uint64(len(data)-HeaderSize-extendedFooterSize) {
		t.Fatalf("metadata size = %d for %d-byte frame", metadataSize, len(data))
	}

	decoded, err := ReadContentTable(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadContentTable: %v", err)
	}
	if decoded.Framing != VirtualDirectoryFramingExtended {
		t.Fatalf("framing = %d, want extended", decoded.Framing)
	}
	payload, err := decoded.GetFileData("EditData/state")
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "extended-frame-payload" {
		t.Fatalf("payload = %q", payload)
	}

	var rewritten bytes.Buffer
	if err := WriteContentTable(&rewritten, decoded); err != nil {
		t.Fatalf("rewrite extended frame: %v", err)
	}
	if rewritten.Bytes()[7] != SerializeTypeMsgPackExtended {
		t.Fatalf("rewritten serialize marker = 0x%02x", rewritten.Bytes()[7])
	}
	if _, err := ReadContentTable(bytes.NewReader(rewritten.Bytes())); err != nil {
		t.Fatalf("read rewritten extended frame: %v", err)
	}
}

func TestExtendedVirtualDirectoryFramingRejectsMalformedFooter(t *testing.T) {
	table := &ContentTable{
		Version: ctVersion,
		Framing: VirtualDirectoryFramingExtended,
		Raw:     make([]byte, HeaderSize),
		Files:   make(map[string]VirtualFile),
	}
	if err := table.AddFile("state", []byte{1}); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := WriteContentTable(&encoded, table); err != nil {
		t.Fatal(err)
	}
	valid := encoded.Bytes()

	tests := []struct {
		name   string
		mutate func([]byte) []byte
		want   string
	}{
		{
			name: "bad signature",
			mutate: func(data []byte) []byte {
				data[len(data)-1] ^= 0xff
				return data
			},
			want: "footer signature",
		},
		{
			name: "zero size",
			mutate: func(data []byte) []byte {
				binary.LittleEndian.PutUint64(data[len(data)-extendedFooterSize:], 0)
				return data
			},
			want: "invalid extended msgpack size",
		},
		{
			name: "oversized metadata",
			mutate: func(data []byte) []byte {
				binary.LittleEndian.PutUint64(data[len(data)-extendedFooterSize:], uint64(len(data)))
				return data
			},
			want: "invalid extended msgpack size",
		},
		{
			name: "truncated footer",
			mutate: func(data []byte) []byte {
				return data[:HeaderSize+extendedFooterSize]
			},
			want: "file too small",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			malformed := test.mutate(append([]byte(nil), valid...))
			_, err := ReadContentTable(bytes.NewReader(malformed))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ReadContentTable() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestWriteContentTableRejectsUnsupportedFramingBeforeWriting(t *testing.T) {
	table := &ContentTable{Framing: VirtualDirectoryFraming(255)}
	var output bytes.Buffer
	err := WriteContentTable(&output, table)
	if err == nil || !strings.Contains(err.Error(), "unsupported VirtualDirectory framing") {
		t.Fatalf("WriteContentTable() error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("writer received %d bytes", output.Len())
	}
}
