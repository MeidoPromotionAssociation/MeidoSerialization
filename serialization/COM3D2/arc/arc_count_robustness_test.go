package arc

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func TestReadHashTableRejectsNegativeCountsWithoutPanicking(t *testing.T) {
	tests := []struct {
		name               string
		directories, files int32
		depth              int32
	}{
		{name: "directories", directories: -1},
		{name: "files", files: -1},
		{name: "depth", depth: -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var wire bytes.Buffer
			writer := stream.NewBinaryWriter(&wire)
			for _, err := range []error{
				writer.WriteInt64(0),
				writer.WriteUInt64(0),
				writer.WriteInt32(test.directories),
				writer.WriteInt32(test.files),
				writer.WriteInt32(test.depth),
				writer.WriteInt32(0),
			} {
				if err != nil {
					t.Fatalf("build fixture: %v", err)
				}
			}

			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("readHashTable panicked: %v", recovered)
				}
			}()
			_, err := readHashTable(stream.NewBinaryReader(bytes.NewReader(wire.Bytes())))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "negative") {
				t.Fatalf("error = %v, want a negative-count rejection", err)
			}
		})
	}
}

func TestReadHashTableDoesNotPreallocateHugePositiveCountsOnTruncatedStreams(t *testing.T) {
	const hugeCount int32 = 1<<31 - 1
	tests := []struct {
		name               string
		directories, files int32
		depth              int32
	}{
		{name: "directories", directories: hugeCount},
		{name: "files", files: hugeCount},
		{name: "depth", depth: hugeCount},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var wire bytes.Buffer
			writer := stream.NewBinaryWriter(&wire)
			for _, err := range []error{
				writer.WriteInt64(0),
				writer.WriteUInt64(0),
				writer.WriteInt32(test.directories),
				writer.WriteInt32(test.files),
				writer.WriteInt32(test.depth),
				writer.WriteInt32(0),
			} {
				if err != nil {
					t.Fatalf("build fixture: %v", err)
				}
			}

			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("readHashTable panicked: %v", recovered)
				}
			}()
			if _, err := readHashTable(stream.NewBinaryReader(bytes.NewReader(wire.Bytes()))); err == nil {
				t.Fatal("truncated hash table unexpectedly succeeded")
			}
		})
	}
}
