package COM3D2

import (
	"bytes"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/utilities"
)

var (
	ArcSignature = []byte{
		0x77, 0x61, 0x72, 0x63, // warc
		0xFF, 0xAA, 0x45, 0xF1, // ?
		0xE8, 0x03, 0x00, 0x00, // 1000
		0x04, 0x00, 0x00, 0x00, // 4
		0x02, 0x00, 0x00, 0x00, // 2
	}

	DirSignature = []byte{
		0x20, 0x00, 0x00, 0x00, // 32
		0x10, 0x00, 0x00, 0x00, // 16
	}
)

type Arc struct {
}

func ReadArc(r io.Reader) (*Arc, error) {
	arc := &Arc{}

	signature, err := utilities.ReadBytes(r, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature: %w", err)
	}
	if !bytes.Equal(signature, ArcSignature) {
		return nil, fmt.Errorf("invalid ARC signature, want %v, got %v", ArcSignature, signature)
	}

	return arc, nil
}
