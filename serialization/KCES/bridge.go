package KCES

import (
	"bytes"
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

const (
	// GP03BridgeSignature is the BinaryWriter string at the start of every
	// bridge file emitted by KCES ExportCM.ExportMaidData.
	GP03BridgeSignature = "GP03_BRIDGE"
	// GP03BridgeVersion is the bridge version emitted by KCES 1.34.4.
	GP03BridgeVersion int32 = 2001
	// GP03BridgeCOM3D2Version is emitted by COM3D2_5 Maid.ExportBridgeGP03
	// for the reverse COM3D2 -> KCES transfer. Observed game output leaves the
	// legacy slot empty and may leave the current-preset slot empty; the outer
	// serializer deliberately does not enforce either inner-payload convention.
	GP03BridgeCOM3D2Version int32 = 2000
	// KCESGP03BridgeFormat identifies the editable JSON representation.
	KCESGP03BridgeFormat = "kces-gp03-bridge"
)

// GP03BridgeFile represents the shared GP03 bridge outer wire written by KCES
// ExportCM.cs (v2001) and COM3D2_5 Maid.ExportBridgeGP03 (v2000).
// LegacyPreset and CurrentPreset are deliberately opaque. Their inner formats
// have independent versioning and game-side migration callbacks, so the outer
// bridge codec only preserves the two length-delimited byte strings verbatim.
type GP03BridgeFile struct {
	Format        string `json:"format"`
	Signature     string `json:"signature"`
	Version       int32  `json:"version"`
	GUID          string `json:"guid"`
	LegacyPreset  []byte `json:"legacyPreset"`
	CurrentPreset []byte `json:"currentPreset"`
	TrailingData  []byte `json:"trailingData,omitempty"`
}

// IsGP03BridgeData reports whether data begins with BinaryWriter's encoding of
// the GP03_BRIDGE string. Full validation remains the responsibility of
// DecodeGP03Bridge.
func IsGP03BridgeData(data []byte) bool {
	// GP03_BRIDGE is eleven UTF-8 bytes, so BinaryWriter uses a single 7-bit
	// length byte. A fixed prefix avoids allocating based on an attacker-owned
	// length while probing unrelated files.
	return bytes.HasPrefix(data, []byte("\x0b"+GP03BridgeSignature))
}

// DecodeGP03Bridge decodes the shared bridge layout. Stored versions and both
// length-delimited preset blobs remain opaque and are preserved verbatim.
func DecodeGP03Bridge(data []byte) (*GP03BridgeFile, error) {
	r := bytes.NewReader(data)
	br := stream.NewBinaryReader(r)

	signature, err := readGP03BridgeString(br, "signature")
	if err != nil {
		return nil, err
	}
	if signature != GP03BridgeSignature {
		return nil, fmt.Errorf("invalid GP03 bridge signature %q (expected %q)", signature, GP03BridgeSignature)
	}

	version, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read GP03 bridge version: %w", err)
	}

	guid, err := readGP03BridgeString(br, "GUID")
	if err != nil {
		return nil, err
	}

	legacyLength, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read GP03 bridge legacy preset length: %w", err)
	}
	if legacyLength < 0 {
		return nil, fmt.Errorf("negative GP03 bridge legacy preset length %d", legacyLength)
	}
	// A current-preset Int32 length must remain after the legacy payload.
	if r.Len() < 4 || int64(legacyLength) > int64(r.Len()-4) {
		return nil, fmt.Errorf("GP03 bridge legacy preset length %d cannot fit in %d bytes while preserving the current preset length", legacyLength, r.Len())
	}
	legacyPreset := make([]byte, int(legacyLength))
	if _, err := io.ReadFull(r, legacyPreset); err != nil {
		return nil, fmt.Errorf("read GP03 bridge legacy preset payload: %w", err)
	}

	currentLength, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read GP03 bridge current preset length: %w", err)
	}
	if currentLength < 0 {
		return nil, fmt.Errorf("negative GP03 bridge current preset length %d", currentLength)
	}
	if int64(currentLength) > int64(r.Len()) {
		return nil, fmt.Errorf("GP03 bridge current preset length %d exceeds %d remaining bytes", currentLength, r.Len())
	}
	currentPreset := make([]byte, int(currentLength))
	if _, err := io.ReadFull(r, currentPreset); err != nil {
		return nil, fmt.Errorf("read GP03 bridge current preset payload: %w", err)
	}
	result := &GP03BridgeFile{
		Format:        KCESGP03BridgeFormat,
		Signature:     signature,
		Version:       version,
		GUID:          guid,
		LegacyPreset:  legacyPreset,
		CurrentPreset: currentPreset,
	}
	if r.Len() != 0 {
		result.TrailingData = make([]byte, r.Len())
		if _, err := io.ReadFull(r, result.TrailingData); err != nil {
			return nil, fmt.Errorf("read GP03 bridge trailing data: %w", err)
		}
	}
	return result, nil
}

