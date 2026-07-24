package KCES

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestDecodeHitCheckRejectsImpossibleCountBeforeAllocation(t *testing.T) {
	// BinaryWriter string "HitCheck", followed by an impossible positive count.
	data := append([]byte{byte(len(HitCheckSignature))}, []byte(HitCheckSignature)...)
	var count [4]byte
	binary.LittleEndian.PutUint32(count[:], uint32(1<<30))
	data = append(data, count[:]...)

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("DecodeHitCheck panicked: %v", recovered)
		}
	}()
	_, err := DecodeHitCheck(data)
	if err == nil || !strings.Contains(err.Error(), "exceeds remaining data capacity") {
		t.Fatalf("error = %v, want count/capacity rejection", err)
	}
}

func TestEncodeHitCheckRejectsNonGameSignature(t *testing.T) {
	if _, err := EncodeHitCheck(&HitCheck{Signature: "not-hitcheck"}); err == nil {
		t.Fatal("EncodeHitCheck accepted a signature its own decoder and the game reject")
	}
	if _, err := EncodeHitCheck(&HitCheck{}); err == nil {
		t.Fatal("EncodeHitCheck silently injected the current signature")
	}
}

func TestHitCheckRejectsTrailingBytesAndNegativeCount(t *testing.T) {
	t.Run("ordinary trailing bytes", func(t *testing.T) {
		value := NewHitCheck()
		wire, err := EncodeHitCheck(value)
		if err != nil {
			t.Fatal(err)
		}
		wire = append(wire, 0xde, 0xad, 0xbe, 0xef)
		if _, err := DecodeHitCheck(wire); err == nil || !strings.Contains(err.Error(), "trailing") {
			t.Fatalf("DecodeHitCheck() error = %v, want trailing-data rejection", err)
		}
	})

	t.Run("negative count", func(t *testing.T) {
		wire := append([]byte{byte(len(HitCheckSignature))}, []byte(HitCheckSignature)...)
		wire = append(wire, 0xff, 0xff, 0xff, 0xff, 1, 2, 3)
		if _, err := DecodeHitCheck(wire); err == nil || !strings.Contains(err.Error(), "negative") {
			t.Fatalf("negative count error = %v", err)
		}
	})
}
