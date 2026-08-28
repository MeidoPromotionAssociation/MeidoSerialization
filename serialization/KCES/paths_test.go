package KCES

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio"
)

func TestKCESPathsRoundTrip(t *testing.T) {
	input := NewKCESPathsFile()
	input.Paths = []string{"system", "parts", "日本語", "cas"}
	encoded, err := EncodeKCESPaths(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeKCESPaths(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, input) {
		t.Fatalf("round trip = %+v, want paths=%v", decoded, input.Paths)
	}
}

func TestKCESPathsRejectsMalformedWireAndJSONValues(t *testing.T) {
	value := NewKCESPathsFile()
	value.Paths = []string{"system"}
	valid, err := EncodeKCESPaths(value)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("trailing", func(t *testing.T) {
		if _, err := DecodeKCESPaths(append(append([]byte(nil), valid...), 0)); err == nil || !strings.Contains(err.Error(), "trailing") {
			t.Fatalf("trailing-data error=%v", err)
		}
	})
	t.Run("negative count", func(t *testing.T) {
		var out bytes.Buffer
		_ = binaryio.WriteString(&out, KCESPathsSignature)
		_ = binaryio.WriteInt32(&out, 1000)
		_ = binaryio.WriteInt32(&out, -1)
		out.Write([]byte{1, 2, 3})
		if _, err := DecodeKCESPaths(out.Bytes()); err == nil || !strings.Contains(err.Error(), "negative") {
			t.Fatalf("negative count error=%v", err)
		}
	})
	t.Run("impossible count", func(t *testing.T) {
		var out bytes.Buffer
		_ = binaryio.WriteString(&out, KCESPathsSignature)
		_ = binaryio.WriteInt32(&out, 1000)
		_ = binaryio.WriteInt32(&out, 2)
		out.WriteByte(0)
		if _, err := DecodeKCESPaths(out.Bytes()); err == nil || !strings.Contains(err.Error(), "cannot fit") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("count above former implementation limit", func(t *testing.T) {
		var out bytes.Buffer
		_ = binaryio.WriteString(&out, KCESPathsSignature)
		_ = binaryio.WriteInt32(&out, 1000)
		_ = binaryio.WriteInt32(&out, (1<<20)+1)
		out.WriteByte(0)
		if _, err := DecodeKCESPaths(out.Bytes()); err == nil || !strings.Contains(err.Error(), "cannot fit") {
			t.Fatalf("error = %v, want physical truncation rather than an arbitrary count limit", err)
		}
	})
	t.Run("bad signature", func(t *testing.T) {
		bad := append([]byte(nil), valid...)
		bad[1] = 'X'
		decoded, err := DecodeKCESPaths(bad)
		if err != nil || decoded.Signature == KCESPathsSignature {
			t.Fatalf("decoded=%+v error=%v", decoded, err)
		}
	})
	t.Run("truncated string", func(t *testing.T) {
		bad := append([]byte(nil), valid[:len(valid)-1]...)
		if _, err := DecodeKCESPaths(bad); err == nil {
			t.Fatal("truncated path unexpectedly decoded")
		}
	})
	t.Run("invalid UTF-8", func(t *testing.T) {
		var out bytes.Buffer
		_ = binaryio.WriteString(&out, KCESPathsSignature)
		_ = binary.Write(&out, binary.LittleEndian, int32(1000))
		_ = binary.Write(&out, binary.LittleEndian, int32(1))
		out.WriteByte(1)
		out.WriteByte(0xff)
		wire := out.Bytes()
		decoded, err := DecodeKCESPaths(wire)
		if err != nil || len(decoded.Paths) != 1 || decoded.Paths[0] != string([]byte{0xff}) {
			t.Fatalf("decoded=%+v error=%v", decoded, err)
		}
		reencoded, err := EncodeKCESPaths(decoded)
		if err != nil || !bytes.Equal(reencoded, wire) {
			t.Fatalf("invalid-UTF8 round trip=%x error=%v want=%x", reencoded, err, wire)
		}
	})
	for _, raw := range []*KCESPathsFile{
		{Signature: "wrong", Version: -7, Paths: []string{"", "has\x00nul"}},
	} {
		wire, err := EncodeKCESPaths(raw)
		if err != nil {
			t.Fatalf("wire values should be preserved: %v", err)
		}
		decoded, err := DecodeKCESPaths(wire)
		if err != nil || !reflect.DeepEqual(decoded.Paths, raw.Paths) || decoded.Version != -7 || decoded.Signature != "wrong" {
			t.Fatalf("wire values changed: decoded=%+v err=%v", decoded, err)
		}
	}
}