// EncodeGP03Bridge writes the exact shared BinaryWriter layout. The version is
// never defaulted or upgraded; use NewGP03BridgeFile when creating a new KCES
// -> COM3D2 bridge.
func EncodeGP03Bridge(value *GP03BridgeFile) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil GP03 bridge")
	}
	if value.Format != "" && value.Format != KCESGP03BridgeFormat {
		return nil, fmt.Errorf("unsupported GP03 bridge JSON format %q", value.Format)
	}
	signature := value.Signature
	if signature != GP03BridgeSignature {
		return nil, fmt.Errorf("invalid GP03 bridge signature %q (expected %q)", signature, GP03BridgeSignature)
	}
	version := value.Version
	if uint64(len(value.LegacyPreset)) > uint64(math.MaxInt32) {
		return nil, fmt.Errorf("GP03 bridge legacy preset length %d exceeds Int32", len(value.LegacyPreset))
	}
	if uint64(len(value.CurrentPreset)) > uint64(math.MaxInt32) {
		return nil, fmt.Errorf("GP03 bridge current preset length %d exceeds Int32", len(value.CurrentPreset))
	}

	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	if err := bw.WriteString(signature); err != nil {
		return nil, fmt.Errorf("write GP03 bridge signature: %w", err)
	}
	if err := bw.WriteInt32(version); err != nil {
		return nil, fmt.Errorf("write GP03 bridge version: %w", err)
	}
	if err := bw.WriteString(value.GUID); err != nil {
		return nil, fmt.Errorf("write GP03 bridge GUID: %w", err)
	}
	if err := bw.WriteInt32(int32(len(value.LegacyPreset))); err != nil {
		return nil, fmt.Errorf("write GP03 bridge legacy preset length: %w", err)
	}
	if err := bw.WriteBytes(value.LegacyPreset); err != nil {
		return nil, fmt.Errorf("write GP03 bridge legacy preset: %w", err)
	}
	if err := bw.WriteInt32(int32(len(value.CurrentPreset))); err != nil {
		return nil, fmt.Errorf("write GP03 bridge current preset length: %w", err)
	}
	if err := bw.WriteBytes(value.CurrentPreset); err != nil {
		return nil, fmt.Errorf("write GP03 bridge current preset: %w", err)
	}
	if len(value.TrailingData) != 0 {
		if err := bw.WriteBytes(value.TrailingData); err != nil {
			return nil, fmt.Errorf("write GP03 bridge trailing data: %w", err)
		}
	}
	return out.Bytes(), nil
}

func NewGP03BridgeFile() *GP03BridgeFile {
	return &GP03BridgeFile{
		Format:    KCESGP03BridgeFormat,
		Signature: GP03BridgeSignature,
		Version:   GP03BridgeVersion,
	}
}

func readGP03BridgeString(br *stream.BinaryReader, field string) (string, error) {
	value, err := br.ReadString()
	if err != nil {
		return "", fmt.Errorf("read GP03 bridge %s: %w", field, err)
	}
	return value, nil
}
