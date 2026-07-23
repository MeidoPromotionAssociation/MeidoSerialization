package KCES

import (
	"math"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestDecodeCompressedMsgpack_PropagatesRecognizedEnvelopeErrors(t *testing.T) {
	raw, err := ct.EncodeMsgpack([]interface{}{int64(1000), "payload long enough to produce a compressed envelope"})
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := ct.CompressLz4BlockArray(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) < 2 {
		t.Fatalf("compressed envelope too short: %d", len(compressed))
	}

	var out []interface{}
	err = decodeCompressedMsgpack(compressed[:len(compressed)-1], &out, "test value")
	if err == nil {
		t.Fatal("corrupt recognized LZ4 envelope unexpectedly decoded")
	}
	if !strings.Contains(err.Error(), "decompress test value") {
		t.Fatalf("error lost decompression context: %v", err)
	}
}

func TestNumericHelpersPreserveCLRFloatConversionsAndRejectNegativeUnsignedValues(t *testing.T) {
	if _, ok := toUint64Val(int64(-1)); ok {
		t.Fatal("toUint64Val wrapped a negative int64")
	}
	if value, ok := toFloat32(math.MaxFloat64); !ok || !math.IsInf(float64(value), 1) {
		t.Fatalf("toFloat32 overflow conversion = %v, %v; want +Inf", value, ok)
	}
	if value, ok := toFloat32(math.NaN()); !ok || !math.IsNaN(float64(value)) {
		t.Fatalf("toFloat32 NaN conversion = %v, %v", value, ok)
	}
	if value, ok := toFloat64(math.Inf(1)); !ok || !math.IsInf(value, 1) {
		t.Fatalf("toFloat64 +Inf conversion = %v, %v", value, ok)
	}
}

func TestDecodeCompressedMsgpack_AllowsOrdinaryMessagePack(t *testing.T) {
	raw, err := ct.EncodeMsgpack([]interface{}{int64(1000), "plain"})
	if err != nil {
		t.Fatal(err)
	}
	var out []interface{}
	if err := decodeCompressedMsgpack(raw, &out, "plain value"); err != nil {
		t.Fatalf("ordinary MessagePack rejected: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("decoded array length got %d, want 2", len(out))
	}
}
